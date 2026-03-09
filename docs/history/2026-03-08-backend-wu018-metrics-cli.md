# WU-018: Metrics CLI Commands

**Date:** 2026-03-08
**Roles:** Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented the `modeltap metrics` command and `modeltap metrics rebuild` subcommand, replacing the previous stubs. The metrics command queries daily (or hourly) aggregation data from the store and displays it in table, JSON, or CSV format with filtering by time range and grouping options.

## Files Modified

- `internal/cli/metrics.go` — Replaced stub with full implementation: added `metricsStore` package variable with `SetMetricsStore()` injection; added `--since`, `--until`, `--group-by` (provider|model|day|hour), and `--format` (table|json|csv) flags; default behavior queries daily metrics for last 30 days displayed as a table using `text/tabwriter`; `rebuild` subcommand calls `store.RebuildMetrics()` and prints a success message

- `internal/cli/root_test.go` — Removed `metrics` and `metrics rebuild` entries from `TestStubCommandsOutput` since they are no longer stubs

## Files Created

- `internal/cli/metrics_test.go` — 12 test cases using an in-memory SQLite store seeded with test requests across 3 days, 2 providers, and 2 models: table display, `--since` filter, `--until` filter, `--group-by provider`, `--group-by model`, `--group-by hour`, `--format json` (validates JSON unmarshaling), `--format csv` (validates CSV parsing), `metrics rebuild`, error cases for no store and invalid group-by, empty results

## Test Results

All 60 CLI tests pass. All storage tests continue to pass.
