# 2026-04-18 — Session: Bundle 4 (BFF Foundation) complete

## Topic

Continued v0.2.0 Phase 3 from the prior session. User asked to complete
the remainder of Bundle 4 (WU-047 server and WU-049 capabilities) and
then left the session to run autonomously. Both WUs landed race-clean.

Session also absorbed a pre-existing `go vet` regression (from WU-042)
that was surfaced when BFF tests pushed the full build into vet's
analyzer graph.

## Work Completed

### WU-045 follow-on — Store.Ping

Added `Ping(ctx context.Context) error` to the `storage.Store`
interface and its `SQLiteStore` implementation, delegating to
`db.PingContext`. Prerequisite for WU-047's health/ready handlers
per Bundle 4 design D4.6.

Commit `2d40469` (`WU-045: add Store.Ping for BFF health handler`).

### WU-047 — Server, listeners, health/ready handlers

Replaced the WU-048 `Server` stub with the full server per design D3:

- `NewServer(store, config)` initializes the dispatcher and registers
  the three connection-lifecycle handlers
  (`connection.ping` / `connection.health` / `connection.ready`).
  Application-method handlers (sessions, turns, capabilities, models,
  etc.) are registered by their owning WUs via `Server.Dispatcher()`.
- `Start` binds the unix-socket listener with stale-socket detection
  (dial-probe; if dial succeeds another server is active → reject; if
  it fails → remove and rebind) and configurable `SocketMode` (default
  `0o600`), and the TLS listener (TLS 1.2+). Partial-start failures
  roll back listeners.
- `acceptLoop` → `handleConnection` enforces `MaxConnections` and
  wraps each `net.Conn` in a `Connection` with `requiresAuth` set to
  `true` for the TLS listener (so the state machine visits
  `ConnAuthenticating`).
- `Shutdown(ctx)` cancels the server context, closes listeners, closes
  active transports to wake read loops, removes the unix socket file,
  and waits for all goroutines to drain (respecting the context
  deadline).
- `TLSAddr()` exposes the bound TLS address for ephemeral-port tests.
- `handleConnectionHealth` populates a full `HealthResponse`: probes
  storage via `Store.Ping`, derives the capabilities dependency status
  from connection state as a stub until WU-049 lands, stubs
  auth/providers/routing to "ready" until their owning WUs.
- `handleConnectionReady` flips `Ready` only when the connection state
  is `ConnReady` AND `Store.Ping` succeeds (per FEAT-0008 line 445).
- `ServerVersion` is a package var (`dev` default, override-able via
  `-ldflags "-X .../bff.ServerVersion=..."`).

Tests cover socket bind + connect, permissions, stale-socket removal,
active-socket reject, graceful shutdown drain, max-connections gate,
concurrent accept, TLS bind + client handshake, health
populated/unavailable/active-session, ready true/false matrix, and
core-handler registration.

Commit `30fe4bf` (`WU-047: BFF server listeners, accept loop, health/ready handlers`).

### WU-049 — Capabilities manager + register/update handlers

Implemented the capabilities layer per design D5:

- `CapabilityManager` (per-connection) with thread-safe
  `Tools()` / `ProjectContext()` / `NegotiatedVersion()` snapshots.
  `Tools()` returns a defensive copy so callers can mutate without
  affecting manager state.
