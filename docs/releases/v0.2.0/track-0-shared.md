# Track 0: Shared Prerequisites

**Release:** v0.2.0
**WU Range:** WU-039 through WU-045 (7 work units)
**Gates:**
- Track A (BFF Server): all of Track 0 (WU-039–WU-045) must complete before Track A begins.
- Track B (Terminal Harness): harness-local work (WU-068–WU-072, WU-075–WU-079) may begin after WU-039 alone is stable. Track B work that touches the protocol client or streaming/session payloads (WU-073, WU-074, WU-080+) additionally requires WU-040 and WU-041.

## WU-039: Protocol Types — Core Messages and Framing

**Size:** Medium | **Dependencies:** None

Implements `internal/protocol/`:
- `protocol.go` — protocol version constants, NDJSON framing helpers, `Mode` type (`plan`/`build`/`auto`), canonical field name documentation
- `messages.go` — all harness→server request types: `TurnSubmit`, `TurnCancel`, `ToolResult`, `ContentTransform`, `SessionResume`, `SessionList`, `SessionDetails`, `SessionCompact`, `CompactApply`, `SessionClear`, `SessionFork`, `SessionSync`, `ModelSwitch`, `ModelList`, `ContextList`, `CapabilitiesRegister`, `CapabilitiesUpdate`, `ConnectionPing`, `ConnectionHealth`, `ConnectionReady`
- `protocol_test.go` — round-trip marshal/unmarshal for every message type

**Definition of done:** `go build ./...` passes. Round-trip serialization tests pass for every message type. The package contains only types and serialization — no business logic.

---

## WU-040: Protocol Types — Streaming Events

**Size:** Medium | **Dependencies:** WU-039 | **Parallelizes with:** WU-041, WU-042

Implements `internal/protocol/events.go`:
- All server→harness streaming event types: `TokenDelta`, `BranchStarted`, `BranchComplete`, `BranchError`, `ToolCall`, `StatusUpdate`, `KnowledgeHit`, `CostUpdate`, `CompactPlan`, `CompactSuggest`, `CompactNotice`, `TurnComplete`, `ModelSelected`, `Error`
- All server→harness non-streaming types: `CapabilitiesRequest`, `ConnectionPong`
- Round-trip tests for all event types

**Definition of done:** All event types have round-trip serialization tests. `go build ./...` passes.

---

## WU-041: Protocol Types — Tools, Sessions, Models, Health, Errors

**Size:** Medium | **Dependencies:** WU-039 | **Parallelizes with:** WU-040, WU-042

Implements:
- `tools.go` — `ToolDefinition` (name, namespace, description, input_schema, output_envelope, risk_level, capabilities_required), `ToolCatalog`
- `sessions.go` — `SessionSummary`, `SessionDetail`, `TurnSummary`, `ServerEvent` payloads
- `models.go` — `ModelInfo`, `ModelListResponse`, `ModelSelectedEvent` payloads
- `health.go` — `HealthResponse`, `ReadyResponse`, `ProviderStatus`, dependency statuses
- `errors.go` — `DiagnosticCode` constants (`MT-CONN-001` through `MT-CONN-012`), `Diagnostic` struct
- `compact.go` — `CompactCategory`, `CompactPlanResponse`, `CompactApplyRequest`
- Round-trip tests for all types

**Definition of done:** All types have round-trip serialization tests. `go build ./...` passes.

---

## WU-042: ADR-0006 Amendment — Provider Outbound Formatting Interface

**Size:** Small | **Dependencies:** WU-039 | **Parallelizes with:** WU-040, WU-041

Implements:
- ADR amendment document: `docs/adr/0006-amendment-001-outbound-formatting.md`
- Extend `Provider` interface in `internal/provider/provider.go` with:
  - Canonical `Message` type (role, content, tool_calls, tool_results, attachments, metadata)
  - `FormatMessages(canonical []Message, systemPrompt string, windowSize int) ([]byte, error)`
  - `FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error)`
- Stub implementations in Anthropic and OpenAI adapters that return `ErrNotImplemented`

**Definition of done:** ADR amendment written. `Provider` interface extended. Existing tests still pass (stubs satisfy interface). `go build ./...` passes.

---

## WU-043: Anthropic Outbound Formatting

**Size:** Medium | **Dependencies:** WU-042 | **Parallelizes with:** WU-044, WU-045

Implements `FormatMessages` and `FormatToolDefinitions` for the Anthropic adapter:
- Translate canonical message history → Anthropic Messages API format
- `messages` array with `role`/`content` blocks
- `tool_use`/`tool_result` block types
- System prompt as `system` parameter
- Tool definitions as `tools` parameter
- Context window truncation (drop oldest turns, preserve system prompt + recent)

