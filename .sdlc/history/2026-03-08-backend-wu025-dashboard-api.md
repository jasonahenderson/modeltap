# WU-025: Dashboard API Endpoints

**Date:** 2026-03-08
**Role:** Designer, Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented REST API endpoints for the modeltap web dashboard, providing JSON APIs for log browsing, metrics, and proxy status.

## Files Created

- `internal/dashboard/api.go` — API handler with all four endpoints
- `internal/dashboard/api_test.go` — 15 tests covering all endpoints

## Endpoints Implemented

| Endpoint | Method | Description |
|---|---|---|
| `/api/logs` | GET | Paginated, filterable list of captured requests |
| `/api/logs/{id}` | GET | Full detail of a single request |
| `/api/metrics` | GET | Aggregated usage metrics with group_by support |
| `/api/status` | GET | Proxy config, database record count, retention settings |

### `/api/logs` Query Parameters
- `provider`, `model` — filter by provider/model
- `since`, `until` — RFC3339 time range filter
- `status` — filter by HTTP status code
- `limit` (default 50), `offset` — pagination

### `/api/metrics` Query Parameters
- `since`, `until` — RFC3339 time range filter
- `group_by` — one of `hour`, `day`, `provider`, `model` (default: `day`)

## Design Decisions

- Used Go 1.22+ `http.ServeMux` method-based routing (`GET /api/logs/{id}`)
- All responses set `Content-Type: application/json`
- Error responses return JSON `{"error": "message"}` with appropriate HTTP status codes (400, 404, 500)
- `ListenAndServe` binds to `127.0.0.1` by default for security
- Pagination returns `total` count independently of `limit`/`offset` for client-side pagination UI
- Metrics group_by `provider`/`model` aggregates daily metrics across time periods
- No direct SQL — all queries go through the `storage.Store` interface

## Test Coverage

- 15 tests, all passing
- Tests use in-memory SQLite with seeded test data (10 requests across 2 providers)
- Covers: JSON format, pagination, all filters, detail view, 404 handling, metrics group_by modes, time filters, status endpoint config values, input validation (400 errors)

## Verification

```
go build ./...  — PASS
go test ./internal/dashboard/ — 15/15 PASS
```

Pre-existing failures in `internal/integration` and `internal/proxy` are unrelated to this work unit.
