# Track A: FEAT-0008 BFF Server

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

**Release:** v0.2.0
**WU Range:** WU-046 through WU-067, plus WU-091 (23 work units)
**Depends on:** Track 0 (WU-039 through WU-045)
**Can parallelize with:** Track B (Terminal Harness)

## Foundation

### WU-046: JSON-RPC Transport Layer
**Size:** Medium | **Dependencies:** WU-039, WU-040, WU-041

NEW `internal/bff/transport.go` — NDJSON JSON-RPC 2.0 reader/writer over `net.Conn`. Message dispatch (method routing). Request/response correlation by `id`. Error response formatting. Tests with in-memory pipes.

**Done:** Can send/receive JSON-RPC messages over `net.Conn`. Correlation works. Error responses use JSON-RPC 2.0 format.

**Security requirement (inherited from WU-039 review SR-039-01):** on `protocol.ErrFrameTooLarge` from `FrameReader.ReadFrame`, the transport MUST close the connection immediately. The reader is left mid-frame and cannot be safely resynchronized against attacker-controlled bytes. Tests must cover the close-on-oversize path.

**Validation requirements (inherited from WU-039 design review A-01, A-02):**
- Reject `turn.submit` whose raw JSON omits the required `sequence` field. The protocol types use a plain `int` (design D5); at the transport edge, use a strict decoder (`DisallowUnknownFields` equivalent) or a presence check that distinguishes "omitted" from "sent as zero".
- On decode, reject `turn.submit` frames whose `mode` value fails `protocol.Mode.Valid()`. The protocol types deliberately round-trip unknown modes; the transport must police them.
- Tests must cover both rejection paths.

### WU-047: Protocol Endpoint — Socket and TLS Listeners
**Size:** Medium | **Dependencies:** WU-046 | **Parallelizes with:** WU-048

NEW `internal/bff/server.go` — BFF server struct with Unix socket and TLS listeners. Accept connections, hand off to transport. Graceful shutdown. Config integration (`server.socket`, `server.address`, `server.tls`). Update `modeltap serve` to start BFF alongside proxy.

**Done:** Server listens on Unix socket. Server listens on TLS. Graceful shutdown. Integration with `modeltap serve`.

### WU-048: Connection Lifecycle State Machine
**Size:** Medium | **Dependencies:** WU-046 | **Parallelizes with:** WU-047

NEW `internal/bff/connection.go` — 9 states (discovering → starting → connecting → authenticating → registering → ready → degraded → reconnecting → failed). Transitions. Per-connection state tracking. Heartbeat handler (ping/pong). Health check handler (connection.health, connection.ready). Timeout and grace-period logic.

**Timing constants** (per FEAT-0008):
- Heartbeat interval: 15s (harness-initiated pings)
- Heartbeat timeout: **30s** of missed pings → connection considered lost
- Session-lock grace period: **10s** after timeout before the lock is released (handles brief reconnections)
- Degraded → reconnecting: immediately on timeout
- Reconnecting → failed: after exponential backoff retries exhausted (see WU-074 for harness side)

Both values are exposed as constants in `internal/bff/connection.go` and documented in config for future override. Server-side grace-period release is wired so WU-050 releases the lock at `heartbeat_timeout + grace_period` (40s total) from the last pong.

**Done:** State machine tests cover all valid transitions. Heartbeat round-trip works (ping every 15s, timeout at 30s). Health response returns structured status. Session lock releases at exactly 40s after last pong in tests (10s grace applied). Reconnect inside the grace window keeps the lock.

### WU-049: Capability Registration, Version Negotiation, and Project Context
**Size:** Medium | **Dependencies:** WU-046, WU-041, WU-045 | **Parallelizes with:** WU-047, WU-048

**Additional surface (inherited from WU-039 design review A-05):** expose the current NDJSON `MaxFrameSize` and the max-attachment-size limit in the capability handshake so the harness can refuse oversize attachments before serializing. WU-041 `errors.go` should add a diagnostic code (e.g., `MT-CONN-013`) for "attachment too large" so the harness renders an actionable message. Coordinate with a FEAT-0008 amendment that ratifies the cap value.

