---
feature: FEAT-0017
title: Durable Runs and Background Agents
status: accepted
date: 2026-04-29
parent: FEAT-0015
series: Professional Harness Runtime
series-role: member
series-order: 2
depends-on:
  - FEAT-0016: Managed Codegen Run Pipeline
adr-constraints:
  - ADR-0014: Harness Base Strategy
---

# FEAT-0017: Durable Runs and Background Agents

## Problem

Users need to run long or parallel work without blocking the active terminal
conversation: implementation attempts, documentation tasks, reviews, release
artifact drafting, and debugging investigations. Today the harness primarily
models one active turn stream. If the user disconnects, switches context, or
wants multiple bounded tasks in flight, there is no durable job surface.

Background agents should not become a separate architecture from foreground
agents. The same run engine should handle both; the difference is whether the
user is attached to the run and whether the run can ask interactive questions.

## Solution

Represent foreground and background work as durable runs with attachment state.
A foreground run is attached and owns the active conversation surface. A
background run is detached or non-focused, appears in a run queue, and continues
under explicit permission and workspace policy.

The BFF stores run state and progress. The harness renders the attached run in
the conversation shell and exposes a jobs/runs surface for detached work.

## Key Capabilities

### Attachment Semantics

Runs use the FEAT-0015 attachment vocabulary:

- `attached`: the run owns the current foreground transcript/composer.
- `detached`: the run continues without owning the foreground surface.

Observers are per-client subscriptions. A client may observe a run without
holding the attachment lease or composer. `blocked` is only a queue/UI grouping
for runs in `waiting_permission` or `waiting_user`.

Users can attach, detach, cancel, continue, retry, or fork a run without losing
its transcript or artifacts.

Attachment state is BFF-authoritative. A run has at most one attached client at
a time; additional connected clients may observe it. The harness requests
attachment through `/attach <run-id>`, and the BFF grants or rejects the request
with a reason such as already attached elsewhere, terminal run, or unavailable
run. If an attached harness disconnects without detaching, the BFF moves the run
to `detached` after a configurable grace period, defaulting to 60 seconds.
During the grace period, the run may continue only according to its configured
background-policy posture for stages that do not require local side effects.
Stages that may continue without the disconnected executor include
`prompt_plan`, in-flight `model_call`, and `artifact_capture` over already
captured content. `tool_loop` and `validation` pause when they require the
disconnected executor. The server-safe tool surface is owned by FEAT-0021 and
locked by the run-runtime ADR.

Attachment never auto-promotes from an observer. When an attached client misses
the grace period, the run becomes `detached`; clients must explicitly attach.
The first valid attach claim wins, and concurrent attach requests are serialized
BFF-side.

Attachment state is orthogonal to run status and pipeline stage. A run can be
`running` while `detached`, `waiting_permission` while `attached`, or
`waiting_user` while `detached`. `blocked` is a UI grouping for runs whose
status is `waiting_permission` or `waiting_user`; it is not a separate lifecycle
status.

### Background Permission Behavior

Background runs cannot assume the user is available.

When a background run requests a side effect outside its pre-approved policy,
the runtime must do one of:

- pause in `waiting_permission`
- auto-deny and continue if the workflow permits
- fail with a policy error

The default for mutating operations should be pause, not silent mutation.

When a run requires a local side-effect tool and no harness/local executor is
connected, the run pauses with status `waiting_user` and an executor-disconnected
reason. BFF-only stages may continue when they do not simulate local side
effects, including prompt/context planning, routing, model calls that can
complete without tool calls, and summarization over already captured artifacts.
The precise BFF-safe tool surface is enumerated by the run-runtime ADR.

### Run Queue

The harness exposes a compact queue of active and recent runs:

- queued
- running
- blocked
- failed
- completed
- cancelled

Each row should include workflow, title/summary, model or agent label, elapsed
time, cost when available, current pipeline phase mapped from FEAT-0016 stages,
and whether user input is required. A `stuck` badge is shown when a run has not
advanced stage or event sequence for a configurable interval.

### Detached Transcripts

Background runs keep separate transcripts from the active foreground transcript.
Attaching to a run shows its run transcript and artifacts. Returning to the main
conversation should not merge unrelated background chatter into the foreground
surface.

