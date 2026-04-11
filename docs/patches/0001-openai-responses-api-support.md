# PATCH-0001: OpenAI Responses API Support

**Status:** proposed
**Date:** 2026-04-10
**Related:** ADR-0006 (Multi-Provider Support), `docs/usage-guide.md` (Codex caveat)
**Branch:** patch/0001-openai-responses-api
**PR:** _(added when PR is created)_

## Problem

modeltap's OpenAI adapter only detects and parses OpenAI's Chat Completions endpoint (`/v1/chat/completions`). The Codex CLI — and any other client built on the OpenAI Responses API (`/v1/responses`) — sends requests to a different endpoint with a different request shape, a different response shape, and a different SSE streaming format. Today, those requests are still proxied through modeltap (the reverse proxy itself is endpoint-agnostic), but they are either not labeled as `openai` at all or are captured with empty model and token metadata. This breaks per-provider metrics for Codex users, undercounts token usage, and leaves a hole in the usage-guide promise that "pointing your tool at modeltap captures everything."

The gap was identified while documenting Codex setup in `docs/usage-guide.md` and is the most pressing missing-coverage item for the v1 OpenAI adapter. Severity: significant — affects every Codex user and any future tool built on the Responses API surface.

## Scope

- Extend `OpenAIProvider.Detect` in `internal/provider/openai.go` to also match paths containing `/v1/responses`, with the same `Anthropic-Version` header exclusion that the existing `/v1/chat/completions` branch uses.
- Add a `responsesRequest` struct alongside the existing `openaiRequest` and dispatch by body shape inside `ParseRequest`. Detection rule: if the request body contains an `input` field (and no `messages` field), parse as Responses; otherwise parse as Chat Completions. Reuse the existing `RequestMetadata` struct — `Model`, `MaxTokens` (from `max_output_tokens`), `Messages` (item count for array `input`, or `1` for string `input`), `Temperature`, `Stream`.
- Add a `responsesResponse` struct alongside the existing `openaiResponse` and dispatch by body shape inside `ParseResponse`. Detection rule: if the response body contains an `output` field (and no `choices` field), parse as Responses; otherwise parse as Chat Completions. Map `usage.input_tokens` and `usage.output_tokens` (Responses API naming) to the existing `ResponseMetadata.InputTokens` / `OutputTokens`. Map `status` to `ResponseMetadata.StopReason`.
- Add a Responses-API branch inside `ReassembleStream`. Detection rule: if any incoming `StreamChunk` has a non-empty `EventType` matching the `response.*` namespace (e.g., `response.created`, `response.output_text.delta`, `response.completed`), reassemble using Responses-API event semantics; otherwise fall through to the existing bare-`data:` Chat Completions branch.
  - From `response.created`: capture model.
  - From `response.output_text.delta` events: concatenate `delta` strings into the assembled content.
  - From `response.completed`: capture final `usage.input_tokens`, `usage.output_tokens`, and `status` as stop reason; allow the model field here to override the one from `response.created` if it differs (the completed event reports the resolved model id with date suffix).
- Add table-driven unit tests in `internal/provider/openai_test.go` covering: Responses-API detection by path, request parsing for both string `input` and array-of-items `input`, response parsing with `output` and Responses-API usage field names, and stream reassembly across a captured `response.created` → `response.output_text.delta` × N → `response.completed` event sequence.
- Add a smoke test that round-trips a Responses-API SSE capture: feed raw bytes through the proxy capture path, confirm the resulting stored `Request` row has populated `Model`, `InputTokens`, `OutputTokens`, and reassembled response body.
- Update `docs/usage-guide.md` to remove the Codex caveat about empty metadata, replacing it with a note that the Responses API is now first-class.

## Out of Scope

