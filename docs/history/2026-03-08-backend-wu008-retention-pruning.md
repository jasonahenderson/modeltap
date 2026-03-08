# WU-008: Retention Pruning

**Date:** 2026-03-08
**Roles:** Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented a background retention pruner that periodically deletes records older than a configurable retention period, as specified by ADR-0005.

## Files Created

- `internal/storage/pruner.go` — Pruner struct and implementation
- `internal/storage/pruner_test.go` — Five test cases covering all requirements

## Implementation Details

### Pruner (`internal/storage/pruner.go`)

- `Pruner` struct holds a `Store`, `retentionDays int`, and `interval time.Duration`
- `NewPruner(store, retentionDays, interval)` constructor
- `Start(ctx)` launches a background goroutine that:
  - Returns immediately (no-op) if `retentionDays <= 0`
  - Runs an initial prune on startup, then on each tick of the configured interval
  - Calls `store.DeleteBefore()` with a cutoff of `now - retentionDays`
  - Logs pruned record count via `log/slog` each cycle
- `Stop()` cancels the goroutine context and waits via `sync.WaitGroup`

### Tests (`internal/storage/pruner_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestPruner_DeletesOldRecords` | Records older than retention period are deleted; recent ones preserved |
| `TestPruner_PreservesRecentRecords` | All records within retention window survive pruning |
| `TestPruner_StopsCleanlyOnContextCancel` | Pruner goroutine exits promptly when context is cancelled |
| `TestPruner_ZeroRetentionDays_NoPruning` | `retentionDays=0` means keep forever, no deletions |
| `TestPruner_NegativeRetentionDays_NoPruning` | Negative values also mean keep forever |

All tests use in-memory SQLite with short intervals (50ms) for fast execution.

## Verification

- `go build ./...` — passes
- `go test ./internal/storage/...` — all 17 tests pass (5 new pruner + 12 existing)

## Definition of Done Checklist

- [x] Pruner runs on configurable interval
- [x] Records older than retention_days are deleted
- [x] Pruner stops cleanly on shutdown
- [x] retentionDays=0 means no pruning
- [x] All tests pass
