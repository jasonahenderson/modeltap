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
| 100 | Behavior-Preserving Shell Extraction Implementation | L | In progress | Stage A in flight |
| 101 | Developer Documentation and Embedding Examples | M | In progress | Structural pass in flight |
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

- **WU-100 Stage A** — introduce `internal/harnessshell` package skeleton with
  mirror types from spike. No behavior change. Building blocks for later
  stages.
- **WU-101 structural pass** — three doc skeletons (`internal/harnessshell/README.md`,
  `internal/harnesshost/README.md`, `docs/guides/harness-shell-embedding.md`)
  with provisional names per WU-098/WU-099. Reconciled with final names after
  WU-100 cutover.

## In progress

| Item | Owner | Started | Notes |
|------|-------|---------|-------|
| WU-100 Stage A | backend agent | 2026-04-26 | Type-duplication phase only |
| WU-101 structural pass | docs agent | 2026-04-26 | Provisional names; reconciliation pass after WU-100 |

## Open items

- CLI entrypoint rename for `modeltap harness-spike` is a Phase 3
  implementation decision; final command name TBD.
- Demo runtime package `internal/harnessdemo` does not yet exist; will be
  introduced during WU-100 Stage 6 / Stage E.
- Phase 2 disposition tables in `docs/releases/v0.2.1/.reviews/` should be
  re-checked once implementation reveals any unforeseen issues; the
  `revise the design explicitly` rule from `.agents/process.md` applies.