NEW `internal/bff/capabilities.go` — handles `capabilities.register`, `capabilities.update`, `capabilities.request`. Per-connection ownership of:

- **Tool catalog** — storage, schema validation (name, namespace, description, input_schema, output_envelope, risk_level, capabilities_required), add/remove on `capabilities.update`, re-request via `capabilities.request`.
- **Protocol version negotiation** — server declares supported version range; rejects incompatible harness versions with `MT-CONN-004` (`version_mismatch`, terminal); selects highest mutually supported version within range and exposes it to downstream consumers (prompt assembly, dispatch).
- **Project context** — captures `project.root`, `project.config_file`, and `project.config_content` from `capabilities.register` and from `session.resume`. Stores per-connection/per-session so Layer 4 prompt assembly (WU-054) reads config content without touching the filesystem. Re-reads `config_content` on every turn (harness-sourced) so edits take effect immediately. Exposes getters for session creation (project scoping) and path normalization.

**Done:** Registration stores tools, protocol version, and project context. Update adds/removes tools. Invalid schemas rejected. Out-of-range protocol versions produce `MT-CONN-004` and terminate the connection. Project context is retrievable by connection ID and session ID, and is refreshed on `session.resume`. Tests cover: tool registration round-trip, version negotiation (compatible, below range, above range), project context capture and refresh, invalid schema rejection.

## Sessions and Conversation

### WU-050: Session Management
**Size:** Large | **Dependencies:** WU-045, WU-046 | **Parallelizes with:** WU-049

NEW `internal/bff/session.go` — session manager using storage layer. `session.resume` (restore conversation, check lock, project context). `session.list` (filter by project, summaries). `session.details` (timeline, compacted turns, pinned items, server events). Session creation on first `turn.submit`. `session.clear`, `session.fork`. Auto-generated session summaries.

**Session lock mechanics** (one harness per session):
- Lock acquired on `session.resume` or on session creation.
- Lock released on graceful disconnect, or on connection timeout per WU-048 (`heartbeat_timeout + grace_period` = 40s from last pong).
- Locked-out clients receive `MT-CONN-008` (`session_locked`) with the owner's fingerprint and lock expiry timestamp.
- Force-release path: the `modeltap session unlock <id>` CLI (WU-065) invokes an internal admin handler that clears `lock_owner` and `lock_expires_at` regardless of grace window. The handler rejects unlocks of sessions with an actively-streaming turn unless `--force` is passed.

**Done:** Create, resume, list, details all work. Lock prevents concurrent access. Lock auto-releases at 40s after last pong. Lock survives reconnection inside the grace window. Fork creates independent copy. Clear preserves in storage. Tests cover the grace-window case (reconnect at 35s keeps the lock; reconnect at 45s is rejected with `MT-CONN-008`) and force-release via the admin handler.

### WU-051: Conversation State — Canonical Format and Persistence
**Size:** Medium | **Dependencies:** WU-050, WU-042

NEW `internal/bff/conversation.go` — stores turns in canonical format. Appends user messages from `turn.submit`, assistant messages from provider responses. Tool call/result correlation. Attachment and paste storage. Persists to `turns` table. Restores on resume. Turn sequence tracking.

**Done:** Conversation builds correctly across turns. Tool calls correlate. Attachments stored. Restores from DB.

### WU-052: Provider Format Translation — Turn Dispatch
**Size:** Medium | **Dependencies:** WU-051, WU-043, WU-044

NEW `internal/bff/dispatch.go` — takes canonical conversation, selects provider adapter, calls `FormatMessages`, sends HTTP request. Returns raw response for streaming relay. Handles non-streaming. Error wrapping with diagnostic codes.

**Done:** Dispatches to Anthropic and OpenAI with correct format. Non-streaming round-trip works. Provider errors map to diagnostic codes.

## Streaming, Prompts, Cost

### WU-053: Streaming Relay — SSE to Protocol Events
**Size:** Large | **Dependencies:** WU-052 | **Parallelizes with:** WU-054

