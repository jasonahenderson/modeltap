# 2026-04-25 — Session: WU-097 refactor plan and migration sequencing

## Goal

Write the first `v0.2.1` design artifact: the refactor plan that defines how
the accepted shell spike behavior moves toward a reusable component without
turning extraction into a silent redesign.

## What changed

- Added `.sdlc/releases/v0.2.1/designs/2026-04-25-design-refactor-plan-097.md`
- Marked the WU-097 design item complete in
  `.sdlc/releases/v0.2.1/plan.md`

## Key decisions

- `FEAT-0014` is the parity target for extraction
- the existing production harness conversation UI is replacement scope, not the
  behavior authority
- the later `v0.2.0` harness line is still the source of command/runtime
  integration inventory for later design work
- the migration should proceed in stages:
  - freeze behavior
  - design shell boundary
  - extract shell-local code
  - attach modeltap host integration
  - cut over and retire superseded shell code

## Notes

- This branch does not contain the later `internal/harness` line, so the design
  explicitly pushes command/runtime inventory work into `WU-099`.
- `WU-098` and `WU-099` now have a clearer contract for the next design pass.
