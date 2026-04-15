---
feature: FEAT-0008
title: BFF Server
status: proposed
date: 2026-04-14
adr-constraints:
  - ADR-0001: Go as primary language
  - ADR-0002: SQLite storage, WAL mode
  - ADR-0004: Viper configuration, non-global instances
  - ADR-0005: Always full capture, retention-based pruning
  - ADR-0006: Provider adapter interface
  - ADR-0007: Pre-computed aggregation tables
promoted-from:
  - EXP-0008: Integrated Harness
---

# FEAT-0008: BFF Server

## Problem

Modeltap v1 is a transparent HTTP reverse proxy — it receives provider-shaped API requests, forwards them as-is, and captures traffic passively. This architecture cannot support the integrated harness (EXP-0008) because the harness needs a server that:

- Speaks a purpose-built protocol (not provider HTTP APIs)
- Owns conversation state across turns, sessions, and model switches
- Translates between a canonical message format and provider-specific formats
- Assembles and injects system prompts
- Applies model routing policy (which model handles which request)
- Tracks cost per turn and per session
- Manages context windows (token counting, compaction triggers)
- Streams responses using modeltap-specific event types (knowledge hits, cost updates, status)

The existing proxy core (HTTP forwarding, SSE capture, provider parsing, metrics) remains valuable as the mechanism for communicating with providers. The BFF is a new layer on top that manages conversations and serves the harness.

## Solution

Add a BFF (Backend for Frontend) layer to the modeltap server that accepts connections from the harness via a purpose-built protocol, manages conversation state, and uses the existing proxy core to communicate with providers. The BFF is the harness's only interface to model providers — the harness never speaks provider-specific protocols.

## Key Capabilities

### Harness Protocol Endpoint

The server listens for harness connections on:
- Unix domain socket (solo/local profile): `~/.local/share/modeltap/server.sock`
- TLS (team/enterprise profile): configurable address and port

The protocol is JSON-RPC with bidirectional streaming. The server supports concurrent harness connections (one per authenticated user in multi-user deployments).

### Protocol Specification

The harness protocol is a JSON-RPC 2.0 transport with extensions for server-initiated streaming. This section defines the protocol semantics that downstream features (FEAT-0009, FEAT-0010, FEAT-0012) implement against.

**Framing**: JSON-RPC 2.0 messages encoded as newline-delimited JSON (NDJSON) over the transport (Unix socket or TLS). Each message is a complete JSON object terminated by `\n`.

**Correlation**: every request from the harness includes a `id` field (JSON-RPC standard). The server's response or error carries the same `id`. Streaming events for a turn carry a `turn_id` field that correlates all events to the originating `turn.submit` request.

**Stream lifecycle**: a `turn.submit` request initiates a streaming response. The server sends zero or more streaming events (`token.delta`, `tool.call`, `status.update`, `knowledge.hit`, `cost.update`, `compact.notice`) followed by exactly one terminal event (`turn.complete` or `error`). The terminal event closes the stream for that turn. The harness must not send a new `turn.submit` until the previous turn's stream terminates, or it sends an explicit `turn.cancel`.

