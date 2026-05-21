---
patch: "PATCH-0013"
title: "Set SQLite `busy_timeout` on every pool connection"
status: "proposed"
date: "2026-04-22"
related:
  - "ADR-0002 (SQLite storage)"
  - "PATCH-0012 (surfaced this failure in `make` default target)"
branch: "exploration/integrated-harness"
---

# PATCH-0013: Set SQLite `busy_timeout` on every pool connection

## Problem

`internal/storage/sqlite.go:37` opens the database with:

```go
dsn := dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
```

The DSN does not set `busy_timeout`. `modernc.org/sqlite` (the driver chosen in ADR-0002) inherits SQLite's default of **0 milliseconds**, meaning any connection that encounters a lock returns `SQLITE_BUSY (5)` immediately instead of briefly waiting for the writer ahead of it.

Under concurrent writes this produces two real, reproducible failures:

1. **Dropped captures in production.** The proxy's entire job is to capture request/response pairs from concurrent upstream traffic. When two upstreams finish close together, the second goroutine's `INSERT` racing the first wins or loses on a coin flip: if it loses the lock race it returns `SQLITE_BUSY` and the capture is lost. The failing integration test logs the production-relevant symptom verbatim: `ERROR failed to save captured request error="inserting request: database is locked (5) (SQLITE_BUSY)"`.
2. **Integration test flake.** `TestMetricsAggregation` at `internal/integration/integration_test.go:535` fires 3 concurrent `POST /v1/messages` requests, expects 3 rows to land in the store, and times out waiting for the third because two of the three capture goroutines collide and one gets `SQLITE_BUSY`. A follow-on `disk I/O error (1802)` (`SQLITE_IOERR_BLOCKED`) sometimes appears when WAL recovery on the next test encounters a still-locked connection from the previous failure — same root cause.

This is a real storage bug, not a test bug. The test happens to be the cheapest way to reproduce it on a single machine.

## Scope

1. **Add `_pragma=busy_timeout(5000)` to the DSN** in `internal/storage/sqlite.go:37`. Every connection in the pool will wait up to 5 seconds for a lock before returning `SQLITE_BUSY`.
2. **Add `TestBusyTimeoutConfigured` to `internal/storage/sqlite_test.go`**. Opens a new store, queries `PRAGMA busy_timeout`, asserts it returns `5000` on both in-memory and file-backed databases.
3. **No schema changes.** `busy_timeout` is a connection-scoped setting, not stored in the database file. No migration required.
4. **No API changes.** `NewSQLiteStore` signature and callers unchanged.

## Out of Scope

- **Making `busy_timeout` configurable.** The value is deliberately hardcoded. 5000ms is a conservative ceiling that covers any realistic WAL-serialized write burst; nothing in the product's usage pattern benefits from tuning it per deployment.
- **Fixing the separate test flakes** that may surface once this is fixed. If any remain, they are distinct race conditions with distinct root causes; address them individually.
- **Retry logic in capture writers.** With `busy_timeout` set, retries would rarely fire. Adding them now would be speculation; revisit only if a real `SQLITE_BUSY` is observed in production after this patch.
- **Tuning `PRAGMA synchronous`, `cache_size`, `mmap_size`, or other SQLite performance pragmas.** Out of scope; separate consideration.

## Checklist

- [ ] DSN in `internal/storage/sqlite.go` includes `_pragma=busy_timeout(5000)`
- [ ] New test `TestBusyTimeoutConfigured` asserts `PRAGMA busy_timeout = 5000` on fresh in-memory and file-backed stores
- [ ] `TestMetricsAggregation` passes deterministically (it's the implicit regression test — confirm it stops timing out under `go test -race`)
- [ ] `make fmt-check vet test build` passes
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.2.0/changelog.md` entry added

## Fix Detail

### The one-line change

Before:
```go
dsn := dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
```

After:
```go
dsn := dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
```

### Why 5000ms

- SQLite's own documentation (`https://www.sqlite.org/c3ref/busy_timeout.html`) treats `busy_timeout` as a correctness control, not a performance knob. The value needs to be "large enough to absorb realistic writer-ahead-of-me delays."
- In WAL mode, writers serialize at the page level and commits are typically sub-millisecond. A 5s ceiling means "if we can't get the lock in 5 seconds something is wrong" — it's insurance, not active latency.
- `mattn/go-sqlite3` ships default examples with 5000ms; community convention in Go + SQLite is strong here.
- Lower (100-500ms) is tempting but risks genuine concurrent bursts (e.g., burst of captures during a traffic spike) failing spuriously. Higher (30s+) risks masking deadlocks.

### Why a DSN pragma rather than `db.Exec("PRAGMA busy_timeout = 5000")`

`busy_timeout` is connection-scoped. Go's `database/sql` maintains a pool of connections; `db.Exec` sets the pragma on whichever connection happens to serve that call, which is the wrong connection by the time real work starts. DSN pragmas are replayed by `modernc.org/sqlite` on **every** connection open, which is what we want.

This is the same reason `foreign_keys` and `journal_mode` already live in the DSN — connection-scoped settings must be set per-connection.

### Why this is a proper storage fix, not a test fix

The symptom surfaces in a test, but the root cause is "the production capture path silently drops data under concurrent upstream traffic." If the test were adjusted to tolerate the flake (e.g., send only one request at a time), the production bug would remain. Fixing the storage DSN is the correct layer.

### Cross-check against existing WAL tests

`TestWALModeEnabled` (`internal/storage/sqlite_test.go:435`) verifies `journal_mode = wal` on both in-memory and file-backed stores. The new `TestBusyTimeoutConfigured` follows the exact same pattern: open → query PRAGMA → assert. Two parallel tests for the two connection-scoped settings we rely on.
