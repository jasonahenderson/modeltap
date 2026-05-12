---
feature: FEAT-0016
title: Managed Codegen Run Pipeline
status: accepted
date: 2026-04-29
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 1
depends-on:
  - FEAT-0008: BFF Server
  - FEAT-0009: Terminal Harness
  - FEAT-0014: Harness Conversation Shell
adr-constraints:
  - ADR-0014: Harness Base Strategy
---

# FEAT-0016: Managed Codegen Run Pipeline

## Problem

Modeltap can submit a turn, stream a model response, and mediate tool calls, but
an implementation request is still mostly treated as a chat turn. Professional
coding work needs a managed transaction: the system should know what stage the
work is in, what context and policy apply, which tools ran, what changed, what
validation proved, and how to recover if the run fails or is cancelled.

Without a managed pipeline, context planning, validation, patch artifacts,
background runs, and memory promotion become ad hoc features with incompatible
state models.

## Solution

Define code generation as a durable run pipeline owned primarily by the BFF and
surfaced by the harness. Each implementation or debug run moves through explicit
stages, but the pipeline is a state graph rather than a one-shot linear
sequence:

`preflight -> context_plan -> prompt_plan -> model_call -> tool_loop -> validation -> artifact_capture -> checkpoint -> completion`

The BFF stores lifecycle metadata and emits progress events. The harness renders
the active stage compactly, enforces local side-effect policy, executes local
tools, and lets the user inspect or recover the run.

The normal happy path follows the sequence above. The run runtime must also
support these legal reentry edges:

- `tool_loop -> prompt_plan` when tool results or policy require prompt/context
  revision before another provider call.
- `validation -> model_call` when validation feedback starts a repair turn.
- `validation -> prompt_plan` when repair requires revised context or prompt
  layers.
- `model_call -> preflight` when provider failure or policy change forces a
  restart of the dispatch plan.
- terminal transitions from any active stage to `completion`, `failed`, or
  `cancelled` according to run policy.

Transition policy, retry limits, and idempotency boundaries should be decided by
the run-runtime ADR.

## Key Capabilities

### Run Lifecycle

The pipeline must support these stage states. Stages are available pipeline
states, not mandatory work for every run. Each workflow type declares required
and optional stages; skipped stages are recorded with a reason. A simple
foreground chat run may use only `preflight`, `model_call`, and `completion`.

- `preflight`: resolve workflow, mode, model policy, user/project policy, and
  workspace.
- `context_plan`: assemble the files, rules, memory, and user-provided context
  intended for the turn.
- `prompt_plan`: assemble prompt layers and tool definitions with budget
  metadata.
- `model_call`: dispatch to the selected model or agent. Persist at minimum the
  prompt-layer plan, assembled tool-definition metadata, model identity and
  parameters, response stream events, tool-call envelopes, and provider usage.
  Full prompt-content capture is configurable and subject to artifact redaction
  policy.
- `tool_loop`: process tool calls and tool results until the model stops or
  policy blocks. During streaming, `model_call` and `tool_loop` may overlap:
  tool calls can be requested while response content is still arriving.
- `validation`: run or record checks when the workflow requires validation.
- `artifact_capture`: collect patch, command, validation, approval, and cost
  evidence.
- `checkpoint`: persist enough state to continue, retry, fork, or inspect. Every
  legal stage transition also produces a checkpoint; this stage is the final
  pre-completion checkpoint.
- `completion`: record final outcome and summary.

Parallel tool calls within a turn are recorded in provider emission order and
resolve independently by `tool_call_id`. The default execution policy may run
independent read-only or server-safe calls concurrently, but mutating workspace
operations are serialized unless policy explicitly permits safe parallel
execution. Each tool call resolves to one of `success`, `failure`, `denied`,
`timeout`, or `cancelled`; the loop continues until the model stops or policy
halts the run. A run-level `tool_loop` failure is emitted only when the model or
policy determines that the run cannot proceed.

Checkpoints are written atomically through the BFF's durable store before the
runtime advances to the next stage. Exact transaction/fsync boundaries and
minimum checkpoint data belong in the run-runtime ADR.

Every run belongs to exactly one session. A session may contain zero or more
runs, and run-level operations addressed by `run_id` are separate from
session-level operations addressed by `session_id`.

A run is created by an initiating event, typically a user turn but also a parent
run fork, synthesis aggregation request, or system-scheduled trigger. The run
records the initiator type as metadata. A user-initiated run may consume
additional turns during its lifecycle, such as repair turns, validation triage
turns, and clarification turns. Each turn within the run has its own `turn_id`;
the run records the ordered list of `turn_id`s it owns. A turn cannot belong to
more than one run. Repair, validation triage, and clarification turns reenter
`model_call`, or `prompt_plan` when context or prompt layers must be revised,
within the same `run_id`.