**Cancellation**: the harness may send `turn.cancel` with the `turn_id` at any time during a streaming response. The server makes a best-effort to stop the provider request and emits `turn.complete` with `cancelled: true`. Provider-side cancellation is not guaranteed (some providers don't support it), but the server stops forwarding events to the harness immediately.

**Tool call round-trips**: when the server emits `tool.call`, streaming pauses. The harness executes the tool (or rejects it) and sends `tool.result` with the matching `tool_call_id`. The server resumes streaming after receiving the result. Multiple `tool.call` events may be emitted in sequence within a single turn, each requiring a `tool.result` before the stream continues.

**Capability and tool registration**: on connection establishment (after auth, see FEAT-0010), the harness sends a `capabilities.register` message declaring available tools, supported protocol version, harness metadata (version, platform), and the project context (see Project Context below).

The server uses the tool catalog when assembling the model prompt — only tools the harness has registered are included in the model's tool definitions. When MCP servers connect or disconnect, the harness sends `capabilities.update` to add or remove tools. The server can also send `capabilities.request` to ask the harness to re-register (e.g., after reconnection).

**Tool catalog schema**: each tool in `capabilities.register` and `capabilities.update` uses this canonical schema:

```json
{
  "name": "Read",
  "namespace": "builtin",
  "description": "Read file contents. Supports text, PDF, DOCX, images, and spreadsheets...",
  "input_schema": {
    "type": "object",
    "properties": {
      "path": { "type": "string", "description": "File path relative to project root" },
      "offset": { "type": "integer", "description": "Start line (optional)" },
      "limit": { "type": "integer", "description": "Max lines (optional)" }
    },
    "required": ["path"]
  },
  "output_envelope": "text",
  "risk_level": "read_only",
  "capabilities_required": []
}
```

| Field | Description |
|-------|-------------|
| `name` | Tool name as the model sees it |
| `namespace` | `builtin` for core tools, `mcp:<server_name>` for MCP tools (e.g., `mcp:github`) |
| `description` | Full behavioral description including when to use and when NOT to (system prompt Layer 2) |
| `input_schema` | JSON Schema for the tool's input parameters |
| `output_envelope` | `text`, `json`, `binary`, or `image` — how the result is formatted |
| `risk_level` | Static metadata: `read_only`, `write`, `execute`, `destructive`. This is tool-inherent risk, not dynamic permission state. |
| `capabilities_required` | Model capabilities needed (e.g., `["vision"]` for image output). The server omits tools from the model prompt when the active model lacks required capabilities. |

**Risk level vs. permission state**: `risk_level` is static metadata about the tool's inherent danger (Read is always `read_only`, Bash is always `execute`). Permission state (default/accept-edits/autonomous, per-tool approval history) is harness-local and not sent to the server. The server uses `risk_level` for prompt assembly (e.g., instructing the model to be cautious with `destructive` tools). The harness uses its local permission state to decide whether to execute or prompt.

**Unified Read dispatch**: the harness registers one `Read` tool. The model calls `Read` with a file path. The harness detects the file type and applies the appropriate extraction (text, PDF, DOCX, image base64, spreadsheet). The server sees `Read` in the tool catalog with `capabilities_required: []` — but if an image is read and the current model lacks `vision`, the server can note this in the response context.

**Tool call and result payloads**:

`tool.call` (server → harness):
```json
{
  "tool_call_id": "tc_001",
  "tool": "Edit",
  "namespace": "builtin",
  "input": { "path": "auth.go", "old_string": "...", "new_string": "..." }
}
```

`tool.result` (harness → server):
```json
{
  "tool_call_id": "tc_001",
  "status": "success",
  "output": "Edit applied: 1 replacement in auth.go",
  "output_type": "text"
}
```

Or rejection:
```json
{
  "tool_call_id": "tc_001",
  "status": "rejected",
  "reason": "user_denied",
  "output": "User denied Edit on auth.go"
}
```

Or error:
```json
{
  "tool_call_id": "tc_001",
  "status": "error",
  "error": "file_not_found",
  "output": "auth.go does not exist"
}
```

**Project context**: the harness transmits the project root on connection and session start:

```json
{
  "project": {
    "root": "/Users/alice/Projects/modeltap",
    "config_file": ".modeltap.yaml",
    "config_content": "..."
  }
}
```

The server uses the project root for: session scoping (sessions are per-project), project instruction loading (Layer 4 system prompt — the server receives the config content from the harness rather than reading the filesystem directly, since the server may be remote), path normalization in session details, and knowledge layer project scoping. File paths in tool calls and results are relative to the project root.

**Protocol versioning**: the harness and server exchange protocol versions during `capabilities.register`. The server declares its supported version range. If the harness's version is outside the range, the connection is rejected with a version-mismatch error. Within a compatible range, the server uses the highest mutually supported version.

**Auth handshake boundary**: auth negotiation (FEAT-0010) occurs before `capabilities.register`. The connection is not considered established until both auth and capability registration complete. See FEAT-0010 for the auth handshake details.

### Protocol Messages

**Harness → Server:**

| Message | Description |
|---------|-------------|
| `capabilities.register` | Declare tools, protocol version, and harness metadata |
| `capabilities.update` | Add or remove tools (e.g., MCP server connected/disconnected) |
| `turn.submit` | User message with optional file attachments and tool results |
| `content.transform` | Pre-turn content transformation (e.g., summarize large paste). Returns transformed content. Captured separately from conversation turns. |
| `turn.cancel` | Cancel the current streaming turn |
| `tool.result` | Result of a tool execution (success, error, or rejected) |
| `session.resume` | Resume an existing session by ID |
| `session.list` | List available sessions for this user/project (returns summaries) |
| `session.details` | Get full session timeline, pinned items, files touched |
| `session.compact` | Request interactive context compaction (server returns plan) |
| `compact.apply` | Apply compaction plan with user's per-category choices |
| `session.clear` | Clear live context (retain in storage) |
| `session.fork` | Branch session into independent continuation |
| `model.switch` | Set session model override (or clear to return to routing policy) |
| `model.list` | List available models with routing roles and capabilities |
| `context.list` | List files, knowledge injections, and context budget |

**Server → Harness (streaming):**

| Event | Description |
|-------|-------------|
| `model.selected` | Which model was chosen for this turn and why (routing reason) |
| `token.delta` | Incremental text from model response (`turn_id` correlated, `branch_id` for multi-model) |
| `branch.started` | Multi-model: a review branch has started (`branch_id`, model, provider) |
| `branch.complete` | Multi-model: a branch finished (`branch_id`, final usage) |
| `branch.error` | Multi-model: a branch failed (`branch_id`, error, diagnostic code) |
| `tool.call` | Model requests a tool execution (`tool_call_id` for correlation) |
| `status.update` | Status message ("routing to claude-opus-4-6...") |
| `knowledge.hit` | Knowledge context was injected (summary, relevance score) |
| `cost.update` | Running token count and cost for this turn |
| `compact.plan` | Compaction analysis with per-category breakdown and suggested actions |
| `compact.suggest` | Context pressure advisory (threshold reached, suggest /compact) |
| `compact.notice` | Auto-compaction applied (what was compressed, what was retained) |
| `turn.complete` | Turn finished (final usage, model, latency, total cost, cancelled flag) |
| `error` | Server-side error (provider failure, budget exceeded, auth failure) |

**Server → Harness (non-streaming):**

| Message | Description |
|---------|-------------|
| `capabilities.request` | Ask harness to re-register capabilities |
| `connection.ping` | Heartbeat from harness (server replies with `connection.pong`) |
| `connection.pong` | Heartbeat reply from server |
| `connection.health` | Request machine-readable health status from server |
| `connection.ready` | Request readiness check (auth, storage, providers, routing) |
| `session.sync` | After reconnect: request authoritative state of active turn |

### Connection Lifecycle and Self-Recovery

The harness treats the BFF connection as a managed lifecycle, not a raw socket.

**Connection states:**

```
discovering → starting → connecting → authenticating → registering → ready → degraded → reconnecting → failed
```

| State | Description |
|-------|-------------|
| `discovering` | Harness checks if server is reachable (socket exists, port responds) |
| `starting` | Local profile only: harness auto-starts the local service if installed but stopped |
| `connecting` | TCP/TLS handshake or Unix socket connection in progress |
| `authenticating` | Auth handshake (FEAT-0010) in progress |
| `registering` | Capability registration (`capabilities.register`) in progress |
| `ready` | Fully connected, authenticated, registered. Turns can be submitted. |
| `degraded` | Connected but a dependency is unhealthy (provider down, storage unready, etc.) |
| `reconnecting` | Connection lost, attempting automatic recovery |
| `failed` | Automatic recovery exhausted. User action required. |

**Transitions:**
- `ready → degraded`: a `connection.health` check reports an unhealthy dependency. The harness can still submit turns, but the server may return errors for routes that depend on the degraded component.
- `ready → reconnecting`: heartbeat missed or connection dropped. Harness begins recovery.
- `reconnecting → connecting`: retry attempt. Exponential backoff: 1s, 2s, 4s, 8s, max 30s. Max retries: 10 (configurable).
- `reconnecting → failed`: max retries exhausted or terminal error (version mismatch, auth permanently revoked).
- Any state → `failed`: terminal error detected (incompatible protocol version, TLS cert rejected, socket permission denied).

**Local service auto-start (solo profile):**

When the harness is configured for a local socket and the server is not running:

1. Check if the socket file exists.
2. If socket exists: test if a process is listening. If not → stale socket.
3. Stale socket: check if any process owns the socket file (via `lsof` or `/proc`). If no owner → safe to remove. Remove and proceed to step 4. If owner exists but unresponsive → print diagnostic, do not auto-remove.
4. If service is installed (FEAT-0004): start it via `modeltap service start`. Wait for readiness (poll `connection.ready`, timeout 10s).
5. If service is not installed: start a server subprocess (`modeltap serve` as a child process). The subprocess inherits the harness's config.
6. If startup fails → `failed` state with diagnostic.

**Heartbeat:**
- Harness sends `connection.ping` every 15 seconds (configurable).
- Server replies with `connection.pong` within 5 seconds.
- After 3 consecutive missed pongs, harness transitions to `degraded`.
- After 5 consecutive missed pongs, harness transitions to `reconnecting`.
- Server-side: after heartbeat timeout (missed pings for 30 seconds) plus grace period (10 seconds), the server releases the active session lock.

**Readiness and health:**

`connection.health` returns machine-readable status of all server dependencies:

```json
{
  "server_version": "0.2.0",
  "protocol_version": "1",
  "uptime_seconds": 3600,
  "auth": { "status": "ready", "method": "oidc" },
  "storage": { "status": "ready", "path": "/var/lib/modeltap/data.db" },
  "capabilities": { "status": "ready", "tools_registered": 14 },
  "providers": {
    "anthropic": { "status": "ready" },
    "openai": { "status": "ready" },
    "ollama-local": { "status": "ready", "models": 3 },
    "ollama-gpu": { "status": "unavailable", "error": "connection refused" }
  },
  "routing": { "status": "degraded", "reason": "ollama-gpu unavailable, fallback active" },
  "active_session": { "id": "sess_a8f3c2", "owner": "alice" }
}
```

`connection.ready` is a simplified boolean: is the server ready to accept a `turn.submit` right now? Returns `true` only if auth, storage, capability registration, and at least one routing target are healthy.

### In-Flight Turn Recovery

When the connection drops during an active turn (streaming response, pending tool call, or multi-model parallel review), the harness must be able to recover cleanly on reconnect.

**Idempotency rules:**
- `turn.submit` carries a client-generated `turn_id` (UUID) and `sequence` number. If the server receives a `turn.submit` with a `turn_id` it has already processed, it does not re-send to the provider. It returns the existing turn state.
- `tool.result` is idempotent by `tool_call_id`. Duplicate submissions are accepted and ignored.

**`session.sync` after reconnect:**

After re-establishing the connection (auth + capabilities), the harness sends `session.sync` for its active session. The server returns the authoritative state:

```json
{
  "session_id": "sess_a8f3c2",
  "active_turn": {
    "turn_id": "turn_x9f2",
    "status": "pending_tool_result",
    "pending_tool_calls": [
      { "tool_call_id": "tc_001", "tool": "Edit", "status": "awaiting_result" }
    ],
    "completed_tokens": 1247,
    "token_replay_available": false,
    "summary": "Model requested Edit on auth.go, awaiting tool result"
  },
  "multi_model": null
}
```

Or for a multi-model turn:

```json
{
  "active_turn": {
    "turn_id": "turn_y3a1",
    "status": "streaming",
    "multi_model": {
      "reviewers": [
        { "model": "gpt-5.4", "status": "complete", "tokens": 892 },
        { "model": "claude-opus-4-6", "status": "streaming", "tokens": 341 }
      ]
    },
    "token_replay_available": false,
    "summary": "gpt-5.4 complete, claude-opus-4-6 still streaming"
  }
}
```

**Token replay**: the server does not guarantee token replay after reconnect (buffering all tokens for replay is expensive). Instead, `session.sync` returns a summary of what was produced before the disconnect. The harness displays this summary and continues receiving new tokens from the resumed stream. If the turn completed while disconnected, `session.sync` returns the full completed turn.

### Diagnostic Taxonomy

Every connection failure maps to a stable diagnostic code with structured information. The harness renders these as actionable messages.

| Code | Category | Cause | Auto-Repair | Suggested Command |
|------|----------|-------|-------------|-------------------|
| `MT-CONN-001` | `service_not_running` | Local service not started | Auto-start attempted | `modeltap service install` |
| `MT-CONN-002` | `stale_socket` | Socket exists, no listener | Remove if safe | `rm ~/.local/share/modeltap/server.sock` |
| `MT-CONN-003` | `socket_permission` | Socket exists, wrong owner/mode | None | `ls -la <path>`, check user |
| `MT-CONN-004` | `version_mismatch` | Server/harness protocol incompatible | None (terminal) | `modeltap upgrade` or update server |
| `MT-CONN-005` | `tls_untrusted` | Server TLS cert not trusted | None (terminal) | Check `server.tls.cert` config |
| `MT-CONN-006` | `auth_expired` | OIDC token expired, refresh failed | Re-auth attempted | `modeltap auth login` |
| `MT-CONN-007` | `storage_unready` | SQLite database inaccessible | None | Check `storage.path`, permissions |
| `MT-CONN-008` | `session_locked` | Another harness owns the session | None | `modeltap session unlock <id>` |
| `MT-CONN-009` | `provider_unavailable` | Provider endpoint down | Routing fallback | Check provider config, network |
| `MT-CONN-010` | `capability_registration_failed` | Tool registration rejected | Re-register attempted | Check MCP server configs |
| `MT-CONN-011` | `model_unavailable` | Requested model not in registry | Routing fallback | `/models` to see available |
| `MT-CONN-012` | `heartbeat_timeout` | Server stopped responding | Reconnect attempted | `modeltap server status` |

Each diagnostic is emitted as a structured `error` event with fields: `code`, `category`, `cause`, `auto_repair_attempted` (bool), `repair_result`, `suggested_command`, `path_or_endpoint`.

The harness renders these as:

```
⚠ MT-CONN-002 stale_socket: found ~/.local/share/modeltap/server.sock
  but no server is listening. Removed stale socket and restarted service.
```

Or when auto-repair fails:

```
✗ MT-CONN-003 socket_permission: ~/.local/share/modeltap/server.sock is
  owned by root. Cannot connect.
  → Check socket permissions: ls -la ~/.local/share/modeltap/server.sock
```

### Conversation State Management

The server owns a canonical conversation format that is provider-neutral:

- Messages are stored as `{role, content, tool_calls[], tool_results[], attachments[], metadata}`
- The server persists conversations to SQLite (ADR-0002) and restores them on session resume
- Conversation history survives model switches — switching from Claude to Llama mid-session preserves the full history
- Each turn records: model used, provider, token counts (in/out), cost, latency, timestamp

### Provider Message Format Translation

When sending a conversation to a provider, the server translates from canonical format to provider-specific format:

- **Anthropic**: `messages` array with `role` and `content` blocks, `tool_use`/`tool_result` block types
- **OpenAI Chat Completions**: `messages` array with `role` and `content` string/array, `function_call`/`tool_calls`
- **OpenAI Responses API**: `input` with typed event model
- **Ollama**: `messages` array with `role` and `content` string

Translation handles:
- Message structure differences
- Tool call/result format differences
- System prompt conventions (system role vs. system parameter)
- Context window truncation (drop oldest turns when exceeding model's window, preserving system prompt and recent context)
- Feature availability (graceful handling when a model doesn't support tool use)

The provider adapter interface (ADR-0006) is extended to support both parsing (inbound response) and formatting (outbound request) for full message histories. This is a material change to the accepted ADR-0006 interface — it must be formalized as an amendment (`ADR-0006-amendment-001`) or superseding ADR before this feature is accepted. The amendment adds `FormatMessages(canonical []Message, windowSize int) (providerPayload, error)` to the provider interface alongside the existing parse/detect methods.

### System Prompt Engine

The system prompt is the primary driver of session quality. It is not a single static string — it is assembled per-turn from seven layers, each serving a different purpose. The model sees one prompt; the server builds it from multiple sources.

**Prompt Assembly (top to bottom, all concatenated):**

**Layer 1 — Core behavioral prompt (always present, ships with product):**

The methodology specification that shapes every interaction. This is the engineering investment that makes sessions feel like working with a competent colleague. It covers:

- *Work methodology*: read files before editing. Try the simplest approach first. If an approach fails, diagnose why before switching tactics. Don't retry blindly, but don't abandon a viable approach after one failure.
- *Scope discipline*: don't add features, refactor code, or make improvements beyond what was asked. Don't add docstrings, comments, or type annotations to code you didn't change. Don't create helpers or abstractions for one-time operations.
- *Output discipline*: keep text output brief and direct. Lead with the answer, not the reasoning. If you can say it in one sentence, don't use three. No filler words or preamble.
- *Safety*: don't introduce security vulnerabilities. Confirm before destructive operations. Don't push to remote repositories without explicit instruction.
- *Error recovery*: when a tool call fails, read the error, check assumptions, try a focused fix. Escalate to the user only when genuinely stuck after investigation.

This layer is hundreds of lines, not a paragraph. It is the most carefully engineered part of the product and should be iteratively refined based on real session quality. It ships as a bundled asset, not user-editable configuration.

**Layer 2 — Tool-use instructions (always present, ships with product):**

Per-tool behavioral guidance injected alongside the tool definitions. Each tool's description includes not just what it does, but when to use it and when NOT to:

- *Read*: "always read a file before editing it"
- *Edit*: "use exact string matching — provide enough surrounding context to match uniquely"
- *Bash*: "do NOT use Bash to run cat, head, tail, sed, awk — use the dedicated Read/Edit tools instead"
- *Write*: "prefer editing existing files over creating new ones"
- *Git*: "prefer new commits over amending. Never force push without explicit instruction"

These instructions are injected into the tool schema descriptions that the provider receives, so they are model-visible regardless of which provider or model is selected.

**Layer 3 — Domain prompt (per-domain, configurable):**

Behavioral specification for the workflow domain. Configurable per server or per project via `system_prompt.domain` in config.

For the coding domain (built-in default):
- Go code: gofmt, go vet, effective Go idioms
- Tests: table-driven, in _test.go files alongside production code
- Commits: conventional format, meaningful messages
- Reviews: check for OWASP top 10, injection, data leakage

For other domains (shipped as domain packages or user-authored):
- Legal: never state a conclusion without citing authority, distinguish binding vs. persuasive authority, preserve privilege
- Finance: verify every number against source data, distinguish GAAP vs. IFRS, flag materiality thresholds
- Custom: user provides a markdown file path in config

```yaml
system_prompt:
  domain: coding                    # built-in: coding, legal, finance
  # or custom:
  domain: prompts/my-domain.md      # path to custom domain prompt
```

**Layer 4 — Project instructions (per-project):**

Loaded from `.modeltap.yaml` or `MODELTAP.md` in the project root. Specifies project-specific conventions, architecture decisions, team preferences, and constraints. Equivalent to Claude Code's `CLAUDE.md` but server-managed.

The server reads this file from the harness's project root (communicated during session start) and includes it verbatim. It is re-read on every turn so edits take effect immediately.

**Layer 5 — Mode prompt (per-mode, see Execution Mode Support):**

Different system prompt fragments for plan mode vs. build mode:

- *Plan mode*: "Analyze the task and propose a step-by-step plan. Read files to understand context, but do not make changes. Present the plan for user approval with clear steps, affected files, and rationale."
- *Build mode*: "Execute the task directly. Read relevant files, make changes, run tests, and iterate until done. Report what you changed and why."
- *Auto mode*: same as build mode, plus "proceed without asking for confirmation on standard operations."

The mode is communicated to the server by the harness (see `mode` field on `turn.submit`). The server selects the appropriate mode prompt fragment.

**Layer 6 — Knowledge injections (per-turn, when FEAT-0011 is active):**

Relevant prior context from the knowledge layer, assembled fresh each turn. Formatted as a clearly labeled block:

```
<prior-context source="knowledge-layer">
Decision (2026-04-10, this project): Use JWT for API auth. Rationale: stateless, no server session storage needed.
Decision (2026-04-12, this project): Token expiry: 15min access, 7d refresh.
</prior-context>
```

The model sees this as contextual reference, not as instructions. The block is clearly delimited so the model can weigh it appropriately.

**Layer 7 — Session state (per-turn):**

Current session context assembled by the server:
- Pinned items (user-designated always-carry-forward state)
- Active plan (if in plan mode and a plan has been approved, the plan steps are included)
- Compaction summaries (compressed turns replaced by their summaries)
- Files currently in context (names and sizes, not contents — contents are in the conversation history)
- Active model override (if any)

**Assembly and delivery:**

The server concatenates all layers into a single system prompt string on every turn. The total is included in the provider request as the system message (Anthropic `system` parameter, OpenAI `system` role message, etc.). The prompt is re-assembled on every turn — never cached — so that knowledge injections, session state, mode changes, and project instruction edits take effect immediately.

The assembled prompt's token count is tracked and included in the context budget. If the system prompt grows too large (e.g., many knowledge injections), the server trims Layer 6 (knowledge injections) first, then Layer 7 (session state summaries), preserving Layers 1-5 which are essential for behavior quality.

### Execution Mode Support

The server supports three execution modes communicated by the harness via a `mode` field on `turn.submit`:

- **`plan`**: the server injects the plan-mode prompt (Layer 5). The model is instructed to analyze and propose. The server forwards all tool calls to the harness as normal — the harness decides which to execute (reads) and which to collect into the plan display (writes). Read-only tool calls (`Read`, `Glob`, `Grep`, `Git status/log/diff`) are expected and normal in plan mode — the model needs context to plan well.

- **`build`**: the server injects the build-mode prompt (Layer 5). The model is instructed to execute directly. All tool calls flow to the harness for normal permission-based execution.

- **`auto`**: same as build, with an additional prompt fragment encouraging the model to proceed without requesting confirmation for standard operations.

When the harness approves a plan and switches to build mode for execution, the harness sends a `turn.submit` with `mode: "build"` and includes the approved plan text in the user message. The server injects the build-mode prompt and includes the plan in Layer 7 (session state) so the model has it as a reference during execution.

The server does not enforce mode boundaries — it provides the appropriate prompt. Mode enforcement (intercepting write tool calls in plan mode) is the harness's responsibility per the execution boundary principle (see FEAT-0009).

### Model Configuration: Three-Layer Architecture

Model configuration separates three concerns: where services run (provider endpoints), what models are available (model registry), and how they're used (routing policy).

#### Layer 1: Provider Endpoints

Provider endpoints define where model services run and how to authenticate:

```yaml
providers:
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    # Cloud provider — models are well-known, auto-registered from built-in catalog

  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}

  ollama-local:
    type: ollama
    host: http://localhost:11434
    discover: true                    # poll /api/tags for available models

  ollama-gpu:
    type: ollama
    host: http://gpu-server.internal:11434
    discover: true

  mlx-local:
    type: mlx
    host: http://localhost:8080
    discover: true
```

Multiple endpoints of the same type are supported (e.g., two Ollama instances). Each endpoint has a unique name used for model-to-endpoint mapping.

#### Layer 2: Model Registry

The model registry maps model names to provider endpoints with metadata. It is populated from two sources:

**Auto-discovery** (primary):
- **Ollama/MLX**: the server polls each endpoint's model list (`/api/tags` for Ollama) on startup and periodically (default: every 60 seconds). Models that appear are auto-registered with the endpoint they were found on.
- **Cloud providers** (Anthropic, OpenAI): the server has a built-in catalog of available models per provider type. No polling needed. The server validates the API key on startup.

**Manual overrides** (optional): the user can pin models to specific endpoints, override metadata, or add models that auto-discovery doesn't find:

```yaml
models:
  # Override auto-discovered metadata
  claude-opus-4-6:
    provider: anthropic
    description: "Strongest reasoning and code generation"

  # Pin a local model to a specific endpoint
  llama-3.1-70b:
    provider: ollama-gpu              # always use the GPU server for this model
    description: "Strong local model, security review"

  # Add a model that auto-discovery misses
  custom-fine-tune:
    provider: ollama-local
    context_window: 32000
    capabilities: [tool_use]
    cost: { input: 0, output: 0 }
    description: "Project-specific fine-tune"
```

Auto-discovered models do not need manual entries unless the user wants to override defaults. The registry merges auto-discovered and manual entries, with manual taking precedence.

**Registry fields per model** (auto-populated where possible):

| Field | Source | Description |
|-------|--------|-------------|
| `provider` | Discovery or manual | Which endpoint this model runs on |
| `context_window` | Built-in catalog or discovery | Max context tokens |
| `capabilities` | Built-in catalog or discovery | `[tool_use, vision, long_context, embedding]` |
| `cost` | Built-in catalog or manual | `{ input: $/1K, output: $/1K }` |
| `description` | Manual or generated | Human-readable purpose |
| `status` | Runtime | `available`, `unavailable`, `error` |

**Model availability changes**: if a local model disappears (Ollama restarted, model unloaded), the server marks it `unavailable` in the registry. If the routing policy references it, the server falls back up the routing tree. The harness receives a `status.update`: "llama-3.1-70b unavailable on ollama-gpu, falling back to claude-opus-4-6."

**Duplicate models across endpoints**: if `llama-3.1-8b` appears on both `ollama-local` and `ollama-gpu`, the server registers both. The user can pin via the manual config, or the server selects the first available endpoint (configurable: prefer local, prefer fastest, round-robin).

#### Layer 3: Routing Policy

The routing policy maps workflow roles to models from the registry. Role names are hierarchical — organized by domain and activity:

```yaml
routing:
  default: claude-sonnet-4-6         # global fallback
  cheap: llama-3.1-8b
  embedding: nomic-embed-text

  backend:
    default: claude-opus-4-6         # all backend work defaults to opus
    design: claude-sonnet-4-6        # except design docs
    code: claude-opus-4-6
    review: gpt-5.4
    review_security: llama-3.1-70b   # local, no data leaves machine

  frontend:
    default: claude-opus-4-6
    design: claude-opus-4-6
    code: claude-opus-4-6
    review: gpt-5.4

  infrastructure:
    default: claude-sonnet-4-6
    code: claude-opus-4-6
    review: [gpt-5.4, claude-opus-4-6]   # multi-reviewer, parallel

  planning:
    default: claude-sonnet-4-6
    review: gpt-5.4
```

**Resolution** — dot-path with fallback up the tree:

| Query | Resolution | Reason |
|-------|-----------|--------|
| `backend.review` | gpt-5.4 | Exact match |
| `backend.testing` | claude-opus-4-6 | No match → `backend.default` |
| `backend` | claude-opus-4-6 | Resolves to `backend.default` |
| `infrastructure.review` | [gpt-5.4, claude-opus-4-6] | Multi-reviewer |
| `something.unknown` | claude-sonnet-4-6 | No match at any level → root `default` |

Resolution order: `category.role` → `category.default` → `default`

**Single-model roles** (string value): the server routes to that model.

**Multi-model roles** (array value): the server runs all listed models in parallel as background threads. Each model's response is returned to the harness as a separate labeled result (see FEAT-0009 for display). Reviewers work independently and don't see each other's output. Sequential review (where reviewer 2 sees reviewer 1's findings) is an orchestration concern handled by FEAT-0013 (Agent Teams).

**How roles are selected**: the BFF classifies the current turn's intent based on the conversation context, system prompt, and any explicit user request. The user can request a specific role via skill invocations (FEAT-0012) or direct commands. The `default` role (at any tree level) is the fallback when no specific role matches.

Routing policy is extensible — future work (EXP-0007) may add complexity-based routing and cost-aware automatic selection. This feature implements the static hierarchical policy engine with parallel multi-model support.

### Model Transparency

The user must always know which model is being used and why. The server ensures this through:

**`model.selected` event**: emitted at the start of every turn before `token.delta` begins streaming. Contains:
- `model`: the model name (e.g., `claude-opus-4-6`), or an array for multi-model roles
- `provider`: the provider (e.g., `anthropic`), or an array
- `reason`: why this model was selected — `"routing_policy:coding"`, `"user_override"`, `"fallback:default"`, `"routing_policy:code_review[2]"` (multi-model with count), etc.

**Multi-model branch streaming**: for multi-model roles, the server manages parallel provider calls as branches. Each branch has a lifecycle:

| Event | Description |
|-------|-------------|
| `branch.started` | A branch has started (`branch_id`, `model`, `provider`) |
| `token.delta` | Incremental text, tagged with `branch_id` |
| `cost.update` | Per-branch running cost, tagged with `branch_id` |
| `branch.complete` | Branch finished successfully (`branch_id`, final usage) |
| `branch.error` | Branch failed (`branch_id`, error detail, diagnostic code) |
| `turn.complete` | Aggregate turn completion (all branches done or errored, total cost) |

Branch lifecycle:
1. Server emits `model.selected` with the full reviewer list
2. Server emits `branch.started` for each branch as it begins
3. `token.delta` and `cost.update` events arrive interleaved, tagged with `branch_id`
4. Each branch terminates with `branch.complete` or `branch.error`
5. After all branches terminate, the server emits aggregate `turn.complete`

**Branch cancellation**: `turn.cancel` cancels all branches. Individual branch cancellation is not supported — if one reviewer is stuck, the user cancels the whole turn.

**Branch-aware `session.sync`**: after reconnect, `session.sync` for a multi-model turn returns per-branch state (complete with summary, streaming with token count, failed with error, or pending). The harness renders completed branches immediately and resumes streaming branches.

This ensures the harness can display the model(s) before any response text appears, render results progressively as branches complete, and recover cleanly after disconnection.

**Model override**: the user can override routing for the session via `model.switch`:
- `/model claude-opus-4-6` — all subsequent turns use this model regardless of routing policy
- The override is sticky for the session until explicitly cleared
- The server records the override in session state; it persists across harness reconnection

**Clearing an override**: the user can return to routing-policy defaults:
- `/model auto` — clears the override, routing policy resumes
- The `model.selected` event shows `reason: "routing_policy:..."` again after clearing

**Model listing**: the server responds to `model.list` with all available models, their routing roles, capabilities, and cost:

```json
{
  "models": [
    {
      "name": "claude-opus-4-6",
      "provider": "anthropic",
      "roles": ["coding", "default"],
      "capabilities": ["tool_use", "vision", "long_context"],
      "context_window": 200000,
      "cost_per_1k_input": 0.015,
      "cost_per_1k_output": 0.075,
      "description": "Strongest reasoning and code generation"
    },
    {
      "name": "llama-3.1-8b",
      "provider": "ollama",
      "roles": ["cheap"],
      "capabilities": ["tool_use"],
      "context_window": 128000,
      "cost_per_1k_input": 0.0,
      "cost_per_1k_output": 0.0,
      "description": "Fast local model, good for simple tasks and explanations"
    }
  ],
  "current_override": null,
  "routing_policy": {
    "default": "claude-sonnet-4-6",
    "coding": "claude-opus-4-6",
    "review": "gpt-4",
    "cheap": "llama-3.1-8b"
  }
}
```

The `roles` field shows which routing categories this model is assigned to. The `description` field provides a human-readable recommendation. In enterprise deployments (FEAT-0010), models the user's role cannot access are either omitted or marked `"access": "denied"`.

### Session List and Details

The server supports rich session exploration for resume decisions.

**`session.list` response** — returns recent sessions with enough context to choose:

```json
{
  "sessions": [
    {
      "id": "sess_a8f3c2",
      "project": "~/Projects/modeltap",
      "status": "active",
      "summary": "rate limiting implementation",
      "last_active": "2026-04-15T10:23:00Z",
      "context_pct": 0.47,
      "total_cost": 1.23,
      "turn_count": 24,
      "model": "claude-opus-4-6",
      "model_override": null,
      "last_turn_summary": "backend agent completed, reviewer found 2 issues",
      "files_touched": ["ratelimit.go", "router.go", "ratelimit_test.go"],
      "pinned_count": 2
    }
  ]
}
```

**`session.details` response** — full session timeline for inspection before resume:

```json
{
  "id": "sess_a8f3c2",
  "summary": "rate limiting implementation",
  "created_at": "2026-04-14T14:22:00Z",
  "last_active": "2026-04-15T10:23:00Z",
  "model": "claude-opus-4-6",
  "model_override": null,
  "context_pct": 0.47,
  "total_cost": 1.23,
  "turns": [
    { "sequence": 1, "summary": "Read auth.go, config.go", "compacted": false, "model": "claude-opus-4-6", "cost": 0.03 },
    { "sequence": 2, "summary": "Planned JWT migration (6 steps)", "compacted": false, "model": "claude-opus-4-6", "cost": 0.05 },
    { "sequence": 3, "summary": "User approved plan", "compacted": false, "model": "claude-opus-4-6", "cost": 0.01 },
    { "sequence": 4, "summary": "researched JWT libraries", "compacted": true, "original_turns": [4, 5, 6, 7], "cost": 0.12 },
    { "sequence": 8, "summary": "Decision: use golang-jwt/jwt/v5", "compacted": false, "model": "claude-opus-4-6", "cost": 0.04 }
  ],
  "pinned_items": [
    "Use golang-jwt/jwt/v5, not dgrijalva",
    "Token expiry: 15min access, 7d refresh"
  ],
  "files_touched": ["auth.go", "config.go"],
  "files_modified": [],
  "server_events": [
    { "type": "auto_compact", "at": "2026-04-15T03:00:00Z", "freed_tokens": 12800, "detail": "debugging session summarized, stale files dropped" }
  ]
}
```

**Session summaries** are auto-generated by the server: after the first 2-3 turns, the server prompts a cheap model to produce a short session title (e.g., "rate limiting implementation"). The summary updates periodically if the session topic shifts.

**Server events** track what happened to the session outside of user interaction (auto-compaction, server restarts) so the harness can inform the user on resume.

### Streaming Relay

The server receives SSE streams from providers and translates them into harness protocol events:

1. Provider SSE chunks arrive at the server
2. Server parses chunks using the provider adapter (ADR-0006)
3. Server emits `token.delta` events to the harness immediately (zero added latency)
4. Server accumulates the full response in the background
5. When the stream completes, server emits `turn.complete` with final metadata
6. Server asynchronously: logs the full request/response, queues for embedding, updates metrics

### Cost Tracking

Per-turn and per-session cost tracking:

- Token counts (input/output) from provider response headers or stream metadata
- Cost computed from the server's pricing table (model → cost-per-token)
- Running session total maintained across turns
- `cost.update` events streamed to the harness during each turn
- Cost data stored in aggregation tables (ADR-0007) for metrics queries

### Context Window Management

The server tracks context window usage and provides context analysis and compaction.

**Token counting**: the server maintains a running token count for the full conversation (system prompt + history + knowledge injections + current turn) and reports it to the harness via `cost.update` events.

**Pressure warnings**: at a configurable threshold (default: 78%), the server emits a `compact.suggest` event to the harness advising the user to compact.

**Context categorization and compaction plan**: when the harness sends `session.compact` (interactive) or when auto-compact triggers (at 92% threshold), the server performs context analysis:

1. **Categorize** all context into semantic categories:
   - Architecture/design decisions (reference count, recency)
   - Debugging sessions (resolved vs. active)
   - Test iterations (passing vs. failing, retry cycles)
   - File contents (still referenced vs. stale)
   - Planning discussion (approved vs. pending items)
   - Tool call metadata (reproducible from capture, low value)
   - Knowledge injections (still relevant vs. stale)

2. **Score** each category by value signals:
   - Reference count: how often this content was referenced in later turns
   - Recency: when was it last relevant
   - Resolution status: is the issue resolved, the test passing, the plan approved
   - Pin status: user-pinned items are never suggested for removal

3. **Generate a compaction plan**: for each category, the server proposes an action:
   - **keep**: high-value, retain verbatim
   - **summarize**: moderate value, compress to a summary (server generates the summary text)
   - **drop**: low value, remove from live context
   - **pin**: user-designated always-carry-forward

4. **Return the plan to the harness** via `compact.plan` response, including:
   - Per-category breakdown: name, token count, value score, suggested action, summary preview (for summarize actions)
   - Per-file breakdown within the file-contents category
   - Estimated savings (tokens freed, percentage reduction)
   - The harness decides how to present this to the user (see FEAT-0009)

5. **Apply the plan**: the harness sends `compact.apply` with the user's choices (which may differ from the suggestions). The server executes:
   - Summarized categories: replace verbose content with the summary in the live conversation
   - Dropped categories: remove from the live conversation
   - Pinned categories: mark as always-carry-forward
   - Full originals are always retained in storage and the knowledge layer

**Auto-compaction**: at 92% context usage, the server runs the same analysis but applies its default recommendations automatically (configurable as `context.auto_compact_policy`). The harness receives a `compact.notice` event showing what was compacted. The user can always `/compact` afterward to review and adjust.

**Compaction is lossless**: nothing is deleted from storage. Compacted content moves from the live context window to searchable long-term memory. The knowledge layer (FEAT-0011) can re-retrieve compacted content if it becomes relevant in a later turn.

### Session Persistence

Sessions are persisted to SQLite and survive:

- Harness disconnection and reconnection
- Server restarts
- Model switches within a session

Session data includes: conversation history (canonical format), active model, routing overrides, pinned items, compaction state, cost totals, creation/update timestamps, and project association.

### Session and Turn Storage Model

This feature introduces two new table families alongside the existing request log and aggregation tables (ADR-0002, ADR-0007):

**`sessions` table:**

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT PK | Session UUID |
| `user_id` | TEXT | Owner (for FEAT-0010 isolation) |
| `project` | TEXT | Project path or identifier |
| `active_model` | TEXT | Currently selected model |
| `routing_overrides` | JSON | Per-session routing overrides |
| `pinned_items` | JSON | Pinned context items |
| `total_cost` | REAL | Running session cost |
| `total_input_tokens` | INTEGER | Running input token total |
| `total_output_tokens` | INTEGER | Running output token total |
| `created_at` | TIMESTAMP | Session creation time |
| `updated_at` | TIMESTAMP | Last activity time |
| `status` | TEXT | active, suspended, completed |

**`turns` table:**

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT PK | Turn UUID |
| `session_id` | TEXT FK | Parent session |
| `sequence` | INTEGER | Turn order within session |
| `role` | TEXT | user, assistant |
| `content` | TEXT | Canonical message content (JSON) |
| `model` | TEXT | Model used for this turn |
| `provider` | TEXT | Provider used |
| `input_tokens` | INTEGER | Input tokens for this turn |
| `output_tokens` | INTEGER | Output tokens for this turn |
| `cost` | REAL | Cost for this turn |
| `latency_ms` | INTEGER | Provider response latency |
| `tool_calls` | JSON | Tool calls made in this turn |
| `compacted` | BOOLEAN | Whether this turn has been compacted |
| `compacted_summary` | TEXT | Summary if compacted |
| `created_at` | TIMESTAMP | Turn timestamp |

**Retention**: session and turn data follows the existing retention policy (ADR-0005). Sessions older than the configured retention period are pruned, along with their turns. The raw request log (existing capture) is the authoritative record — session/turn tables are a structured view on top of it.

**Relationship to ADR-0007 aggregation**: per-turn cost data feeds into the existing hourly/daily aggregation tables via the same rollup mechanism. Sessions add a new aggregation dimension but do not change the rollup architecture.

## CLI Integration

The BFF server is started via existing service management (FEAT-0004) or inline:

```
modeltap serve                    # Start the server (foreground)
modeltap service install          # Install as background service (existing FEAT-0004)
```

New server administration and connectivity commands:

```
modeltap server status            # Machine-readable and human-readable health
                                  # (protocol, auth, storage, providers, routing, sessions)
modeltap server sessions          # List active and recent sessions
modeltap server session <id>      # Show session details (turns, cost, model history)
modeltap session unlock <id>      # Force-release a stuck session lock
modeltap auth login               # Re-authenticate (OIDC re-auth flow)
```

`modeltap server status` output includes: server version, protocol version, uptime, listener state (socket/TLS), auth readiness, storage readiness, provider endpoint status per endpoint, model registry status (available/unavailable counts), routing readiness, active sessions, and any degraded dependencies with diagnostic codes.

## Configuration

New configuration keys:

```yaml
server:
  # Listen address (network)
  address: 0.0.0.0:8443
  tls:
    cert: /path/to/cert.pem
    key: /path/to/key.pem
  # Listen socket (local, overrides address if set)
  socket: ~/.local/share/modeltap/server.sock

# System prompt
system_prompt:
  # Path to domain-specific prompt file, or inline
  domain: coding    # built-in: coding. Custom: path to .md file
  # Project instructions file name (searched in project root)
  project_file: .modeltap.yaml

# Layer 1: Provider endpoints
providers:
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
  openai:
    type: openai
    api_key: ${OPENAI_API_KEY}
  ollama-local:
    type: ollama
    host: http://localhost:11434
    discover: true
  ollama-gpu:
    type: ollama
    host: http://gpu-server.internal:11434
    discover: true

# Layer 2: Model registry (optional — auto-discovery fills most of this)
models:
  llama-3.1-70b:
    provider: ollama-gpu              # pin to GPU server
    description: "Strong local model, security review"

# Layer 3: Routing policy (hierarchical)
routing:
  default: claude-sonnet-4-6
  cheap: llama-3.1-8b
  embedding: nomic-embed-text

  backend:
    default: claude-opus-4-6
    design: claude-sonnet-4-6
    code: claude-opus-4-6
    review: gpt-5.4
    review_security: llama-3.1-70b

  frontend:
    default: claude-opus-4-6
    review: gpt-5.4

  infrastructure:
    default: claude-sonnet-4-6
    review: [gpt-5.4, claude-opus-4-6]  # parallel multi-reviewer

  planning:
    default: claude-sonnet-4-6
    review: gpt-5.4

# Context management
context:
  warning_threshold: 0.78     # emit warning at 78%
  compact_threshold: 0.92     # auto-compact at 92%
  compact_model: llama-3.1-8b # cheap model for summarization (default: routing.cheap)

# Session management
sessions:
  retention: 90d              # how long to keep session data
  max_per_user: 100           # max active sessions per user
```

## Non-Goals

- **Multi-user identity and isolation**: handled by FEAT-0010. This feature implements the server infrastructure; FEAT-0010 adds auth and per-user scoping.
- **Knowledge injection**: handled by FEAT-0011. This feature provides the hook point (system prompt assembly step 3) but does not implement semantic search or embedding.
- **Multi-model orchestration**: future work per EXP-0007. This feature implements static routing policy, not task decomposition.
- **Web frontend support**: future work. The harness protocol is the only client interface in this feature.
- **MCP server for external clients**: unchanged from ADR-0009, out of scope for this feature.

## Success Criteria

### Core

1. A harness client can connect to the server via Unix socket or TLS and authenticate (initially: local socket with peer credentials).
2. The server accepts a conversation turn, routes it to a configured provider, and streams the response back as harness protocol events.
3. Conversation state persists across harness disconnection and reconnection — resuming a session restores the full conversation history.
4. Switching models mid-session preserves conversation context — the server translates the canonical history to the new provider's format.
5. The server assembles system prompts from all seven layers and includes them in every provider request.
6. Cost tracking reports accurate token counts and costs per turn and per session, matching provider billing within 5%.
7. Context window management triggers interactive and auto-compaction at configured thresholds.
8. The existing proxy functionality (capture, metrics, retention) continues to work — the BFF layer does not break the v1 proxy core.
9. The server handles concurrent harness connections (preparation for FEAT-0010 multi-user, even if initially single-user).

### Protocol Methods (harness-facing)

These criteria cover specific protocol methods that FEAT-0009 depends on:

19. `model.list` returns all available models with provider, cost, capabilities, context window, and routing roles. Models on unavailable providers are marked `unavailable`.
20. `model.selected` is emitted before every turn's first `token.delta`, showing model name, provider, and routing reason.
21. `model.switch` with a model name sets a persistent session override. `model.switch` with `auto` clears it. Override persists across reconnection.
22. `session.list` returns recent sessions with summary, context usage, cost, turn count, model, files touched, and server events.
23. `session.details` returns full session timeline including compacted turns, pinned items, files touched/modified, and server events (auto-compaction, restarts).
24. `compact.plan` returns categorized context breakdown with per-category token counts, value scores, suggested actions, and summary previews.
25. `compact.apply` accepts user-modified per-category actions and applies them. The server confirms what was compacted.
26. `capabilities.register` accepts the tool catalog schema (name, namespace, description, input_schema, output_envelope, risk_level, capabilities_required) and project context. The server reflects registered tools in model prompts.
27. `capabilities.update` adds or removes tools dynamically. The server updates model prompts on the next turn.
28. `content.transform` accepts raw content and a transform type (e.g., "summarize"), routes to the cheap model, captures the raw content, and returns the transformed result with cost attribution.
29. Multi-model turns emit `branch.started`, branch-tagged `token.delta`, branch-tagged `cost.update`, `branch.complete`/`branch.error`, and aggregate `turn.complete`. Branch state is available via `session.sync` after reconnect.
30. `connection.health` returns structured status of all server dependencies (auth, storage, providers, routing, sessions) with diagnostic codes for degraded components.

### Connectivity and Self-Recovery

10. The harness auto-starts or reconnects to the local BFF in the solo profile without user action when the service is installed but stopped.
11. The harness detects stale socket files and removes them when safe (no owning process), or prints a specific remediation command when not safe.
12. The harness detects half-open or wedged connections within the heartbeat interval (default: 3 missed pongs at 15s interval = 45s) and begins reconnecting with exponential backoff.
13. Reconnection after harness crash, BFF crash, or network drop preserves session identity. `session.sync` reports the final state of any in-flight turn.
14. Replayed `turn.submit` (same `turn_id`) and `tool.result` (same `tool_call_id`) are idempotent — no duplicate model calls or tool effects.
15. Multi-model streamed turns can be synchronized after reconnect via `session.sync`, including per-model completed/failed/pending state.
16. Every connection failure maps to a diagnostic code from the taxonomy. Each diagnostic includes cause, auto-repair attempted, and suggested next command.
17. `modeltap server status` reports machine-readable health: protocol version, auth readiness, storage readiness, provider endpoint status, model registry status, routing readiness, active sessions, and degraded dependencies with diagnostic codes.
18. Tests exist for: version mismatch, auth expiry, TLS trust failure, socket permission denied, stale socket, provider down, model unavailable, storage unready, capability registration failed, and session locked — each asserting the correct diagnostic code and user-facing message.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0001 (Go) | BFF is implemented in Go, same binary |
| ADR-0002 (SQLite) | Session state and conversation history stored in SQLite |
| ADR-0004 (Viper) | New config keys use Viper, non-global instances |
| ADR-0005 (Capture) | Full capture continues — BFF adds conversation-level capture alongside request-level |
| ADR-0006 (Providers) | Provider adapters extended with formatting (outbound) in addition to parsing (inbound). Requires ADR-0006 amendment before acceptance. |
| ADR-0007 (Metrics) | Cost tracking feeds into existing aggregation tables |
| ADR-0009 (MCP) | MCP stdio interface is separate and unchanged. BFF does not use MCP for harness communication. |

## Resolved Questions

1. ~~**Protocol versioning**~~: resolved in the Protocol Specification section. Version exchange happens during `capabilities.register`; incompatible clients are rejected.

2. **Session conflict**: when a second harness attempts to resume a session that is already active, the server rejects the resume with a `session.active` error identifying the current participant. Only one harness may be active on a session at a time. A future **observer mode** (read-only live stream of an active session, no input) is a natural upgrade for mentor/apprentice workflows (EXP-0005), but is not part of this feature.

3. **Compaction model**: compaction uses a configurable cheap model, not the session's active model. Default: the model assigned to the `cheap` routing role (e.g., `llama-3.1-8b`). Configurable via `context.compact_model` in server config. This follows OpenCode's approach — a separate, potentially cheaper model handles summarization, keeping compaction costs minimal.

## Open Questions

1. ~~**Session handoff**~~: resolved. When the active harness disconnects (crash, network loss, close), the session automatically becomes available for resume after a short grace period (default: 10 seconds, to handle brief reconnections). If a harness crashes without a clean disconnect, the server detects the broken connection via heartbeat timeout and releases the lock. Users can also force-release a stuck session via `modeltap session unlock <id>` or `/session unlock` from another harness connection, in case the grace period or heartbeat detection fails.
2. **Compaction quality**: should the server validate compaction summaries (e.g., check that key decisions are preserved) or trust the compact model's output?
