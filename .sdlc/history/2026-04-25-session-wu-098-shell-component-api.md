# 2026-04-25 — Session Log: WU-098 shell component API design

## Scope

Completed Phase 1 design work for `WU-098` in release `v0.2.1`.

## Inputs Reviewed

- `.sdlc/features/0014-harness-conversation-shell.md`
- `.sdlc/patches/0015-harness-shell-component-api.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-refactor-plan-097.md`
- `.sdlc/releases/v0.2.1/track-a-harness-shell-componentization.md`
- `internal/harnessspike/app.go`
- `internal/harnessspike/app_test.go`

## What Landed

Created:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md`

Updated:

- `.sdlc/releases/v0.2.1/plan.md`
  - marked `WU-098` design complete

## Key Decisions

- the first extracted reusable package target is `internal/harnessshell`
- the shell boundary is action/event based, not callback based
- Bubble Tea remains the integration model, but host effects cross the boundary
  as typed actions and typed host events
- shell-owned invariants were fixed for queue, permission, token, and scroll
  behavior using the current spike as the extraction baseline
- fake/demo streaming logic is explicitly outside the reusable package contract

## Remaining Phase 1 Work

- `WU-099` — modeltap host adapter and integration design
- `WU-100` — extraction implementation design
- `WU-101` — developer docs and embedding examples design
- `WU-102` — parity and regression verification design

## Notes For Resume

- this branch remains in `v0.2.1` Phase 1
- `WU-099` is the next dependency-legal design step
- `WU-099` should translate the later `v0.2.0` harness/runtime inventory onto
  the `internal/harnessshell` action/event contract defined here
