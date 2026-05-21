---
patch: "PATCH-0025"
title: "Probe cloud-provider upstream directly in health check"
status: "approved"
date: "2026-05-08"
related:
  - "PATCH-0005 (route BFF provider traffic through proxy by default)"
  - "PATCH-0021 (health-check wiring)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F3)"
branch: "patch/0025-cloud-probe-upstream-target"
---

# PATCH-0025: Probe cloud-provider upstream directly in health check

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

Cloud-provider health checks always report `unavailable` even when
the API key is valid. The probe sends `HEAD <ep.Host>` and
`ep.Host` is auto-set in `internal/cli/bff_wiring.go` to
`http://127.0.0.1:<proxyPort>` per PATCH-0005 (route BFF traffic
through the v0.1 capture proxy by default). The proxy responds 404
to a HEAD with empty path; the probe interprets that as
`HTTP 404` and the endpoint stays unavailable.

Confirmed by inspecting captured requests after PATCH-0019:

```
ID        TIMESTAMP             PROVIDER   MODEL  STATUS  LATENCY
9b02141b  2026-05-08T19:20:43Z                    404     138ms
cd3ded6f  2026-05-08T19:20:43Z  anthropic         404     138ms
```

Two probe rows per minute (one per cloud provider), both 404 from
the local proxy. Recorded as Finding F3 in
`.sdlc/releases/v0.3.0/retrospective.md`.

The endpoint's `Host` is the **dispatch** target (the BFF sends
turn payloads through it; the proxy then forwards to upstream).
The probe needs a separate **upstream** target so it can verify
the credentialed cloud API is reachable, independent of the local
proxy's HTTP routing.

## Scope

1. **Add `Upstream string` field** to
   `internal/bff/providers.go` `ProviderEndpoint`. It carries the
   canonical upstream URL used by health-check probes; `Host`
   continues to control dispatch routing.

2. **Populate `Upstream` in `internal/cli/bff_wiring.go`** at
   endpoint construction time:
   - If `pc.Upstream` is explicitly set in user config, use that.
   - Else fall back to `defaultHostFor(pc.Type)` (the canonical
     `https://api.anthropic.com` / `https://api.openai.com` /
     etc.).

3. **Use `Upstream` in `checkCloudEndpoint`** in
   `internal/bff/providers.go` instead of `ep.Host`. The dispatch
   path remains unchanged; only the probe URL changes.

4. **Tests** in `internal/bff/providers_test.go`:
   - Probe URL derives from `Upstream` not `Host` (use httptest
     to capture the request and assert the URL).
   - Explicit `pc.Upstream` overrides the default.

## Out of Scope

- **Probing Ollama / MLX endpoints differently.** Those already
  probe `ep.Host` directly (no proxy interposition for local
  providers); no change needed.
- **Verifying API key validity.** The probe still treats
  HTTP <500 as reachable; an invalid key produces 401 which is
  still "reachable." Validating credentials is a richer check
  deferred to a follow-up.
- **Reflecting Upstream in `modeltap config show`.** The shown
  config still surfaces `host` as user-facing config. The new
  `Upstream` field is a derived BFF runtime concept.

## Checklist

- [ ] `ProviderEndpoint.Upstream` field added
- [ ] `bff_wiring.go` populates `Upstream` from
  `pc.Upstream` || `defaultHostFor(pc.Type)`
- [ ] `checkCloudEndpoint` uses `Upstream` for probe URL
- [ ] Tests cover probe target and explicit upstream override
- [ ] `go test ./...` passes
- [ ] Smoke verification: after restart, model.list shows cloud
  models marked `"ready"` (Anthropic, OpenAI) instead of
  `"unavailable"`
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` Finding F3 status
  updated to "Fixed in PATCH-0025"
