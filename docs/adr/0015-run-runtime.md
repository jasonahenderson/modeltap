---
status: proposed
date: 2026-05-05
decision-makers: Jason Henderson
related:
  - FEAT-0015
  - FEAT-0016
  - FEAT-0017
  - ADR-0014
  - docs/releases/v0.3.0/plan.md
---

# ADR-0015: Run Runtime Ownership and Semantics

## Context

Modeltap currently treats an active harness request as a `turn.submit` flow. The
BFF validates the turn, stores the user turn, chooses a model, dispatches a
provider stream, and emits turn-scoped notifications. The terminal shell already
uses `RunID` in its host-event boundary, but today that identifier is a shell
correlation value rather than a durable BFF-owned runtime record.

FEAT-0015 through FEAT-0017 require that professional work becomes durable,
inspectable, resumable, and policy-aware without turning the harness into a
self-contained agent loop. ADR-0014 keeps the harness as a thin orchestration
client and makes the BFF authoritative for runtime state.

## Decision

The BFF owns run identity, trace identity, lifecycle status, pipeline stage,
attachment state, event sequencing, checkpoint records, stage deadlines, and
run liveness state. The harness owns local execution facts: tool results, local
permission decisions, file previews, executor connection state, heartbeats, and
user commands. The harness reports those facts to the BFF; the BFF integrates
them and emits canonical `run.*` events.

Every `turn.submit` creates or continues a foreground run in v0.3.0. The
existing `turn.submit` method remains supported for compatibility, but the BFF
wraps it in a durable run before provider dispatch. New run-native control and
inspection methods use the `run.*` namespace. `run.create` creates a queued run
without starting provider work; `turn.submit` remains the compatibility path for
creating and immediately starting an attached foreground run.

Canonical run statuses are:

- `queued`
- `running`
- `waiting_permission`
- `waiting_user`
- `checkpointed`
- `completed`
- `failed`
- `cancelled`

Canonical pipeline stages are:

- `preflight`
- `context_plan`
- `prompt_plan`
- `model_call`
- `tool_loop`
- `validation`
- `artifact_capture`
- `checkpoint`
- `completion`

Canonical attachment states are:

- `attached`
- `detached`

Observers are subscriptions, not attachment state. `blocked` is a list grouping
for `waiting_permission` and `waiting_user`, not a lifecycle status.

Canonical user control verbs are `attach`, `detach`, `cancel`, `continue`,
`retry`, and `fork`. `pause` is not a user-facing verb in this release. If a run
cannot proceed, it transitions to `waiting_permission`, `waiting_user`,
`checkpointed`, `failed`, or `cancelled` with a structured reason.

## Executor Availability

Local side-effect execution requires a connected harness or local executor. BFF
stages that do not need local side effects may continue while a run is detached
or while the harness is inside its disconnect grace period. In v0.3.0 those
BFF-safe stages are `preflight`, `prompt_plan`, in-flight `model_call` until it
requires a local tool result, `artifact_capture` over already captured metadata,
`checkpoint`, and `completion`.

`tool_loop` and `validation` pause when the required executor is disconnected.
The BFF transitions the run to `waiting_user` with reason
`executor_disconnected` unless a future FEAT-0021 policy explicitly permits a
server-safe tool surface. v0.3.0 does not simulate local side effects server
side.

If an attached harness disconnects, the BFF starts a 60 second default grace
period. During that period, BFF-safe work may continue. If unresolved local tool
calls remain past the deadline, the run transitions to `failed` with reason
`executor_disconnected_during_tool_call`. Later reported tool results are stored
as forensic events and are not fed back into the active model loop.

## Checkpoint Semantics

The BFF writes a checkpoint after every legal stage transition and before every
terminal transition. A checkpoint stores enough metadata to inspect, retry,
continue, or fork later, but v0.3.0 retry/continue may be shallow if a stage
cannot safely replay.

Minimum checkpoint fields:

- `checkpoint_id`
- `run_id`
- `sequence`
- `stage`
- `status`
- `reason`
- `turn_ids`
- `model_call_ids`
- `pending_tool_call_ids`
- `last_event_seq`
- `workflow_type`
- `attachment_state`
- `summary`
- `created_at`
- `schema_version`
- extension JSON for context, artifacts, policy, workspace, memory, and routing

