# WU-014: Request/Response Capture Middleware

**Date:** 2026-03-08
**Role:** Designer, Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented the CaptureMiddleware that intercepts all proxied HTTP requests and responses, captures their full bodies and headers, detects the LLM provider, extracts metadata (model, tokens), and saves everything to the Store asynchronously.

## Changes

### New Files

- `internal/proxy/capture.go` — CaptureMiddleware and responseRecorder implementation
  - `CaptureMiddleware` struct with `Wrap(next http.Handler) http.Handler`
  - Request body read-and-rebuffer via `io.ReadAll` + `io.NopCloser(bytes.NewReader(...))`
  - `responseRecorder` wraps `http.ResponseWriter` to tee response body and capture status code
  - Implements `http.Flusher` interface for streaming compatibility
  - Provider detection via Registry, metadata extraction from request/response bodies
  - Asynchronous save via goroutine (fire-and-forget)
  - SSE/streaming responses (`text/event-stream`) are skipped (deferred to WU-015)
  - Headers serialized as JSON `map[string]string`

- `internal/proxy/capture_test.go` — 8 test cases
  - Saves request and response bodies to store
  - Detects Anthropic provider and extracts model, token counts, latency
  - Response returned to client unchanged (body, status, headers)
  - Unknown provider still captures raw data without metadata
  - Request body preserved for both middleware and upstream proxy
  - OpenAI provider detection and metadata extraction
  - Request and response headers captured as JSON
  - Non-success status codes (429) captured correctly

### Modified Files

- `internal/proxy/server.go` — Added `Store` and `Registry` fields to `ServerConfig`; wraps the reverse proxy handler with `CaptureMiddleware` when both are provided
- `internal/cli/start.go` — Creates SQLiteStore from config, creates provider Registry with Anthropic and OpenAI providers, passes both to `ServerConfig`

## Design Decisions

- **Async save**: The store write happens in a goroutine after the response completes, ensuring zero added latency to the proxied response.
- **Body re-buffering**: Request body is fully read into a `[]byte`, then re-wrapped as a `bytes.NewReader` so the reverse proxy can still forward it.
- **Response tee**: `responseRecorder.Write()` writes to both an internal buffer and the original `ResponseWriter` simultaneously.
- **SSE skip**: Streaming responses are detected by `Content-Type: text/event-stream` and excluded from capture (WU-015 scope).
- **Backward compatibility**: `ServerConfig.Store` and `Registry` are optional; when nil, the proxy works without capture (existing tests unaffected).
- **Temp file for tests**: Used temp-file SQLite databases in capture tests instead of `:memory:` to avoid connection pool issues with in-memory databases across goroutines.

## Test Results

All 16 proxy tests pass (8 existing + 8 new capture tests). Build succeeds with `go build ./...`.
