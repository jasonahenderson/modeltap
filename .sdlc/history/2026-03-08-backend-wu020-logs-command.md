# WU-020: Logs Command

**Date:** 2026-03-08
**Role:** Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented the `modeltap logs` command that lists captured request/response
logs in a formatted table with filtering support via flags.

## Changes

### `internal/cli/logs.go`
- Replaced stub with full implementation
- Added flags: `--provider`, `--model`, `--since`, `--until`, `--status` (int), `--limit` (default 50)
- Store injection via package-level `SetLogsStore()` function
- Builds `storage.ListFilter` from flag values
- Parses `--since`/`--until` as duration shorthand (7d, 24h) or RFC3339 (reuses `parseTimeFlag` from export.go)
- Displays results as a formatted table using `text/tabwriter`
- Table columns: ID (truncated to 8 chars), Timestamp, Provider, Model, Status, In Tokens, Out Tokens, Cost, Latency
- Shows "No log entries found." when results are empty

### `internal/cli/logs_test.go` (new)
- 13 tests covering:
  - Table display with header and data rows
  - ID truncation to 8 characters
  - `--provider` filter
  - `--model` filter
  - `--status` filter
  - `--since` duration filter (hours and days)
  - `--until` RFC3339 filter
  - `--limit` flag
  - Default limit is 50
  - Empty results message
  - No store configured error
  - Combined filters (provider + since)
- Uses in-memory SQLite store seeded with 3 test requests at different timestamps, providers, and statuses

### `internal/cli/root_test.go`
- Removed logs from stub command test (no longer a stub; tested separately in logs_test.go)

## Verification

- `go build ./...` succeeds
- `go test ./internal/cli/...` all 38 tests pass
- Pre-existing failure in `internal/storage` (TestPruner_PreservesRecentRecords) is unrelated to these changes
