# 2026-04-16 — Design: Protocol Types Bundle (WU-040 + WU-041 + WU-093)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Review Tier

**Assigned:** C (bundled)
**Basis:** Protocol contract + cross-track consumers. All three WUs are Tier C per `track-0-shared.md` (protocol types consumed by both BFF server and harness).
**Plan default:** C (matches)
**Escalation reason:** n/a

## Scope

This is a bundled design covering three related WUs that share a single package surface (`internal/protocol/`):

- **WU-040** — Streaming events (`events.go`): 14 streaming events + 2 non-streaming server-initiated messages, all wrapped in the `Notification` envelope added in WU-039.
- **WU-041** — Response and payload types across `tools.go`, `sessions.go`, `models.go`, `health.go`, `errors.go`, `compact.go`: ~25 types that complete the response side of every harness→server request declared in WU-039.
- **WU-093** — Shared golden fixtures at `internal/protocol/fixtures/` and cross-track conformance tests at `internal/protocol/conformance_test.go` covering 100% of types exported by the package.

**Out of scope (deferred):**
- Dispatch / transport (WU-046)
- Validation beyond zero-value JSON round-trip (WU-046)
- Business logic (all other WUs)
- Provider format translation (WU-042/043/044 — separate bundle)
- Storage schema (WU-045 bundle)

## Bundle rationale

