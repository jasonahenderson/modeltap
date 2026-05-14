# 2026-04-28 — Harness Excellence Research

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Context

Explored what makes Claude Code, Codex, and the post-leak clone ecosystem strong
agent harnesses, and compared those traits against modeltap's current harness
direction.

## Work Completed

- Added `docs/explorations/0011-harness-excellence-gap-analysis.md`.
- Revised EXP-0011 to focus on internal call-loop mechanics, not only feature
  parity: managed turn preflight, execution, postflight artifacts, and recovery.
- Reframed the analysis around code generation quality rather than chasing
  agentic feature breadth: repo-aware context, edit discipline, validation
  feedback, codegen prompt contracts, and quality-driven routing.
- Added a secondary feature-gap inventory so feature gaps are captured without
  displacing code generation quality as the main objective.
- Added a stack-ranked BFF/harness upgrade assessment focused on the most
  impactful codegen-quality improvements and likely downstream artifact split.
- Updated `docs/explorations/README.md` to include EXP-0010 and EXP-0011.
- Kept the work upstream as exploration only; no release implementation or code
  changes were made.

## Notes

- The research uses public analyses and clone documentation as design input.
- No leaked proprietary source code was copied or referenced directly.
