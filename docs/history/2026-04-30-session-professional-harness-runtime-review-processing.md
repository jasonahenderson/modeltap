# 2026-04-30 — Professional Harness Runtime Review Processing

## Context

Processed Kimi and Claude peer reviews for FEAT-0015 through FEAT-0022 under
`docs/features/.reviews/`.

## Work Completed

- Aligned run terminology across FEAT-0015 and FEAT-0016 by separating run
  status, pipeline stage, and attachment state.
- Clarified `waiting_user`, `/run` vs `/runs`, background executor availability,
  artifact-oriented workflow behavior, and workflow role terminology.
- Removed `promoted-from: FEAT-0015` from member features; `parent` and `series`
  now carry the umbrella relationship.
- Relaxed over-strict dependencies in FEAT-0019, FEAT-0021, and FEAT-0022 where
  the review found useful earlier slices.
- Added FEAT-0015 Future ADRs covering run semantics, prompt/rules, validation,
  artifacts, policy/workspace, and memory/routing/extension trust.
- Added dispositions to the Claude JSON/Markdown findings and the Kimi review
  synthesis.

## Notes

- No product implementation was performed.
- FEAT-0015 through FEAT-0022 remain `draft`; the next promotion step should
  draft the run-runtime ADR before moving the series toward `proposed`.
