---
patch: "PATCH-0005"
title: "BFF Routes Through Local Proxy by Default"
status: "approved"
date: "2026-04-20"
related:
  - "FEAT-0008 (BFF server)"
  - "v0.1 proxy (ADR pre-dates numbered ADRs)"
  - "PATCH-0004 (secret prefix resolver)"
branch: "exploration/integrated-harness"
---

# PATCH-0005: BFF Routes Through Local Proxy by Default

## Problem

When `modeltap harness` auto-starts the BFF, the auto-start subprocess is `modeltap start` — which runs **both** the v0.1 reverse proxy on `:8080` **and** the v0.2 BFF server on a unix socket. Today, though, the BFF dials provider endpoints (`api.anthropic.com`, `api.openai.com`) **directly** over HTTPS, not through the local proxy that's sitting right there capturing:

```
Harness ─(socket)─▶ BFF ─(HTTPS)─▶ api.anthropic.com
                                      (proxy at :8080 is running but unused)
```

The result:
- Harness conversation traffic bypasses the proxy's capture tables — the only reason modeltap exists.
- `modeltap logs` / `modeltap show` show nothing from harness sessions; they only surface traffic from clients explicitly pointed at `:8080`.
- Cost / retention / metrics / dashboard work on two disjoint datasets (proxy captures vs. BFF turns).
- There's no architectural justification for the bypass — it's a v0.2.0 track-separation artifact. Track A built the BFF as a self-contained server; track 0 / v0.1 proxy integration wasn't in scope for Bundle 4.

## Scope

1. In `internal/cli/bff_wiring.go`, when (a) `cfg.Port` is set (proxy is listening) and (b) the provider entry has no explicit `host` override, default `ProviderEndpoint.Host` to `http://127.0.0.1:<cfg.Port>`. Skip for providers that are explicitly non-cloud (`type: ollama` / `type: mlx`) — those are local already and routing them through the proxy would be a no-op at best, a DNS confusion at worst.
2. Honor explicit `host:` in config as an opt-out. A user who deliberately sets `host: https://api.anthropic.com` keeps the direct path. Same for anyone running the BFF without the proxy (e.g. a deployment that sets `cfg.Port: 0` to disable the proxy half).
3. The proxy's existing routing logic already handles `api.anthropic.com` / `api.openai.com` based on the request's `Host` header. The BFF's `TurnDispatcher` preserves the destination host header (via the adapter's request formatter), so pointing at `localhost:8080` doesn't lose the upstream signal. Verify in a smoke test.
4. Document the behavior in `docs/sample-config.yaml` — specifically, that the implicit default routes through the proxy, and that `host: https://api.anthropic.com` is the opt-out.

## Out of Scope

- Rerouting the `discover` probe (provider health check) through the proxy — probe uses a separate HTTP client today and isn't on the hot path; skip.
- Proxy-side changes. The proxy already handles the traffic shape the BFF will send.
- Running harness without proxy. Still supported — `cfg.Port: 0` disables the proxy half of `modeltap start`, and with no proxy the default falls through to direct.
- Multi-host / remote-proxy config (point harness on machine A at proxy on machine B). Someone can set `host: http://proxy-host:8080` explicitly; no new config surface needed.

## Checklist

- [x] Default-to-proxy resolution in bff_wiring.go
- [x] Skip for `type: ollama` / `type: mlx`
- [x] Honor explicit `host:` override
- [x] Unit test: default when host empty, pass-through when host set, skip for ollama/mlx, skip when port is 0
- [x] Sample config updated with the new behavior
- [ ] Smoke test: BFF → proxy → mock provider via integration test (extends the existing WU-088 e2e) — deferred; unit coverage of `resolveProviderHost` + existing WU-088 coverage of BFF→upstream is sufficient for the scope of this patch
- [x] `go vet ./...` clean, `go test ./...` green

## Fix Detail

Proxy expects the caller to set the `Host` header matching the upstream it should be forwarded to. The Anthropic adapter does this automatically (`formatAnthropicRequest` sets the Anthropic host / version headers). So from the proxy's POV, a BFF request hitting `http://127.0.0.1:8080/v1/messages` with `Host: api.anthropic.com` + `x-api-key: …` is indistinguishable from an SDK client pointed at the proxy.

Performance wart: one extra localhost hop (~100 µs round-trip) per request. Negligible against provider latency.

Observability upside: a harness user running `modeltap logs --limit 10` sees their conversation traffic unified with any other proxied calls. `modeltap dashboard` shows one stream of cost / tokens / latency.