NEW `internal/bff/streaming.go` — receives SSE from provider, parses via adapter's `ReassembleStream`, emits `token.delta` to harness, accumulates full response, emits `turn.complete`. Handles `turn.cancel`. Background logging after stream completes.

**Done:** SSE chunks → `token.delta` events. `turn.complete` has correct tokens/cost. Cancellation stops forwarding.

### WU-054: System Prompt Engine — Layers 1-5
**Size:** Medium | **Dependencies:** WU-049, WU-046 | **Parallelizes with:** WU-053

NEW `internal/bff/prompt.go` — Layer 1 (core behavioral, bundled asset), Layer 2 (tool-use from catalog), Layer 3 (domain from config), Layer 4 (project instructions from harness), Layer 5 (mode: plan/build/auto). Token counting.

**Done:** 5 layers assemble correctly. Mode switching changes Layer 5. Tool instructions from catalog. Project from harness content.

### WU-055: System Prompt Engine — Layers 6-7 and Assembly
**Size:** Medium | **Dependencies:** WU-054, WU-051

Layer 6 stub (knowledge injection placeholder for FEAT-0011). Layer 7 (session state: pinned items, plan, compaction summaries, files, model override). Full assembly pipeline with trimming (Layer 6 first, then Layer 7 when over budget). Per-turn reassembly.

**Done:** 7-layer assembly works. Trimming tested. Pinned items, plan, summaries included. Budget respected.

### WU-056: Cost Tracking and Metrics Integration
**Size:** Small | **Dependencies:** WU-051, WU-053 | **Parallelizes with:** WU-055

NEW `internal/bff/cost.go` — per-turn cost from token counts + pricing table. Session total. `cost.update` events. Turn cost in `turns` table, session total in `sessions` table. Feeds existing aggregation tables.

**Done:** Cost computed correctly. Session accumulates. Events emitted. Aggregation updated.

## Model Config and Routing

### WU-057: Three-Layer Model Config — Provider Endpoints
**Size:** Medium | **Dependencies:** WU-046 | **Parallelizes with:** WU-050, WU-054

NEW `internal/bff/providers.go` — config parsing for provider endpoints (type, api_key, host, discover). Health checking (startup validation, periodic Ollama/MLX discovery). Status tracking. Multiple endpoints of same type. Extends `internal/config/config.go`.

**Done:** Multiple endpoints parsed. Health detects availability. Ollama discovery polls `/api/tags`. Status tracked.

### WU-058: Three-Layer Model Config — Model Registry
**Size:** Medium | **Dependencies:** WU-057

NEW `internal/bff/registry.go` — auto-discovery + manual config. Built-in catalog for cloud providers. Manual overrides. Registry fields. Duplicate resolution. Periodic refresh.

**Done:** Registry populates from discovery and config. Built-in catalog covers major models. Overrides take precedence. Duplicates resolved.

### WU-059: Three-Layer Model Config — Hierarchical Routing Policy
**Size:** Large | **Dependencies:** WU-058, WU-050

NEW `internal/bff/routing.go` — hierarchical routing with dot-path resolution (category.role → category.default → default). Single-model and multi-model roles. `model.list` handler. `model.switch` handler (set/clear override). `model.selected` event emission.

**Done:** Hierarchical resolution tested for all spec cases. Multi-model returns arrays. Override persists. `model.list` and `model.selected` work.

### WU-060: Multi-Model Branching — Parallel Provider Calls

**Status: DEFERRED — superseded by sub-agents (FEAT-0013).**

Decided 2026-04-18 after the BFF pipeline (WU-046–059, WU-052/053
streaming relay, WU-091/064/063/062 handlers) landed. The parallel
fan-out + per-branch event tagging + cancellation + recovery
semantics are structurally identical to what FEAT-0013 sub-agents
must already provide. Building branch infrastructure in
`internal/bff/branch.go` would be duplicate machinery with
overlapping wire events and parallel state-tracking.

**How the use case is satisfied instead:**

- **Parallel model execution**: spawn N sub-agents, one per model
  (FEAT-0013 §"flows" with `parallel:` block). Each sub-agent has
  isolated context, can use tools naturally, and emits its own
  results through the existing single-model streaming relay path.
