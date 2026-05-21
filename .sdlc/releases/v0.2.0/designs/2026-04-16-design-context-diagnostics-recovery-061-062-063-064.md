# 2026-04-16 — Design: Context, Diagnostics, Recovery Bundle (WU-061 + WU-062 + WU-063 + WU-064)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Scope

This bundle covers context management, content transformation, diagnostics, and in-flight recovery in `internal/bff/`:

- **WU-061** — Context window management and interactive compaction (`compact.go`): token counting, pressure warnings, context categorization, value scoring, compaction plan generation, apply logic, auto-compaction.
- **WU-062** — Content transform (`transform.go`): pre-turn summarization via cheap model, raw capture, cost attribution.
- **WU-063** — Diagnostic taxonomy (`diagnostics.go`): all 12 MT-CONN codes, structured error events, lifecycle integration.
- **WU-064** — In-flight recovery (`recovery.go`): turn.submit and tool.result idempotency, session.sync handler.

**Out of scope:** CLI commands for compaction (WU-065), harness compaction UI (WU-084), harness connection UX for diagnostics (WU-086).

## Bundle Rationale

These four WUs share the `internal/bff/` package and handle cross-cutting concerns: compaction (061) consumes diagnostics (063) for error reporting, content transform (062) uses the same dispatch path as compaction's summarization, and recovery (064) needs to understand the streaming relay's (053) state. They don't form a strict pipeline but share enough internal APIs that designing them together prevents interface conflicts.

## Design Decisions

### D1. Package structure

```
internal/bff/
  compact.go       — WU-061: CompactionEngine, categorization, planning, apply
  compact_test.go
  transform.go     — WU-062: ContentTransformer, summarize, cost attribution
  transform_test.go
  diagnostics.go   — WU-063: DiagnosticEmitter, all 12 codes, event integration
  diagnostics_test.go
  recovery.go      — WU-064: IdempotencyGuard, session.sync handler
  recovery_test.go
```

### D2. Context window management and compaction (WU-061)

#### D2.1. CompactionEngine

```go
// CompactionEngine manages context pressure, categorization, and compaction.
type CompactionEngine struct {
    store    storage.Store
    router   *RoutingPolicy // for resolving compact_model
    registry *ModelRegistry
    config   CompactionConfig
}

type CompactionConfig struct {
    PressureWarningThreshold float64       // default: 0.78
    AutoCompactThreshold     float64       // default: 0.92
    CompactModel             string        // empty → resolve via routing "cheap" role
}

func NewCompactionEngine(store storage.Store, router *RoutingPolicy, registry *ModelRegistry, config CompactionConfig) *CompactionEngine
```

#### D2.2. Context pressure monitoring

```go
// CheckPressure computes current context usage and emits warnings.
// Called after each turn completes. turnID is the just-completed turn
// (required by CompactSuggest and CompactNotice event types).
func (ce *CompactionEngine) CheckPressure(conn *Connection, session *ActiveSession, turnID string) {
    windowSize := ce.registry.Get(session.ActiveModel).Info.ContextWindow
    usedTokens := ce.estimateContextTokens(session)
    pct := float64(usedTokens) / float64(windowSize)
    
    session.ContextPct = pct
    
    if pct >= ce.config.AutoCompactThreshold {
        ce.autoCompact(conn, session)
    } else if pct >= ce.config.PressureWarningThreshold {
        conn.transport.SendNotification(&protocol.Notification{
            Method: "compact.suggest",
            Params: marshal(CompactSuggest{ContextPct: pct, Threshold: ce.config.AutoCompactThreshold}),
        })
    }
}

func (ce *CompactionEngine) estimateContextTokens(session *ActiveSession) int {
    total := 0
    for _, msg := range session.Conversation.Messages() {
        total += provider.EstimateMessageTokens(msg)
    }
    return total
}
```

#### D2.3. Context categorization

Seven categories per FEAT-0008:

```go
type CompactCategory struct {
    Name            string  // architecture, debugging, testing, files, planning, tool_metadata, knowledge
    TokenCount      int
    ValueScore      float64 // 0.0-1.0 (higher = more valuable, less likely to compact)
    SuggestedAction string  // keep, summarize, drop, pin
    Turns           []int   // turn sequences in this category
}

// Categorize assigns each turn to a category based on content analysis.
func (ce *CompactionEngine) Categorize(session *ActiveSession) []CompactCategory
```

Categorization heuristics:
- **architecture**: turns discussing design, structure, interfaces
- **debugging**: turns with error messages, stack traces, debugging
- **testing**: turns about tests, test results
- **files**: file reads and their content
- **planning**: plan mode turns, task lists
- **tool_metadata**: tool call/result pairs (commands executed, outputs)
- **knowledge**: knowledge injection content (Layer 6 — currently empty)

