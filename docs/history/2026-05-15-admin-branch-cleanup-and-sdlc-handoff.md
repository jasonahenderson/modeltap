# 2026-05-15 — ADMIN: Branch Cleanup and SDLC Handoff

## Summary

Cleaned up local branch state after salvage PR handling and prepared the
`admin/sdlc-directory-migration-plan` branch as the restart point for the SDLC
directory migration.

## Branch and PR Cleanup

- Confirmed PR #11 (`salvage/harness-shell-componentization` -> `main`) was
  merged.
- Fixed PR #11 lint before merge by removing unused spike styles and replacing
  a loop with `append(parts, segments...)`.
- Confirmed PR #11 checks passed after the fix:
  - lint: success
  - test: success
  - build: success
  - DCO: success
- Deleted local branches whose work was captured on `origin/main`:
  - `salvage/harness-shell-componentization`
  - `salvage/hopeful-adr-simplification`
  - `patch/0016-finalize`
- Deleted superseded PATCH-0016 branch locally and remotely:
  - local: `patch/0016-pr1-ci-test-failures`
  - remote: `origin/patch/0016-pr1-ci-test-failures`

## Important Finding

GitHub reported PR #12 (`salvage/hopeful-adr-simplification` -> `main`) as
merged even though it had previously been closed after review as stale/mixed.
That PR changed ADRs in ways that appeared outdated relative to current code and
accepted feature docs, including:

- ADR-0004: proposed replacing Viper with stdlib + YAML, while current code
  still uses Viper.
- ADR-0007: proposed query-time metrics, while current storage still uses
  `hourly_usage` and `daily_usage`.
- ADR-0008/0009: proposed deferring knowledge/MCP decisions, while accepted
  feature docs still depend on them.

Follow-up may be needed on `main` if PR #12 was merged accidentally.

## SDLC Branch State

Current branch:

```text
admin/sdlc-directory-migration-plan
```

Remote tracking branch:

```text
origin/admin/sdlc-directory-migration-plan
```

The branch targets `release/v0.3.0`, not `main`, by user decision.

Current SDLC plan commit:

```text
87b6733 ADMIN: plan SDLC directory migration
```

Plan artifact:

```text
docs/history/2026-05-15-admin-sdlc-directory-migration-plan.md
```

## Restart Prompt

Use this prompt after clearing context:

```text
We are in /Users/jasonhenderson/Projects/jasonahenderson/modeltap.

Checkout/use branch admin/sdlc-directory-migration-plan, which targets
release/v0.3.0. Before editing, run git status --short --branch and confirm the
worktree is clean.

Read docs/history/2026-05-15-admin-sdlc-directory-migration-plan.md, then
execute the SDLC migration plan.

Keep ADRs in docs/adr/ for this first pass unless instructed otherwise. Move
lifecycle artifacts from docs/{history,features,patches,explorations,releases}
to .sdlc/{history,features,patches,explorations,releases}. Update canonical
process docs and references accordingly. Preserve unrelated branches and
worktree changes. Commit as ADMIN-only process work; do not mix product changes.
```