- **Routing-based ergonomic** (`coding.review: [opus, gpt-5]` in
  config): preserved by a thin sub-agent flow that fans out from a
  single user-facing role to N model-specific sub-agents. Defined
  declaratively, not in BFF code.
- **Reconciliation** (combining / picking from N results): out of
  scope for the sub-agent feature itself. Implemented as either:
  - a dedicated synthesizer sub-agent (the `research` flow pattern
    in FEAT-0013 §"flow library"),
  - a harness-side UI picker (panel-per-result, user clicks),
  - or a discrete reconciliation primitive added later.
  The choice depends on the use case; the sub-agent layer doesn't
  prescribe one.

**handleTurnSubmit behavior**: rejects multi-model routing with a
clear error pointing at FEAT-0013 / sub-agents. The rejection is
not a regression — the design exists, the alternative is documented.

**Files NOT created**: `internal/bff/branch.go`, `branch_test.go`,
the `BranchManager` type, the `MultiModelOpts` struct, the per-
branch goroutine pool. The `branch_id` field on streaming events
is retained — sub-agents may reuse it for their own per-agent
tagging.

## Context, Diagnostics, Recovery

### WU-061: Context Window Management and Interactive Compaction
**Size:** Large | **Dependencies:** WU-055, WU-051 | **Parallelizes with:** WU-060

NEW `internal/bff/compact.go` — token counting. Pressure warnings (`compact.suggest`). Context categorization (architecture, debugging, testing, files, planning, tool metadata, knowledge). Value scoring. Compaction plan generation (`compact.plan`). Apply logic (`compact.apply`). Auto-compaction at configurable threshold. `compact.notice`.

**Configurable thresholds and compaction model** (per FEAT-0008 success criterion): extend `internal/config/config.go` with a `context` block:

```yaml
context:
  pressure_warning_threshold: 0.78   # emits compact.suggest
  auto_compact_threshold: 0.92       # triggers auto-compact
  auto_compact_policy: default       # placeholder for named policies
  compact_model: ""                  # empty → use routing `cheap` role; explicit model ID overrides
```

Defaults (0.78 / 0.92, compact_model empty → cheap routing role resolved at call time) match FEAT-0008 documented behavior. Summarization calls resolve `compact_model` through the routing policy (WU-059) so a missing `cheap` role produces a diagnostic rather than a silent fallback. Values are read at server start and hot-reloadable on SIGHUP if the existing config supports it. Per-session override is deferred to a future WU.

**Done:** Token counting accurate. Warning fires at configured `pressure_warning_threshold`. Auto-compact fires at configured `auto_compact_threshold`. `compact_model` config is honored: empty resolves via routing, explicit model ID routes directly to the named model. Changing config and restarting the server changes the thresholds and the compaction model. Categories scored. Plan generated. Apply works. Originals retained. Tests cover custom thresholds, explicit `compact_model`, and empty `compact_model` fallback.

### WU-062: Content Transform
**Size:** Small | **Dependencies:** WU-052, WU-059 | **Parallelizes with:** WU-061

NEW `internal/bff/transform.go` — handles `content.transform` (e.g., summarize). Routes to cheap model. Captures raw per ADR-0005. Returns result with cost attribution.

**Done:** Transform routes to cheap model. Raw captured. Cost attributed separately.

### WU-063: Diagnostic Taxonomy
**Size:** Medium | **Dependencies:** WU-048, WU-041 | **Parallelizes with:** WU-050, WU-054

NEW `internal/bff/diagnostics.go` — all 12 codes (MT-CONN-001 through 012). Structured error events. Integration with connection lifecycle.

**Done:** All 12 codes implemented. Tests for each scenario.

### WU-064: In-Flight Recovery — Idempotency and session.sync
**Size:** Medium | **Dependencies:** WU-050, WU-053, WU-060 | **Parallelizes with:** WU-061

NEW `internal/bff/recovery.go` — `turn.submit` idempotency by `turn_id`. `tool.result` idempotency by `tool_call_id`. `session.sync` handler: active turn status, pending tools, completed tokens, branch states. No token replay (summary instead).

