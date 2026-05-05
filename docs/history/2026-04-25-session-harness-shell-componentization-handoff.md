# 2026-04-25 — Harness shell componentization handoff

## Goal

Record the accepted spike baseline, the docs/release artifacts that now govern
the work, and the exact set of changes that should move from the spike branch
back to the feature branch for continued implementation.

## Branch State

Current branch: `spike/scrolling-surface-eval`

This branch should stop being the place where production componentization work
continues. It is now the prototype/reference branch for the accepted shell
behavior.

Further work should continue on the feature branch under `v0.2.1`.

## Accepted Behavior Baseline

The accepted behavior target is:

- `FEAT-0014` — [0014-harness-conversation-shell.md](../features/0014-harness-conversation-shell.md)

Important clarification:

- parity is measured against the spike behavior captured in `FEAT-0014`
- parity is **not** measured against the currently broken `internal/harness`
  conversation UI

The current production harness remains useful as a source of command/runtime
inventory, but not as the authority for shell interaction behavior.

## Spike Commits To Carry Forward

These commits define the current accepted shell behavior baseline and should be
merged or cherry-picked into the feature branch:

- `eb55263` — `SPIKE: scrolling-surface checkpoint — ▎ marker, flat surface, arrow history`
- `edabd93` — `SPIKE: move permission controls into composer`
- `eef5324` — `SPIKE: simplify permission action list`
- `99ec256` — `SPIKE: finish harness permission and scroll behavior`

Earlier spike checkpoints may remain as historical context, but the four commits
above are the concrete baseline for the conversation-shell behavior.

## Files To Bring Over

### Spike shell code

- `internal/harnessspike/app.go`
- `internal/harnessspike/app_test.go`
- `internal/harnessspike/styles.go`

### Feature / patch / release docs

- `docs/features/0014-harness-conversation-shell.md`
- `docs/features/0009-terminal-harness.md`
- `docs/features/README.md`
- `docs/patches/0015-harness-shell-component-api.md`
- `docs/patches/README.md`
- `docs/releases/README.md`
- `docs/releases/v0.2.1/plan.md`
- `docs/releases/v0.2.1/track-a-harness-shell-componentization.md`
- `docs/history/2026-04-24-session-harness-shell-feature-doc.md`
- `docs/history/2026-04-24-session-harness-shell-release-setup.md`
- `docs/history/2026-04-25-session-harness-shell-componentization-handoff.md`

### Existing spike history that should remain available for reference

- `docs/history/2026-04-23-session-harness-spike-shell.md`
- `docs/history/2026-04-24-session-harness-spike-tool-permission.md`
- `docs/history/2026-04-24-session-harness-spike-tool-permission-handoff.md`
- `docs/history/2026-04-24-session-harness-spike-scrolling-surface-checkpoint.md`
- `docs/history/2026-04-24-session-harness-spike-scrolling-surface-handoff.md`

## Files Not To Bring Over Blindly

- unrelated dirty `internal/harness/` files on this branch
- unrelated dirty `internal/cli/harness.go`
- local artifacts such as `.claude/worktrees/`
- the local `modeltap` artifact
- unrelated history files not tied to the shell/componentization track

## Release / Process State

The release and artifact state after transfer should be:

- `FEAT-0014` accepted
- `PATCH-0015` approved
- `v0.2.1` created as a separate patch release
- `v0.2.1` starts in **Phase 1 — Design**

This matters because `v0.2.0` is already in Phase 3 and should not absorb new
design-stage work.

## Componentization Framing

`v0.2.1` should treat the current `internal/harness` TUI as:

- useful for command inventory
- useful for runtime/host integration inventory
- useful for identifying what host-side capabilities the new shell must drive

But it should treat the current `internal/harness` conversation UI as:

- replacement scope
- not the UX authority

`WU-097` is the first required design artifact because the work has two design
layers:

1. refactor plan and migration sequencing
2. detailed component/API/integration design under that plan

## Recommended Merge Sequence

1. Commit the spike-branch docs that finalize feature/patch/release setup and
   this handoff.
2. Merge or cherry-pick the accepted spike baseline commits plus the docs into
   the feature branch.
3. Continue all future work on the feature branch under `v0.2.1`.
4. Stop using this spike branch for ongoing implementation.

## Next Step On Feature Branch

Write the `WU-097` design doc for:

- refactor plan
- migration sequencing
- current `internal/harness` keep/adapt/replace classification
- cutover strategy from old harness UI to the extracted shell component
