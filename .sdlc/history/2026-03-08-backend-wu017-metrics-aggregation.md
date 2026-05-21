# WU-017: Metrics Aggregation Tables

**Date:** 2026-03-08
**Roles:** Designer, Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Added pre-computed hourly and daily aggregation tables to the SQLite storage layer, per ADR-0007. Each request write atomically updates both rollup tables. Query methods and a full rebuild capability are provided.

## Files Modified

- `internal/storage/store.go` — Added `UsageMetrics`, `MetricsFilter` types and `QueryHourlyMetrics`, `QueryDailyMetrics`, `RebuildMetrics` to the `Store` interface

- `internal/storage/sqlite.go` — Added `hourly_usage` and `daily_usage` table migrations; modified `SaveRequest` to use a transaction with upsert statements for both aggregation tables; implemented `QueryHourlyMetrics`, `QueryDailyMetrics`, and `RebuildMetrics`

## Files Created

- `internal/storage/metrics_test.go` — Seven test cases covering all requirements

## Implementation Details

### Aggregation Tables

- `hourly_usage` — keyed by `(hour, provider, model)` where hour is RFC3339 truncated to hour
- `daily_usage` — keyed by `(day, provider, model)` where day is `YYYY-MM-DD` format

Both tables track: `request_count`, `input_tokens`, `output_tokens`, `estimated_cost_usd`, `total_latency_ms`, `error_count`.

### SaveRequest Changes

- Wraps the insert and both upserts in a single database transaction
- Uses `INSERT ... ON CONFLICT ... DO UPDATE` (upsert) to atomically increment counters
- Error count increments when `response_status >= 400`

### Query Methods

- `QueryHourlyMetrics` / `QueryDailyMetrics` accept `MetricsFilter` with optional `Since`, `Until`, `Provider`, `Model` fields
- Results are grouped by period/provider/model and include computed `AvgLatencyMs` (total_latency_ms / request_count)

### RebuildMetrics

- Deletes all rows from both aggregation tables
- Re-aggregates from the `requests` table using `INSERT ... SELECT ... GROUP BY` with `strftime` for time bucketing
- Runs in a single transaction

## Tests (`internal/storage/metrics_test.go`)

1. `TestSaveRequest_UpdatesAggregationTables` — verifies a single save populates both tables
2. `TestAggregation_SameHourSameModel` — verifies multiple requests aggregate into one row
3. `TestAggregation_DifferentModelsProviders` — verifies separate rows per provider/model
4. `TestQueryHourlyMetrics_TimeFilters` — verifies Since/Until filtering
5. `TestQueryDailyMetrics_ProviderModelFilters` — verifies provider/model filtering
6. `TestRebuildMetrics` — corrupts aggregation, rebuilds, verifies correctness
7. `TestErrorCount_IncrementForNon2xx` — verifies error_count for status codes 400, 429, 500

## Verification

- `go build ./...` passes
- `go test ./internal/storage/...` passes (all storage tests green)
- Pre-existing failures in `internal/cli` and `internal/proxy` are unrelated
