# Track 0: Shared Prerequisites

**Release:** v0.2.0
**WU Range:** WU-039 through WU-045 (7 work units)
**Must complete before:** Track A (BFF Server) and Track B (Terminal Harness)

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
- New types in `internal/storage/store.go`: `Session`, `Turn`, `SessionFilter`
- New `Store` interface methods: `CreateSession`, `GetSession`, `UpdateSession`, `ListSessions`, `CreateTurn`, `GetTurn`, `ListTurns`, `GetSessionTurns`, `DeleteSessionsBefore`
- SQLite implementation for all new methods

**Definition of done:** Migration creates tables without breaking existing schema. All new CRUD methods have table-driven tests. Existing storage tests still pass. `go build ./...` passes.
