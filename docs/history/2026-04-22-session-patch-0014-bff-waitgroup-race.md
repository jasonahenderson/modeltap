# Session Log — PATCH-0014 BFF `sync.WaitGroup` race fix

**Date:** 2026-04-22
**Branch:** exploration/integrated-harness
**Context:** Short build-unblock session. `make` was failing with a `-race` data race in `TestServer_StaleSocketRemoval`. Root cause diagnosed and fixed under PATCH-0014.

## What was discussed / decided

User ran `make` and pasted the failing output. `go test -race ./...` reported a deterministic data race in `internal/bff`:

```
WARNING: DATA RACE
Read at ... by goroutine 311:
  internal/bff.(*Server).handleConnection
      internal/bff/server.go:397    // s.wg.Add(1)
Previous write at ... by goroutine 312:
  internal/bff.(*Server).Shutdown.func1
      internal/bff/server.go:260    // s.wg.Wait() in a goroutine
--- FAIL: TestServer_StaleSocketRemoval (0.00s)
    testing.go:1712: race detected during execution of test
```

### Diagnosis

Classic `sync.WaitGroup` misuse:

1. `acceptLoop` (`server.go:362`) held a wg reference from `Start` (`server.go:210/225`) and blocked in `ln.Accept()`.
2. On accept, it did `go s.handleConnection(conn, ...)` — no Add issued before the spawn.
3. The fresh `handleConnection` goroutine internally called `s.wg.Add(1)` at line 397. Between the `go` and that `Add(1)`, the handler goroutine holds no wg reference.
4. `TestServer_StaleSocketRemoval` cleanup called `srv.Shutdown(ctx)` which closes the listener, then spawns a goroutine that calls `s.wg.Wait()` (line 260).
5. If `Accept` returns `net.ErrClosed` and the accept loop's Done runs before `handleConnection` hits its `Add(1)`, the counter reaches zero, `Wait()` returns, and the subsequent `Add(1)` races with the Wait. Go's race detector correctly flags this as a data race even when the arithmetic happens to resolve cleanly.

Per `sync.WaitGroup` docs: "calls with a positive delta that occur when the counter is zero must happen before a Wait." `Add(1)` must be issued from a goroutine whose wg reference is already on the books.

### Fix

Moved `s.wg.Add(1)` from `handleConnection` into `acceptLoop`, immediately before `go s.handleConnection(...)`. The accept loop already holds a wg reference (from `Start`), so its counter is known > 0 at the moment of the Add — that Add can never race with a concurrent `Wait`. Hoisted a top-of-function `defer s.wg.Done()` in `handleConnection` so the `MaxConnections` early-reject path also balances the Add.

Added inline comments at both sites explaining the ownership convention so a future maintainer doesn't inline the Add back into the handler.

### Verification

- `go test -race -count=3 -run TestServer ./internal/bff/` — 3/3 pass.
- `make test` — all packages pass under `-race`.

## Actions taken

- Fixed `internal/bff/server.go` (moved `wg.Add(1)` into `acceptLoop`, hoisted `defer wg.Done()` in `handleConnection`, added comments).
- Drafted `docs/patches/0014-bff-shutdown-waitgroup-race.md` with full problem/scope/fix detail.
- Added PATCH-0014 to `docs/patches/README.md` index (status: approved).
- Added PATCH-0014 entry to `docs/releases/v0.2.0/changelog.md` Patches table.
- Status on the patch doc flipped `proposed` → `approved` after user confirmation.

## Files created or modified

Modified:
- `internal/bff/server.go` — moved `wg.Add(1)` into `acceptLoop`; `defer wg.Done()` at top of `handleConnection`; inline comments.
- `docs/patches/README.md` — added PATCH-0014 row.
- `docs/releases/v0.2.0/changelog.md` — added PATCH-0014 row with "Shipped" framing.

Created:
- `docs/patches/0014-bff-shutdown-waitgroup-race.md` — patch doc.
- `docs/history/2026-04-22-session-patch-0014-bff-waitgroup-race.md` — this log.

## What's next / open items

- PATCH-0014 status stays `approved` until the commit merges to `main`; flip to `done` at that point (or record a PR link if one is created).
- No follow-on work required. No other `-race` failures remain under `make test`.
