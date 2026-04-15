# FEAT-0008 Connectivity Review

- Feature: `docs/features/0008-bff-server.md`
- Review date: 2026-04-15
- Reviewer: peer review — harness/BFF connectivity
- total_findings: 5
- blocking: 4
- significant: 1
- advisory: 0
- top_line: FEAT-0008 continues to improve as a BFF/server spec, especially around protocol framing, session handoff, model registry, and routing transparency. The harness/BFF connection is still not airtight because self-recovery, health/readiness, in-flight resume semantics, and actionable diagnostics remain implicit rather than specified as testable behavior.

## Review Scope

This review focuses only on the connectivity contract between the terminal harness and the BFF/proxy server:

- Local Unix socket and remote TLS connection establishment
- Service discovery, local auto-start, and stale socket recovery
- Heartbeat, readiness, liveness, and dependency health checks
- Recovery after BFF crash, harness crash, network drop, auth expiry, version mismatch, or provider registry degradation
- In-flight turn and tool-call recovery after reconnect
- User-visible communication and suggested remediation when automatic recovery cannot fix the issue

## What The Current Spec Now Covers Well

- Protocol framing is defined as NDJSON JSON-RPC over Unix socket or TLS.
- Request correlation, streaming event correlation, cancellation, tool-call round trips, capability registration, and protocol version negotiation are specified at a useful first-pass level.
- Session conflict is resolved: only one harness may actively own a session, with `session.active` on conflicting resume.
- Session handoff is partially resolved: disconnected sessions become resumable after a grace period, heartbeat timeout releases stuck locks, and users can force unlock.
- Model configuration now has a more explicit provider endpoint, model registry, and routing-policy structure. This improves model transparency, but also increases the need for machine-readable health because provider discovery and routing can degrade independently of the socket connection.

## Findings

### F1 — Blocking

**Reviewer:** Connectivity Reliability

**Affected sections:** Harness Protocol Endpoint, Protocol Specification, Session Persistence, Resolved Questions, Open Questions, Success Criteria

**Summary:** The feature still lacks a connection lifecycle state machine, so self-curing behavior remains implementation-defined.

**Detail:** FEAT-0008 now references reconnect and heartbeat timeout for session handoff, but it still does not define the lifecycle of the harness/BFF connection itself. There is no normative sequence for `discovering -> starting local service -> connecting -> authenticating -> registering capabilities -> ready -> degraded -> reconnecting -> failed`, no retry/backoff policy, no transient-vs-terminal failure taxonomy, and no local repair policy for stopped services or stale sockets. For an "airtight" connection, these behaviors need to be product requirements, not implementation judgment.

**Recommendation:** Add a "Connection Lifecycle and Self-Recovery" section. Define connection states, transitions, retry timing, max retry behavior, local-service auto-start rules, stale socket cleanup rules, remote reconnect behavior, and the exact point at which the harness stops retrying and tells the user what to do.

### F2 — Blocking

**Reviewer:** Health and Readiness

**Affected sections:** Protocol Messages, Model Configuration, CLI Integration, Resolved Questions, Success Criteria

**Summary:** Heartbeat timeout is mentioned, but heartbeat, readiness, and dependency health protocol primitives are still absent.

**Detail:** The spec says the server detects broken connections via heartbeat timeout, but it does not define heartbeat messages, interval, timeout, missed-heartbeat threshold, readiness checks, or machine-readable health output. This gap is larger now that FEAT-0008 includes provider endpoint discovery, model registry status, routing fallback, storage, auth, and capability registration. The harness needs to know whether the BFF is merely reachable or actually ready to serve a turn with a healthy store, compatible protocol, registered tools, and available route target.

**Recommendation:** Add protocol methods such as `connection.ping`, `connection.pong`, `connection.health`, and `connection.ready`, or equivalent JSON-RPC methods. Define heartbeat interval/timeout defaults. Health should report server version, supported and negotiated protocol version, uptime, listener state, active session id, auth state, capability registration state, storage readiness, provider endpoint status, model registry status, routing readiness, and degraded dependencies.

### F3 — Blocking

**Reviewer:** Recovery Semantics

**Affected sections:** Protocol Specification, Streaming Relay, Session Persistence, Session and Turn Storage Model, Open Questions, Success Criteria

**Summary:** Reconnect behavior is not defined for in-flight turns, partial streams, pending tool calls, or multi-model streams.

**Detail:** Sessions survive reconnect at a high level, but the spec still does not define what happens if the connection drops during a provider stream, while a `tool.call` is waiting for `tool.result`, or after the server emitted tokens the harness did not receive. The newer multi-model routing behavior adds another failure mode: interleaved `token.delta` events can be tagged by reviewer/model, but there is no resume contract for partially completed parallel streams. The turns table has `id`, `sequence`, `tool_calls`, and content fields, but no idempotency or synchronization contract.

**Recommendation:** Define turn-level idempotency and resume rules. `turn.submit` should carry a stable client-generated `turn_id`, session id, and sequence number. `tool.result` should be idempotent by `tool_call_id`. Add `session.sync` or `turn.resume` returning the authoritative state of the active turn, pending tool calls, completed model branches, failed model branches, and whether token replay is available. If token replay is not supported, define the exact recovery summary the server returns.

### F4 — Blocking

