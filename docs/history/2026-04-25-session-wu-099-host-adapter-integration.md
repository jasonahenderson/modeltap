# 2026-04-25 — Session Log: WU-099 host adapter and integration design

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Scope

Completed Phase 1 design work for `WU-099` in release `v0.2.1`.

## Inputs Reviewed

- `docs/releases/v0.2.1/plan.md`
- `docs/releases/v0.2.1/track-a-harness-shell-componentization.md`
- `docs/releases/v0.2.1/designs/2026-04-25-design-refactor-plan-097.md`
- `docs/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md`
- `docs/features/0014-harness-conversation-shell.md`
- `docs/patches/0015-harness-shell-component-api.md`
- sibling checkout inventory from `/Users/jasonhenderson/Projects/jasonahenderson/modeltap`:
  - `internal/harness/app_conn.go`
  - `internal/harness/messages.go`
  - `internal/harness/model.go`
  - related `internal/harness`, `internal/bff`, and `internal/protocol` surfaces

## What Landed

Created:

- `docs/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md`

Updated:

- `docs/releases/v0.2.1/plan.md`
  - marked `WU-099` design complete

## Key Decisions

- the modeltap-specific host adapter package target is `internal/harnesshost`
- the reusable shell remains `internal/harnessshell`; the fake/demo host moves
  to a separate `internal/harnessdemo`-style adapter layer
- the host adapter consumes shell actions and projects runtime state back as
  shell host events rather than exposing the full later harness inventory
- the minimal host interface covers submit, interrupt, host-native commands,
  permission resolution, preview loading, and paste summarization only
- command routing is explicitly split between shell-native and host-native
  surfaces

## Remaining Phase 1 Work

- `WU-100` — behavior-preserving extraction implementation design
- `WU-101` — developer docs and embedding examples design
- `WU-102` — parity and regression verification design

## Notes For Resume

- current release remains `v0.2.1`, Phase 1
- `WU-100` and `WU-101` are now dependency-legal and may run in parallel
- `WU-102` still waits on `WU-100`