**Done:** Duplicates don't re-dispatch. Sync returns correct state for all cases.

## CLI and Providers

### WU-065: CLI — serve, server status, server sessions, session unlock
**Size:** Medium | **Dependencies:** WU-047, WU-048, WU-050, WU-063 | **Parallelizes with:** WU-064

Updates to `internal/cli/`:

- `modeltap serve` starts BFF alongside proxy.
- `modeltap server status` queries health.
- `modeltap server sessions` — list active and recent sessions (calls `session.list` via the local protocol endpoint; renders summary, project, status, context_pct, cost, turn count, model).
- `modeltap server session <id>` — show session details (calls `session.details`; renders timeline, pinned items, files touched/modified, server events, cost history).
- `modeltap session unlock <id>` — force-release a stuck session lock.

New Cobra commands under the existing `server` group plus top-level `session` group.

**Done:** `serve` starts BFF. `server status` shows health. `server sessions` lists sessions with the fields above. `server session <id>` renders session details. `session unlock` releases lock. Help updated for all new commands. Smoke tests verify each subcommand against a running BFF.

### WU-066: Ollama Provider Adapter
**Size:** Medium | **Dependencies:** WU-042 | **Parallelizes with:** any from WU-052+

NEW `internal/provider/ollama.go` — full `Provider` interface including `FormatMessages` and `FormatToolDefinitions`. Ollama message format. NDJSON streaming. Tool use support. `Detect` by host pattern.

**Done:** Full interface implemented. Format translation tested. Stream parsing tested. Tool definitions formatted.

### WU-067: BFF Server Integration Tests
**Size:** Large | **Dependencies:** all Track A

NEW `internal/bff/integration_test.go` — E2E with real BFF + in-memory storage. Tests: connect, register (including version negotiation and project context), turn with streaming, tool round-trip, session CRUD, `server sessions`/`server session <id>` payloads, model switch, compaction, multi-model, command history append/list, health, diagnostics.

**Done:** Integration tests cover FEAT-0008 success criteria. All pass.

## Command History (Server Side)

### WU-091: Command History Storage and Protocol
**Size:** Medium | **Dependencies:** WU-045, WU-046, WU-050 | **Parallelizes with:** WU-092

Required to satisfy FEAT-0009's cross-session, cross-project command history traversal (sourced from the BFF).

Implements:

- NEW `command_history` table in `internal/storage/sqlite.go` (migration v2 addendum): `id INTEGER PK`, `user_id TEXT`, `project TEXT`, `session_id TEXT`, `content TEXT`, `created_at TIMESTAMP`, plus index on `(user_id, created_at DESC)`.
- NEW `internal/bff/history.go` — protocol handlers:
  - `history.append` (harness → server): records a submitted user command. The server appends automatically on every `turn.submit`, so the handler is a thin pass-through used when the harness wants to record unsent drafts (optional).
  - `history.list` (harness → server): returns recent entries with parameters `{ scope: "user"|"project"|"session", limit: int, before: cursor }`. Server scopes results by `user_id` (always), `project` (when scope ≥ project), and `session_id` (when scope = session). Paginated cursor-based response.
- Extend `internal/protocol/messages.go` (WU-039) planning to include `HistoryAppend`, `HistoryList`, `HistoryListResponse` types. If not added during WU-039, add them here and note the protocol change.
- Hook `session.resume` and `turn.submit` in WU-050/WU-051 so every user turn is captured (idempotent by `turn_id`).

**Done:** Table created. `turn.submit` path appends history. `history.list` returns entries scoped correctly for user/project/session. Pagination cursor works. Tests cover scoping rules, pagination, and idempotent appends.

## Critical Path

```
046 → 050 → 051 → 052 → 053 → 055 → 061 (compaction)
                    052 → 059 → 060 (multi-model)
```

~14 sequential WUs on the critical path. Side paths (057→058→059, 063, 056, 054→055) branch off and merge. WU-091 (command history) runs off WU-050 and does not extend the critical path.
