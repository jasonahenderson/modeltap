# 2026-05-15 — ADMIN: SDLC Directory Migration

## Plan

- Keep ADRs out of the initial lifecycle move for this pass.
- Move lifecycle artifacts from `docs/{explorations,features,patches,releases,history}` into `.sdlc/`.
- Update canonical process, agent entrypoints, and tracked references to use `.sdlc/*`.
- Preserve the existing `.sdlc/review-artifacts/` tree.

## Work Completed

- Moved lifecycle artifact directories into `.sdlc/` with `git mv`.
- Updated `.agents/process.md`, agent contracts, `AGENTS.md`, `CLAUDE.md`, and `docs/agents.md` for the new canonical paths.
- Swept tracked references from old lifecycle paths to `.sdlc/*`, while keeping the prior ADR home unchanged.
- Fixed release-doc ADR links that became stale after moving release directories.
- Left compatibility directories out of `docs/` because no tooling requirement was found.

## Validation

- `rg -n 'docs/(history|explorations|features|patches|releases)' ...` only reports intentional historical migration-plan references.
- `rg -n '(\\.\\./)+adr/' .sdlc docs README.md internal` reports no stale relative ADR links.
- `go test ./...` passed.
