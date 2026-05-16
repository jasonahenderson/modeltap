# WU-015: SSE Stream Capture

**Date:** 2026-03-08
**Role:** Designer, Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Extended the capture middleware to handle SSE (Server-Sent Events) streaming responses. Streaming chunks are forwarded to the client immediately with zero added latency while being buffered for post-stream reassembly. After the stream ends, the buffered SSE data is parsed into `StreamChunk`s, passed to the provider's `ReassembleStream()` method, and the resulting full response text and metadata are saved to the Store.

## Changes

### Modified Files

- `internal/proxy/capture.go`
  - Removed the early-return SSE skip (previously deferred to this WU)
  - Added SSE detection branch: when `Content-Type` contains `text/event-stream`, the buffered body is parsed into `StreamChunk`s and reassembled via the provider adapter
  - Added `parseSSEChunks()` function that splits raw SSE bytes on `\n\n` boundaries, extracts `event:` and `data:` fields, and produces `provider.StreamChunk` values
  - Anthropic format: `EventType` populated from `event:` line, `Data` contains JSON payload only
  - OpenAI format: `EventType` left empty, `Data` contains raw `data: ...` line (as OpenAI's `ReassembleStream` expects)
  - Unknown provider SSE: raw SSE data saved as `ResponseBody` with no metadata extraction
  - Non-streaming path unchanged

### New Files

- `internal/proxy/stream_capture_test.go` — 6 test cases
  - `TestStreamCapture_Anthropic`: Full Anthropic SSE stream (message_start, content_block_delta, message_delta, message_stop) captured, reassembled, metadata extracted (model, input/output tokens)
  - `TestStreamCapture_OpenAI`: Full OpenAI SSE stream (data chunks, usage, [DONE]) captured and reassembled with correct metadata
  - `TestStreamCapture_ChunksFlushedImmediately`: Verifies chunks are forwarded to client without buffering delay by measuring inter-chunk timing
  - `TestStreamCapture_NonSSEStillHandled`: Confirms non-streaming JSON responses still use the existing capture path
  - `TestStreamCapture_ReassembledResponseSavedWithMetadata`: Verifies all record fields (provider, model, tokens, status, method, headers, request body) are populated
  - `TestStreamCapture_UnknownProviderSSE`: SSE from unknown provider saves raw data without metadata

## Design Decisions

- **Zero-latency forwarding**: The `responseRecorder` already implements `http.Flusher`, so each chunk written by the upstream handler is immediately flushed to the client. Buffering is only for the capture copy.
- **Post-stream reassembly**: Parsing happens after `ServeHTTP` returns (i.e., after the upstream closes the connection), so it does not affect streaming performance.
- **Provider-aware chunk format**: The SSE parser produces chunks in the format each provider's `ReassembleStream` expects — Anthropic gets event type + JSON payload, OpenAI gets raw `data:` lines.
- **ADR-0005 compliance**: Full capture with no data loss. Every SSE event is buffered and the complete reassembled response is saved.

## Test Results

All 6 new streaming tests pass. All existing tests continue to pass. Build succeeds with no errors.
