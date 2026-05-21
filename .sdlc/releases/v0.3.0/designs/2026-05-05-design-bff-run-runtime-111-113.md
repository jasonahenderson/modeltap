# 2026-05-05 - Design: BFF Run Runtime (WU-111 to WU-113)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Scope

This design covers:

- WU-111: BFF run registry and lifecycle store
- WU-112: `turn.submit` to foreground-run integration
- WU-113: pipeline stage/status emission and checkpoint metadata

It depends on WU-108 through WU-110.

## Package Shape

Add `internal/bff/run_runtime.go` and supporting files:

- `run_types.go`: BFF runtime structs and closed constants
- `run_registry.go`: in-memory active-run registry and cancellation hooks
- `run_store.go`: thin adapter around `internal/storage` run APIs
- `run_events.go`: helper methods for canonical event append/emit
- `run_controls.go`: handlers for `run.*` methods

Keep provider dispatch in the existing `dispatch.go` path. The run runtime wraps
dispatch; it does not replace provider adapters.

## Runtime Registry

`Server` gains:

```go
type Server struct {
    ...
    runs *runRegistry
}
```

`runRegistry` tracks active cancellation and attachment state for in-flight runs:

```go
type activeRun struct {
    id string
    sessionID string
    attachedConnectionID string
    cancel context.CancelFunc
    lastSeq int64
}
```

The registry is not the source of truth. SQLite is. On startup or reconnect,
the BFF reconstructs summaries from storage.

## `turn.submit` Flow

`handleTurnSubmit` changes from "append user turn, dispatch provider" to:

1. validate `TurnSubmit`
2. ensure session and access, preserving existing behavior
3. derive idempotency key:
   - new optional request field if present
   - else `turn:<session_id>:<turn_id>` for compatibility
4. open one durable pre-dispatch transaction
5. create a run row if none exists for that key
6. append the user turn and command history
7. link the user turn to the run
8. append `run.started` and initial checkpoint
9. commit the transaction
10. emit `run.stage_changed` for `preflight`
11. resolve model and emit `run.stage_changed` for `prompt_plan`
12. emit `run.stage_changed` for `model_call`
13. dispatch provider stream
14. register run cancellation
15. return accepted response with optional `run_id`

If any part of the pre-dispatch transaction fails, no provider call starts and
no partial run/turn/link state is committed.

`run.create` uses the same run creation and idempotency path but stops after the
queued run, initial event, and initial checkpoint are durable. It does not append
a user turn and does not dispatch a provider call.

## Stream Relay Integration

`NewStreamRelay` currently emits turn events. WU-113 wraps or extends it so
provider stream facts also append run events:

- model selection: `run.progress` payload type `model_selected`
- token deltas: optional non-essential `run.progress`
- tool calls: `run.tool_call_requested`
- tool results: `run.tool_result_recorded`
- usage/cost: `run.progress` payload type `usage_update`
- completion: `run.completed`
- provider error: `run.failed`
- cancellation: `run.cancelled`

Existing turn events remain emitted for compatibility.

Model-call accounting is recorded through `run_model_calls` by
`model_call_id`. Tool-result delivery is recorded through `run_tool_results` by
`tool_call_id`. Duplicate reports return the stored state and do not double-count
usage or re-enter the model loop.

The v0.3.0 tool loop is sequential. Tool calls are processed in provider
emission order, and mutating operations are serialized. Parallel read-only tool
resolution is deferred until policy and validation work can classify operations
reliably.

## Status and Stage Transitions

Required v0.3.0 transitions:

- create: `queued` + `preflight`
- accepted foreground dispatch: `running` + `preflight`
- prompt assembly: `running` + `prompt_plan`
- provider dispatch: `running` + `model_call`
- tool call pending: `waiting_permission` or `running` + `tool_loop`
- local executor disconnected: `waiting_user` + current stage
- stage deadline exceeded: `failed` + current stage with reason
  `stage_timeout`
- provider complete: `completed` + `completion`
- cancel requested: `cancelled` + current stage or `completion`
- provider/tool fatal error: `failed` + current stage
- explicit checkpoint without active work: `checkpointed` + `checkpoint`

`waiting_permission` and `waiting_user` are first-class states. Command handlers
and list filters must distinguish them.

Inactive downstream stages:

- `context_plan`: emitted as skipped/no-op with reason `not_enabled_v0.3.0`
  only when the run summary needs the full stage list.
- `validation`: same.
- `artifact_capture`: same, except minimal usage/checkpoint metadata may be
  recorded as run events, not FEAT-0020 artifacts.

Legal reentry edges:

| Edge | v0.3.0 status |
|---|---|
| `tool_loop -> prompt_plan` | active; tool results can trigger prompt assembly before the next provider call |
| `model_call -> preflight` | active for retryable provider dispatch failures or policy changes before retry |
| `validation -> model_call` | no-op/inactive until v0.3.2 |
| `validation -> prompt_plan` | no-op/inactive until v0.3.2 |
| any active stage -> terminal | active |

## Checkpoints

Write checkpoints after:

- run create
- every stage change
- every waiting state
- terminal transition

Checkpoint payload includes:

- turn IDs linked so far
- model/provider selected so far
- pending tool calls
- last event sequence
- usage totals
- extension JSON placeholders

Checkpoint writes are synchronous with lifecycle storage updates. Non-essential
progress events do not require checkpoints.

## Run Controls

Handlers:

- `handleRunList`
- `handleRunDetails`
- `handleRunAttach`
- `handleRunDetach`
- `handleRunCancel`
- `handleRunRetry`
- `handleRunContinue`
- `handleRunFork`
- `handleRunEvents`
- `handleRunPermissions`
- `handleRunResolvePermission`
- `handleRunCreate`
- `handleRunHeartbeat`

`run.cancel` uses the same cancellation function as `turn.cancel`. `turn.cancel`
becomes a compatibility wrapper that locates the owning run for the turn and
then calls the run cancellation path.

`run.retry`, `run.continue`, and `run.fork` are implemented conservatively:

- reject terminal states that cannot replay
- allow continuing `waiting_permission`/`waiting_user` only when the blocking
  reason is resolved
- fork creates a new queued run with `parent_run_id` and copied summary, but no
  workspace/artifact cloning

## Cost, Usage, and Model Selection

Run records aggregate:

- final input tokens
- final output tokens
- total cost
- model
- provider
- latency when available

Aggregation happens from existing `CostTracker` and `TurnComplete` facts. Model
selection is recorded before provider dispatch. If dispatch fails before
selection is stable, the run fails with empty model/provider and a structured
reason.

## Tests

- `turn.submit` persists a run before provider dispatch
- existing turn response/event compatibility remains intact
- run lifecycle events have contiguous sequence numbers
- `waiting_permission` and `waiting_user` list/detail filters differ
- cancellation through `turn.cancel` and `run.cancel` reaches the same active
  run
- provider completion updates run totals and terminal checkpoint
- disconnected executor transitions to `waiting_user`
- duplicate model-call accounting and tool-result delivery are idempotent
- permission resolution transitions `waiting_permission` runs correctly
- `run.create` persists a queued run without provider dispatch
- `run.heartbeat` updates liveness and missing heartbeat can drive executor
  disconnect behavior
- `model_call` or `tool_loop` hard deadline transitions to `failed` with
  reason `stage_timeout`
- `tool_loop -> prompt_plan` reentry is recorded as a legal transition
