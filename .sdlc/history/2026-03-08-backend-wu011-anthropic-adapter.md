# WU-011: Anthropic Provider Adapter

**Date:** 2026-03-08
**Roles:** Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented the Anthropic provider adapter in `internal/provider/anthropic.go` with full test coverage in `internal/provider/anthropic_test.go`. The adapter detects Anthropic API requests, parses request/response metadata, and reassembles SSE streaming responses.

## Files Created

- `internal/provider/anthropic.go` — Anthropic provider adapter implementation
- `internal/provider/anthropic_test.go` — 20 unit tests with table-driven test cases

## Implementation

### Detection (`Detect`)

Three detection strategies, any of which triggers a match:

1. **Host-based**: `r.Host` or `r.URL.Host` equals `api.anthropic.com`
2. **Header-based**: presence of `anthropic-version` header
3. **Path + header**: URL path contains `/v1/messages` AND request has `x-api-key` header

### Request Parsing (`ParseRequest`)

Extracts from JSON body:
- `model` — model identifier (e.g., `claude-sonnet-4-20250514`)
- `max_tokens` — maximum output tokens
- `messages` — count of conversation messages
- `stream` — streaming flag
- `temperature` — optional sampling temperature
- `system` — system prompt (string form)

### Response Parsing (`ParseResponse`)

Extracts from JSON body:
- `model` — model used for generation
- `usage.input_tokens` / `usage.output_tokens` — token counts
- `stop_reason` — why generation stopped (`end_turn`, `max_tokens`, etc.)

### Stream Reassembly (`ReassembleStream`)

Handles Anthropic's SSE event types:
- `message_start` — extracts model and input token count
- `content_block_delta` — appends `delta.text` to build full response
- `message_delta` — extracts output token count and stop reason
- `content_block_start`, `content_block_stop`, `message_stop`, `ping` — acknowledged but no metadata extracted

Unknown event types are silently ignored for forward compatibility.

## Test Coverage

20 tests, all passing:

### Name (1 test)
- Returns `"anthropic"`

### Detect (7 tests)
- Matches `api.anthropic.com` host (both `r.Host` and `r.URL.Host`)
- Matches `anthropic-version` header
- Matches `/v1/messages` path with `x-api-key` header
- Matches `/v1/messages` subpath with `x-api-key`
- No match for unrelated host/headers
- No match for `/v1/messages` without `x-api-key`

### ParseRequest (4 tests)
- Typical single-message request
- Streaming request with system prompt and temperature
- Multi-turn conversation (5 messages)
- Invalid JSON error handling

### ParseResponse (3 tests)
- Successful `end_turn` response with token counts
- `max_tokens` stop reason response
- Invalid JSON error handling

### ReassembleStream (6 tests)
- Full streaming conversation (message_start, ping, content_block_start/delta/stop, message_delta, message_stop)
- Stream with `max_tokens` stop
- Empty chunks (zero-value metadata)
- Invalid JSON in message_start, content_block_delta, message_delta

### Compile-time interface check
- `var _ Provider = (*AnthropicProvider)(nil)`

## Build Verification

- `go build ./...` — passes
- `go test ./internal/provider/ -run Anthropic` — 20/20 PASS
- Pre-existing OpenAI test file references undefined types (unrelated to this work unit)
