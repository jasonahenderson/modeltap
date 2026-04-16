# 2026-04-16 — plan review naming convention update

## Summary

Updated the repo guidance for non-canonical work-plan review artifact naming.

## Changes

- Clarified in `AGENTS.md` that non-canonical work-plan reviews should include the reviewing model or harness name in the filename when known.
- Added the same convention to `docs/agents.md`.
- Updated the review-layout READMEs for:
  - `docs/features/.reviews/README.md`
  - `docs/patches/.reviews/README.md`
  - `docs/adr/.reviews/README.md`
  - `docs/releases/README.md`
  - `docs/releases/.reviews/README.md`

## Convention

- Keep canonical per-doc findings filenames unchanged:
  - `{stem}-findings.md`
  - `{stem}-findings.json`
- For non-canonical work-plan reviews, prefer reviewer-first filenames when the reviewer identity is known:
  - `codex-plan-review.md`
  - `codex-0008-bff-server-connectivity-review.md`
  - `gpt5-0001-openai-responses-api-support-plan-review.md`
