# BFF Foundation Bundle (WU-046 + 047 + 048 + 049) Pre-Review Lint — Claude Subagent

**Reviewer:** Claude subagent (fresh context, same-model pre-review — not Tier C peer review)
**Date:** 2026-04-16
**Subject:** `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md`
**Bundle:** WU-046 (JSON-RPC transport) + WU-047 (protocol endpoint) + WU-048 (connection lifecycle) + WU-049 (capability registration)

## Reviewer caveat

Same-model lint: shares the Designer's training distribution, tokenizer, and reasoning heuristics. Most likely to catch mechanical drift from FEAT-0008 spec, field-name mismatches with the protocol package, scope gaps against track-a WU descriptions, and cross-bundle inconsistency with the protocol-types and storage designs. Least likely to catch Claude-characteristic reasoning blind spots (e.g., uncritical acceptance of plausible symmetry between heartbeat directions). Does not substitute for a cross-model Tier-C peer review.

## Summary

The bundled design is well-structured and covers the four WUs comprehensively. The package layout, goroutine model, state machine, and test strategy are sound. Five blocking findings: (B-01) the heartbeat direction is inverted relative to FEAT-0008 — the spec says "harness sends `connection.ping`" and "server replies with `connection.pong`", but the design has the server sending pings and the harness replying; (B-02) the `CapabilitiesRegisterResponse` defined locally in D5.2 conflicts with the protocol-types bundle's `CapabilitiesRegisterResponse` (different fields, different location); (B-03) `ConnectionPong` is used with a `Time` field that does not exist in the protocol-types design (which declares `ConnectionPong` with no fields); (B-04) the `capabilities.request` server-to-harness message is not wired or mentioned anywhere in the design despite WU-049 track spec requiring re-request via `capabilities.request`; (B-05) the design defines `ConnState` constants locally but the protocol-types bundle already defines connection state names — the design must clarify whether these are the same type or a server-internal shadow.

Six attention items and three nits round out the findings.

## Blocking findings

### B-01. Heartbeat direction is inverted relative to FEAT-0008

