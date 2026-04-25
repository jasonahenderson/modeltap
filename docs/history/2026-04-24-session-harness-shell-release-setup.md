# 2026-04-24 — Harness shell componentization release setup

## Goal

Finalize the shell-component API patch and set up a clean release structure for
the follow-on componentization work.

## What changed

- Marked `PATCH-0015` approved and assigned it to `v0.2.1`
- Marked `FEAT-0037` accepted
- Added `docs/releases/v0.2.1/plan.md`
- Added `docs/releases/v0.2.1/track-a-harness-shell-componentization.md`
- Updated release indexing in `docs/releases/README.md`

## Why

`v0.2.0` is already in Phase 3, so new design-stage work for shell
componentization should not be added silently to that release. The shell
componentization effort now has:

- an accepted behavior target (`FEAT-0037`)
- an approved implementation-scoped contract (`PATCH-0015`)
- a separate patch release with its own phase state and WUs

## Notes

- `WU-097` exists specifically to capture the refactor plan and migration
  sequencing before the more detailed component/API design work.
- `v0.2.1` starts in Phase 1 and is the correct home for the design/review/code
  sequence for this work.
