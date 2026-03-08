# WU-009: Export Command

**Date:** 2026-03-08
**Role:** Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented the `modeltap export` command with JSONL and CSV output formats,
time-range filtering via `--since` and `--until` flags, and comprehensive test
coverage using in-memory SQLite stores.

## Changes

### `internal/cli/export.go`
- Replaced stub with full implementation
- Added `--format` flag (jsonl|csv, default: jsonl)
- Added `--since` and `--until` flags supporting duration shorthand (e.g., "24h", "7d") and RFC3339 timestamps
- JSONL output: one JSON object per line with fields: id, timestamp, provider, model, status, input_tokens, output_tokens, latency_ms, cost
- CSV output: header row followed by data rows with the same fields
- Store injection via package-level `SetExportStore()` function
- Invalid format produces a clear error message

### `internal/cli/export_test.go` (new)
- 11 tests covering:
  - Default format is JSONL
  - Explicit `--format jsonl` produces valid JSONL
  - `--format csv` produces valid CSV with correct header
  - `--since` duration filter (hours and days)
  - `--until` RFC3339 filter
  - Combined `--since` and `--until` range filter
  - Invalid format error
  - Empty store (JSONL and CSV)
- Uses in-memory SQLite store seeded with 3 test requests at different timestamps

### `internal/cli/root_test.go`
- Removed export from stub command test (no longer a stub; tested separately)

## Verification

- `go build ./...` succeeds
- `go test ./...` all packages pass