- **Tool calls and function calling in Responses API output** — the patch captures and counts the message-shaped output items but does not introduce new metadata fields for tool-call structures. A follow-up patch can add tool-call accounting once a consumer needs it.
- **`previous_response_id` chaining and conversation threading** — Responses API supports stateful conversations via `previous_response_id`. modeltap captures the field verbatim in the request body but does not yet model conversation graphs. Out of scope here; revisit alongside the knowledge-layer work in ADR-0008.
- **OpenAI Assistants API (`/v1/assistants`, `/v1/threads`)** — different surface, different shapes, different streaming. Track separately if a real consumer appears.
- **OpenAI Realtime API (WebSocket-based)** — out of scope for the reverse-proxy capture model entirely. Would need a separate transport adapter, not a parser extension.
- **Pricing-table updates** — existing OpenAI pricing entries cover the same model ids the Responses API reports, so no change required. If a Responses-API-only model id appears that the pricing table doesn't recognize, file as a follow-up.
- **Refactoring the OpenAI adapter into separate Chat-Completions and Responses providers** — keeping both inside one adapter with shape-based dispatch is intentional. They share authentication, host detection, model identifiers, and pricing; splitting would duplicate registration and detection logic without a real benefit. Revisit only if the Responses-API parsing grows beyond what one file can hold cleanly.

## Checklist

- [ ] `OpenAIProvider.Detect` matches `/v1/responses` paths with the same `Anthropic-Version` exclusion as `/v1/chat/completions`
- [ ] `ParseRequest` dispatches by body shape (`input` vs `messages`) and populates `RequestMetadata` for both shapes
- [ ] `ParseResponse` dispatches by body shape (`output` vs `choices`) and populates `ResponseMetadata` for both shapes, mapping Responses-API `input_tokens` / `output_tokens` correctly
- [ ] `ReassembleStream` handles `response.*` typed events and reassembles the output text from `response.output_text.delta` deltas
- [ ] Final `response.completed` event's `usage` block populates input/output token counts on the assembled `ResponseMetadata`
- [ ] Stop reason is sourced from `response.completed`'s `status` (or per-item status if the top-level is missing)
- [ ] Table-driven unit tests in `internal/provider/openai_test.go` cover detection, both request shapes, both response shapes, and stream reassembly
- [ ] Round-trip capture smoke test in `internal/proxy/` confirms a stored row has populated model, tokens, and reassembled body for a Responses-API stream
- [ ] `go test ./internal/provider/... ./internal/proxy/...` passes
- [ ] `go vet ./...` clean
- [ ] `gofmt -l` reports no diffs
- [ ] No regressions in existing Chat Completions detection or parsing (existing tests still pass unchanged)
- [ ] `docs/usage-guide.md` Codex section updated: caveat replaced with first-class support note

## Fix Detail

### Detection by body shape

Reusing one adapter for both endpoints relies on the request and response bodies being unambiguous:

- Chat Completions request: has `messages: [...]`, no `input`
- Responses request: has `input: <string|array>`, no `messages`
- Chat Completions response: has `choices: [...]`, no `output`; usage uses `prompt_tokens` / `completion_tokens`
- Responses response: has `output: [...]`, no `choices`; usage uses `input_tokens` / `output_tokens`

A small `peek` step at the top of `ParseRequest` and `ParseResponse` reads the JSON into `map[string]json.RawMessage`, checks which discriminator key is present, and routes to the appropriate typed unmarshal. If neither key is present (truncated body, error response, malformed JSON), fall back to the existing Chat Completions parser and let its error path surface.

### Stream event taxonomy

The Responses API SSE stream uses typed events distinguishable from Chat Completions by the presence of an `event:` line. The capture middleware's existing `parseSSEChunks` already populates `StreamChunk.EventType` when present (this is how the Anthropic adapter works), so the Responses-API branch keys on `chunk.EventType` matching the `response.*` namespace.

Minimum event types the patch must handle:

| Event | Used For |
|---|---|
| `response.created` | initial model name |
| `response.output_text.delta` | content accumulation (`delta` field) |
| `response.completed` | final model, usage tokens, stop reason |

Other event types (`response.in_progress`, `response.output_item.added`, `response.content_part.added`, `response.output_text.done`, `response.content_part.done`, `response.output_item.done`) are skipped; they carry no information that the assembled `ResponseMetadata` needs. Skipping unknown events keeps the parser forward-compatible with future event types.

### Why no interface change

The `Provider` interface (`internal/provider/provider.go`) does not pass the request URL or path to `ParseRequest`/`ParseResponse`. Body-shape dispatch avoids needing an interface change just to thread a path string through. If a future provider has bodies that are genuinely indistinguishable across endpoints, revisit then.