### BFF Responsibilities

- create a run ID before dispatching model work
- persist run stage transitions
- assemble or reference context and prompt plans
- route the model call
- correlate stream events, tool calls, cost, and provider usage with the run
- store checkpoint metadata
- expose run details through protocol/API methods

Cost and usage attribution is recorded at three levels: per `model_call_id`
with provider, model, tokens, cost, and latency; per `tool_call_id` with
executor, duration, and outcome; and per pipeline stage as aggregated totals.
Run-level totals are derived from those lower-level records.

Run status and pipeline-stage transitions are BFF-authoritative. The harness
reports facts such as tool results, user actions taken locally, local errors,
and executor disconnects; the BFF integrates those reports and is the sole
emitter of status and stage change events. Harness rendering reflects the last
BFF-emitted state. Harness commands such as cancel, retry, continue, and
fork are requests to the BFF; the BFF acknowledges and emits the resulting
transition or rejects the request with a reason.

### Harness Responsibilities

- render active run stage and status without noisy transcript chatter
- execute local tools only through the run's policy/workspace context
- surface inspectable summaries for context, prompt, policy, and artifacts
- provide cancel, retry, continue, and fork actions against the run ID
- preserve FEAT-0014 shell semantics by consuming run events through the host
  boundary

### Pipeline Events

The protocol should expose structured run lifecycle events. Exact names are a
design detail, but the harness needs:

- run started
- stage changed
- stage progress update
- tool call requested
- tool result recorded
- artifact recorded
- checkpoint recorded
- run completed, failed, cancelled, or blocked

BFF-to-harness event streams use bounded buffering. Overflow may coalesce or drop
older non-essential progress updates, but must not drop stage transitions,
tool-request/result events, artifact events, checkpoints, or terminal events.
If essential-event buffering is exhausted, the BFF pauses upstream streaming or
transitions the run according to liveness policy rather than silently losing
events.

## UI / CLI / API Integration

The harness should show the current stage in status chrome and expose details on
demand:

- `/run` shows the active run summary
- `/run context` shows the context plan summary
- `/run prompt` shows prompt-layer metadata
- `/run policy` shows active policy and workspace
- `/cancel`, `/continue`, `/retry`, and `/fork` operate on the active run

The BFF protocol needs run-aware submit and inspection surfaces, either by
extending `turn.submit` or adding run-specific methods.

`/run` is singular and always refers to the currently attached run. `/runs` or
`/jobs` is the plural queue/list surface defined by FEAT-0017.

## Configuration

Configuration should allow:

- default pipeline behavior per workflow type
- checkpoint cadence and retention
- per-stage warning thresholds and hard deadlines
- whether prompt-layer metadata is visible by default

Stage warning thresholds are lower than hard deadlines. Deadline expiry
transitions the run according to policy, defaulting to `failed` with reason
`stage_timeout` for active stages. `waiting_permission` and `waiting_user` use
the umbrella runtime's permission and user-input timeout policies rather than
active-stage deadlines.

## Non-Goals

- This feature does not define background queue UX; see FEAT-0017.
- This feature does not define repository context selection; see FEAT-0018.
- This feature does not define validation planning; see FEAT-0019.
- This feature does not require multi-agent teams.

## Success Criteria

1. An implementation request creates a stable run ID before model dispatch.
2. The BFF records and emits stage transitions for the run.
3. Tool calls, model selection, cost, and terminal outcome are correlated with
   the run ID.
4. The harness renders stage/status and can inspect run metadata on demand.
5. Cancel, retry, continue, and fork actions address a run ID rather than
   transcript-only state.
6. Existing simple chat remains compatible and can be represented as a
   foreground run.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Requires BFF-first orchestration with the harness as universal orchestration client |
| ADR-0015 | Decides run ownership, lifecycle terminology, transition policy, checkpoint schema, event buffering, executor availability, and v0.3.0 liveness semantics |

## Resolved for v0.3.0

1. `turn.submit` remains compatible and becomes run-aware; new control and
   inspection surfaces use `run.*`, including `run.create`.
2. Minimum checkpoint data is defined by ADR-0015 and WU-109/WU-113.
3. Simple attached chat is represented as a lightweight foreground run using
   only the stages it needs.
4. Full prompt metadata visibility is deferred to v0.3.1 prompt/context
   planning and artifact redaction policy.
