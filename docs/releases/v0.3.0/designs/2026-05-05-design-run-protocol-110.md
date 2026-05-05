# 2026-05-05 - Design: Run Protocol Methods and Events (WU-110)

## Scope

This design covers WU-110:

- `run.*` JSON-RPC methods
- `run.*` event taxonomy
- compatibility with existing `turn.*` traffic
- replay and gap semantics

It does not define storage internals or shell rendering.

## Protocol Placement

Add `internal/protocol/runs.go` for request/response payloads and
`internal/protocol/run_events.go` for notification payloads. Keep existing
`turn.*` structs unchanged unless WU-112 requires additive optional fields.

## Methods

New method constants:

- `run.list`
- `run.details`
- `run.attach`
- `run.detach`
- `run.cancel`
- `run.retry`
- `run.continue`
- `run.fork`
- `run.events`
- `run.permissions`
- `run.resolve_permission`

`turn.cancel` remains supported and maps to the active run containing the turn.
New harnesses should call `run.cancel`.

### `run.list`

Request:

```go
type RunList struct {
    SessionID string `json:"session_id,omitempty"`
    Status    string `json:"status,omitempty"`
    Limit     int    `json:"limit,omitempty"`
    Offset    int    `json:"offset,omitempty"`
}
```

Response:

```go
type RunListResponse struct {
    Runs []RunSummary `json:"runs"`
}
```

`RunSummary` includes run ID, session ID, title, workflow type, status, stage,
attachment state, model/provider, elapsed timestamps, cost, token totals, and
whether input is required. It also includes:

- `last_advanced_at`
- `stuck`
- `stuck_seconds`

`input_required` is true when status is `waiting_permission` or `waiting_user`.
`stuck` is true when the run is non-terminal and has not advanced event
sequence or stage since the configured stuck threshold, defaulting to 300
seconds in v0.3.0.

### `run.details`

Request by `run_id`. Response includes summary metadata, turn IDs, latest
checkpoint, and a bounded tail of events.

### `run.attach` and `run.detach`

Attach request includes `run_id` and optional `last_observed_seq`. Attach
response returns the granted attachment state, replay fidelity, replayed events
or summary, and latest checkpoint. Detach returns the resulting state.

### `run.cancel`, `run.retry`, `run.continue`, `run.fork`

All are control requests addressed by `run_id` and optional `reason`.

v0.3.0 semantics:

- `cancel` is real and cooperative.
- `retry` is checkpoint-aware but may reject unsupported stages.
- `continue` resumes only from supported waiting/checkpointed states.
- `fork` creates a sibling run record with parent link and copied summary
  metadata; full workspace/artifact clone behavior is deferred.

### `run.events`

Request:

```go
type RunEvents struct {
    RunID    string `json:"run_id"`
    AfterSeq int64  `json:"after_seq"`
    Limit    int    `json:"limit,omitempty"`
}
```

Response returns ordered events, `latest_seq`, `has_more`, `replay_available`,
and `fidelity` (`full` or `summary`).

### `run.permissions`

Lists pending permission or user-input blockers for one run, or for active runs
in the current session when `run_id` is omitted. v0.3.0 uses this for foreground
and attached-run permission resolution; the full blocked-run inbox remains in
v0.3.3 WU-144.

### `run.resolve_permission`

Applies a permission decision to a pending run-correlated request:

```go
type RunResolvePermission struct {
    RunID     string `json:"run_id"`
    RequestID string `json:"request_id"`
    Decision  string `json:"decision"`
}
```

The BFF records the decision, appends a run event, and transitions out of
`waiting_permission` when all pending permission blockers for the run are
resolved. Non-permission `waiting_user` resolution remains command-specific in
v0.3.0.

## Events

New notification method constants:

- `run.started`
- `run.stage_changed`
- `run.status_changed`
- `run.progress`
- `run.tool_call_requested`
- `run.tool_result_recorded`
- `run.artifact_recorded`
- `run.checkpoint_recorded`
- `run.attached`
- `run.detached`
- `run.completed`
- `run.failed`
- `run.cancelled`

Every run event payload includes:

- `run_id`
- `seq`
- `session_id`
- `turn_id,omitempty`
- `stage,omitempty`
- `status,omitempty`
- `created_at`

`run.progress` may be coalesced. All other events are essential and must not be
silently dropped.

## `turn.submit` Compatibility

The response to `turn.submit` remains:

```go
type TurnSubmitResponse struct {
    TurnID string
    SessionID string
    Status string
}
```

WU-112 may add optional `run_id` with `omitempty`. Existing clients ignore it.
The BFF also emits `run.started` after acceptance for v0.3.0 clients.

Existing turn-scoped events (`token.delta`, `tool.call`, `cost.update`,
`model.selected`, `turn.complete`) continue during v0.3.0. WU-112/WU-113 emit
matching run events in parallel. Later releases may make the harness consume
only run events, but v0.3.0 must preserve turn event compatibility.

## Error Semantics

Control methods return JSON-RPC errors for unknown run, unauthorized user,
terminal run, invalid transition, or attachment conflict. Attachment conflicts
must name whether the run is attached elsewhere or terminal.

Replay gaps are not errors. The response marks fidelity as `summary` and
includes latest checkpoint metadata.

## Tests

- protocol JSON fixtures for every `run.*` method and essential event
- unknown fields do not break existing `turn.submit`
- event sequence fixtures reject missing `run_id` or non-monotonic sequences
- attach replay returns full event list when retained and summary when gapped
- permission resolution transitions a run out of `waiting_permission`
- run summaries expose `input_required` and `stuck`
