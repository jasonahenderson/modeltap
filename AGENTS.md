# Project Instructions for Agents

## About This Project

`modeltap` is a Go reverse proxy for AI/ML clients that captures requests and responses, tracks usage metrics, and is evolving toward a cross-model knowledge and orchestration layer.

## Process Definitions

Canonical process rules: `.agents/process.md`
Agent-team contract: `.agents/contracts/agent-team.md`
Base agent contract: `.agents/contracts/base.md`

## Key Directories

- `.sdlc/explorations/` — upstream explorations that may promote into features, patches, or ADRs
- `.sdlc/features/` — behavior-scoped feature specs
- `.sdlc/patches/` — implementation-scoped work authorization docs
- `.sdlc/adr/` — architectural decision records
- `.sdlc/releases/` — release-scoped plans, status, changelogs, and release-local reviews
- `.sdlc/history/` — status, plans, and session/work logs
- `docs/agents.md` — human-readable agent-team overview

## Working Conventions

1. Keep responses concise unless the user asks for depth.
2. Follow accepted ADRs and accepted features; explorations are advisory and upstream.
3. Log significant work to `.sdlc/history/` before stopping.
4. If repo-process work and product work are mixed, split them into separate commits or tracked artifacts.
5. **Release execution is phased — Phase 1 → Phase 2 → Phase 3, strict order, no interleaving.** Phase 1 = design ALL WUs across ALL tracks (no code, no reviews). Phase 1 is not complete until every track has designs for every WU. Phase 2 = user reviews designs however they choose (no code, no new designs); begins only when user confirms Phase 1 complete. Phase 3 = implement ALL WUs; begins only after Phase 2 findings are processed. Phase transitions are explicit ADMIN commits. Current phase lives in each release's `plan.md`. See `docs/agents.md` §"Workflow / Prime directives".

## Canonical Rules

Use `.agents/process.md` for:

- artifact taxonomy
- release-phase rules
- review artifact placement
- commit policy
- logging expectations

## Repo-Specific Expectations

1. Keep responses concise unless the user asks for depth.
2. Follow accepted ADRs and accepted features; explorations are advisory and upstream.
3. Log significant work to `.sdlc/history/` before stopping.
4. If repo-process work and product work are mixed, split them into separate commits or tracked artifacts.
5. Release execution is phased at the release level. See `.agents/process.md`.