**Reviewer:** User Communication

**Affected sections:** Protocol Messages, CLI Integration, Success Criteria, Resolved Questions, Open Questions

**Summary:** User-facing diagnostics are still generic and do not meet the "clearly communicative with suggestions" requirement.

**Detail:** The spec has a generic `error` event, `session.active`, `modeltap server status`, and force-unlock commands. It still does not define stable diagnostic codes, error categories, or required remediation text. Users need precise messages for service not installed, service stopped, stale socket, socket permission denied, TLS trust failure, auth expired, protocol version mismatch, session locked, provider endpoint down, model unavailable, routing fallback, storage unready, and capability registration failure.

**Recommendation:** Add a diagnostic taxonomy. Every connection failure should include a stable code, short cause, automatic repair attempted, path/endpoint involved, and suggested next command. Example codes: `MT-CONN-001 service_not_running`, `MT-CONN-002 stale_socket`, `MT-CONN-003 socket_permission_denied`, `MT-CONN-004 version_mismatch`, `MT-CONN-005 tls_untrusted`, `MT-CONN-006 auth_expired`, `MT-CONN-007 storage_unready`, `MT-CONN-008 session_locked`, `MT-CONN-009 provider_unavailable`, `MT-CONN-010 capability_registration_failed`.

### F5 — Significant

**Reviewer:** Service Integration

**Affected sections:** Harness Protocol Endpoint, CLI Integration, Configuration, Relationship to ADRs, Open Questions

**Summary:** Local service bootstrap and session unlock behavior are still not fully integrated into CLI and configuration semantics.

**Detail:** FEAT-0008 relies on FEAT-0004/ADR-0012 for service management and defines a Unix socket path, but it does not define how the harness determines whether the local BFF is installed, stopped, stale, wrong-version, using a different config file, or safe to auto-start. The open question text introduces `modeltap session unlock <id>` and `/session unlock`, but the CLI Integration section still only lists `modeltap server status`, `modeltap server sessions`, and `modeltap server session <id>`. The connection repair path should be visible in the CLI contract.

**Recommendation:** Add a local bootstrap algorithm and CLI commands. The algorithm should check configured socket path, validate socket ownership and mode, query service status, auto-start service or subprocess when allowed, wait for readiness, clean stale sockets only when safe, and print fallback commands when repair is unsafe. Add unlock/status/diagnostic commands introduced by connectivity recovery to CLI Integration.

## Required Acceptance Additions

Before FEAT-0008 is accepted, add success criteria covering:

1. The harness auto-starts or reconnects to the local BFF in the solo profile without user action when the service is installed but stopped.
2. The harness detects stale socket files and repairs them when safe, or prints a specific remediation command when not safe.
3. The harness detects half-open or wedged connections within a bounded heartbeat interval and reconnects with exponential backoff.
4. Reconnection after harness crash, BFF crash, and network drop preserves session identity and reports the final state of any in-flight turn.
5. Replayed `turn.submit` and `tool.result` messages are idempotent and do not duplicate model calls or tool effects.
6. Multi-model streamed turns can be synchronized after reconnect, including per-model completed/failed/pending state.
7. Every connection failure maps to a documented diagnostic code with a user-facing cause, repair attempt, and suggested next command.
8. `modeltap server status` reports machine-readable and human-readable connection health, including socket/TLS listener state, protocol version, auth readiness, storage readiness, provider endpoint readiness, model registry status, routing readiness, and active sessions.
9. Version mismatch, auth expiry, TLS trust failure, permission-denied socket, stale socket, provider-down, model-unavailable, storage-unready, capability-registration-failed, and session-locked scenarios each have tests asserting the diagnostic category.

## Suggested Feature Text

Add a section like this to FEAT-0008:

```markdown
### Connection Lifecycle and Self-Recovery

The harness treats the BFF connection as a managed lifecycle, not a raw socket.

Connection states:
`discovering -> starting -> connecting -> authenticating -> registering -> ready -> degraded -> reconnecting -> failed`.

The harness automatically attempts recovery for transient local failures:
- starts the local service if it is installed but stopped
- removes stale socket files when no live process owns them
- retries connection with exponential backoff
- re-registers capabilities after reconnect
- runs `session.sync` to recover active session and in-flight turn state

Heartbeat:
- harness sends `connection.ping` every N seconds while connected
- server replies with `connection.pong`
- after M missed heartbeats, the harness enters `degraded`, then `reconnecting`
- server releases active-session locks after heartbeat timeout plus grace period

Readiness:
- `connection.ready` returns whether auth, storage, capability registration, provider endpoints, model registry, and routing policy are ready
- degraded dependencies are returned with diagnostic codes and suggested remediation

When automatic recovery cannot safely resolve the issue, the harness prints a diagnostic with:
- stable code
- human-readable cause
- automatic repair attempted
- likely next command
- path or endpoint involved

Example:
`MT-CONN-002 stale socket: modeltap found ~/.local/share/modeltap/server.sock, but no server is listening. Removed stale socket and restarted service.`
```

## Bottom Line

The current spec is stronger as a BFF feature, but the harness/proxy connection is still not "air tight." Connectivity needs to be elevated to a first-class feature surface with explicit self-healing, health, idempotent resume, and user diagnostic requirements.
