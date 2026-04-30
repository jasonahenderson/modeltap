---
feature: FEAT-0017
title: Durable Runs and Background Agents
status: draft
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

Runs have attachment state:

- `attached`: the run owns the current foreground transcript/composer.
- `detached`: the run continues without owning the foreground surface.
- `observable`: the user watches progress but does not interact.
- `blocked`: the run needs permission or user input.

Users can attach, detach, cancel, continue, retry, or fork a run without losing
its transcript or artifacts.

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

### Run Queue

The harness exposes a compact queue of active and recent runs:

- queued
- running
- blocked
- failed
- completed
- cancelled

Each row should include workflow, title/summary, model or agent label, elapsed
time, cost when available, current stage, and whether user input is required.

### Detached Transcripts

Background runs keep separate transcripts from the active foreground transcript.
Attaching to a run shows its run transcript and artifacts. Returning to the main
conversation should not merge unrelated background chatter into the foreground
surface.

### Resume After Restart

If the harness restarts while the BFF is still available, the user can list
active runs and reattach. The BFF may replay full events, summarize missed
events, or show checkpointed state depending on retention and protocol support.

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

## Non-Goals

- Do not create an unbounded autonomous swarm model.
- Do not require every background run to use a Git worktree.
- Do not bypass local tool permission enforcement.
- Do not replace FEAT-0013 Agent Teams; a team can be one kind of run.

## Success Criteria

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
| Future ADR | Should decide durable run persistence, attach/detach semantics, and local executor availability |

## Open Questions

1. Can background runs continue local tool execution if no harness/local
   executor is connected?
2. Should the BFF provide server-side tools for background-safe operations?
3. What is the default behavior for background write requests in solo mode?
4. Should completed background transcripts be merged, summarized, or only linked
   from the foreground session?
