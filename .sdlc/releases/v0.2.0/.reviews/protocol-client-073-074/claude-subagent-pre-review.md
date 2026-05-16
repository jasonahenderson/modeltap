# Pre-Review: Protocol Client Bundle (WU-073 + WU-074)

**Design doc:** `.sdlc/history/2026-04-16-design-protocol-client-073-074.md`
**Reviewed against:**
- `.sdlc/features/0009-terminal-harness.md` (FEAT-0009)
- `.sdlc/features/0008-bff-server.md` (FEAT-0008)
- `internal/protocol/protocol.go` and `internal/protocol/messages.go` (WU-039 types)
- `.sdlc/releases/v0.2.0/track-b-terminal-harness.md` (Track B WU descriptions)
- `.sdlc/history/2026-04-16-design-bff-foundation-046-047-048-049.md` (server-side design)

**Reviewer:** Claude subagent (automated pre-review)
**Date:** 2026-04-16

---

## Summary

| Severity | Count |
|----------|-------|
| Blocking | 2 |
| Attention | 5 |
| Nit | 3 |

---

## Blocking

### B-01. Heartbeat direction is inverted between client and server designs

- **What:** The client design (D3.5, lines 388-403) has the harness _sending_ `connection.ping` via `cm.client.Ping(ctx)` and tracking the server's response time as `cm.lastPong`. This matches the FEAT-0008 spec (line 211: "the heartbeat is harness-initiated: the harness sends `connection.ping`, the server replies with `connection.pong`"; line 416: "Harness sends `connection.ping` every 15 seconds"). However, the **server-side design** (BFF foundation D4.5, lines 390-410) also has the _server_ sending `connection.ping` notifications and tracking harness pongs. This means both sides think they are the ping initiator.
- **Spec reference:**
  - `.sdlc/features/0008-bff-server.md:211` -- "heartbeat is harness-initiated"
  - `.sdlc/features/0008-bff-server.md:416-417` -- "Harness sends `connection.ping` every 15 seconds. Server replies with `connection.pong` within 5 seconds."
  - Server design D4.5 (line 398): "The server sends `connection.ping` notifications at `HeartbeatInterval` (15s). The harness responds with `connection.ping` requests (pong). The server tracks `lastPong`."
