# WU-013: Basic Reverse Proxy

**Date**: 2026-03-08
**Role**: Designer, Test Engineer, Backend Implementer
**Status**: Complete

## Summary

Implemented the core reverse proxy server using `httputil.ReverseProxy` from the standard library. The `modeltap start` command now launches a functioning HTTP reverse proxy that forwards all requests to the configured upstream and returns responses unmodified.

## Files Created

- `internal/proxy/server.go` - Proxy server implementation

## Files Modified

- `internal/cli/start.go` - Wired proxy server with signal handling
- `internal/cli/root_test.go` - Removed start from stub test (no longer a stub)

## Test File Created

- `internal/proxy/server_test.go` - 7 test functions covering forwarding, headers, status codes, response bodies, request bodies, validation, and graceful shutdown

## Design

### ServerConfig / Server

```go
type ServerConfig struct {
    Port        int
    UpstreamURL string
}

type Server struct {
    proxy    *httputil.ReverseProxy
    upstream *url.URL
    port     int
    server   *http.Server
}
```

- `NewServer(cfg)` validates config, creates `httputil.NewSingleHostReverseProxy`, customizes the Director to set `req.Host` to the upstream host
- `Start()` blocks on `ListenAndServe`
- `Shutdown(ctx)` performs graceful shutdown
- `Handler()` exposes the HTTP handler for testing without binding a port

### CLI Integration

The start command:
1. Loads config (flags > env > file > defaults)
2. Creates proxy server
3. Registers SIGINT/SIGTERM handler via `signal.NotifyContext`
4. Starts server in a goroutine
5. Blocks until signal or server error
6. On signal, performs graceful shutdown with 10-second timeout

## Test Coverage

| Test | What it verifies |
|------|-----------------|
| TestProxyForwardsRequests | Requests reach upstream, response body returned |
| TestProxyForwardsHeaders | Authorization, Content-Type, custom headers preserved |
| TestProxyPreservesStatusCodes | 200, 400, 401, 404, 429, 500 all pass through |
| TestProxyReturnsResponseBody | JSON response body and Content-Type header intact |
| TestNewServerValidation | Rejects empty upstream, invalid port, missing scheme |
| TestGracefulShutdown | Shutdown completes without error |
| TestProxyForwardsRequestBody | POST request body arrives at upstream intact |

## Decisions

- Used `httputil.NewSingleHostReverseProxy` per ADR-0001
- Set `req.Host` to upstream host in Director (required by most API providers)
- No capture/logging middleware added (deferred to WU-014)
- No provider detection at proxy level (deferred to WU-014)
- Graceful shutdown uses 10-second timeout for connection draining

## Verification

- `go build ./...` passes
- `go test ./...` passes (all packages)