Run transcript events do not append directly to the session transcript. The
harness composes the foreground view from the active session transcript plus,
when attached or observing, the selected run's transcript stream.

### Resume After Restart

If the harness restarts while the BFF is still available, the user can list
active runs for the current session and reattach. The BFF replays run events from
the last observed sequence number when the events are within retention. If full
replay is unavailable, the BFF returns a summary plus the latest checkpoint with
a visible fidelity note.

Tools already running when a harness disconnects may complete locally. On
reconnect, the harness reports each terminal result. If the run reaches the
grace-period deadline without reconnect while such a tool is unresolved, the BFF
transitions the run to `failed` with reason
`executor_disconnected_during_tool_call`; later reported results are retained as
forensic artifacts and are not fed back into the active model loop.

## UI / CLI / API Integration

Expected harness commands:

- `/runs` or `/jobs`
- `/attach <run-id>`
- `/detach`
- `/cancel <run-id>`
- `/continue <run-id>`
- `/retry <run-id>`
- `/fork <run-id>`
- `/permissions` for blocked run requests

`/cancel <run-id>` follows FEAT-0015 cancellation and workspace lifecycle rules:
captured artifacts are retained, terminal-state workspace cleanup is triggered,
and sibling runs are not cancelled unless the workflow declares cascade-on-cancel.

Expected protocol/API surfaces:

- run list
- run details
- attach/detach
- cancel/pause/resume
- stream run events from a checkpoint
- resolve pending permission for a run

## Configuration

Configuration should support:

- maximum concurrent background runs
- default background permission behavior
- blocked-run retention
- completed-run retention
- notification behavior for blocked or completed runs
- background scheduling policy and stuck-run thresholds

The default background scheduler is FIFO with explicit no-priority semantics in
the first slice. Foreground-promoted runs do not consume background slots. Future
policy may add workflow priority and run-age boosts.

Blocked-run notifications fire within a bounded delay, defaulting to five
seconds, after `waiting_permission` or `waiting_user`. Identical-cause
notifications coalesce. Completed-run notifications are best-effort. Notification
content defaults to title-only per surface; richer content is opt-in per surface
and workflow.

Run retention follows the FEAT-0015 retention envelope. Artifact and transcript
retention must not make a recent run inspectable only as metadata with missing
required evidence.

## Non-Goals

- Do not create an unbounded autonomous swarm model.
- Do not require every background run to use a Git worktree.
- Do not bypass local tool permission enforcement.
- Do not replace FEAT-0013 Agent Teams; a team can be one kind of run.
- Do not define the exact server-safe tool list here; FEAT-0021 owns that slot.

## Success Criteria

Acceptance scope: v0.3.0 implements the durable-run foundation slice: run
identity, attachment/detach/list/reattach behavior, separate detached
transcripts, restart/replay behavior, and shared lifecycle semantics. Background
blocked-operation policy, the full permission/input inbox, and isolated writer
workspace behavior remain deferred to v0.3.3+ as recorded in the v0.3.0 release
plan.

1. A user can start a run, detach it, list it, and reattach later.
2. A detached run has a separate transcript and artifact list.
3. A background run that needs unapproved side effects pauses or follows an
   explicit auto-deny policy.
4. Blocked background runs surface in a visible permission/input inbox.
5. Harness restart does not erase BFF-known active runs.
6. Foreground runs and background runs share the same run lifecycle and
   artifact model from FEAT-0016.

## Relationship to ADRs

| ADR | Relationship |
|---|---|
| ADR-0014 | Requires the terminal harness to remain the universal orchestration client |
| ADR-0015 | Decides durable run persistence, attachment grace-period policy, executor-disconnect behavior, and the no-server-side-local-effects rule for v0.3.0 |

## Resolved or Deferred for v0.3.0

1. What configurable grace-period values should apply before an attached run is
   marked detached after harness disconnect? Default: 60 seconds in ADR-0015.
2. What is the default behavior for background write requests in solo mode?
   Deferred to FEAT-0021/v0.3.3 policy work; v0.3.0 does not silently execute
   disconnected local side effects.
3. Should completed background transcripts be merged, summarized, or only linked
   from the foreground session? v0.3.0 keeps detached transcripts separate and
   does not merge unrelated background chatter into the foreground surface.
