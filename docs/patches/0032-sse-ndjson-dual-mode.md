---
patch: "PATCH-0032"
title: "Make SSEParser pass NDJSON lines through for Ollama-style providers"
status: "proposed"
date: "2026-05-11"
related:
  - "docs/releases/v0.3.0/retrospective.md (Finding F14)"
  - "ADR-0006 (provider adapter interface)"
branch: "patch/0032-sse-ndjson-dual-mode"
---

# PATCH-0032: Make SSEParser pass NDJSON lines through for Ollama-style providers

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

When the BFF streams a turn against an Ollama-backed model, the
transcript shows the assistant chrome ("working" → "done", Cost:
$0.0000) but no response text. The persisted assistant turn has
empty `content`, and `EventTurnComplete` reports
`final_output_tokens=0`.

Tracing:

1. Ollama's `/api/chat` endpoint emits **NDJSON** — one JSON object
   per newline, no SSE framing:
   ```
   {"model":"...","message":{"content":"Hello"},"done":false}
   {"model":"...","message":{"content":" world"},"done":false}
   {"model":"...","message":{"content":""},"done":true,"eval_count":42}
   ```

2. `internal/bff/streaming.go` wraps every provider response in
   `SSEParser`. `SSEParser.Next` only returns lines prefixed with
   `data:`; lines that don't match any known SSE field (`data:`,
   `event:`, `id:`, `retry:`, `:`) fall through and are silently
   dropped.

3. Ollama's JSON-object lines start with `{`, not `data:`. Every
   chunk is discarded before `provider.ParseStreamEvent` is called.
   The relay loop sees no `StreamEventText`, accumulates empty
   content, and emits an empty `turn.complete`.

The Ollama adapter even anticipated this — see the comment at
`internal/provider/ollama.go:110-113`:
"Ollama doesn't use SSE framing; the BFF's SSEParser will still
split on blank lines, but Ollama emits one JSON per
newline-delimited line — the relay loops over these without
intermediate framing." But that hand-off was never actually wired
through; the parser silently drops the lines.

Surfaced manually during the v0.3.0 smoke test (step 3, simple
foreground turn against `qwen3.5:27b` locally). Recorded as
Finding F13 — no wait, **F14** — in
`docs/releases/v0.3.0/retrospective.md`. Local-provider streaming
is end-to-end broken.

## Scope

1. **Teach `SSEParser.Next` to recognize NDJSON.** Lines that
   start with `{` are treated as bare JSON payloads and returned
   to the caller exactly as for `data:` content. The `{` prefix is
   unambiguous against the known SSE field names
   (`data:`/`event:`/`id:`/`retry:`/`:`), so detection is just a
   prefix check — no per-provider mode flag.

2. **Add a unit test.** Feed `SSEParser` an Ollama-shaped NDJSON
   stream and assert each line is yielded as its own payload.

3. **Add a relay-level regression test.** Drive `StreamRelay`
   end-to-end through `OllamaProvider.ParseStreamEvent` with the
   NDJSON fixture and assert that `acc.Content` accumulates the
   message text. Without this, a future change could re-break the
   parser hand-off without breaking the unit test.

## Out of Scope

- **Per-provider framing config.** A `streamFormat` field on the
  provider adapter (so each adapter declares NDJSON vs SSE) is the
  cleaner architectural shape but is a larger refactor; the
  prefix-based detection is correct enough because the two framings
  are textually disjoint.

- **Ollama streaming protocol parity beyond chat.** Tool use,
  vision, embeddings, and `/api/generate` are out of scope.

- **MLX and other future local providers.** Out of scope; if they
  ship NDJSON, this fix handles them; if they ship SSE, the
  existing `data:` path already does.

## Checklist

- [ ] `SSEParser.Next` returns lines starting with `{` as
  payloads
- [ ] Unit test feeds Ollama-shaped NDJSON and asserts
  payload-by-line yield
- [ ] Relay-level test asserts `OllamaProvider` accumulates
  non-empty content end-to-end through `StreamRelay`
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/retrospective.md` F14 entry references
  this patch as fix

## Fix Detail

The parser change is one branch in `SSEParser.Next`:

```go
if strings.HasPrefix(line, "{") {
    // NDJSON: provider emitted a bare JSON object per line
    // instead of SSE framing. Pass it straight through.
    return []byte(line), nil
}
```

Order matters: this must sit before the "skip unknown SSE
field" fall-through. The existing `data:` branch is unchanged so
Anthropic and OpenAI continue to work identically.

Why prefix detection rather than a per-provider format flag? The
two framings are textually disjoint — SSE field names (`data:`,
`event:`, `id:`, `retry:`, `:`) cannot start with `{`, and any
JSON object line in NDJSON must start with `{`. The parser stays
provider-agnostic and we don't have to thread a streamFormat
parameter through `StreamRelay` and every test helper.
