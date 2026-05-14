---
patch: "PATCH-0022"
title: "Set default max_tokens for turn.submit dispatch"
status: "approved"
date: "2026-05-08"
related:
  - "FEAT-0008 (BFF server)"
  - "docs/releases/v0.3.0/retrospective.md (Finding F7)"
branch: "patch/0022-turn-submit-max-tokens-default"
---

# PATCH-0022: Set default max_tokens for turn.submit dispatch

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

Every `turn.submit` against an Anthropic provider returns
`HTTP 400: provider anthropic returned HTTP 400` because the BFF
sends `"max_tokens": 0` on the wire. Captured request body, taken
directly from the requests table during smoke-test debug:

```json
{
  "model": "claude-sonnet-4-6",
  "max_tokens": 0,
  "system": "...",
  "messages": [...],
  "stream": true
}
```

Anthropic requires `max_tokens` to be a positive integer; zero is
rejected. The BFF's turn-submit handler in `internal/bff/turn.go`
constructs `DispatchOpts` without setting `MaxTokens`, so it defaults
to Go's zero value and that zero flows through the dispatcher and
provider adapter onto the wire.

The transform handler in `internal/bff/transform.go` already
applies a sensible fallback (`if maxTokens <= 0 { maxTokens = 1024 }`)
for `content.transform`. The turn.submit path does not.

This was caught by the v0.3.0 manual smoke test by inspecting
captured request bodies after PATCH-0019 made the requests CLI
functional. Recorded as Finding F7 in
`docs/releases/v0.3.0/retrospective.md`.

## Scope

1. **`internal/bff/turn.go`** — set `MaxTokens` on the
   `dispatchOpts` value before dispatching. Use `4096` as a
   conservative default that all v0.3.0 catalog models accept and
   that does not over-allocate.

2. **No new config knob.** Per-model or config-level overrides are
   out of scope; a single sane default unblocks turn.submit and
   leaves room for a follow-up that lets users tune max_tokens via
   `bff.max_tokens` or per-model entries.

## Out of Scope

- **Per-model defaults.** Sonnet 4-6 supports much larger output
  windows than 4096; some Ollama/local models support smaller. A
  per-model lookup against `entry.Info` (or new metadata) is a
  follow-up.
- **Config-level override.** A `bff.max_tokens` field would let
  users tune; not in this patch.
- **Harness-side `max_tokens` plumbing.** The harness does not
  currently set `max_tokens` on its turn.submit request; if it
  ever does, the harness value should win. For now the BFF default
  is the only source.
- **F8 (`/models` submitted as user content).** Separate defect:
  the shell submits slash commands as turn content instead of
  dispatching them as host commands. Filed in retrospective; not
  this patch.

## Checklist

- [ ] `internal/bff/turn.go` sets `MaxTokens` (e.g., 4096) on
  `dispatchOpts` before `srv.dispatch.Dispatch(...)`
- [ ] `go test ./...` passes
- [ ] Smoke verification: a turn.submit against Anthropic produces
  a 200 (or any non-400) response and captured request body shows
  a positive `max_tokens` value
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/retrospective.md` Finding F7 status
  updated to "Fixed in PATCH-0022"
