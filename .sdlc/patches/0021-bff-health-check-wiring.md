---
patch: "PATCH-0021"
title: "Wire provider health checks in BFF production startup"
status: "approved"
date: "2026-05-08"
related:
  - "FEAT-0008 (BFF server)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F1)"
branch: "patch/0021-bff-health-check-wiring"
---

# PATCH-0021: Wire provider health checks in BFF production startup

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

`internal/cli/bff_wiring.go` registers provider endpoints and
refreshes the model registry, but it never calls
`srv.Providers().StartHealthChecks(...)`. As a result every endpoint
stays at the zero-value `ProviderStatus` (reported as `"unavailable"`),
the model registry's built-in catalog reports every model as
unavailable, and Ollama/MLX auto-discovery (which depends on the
health-check probe populating `ep.Models()`) never runs. `model.list`
therefore returns either an empty or all-unavailable catalog, and
`/models` in the production shell appears non-functional.

This was caught by the v0.3.0 manual smoke test and is recorded as
Finding F1 in `.sdlc/releases/v0.3.0/retrospective.md`. Production
wiring forgotten — same shape as F5.

## Scope

1. **`internal/cli/bff_wiring.go`** — call
   `srv.Providers().StartHealthChecks(0)` after the endpoint Add
   loop and before `srv.Models().Refresh()`. Default interval (60s)
   is appropriate; the initial synchronous CheckAll happens before
   the call returns, so the immediately-following Refresh sees
   current status and discovered model lists.

2. **`internal/bff/server.go`** — `Server.Shutdown` now calls
   `s.providers.Stop()` so the background poll goroutine exits
   cleanly on shutdown. Stop is a no-op if StartHealthChecks was
   never called (safe for tests).

## Out of Scope

- **Cloud provider probe target (F3).** `resolveProviderHost`
  defaults Anthropic/OpenAI to `http://127.0.0.1:<proxyPort>` so
  traffic flows through the capture pipeline; the health-check
  probe inherits that target and gets a 404 from the proxy on a
  HEAD with empty path. This patch surfaces the F3 evidence — every
  cloud probe in the captured-requests table is a 404 — but the
  fix to the probe target itself is a separate patch.
- **Per-provider health-check tuning.** The default 60s interval
  is fine for v0.3.0; configuration knobs are deferred.
- **Manual reload / `modeltap status` probe (F4).** Separately
  scoped under retrospective Recommendation 10.

## Checklist

- [ ] `srv.Providers().StartHealthChecks(0)` called in
  `bff_wiring.go` before `srv.Models().Refresh()`
- [ ] `Server.Shutdown` calls `s.providers.Stop()`
- [ ] `go test ./...` passes
- [ ] Smoke verification: `model.list` over the BFF socket returns
  a populated catalog with Ollama models marked `"ready"`
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` Finding F1 status
  updated to "Fixed in PATCH-0021"