- **Why blocking:** If both sides ping and neither responds, the connection will time out immediately. The client design correctly follows FEAT-0008 (harness initiates), but it must be aware that the server design contradicts the spec. During implementation, the server design must be corrected, or the client must handle unsolicited server pings. Either way, the two designs are incompatible as written and one side's heartbeat will fail.
- **Suggested fix:** The client design is spec-correct. Flag the server design (BFF foundation D4.5) for amendment: the server should _respond_ to `connection.ping` requests (already handled by `handleConnectionPing` in D4.5), not _initiate_ pings. Remove the server-side `startHeartbeat` sending logic and keep only the timeout monitor that checks `lastPong` (updated when the harness's `connection.ping` request arrives). This was already flagged in the BFF foundation review (B-02) but is repeated here because the client design silently depends on the fix.

### B-02. Heartbeat degradation threshold differs from FEAT-0008

- **What:** The client design (D3.5, lines 400-403) transitions directly from Ready to Reconnecting when `time.Since(cm.lastPong) > HeartbeatTimeout` (30s). FEAT-0008 specifies a two-stage degradation: 3 consecutive missed pongs -> `degraded`, 5 consecutive missed pongs -> `reconnecting` (lines 418-419).
- **Spec reference:**
  - `.sdlc/features/0008-bff-server.md:418` -- "After 3 consecutive missed pongs, harness transitions to `degraded`."
  - `.sdlc/features/0008-bff-server.md:419` -- "After 5 consecutive missed pongs, harness transitions to `reconnecting`."
  - `.sdlc/features/0008-bff-server.md:1224` -- "3 missed pongs at 15s interval = 45s"
- **Why blocking:** The design skips the degraded state entirely for heartbeat failures, going straight to reconnecting. This contradicts the spec's two-stage model and means the user never sees the `[degraded]` indicator for heartbeat issues (FEAT-0009 success criterion 22). The timeout math also differs: the design uses a flat 30s timeout, while the spec implies 45s (3 x 15s) for degraded and 75s (5 x 15s) for reconnecting.
- **Suggested fix:** Track a `missedPongs` counter. On each heartbeat tick, if the ping fails or no pong is received, increment the counter. At 3 missed pongs, transition to `StateDegraded`. At 5 missed pongs, transition to `StateReconnecting`. Reset the counter on successful pong. This aligns with FEAT-0008 and gives the user a visible warning before disconnection.

---

## Attention

### A-01. `ProtocolClient.Ping` sends a request but discards the `connection.pong` payload

- **What:** The `Ping` helper (D2.5, line 196) sends `connection.ping` as a JSON-RPC request (with `id`) and expects a response. FEAT-0008 says the server replies with `connection.pong`. The server design's `handleConnectionPing` returns `ConnectionPong` as the result payload. This works because JSON-RPC responses carry the result, but the client design's `Ping` method does not decode the response at all (`func (c *ProtocolClient) Ping(ctx context.Context) error`). It discards the result.
- **Why attention:** The Ping method works as a heartbeat (success = connection alive), but if the response ever carries diagnostic info (e.g., server time for clock skew detection), it will be silently dropped. Not blocking because the current spec does not require reading the pong payload.
- **Suggested fix:** Consider returning `(json.RawMessage, error)` or a `PongResponse` to future-proof, or document that the pong payload is intentionally discarded.

### A-02. `RegisterResponse` duplicates `CapabilitiesRegisterResponse` from server design

- **What:** The client design defines `RegisterResponse` (D2.5, lines 187-192) with fields `NegotiatedVersion`, `ServerVersion`, `MaxFrameSize`, `MaxAttachmentSize`. The server design defines `CapabilitiesRegisterResponse` (D5.2, lines 499-505) with identical fields. These are the same wire type.
- **Why attention:** Two designs independently define the same wire structure. If one changes during implementation and the other does not, they will diverge. Since `CapabilitiesRegisterResponse` is explicitly placed in `internal/bff/capabilities.go` (not in `internal/protocol/`), the client has to define its own copy. This is acceptable but fragile.
- **Suggested fix:** Consider adding a shared `protocol.RegisterResult` type in `internal/protocol/` (perhaps in WU-041 scope) so both sides decode the same struct. Alternatively, document the wire contract explicitly so both sides stay in sync.

### A-03. State machine uses separate `ConnState` type from server-side `ConnState`

- **What:** The client design (D3.2, lines 266-279) defines `ConnState` as `type ConnState int` in `internal/harness/`. The server design (D4.1, lines 272-308) defines its own `ConnState` as `type ConnState int` in `internal/bff/`. Both use the same iota values and state names, but they are distinct Go types in distinct packages.
- **Why attention:** This is intentional (stated in D1, line 28), but means the 9-state model has two sources of truth. If a state is added or reordered in one package, the other must be manually updated. The `harnessTransitions` map (D3.2) and `validTransitions` map (server D4.3) are disjoint subsets of the same state machine -- there is no compile-time check that they are consistent.
- **Suggested fix:** Consider defining the state constants in `internal/protocol/` as the canonical enum, with each side importing them and defining its own valid-transition subset. This gives a single iota ordering.

### A-04. `Notify` method exists but FEAT-0008 forbids harness notifications

- **What:** The `ProtocolClient` exposes a `Notify` method (D2.3, line 120) that sends a JSON-RPC notification (no `id`). FEAT-0008 (line 211 note) and `internal/protocol/protocol.go` (lines 112-121) both state: "Harness->server frames MUST use Request, not Notification."
- **Why attention:** Having the method available invites misuse. Any harness code that calls `Notify` instead of `Call` will produce a protocol violation that the server will reject.
- **Suggested fix:** Remove `Notify` from the public API, or rename it to something like `notifyInternal` if it is needed for testing. If there is a legitimate use case for harness-initiated notifications, document it and get a FEAT-0008 amendment.

### A-05. `Reconnecting -> Connecting` transition path implies multi-step reconnect, but prose describes it as atomic

- **What:** The `harnessTransitions` map (D3.2, line 296) allows `StateReconnecting -> StateConnecting`. The reconnect loop (D3.6, line 433) says it does "Dial() + Register() (full reconnection, not just TCP)". However, the transition map does not include `StateReconnecting -> StateRegistering` as a direct path. The reconnect must go through `Connecting -> Registering -> Ready`.
- **Why attention:** The transition map is technically correct (reconnect goes to Connecting, then Connecting goes to Registering), but the reconnect loop description says "Dial + Register" as if it is a single step. If the implementation treats reconnection as atomic (skip Connecting state, go straight to Registering), the state machine will reject it. The transition map and the prose should be consistent.
- **Suggested fix:** Clarify in D3.6 that the reconnect loop transitions through `Reconnecting -> Connecting -> Registering -> Ready` (three transitions), not a single "Dial + Register" step.

---

## Nit

### N-01. `FrameReader`/`FrameWriter` field names do not match protocol.go naming

- **What:** The `ProtocolClient` struct (D2.1, line 40) has fields `transport *protocol.FrameReader` and `writer *protocol.FrameWriter`. The field named `transport` is actually a `FrameReader`, which is confusing since "transport" usually means the full read+write layer. The server design uses `FrameTransport` as its combined type name.
- **Suggested fix:** Rename to `reader *protocol.FrameReader` for clarity.

### N-02. `SessionResume` helper takes decomposed args instead of the protocol struct

- **What:** The `SessionResume` helper (D2.5, line 201) takes `sessionID string, project protocol.ProjectContext` as separate parameters. The `protocol.SessionResume` struct (messages.go, line 157) has both `SessionID` and `Project` fields. The helper could take a `*protocol.SessionResume` directly for consistency with `SubmitTurn` (which takes `*protocol.TurnSubmit`).
- **Suggested fix:** Use `func (c *ProtocolClient) SessionResume(ctx context.Context, req *protocol.SessionResume) (json.RawMessage, error)` for API consistency.

### N-03. Missing `connection.ready` helper

- **What:** The protocol defines `MethodConnectionReady` and `ConnectionReady` struct (messages.go, lines 237-242). The client design has helpers for `Ping`, `Health`, `Register`, etc., but no `Ready` helper. The auto-start flow (D3.4, step 3) polls for readiness, which would benefit from a typed helper.
- **Suggested fix:** Add `func (c *ProtocolClient) Ready(ctx context.Context) (bool, error)` alongside `Ping` and `Health`.
