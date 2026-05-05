# 2026-04-17 — Session Handoff: Cross-Repo Process Core

This handoff captures the current state of the process-core consolidation work
across `modeltap`, `keyproxy/alpha`, `meetingplaceai/alpha`, and the new shared
repo `agent-process-core`.

## Topic

Consolidate templates, process contracts, and hook guardrails so the three app
repos can work the same way where it matters, while keeping repo-specific
overlays for release policy, directory naming, and tool wiring.

## Key Conclusion

The best model is:

1. a shared process core for common templates/contracts/hook logic
2. repo-local overlays for differences
3. tool adapters (`AGENTS.md`, `CLAUDE.md`, hook registration) per repo

GitHub template usage makes sense for bootstrap, but not as the only drift
management mechanism. Ongoing alignment should be handled by periodic
LLM-assisted reconciliation against a shared-core manifest.

## Repos Touched

### `modeltap`

Created `.agents/` starter structure:

- `.agents/process.md`
- `.agents/contracts/base.md`
- `.agents/contracts/agent-team.md`

Updated:

- `AGENTS.md`
- `CLAUDE.md`

Created process-planning/history docs:

- `docs/history/2026-04-16-admin-process-structure-alignment.md`
- `docs/history/2026-04-16-admin-process-adoption-checklist.md`
- `docs/history/2026-04-16-admin-shared-process-core-layout-and-sync-plan.md`
- `docs/history/2026-04-16-admin-cross-repo-process-consolidation-checklist.md`

### `keyproxy/alpha`

Created:

- `.agents/semver-release-and-design-review.md`
- `docs/history/20260416-1730-admin-semver-release-design-review.md`
- `docs/history/20260416-1815-admin-process-core-migration-plan.md`

Important note:

- this repo already has the strongest `.agents/` structure
- it should be treated as the structural reference, not the unquestioned source
  of truth
- the patch hook path appears drifted and should be fixed through shared config

### `meetingplaceai/alpha`

Created:

- `docs/history/20260416-1815-admin-process-core-migration-plan.md`

Important note:

- this repo still uses the older `.claude/skills` model as the canonical process
  store
- it has mature release/review practice that should be preserved during migration

### `agent-process-core`

Created new repo:

- `/Users/jasonhenderson/Projects/jasonahenderson/agent-process-core`

Created starter files:

- `README.md`
- `AGENTS.md`
- `manifest/shared-core-manifest.yaml`
- `schemas/process-config.schema.json`
- `contracts/base.md`
- `contracts/agent-team.md`
- `templates/madr.md`
- `templates/feature-spec.md`
- `templates/patch.md`
- `templates/exploration.md`
- `templates/release-plan.md`
- `templates/release-status.md`
- `hooks/README.md`
- `examples/repo-overlay.example.md`

## Important Findings

1. A GitHub template repo is useful for bootstrap, but not as the primary
   synchronization mechanism.
2. Drift should be classified as:
   - `core`
   - `local_override`
   - `stale_copy`
3. The duplicated patch hook utility in both `keyproxy/alpha` and
   `meetingplaceai/alpha` appears to reference `docs/implementation/patches`
   while those repos use `docs/patches`.
4. `modeltap` should keep release-level phase gating as a repo-local overlay
   unless the other repos explicitly adopt it.

## Recommended Next Steps

1. Promote the real shared templates and contracts from `keyproxy/alpha` and
   `meetingplaceai/alpha` into `agent-process-core`.
2. Add real shared hook utilities to `agent-process-core/hooks/`.
3. Define one config-driven path model for hook resolution.
4. Finish `meetingplaceai/alpha` migration from `.claude/skills` to `.agents/`.
5. Finish reducing process duplication in `modeltap`.
6. Reconcile `keyproxy/alpha` against the shared-core files once those are
   promoted.

## Good Resume Prompt

Use something close to:

> Read `docs/history/2026-04-17-session-process-core-handoff.md` in `modeltap`
> and continue the cross-repo process-core consolidation. Start by reviewing
> `agent-process-core`, then propose the next concrete patch set.

## Open Decisions

- whether `agent-process-core` should remain only a local repo or become the
  published GitHub template repo
- whether semver release/design-review guidance in `keyproxy/alpha` should stay
  as an addendum or move into the shared core
- when `modeltap` should add hooks, versus waiting until the shared hook layer is
  stabilized
