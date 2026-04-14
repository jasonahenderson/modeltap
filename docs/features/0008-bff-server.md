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

### Protocol Messages

**Harness → Server:**

| Message | Description |
|---------|-------------|
| `turn.submit` | User message with optional file attachments and tool results |
| `tool.result` | Result of a tool execution (success or error) |
| `session.resume` | Resume an existing session by ID |
| `session.list` | List available sessions for this user/project |
| `session.compact` | Request context compaction |
| `session.clear` | Clear live context (retain in storage) |
| `session.fork` | Branch session into independent continuation |
| `model.switch` | Change the active model for this session |
| `context.list` | List files, knowledge injections, and context budget |

**Server → Harness (streaming):**

| Event | Description |
|-------|-------------|
| `token.delta` | Incremental text from model response |
| `tool.call` | Model requests a tool execution |
| `status.update` | Status message ("routing to claude-opus-4-6...", "searching knowledge...") |
| `knowledge.hit` | Knowledge context was injected (summary, relevance score) |
| `cost.update` | Running token count and cost for this turn |
| `compact.notice` | Context was compacted (what was compressed, what was retained) |
| `turn.complete` | Turn finished (final usage, model, latency, total cost) |
| `error` | Server-side error (provider failure, budget exceeded, auth failure) |

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

The provider adapter interface (ADR-0006) is extended to support both parsing (inbound response) and formatting (outbound request) for full message histories.

### System Prompt Engine

The server assembles system prompts from multiple sources, concatenated in priority order:

1. **Domain prompt**: behavioral specification for the workflow domain (coding, legal, finance, etc.). Configurable per server or per project.
2. **Project instructions**: project-level configuration file (`.modeltap.yaml` or equivalent) that specifies conventions, constraints, and preferences.
3. **Knowledge injections**: relevant prior context from the knowledge layer (when FEAT-0011 is active).
4. **Session state**: pinned items, active constraints, compaction summaries.

The assembled system prompt is included in every provider request. It is re-assembled on every turn (not cached) so that knowledge injections and session state reflect the latest information.

### Model Routing Policy

The server selects which model handles each request based on configurable policy:

```yaml
routing:
  default: claude-sonnet-4-6       # fallback for unclassified requests
  coding: claude-opus-4-6          # strong code generation
  review: gpt-4                    # different model to avoid same-model blind spots
  cheap: llama-3.1-8b              # local via Ollama, $0.00
  embedding: nomic-embed-text      # local embedding
```

The user can override routing at any time via `/model <name>`. Explicit model selection overrides policy for that session until changed again.

Routing policy is extensible — future work (EXP-0007) may add complexity-based routing, cost-aware automatic selection, and multi-model orchestration. This feature implements the static policy engine only.

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

The server tracks context window usage and manages compaction:

- Token counting for the full conversation (system prompt + history + current turn)
- Warning events at configurable threshold (default: 78% of model's context window)
- Auto-compaction at high threshold (default: 92%):
  - Identify low-value segments (old turns, resolved tangents, repeated instructions)
  - Summarize them using a compact prompt
  - Replace verbose history with summary in the live conversation
  - Retain full original in storage for future retrieval
- Manual compaction via `session.compact` message

### Session Persistence

Sessions are persisted to SQLite and survive:

- Harness disconnection and reconnection
- Server restarts
- Model switches within a session

Session data includes: conversation history (canonical format), active model, routing overrides, pinned items, compaction state, cost totals, creation/update timestamps, and project association.

## CLI Integration

The BFF server is started via existing service management (FEAT-0004) or inline:

```
modeltap serve                    # Start the server (foreground)
modeltap service install          # Install as background service (existing FEAT-0004)
```

New server administration commands:

```
modeltap server status            # Show server status, connected clients, active sessions
modeltap server sessions          # List active and recent sessions
modeltap server session <id>      # Show session details (turns, cost, model history)
```

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

# Routing policy
routing:
  default: claude-sonnet-4-6
  coding: claude-opus-4-6
  cheap: llama-3.1-8b
  embedding: nomic-embed-text

# Context management
context:
  warning_threshold: 0.78     # emit warning at 78%
  compact_threshold: 0.92     # auto-compact at 92%

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

1. A harness client can connect to the server via Unix socket or TLS and authenticate (initially: local socket with peer credentials).
2. The server accepts a conversation turn, routes it to a configured provider, and streams the response back as harness protocol events.
3. Conversation state persists across harness disconnection and reconnection — resuming a session restores the full conversation history.
4. Switching models mid-session preserves conversation context — the server translates the canonical history to the new provider's format.
5. The server assembles system prompts from domain, project, and session sources and includes them in every provider request.
6. Cost tracking reports accurate token counts and costs per turn and per session, matching provider billing within 5%.
7. Context window management triggers compaction at the configured threshold without user intervention.
8. The existing proxy functionality (capture, metrics, retention) continues to work — the BFF layer does not break the v1 proxy core.
9. The server handles concurrent harness connections (preparation for FEAT-0010 multi-user, even if initially single-user).

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0001 (Go) | BFF is implemented in Go, same binary |
| ADR-0002 (SQLite) | Session state and conversation history stored in SQLite |
| ADR-0004 (Viper) | New config keys use Viper, non-global instances |
| ADR-0005 (Capture) | Full capture continues — BFF adds conversation-level capture alongside request-level |
| ADR-0006 (Providers) | Provider adapters extended with formatting (outbound) in addition to parsing (inbound) |
| ADR-0007 (Metrics) | Cost tracking feeds into existing aggregation tables |
| ADR-0009 (MCP) | MCP stdio interface is separate and unchanged. BFF does not use MCP for harness communication. |

## Open Questions

1. **Protocol versioning**: how does the server handle harness clients running a different protocol version? Strict version match, or negotiated compatibility?
2. **Session conflict**: what happens when two harness clients try to resume the same session? Reject the second, or support collaborative sessions?
3. **Compaction model selection**: should compaction use the session's active model, a cheap dedicated model, or be configurable?
