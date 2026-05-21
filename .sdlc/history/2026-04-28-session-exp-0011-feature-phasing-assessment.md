# 2026-04-28 — EXP-0011 Feature Phasing Assessment

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Context

Assessed `EXP-0011: Harness Excellence Gap Analysis` to turn its gap map into a
stack-ranked professional-harness upgrade path for the BFF and terminal
harness.

## Work Completed

- Reviewed EXP-0011 against the current BFF, terminal harness, conversation
  shell, skills, agent teams, and harness-base ADR docs.
- Checked the current `ProductionRuntime.SubmitTurn` and BFF `handleTurnSubmit`
  shape to ground the assessment in the live code boundary.
- Stack-ranked the changes by code-generation quality impact, foundation value,
  and implementation sequencing risk.
- Determined that the upgrade should phase in through six downstream features
  plus one ADR and one engineering patch/evaluation track.

## Notes

- EXP-0011 remains an exploration and does not authorize implementation.
- Recommended next step is promotion into formal ADR/FEAT/PATCH artifacts before
  opening a new implementation release.