- `handleCapabilitiesRegister`:
  - Replay prevention: handler only runs in `ConnRegistering`;
    subsequent attempts return `CodeNotReady` (complements the
    dispatch gate which lets `capabilities.register` through in any
    Ready/Registering state).
  - Version negotiation: only protocol version `"1"` accepted;
    mismatch returns `CodeVersionMismatch` AND transitions the
    connection to `ConnFailed`.
  - Tool catalog partitioned into Registered / Rejected (partial
    rejection is a successful response; design choice codified from
    WU-041's `RejectedTool` type).
  - Tool validation: name, description, `input_schema` valid JSON,
    `risk_level` ∈ {read_only, write, execute, destructive},
    `output_envelope` ∈ {text, json, binary, image}.
  - Response carries `ServerCapabilities` with `protocol_version`,
    `max_frame_size` (`protocol.MaxFrameSize`), and
    `max_attachment_size` (from `ServerConfig`, 5 MiB default).
- `handleCapabilitiesUpdate`: atomic. If any added tool fails
  validation, the full update is rejected with `CodeCapabilityError`
  and the existing catalog is untouched.
- `CapabilityManager.RequestReregistration` sends the
  `capabilities.request` notification with a reason string (used by
  WU-050's reconnection flow and WU-063 diagnostics).
- Added `capabilities *CapabilityManager` field on `Connection`
  (initialized in `NewConnection`) with a `Capabilities()` accessor.
- `ServerConfig.MaxAttachmentSize` with 5 MiB default.

Tests cover register success (with full `ServerCapabilities` shape),
version mismatch + `ConnFailed` transition, partial rejection for each
validation class (bad risk, bad envelope, empty name, empty
description), replay prevention, update add/remove, atomic reject on
bad update, project-context refresh, `Tools()` defensive-copy
semantics, and `RequestReregistration` wire format.

Commit `01e8169` (`WU-049: capability registration, version negotiation, project context`).

### PATCH — statusMockProvider implements new Provider methods

`go vet` flagged `internal/cli/status_test.go:185` because
`statusMockProvider` didn't implement `FormatMessages` and
`FormatToolDefinitions`, which were added to the `provider.Provider`
interface in WU-042. Added no-op implementations and imported the
`protocol` package. `go test ./...` previously passed because the file
built; `go vet` surfaced it under stricter analysis.

Commit `311c33c` (`PATCH: statusMockProvider implements FormatMessages/FormatToolDefinitions`).

## Files Created or Modified

Created:
- `internal/bff/server_test.go` (WU-047)
- `internal/bff/capabilities.go` (WU-049)
- `internal/bff/capabilities_test.go` (WU-049)
- `docs/history/2026-04-18-session-bff-foundation-bundle-4-complete.md`
  (this log)

Modified:
- `internal/storage/store.go` — added `Ping` to interface
- `internal/storage/sqlite.go` — added `Ping` method
- `internal/bff/server.go` — replaced WU-048 stub with full server
- `internal/bff/connection.go` — added capabilities field + accessor
- `internal/bff/connection_test.go` — `recordingStore.Ping` + helper
- `internal/cli/status_test.go` — extended statusMockProvider
- `docs/releases/v0.2.0/status.md`

## Bundle 4 Progress

- ✅ WU-046 JSON-RPC transport — `9c39877`
- ✅ WU-048 Connection lifecycle state machine — `3242ce8`
- ✅ WU-047 Server listeners + accept loop + health/ready — `30fe4bf`
- ✅ WU-049 Capabilities manager + register/update — `01e8169`

## Notes / Decisions

- `NewConnection` signature extended (from prior session) with
  `requiresAuth bool` so TLS vs unix-socket paths diverge on
  `ConnAuthenticating`. The Server picks the flag at accept time based
  on which listener delivered the conn.
- `Server.Dispatcher()` is the downstream-WU hook for handler
  registration; the design's single monolithic `registerHandlers`
  function (D3.4) is split naturally across WUs — WU-047 registers
  ping/health/ready, WU-049 registers capabilities.*, and future WUs
  register their own methods. This avoids needing a `Dispatcher.Replace`
  method while preserving the design's end-state handler map.
- `handleCapabilitiesRegister` enforces replay prevention in the
  handler itself (in addition to the dispatch gate) because the
  dispatch gate lets `capabilities.register` through in `ConnReady`
  (which is the state the handler leaves you in on success). Without
  the handler check, a malicious or buggy harness could re-register
  and overwrite its own catalog mid-session.
- `handleConnectionHealth`'s capabilities dependency status is derived
  from connection state (`Ready`/`Degraded` → ready) rather than
  querying `CapabilityManager` directly. This gives the right answer
  today and remains correct after WU-050 (session resume) which may
  reset the capabilities manager without clearing state.
- Integration-test flake under `-race` (`TestMetricsAggregation` with
  SQLITE_BUSY) is pre-existing; not blocking Bundle 4.

## Next / Open Items

Track A continues with:
- **WU-050** — Session management (create, resume, list, details, lock).
  Integrates with the grace-period release wired in WU-048.
- **WU-051/052** — Conversation state and turn dispatch.
- **WU-053–056** — Streaming relay, system-prompt engine, cost tracking.
- **WU-057–060** — Three-layer model config and routing.

Integration with `modeltap serve` (design D3.5) is explicitly deferred
from WU-047 because the CLI wiring depends on the Viper config surface
added by WU-057+. Server is currently callable as a library.

Track B scaffold (WU-068–072) remains parallelizable — could be picked
up by a worktree-isolated agent.
