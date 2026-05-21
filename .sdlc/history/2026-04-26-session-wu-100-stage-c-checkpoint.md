# 2026-04-26 — Session: WU-100 Stage C checkpoint

## Scope

Picked up the WU-100 Stage C work that was sitting uncommitted in the working
tree at the start of the session. Split into two safe, rollback-bounded
commits and resolved the WU-098-deferred `tea.Msg` envelope choice.

## Starting state

- Branch: `spike/scrolling-surface-eval`
- Phase: 3 — Implementation (WU-100 Stages A+B done at `1cb1eb4`, `1e32b57`)
- Working tree had ~450 lines of uncommitted edits across
  `internal/harnessshell/{input,permissions,queue,state,tokens}.go` — Stage C
  helper scaffolding the prior session built but did not commit.
- No prior session log for the in-progress Stage C work; status said "Stage C
  is queued for dispatch".

## Decisions

### `tea.Msg` envelope shape (WU-098 §"Concrete forwarding shape")

**Decision:** single `ActionMsg{Action Action}` envelope.

**Rationale:** the reusable shell aims to be embedded in arbitrary hosts; a
single envelope keeps host loops minimal and lets host-specific dispatch live
in the adapter layer (`internal/harnesshost` per WU-099). The exhaustive-
switch benefit of per-action concrete msgs is best served at the adapter,
not at the outer Bubble Tea loop, where a host that cares about a single
action type would otherwise need a no-op case for every other action.

### Two-commit split for safety

Rather than a single Stage C commit, split into:

1. Pure additive helper scaffolding (no contract changes, no Model wire-up)
2. Public Model wire-up + chosen envelope + smoke tests

This gave a clean rollback boundary between "scaffolding lands" and "shell
becomes operable in isolation." It also let the user weigh in on the
envelope choice between commits.

## Commits landed

```
19a546b WU-100: Stage C scaffolding — shell-state helpers
d106fc6 WU-100: Stage C wire-up — Model API for shell-local keys
```

### `19a546b` scaffolding scope

- Composer history, paste/dropped-path detection helpers (`input.go`)
- Composer-token lifecycle helpers (`tokens.go`)
- Queue FIFO + merge helpers respecting WU-098 invariants (`queue.go`)
- Pending-permission lifecycle helpers (`permissions.go`)
- `state` struct: added `streaming`, `streamPulse`, `activeRunID`,
  `submissionCounter` and `TranscriptItem.SubmissionID`/`RunID`; removed
  Stage A out-of-scope chrome fields (`sidebarItems`, `sidebarIndex`,
  `dialog`, `palette`, `agentList`, `agentDetail`) per WU-100 §"Definite
  scope rule for the reusable package"

### `d106fc6` wire-up scope

- `ActionMsg{Action Action}` envelope added to `types.go`
- `Model.New` initializes textarea + viewport with theme-neutral lipgloss
  styles (no `internal/harness/theme` import)
- `Model.Init` returns `textarea.Blink`
- `Model.Update` handles WindowSize, KeyMsg (Tab focus cycle, single-line
  history Up/Down, Ctrl+P/N token selection, transcript-ref Up/Down/j/k),
  MouseMsg, and forwards remaining messages to the focused widget
- `drainPendingActions` empties `state.pendingActions` into a
  `tea.Cmd` per-action that emits `ActionMsg`
- `Model.View` projects shell-owned state into `RenderInput`, calls
  `Render`, pipes the result into a *local copy* of the viewport so View
  stays pure (no observable mutation of `m`)
- Smoke tests in `model_test.go` (internal package, not parity coverage)

## What's next

- **Submit / interrupt / permission / preview action emission.** Wire the
  remaining KeyMsg cases (`Enter` submit, second `Esc` interrupt,
  permission Apply, preview-token Ctrl+O) into `Model.Update` so they emit
  `SubmitTurnAction` / `InterruptRunAction` / `ResolvePermissionAction` /
  `LoadPreviewAction`. The helpers for these flows already exist on the
  state struct from `19a546b`; only the key-routing and action push are
  missing.
- **Host-event intake methods.** Add `apply*Event` methods (or fold the
  application into `Model.Update`'s `HostEvent` case) for
  `SubmissionAcceptedEvent`, `RunStartedEvent`, `RunDeltaEvent`,
  `RunCompletedEvent`, `RunStoppedEvent`, `RunFailedEvent`,
  `PermissionRequestedEvent`, `PermissionResolvedEvent`, `PreviewLoaded
  Event`, `HostStatusEvent`.
- **Spike narrowing.** Once the Model can be driven end-to-end by Actions +
  HostEvents, refactor `internal/harnessspike` into a translation shim:
  spike calls `Model.Update`, catches `ActionMsg`, drives the existing
  fake runtime, and feeds host events back into the model.
- **Scroll-preservation rewiring.** The spike's manual-scroll-preserved /
  follow-tail logic still lives in `App.refreshTranscript`. Once the spike
  is a thin shim, this needs to move into `Model.Update` (likely on
  `RunDeltaEvent`) so the FEAT-0014 invariants hold.

## Open items

- The `submissionCounter` / `nextSubmissionID` helpers are not yet called
  from anywhere. They will be used by the submit-action emission step.
- Host-event intake design is specified by WU-098/WU-099; implementation
  detail (one big `Apply(HostEvent)` switch vs per-event methods) is a
  Stage C decision still to make.
- Spike-shim cutover risk: the spike's `App` struct currently owns mutable
  state that overlaps with `Model.state`. The cutover needs to delete the
  spike-owned copy without losing scroll-preservation or queue invariants.
  This is the highest-risk part of Stage C and will need its own commit
  boundary.
