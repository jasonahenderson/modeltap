# Implementation Plan: FEAT-0014 (Harness Conversation Shell) + PATCH-0015 (Harness Shell Component API)

## Context

`v0.2.0` remains the active implementation release for the first integrated BFF
and terminal harness. The conversation-shell spike completed after `v0.2.0`
entered Phase 3 and established a more specific shell interaction contract:

- single scrolling transcript surface
- tail-mounted composer
- queue release on empty `Enter` when idle
- composer-driven permission handling
- inline paste expansion with file references previewed on demand

That interaction contract now lives in `FEAT-0014`. The next step is not a UX
redesign. It is behavior-preserving extraction of that shell into a reusable,
well-documented component with a clean host/runtime API, as authorized by
`PATCH-0015`.

Because `v0.2.0` is already in Phase 3, this work is planned as a separate
patch release rather than being added silently to the active release.

## Scope

This release covers:

- behavior-preserving componentization of the harness conversation shell
- definition of a clean shell/host API boundary
- modeltap host-adapter integration for the extracted component
- developer documentation and embedding guidance for future reuse/extraction

This release does not cover:

- new shell UX beyond the current `FEAT-0014` behavior
- branch/retry semantics
- production permission-object redesign beyond the agreed shell boundary
- publishing the component as its own repository

## Approach

**One track:**
- **Track A** (6 WUs): FEAT-0014 shell componentization + PATCH-0015 API and docs work

**Total: 6 work units (WU-097 through WU-102)**

| WU | Title | Dependencies | Size | Parallelizes With |
|----|-------|-------------|------|-------------------|
| 097 | Refactor plan and migration sequencing | — | M | — |
| 098 | Shell component API and package-boundary design | 097 | M | 099 |
| 099 | Modeltap host adapter and integration design | 097 | M | 098 |
| 100 | Behavior-preserving shell extraction implementation | 098, 099 | L | 101, 102 |
| 101 | Developer documentation and embedding examples | 098, 099 | M | 100, 102 |
| 102 | Parity and regression test sweep | 100 | M | 101 |

**Critical path:** 097 → 098/099 → 100 → 102. WU-101 runs from the accepted API
and integration design and should complete before the release closes.

## Track A: FEAT-0014 + PATCH-0015

See [track-a-harness-shell-componentization.md](track-a-harness-shell-componentization.md).

## Phased Execution

Per `.agents/process.md`, this release executes in three release-level phases:

- **Phase 1 — Design:** design all WUs across the release. No code.
- **Phase 2 — Review (current):** user-chosen HIL/design review of the
  completed designs. No new designs. No code.
- **Phase 3 — Implementation:** implement all WUs in dependency-legal order.

Current phase: **Phase 2 — Review.**

Phase 1 closed on 2026-04-26 with all six WU designs complete and the
pre-design plan reviews dispositioned. Phase 2 covers a full plan + design
review pass. Reviews are commissioned to other models first; the human
reviewer (HIL) processes review findings before Phase 3 begins.

### Phase 1 Completion Checklist

- [x] WU-097 design: refactor plan and migration sequencing
- [x] WU-098 design: shell component API and package boundary
- [x] WU-099 design: modeltap host adapter and integration
- [x] WU-100 design: behavior-preserving extraction implementation
- [x] WU-101 design: developer docs and embedding examples
- [x] WU-102 design: parity and regression verification

### Review Gates

- Pre-design plan reviews live under `docs/releases/v0.2.1/.reviews/` and may
  update the release plan or early design artifacts during Phase 1.
- Phase 2 begins only after every WU design in this release is complete.
- Phase 2 closes only after the commissioned design-review artifacts are
  recorded under `docs/releases/v0.2.1/.reviews/` and their findings are
  dispositioned explicitly.

## Verification

After all WUs complete:

1. The extracted shell component preserves the current `FEAT-0014` behavior.
2. The component boundary is action/event oriented and avoids callback-shaped
   API contracts.
3. The modeltap host adapter drives the extracted component without importing
   modeltap runtime concerns back into the reusable package.
4. Developer documentation is sufficient to embed the component without reading
   the old spike implementation first.
5. The component package is organized so it can later move to its own
   repository with minimal contract churn.