Checkpoint writes are part of the same SQLite transaction as the lifecycle event
that advances the run state. Fsync follows the repository's existing SQLite WAL
storage contract: the durability boundary is SQLite transaction commit. v0.3.0
does not add manual fsync calls outside SQLite.

v0.3.0 checkpoint records use `schema_version = 1`. v0.3.0 readers must accept
version 1 records. Subsequent minor releases that change checkpoint payloads
must read at minimum v0.3.0 version 1 records or provide an explicit migration.

Checkpoint payloads may reference the current `runs` row for fields that are
also stored on the run, including `workflow_type` and attachment state. They do
not need to duplicate those fields as checkpoint table columns.

## Event Ordering

Run events are append-only and monotonically sequenced per `run_id`. Essential
events must not be silently dropped: lifecycle, stage transition, blocked,
unblocked, tool request, tool result, checkpoint, artifact, and terminal events.
Progress events may be coalesced when buffers overflow, but the BFF must either
preserve essential events or pause/fail the run according to liveness policy.

Reattach requests include the last observed sequence. If replay has a gap, the
BFF returns a summary plus the latest checkpoint and marks replay fidelity as
partial.

## Liveness and Observability

Every run has a BFF-generated opaque `trace_id`. The trace ID is recorded on the
run row and propagated through BFF logs, harness/executor requests, and
model-provider dispatch metadata where the provider adapter can carry it safely.

Attached harnesses and local executors send run-scoped heartbeats. v0.3.0 uses
heartbeats for attached foreground runs and detached runs that still have local
tool execution in progress. Missing heartbeats do not fail a completed or idle
detached run, but they can move active local-executor work to `waiting_user` or
`failed` according to the executor-disconnect rules above.

v0.3.0 defines stage warning thresholds and hard deadlines as runtime policy
values. Defaults are implementation constants unless later config work moves
them to user config:

| Stage | Warning threshold | Hard deadline |
|---|---:|---:|
| `model_call` | 120 seconds | 600 seconds |
| `tool_loop` | 120 seconds without tool-result progress | 900 seconds |

When an active stage exceeds its hard deadline, the BFF appends a
`run.stage_timeout` event and transitions the run to `failed` with reason
`stage_timeout`. Waiting states (`waiting_permission` and `waiting_user`) use
their own timeout policies and are not failed by active-stage deadlines in
v0.3.0.

## Workflow Type

Run records include `workflow_type`, defaulting to `implementation`. v0.3.0
stores the FEAT-0015 enum so downstream releases can use it without schema
churn:

- `exploration`
- `feature`
- `adr`
- `release`
- `implementation`
- `debug`
- `docs`
- `devops`

Unknown workflow types are rejected at run creation in v0.3.0. Future workflow
profiles may add aliases, but persisted run rows use this canonical enum until
a later ADR revises it.

## Run Families

`parent_run_id` is reserved in v0.3.0 for future forks, sibling runs, and
synthesis runs. Run-family budgets, inherited deadlines, and cancellation
cascade behavior are explicitly deferred until a later release introduces
child/sub-agent execution. v0.3.0 cancellation affects only the addressed run
unless a later accepted design revises the family policy.

## Consequences

- Good: durable runs can be added without breaking existing `turn.submit`
  clients.
- Good: downstream context, artifact, policy, memory, and routing releases get
  stable extension points.
- Good: background behavior is conservative by default; disconnected local
  writes do not proceed silently.
- Bad: v0.3.0 cannot provide full autonomous detached local-tool execution.
- Bad: retry and continue are initially constrained by checkpoint fidelity and
  provider/tool replay limits.

## Confirmation

This ADR is accepted when v0.3.0 designs show:

1. `turn.submit` creates a durable run before model dispatch.
2. Run storage includes status, stage, attachment state, workflow type,
   sequenced events, and checkpoints.
3. `run.*` protocol methods cover inspect, list, attach, detach, cancel,
   retry, continue, fork, and replay.
4. Harness projection keeps detached run transcript events separate from the
   active foreground transcript.
5. Tests cover executor disconnect, event replay gaps, and
   `waiting_permission` versus `waiting_user`.
6. Tests cover trace ID persistence, heartbeat liveness, stage timeout, and
   checkpoint schema-version compatibility.