These three WUs share one Go package and one protocol surface. WU-040 events use types from WU-041 (`ToolDefinition` for `tool.call` payload, `Diagnostic` for `error` event), and WU-093 fixtures must cover every exported type from WU-040 and WU-041. Designing them together prevents the drift that tripped the WU-039 envelope (where Codex's peer review caught `omitempty` on `Request.ID` that a single-WU design missed).

## Design Decisions

### D1. JSON-RPC envelope wrapping
- **Harness→server requests** (already shipped in WU-039) use `Request`.
- **Server→harness streaming events** (WU-040) use `Notification` (no `id`). Every event type defined here is a *payload* that serializes into `Notification.Params`.
- **Server→harness responses** (WU-041) use `Response` (matching `id`). Every response type defined here is a *payload* that serializes into `Response.Result`.

### D1.1. Every harness→server request has a `Response` payload (resolves B-01)

JSON-RPC 2.0 request/response correlation applies to all 20 harness→server methods (including streaming-initiation requests). Even methods whose "real" output is a stream of `Notification` events still return a `Response` containing a minimal payload that confirms receipt. Three families:

**Streaming-initiation ACKs** (`turn.submit`, `turn.cancel`, `tool.result`):

```go
// TurnSubmitResponse acknowledges a turn.submit. status == "accepted" on
// first submission; status in {"in_flight","complete","error","cancelled"}
// on an idempotent replay, with Sync populated from SessionSyncResponse
// (FEAT-0008 §"Idempotency rules" treats duplicate turn.submit as an
// implicit session.sync).
type TurnSubmitResponse struct {
    TurnID string               `json:"turn_id"`
    Status string               `json:"status"` // accepted | in_flight | complete | error | cancelled
    Sync   *SessionSyncResponse `json:"sync,omitempty"` // populated on replay
}

// TurnCancelResponse confirms the cancel request was recorded. The actual
// cancellation signal surfaces as `turn.complete` with cancelled=true.
type TurnCancelResponse struct {
    TurnID   string `json:"turn_id"`
    Accepted bool   `json:"accepted"`
}

// ToolResultResponse confirms receipt of a tool execution result.
// Idempotent by tool_call_id (FEAT-0008 §"Idempotency rules").
type ToolResultResponse struct {
    ToolCallID string `json:"tool_call_id"`
    Accepted   bool   `json:"accepted"`
}
```

All three types live in `messages.go` alongside their request partners.

**Fixture coverage for each:** one "first-submission" fixture + one "replay" fixture (for `TurnSubmitResponse` with Sync populated).

**Data-returning responses** (all other harness→server methods in WU-039): covered by the WU-041 response types below (`SessionListResponse`, `SessionDetail`, `ModelListResponse`, etc.).

**Pure ACKs** (`connection.ping`, `connection.health`, `connection.ready`): pong / health / ready responses defined in events.go (`ConnectionPong`) and health.go (`HealthResponse`, `ReadyResponse`).

### D2. All events carry `turn_id` and optionally `branch_id`
Per FEAT-0008 §"Correlation" (line 54), streaming events for a turn carry `turn_id`. Multi-model branching (FEAT-0008 §"Multi-Model Branching", WU-060) adds `branch_id` to branch-scoped events. Events with `branch_id` use `*string` or `omitempty` — present only when emitted under a multi-model turn.

### D3. Canonical snake_case JSON tags
Consistent with WU-039. Every field has an explicit `json:"..."` tag; no reliance on default lowercasing. `TestCanonicalFieldNames` (WU-093) extends to all new types.

### D4. Required vs. optional follows WU-039 D5 rules
- Required scalar → plain type, no `omitempty`
- Optional with presence semantics → pointer type + `omitempty`
- Optional where zero is indistinguishable from absent → plain type + `omitempty`
- Required array/map → plain type, no `omitempty`
- Optional array/map → plain type + `omitempty`

### D5. `ModelSelected.Model` / `.Provider` can be string OR array
FEAT-0008 line 813-815: single-model turn emits `"model":"claude-opus-4-6"`; multi-model emits `"model":["claude-opus-4-6","llama-3.1-70b"]`. Go does not have native sum types. Chosen representation:
```go
type ModelSelected struct {
    TurnID   string          `json:"turn_id"`
    Model    json.RawMessage `json:"model"`    // string or []string
    Provider json.RawMessage `json:"provider"` // string or []string
    Reason   string          `json:"reason"`
}
```
Helper accessors `ModelSelected.IsMulti()`, `.SingleModel()`, `.MultiModels()` will be added. This defers the polymorphism to the consumer and keeps the wire shape honest. Alternative considered: always emit as array (single = one-element). Rejected because it breaks wire compat with FEAT-0008 examples.

### D6. `DiagnosticCode` as typed string with exhaustive constants

```go
type DiagnosticCode string

const (
    DiagServiceNotRunning            DiagnosticCode = "MT-CONN-001" // service_not_running
    DiagStaleSocket                  DiagnosticCode = "MT-CONN-002" // stale_socket
    DiagSocketPermission             DiagnosticCode = "MT-CONN-003" // socket_permission
    DiagVersionMismatch              DiagnosticCode = "MT-CONN-004" // version_mismatch
    DiagTLSUntrusted                 DiagnosticCode = "MT-CONN-005" // tls_untrusted
    DiagAuthExpired                  DiagnosticCode = "MT-CONN-006" // auth_expired
    DiagStorageUnready               DiagnosticCode = "MT-CONN-007" // storage_unready
    DiagSessionLocked                DiagnosticCode = "MT-CONN-008" // session_locked
    DiagProviderUnavailable          DiagnosticCode = "MT-CONN-009" // provider_unavailable
    DiagCapabilityRegistrationFailed DiagnosticCode = "MT-CONN-010" // capability_registration_failed
    DiagModelUnavailable             DiagnosticCode = "MT-CONN-011" // model_unavailable
    DiagHeartbeatTimeout             DiagnosticCode = "MT-CONN-012" // heartbeat_timeout
)
```

All 12 codes per FEAT-0008 §"Diagnostic Taxonomy" (lines 503-514). New codes added in later WUs increment the number (`MT-CONN-013`, etc.); no versioning scheme — FEAT-0008 does not specify a versioning strategy for diagnostic codes, and this design chooses increment-only in favor of simplicity.

### D7. `ToolDefinition` moves from `messages.go` to `tools.go`
WU-039 placed `ToolDefinition` in `messages.go` because `CapabilitiesRegister` embeds it. WU-041 adds a dedicated `tools.go` for tool-related types. `internal/protocol/` is a single Go package, so the move is purely a file relocation: the declaration lives in `tools.go`, and `messages.go` continues to reference `ToolDefinition` by its bare name within the same package — no type alias, no cross-package import. Wire shape unchanged.

### D8. `ServerCapabilities` formalized as a struct

FEAT-0008 mentions `server_capabilities` with `protocol_version` and hints at additional fields (`protocol_version_range`, `supported_transforms`) without an exhaustive enumeration. Formalize with the three fields FEAT-0008 names explicitly:

```go
type ServerCapabilities struct {
    ProtocolVersion      string   `json:"protocol_version"`
    ProtocolVersionRange string   `json:"protocol_version_range,omitempty"` // e.g., "1-3"
    SupportedTransforms  []string `json:"supported_transforms,omitempty"`   // e.g., ["summarize"]
    MaxFrameSize         int      `json:"max_frame_size"`                   // bytes; currently protocol.MaxFrameSize (10 MiB)
    MaxAttachmentSize    int      `json:"max_attachment_size"`              // bytes; configurable, default 5 MiB
}
```

**Amendment (2026-04-16, post-Bundle 4 pre-review):** `MaxFrameSize` and `MaxAttachmentSize` added per WU-039 review finding A-05. The harness uses these to refuse oversize attachments before serializing. Both fields are required (not omitempty) — the harness needs them at registration time.

Additional fields can be added by later WUs with fixture updates; forward-compat is preserved because `encoding/json` tolerates unknown fields on decode.

### D9. Fixtures are organized by category, one file per type
`fixtures/` layout:
```
internal/protocol/fixtures/
├── README.md                       # freeze contract, update procedure
├── requests/
│   ├── turn_submit_full.json
│   ├── turn_submit_minimal.json
│   ├── turn_cancel.json
│   └── ...                         # one per WU-039 request type
├── events/
│   ├── token_delta.json
│   ├── branch_started.json
│   ├── ...                         # one per WU-040 event type
├── responses/
│   ├── session_list.json
│   ├── model_list.json
│   └── ...                         # one per WU-041 response type
├── errors/
│   ├── mt_conn_001.json
│   └── ...                         # one per diagnostic code
└── _covered.json                   # registry test-asserted list of types with fixtures
```

### D10. Conformance test strategy

Five checks per type or fixture:

1. **Forward:** unmarshal fixture into typed struct → re-marshal → equal to fixture (canonical-normalize to handle key ordering).
2. **Reverse:** synthesize representative instance → marshal → equal to fixture.
3. **Strict schema — unknown fields:** fixture decoded with `DisallowUnknownFields` succeeds on the canonical shape; adding a junk field fails.
4. **Strict schema — missing required fields:** a parallel set of *negative* fixtures in `fixtures/negative/` has each required field intentionally pruned and asserts the decoder rejects the frame. One negative fixture per top-level request/event/response type; reusing one per field class (all required strings, all required ints, etc.) is acceptable but at least one negative fixture per type is required. This aligns with the WU-093 track-0 spec requirement that conformance fails "if it has unknown fields **or is missing required fields**."
5. **Coverage registry:** reflection walks every exported type in `internal/protocol/` and asserts each has at least one positive fixture OR is marked `nested_only` in `fixtures/_covered.json` with a pointer to the parent fixture that exercises it. Nested-only types are covered transitively by parent fixtures; the registry makes this explicit so the coverage gate is auditable. Types that are deliberately untestable (e.g., pure marker types) may be listed under `skipped` with a rationale string, but skipping requires human review.

`_covered.json` sketch:

```json
{
  "positive": {
    "TurnSubmit":   ["requests/turn_submit_full.json", "requests/turn_submit_minimal.json"],
    "TokenDelta":   ["events/token_delta.json"]
  },
  "nested_only": {
    "Attachment":   { "parent_fixture": "requests/turn_submit_full.json" },
    "Paste":        { "parent_fixture": "requests/turn_submit_full.json" }
  },
  "skipped": {}
}
```

## File Layout

```
internal/protocol/
├── protocol.go         # (WU-039) envelope, framing, Mode, version
├── messages.go         # (WU-039 + WU-041 extension) request+response pair types
├── events.go           # (WU-040) 16 streaming / non-streaming server events
├── tools.go            # (WU-041) ToolDefinition moves here; ToolCatalog, ToolResultRequest alias stays
├── sessions.go         # (WU-041) SessionSummary, SessionDetail, TurnSummary, ServerEvent, SessionSyncResponse, etc.
├── models.go           # (WU-041) ModelInfo, ModelListResponse, ModelSwitchResponse, ModelSelected helpers
├── health.go           # (WU-041) HealthResponse, ReadyResponse, ProviderStatus, DependencyStatus, ServerCapabilities
├── errors.go           # (WU-041) DiagnosticCode constants, Diagnostic, ServerError payload
├── compact.go          # (WU-041) CompactCategory, CompactFileBreakdown, CompactPlan, CompactApplyRequest/Response
├── fixtures/           # (WU-093) golden files + README
│   ├── requests/
│   ├── events/
│   ├── responses/
│   └── errors/
├── protocol_test.go    # (WU-039) existing
├── events_test.go      # (WU-040) round-trip + method-constant
├── messages041_test.go # (WU-041) round-trip for response types
└── conformance_test.go # (WU-093) fixture-based conformance
```

## Type Catalog

Condensed format: each type gets its Go type name, method constant (if any), file, fields (name → type, required/optional, purpose), and nested types. Full field breakdowns with FEAT-0008 line references are in `/tmp/wu-040-041-catalog.md` (session work product). A fixture exists for each type unless noted.

### events.go — WU-040 streaming + non-streaming server events

All 16 events wrap their payload in `Notification`. Method-name constants colocated.

#### Method-name constants

```go
const (
    EventTokenDelta          = "token.delta"
    EventBranchStarted       = "branch.started"
    EventBranchComplete      = "branch.complete"
    EventBranchError         = "branch.error"
    EventToolCall            = "tool.call"
    EventStatusUpdate        = "status.update"
    EventKnowledgeHit        = "knowledge.hit"
    EventCostUpdate          = "cost.update"
    EventCompactPlan         = "compact.plan"     // also the response method for session.compact
    EventCompactSuggest      = "compact.suggest"
    EventCompactNotice       = "compact.notice"
    EventTurnComplete        = "turn.complete"
    EventModelSelected       = "model.selected"
    EventError               = "error"
    EventCapabilitiesRequest = "capabilities.request"
    EventConnectionPong      = "connection.pong"
)
```

**Method/type file split note:** per D9 file layout, method-name constants for streaming events live in `events.go`, but the `CompactPlan` struct type lives in `compact.go` (same package, single declaration per D7 / B-03 fix). This is the one case where a method constant and its payload type are in different files. `events.go` references `CompactPlan` by bare name.

#### Event types

Go types for each field are listed explicitly. `string` unless otherwise stated; `int` for token counts; `float64` for costs, percentages, and scores; `bool` where named.

| Go type | Method | Fields |
|---------|--------|--------|
| `TokenDelta` | `token.delta` | `turn_id string` (R), `branch_id string` (O, omitempty), `text string` (R) |
| `BranchStarted` | `branch.started` | `turn_id string` (R), `branch_id string` (R), `model string` (R), `provider string` (R) |
| `BranchComplete` | `branch.complete` | `turn_id string` (R), `branch_id string` (R), `final_input_tokens int` (R), `final_output_tokens int` (R), `model string` (R), `provider string` (R) |
| `BranchError` | `branch.error` | `turn_id string` (R), `branch_id string` (R), `error string` (R), `message string` (R), `diagnostic_code DiagnosticCode` (R), `model string` (R), `provider string` (R) |
| `ToolCall` | `tool.call` | `turn_id string` (R), `tool_call_id string` (R), `tool string` (R), `namespace string` (R), `input json.RawMessage` (R — passthrough of tool's input_schema) |
| `StatusUpdate` | `status.update` | `turn_id string` (R), `phase string` (R — enum: routing/knowledge_search/provider_call/compacting), `detail string` (R), `timestamp string` (R, ISO8601) |
| `KnowledgeHit` | `knowledge.hit` | `turn_id string` (R), `summary string` (R), `source_date string` (R, ISO8601 date), `relevance float64` (R, 0-1) |
| `CostUpdate` | `cost.update` | `turn_id string` (R), `branch_id string` (O, omitempty), `input_tokens int` (R), `output_tokens int` (R), `input_cost float64` (R), `output_cost float64` (R), `total_cost float64` (R) |
| `CompactSuggest` | `compact.suggest` | `turn_id string` (R), `context_pct float64` (R), `threshold float64` (R), `message string` (R) |
| `CompactNotice` | `compact.notice` | `turn_id string` (R), `triggered_by string` (R; known value: `"threshold_exceeded"`; FEAT-0008 does not state a closed enum, so this field is free-form), `tokens_freed int` (R), `summary string` (R) |
| `TurnComplete` | `turn.complete` | `turn_id string` (R), `final_input_tokens int` (R), `final_output_tokens int` (R), `total_cost float64` (R), `model string` (R), `provider string` (R), `latency_ms int` (R), `cancelled bool` (R) |
| `ModelSelected` | `model.selected` | `turn_id string` (R), `model json.RawMessage` (R — string or []string per D5), `provider json.RawMessage` (R — string or []string per D5), `reason string` (R). Helpers: `IsMulti() bool`, `SingleModel() (string, string, error)`, `MultiModels() ([]string, []string, error)`. |
| `ServerError` | `error` | `turn_id string` (O, omitempty), `code string` (R — coarse error bucket: `provider_error`, `budget_exceeded`, `auth_failure`, etc.; **not** a `DiagnosticCode` — the specific MT-CONN-* code lives in the nested `diagnostic`), `message string` (R), `diagnostic Diagnostic` (R — see errors.go) |
| `CapabilitiesRequestEvent` | `capabilities.request` | `reason string` (O, omitempty) — known values per FEAT-0008 line 208: `"reconnection"`, `"tool_schema_drift"`. Earlier draft included `"server_restart"`; dropped because FEAT-0008 does not ratify it. |
| `ConnectionPong` | `connection.pong` | No fields |

`CompactPlan` as a streaming event name collides with `CompactPlanResponse` from `session.compact`. Decision: the `compact.plan` method notifies harness of an *inline* compaction plan offer; the response to `session.compact` carries the same payload shape. **Single type `CompactPlan` defined in `compact.go`** is used by both paths. Because `internal/protocol/` is a single Go package, `events.go` references `CompactPlan` by bare name — no duplicate declaration, no import alias.

### tools.go — WU-041 tool types

| Go type | Purpose | Fields |
|---------|---------|--------|
| `ToolDefinition` | Canonical tool catalog entry. **Moved from WU-039 messages.go.** Used in `CapabilitiesRegister`, `CapabilitiesUpdate.AddedTools`, `tool.call` dispatch. | `name`, `namespace`, `description`, `input_schema` (`json.RawMessage`), `output_envelope` (enum), `risk_level` (enum), `capabilities_required` (O, omitempty) |
| `ToolCatalog` | Convenience wrapper for a full catalog snapshot. | `tools []ToolDefinition` |
| `CapabilitiesRegisterResponse` | Response to `capabilities.register`. | `registered []ToolDefinition` (R), `server_capabilities ServerCapabilities` (R, from health.go), `rejected []RejectedTool` (O, omitempty) |
| `RejectedTool` | Nested in `CapabilitiesRegisterResponse.rejected`. | `name` (R), `reason` (R) |
| `ToolResultRequest` | (alias already exists in WU-039 messages.go; remains a re-export.) | — |

### sessions.go — WU-041 session types

| Go type | Purpose | Fields |
|---------|---------|--------|
| `SessionSummary` | Element of `session.list` response array. | `id`, `project`, `status` (enum), `summary`, `last_active` (ISO8601), `context_pct`, `total_cost`, `turn_count`, `model`, `model_override` (O), `last_turn_summary`, `files_touched`, `pinned_count` |
| `SessionListResponse` | Response to `session.list`. | `sessions []SessionSummary` |
| `SessionDetail` | Response to `session.details`. **Name intentionally breaks the `*Response` suffix pattern** — the track-0-shared.md WU-041 spec names this type `SessionDetail` (not `SessionDetailsResponse`), and this design preserves the spec name. Wire shape is the response body. | `id string` (R), `summary string` (R), `created_at string` (R, ISO8601), `last_active string` (R, ISO8601), `model string` (R), `model_override string` (O, omitempty), `context_pct float64` (R), `total_cost float64` (R), `turns []TurnSummary` (R), `pinned_items []string` (O, omitempty), `files_touched []string` (R), `files_modified []string` (R), `server_events []ServerSessionEvent` (O, omitempty) |
| `TurnSummary` | Element of `SessionDetail.turns`. | `sequence int` (R), `summary string` (R), `compacted bool` (R), `original_turns []int` (O, omitempty), `model string` (R), `cost float64` (R) |
| `ServerSessionEvent` | Element of `SessionDetail.server_events`. Renamed from FEAT-0008's "ServerEvent" to avoid conflict with `protocol.ServerError` (events.go). | `type string` (R), `at string` (R, ISO8601), `freed_tokens int` (O, omitempty), `detail string` (R) |
| `SessionSyncResponse` | Response to `session.sync`. Complex: single-model vs. multi-model shapes. | `session_id`, `active_turn ActiveTurnState`, `multi_model *MultiModelState` (O) |
| `ActiveTurnState` | Nested. | `turn_id string` (R), `status string` (R) — enum: `pending_tool_result` / `streaming` / `complete` / `error` / `cancelled`; only `pending_tool_result` and `streaming` appear literally in FEAT-0008 §"In-Flight Turn Recovery" (lines 460-494). `complete`, `error`, `cancelled` are **design-inferred** to cover the full turn lifecycle; these three need FEAT-0008 amendment confirmation (flagged under "Deviations" below). `pending_tool_calls []PendingToolCall` (O, omitempty), `completed_tokens int` (O, omitempty), `token_replay_available bool` (R), `summary string` (R) |
| `PendingToolCall` | Nested. | `tool_call_id string` (R), `tool string` (R), `status string` (R — known value: `awaiting_result`) |
| `MultiModelState` | Nested; pointer on the parent is nil for single-model turns. | `reviewers []ReviewerState` (R) |
| `ReviewerState` | Nested. | `model string` (R), `status string` (R) — enum: `complete` / `streaming` / `failed` / `pending`; FEAT-0008 §"Session Sync Multi-Model" line 484-487 shows `complete`, `streaming`, `failed` explicitly. `pending` is **design-inferred** for the "branch queued but not yet started" case and needs FEAT-0008 amendment (flagged under "Deviations" below). `tokens int` (R), `branch_id string` (O, omitempty) |
| `SessionResumeResponse` | Response to `session.resume`. | `session_id`, `model`, `model_override` (O), `project ProjectContext` (from messages.go) |
| `SessionClearResponse` | Response to `session.clear`. | `cleared_turns`, `retained_in_storage` (always true) |
| `SessionForkResponse` | Response to `session.fork`. | `new_session_id`, `original_session_id` |
| `ContextListResponse` | Response to `context.list`. | `files []ContextFile`, `knowledge_injections []KnowledgeInjection`, `pinned_items []string`, `context_tokens`, `context_window`, `context_pct`, `system_prompt_tokens`, `knowledge_injection_tokens` |
| `ContextFile` | Nested. | `path`, `size_bytes`, `attached_turn`, `stale` |
| `KnowledgeInjection` | Nested. | `summary`, `source_date`, `relevance` |

### models.go — WU-041 model types

| Go type | Purpose | Fields |
|---------|---------|--------|
| `ModelInfo` | Element of `ModelListResponse.models`. | `name`, `provider`, `roles []string`, `capabilities []string`, `context_window` (int), `cost_per_1k_input` (float), `cost_per_1k_output` (float), `description`, `status` (O, enum), `access` (O, enum — enterprise) |
| `ModelListResponse` | Response to `model.list`. | `models []ModelInfo` (R), `current_override string` (O, omitempty), `routing_policy RoutingPolicy` (R) |
| `RoutingPolicy` | Dot-path role name → model name or array. Represented as `map[string]json.RawMessage` to allow string-or-array values. **No helpers.** Resolution logic (dot-path walking, fallback tree) belongs in the handler layer; WU-059 (routing policy) owns it. Consistent with WU-039's "types-only, no business logic" scope (Mode.Valid is the only tolerated trivial helper). | — |
| `ModelSwitchResponse` | Response to `model.switch`. | `override_set` (bool), `model` (O), `reason` — e.g., "override_set", "override_cleared" |

### health.go — WU-041 health/capability types

| Go type | Purpose | Fields |
|---------|---------|--------|
| `HealthResponse` | Response to `connection.health`. | `server_version`, `protocol_version`, `uptime_seconds`, `auth DependencyStatus`, `storage DependencyStatus`, `capabilities DependencyStatus`, `providers map[string]ProviderStatus`, `routing DependencyStatus`, `active_session *ActiveSessionInfo` (O) |
| `ReadyResponse` | Response to `connection.ready`. | `ready bool` |
| `DependencyStatus` | Reused for auth/storage/capabilities/routing. | `status` (enum: ready/unavailable/degraded/error), `method` (O), `path` (O), `reason` (O) |
| `ProviderStatus` | Element of `HealthResponse.providers`. | `status` (enum: ready/unavailable/error), `error` (O), `models` (O, int) |
| `ActiveSessionInfo` | Nested. | `id`, `owner` |
| `ServerCapabilities` | Returned in `CapabilitiesRegisterResponse`. (Per D8.) | `protocol_version`, `protocol_version_range` (O), `supported_transforms` (O), `max_frame_size` (R), `max_attachment_size` (R) |

### errors.go — WU-041 diagnostic types

| Go type | Purpose | Fields |
|---------|---------|--------|
| `DiagnosticCode` | Typed string enum. 12 constants MT-CONN-001 through MT-CONN-012. | — |
| `Diagnostic` | Structured error carried in `ErrorObject.Data` or inline in `ServerError`. | `code DiagnosticCode`, `category`, `cause`, `auto_repair_attempted` (bool), `repair_result` (O), `suggested_command` (O), `path_or_endpoint` (O) |

### compact.go — WU-041 compaction types

| Go type | Purpose | Fields |
|---------|---------|--------|
| `CompactCategory` | Element of `CompactPlan.categories`. | `name`, `token_count`, `value_score`, `suggested_action` (enum: keep/summarize/drop/pin), `summary_preview` (O) |
| `CompactFileBreakdown` | Element of `CompactPlan.files_breakdown`. | `path`, `token_count`, `attached_turn`, `stale`, `suggested_action` (enum: keep/drop) |
| `CompactPlan` | Response to `session.compact` AND payload of `compact.plan` event. | `categories []CompactCategory`, `files_breakdown []CompactFileBreakdown` (O), `estimated_tokens_freed`, `context_pct_before`, `context_pct_after` |
| `CompactApplyResponse` | Response to `compact.apply`. | `applied` (bool), `tokens_freed`, `context_pct_after`, `summary` |

### messages.go — WU-041 response pair additions

Extends existing WU-039 file with response types for requests already declared there.

| Go type | Paired method | Fields |
|---------|---------------|--------|
| `ContentTransformResponse` | `content.transform` | `content`, `model_used`, `cost` |
| `CapabilitiesUpdateResponse` | `capabilities.update` | `added_count`, `removed_count`, `updated_at` |

(`turn.cancel` and `tool.result` have no direct response — stream behavior. Documented inline in `messages.go` godoc, not a struct.)

## WU-093 — Fixtures and Conformance

### Fixture generation approach

Fixtures are hand-authored, not generated. Generation risks the "tests test the generator" anti-pattern. Each fixture is a realistic wire frame that matches a specific FEAT-0008 example or covers an important variant. Update procedure documented in `fixtures/README.md`:

1. Add or change a type in one of the protocol Go files
2. Add or update the fixture(s) by hand
3. Run `go test ./internal/protocol/...`; coverage test fails if a type has no fixture
4. Commit fixture + type change together

### Fixture coverage requirements

Per type category:
- **Requests (WU-039):** one full-populated + one minimal variant each. Already covered in WU-039 tests; promoted to fixtures in WU-093.
- **Events (WU-040):** one happy-path per event type. Multi-model variants of `BranchStarted`, `BranchComplete`, `BranchError`, `CostUpdate`, `ModelSelected` get dedicated fixtures.
- **Responses (WU-041):** one happy-path per response; `SessionListResponse` with empty array + 1-item + multi-item; `ModelListResponse` with single-model routing + multi-model routing entries.
- **Diagnostics (WU-041 errors):** one fixture per MT-CONN-* code, showing the code in context of a `ServerError` event.

Target fixture count: ~85 positive fixtures + ~20 negative fixtures = ~105 total. Breakdown:
- WU-039 requests: 20 full + 10 minimal-variant = 30
- WU-040 events: 16 base + 5 multi-model variants (BranchStarted/Complete/Error, CostUpdate, ModelSelected) = 21
- WU-041 responses: ~20 base + 3 SessionListResponse variants (empty, 1-item, many) = 23
- Diagnostic codes: 12, each embedded in a ServerError or BranchError fixture = 12
- Negative (required-field-missing, one per top-level type): ~20

The "variants" are specific, not hand-waved. Count may drop 5-10 if some types share a negative fixture class, but target bounds the effort.

### Coverage assertion

```go
// conformance_test.go
func TestFixtureCoverage(t *testing.T) {
    covered := loadCoveredRegistry()  // reads fixtures/_covered.json
    exported := reflectExportedTypes("github.com/.../internal/protocol")
    for _, t := range exported {
        if !covered[t] {
            t.Errorf("type %s has no fixture in fixtures/*/; add one or update _covered.json with rationale", t)
        }
    }
}
```

Types with intentional no-fixture status (e.g., pure marker types, type aliases) go in `fixtures/_covered.json` under a `skipped` key with justification strings. Skipping requires review; the registry is human-auditable.

## Test Plan Outline

Distributed across three test files:

- `events_test.go` (WU-040): round-trip for every event type; method-name constants match wire strings; `ModelSelected.IsMulti()` helper works for both shapes; `TestCanonicalFieldNames_Events` extends the WU-039 snake-case check.
- `messages041_test.go` (WU-041): round-trip for every response type; `DiagnosticCode` constants match their wire values; nested type independence (`TurnSummary`, `ReviewerState`, etc. round-trip standalone).
- `conformance_test.go` (WU-093): forward, reverse, strict-schema, coverage. Invoked via `go test` and covers the union of WU-039 + WU-040 + WU-041 types.

Pin a fixture for each of the 12 diagnostic codes so the `DiagnosticCode` typed enum catches any wire-value drift.

## Risks and Open Items

Each item below corresponds to an unresolved or design-inferred topic in FEAT-0008. Labels are descriptive; they do not reference any numbered ambiguity list (no canonical numbering exists in FEAT-0008).

- **Non-risk:** `ModelSelected.Model` / `.Provider` polymorphism. `json.RawMessage` + helpers is the standard Go idiom for untyped-sum JSON. Alternative representations considered and rejected.
- **Non-risk:** `ServerCapabilities` shape (D8). Resolved with the three fields FEAT-0008 names explicitly. Later additions are wire-forward-compatible due to `encoding/json`'s default tolerance for unknown fields.
- **Risk — deferred to later WUs:**
  - **Token replay semantics.** Spec is clear that `session.sync` returns summary, not tokens. WU-064 (in-flight recovery) owns the harness-side handling.
  - **Branch cancellation granularity.** Current design is strict "one turn one cancel"; `TurnCancel` carries no `branch_id`. WU-060 may revisit.
  - **Model selection intent classification.** Not a type-layer concern; routing logic is WU-059's responsibility.
  - **Context window trimming edge cases.** When Layer 6/7 trimming is not enough, server behavior is underspecified. WU-061 owns.
  - **Transform cost attribution granularity.** `ContentTransformResponse.cost` is a single float; finer-grained attribution is WU-056's concern.
  - **Branch retry hint.** No `retriable` field added speculatively to `BranchError`. WU-060 may revisit.
  - **Pinned items scope across session ops.** Behavior across model switches, forks, and `session.clear` is not typed here. WU-050 owns.
  - **Compaction quality validation.** FEAT-0008 does not decide whether the server validates compaction summaries. `CompactApplyResponse` carries no `confidence_score` field today; WU-061 may revisit.
- **Risk — requires FEAT-0008 amendment (flagged in Deviations below):**
  - Design-inferred enum values in `ActiveTurnState.status` (`complete`, `error`, `cancelled`) and `ReviewerState.status` (`pending`). FEAT-0008 shows only a subset literally; the additions are defensible for lifecycle completeness but are not spec-ratified.
- **Open item carried from WU-039 design:** `MaxFrameSize = 10 MiB` is still not ratified in FEAT-0008 (WU-049 will expose it in capability handshake + spec amendment).

## Deviations from track-0-shared.md WU-041 type names

This design renames or relocates a few types from the track spec; TPM should update `track-0-shared.md` WU-041 in lockstep to keep grep-based traceability intact.

| track-0-shared.md name | Design name | Reason |
|------------------------|-------------|--------|
| `ServerEvent` | `ServerSessionEvent` (sessions.go) | Avoid clash with `protocol.ServerError` event payload (events.go). |
| `CompactPlanResponse` | `CompactPlan` (compact.go) | Unified event + response shape; `compact.plan` event and `session.compact` response share the same payload, so one declaration serves both. |
| `ModelSelectedEvent` (listed under models.go in track spec) | `ModelSelected` (events.go) | Consistent with other event names; events.go is the right file per D2 (events are Notifications, not response types). `models.go` is for request/response types only. |

Additional deviations flagged for FEAT-0008 amendment consideration (not introduced by this design but surfaced):

- `ActiveTurnState.status` adds `complete`, `error`, `cancelled` to the `pending_tool_result`/`streaming` literally shown in FEAT-0008. Needs spec confirmation.
- `ReviewerState.status` adds `pending` to the `complete`/`streaming`/`failed` literally shown in FEAT-0008. Needs spec confirmation.
- `CompactNotice.triggered_by` is free-form (known value `threshold_exceeded`) because FEAT-0008 does not state a closed enum.

## Cross-Bundle Consistency Checks

All files live in the single Go package `internal/protocol` — no sub-packages, no cross-package imports. Consistency is about declaration location, not imports.

Before handing off to Phase 3 implementation, verify:

1. `ToolDefinition` is declared exactly once, in `tools.go`. `messages.go` references it by bare name within the same package. Wire shape is unchanged from WU-039.
2. `Diagnostic` is declared exactly once, in `errors.go`. Both `ServerError` (events.go) and `ErrorObject.Data` (protocol.go) reference it by bare name.
3. `ProjectContext` remains declared in `messages.go` (WU-039). `SessionResumeResponse` (sessions.go) references it by bare name.
4. `ToolResult` vs. `ToolResultRequest` alias (WU-039) remains wire-identical — no additional fields added in WU-041.
5. Every `DiagnosticCode` constant MT-CONN-NNN from `errors.go` appears in at least one `ServerError` or `BranchError` fixture so conformance tests exercise the full taxonomy.
6. `CompactPlan` is declared exactly once, in `compact.go`. Both the `compact.plan` event dispatch (events.go) and the `session.compact` response (compact.go) reference it by bare name.

## Files Modified / Created

**New (WU-040):**
- `internal/protocol/events.go`
- `internal/protocol/events_test.go`

**New (WU-041):**
- `internal/protocol/tools.go`
- `internal/protocol/sessions.go`
- `internal/protocol/models.go`
- `internal/protocol/health.go`
- `internal/protocol/errors.go`
- `internal/protocol/compact.go`
- `internal/protocol/messages041_test.go`

**New (WU-093):**
- `internal/protocol/fixtures/README.md`
- `internal/protocol/fixtures/requests/*.json` (20 files)
- `internal/protocol/fixtures/events/*.json` (16 + multi-model variants)
- `internal/protocol/fixtures/responses/*.json` (~20)
- `internal/protocol/fixtures/errors/*.json` (12, one per diagnostic code)
- `internal/protocol/fixtures/_covered.json`
- `internal/protocol/conformance_test.go`

**Modified (WU-041 refactor):**
- `internal/protocol/messages.go` — `ToolDefinition` declaration moves to `tools.go`; messages.go adds `ContentTransformResponse`, `CapabilitiesUpdateResponse`; update comment block header count from "20 harness→server request types" to reflect the addition of paired response types.

## Pre-Review Lint Disposition (2026-04-16)

Subagent pre-review lint ran against the initial draft of this design. Review artifact: `.sdlc/releases/v0.2.0/.reviews/protocol-types-040-041-093/claude-subagent-pre-review.md`. Findings and their dispositions:

**Blocking (all fixed in-WU):**
- B-01 — `turn.submit` Response payload unspecified → **FIXED.** Added D1.1 with explicit `TurnSubmitResponse`, `TurnCancelResponse`, `ToolResultResponse` types covering first-submission and idempotent-replay cases.
- B-02 — Design referenced "FEAT-0008 ambiguity #N" against a list that does not exist → **FIXED.** Replaced every numbered reference with a descriptive label (e.g., "server_capabilities shape", "token replay semantics"). The Risks section now enumerates deferred items by topic.
- B-03 — Cross-package language for same-package files → **FIXED.** D7 and the CompactPlan note rewritten; Cross-Bundle Consistency Checks rewritten to refer to declaration location, not imports.

**Attention (all fixed in-WU except where noted):**
- A-01 `ServerError.code` type → **FIXED.** `ServerError.Code` is `string` (coarse bucket: `provider_error`, `budget_exceeded`, etc.), NOT `DiagnosticCode`. The specific MT-CONN-* code lives in the nested `Diagnostic.Code`. Clarified in the event table.
- A-02 `RoutingPolicy.Resolve` helper → **FIXED.** Removed. Resolution logic belongs in WU-059 handler layer per WU-039 "types only, no business logic" scope.
- A-03 12 MT-CONN-* constants not enumerated → **FIXED.** D6 now lists all 12 with FEAT-0008 category names inline.
- A-04 Field Go types inconsistent → **FIXED.** Every row in the type catalog now carries explicit Go types (string / int / float64 / bool / typed enums).
- A-05 `CompactNotice.triggered_by` narrowed enum → **FIXED.** Now documented as free-form string with `"threshold_exceeded"` as known value.
- A-06 `CapabilitiesRequestEvent.reason` included `server_restart` without spec support → **FIXED.** Dropped; only FEAT-0008-ratified values remain.
- A-07 Inferred enum values in `ActiveTurnState.status`, `ReviewerState.status` → **FIXED.** Each inferred value annotated inline; flagged in new "Deviations" section for FEAT-0008 amendment.
- A-08 `SessionDetail` naming break → **FIXED.** Decision documented inline in the sessions.go table.
- A-09 Spec type renames not flagged → **FIXED.** Added "Deviations from track-0-shared.md WU-041 type names" section with three renames and rationale.
- A-10 Nested-type fixture policy unclear → **FIXED.** D10 coverage registry now explicitly supports `nested_only` mapping to parent fixtures, with `_covered.json` sketch.
- A-11 Missing required-field check in conformance → **FIXED.** D10 expanded from four checks to five; check 4 covers negative (required-field-missing) fixtures explicitly.

**Nits:**
- N-01 `// MT-CONN-002 through MT-CONN-012` elision → resolved by A-03 enumeration.
- N-02 Fixture count estimate too hand-wavy → **FIXED.** Concrete breakdown: ~105 total (85 positive + 20 negative).
- N-03 Method-const vs. type in different files → **FIXED.** Explicit note added below the method-name constants block.
- N-04 `SessionListResponse` lacks pagination → **Deferred as FEAT-0008 gap.** Flagged so downstream harness UI WUs (FEAT-0009) don't assume the field is missing by oversight.

All dispositions complete. Design ready for Phase 2 opt-in decision.
