# Project Instructions for Agents

## About This Project

`modeltap` is a Go reverse proxy for AI/ML clients that captures requests and responses, tracks usage metrics, and is evolving toward a cross-model knowledge and orchestration layer.

## Key Directories

- `docs/explorations/` — upstream explorations that may promote into features, patches, or ADRs
- `docs/features/` — behavior-scoped feature specs
- `docs/patches/` — implementation-scoped work authorization docs
- `docs/adr/` — architectural decision records
- `docs/history/` — status, plans, and session/work logs
- `docs/agents.md` — detailed agent-team workflow and roles

## Working Conventions

1. Keep responses concise unless the user asks for depth.
2. Follow accepted ADRs and accepted features; explorations are advisory and upstream.
3. Log significant work to `docs/history/` before stopping.
4. If repo-process work and product work are mixed, split them into separate commits or tracked artifacts.

## Artifact Taxonomy

Choose the artifact by scope of change, not by size and not by semver intuition.

- `EXP` — upstream problem framing and design-space exploration
- `FEAT` — behavior-scoped product definition
- `PATCH` — implementation-scoped product or engineering-system work
- `ADR` — architectural decisions with future constraint value
- `WU` — implementation work unit under an accepted feature
- `ADMIN` — repo process and workflow changes

Crystal-clear rules:

- `PATCH` does not mean semver patch. It means implementation-scoped work.
- `EXP` does not authorize code changes by itself. Promote it first if implementation work is needed.
- `WU` commits are the normal implementation path for accepted features already broken into work units.
- `ADMIN` is outside the numbered product artifact pipeline. Use it for instruction files, prompts, review conventions, logging/process rules, and doc-structure changes.

## Exploration Documents

- Explorations live in `docs/explorations/` and use `NNNN-title-with-dashes.md` filenames.
- Use YAML front matter with at least: `exploration`, `title`, `status`, `date`.
- Valid statuses are `exploring`, `deferred`, `promoted`, `superseded`, `closed`.
- Exploration review artifacts, if used, live in `docs/explorations/.reviews/` using the same file stem.
- Explorations are upstream artifacts. They may promote into:
  - a feature spec when behavior and success criteria need to be defined
  - a patch when the work is implementation-scoped
  - an ADR when an architectural choice is needed
- When promoting an exploration, keep the exploration file and add `promoted-to` references rather than deleting upstream context.

## Commit Requirements

Every meaningful change should use the right commit prefix:

- `WU-NNN:` — implementation work under an accepted feature work unit
- `PATCH-NNNN:` — implementation-scoped work under an approved patch
- `FEAT-NNNN:` — drafting or revising a feature spec
- `ADR-NNNN:` — drafting or revising an ADR
- `EXP-NNNN:` — drafting or revising an exploration
- `ADMIN:` — process/admin/instruction changes; no numbered doc required by default

Commit body requirements:

- 1-3 short lines describing what changed
- reference the canonical doc path when one exists
- use `git commit -s` for DCO sign-off

## Patch Process

- Patches live in `docs/patches/`.
- Use a patch for implementation-scoped fixes, tooling, infra, DX, internal plumbing, or missing coverage that does not need a full feature spec.
- Every code change outside an accepted feature work unit should trace to an approved patch, unless it is strictly `ADMIN:` work.

## Admin Tasks

Use `ADMIN:` for:

- `CLAUDE.md`, `AGENTS.md`, or other instruction-file changes
- process and workflow rules
- review-structure changes
- documentation taxonomy / directory-structure updates
- prompt, hook, or skills changes

Do not hide product or infra changes inside an `ADMIN:` commit.
