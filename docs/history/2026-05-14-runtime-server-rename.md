# 2026-05-14 — Runtime server terminology cutover

## Summary

Retired live BFF terminology in favor of runtime server terminology.

## Changes

- Added ADR-0016 defining the runtime server, client surfaces, executor role,
  and clean-cut naming policy.
- Renamed FEAT-0008 from BFF Server to Runtime Server and moved the feature
  spec to `docs/features/0008-runtime-server.md`.
- Renamed the live Go package from `internal/bff` to `internal/runtime`.
- Renamed the config namespace from `bff` to `runtime` with no legacy aliases.
- Updated active docs, sample config, README, and current release planning docs
  to use runtime server terminology.
- Added historical terminology notes to old release, patch, history, and
  exploration artifacts that still use BFF wording.

Historical release, patch, and history artifacts were intentionally not bulk
rewritten; the notes preserve the original wording while pointing readers to
ADR-0016 and the live `internal/runtime` / `runtime` names.

## Verification

- `go test ./...` passed.
