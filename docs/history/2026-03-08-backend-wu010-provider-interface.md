# WU-010: Provider Interface Definition

**Date:** 2026-03-08
**Roles:** Designer, Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Defined the Provider adapter interface, supporting types, and a thread-safe Registry in `internal/provider/`. This provides the foundation for provider-specific adapters (Anthropic in WU-011, OpenAI in WU-012) without implementing any actual adapters.

## Files Created

- `internal/provider/provider.go` — Provider interface and metadata types
- `internal/provider/registry.go` — Thread-safe provider registry
- `internal/provider/registry_test.go` — 11 unit tests covering all registry behavior

## Design Decisions

### Provider Interface

Five methods as specified by ADR-0006:

- `Name()` — unique provider identifier string
- `Detect(*http.Request)` — determines if a request targets this provider
- `ParseRequest(body, headers)` — extracts provider-agnostic RequestMetadata
- `ParseResponse(body, headers, statusCode)` — extracts provider-agnostic ResponseMetadata
- `ReassembleStream(chunks)` — reconstructs a complete response from SSE stream chunks

### Metadata Types

- `RequestMetadata` — model, max tokens, message count, temperature, stream flag, system prompt
- `ResponseMetadata` — model, input/output token counts, stop reason
- `StreamChunk` — raw SSE chunk data and event type

### Registry

- Thread-safe via `sync.RWMutex` for concurrent access
- `Register()` replaces existing providers with the same name
- `Detect()` returns the first matching provider (registration order)
- `All()` returns a defensive copy to prevent external mutation

## Test Coverage

11 tests, all passing:

1. `TestNewRegistry` — empty registry on creation
2. `TestRegisterAndGet` — basic register/retrieve
3. `TestGetUnregistered` — nil for unknown names
4. `TestRegisterReplacesExisting` — duplicate name replacement
5. `TestDetectMatchingProvider` — host-based detection
6. `TestDetectNoMatch` — nil when no provider matches
7. `TestDetectMultipleProviders` — correct routing with two providers
8. `TestDetectReturnsFirstMatch` — registration order priority
9. `TestAllReturnsRegistrationOrder` — ordering preserved
10. `TestAllReturnsCopy` — defensive copy prevents mutation
11. `TestDetectEmptyRegistry` — nil on empty registry

## Build Verification

- `go build ./...` — passes
- `go test ./internal/provider/` — 11/11 PASS
- Pre-existing CLI test failure in `internal/cli` (unrelated to this work unit)
