---
patch: "PATCH-0014"
title: "Fix BFF Server `sync.WaitGroup` race between accept and Shutdown"
status: "approved"
date: "2026-04-22"
related:
  - "FEAT-0008 (BFF server)"
branch: "exploration/integrated-harness"
---

# PATCH-0014: Fix BFF Server `sync.WaitGroup` race between accept and Shutdown

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

`go test -race ./...` fails deterministically in `internal/bff`:

```
WARNING: DATA RACE
Read at ... by goroutine 311:
  internal/bff.(*Server).handleConnection
      internal/bff/server.go:397 +0x410        // s.wg.Add(1)
Previous write at ... by goroutine 312:
  internal/bff.(*Server).Shutdown.func1
      internal/bff/server.go:260 +0x38         // s.wg.Wait() in a goroutine
--- FAIL: TestServer_StaleSocketRemoval (0.00s)
    testing.go:1712: race detected during execution of test
```

The race is a standard `sync.WaitGroup` misuse:

1. `acceptLoop` goroutine blocks in `ln.Accept()`. It holds one wg reference (from `Start`).
2. A client dials the socket. `Accept` returns a `net.Conn`. `acceptLoop` calls `go s.handleConnection(conn, ...)` and loops back to `Accept`.
3. The newly-spawned `handleConnection` goroutine has **not yet run** — no wg reference has been added on its behalf.
4. The test calls `srv.Shutdown(ctx)`. Shutdown closes the listener, then spawns a goroutine that calls `s.wg.Wait()`.
5. Whichever runs first is a coin flip: if `Wait()` observes the accept-loop's Done (triggered by `net.ErrClosed`) before `handleConnection` has a chance to call `s.wg.Add(1)`, the counter hits zero and Wait returns — then Add(1) fires with counter at 0, concurrent with a Wait. That is the data race Go's runtime flags.

Per `sync.WaitGroup` documentation: "calls with a positive delta that occur when the counter is zero must happen before a Wait." `Add(1)` must be issued from a goroutine whose wg reference is already accounted for.

## Scope

1. **Move `s.wg.Add(1)` from `handleConnection` into `acceptLoop`** — issue the Add in the same goroutine that already holds a wg reference (the accept loop itself), *before* spawning the handler goroutine. This guarantees the counter cannot reach zero during the Add.
2. **Hoist the matching `Done` in `handleConnection`** to a top-of-function `defer` so every return path (including the `MaxConnections` early-reject) balances the Add.
3. **Add a short comment at both sites** explaining the ownership convention so the next maintainer doesn't reintroduce the race by inlining the Add.

## Out of Scope

- No change to `Shutdown` semantics, connection lifecycle, or the accept-loop error handling.
- No change to the `MaxConnections` policy — rejected connections still close without a protocol reply; they just also balance the wg.
- No new test. The existing `TestServer_StaleSocketRemoval` already exercises the race when run with `-race` and passes repeatedly (`-count=3`) after the fix.

## Checklist

- [x] `s.wg.Add(1)` moved to `acceptLoop` before `go s.handleConnection(...)`
- [x] `defer s.wg.Done()` placed at the top of `handleConnection`
- [x] Inline comments explain the ownership convention
- [x] `go test -race -count=3 -run TestServer ./internal/bff/` passes
- [x] `make test` passes
- [x] `docs/patches/README.md` index updated
- [x] `docs/releases/v0.2.0/changelog.md` entry added

## Fix Detail

### The change

Before (`internal/bff/server.go`):

```go
// acceptLoop
go s.handleConnection(conn, requiresAuth)

// handleConnection
s.mu.Lock()
if s.config.MaxConnections > 0 && len(s.conns) >= s.config.MaxConnections {
    s.mu.Unlock()
    _ = netConn.Close()
    return
}
...
s.conns[c] = struct{}{}
s.wg.Add(1)     // <-- race: runs in a goroutine with no wg reference yet
s.mu.Unlock()

defer func() {
    s.wg.Done()
    s.mu.Lock()
    delete(s.conns, c)
    s.mu.Unlock()
}()
```

After:

```go
// acceptLoop
s.wg.Add(1)     // caller already holds a wg reference; safe to Add
go s.handleConnection(conn, requiresAuth)

// handleConnection
defer s.wg.Done()   // balances the Add in acceptLoop on every return path

s.mu.Lock()
if s.config.MaxConnections > 0 && len(s.conns) >= s.config.MaxConnections {
    s.mu.Unlock()
    _ = netConn.Close()
    return
}
...
s.conns[c] = struct{}{}
s.mu.Unlock()

defer func() {
    s.mu.Lock()
    delete(s.conns, c)
    s.mu.Unlock()
}()
```

### Why this is correct

`sync.WaitGroup` is safe when `Add(n>0)` is called from a goroutine whose own Add has already been issued and not yet Done'd — the counter is known `> 0` at the moment of the Add, so no concurrent `Wait()` can observe a zero. The accept loop itself has `s.wg.Add(1)` from `Start`, so calling `Add(1)` again from inside the accept loop is safe. Spawning a fresh goroutine and having *that* goroutine call `Add(1)` is not safe — the spawner returns to its caller (potentially decrementing its own wg reference) before the spawned goroutine runs.

### Why no new test

The existing `TestServer_StaleSocketRemoval` already triggers the race path (it dials the socket, then Shutdown closes the listener while the accept loop is still mid-spawn). Running it under `-race` is the regression check. Adding a dedicated test would duplicate the coverage without exercising anything new.
