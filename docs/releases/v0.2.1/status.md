# v0.2.1 — Status

**Current phase:** Phase 3 — Implementation
**Branch:** `spike/scrolling-surface-eval` (should retarget to a release branch
before tagging; pending TPM decision)
**Phase 1 closed:** 2026-04-26
**Phase 2 closed:** 2026-04-26

## Phase 3 work units

| WU | Title | Size | State | Notes |
|----|-------|------|-------|-------|
| 097 | Refactor Plan and Migration Sequencing | M | Done (Phase 1 design) | Deliverable is the design doc |
| 098 | Shell Component API and Package-Boundary Design | M | Done (Phase 1 design) | Deliverable is the design doc |
| 099 | Modeltap Host Adapter and Integration Design | M | Done (Phase 1 design) | Deliverable is the design doc |
| 100 | Behavior-Preserving Shell Extraction Implementation | L | In progress | Stages A+B done (`1cb1eb4`, `1e32b57`); Stage C in flight (`19a546b` scaffolding, `d106fc6` Model wire-up); submit/permission/preview emission and host-event intake still ahead |
| 101 | Developer Documentation and Embedding Examples | M | In progress | Structural pass done (`a40e4b9`); reconciliation pass after WU-100 cutover |
| 102 | Parity and Regression Test Sweep | M | Up next | Starts after WU-100 cutover |

## Phase 3 dependency graph

```
WU-100 (extraction)  ──→  WU-102 (parity tests)
       ║
       ║  parallel
       ║
WU-101 (docs)
```

WU-097, WU-098, WU-099 produced design docs in Phase 1; their Phase 3
deliverables are already present.

## Up next

- **WU-100 Stage C continuation** — wire submit/interrupt/permission/preview
  key paths into Model.Update so the shell emits the corresponding typed
  Actions (SubmitTurnAction, InterruptRunAction, ResolvePermissionAction,
  LoadPreviewAction) instead of leaving these flows on the spike. Add host-
  event intake methods so the shell can be driven entirely by inbound
  HostEvents. Then narrow the spike into a translation shim that drives
  Model + fake runtime + host-event feedback.

## In progress

- **WU-100 Stage C** — scaffolding helpers and Model wire-up have landed
  (`19a546b`, `d106fc6`); next commits cover action emission for submit/
  interrupt/permission/preview and host-event intake. Spike still owns
  submit/interrupt/permission paths through its own event loop.

## Done this phase

- WU-100 Stage A (`1cb1eb4`) — `internal/harnessshell` skeleton with action/event
  boundary types, state structs, and styles. Build clean, no
  `internal/harness/theme` import.
- WU-100 Stage B (`1e32b57`) — rendering cutover. Pure-function `Render(RenderInput)
  → RenderResult` value-type bridge. Spike's `refreshTranscript` now delegates
  to `harnessshell.Render`. Sidebar/palette/agent overlays remain in spike.
  Spike tests + build clean.
- WU-100 Stage C scaffolding (`19a546b`) — shell-owned helpers for composer
  history, paste/dropped-path detection, queue merge (FIFO + transient
  pendingSubmissions buffer), and pending-permission lifecycle. Pure additive;
  no behavior change. Stage A out-of-scope chrome fields removed from the
  state struct per WU-100 §"Definite scope rule".
- WU-100 Stage C wire-up (`d106fc6`) — Model.New/Init/Update/View wired for
  shell-local keys (Tab focus cycle, single-line Up/Down history, Ctrl+P/N
  composer-token selection, Up/Down/j/k transcript-token movement) and
  shell-owned RenderInput projection. Resolves the WU-098-deferred envelope
  choice with `ActionMsg{Action Action}` (single envelope; concrete dispatch
  at the host adapter). Smoke tests cover wire-up only — WU-102 still owns
  parity coverage.
- WU-101 structural pass (`a40e4b9`) — `internal/harnessshell/README.md`,
  `internal/harnesshost/README.md`, `docs/guides/harness-shell-embedding.md`
  with 57 provisional placeholders to be reconciled after WU-100 cutover.

## Open items

- CLI entrypoint rename for `modeltap harness-spike` is a Phase 3
  implementation decision; final command name TBD.
- Demo runtime package `internal/harnessdemo` does not yet exist; will be
  introduced during WU-100 Stage 6 / Stage E.
- Phase 2 disposition tables in `docs/releases/v0.2.1/.reviews/` should be
  re-checked once implementation reveals any unforeseen issues; the
  `revise the design explicitly` rule from `.agents/process.md` applies.
