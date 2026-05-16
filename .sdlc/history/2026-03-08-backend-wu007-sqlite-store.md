# WU-007: SQLite Schema and Store Interface

**Date:** 2026-03-08
**Roles:** Designer, Test Engineer, Backend Implementer

## Summary

Implemented the storage layer for modeltap using SQLite (pure Go driver via modernc.org/sqlite) with a clean Store interface and full CRUD operations for captured API request/response data.

## What Was Done

### Phase 1: Design
- Defined `Store` interface in `internal/storage/store.go` with six methods: `SaveRequest`, `GetRequest`, `ListRequests`, `CountRequests`, `DeleteBefore`, `Close`
- Defined `Request` struct with 15 fields covering full request/response capture (headers, bodies, tokens, latency, cost)
- Defined `ListFilter` struct supporting provider, model, time range, status code, limit, and offset filters

### Phase 2: Tests
- Wrote comprehensive tests in `internal/storage/sqlite_test.go` (8 top-level tests, 27 sub-tests total)
- Tests cover: save/get roundtrip, auto-ID generation, not-found handling, 12 filter combinations, ordering, 7 pagination scenarios, count queries, delete-before, WAL mode verification, and interface compliance
- Used in-memory SQLite (`:memory:`) for speed; file-based DB for WAL verification
- Table-driven tests for filters, pagination, and count operations

### Phase 3: Implementation
- Installed `modernc.org/sqlite` (pure Go, no CGO) and `github.com/google/uuid`
- Implemented `SQLiteStore` in `internal/storage/sqlite.go`:
  - `NewSQLiteStore(dbPath)` opens DB, enables WAL mode, runs migrations, creates parent directories
  - Schema: `requests` table with indexes on timestamp, provider, model
  - All SQL uses parameterized queries (no string concatenation)
  - UUID generation for requests with empty IDs
  - Timestamps stored as RFC3339Nano strings
  - Home directory (`~`) expansion in db_path
  - Results ordered by timestamp descending (newest first)

## Files Created/Modified
- `internal/storage/store.go` — interface and type definitions
- `internal/storage/sqlite.go` — SQLite implementation
- `internal/storage/sqlite_test.go` — unit tests
- `go.mod` / `go.sum` — added modernc.org/sqlite, google/uuid dependencies

## Test Results
All tests pass: `go test ./...` succeeds across all packages (cli, config, storage).

## Decisions
- Used `<=` semantics for `Until` filter (inclusive upper bound)
- `GetRequest` returns `(nil, nil)` for not-found rather than an error, allowing callers to distinguish not-found from actual errors
- `ListRequests` returns results newest-first by default
- `DeleteBefore` uses strict `<` comparison (exclusive boundary)
