# 2026-05-12 Session: PATCH-0036 run proxy correlation

## Summary

Created a separate worktree and branch for PATCH-0036, then implemented the
patch to correlate raw proxy captures with durable run history by carrying
`run_id` and `trace_id` through storage, capture, BFF dispatch, CLI request
filtering, and export/detail surfaces.

## Artifacts

- Worktree: `/Users/jasonhenderson/Projects/jasonahenderson/modeltap-patch-0036-run-proxy-correlation`
- Branch: `patch/0036-run-proxy-correlation`
- Patch doc: `docs/patches/0036-run-proxy-correlation.md`

## Implementation

- Added SQLite schema version 4 with `requests.run_id`, `requests.trace_id`,
  and indexes for both fields.
- Added `RunID` and `TraceID` to captured request storage, request filters,
  and request detail/export output.
- Added private BFF-to-proxy correlation headers that are stamped only for
  local proxy-routed provider endpoints and stripped before upstream forwarding.
- Added `requests list --run`, `requests list --trace`, and matching export
  filters.
- Marked `PATCH-0036` done and updated the patch index.

## Verification

- `go test ./internal/storage ./internal/proxy ./internal/bff ./internal/cli`
- `go test ./...`
