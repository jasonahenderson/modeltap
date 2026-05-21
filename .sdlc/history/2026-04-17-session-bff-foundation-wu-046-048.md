# 2026-04-17 — Session: BFF Foundation Phase 3 (WU-046 + WU-048)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Topic

Continued v0.2.0 Phase 3 (Implementation). Track 0 was complete entering
the session (commit `6b4501c` "Track 0 complete — all 9 WUs implemented").
Began Track A foundation (Bundle 4: WU-046–049) on branch
`exploration/integrated-harness`.

User chose Track A (server-first per the plan's recommended serialization)
over the parallel Track B scaffold.

## Work Completed

### Protocol amendment (Bundle 4 prerequisite)

Added the `MT-CONN-013` "attachment too large" diagnostic code that the
Bundle 4 design (D5.6) requires for `WU-049`. `MaxFrameSize` and
`MaxAttachmentSize` were already on `ServerCapabilities` from earlier
work, so only the diagnostic was missing.

- `internal/protocol/errors.go` — `DiagAttachmentTooLarge`
- `internal/protocol/messages041_test.go` — pin to "MT-CONN-013"
- `internal/protocol/conformance_test.go` — round-trip + coverage entries
- `internal/protocol/fixtures/errors/mt_conn_013.json` — golden fixture

Commit `e88a6d4` (`WU-041: add MT-CONN-013 attachment-too-large diagnostic code`).

### WU-046 — JSON-RPC transport layer

Created `internal/bff/` and the WU-046 transport surface per design D2:

- `FrameTransport` — NDJSON read/write over `net.Conn` with envelope
  classification (Request / Notification / Response). Concurrent writes
  serialized via mutex; oversize frames return `TransportError{Close:true}`.
- `Dispatcher` — method-name routing with `MethodNotFound` errors and
  duplicate-registration panic.
- JSON-RPC standard codes plus FEAT-0008 application codes
  (`-32000..-32007`, design D2.4).
- `ValidateTurnSubmit` — edge validation (sequence presence via map
  decode; mode enum value).
- Minimal `Connection` stub so the `Handler` signature compiles.

Tests cover envelope classification, oversize-frame teardown, send
error/notification/response, concurrent writes, dispatcher
register/dispatch/duplicate-panic, and turn.submit validation. All pass
under `go test -race`.

Commit `9c39877` (`WU-046: JSON-RPC transport layer for BFF`).

### WU-048 — Connection lifecycle state machine

Replaced the WU-046 `Connection` stub with the full state machine per
design D4, plus a stub `Server` struct in a new `server.go`:

- `ConnState` (9 states) with canonical FEAT-0008 `String()` names pinned
  by `TestConnState_StringValues` (wire-visible via HealthResponse,
  Diagnostic, session.sync).
- `validTransitions` map and `Connection.transition()` enforcing the
  five legal post-Connecting transition sets.
- `Connection.Run()` read loop with dispatch gating (`CodeNotReady` for
  non-ready states; `capabilities.register` allowed in `ConnRegistering`,
  `connection.ping` allowed post-Connecting).
- Heartbeat monitor — passive watcher of harness-initiated pings; fails
  the connection when `lastPing` is older than `HeartbeatTimeout`.
  `lastPing` initialized to creation time so a freshly-accepted
  connection isn't flagged before its first ping (A-01 fix).
- Grace-period release (`scheduleGracePeriodRelease` /
  `cancelGracePeriodRelease`) — on terminal failure with a bound
  session, schedules `ReleaseSessionLock` after `GracePeriod`;
  cancellable so a reconnecting harness can rescue the lock.
- `handleConnectionPing` — updates `lastPing`, returns
  `*protocol.ConnectionPong`.
- `ServerConfig` with FEAT-0008 default timing (15s / 30s / 10s = 40s
  total budget, pinned by `TestConnection_GracePeriod_TimingMath`).
- `requiresAuth` flag on `NewConnection` so unix-socket connections
  skip `ConnAuthenticating` (design D4.1) — small extension to the
  D4.2 constructor signature.

Subtle fix during green phase: when an oversize frame arrives, the read
loop must NOT attempt `SendError` before closing — the wire is mid-frame
and the peer may not be reading. The defer chain calls `transport.Close()`
unconditionally on Run exit.

Tests cover state transitions (valid + invalid), socket-vs-TLS init paths,
dispatch gating, heartbeat timeout + initial grace, grace-period
expire/cancel/no-session, oversize-frame teardown, clean EOF teardown,
and end-to-end ping dispatch. All pass under `go test -race`.

Commit `3242ce8` (`WU-048: connection lifecycle state machine`).

### Status updates

- `e88a6d4` then `ada0459` (status after WU-046) then `eca7e7b` (status
  after WU-048).

## Files Created or Modified

Created:
- `internal/bff/transport.go`
- `internal/bff/transport_test.go`
- `internal/bff/connection.go` (replaced WU-046 stub)
- `internal/bff/connection_test.go`
- `internal/bff/server.go`
- `internal/protocol/fixtures/errors/mt_conn_013.json`
- `.sdlc/history/2026-04-17-session-bff-foundation-wu-046-048.md` (this log)

Modified:
- `internal/protocol/errors.go`
- `internal/protocol/messages041_test.go`
- `internal/protocol/conformance_test.go`
- `.sdlc/releases/v0.2.0/status.md`

## Bundle 4 Progress

- ✅ WU-046 transport — `9c39877`
- ✅ WU-048 connection state machine — `3242ce8`
- ⏳ WU-047 server (listeners, accept loop, handler registration,
  `handleConnectionHealth` / `handleConnectionReady`)
- ⏳ WU-049 capabilities (registration, version negotiation, project
  context)

## Notes / Decisions

- `NewConnection` signature extended with `requiresAuth bool` vs the
  exact design (D4.2) `func NewConnection(id, transport, server)`. This
  is the cleanest way to encode unix-vs-TLS skip-auth without smuggling
  TLS knowledge into `FrameTransport`.
- Test infra uses an embedded-`storage.Store` `recordingStore` that
  overrides only `ReleaseSessionLock` — methods not exercised by WU-048
  will nil-pointer-deref if called, which is acceptable for a focused
  test fake.
- Health/ready handlers (`handleConnectionHealth` / `handleConnectionReady`)
  are deferred to WU-047 because they need `Server.startTime`, a
  `store.Ping()` method (not yet on the `Store` interface), and the
  capability-registered flag from WU-049.

## Next / Open Items

When resuming WU-047:

1. Add `Ping(ctx context.Context) error` to `storage.Store` interface
   and the SQLite implementation as a small WU-045 follow-on commit
   (mirror of the MT-CONN-013 pre-step pattern).
2. Implement `Server.Start` / `Shutdown`, unix-socket listener with
   stale-socket detection and configurable `SocketMode`, TLS listener,
   accept loop, max-connections gate.
3. Wire `handleConnectionHealth` and `handleConnectionReady` per design
   D4.6 (with appropriate stubs for the auth/providers/routing
   dependencies).
4. `registerHandlers` for the 22 protocol methods (most as stubs that
   return `CodeNotReady`-equivalent errors until their owning WUs land).
5. Then proceed to WU-049 (capabilities).

Track B scaffold (WU-068–072) remains parallelizable — could be picked
up by a worktree-isolated agent in parallel with the rest of Track A.