- **What:** FEAT-0008 section "Heartbeat" (lines 416-420) unambiguously states: "Harness sends `connection.ping` every 15 seconds (configurable). Server replies with `connection.pong` within 5 seconds." The design (D4.5) inverts this: "The server sends `connection.ping` notifications at `HeartbeatInterval` (15s). The harness responds with `connection.ping` requests (pong)." The code in `startHeartbeat` shows the server sending pings and tracking `lastPong`. The protocol-types design confirms `connection.pong` is a non-streaming server-to-harness message — consistent with the spec (server sends the pong reply), not with this design (server initiating pings).
- **Evidence:**
  - `.sdlc/features/0008-bff-server.md:416-417` — "Harness sends `connection.ping` every 15 seconds"
  - `.sdlc/features/0008-bff-server.md:418` — "Server replies with `connection.pong` within 5 seconds"
  - `.sdlc/features/0008-bff-server.md:420` — "Server-side: after heartbeat timeout (missed pings for 30 seconds)..." — the server is missing *pings from the harness*, not *pongs from the harness*
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:390-410` — D4.5: "server sends `connection.ping` notifications... The harness responds with `connection.ping` requests (pong)."
  - Protocol types design line 269: `ConnectionPong | connection.pong | No fields` — server-to-harness non-streaming message
  - `internal/protocol/messages.go:37` — `MethodConnectionPing = "connection.ping"` is a harness-to-server request
- **Why blocking:** If implemented as designed, the server would be initiating heartbeat pings but the protocol defines `connection.ping` as a harness-to-server method. The server's `startHeartbeat()` goroutine sending `connection.ping` notifications contradicts both the spec and the protocol message catalog. The `handleConnectionPing` handler is correct (it's handling the harness's ping and replying), but the server-initiated ping loop is wrong. The server should passively track receipt of harness pings and transition to `ConnFailed` when they stop arriving — it should not initiate pings.
- **Suggested fix:** Remove the server-side `connection.ping` send loop from D4.5. Replace `startHeartbeat` with a heartbeat *monitor* that checks `lastPing` (when the last harness `connection.ping` was received). The `handleConnectionPing` handler records `lastPing = time.Now()` and returns `ConnectionPong{}`. The monitor goroutine checks `time.Since(lastPing) > HeartbeatTimeout` and transitions to `ConnFailed`. The `HeartbeatInterval` config becomes irrelevant server-side (it's a harness setting) — remove it from `ServerConfig` or document it as informational only.

### B-02. `CapabilitiesRegisterResponse` conflicts with the protocol-types bundle

- **What:** The design defines `CapabilitiesRegisterResponse` in `internal/bff/capabilities.go` (D5.2) with fields `{negotiated_version, server_version, max_frame_size, max_attachment_size}`. The protocol-types bundle (`.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md` line 279) defines `CapabilitiesRegisterResponse` in `internal/protocol/tools.go` with fields `{registered []ToolDefinition, server_capabilities ServerCapabilities, rejected []RejectedTool}`. These are two different types with the same name in two different packages, carrying entirely different fields and semantics. The design explicitly says "This type lives in `internal/bff/capabilities.go` (not in `internal/protocol/`) because it is server-specific", but the protocol-types bundle already defines a wire type with this name.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:499-507` — BFF `CapabilitiesRegisterResponse` with 4 fields
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:279` — Protocol `CapabilitiesRegisterResponse` with `registered`, `server_capabilities`, `rejected`
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:322` — `ServerCapabilities` carries `protocol_version` and `protocol_version_range`
- **Why blocking:** The implementer of WU-049 must decide which shape is the actual `Response.Result` for `capabilities.register`. The protocol-types bundle is the canonical wire-shape definition (WU-041's scope). The BFF design should use `protocol.CapabilitiesRegisterResponse`, not define a competing type. The missing fields from the BFF version (`registered`, `rejected`, `server_capabilities`) are important — the protocol design allows partial rejection of tools. The missing fields from the protocol version (`max_frame_size`, `max_attachment_size`) were added by the WU-039 review finding A-05 but never propagated to the protocol-types design.
- **Suggested fix:** (a) Remove the local `CapabilitiesRegisterResponse` from `internal/bff/` and use `protocol.CapabilitiesRegisterResponse` instead. (b) Amend the protocol-types design to add `max_frame_size` and `max_attachment_size` fields to `ServerCapabilities` (or to `CapabilitiesRegisterResponse` directly), since review finding A-05 ratified these limits. (c) The BFF handler populates the protocol type's fields (`registered`, `server_capabilities`, `rejected`) from the `CapabilityManager` state.

### B-03. `ConnectionPong` used with a `Time` field that does not exist

- **What:** The design's `handleConnectionPing` handler (D4.5, line 409) returns `&protocol.ConnectionPong{Time: time.Now().UTC()}`. The protocol-types design declares `ConnectionPong` with **no fields** (line 269: `ConnectionPong | connection.pong | No fields`). There is no `Time` field on `ConnectionPong`.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:409` — `return &protocol.ConnectionPong{Time: time.Now().UTC()}, nil`
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:269` — `ConnectionPong | connection.pong | No fields`
  - FEAT-0008 line 209 — "Reply to harness `connection.ping` heartbeat" — no payload specified
- **Why blocking:** This will not compile. If a `Time` field is desired, it must be added to the protocol-types design first and covered by a fixture.
- **Suggested fix:** Either (a) return `&protocol.ConnectionPong{}` (empty pong, matching the protocol design and FEAT-0008), or (b) amend the protocol-types design to add `Time time.Time` to `ConnectionPong` and add a fixture. Option (a) is simpler and spec-aligned.

### B-04. `capabilities.request` (server-to-harness) is not wired

- **What:** The WU-049 track spec (track-a-bff-server.md lines 47-58) says WU-049 handles `capabilities.register`, `capabilities.update`, **and** `capabilities.request`. FEAT-0008 line 207 defines `capabilities.request` as a server-to-harness message: "Ask harness to re-register capabilities (triggered on reconnect or when server detects tool schema drift)." The design covers `capabilities.register` (D5.2) and `capabilities.update` (D5.3) but never mentions `capabilities.request`. There is no mechanism in the design for the server to proactively ask the harness to re-register.
- **Evidence:**
  - `.sdlc/releases/v0.2.0/track-a-bff-server.md:52` — "handles `capabilities.register`, `capabilities.update`, `capabilities.request`"
  - `.sdlc/features/0008-bff-server.md:207` — `capabilities.request` is server-to-harness
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:449-553` — D5 covers register and update only
  - Protocol-types design line 268: `CapabilitiesRequestEvent | capabilities.request | reason (O, omitempty)`
- **Why blocking:** Without this, the server has no mechanism to trigger tool re-registration after reconnection or schema drift. Downstream WUs (WU-050 session resume, WU-063 diagnostics) expect this capability to exist.
- **Suggested fix:** Add a D5.5 or D5.6 section for `capabilities.request`. The `CapabilityManager` or `Connection` should expose a `RequestReregistration(reason string) error` method that sends a `CapabilitiesRequestEvent` notification to the harness via the transport. Wire it into the reconnection flow (when a connection in `ConnRegistering` detects the harness previously had a session active, or on `session.resume` when tool schema has changed). Test: `TestCapabilities_Request_OnReconnect`.

### B-05. `ConnState` enum redeclares connection states without referencing the protocol-types canonical names

- **What:** The design defines `ConnState` as a new `int` iota enum in `internal/bff/connection.go` with 9 states. FEAT-0008 defines these 9 states as string names (discovering, starting, connecting, etc.) in the "Connection states" section (lines 379-395). The `String()` method on `ConnState` should match these canonical names exactly. However, neither the design nor the protocol-types bundle defines an authoritative string enum for connection states in Go. The concern is that the BFF design defines these states as server-internal iota constants, but the health/diagnostic reporting layer (WU-041 `HealthResponse`, WU-063 diagnostics) may need to report the connection state as a wire-visible string. The design should clarify whether `ConnState.String()` is for logging only, or if it becomes a wire-visible field.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:272-309` — `ConnState` iota with `String()` method
  - `.sdlc/features/0008-bff-server.md:379-395` — canonical state names
  - FEAT-0008 line 437 — `connection.health` response includes `active_session` but no explicit connection state field
- **Why blocking:** If `ConnState.String()` output ever reaches the wire (e.g., in health responses, diagnostics, or session sync), the string values must match FEAT-0008 exactly. The design says `String()` returns "discovering", "starting", etc. but does not assert these match the spec verbatim. Given that downstream WUs (WU-063, WU-065) reference `Connection.transition()` and `Connection.State()`, this type's string representation will inevitably reach the wire. The design must either (a) declare this is internal-only and define a separate wire representation, or (b) assert the `String()` values are canonical and add a test that pins them to the FEAT-0008 names.
- **Suggested fix:** Add to the test table: `TestConnState_StringValues` — asserts each `ConnState.String()` matches the FEAT-0008 canonical name (e.g., `ConnDiscovering.String() == "discovering"`). Add a brief note in D4.1 stating whether the `String()` output is wire-visible or logging-only. If wire-visible, consider defining a `ConnectionState` typed string in `internal/protocol/` for consistency with the rest of the protocol surface.

## Attention findings

### A-01. `connection.ping` allowed "in any state after `ConnConnecting`" — but `ConnAuthenticating` is before `ConnRegistering`

- **What:** D4.4 dispatch gating rule says `connection.ping` is allowed "in any state after `ConnConnecting`". The state machine has `ConnConnecting -> ConnAuthenticating -> ConnRegistering -> ConnReady`. Allowing ping in `ConnAuthenticating` means the harness can start heartbeating before completing auth or registration. This is reasonable for liveness checking, but FEAT-0008 line 159 says "The connection is not considered established until both auth and capability registration complete." If the harness starts pinging before registration completes, the server must track `lastPong` (or `lastPing` per B-01 fix) from the moment of first ping receipt, not from the `ConnReady` transition. The design does not specify when `lastPong`/`lastPing` is initialized.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:383` — "connection.ping (in any state after ConnConnecting)"
  - `.sdlc/features/0008-bff-server.md:159` — connection not established until auth + registration complete
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:396-400` — heartbeat monitor checks `lastPong` but initialization not specified
- **Recommended disposition:** Clarify in D4.2 or D4.5 that `lastPing`/`lastPong` is initialized to `time.Now()` at connection creation (so the heartbeat timeout clock doesn't start until the connection has had a chance to complete registration). Add a test: `TestConnection_Heartbeat_InitialGrace` — verify that a connection in `ConnRegistering` that has not yet received a ping is not timed out within the heartbeat window.

### A-02. `ReadyResponse` in D4.6 returns `{Ready: conn.State() == ConnReady}` — but FEAT-0008 says ready requires more

- **What:** D4.6 says `handleConnectionReady` returns `ReadyResponse{Ready: conn.State() == ConnReady}`. FEAT-0008 line 445 says: "`connection.ready` returns `true` only if auth, storage, capability registration, and at least one routing target are healthy." The design's implementation only checks state, not dependency health. A connection could be in `ConnReady` with a dead storage layer.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:430-431` — `ReadyResponse{Ready: conn.State() == ConnReady}`
  - `.sdlc/features/0008-bff-server.md:445` — requires auth + storage + capabilities + routing healthy
- **Recommended disposition:** Update `handleConnectionReady` to also check `store.Ping()` and (when WU-057 lands) at least one provider's health. For v0.2.0 initial implementation, at minimum check storage readiness alongside state. Document this as a stub that will be extended when provider health checks (WU-057) land.

### A-03. `HealthResponse` in D4.6 stubs provider health but the field list diverges from FEAT-0008

- **What:** D4.6 lists the `HealthResponse` fields as: `server_version`, `protocol_version`, `uptime_seconds`, `providers`, `storage`, `active_session`. FEAT-0008 line 422-443 shows a richer structure: `auth`, `storage`, `capabilities`, `providers` (per-provider), `routing`, `active_session`. The design omits `auth`, `capabilities`, and `routing` dependency statuses. These are defined in the protocol-types bundle's `HealthResponse` type (protocol-types line 317: `auth DependencyStatus, storage DependencyStatus, capabilities DependencyStatus, providers map[string]ProviderStatus, routing DependencyStatus, active_session *ActiveSessionInfo`).
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:418-425` — partial field list
  - `.sdlc/history/2026-04-16-design-protocol-types-040-041-093.md:317` — full `HealthResponse` type
  - `.sdlc/features/0008-bff-server.md:422-443` — full example
- **Recommended disposition:** The handler should populate all fields of `protocol.HealthResponse`, with `auth`, `capabilities`, and `routing` set to stub `DependencyStatus{Status: "ready"}` values until their respective WUs land. The design should list the full field population plan, even if some fields are initially stubbed.

### A-04. `ValidateTurnSubmit` is in the transport layer (WU-046) but turn.submit dispatch is a WU-051/052 concern

- **What:** D2.5 places `ValidateTurnSubmit` (sequence presence check, mode validation) in `transport.go`. The transport layer's responsibility is framing and JSON-RPC envelope dispatch, not method-specific validation. Putting method-specific validation in the transport couples the transport to the message catalog and makes it grow linearly with new methods. A cleaner boundary would validate at the handler layer (the `Handler` function for `turn.submit`). The track spec (track-a-bff-server.md WU-046 lines 19-22) inherited this from the WU-039 review, so it's technically in-scope for WU-046 — but the architectural placement is questionable.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:139-155` — D2.5 `ValidateTurnSubmit` in transport.go
  - `.sdlc/releases/v0.2.0/track-a-bff-server.md:19-22` — inherited validation requirement
- **Recommended disposition:** Keep the validation in WU-046 scope (per track spec), but move the function to a separate `validate.go` file or to the dispatch gate in `connection.go` (D4.4). The transport should classify and dispatch frames; per-method validation should happen at or near the handler. This is a design-quality concern, not a correctness issue.

### A-05. No mention of `session.sync` method wiring in the dispatch gate

- **What:** D4.4 dispatch gating allows `capabilities.register` in `ConnRegistering` and `connection.ping` in any post-connecting state. But `session.sync` (FEAT-0008 lines 456-495) is used "after re-establishing the connection (auth + capabilities)" — i.e., when the harness reconnects and wants to synchronize state. The harness will send `session.sync` immediately after `capabilities.register` completes and the connection reaches `ConnReady`. This is handled by the standard `ConnReady` gate. But the design should explicitly mention `session.sync` in the expected method catalog for `ConnReady` state, since it has special timing requirements (it's the first method after registration on reconnect, before any `turn.submit`).
- **Evidence:**
  - `.sdlc/features/0008-bff-server.md:456-458` — "After re-establishing the connection (auth + capabilities), the harness sends `session.sync`"
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:369-385` — dispatch gate rules
- **Recommended disposition:** Add `session.sync` to the D4.4 commentary as an expected first-method-after-registration case. No code change needed (it's gated by `ConnReady` like other methods), but the design should acknowledge the reconnection flow.

### A-06. Grace period release calls `store.ReleaseSessionLock(sessionID, conn.id)` but the Store interface from WU-045 does not define this method

- **What:** D4.7 references `store.ReleaseSessionLock(sessionID, conn.id)` for releasing the session lock after the grace period expires. The storage design (WU-045) defines the `Store` interface extensions, but the exact method signature for lock release is not quoted in this design's "Dependencies Consumed" section. The storage design defines lock acquisition via SQL (`UPDATE sessions SET lock_owner = ?, lock_expires_at = ? WHERE id = ? AND (lock_owner IS NULL OR lock_expires_at < ?5)`) but the Go interface method name and signature must match.
- **Evidence:**
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:439` — `store.ReleaseSessionLock(sessionID, conn.id)`
  - `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md:649` — dependencies list: `Store` interface (session lock methods)
  - Storage design D3 (store interface extensions) — needs cross-reference for exact method signature
- **Recommended disposition:** Cross-reference the exact `Store` method signature from the storage design. If the storage design calls it `ReleaseSessionLock(ctx context.Context, sessionID string, ownerID string) error`, document that here. If it doesn't exist yet, flag it as a required interface addition for WU-045.

## Nit findings

### N-01. `ServerConfig.SocketPath` default comment says `~/.local/share/modeltap/server.sock` but FEAT-0008 examples use the same path

This is consistent; just noting for posterity. The XDG Base Directory spec uses `$XDG_RUNTIME_DIR` for runtime sockets, not `~/.local/share/`. For a production deployment this may warrant reconsideration, but it matches the current FEAT-0008 spec and is not a design error.

### N-02. Test table for WU-048 does not cover `ConnAuthenticating` state transitions for Unix socket connections

D4.1 says "For Unix socket connections, this state is skipped." The state transition table shows `ConnConnecting -> {ConnAuthenticating, ConnRegistering, ConnFailed}`. A test should cover the direct `ConnConnecting -> ConnRegistering` path for Unix sockets. The test table lists generic state transition tests but does not call out the skip path explicitly.

### N-03. `Dispatcher.Register` panics on duplicate — non-idiomatic for library code

D2.3 says `Register` "Panics on duplicate registration." Go library code typically returns an error rather than panicking. Since registration happens at server startup (not at request time), a panic is defensible but worth a brief justification comment in the code.

## Coverage table

| Track-A WU requirement | Design coverage | Notes |
|-------------------------|-----------------|-------|
| WU-046: NDJSON reader/writer over net.Conn | complete | D2.1 `FrameTransport` wraps `protocol.FrameReader`/`FrameWriter` |
| WU-046: message dispatch by method | complete | D2.3 `Dispatcher` |
| WU-046: request/response correlation | complete | D2.6 defers to Connection (correct) |
| WU-046: error response formatting | complete | D2.4 JSON-RPC error codes |
| WU-046: close on oversize frame (SR-039-01) | complete | D2.1, D2.5 |
| WU-046: turn.submit validation (A-01, A-02) | complete | D2.5 `ValidateTurnSubmit` |
| WU-047: Unix socket listener | complete | D3.2 `startSocketListener` |
| WU-047: TLS listener | complete | D3.2 `startTLSListener` |
| WU-047: graceful shutdown | complete | D3.1 `Shutdown` |
| WU-047: `modeltap serve` integration | complete | D3.4 |
| WU-047: stale socket detection | complete | D3.2 |
| WU-048: 9 states | complete | D4.1 all 9 listed |
| WU-048: transitions | complete | D4.3 transition map |
| WU-048: heartbeat handler | **drift** | B-01: direction inverted vs. FEAT-0008 |
| WU-048: health check handler | partial | A-02, A-03: ready/health field divergence |
| WU-048: timeout and grace-period | complete | D4.7 40s total = 30s + 10s |
| WU-049: tool catalog | complete | D5.1 `CapabilityManager` |
| WU-049: protocol version negotiation | complete | D5.2 version check |
| WU-049: project context capture | complete | D5.5 `UpdateProjectContext` |
| WU-049: MaxFrameSize/attachment-size (A-05) | complete | D5.6 |
| WU-049: `capabilities.request` re-request | **missing** | B-04: not covered |
| WU-049: `CapabilitiesRegisterResponse` | **conflict** | B-02: conflicts with protocol-types bundle |

## What I did NOT review

- **FEAT-0008 spec correctness itself.** Treated as source of truth.
- **WU-050 through WU-067 design correctness.** These are downstream consumers; only checked that the exported interfaces listed in the "Interfaces Exported" section are plausible.
- **Go idiom depth.** The `sync.Mutex` / `sync.RWMutex` usage patterns, goroutine lifecycle management, and context cancellation chains look reasonable but were not stress-tested for race conditions.
- **Storage bundle interface compatibility beyond method names.** Did not trace exact SQL or Go signatures.
- **TLS configuration correctness.** The `startTLSListener` is mentioned but TLS cert loading, minimum TLS version, and cipher suite selection are not specified in the design — presumably deferred to implementation.
- **Config integration details.** D3.4 mentions extending `internal/config/config.go` but does not specify the Viper key paths or default-binding details.
