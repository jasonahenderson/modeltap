# WU-016: Multi-Provider Routing

**Date:** 2026-03-08
**Roles:** Designer, Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Extended the reverse proxy to route requests to different upstream servers based on provider detection. Previously all requests were forwarded to a single `upstream` URL. Now the proxy detects the provider from the incoming request (using the existing provider registry) and routes to a provider-specific upstream if configured, falling back to the default upstream otherwise.

## Changes

### Config (`internal/config/config.go`)
- Added `ProviderConfig` struct with an `Upstream` field
- Added `Providers map[string]ProviderConfig` field to the `Config` struct
- Supports YAML config like:
  ```yaml
  providers:
    anthropic:
      upstream: https://api.anthropic.com
    openai:
      upstream: https://api.openai.com
  ```

### Proxy Server (`internal/proxy/server.go`)
- Added `ProviderUpstreams map[string]string` to `ServerConfig`
- Added `providerUpstreams map[string]*url.URL` and `registry` fields to `Server`
- Provider upstream URLs are parsed and validated at startup
- Replaced the single-host Director with a multi-provider Director that:
  1. Detects the provider from the incoming request via the registry
  2. Looks up the provider-specific upstream URL
  3. Rewrites `req.URL.Scheme`, `req.URL.Host`, and `req.Host` to the target upstream
  4. Falls back to the default upstream if no provider match or no provider-specific upstream

### CLI (`internal/cli/start.go`)
- Builds `providerUpstreams` map from `cfg.Providers` and passes it to `proxy.NewServer`

### Tests (`internal/proxy/routing_test.go`)
- `TestRoutingAnthropicRequestsToAnthropicUpstream` - Anthropic requests (detected by `anthropic-version` header) route to Anthropic upstream
- `TestRoutingOpenAIRequestsToOpenAIUpstream` - OpenAI requests (detected by `/v1/chat/completions` path) route to OpenAI upstream
- `TestRoutingFallbackToDefaultUpstream` - Unknown provider requests fall back to default upstream
- `TestRoutingWithNoProviderUpstreams` - When no provider upstreams are configured, all requests go to default
- `TestRoutingWithNilRegistry` - When no registry is set, all requests go to default

All tests use `httptest.NewServer` for mock upstreams (one per provider plus default).

## Verification

- `go build ./...` passes
- All new routing tests pass
- All existing server tests pass (no regressions)
- Pre-existing test failures in `cli/root_test.go` and `capture_test.go` are unrelated
