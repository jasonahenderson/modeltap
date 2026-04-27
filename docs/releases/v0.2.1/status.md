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
| 100 | Behavior-Preserving Shell Extraction Implementation | L | In progress | Stages A+B done (`1cb1eb4`, `1e32b57`); Stage C (action/event cutover) up next |
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

- **WU-100 Stage C** — action/event cutover. Refactor submit/interrupt/preview/
  permission paths so the shell emits typed actions instead of mutating spike
  state directly; add host-event intake so the shell can be driven entirely by
  inbound events. Spike becomes a translation shim that catches shell actions,
  drives fake-runtime behavior, and feeds host events back into the shell.

## In progress

(none — Stage C is queued for dispatch)

## Done this phase

- WU-100 Stage A (`1cb1eb4`) — `internal/harnessshell` skeleton with action/event
  boundary types, state structs, and styles. Build clean, no
  `internal/harness/theme` import.
- WU-100 Stage B (`1e32b57`) — rendering cutover. Pure-function `Render(RenderInput)
  → RenderResult` value-type bridge. Spike's `refreshTranscript` now delegates
  to `harnessshell.Render`. Sidebar/palette/agent overlays remain in spike.
  Spike tests + build clean.
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