Value scoring: recent turns score higher; user-pinned content scores 1.0; stale file reads score low; debugging completed issues score low.

#### D2.4. Compaction plan generation

```go
// GeneratePlan creates a compaction plan based on current categorization.
func (ce *CompactionEngine) GeneratePlan(session *ActiveSession) (*protocol.CompactPlan, error)
```

Returns `protocol.CompactPlan` (from WU-041) with `Categories`, `FilesBreakdown`, `EstimatedTokensFreed`, `ContextPctBefore`, `ContextPctAfter`.

#### D2.5. session.compact handler

```go
func handleSessionCompact(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Returns the `CompactPlan` for the harness to display. The harness reviews and sends `compact.apply` with the user's choices.

#### D2.6. compact.apply handler

```go
func handleCompactApply(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Params: `protocol.CompactApply` with per-category actions (keep/summarize/drop/pin).

Flow:
1. For each category with action `"summarize"`:
   - Resolve compact model (from config or routing `cheap` role)
   - Send turns to model with "summarize this conversation segment" prompt
   - Replace original turns with summary turn (set `compacted=true`, `compacted_summary`, `original_turns`)
2. For each category with action `"drop"`:
   - Mark turns as compacted with empty summary
3. For each category with action `"pin"`:
   - Add to session's pinned items
4. Persist all changes
5. Emit `compact.notice` event with tokens freed
6. Return `CompactApplyResponse` with results

#### D2.7. Auto-compaction

```go
func (ce *CompactionEngine) autoCompact(conn *Connection, session *ActiveSession) {
    plan, err := ce.GeneratePlan(session)
    // Apply with default actions (summarize low-value, keep high-value)
    // Emit compact.notice with triggered_by: "threshold_exceeded"
    store.AppendServerEvent(ctx, &storage.ServerSessionEvent{
        Type: "auto_compact", Detail: "...", Payload: marshal(map[string]any{"freed_tokens": freed}),
    })
}
```

#### D2.8. Configurable thresholds

Config block (per track-a-bff-server.md WU-061):

```yaml
context:
  pressure_warning_threshold: 0.78
  auto_compact_threshold: 0.92
  compact_model: ""  # empty → routing "cheap" role
```

Values read at server start. `compact_model` resolved through routing policy (WU-059) — missing `cheap` role produces a diagnostic rather than silent fallback.

### D3. Content transform (WU-062)

#### D3.1. ContentTransformer

```go
// ContentTransformer handles content.transform requests (e.g., summarize large pastes).
type ContentTransformer struct {
    dispatcher *TurnDispatcher
    router     *RoutingPolicy
    registry   *ModelRegistry
}

func NewContentTransformer(dispatcher *TurnDispatcher, router *RoutingPolicy, registry *ModelRegistry) *ContentTransformer
```

#### D3.2. content.transform handler

```go
func handleContentTransform(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Params: `protocol.ContentTransform{Content, Transform, MaxLength}` (from WU-039 messages.go).

Flow:
1. Resolve cheap model via routing policy (`cheap` role)
2. Build prompt: "Summarize the following content in at most {MaxLength} tokens:\n\n{Content}"
3. Dispatch via `dispatcher.DispatchSync()` (non-streaming)
4. Capture raw request/response per ADR-0005
5. Return `ContentTransformResponse{Result, Model, InputTokens, OutputTokens, Cost}`
6. Cost attributed separately from the main turn (shown as "transform cost" in cost breakdown)

### D4. Diagnostic taxonomy (WU-063)

#### D4.1. DiagnosticEmitter

```go
// DiagnosticEmitter creates and emits structured diagnostic events.
type DiagnosticEmitter struct {
    conn *Connection
}

func NewDiagnosticEmitter(conn *Connection) *DiagnosticEmitter
```

#### D4.2. All 12 diagnostic codes

```go
// EmitDiagnostic sends a diagnostic event to the harness and optionally
// triggers state transitions.
func (de *DiagnosticEmitter) EmitDiagnostic(code protocol.DiagnosticCode, cause string, opts DiagnosticOpts) error

type DiagnosticOpts struct {
    AutoRepairAttempted bool
    RepairResult        string
    SuggestedCommand    string
    PathOrEndpoint      string
    Terminal            bool // if true, transition to ConnFailed
}
```

Full taxonomy (from FEAT-0008):

| Code | Category | Auto-Repair | Terminal | State Transition |
|------|----------|-------------|----------|-----------------|
| MT-CONN-001 | service_not_running | Auto-start attempted | No | → ConnStarting |
| MT-CONN-002 | stale_socket | Remove if safe | No | Retry connect |
| MT-CONN-003 | socket_permission | None | Yes | → ConnFailed |
| MT-CONN-004 | version_mismatch | None | Yes | → ConnFailed |
| MT-CONN-005 | tls_untrusted | None | Yes | → ConnFailed |
| MT-CONN-006 | auth_expired | Re-auth attempted | No | → ConnAuthenticating |
| MT-CONN-007 | storage_unready | None | No | → ConnDegraded |
| MT-CONN-008 | session_locked | None | No | Error response |
| MT-CONN-009 | provider_unavailable | Routing fallback | No | → ConnDegraded |
| MT-CONN-010 | capability_registration_failed | Re-register | No | Retry |
| MT-CONN-011 | model_unavailable | Routing fallback | No | Error response |
| MT-CONN-012 | heartbeat_timeout | Reconnect | No | → ConnReconnecting |

#### D4.3. Integration with connection lifecycle

The DiagnosticEmitter is wired into the connection state machine (WU-048):

```go
// In Connection.transition():
if to == ConnFailed {
    de.EmitDiagnostic(determineDiagnosticCode(reason), reason, DiagnosticOpts{Terminal: true})
}

// In ProviderRegistry health check failure:
de.EmitDiagnostic(protocol.DiagCodeProviderUnavailable, err.Error(), DiagnosticOpts{
    AutoRepairAttempted: true,
    RepairResult: "routing fallback attempted",
    PathOrEndpoint: endpoint.Host,
})
```

### D5. In-flight recovery (WU-064)

#### D5.1. IdempotencyGuard

```go
// IdempotencyGuard prevents duplicate processing of turn.submit and tool.result.
type IdempotencyGuard struct {
    mu       sync.Mutex
    turnIDs  map[string]TurnStatus    // turn_id → status
    toolIDs  map[string]bool          // tool_call_id → processed
}

type TurnStatus struct {
    Status string // "accepted", "in_flight", "complete", "error", "cancelled"
    Sync   *protocol.SessionSyncResponse // populated for replay
}

func NewIdempotencyGuard() *IdempotencyGuard
```

#### D5.2. Turn.submit idempotency

```go
// CheckTurnSubmit returns (isNew bool, existingStatus *TurnStatus).
// If isNew is false, the turn was already submitted — return the existing status.
func (ig *IdempotencyGuard) CheckTurnSubmit(turnID string) (bool, *TurnStatus)

// RecordTurn records a turn's status for idempotency checks.
func (ig *IdempotencyGuard) RecordTurn(turnID string, status TurnStatus)
```

Per FEAT-0008 idempotency rules: a duplicate `turn.submit` returns the current status of that turn as an implicit `session.sync`. The `TurnSubmitResponse` includes `Status` and `Sync` fields for this case.

#### D5.3. Tool.result idempotency

```go
// CheckToolResult returns true if this tool_call_id was already processed.
func (ig *IdempotencyGuard) CheckToolResult(toolCallID string) bool

// RecordToolResult marks a tool_call_id as processed.
func (ig *IdempotencyGuard) RecordToolResult(toolCallID string)
```

Duplicate `tool.result` is acknowledged but not re-processed.

#### D5.4. session.sync handler

```go
func handleSessionSync(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Returns `protocol.SessionSyncResponse` with:
- `SessionID`: current session
- `ActiveTurn`: if a turn is in-flight:
  - `TurnID`, `Status` (pending_tool_result, streaming, etc.)
  - `PendingToolCalls`: from `Conversation.PendingToolCalls()`
  - `CompletedTokens`: tokens received so far in the stream
  - `TokenReplayAvailable`: always `false` (per FEAT-0008 — summary, not replay)
  - `Summary`: brief description of current state
- `MultiModel`: from `BranchManager.BranchState()` if multi-model turn active

```go
func buildSyncResponse(session *ActiveSession) *protocol.SessionSyncResponse {
    resp := &protocol.SessionSyncResponse{SessionID: session.ID}
    
    if session.ActiveTurn != nil {
        resp.ActiveTurn = &protocol.ActiveTurnState{
            TurnID:               session.ActiveTurn.TurnID,
            Status:               session.ActiveTurn.Status,
            PendingToolCalls:     convertPendingToolCalls(session.Conversation.PendingToolCalls()),
            CompletedTokens:      session.ActiveTurn.CompletedTokens,
            TokenReplayAvailable: false,
            Summary:              session.ActiveTurn.Summary(),
        }
    }
    
    if session.BranchManager != nil {
        resp.MultiModel = session.BranchManager.BranchState()
    }
    
    return resp
}
```

## Test Strategy

### WU-061 tests (`compact_test.go`)

| Test | Description |
|------|-------------|
| `TestCompact_PressureWarning` | Warning emitted at 78% threshold |
| `TestCompact_AutoCompact` | Auto-compact triggered at 92% threshold |
| `TestCompact_CustomThresholds` | Config thresholds honored |
| `TestCompact_Categorization` | Turns assigned to correct categories |
| `TestCompact_ValueScoring` | Recent turns score higher than old |
| `TestCompact_PlanGeneration` | Plan includes categories with actions |
| `TestCompact_Apply_Summarize` | Summarize replaces turns with summary |
| `TestCompact_Apply_Drop` | Drop removes turns |
| `TestCompact_Apply_Pin` | Pin adds to pinned items |
| `TestCompact_Apply_Keep` | Keep preserves turns unchanged |
| `TestCompact_OriginalRetained` | Original turns stored in compacted_summary |
| `TestCompact_CompactModel_Config` | Explicit compact_model routed correctly |
| `TestCompact_CompactModel_Routing` | Empty compact_model resolves via cheap role |
| `TestCompact_CompactModel_MissingCheap` | Missing cheap role → diagnostic |
| `TestCompact_ServerEvent` | auto_compact event recorded |

### WU-062 tests (`transform_test.go`)

| Test | Description |
|------|-------------|
| `TestTransform_Summarize` | Content summarized via cheap model |
| `TestTransform_CostAttribution` | Transform cost separate from turn cost |
| `TestTransform_RawCapture` | Raw request/response captured per ADR-0005 |
| `TestTransform_ModelResolution` | Cheap model resolved from routing |

### WU-063 tests (`diagnostics_test.go`)

| Test | Description |
|------|-------------|
| `TestDiag_AllCodes` | Each of 12 codes emits correct event |
| `TestDiag_Terminal` | Terminal codes trigger ConnFailed |
| `TestDiag_AutoRepair` | Auto-repair result included in event |
| `TestDiag_SuggestedCommand` | Suggested command populated |
| `TestDiag_Integration_ProviderDown` | Provider failure → MT-CONN-009 |
| `TestDiag_Integration_VersionMismatch` | Version mismatch → MT-CONN-004 |
| `TestDiag_Integration_SessionLocked` | Lock contention → MT-CONN-008 |

### WU-064 tests (`recovery_test.go`)

| Test | Description |
|------|-------------|
| `TestIdempotency_TurnSubmit_New` | First submission accepted |
| `TestIdempotency_TurnSubmit_Duplicate` | Duplicate returns existing status |
| `TestIdempotency_TurnSubmit_DuplicateWithSync` | Duplicate includes sync state |
| `TestIdempotency_ToolResult_New` | First result processed |
| `TestIdempotency_ToolResult_Duplicate` | Duplicate acknowledged, not reprocessed |
| `TestSessionSync_NoActiveTurn` | Sync with no active turn returns empty |
| `TestSessionSync_StreamingTurn` | Sync returns streaming status + token count |
| `TestSessionSync_PendingToolResult` | Sync returns pending tool calls |
| `TestSessionSync_MultiModel` | Sync returns branch state |

## Key Files

| Action | Path | WU |
|--------|------|----|
| NEW | `internal/bff/compact.go` | 061 |
| NEW | `internal/bff/transform.go` | 062 |
| NEW | `internal/bff/diagnostics.go` | 063 |
| NEW | `internal/bff/recovery.go` | 064 |
| NEW | `internal/bff/*_test.go` | all |
| MODIFY | `internal/config/config.go` | 061 (context config block) |

## Dependencies Consumed

- `internal/bff/connection.go` (WU-048): `Connection`, state transitions
- `internal/bff/session.go` (WU-050): `ActiveSession`, `SessionManager`
- `internal/bff/conversation.go` (WU-051): `Conversation.Messages()`, `PendingToolCalls()`
- `internal/bff/dispatch.go` (WU-052): `TurnDispatcher.DispatchSync()` for summarization
- `internal/bff/streaming.go` (WU-053): streaming state for session.sync
- `internal/bff/routing.go` (WU-059): `RoutingPolicy.Resolve("cheap")` for compact model
- `internal/bff/branch.go` (WU-060): `BranchManager.BranchState()` for sync
- `internal/protocol/compact.go` (WU-041): `CompactPlan`, `CompactCategory`, `CompactApply`
- `internal/protocol/sessions.go` (WU-041): `SessionSyncResponse`, `ActiveTurnState`
- `internal/protocol/errors.go` (WU-041): `DiagnosticCode` constants, `Diagnostic`
- `internal/storage/store.go` (WU-045): `Store.AppendServerEvent()`

## Interfaces Exported (consumed by downstream WUs)

- **WU-065 (CLI)**: uses `DiagnosticEmitter` for diagnostic reporting in CLI output
- **WU-067 (Integration tests)**: uses all four subsystems in end-to-end flows
- **WU-084 (Session explorer)**: uses `CompactionEngine.GeneratePlan()` for `/compact` UI
