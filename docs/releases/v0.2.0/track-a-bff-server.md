# Track A: FEAT-0008 BFF Server

**Release:** v0.2.0
**WU Range:** WU-046 through WU-067 (22 work units)
**Depends on:** Track 0 (WU-039 through WU-045)
**Can parallelize with:** Track B (Terminal Harness)

## Foundation

### WU-046: JSON-RPC Transport Layer
**Size:** Medium | **Dependencies:** WU-039, WU-040, WU-041

NEW `internal/bff/transport.go` — NDJSON JSON-RPC 2.0 reader/writer over `net.Conn`. Message dispatch (method routing). Request/response correlation by `id`. Error response formatting. Tests with in-memory pipes.

**Done:** Can send/receive JSON-RPC messages over `net.Conn`. Correlation works. Error responses use JSON-RPC 2.0 format.

### WU-047: Protocol Endpoint — Socket and TLS Listeners
**Size:** Medium | **Dependencies:** WU-046 | **Parallelizes with:** WU-048

NEW `internal/bff/server.go` — BFF server struct with Unix socket and TLS listeners. Accept connections, hand off to transport. Graceful shutdown. Config integration (`server.socket`, `server.address`, `server.tls`). Update `modeltap serve` to start BFF alongside proxy.

**Done:** Server listens on Unix socket. Server listens on TLS. Graceful shutdown. Integration with `modeltap serve`.

### WU-048: Connection Lifecycle State Machine
**Size:** Medium | **Dependencies:** WU-046 | **Parallelizes with:** WU-047

NEW `internal/bff/connection.go` — 9 states (discovering → starting → connecting → authenticating → registering → ready → degraded → reconnecting → failed). Transitions. Per-connection state tracking. Heartbeat handler (ping/pong). Health check handler (connection.health, connection.ready). Timeout and grace period logic.

**Done:** State machine tests cover all valid transitions. Heartbeat round-trip works. Health response returns structured status. Grace period releases session lock.

### WU-049: Capability Registration and Tool Catalog
**Size:** Small | **Dependencies:** WU-046, WU-041 | **Parallelizes with:** WU-047, WU-048

NEW `internal/bff/capabilities.go` — handles `capabilities.register`, `capabilities.update`, `capabilities.request`. Tool catalog storage per connection. Validates tool schema fields.

**Done:** Registration stores tools. Update adds/removes. Invalid schemas rejected.

## Sessions and Conversation

### WU-050: Session Management
**Size:** Large | **Dependencies:** WU-045, WU-046 | **Parallelizes with:** WU-049

NEW `internal/bff/session.go` — session manager using storage layer. `session.resume` (restore conversation, check lock, project context). `session.list` (filter by project, summaries). `session.details` (timeline, compacted turns, pinned items, server events). Session creation on first `turn.submit`. Session lock (one harness per session). `session.clear`, `session.fork`. Auto-generated session summaries.

**Done:** Create, resume, list, details all work. Lock prevents concurrent access. Fork creates independent copy. Clear preserves in storage.

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
**Size:** Large | **Dependencies:** WU-059, WU-053 | **Parallelizes with:** WU-061

NEW `internal/bff/branch.go` — parallel goroutines per model. `branch_id` tagging. `branch.started`, branch-tagged events, `branch.complete`/`branch.error`, aggregate `turn.complete`. Branch state for `session.sync`. `turn.cancel` cancels all branches.

**Done:** Parallel branches stream independently. Events tagged. Aggregate completion. Cancel stops all. Branch state available.

## Context, Diagnostics, Recovery

### WU-061: Context Window Management and Interactive Compaction
**Size:** Large | **Dependencies:** WU-055, WU-051 | **Parallelizes with:** WU-060

NEW `internal/bff/compact.go` — token counting. Pressure warnings (`compact.suggest`). Context categorization (architecture, debugging, testing, files, planning, tool metadata, knowledge). Value scoring. Compaction plan generation (`compact.plan`). Apply logic (`compact.apply`). Auto-compaction at 92%. `compact.notice`.

**Done:** Token counting accurate. Warning at threshold. Categories scored. Plan generated. Apply works. Auto-compact triggers. Originals retained.

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

### WU-065: CLI — serve, server status, session unlock
**Size:** Medium | **Dependencies:** WU-047, WU-048, WU-050, WU-063 | **Parallelizes with:** WU-064

Updates to `internal/cli/`: `modeltap serve` starts BFF alongside proxy. `modeltap server status` queries health. `modeltap session unlock <id>`. New Cobra commands.

**Done:** `serve` starts BFF. `server status` shows health. `session unlock` releases lock. Help updated.

### WU-066: Ollama Provider Adapter
**Size:** Medium | **Dependencies:** WU-042 | **Parallelizes with:** any from WU-052+

NEW `internal/provider/ollama.go` — full `Provider` interface including `FormatMessages` and `FormatToolDefinitions`. Ollama message format. NDJSON streaming. Tool use support. `Detect` by host pattern.

**Done:** Full interface implemented. Format translation tested. Stream parsing tested. Tool definitions formatted.

### WU-067: BFF Server Integration Tests
**Size:** Large | **Dependencies:** all Track A

NEW `internal/bff/integration_test.go` — E2E with real BFF + in-memory storage. Tests: connect, register, turn with streaming, tool round-trip, session CRUD, model switch, compaction, multi-model, health, diagnostics.

**Done:** Integration tests cover FEAT-0008 success criteria. All pass.

## Critical Path

```
046 → 050 → 051 → 052 → 053 → 055 → 061 (compaction)
                    052 → 059 → 060 (multi-model)
```

~14 sequential WUs on the critical path. Side paths (057→058→059, 063, 056, 054→055) branch off and merge.