**Definition of done:** Table-driven tests cover: simple text, multi-turn, tool call/result round-trips, system prompt injection, context window truncation, messages with attachments. All tests pass.

---

## WU-044: OpenAI Outbound Formatting

**Size:** Medium | **Dependencies:** WU-042 | **Parallelizes with:** WU-043, WU-045

Implements `FormatMessages` and `FormatToolDefinitions` for the OpenAI adapter:
- Translate canonical messages → OpenAI Chat Completions format
- `messages` array with `role`/`content`
- `tool_calls`/`function` format
- System prompt as system role message
- Tool definitions as `tools` parameter
- Context window truncation

**Definition of done:** Table-driven tests matching WU-043 scope but for OpenAI format. All tests pass.

---

## WU-045: Session and Turn Storage Schema

**Size:** Large | **Dependencies:** WU-039 | **Parallelizes with:** WU-043, WU-044

Implements:
- New `sessions` and `turns` tables in `internal/storage/sqlite.go` (migration v2)
- New types in `internal/storage/store.go`: `Session`, `Turn`, `SessionFilter`, `ServerEvent`
- New `Store` interface methods: `CreateSession`, `GetSession`, `UpdateSession`, `ListSessions`, `CreateTurn`, `GetTurn`, `ListTurns`, `GetSessionTurns`, `AppendServerEvent`, `ListServerEvents`, `DeleteSessionsBefore`
- SQLite implementation for all new methods

**`sessions` table** (fields required to satisfy FEAT-0008 `session.list` and `session.details` payloads):

| Column | Type | Purpose |
|--------|------|---------|
| `id` | TEXT PK | Session UUID |
| `user_id` | TEXT | Owner (FEAT-0010 isolation) |
| `project` | TEXT | Project root / identifier (session scoping) |
| `summary` | TEXT | Auto-generated short title |
| `active_model` | TEXT | Currently selected model |
| `model_override` | TEXT NULL | Session-level model override, if set |
| `routing_overrides` | JSON | Per-session routing overrides |
| `pinned_items` | JSON | User-pinned context items |
| `compaction_state` | JSON | Current compaction state (categorized buckets, thresholds hit, last auto-compact summary) |
| `total_cost` | REAL | Running session cost |
| `total_input_tokens` | INTEGER | Running input token total |
| `total_output_tokens` | INTEGER | Running output token total |
| `context_pct` | REAL | Last observed context usage ratio (for `session.list`) |
| `status` | TEXT | active, suspended, completed |
| `lock_owner` | TEXT NULL | Harness holding the session lock |
| `lock_expires_at` | TIMESTAMP NULL | Lock grace deadline |
| `created_at` | TIMESTAMP | Session creation time |
| `updated_at` | TIMESTAMP | Last activity time |

**`turns` table** (per FEAT-0008 Session and Turn Storage Model):

| Column | Type | Purpose |
|--------|------|---------|
| `id` | TEXT PK | Turn UUID |
| `session_id` | TEXT FK | Parent session |
| `sequence` | INTEGER | Turn order within session |
| `role` | TEXT | user, assistant |
| `content` | TEXT (JSON) | Canonical message content |
| `model` | TEXT | Model used for this turn |
| `provider` | TEXT | Provider used |
| `input_tokens` | INTEGER | Input tokens for this turn |
| `output_tokens` | INTEGER | Output tokens for this turn |
| `cost` | REAL | Cost for this turn |
| `latency_ms` | INTEGER | Provider response latency |
| `tool_calls` | JSON | Tool calls made in this turn |
| `files_touched` | JSON | File paths read in this turn |
| `files_modified` | JSON | File paths written/edited in this turn |
| `compacted` | BOOLEAN | Whether this turn has been compacted |
| `compacted_summary` | TEXT | Summary if compacted |
| `original_turns` | JSON | Sequence IDs collapsed into a compacted turn |
| `created_at` | TIMESTAMP | Turn timestamp |

**`session_events` table** (supports `session.details.server_events` and resume notifications):

| Column | Type | Purpose |
|--------|------|---------|
| `id` | INTEGER PK | Event ID |
| `session_id` | TEXT FK | Parent session |
| `type` | TEXT | `auto_compact`, `server_restart`, etc. |
| `detail` | TEXT | Human-readable description |
| `payload` | JSON | Type-specific fields (e.g., `freed_tokens`) |
| `at` | TIMESTAMP | Event time |

**Definition of done:** Migration creates all three tables without breaking existing schema. All enumerated fields are present. Store methods cover CRUD for sessions, turns, and server events, including derivation helpers for `session.list` (summary + context_pct + files_touched aggregate) and `session.details` (timeline including compacted turns, pinned items, files touched/modified, server events). All new methods have table-driven tests. Existing storage tests still pass. `go build ./...` passes.
