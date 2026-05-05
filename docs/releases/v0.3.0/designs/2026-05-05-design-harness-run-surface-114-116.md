# 2026-05-05 - Design: Harness Run Surface (WU-114 to WU-116)

## Scope

This design covers:

- WU-114: harness run projection and active `/run` surface
- WU-115: run list, attach, detach, cancel, retry, continue, fork commands
- WU-116: reconnect/resume behavior for active and detached runs

It builds on the existing `internal/harnessshell` typed event boundary and
`internal/harnesshost.ProductionRuntime`.

## Current Baseline

The shell already consumes host events with `RunID`:

- `SubmissionAcceptedEvent`
- `RunStartedEvent`
- `RunDeltaEvent`
- `RunCompletedEvent`
- `RunStoppedEvent`
- `RunFailedEvent`

The host runtime exposes `SubmitTurn`, `InterruptRun`, and
`DispatchCommand`. v0.3.0 makes the `RunID` in those events the BFF run ID and
adds host commands that call `run.*` protocol methods.

## Projection

Add `internal/harnesshost/run_projection.go`:

- maps `protocol.RunStartedEvent` to `harnessshell.RunStartedEvent`
- maps essential run terminal events to completed/stopped/failed shell events
- maps permission/waiting events to permission or status shell events
- maintains a per-run transcript buffer for detached runs

Turn token events remain projected for the attached foreground run. Detached
run deltas are not appended to the active foreground transcript. They are stored
in a per-run projection buffer keyed by run ID until the user attaches or
inspects the run.

## Detached Transcript Invariant

The foreground transcript is composed from:

1. the active session transcript
2. the attached run transcript, if any

Detached run transcript events never append directly to the foreground session
transcript. `/runs` may show compact status rows, but it must not inject
background assistant chatter into the main conversation.

Tests must prove:

- a detached run receives token/progress events without changing foreground
  transcript rows
- attaching a run swaps or overlays the selected run transcript intentionally
- detaching returns to the previous foreground surface without merged chatter

## Slash Commands

`ProductionRuntime.DispatchCommand` routes these commands:

- `/run [run-id]`
- `/runs`
- `/jobs` alias for `/runs`
- `/attach <run-id>`
- `/detach [run-id]`
- `/cancel <run-id>`
- `/retry <run-id>`
- `/continue <run-id>`
- `/fork <run-id>`

`/run` with no ID shows the active attached run. `/run <run-id>` fetches details
without taking attachment. `/attach` claims attachment through `run.attach`.

Command output is surfaced as compact transcript event rows plus status chrome.
Long event lists are summarized; users can request more through repeated
commands once pagination exists in a later release.

`/runs` rows include an input-required marker for `waiting_permission` and
`waiting_user`, and a stuck marker when the BFF summary reports `stuck=true`.
The shell does not compute stuck state from wall-clock time independently; it
renders the BFF summary so all clients use the same threshold.

## Runtime Interface Changes

Add optional methods to `harnesshost.Runtime` or extend `DispatchCommand`
internally without changing the shell boundary. Preferred v0.3.0 approach:
keep the public runtime interface stable and implement command routing inside
`ProductionRuntime.DispatchCommand`.

`InterruptRun` should call `run.cancel` when the run ID is a BFF run ID. Keep
fallback to `turn.cancel` only for compatibility with old BFFs.

## Attach/Detach Semantics

Attach:

1. call `run.attach` with last observed sequence if available
2. replace active run ID on success
3. replay returned events through projection
4. mark replay fidelity in status if partial

Detach:

1. call `run.detach`
2. clear shell active run ID only if detaching the active run
3. keep detached transcript buffer available by run ID

Attachment conflict surfaces a transcript event with the BFF reason.

## Reconnect/Resume

On `session.resume` or connection re-establishment:

1. call `run.list` for active/recent session runs
2. for any run previously active in shell state, call `run.events` from last
   observed sequence
3. if replay is full, project events normally
4. if replay is summary-only, render fidelity note and latest checkpoint
5. do not auto-attach observer-only runs

If the BFF reports a run detached after grace timeout, shell state follows the
BFF and clears active run ownership.

## UI State

Add shell state fields only as needed:

- `activeRunID`
- `runSummaries []RunSummaryView`
- `detachedTranscripts map[string][]TranscriptItem`
- `lastObservedRunSeq map[string]int64`
- `runReplayFidelity map[string]string`

Keep BFF protocol structs out of `internal/harnessshell`. Host projection
converts protocol payloads into shell-local view structs.

## Tests

- `/run` calls runtime command path and renders active run summary
- `/runs` distinguishes running, waiting permission, waiting user, completed,
  failed, and cancelled rows
- `/runs` renders input-required and stuck markers from BFF summaries
- `/attach` handles success, conflict, terminal run, and partial replay
- `/detach` preserves detached transcript buffer
- reconnect replays from last sequence and reports summary fidelity on gap
- foreground transcript remains free of detached run events
