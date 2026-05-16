# WU-021: Show Command

**Date:** 2026-03-08
**Role:** Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented the `modeltap show <id>` command that displays full detail of a
captured request/response pair, including pretty-printed JSON bodies and
formatted headers.

## Changes

### `internal/cli/show.go`
- Replaced stub with full implementation
- Store injection via package-level `SetShowStore()` function (matches export/logs pattern)
- Fetches request by ID using `store.GetRequest()`
- Displays three sections:
  - **Header**: ID, Timestamp, Provider, Model, Status, Latency, Tokens (input/output), Cost
  - **Request**: Method, URL, Headers (formatted key: value), Body (pretty-printed JSON)
  - **Response**: Status, Headers (formatted), Body (pretty-printed JSON)
- Pretty-prints JSON bodies using `json.Indent` with 2-space indentation
- Formats headers from JSON object to indented `Key: Value` lines
- If request not found: prints "Request <id> not found" and returns error

### `internal/cli/show_test.go` (new)
- 6 tests covering:
  - Full detail display (section headers, all metadata fields, URLs, methods)
  - JSON body pretty-printing (verifies indented keys)
  - Not-found error message for missing IDs
  - Metadata display (tokens, cost, latency, provider, model, timestamp)
  - Header formatting (Content-Type displayed as `Key: Value`)
  - No-store-configured error
- Seeds in-memory SQLite store with a detailed test request including headers and bodies

### `internal/cli/root_test.go`
- Removed `show` from `TestStubCommandsOutput` since it is no longer a stub
- Added comment noting show is tested in `show_test.go`

## Verification

- `/usr/local/opt/go/bin/go build ./...` — success
- `/usr/local/opt/go/bin/go test ./...` — all packages pass
- All 6 show tests pass
