# 2026-04-25 — Session Log: WU-100 extraction implementation design

## Scope

Completed Phase 1 design work for `WU-100` in release `v0.2.1`.

## Inputs Reviewed

- `.sdlc/releases/v0.2.1/plan.md`
- `.sdlc/releases/v0.2.1/track-a-harness-shell-componentization.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-refactor-plan-097.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md`
- `.sdlc/features/0014-harness-conversation-shell.md`
- `.sdlc/patches/0015-harness-shell-component-api.md`
- `internal/harnessspike/app.go`
- `internal/harnessspike/app_test.go`

## What Landed

Created:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-extraction-implementation-100.md`

Updated:

- `.sdlc/releases/v0.2.1/plan.md`
  - marked `WU-100` design complete

## Key Decisions

- extraction proceeds through staged compatibility cutovers rather than a
  single-shot replacement
- `internal/harnessshell` becomes the canonical shell package,
  `internal/harnesshost` owns modeltap integration, and `internal/harnessspike`
  shrinks to embedding/demo compatibility
- spike parity tests remain the extraction oracle until `WU-102` redistributes
  equivalent coverage
- queue, permission, scroll, token, and interrupt behaviors are explicit
  extraction invariants rather than implicit implementation details

## Remaining Phase 1 Work

- `WU-102` — parity and regression verification design

## Notes For Resume

- current release remains `v0.2.1`, Phase 1
- `WU-102` is now dependency-legal and should use spike tests as the migration
  checklist for redistributed coverage
