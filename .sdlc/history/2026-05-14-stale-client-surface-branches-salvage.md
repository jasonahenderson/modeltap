# 2026-05-14: Stale client-surface branches salvage

## Summary

Salvaged the useful design intent from two stale local-only branches onto the
current `release/v0.3.0` line after PR #7 renamed BFF terminology to runtime
server terminology.

## Source Branches

- `docs/feat-0023-and-bff-amendment-001`
- `patch/0017-session-scoped-project-context`

## Result

- Recreated FEAT-0023 as a current draft desktop GUI client spec.
- Recreated PATCH-0017 as a proposed implementation patch for session-scoped
  project context.
- Added FEAT-0008 Amendment 001 back to the runtime-server spec using current
  terminology.
- Updated feature, patch, and release indexes to reference the salvaged
  artifacts.

## Notes

The old branches forked before the v0.3.0 run-runtime and runtime-server rename
work. They were intentionally not merged directly because doing so would have
reintroduced stale BFF terminology and attempted to remove newer release
artifacts.
