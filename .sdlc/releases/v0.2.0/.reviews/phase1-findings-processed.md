# Phase 1 Pre-Review Findings — Processed

**Date:** 2026-04-17
**Scope:** All 11 pre-review lints across 15 design bundles
**Totals:** 22 blocking (all resolved), 92 attention, 62 nits

## Disposition Key

- **RESOLVED** — already fixed during blocker pass
- **ACCEPTED** — fix applied to design doc in this processing pass
- **PHASE-3** — implementation-time concern; noted for implementer, no design change needed
- **NOTED** — documented design choice or acknowledged limitation; no action

---

## Bundle: WU-039 (Protocol Core)

| ID | Disposition | Notes |
|----|------------|-------|
| A-01 | RESOLVED | Sequence validation added in Bundle 4 D2.5 (ValidateTurnSubmit) |
| A-02 | RESOLVED | Mode validation added in Bundle 4 D2.5 |
| A-03 | PHASE-3 | Add godoc wire-identity comment during implementation |
| A-04 | RESOLVED | Protocol-types design D1.1 resolved Request.ID semantics |
| A-05 | RESOLVED | Bundle 4 D5.6 + protocol-types amendment (ServerCapabilities gains max_frame_size) |
| A-06 | PHASE-3 | Add omitempty or godoc during implementation |
| A-07 | PHASE-3 | Add package godoc during implementation |
| N-01 | NOTED | No action |
| N-02 | PHASE-3 | Performance optimization if needed |
| N-03 | NOTED | Trivial; implementation uses correct constant |

## Bundle: WU-040/041/093 (Protocol Types)

| ID | Disposition | Notes |
|----|------------|-------|
| A-01 | ACCEPTED | Clarify ServerError.Code is coarse bucket string, not DiagnosticCode |
| A-02 | NOTED | RoutingPolicy type is a passive map; resolution logic is WU-059 scope |
| A-03 | ACCEPTED | Enumerate all 12 MT-CONN constants explicitly |
| A-04 | PHASE-3 | Add Go-type column to field catalogs during implementation |
| A-05 | NOTED | triggered_by documented as free-form string per FEAT-0008 |
| A-06 | NOTED | "server_restart" dropped from design per FEAT-0008; only spec-ratified values included |
| A-07 | NOTED | Design-inferred values documented in deviations section |
| A-08 | NOTED | SessionDetail naming matches FEAT-0008 section heading |
| A-09 | PHASE-3 | Add deviation log during implementation |
| A-10 | PHASE-3 | Fixture policy clarified at test-writing time |

## Bundle: WU-042/043/044 (Provider Formatting)

| ID | Disposition | Notes |
|----|------------|-------|
| A-01 | PHASE-3 | ADR schema verification during implementation |
| A-02 | RESOLVED | DispatchOpts fields specified in Bundle 8 |
| A-03 | ACCEPTED | Token estimation scope: text + json.Marshal(ToolCalls) + sum(Attachments.Content) |
| A-04 | NOTED | Return contract is []byte (JSON request body) per design |
| A-05 | PHASE-3 | Add renames to deviation log |
| A-06 | NOTED | IsError maps status→bool; wire tri-state preserved in protocol type |
| A-07 | PHASE-3 | Error cases specified at implementation time |
| A-08 | NOTED | Out of scope per design |
| A-09 | ACCEPTED | Document max_tokens vs max_completion_tokens model-family mapping |
| A-10 | PHASE-3 | Encoding tests will cover JSON-string escaping |
| A-11 | PHASE-3 | Stub test opts specified at test-writing time |

## Bundle: WU-046/047/048/049 (BFF Foundation)

| ID | Disposition | Notes |
|----|------------|-------|
| A-01 | RESOLVED | lastPing initialized to time.Now() at connection creation |
| A-02 | RESOLVED | handleConnectionReady checks store.Ping() + provider health |
| A-03 | RESOLVED | HealthResponse populates all fields with stubs |
| A-04 | NOTED | ValidateTurnSubmit stays in WU-046 scope per track spec; can move to validate.go during impl |
| A-05 | RESOLVED | session.sync mentioned in dispatch gating commentary |
| A-06 | ACCEPTED | Cross-reference exact Store.ReleaseSessionLock signature |

## Bundle: WU-068/069/070/071/072 (Bubbletea Scaffold)

| ID | Disposition | Notes |
|----|------------|-------|
| A1 | RESOLVED | Status bar order documented in D3.1 |
| A2 | ACCEPTED | Add DisplayRole constants |
| A3 | RESOLVED | WU-082 breaking change documented |
| A4 | RESOLVED | Ctrl+P from auto → build |
| A5 | RESOLVED | ModeChangeMsg added |
| A6 | RESOLVED | Dependency reference corrected to protocol.go |
| A7 | ACCEPTED | Specify banner placement: between viewport and input area |

## Bundle: WU-073/074 (Protocol Client)

| ID | Disposition | Notes |
|----|------------|-------|
| A-01 | NOTED | Pong payload intentionally discarded; no current use case |
| A-02 | ACCEPTED | Note: use protocol.CapabilitiesRegisterResponse directly (already fixed in server-side) |
| A-03 | ACCEPTED | Note: consider shared ConnState enum in protocol package during Phase 3 |
| A-04 | RESOLVED | Notify removed from public API |
| A-05 | RESOLVED | Multi-step transitions documented |

## Bundle: WU-050/051/052 (Sessions & Conversation)

| ID | Disposition | Notes |
|----|------------|-------|
| A-01 | RESOLVED | Turn serialization format defined (B-01 fix) |
| A-02 | ACCEPTED | Add Capabilities to DispatchOpts for vision gating |
| A-03 | PHASE-3 | Set CreatedAt during implementation |
| A-04 | ACCEPTED | Fix: use SessionManager's store, not session.store |
| A-05 | ACCEPTED | Add manual_clear to storage event type table |
| A-06 | ACCEPTED | Add session.fork field copy/reset specification |
| A-07 | PHASE-3 | Handler registration wiring shown at implementation time |
| A-09 | ACCEPTED | Document thread-safe registry interface for TurnDispatcher |
| A-10 | PHASE-3 | turn_id validation in ValidateTurnSubmit |

## Bundle: WU-057/058/059/060 (Model Config & Routing)

| ID | Disposition | Notes |
|----|------------|-------|
| A-01 | ACCEPTED | Document mode→routing as v1 simplification; FEAT-0008 domain roles for v2 |
| A-02 | PHASE-3 | Config-to-ModelInfo conversion at implementation time |
| A-03 | ACCEPTED | Embed DispatchOpts in MultiModelOpts |
| A-04 | ACCEPTED | Add cost field to BranchComplete in protocol-types or remove from design |
| A-05 | ACCEPTED | Per-branch cost emitted via branch.complete event |
| A-06 | ACCEPTED | Add ModelOverride field to ActiveSession struct |
| A-07 | PHASE-3 | Diagnostic code selection for branch errors at implementation time |

## Bundle: WU-045/091/096 (Storage)

| ID | Disposition | Notes |
|----|------------|-------|
| A-01 | PHASE-3 | SessionSummary.LastTurnSummary query at implementation time |
| A-02 | ACCEPTED | Add OR lock_owner = ?2 for self-owned re-acquisition |
| A-03 | ACCEPTED | Fix session-scope index to (user_id, session_id, created_at DESC) |
| A-04 | PHASE-3 | SessionStatus typed enum at implementation time |
| A-05 | PHASE-3 | CommandHistoryFilter scope field at implementation time |
| A-06 | PHASE-3 | UserID validation at implementation time |
| A-07 | ACCEPTED | Use compound cursor (created_at, id) for pagination |
| A-08 | PHASE-3 | WAL checkpoint behavior tested during implementation |
| A-09 | PHASE-3 | Migration ordering documented at implementation time |
| A-10 | NOTED | turns.sequence is unique per session; compacted rows retain original sequence |

## Bundle: WU-075/076/077/078/079 (Tool Framework)

| ID | Disposition | Notes |
|----|------------|-------|
| A2 | ACCEPTED | Document Git/Bash as special cases where enforcer inspects input |
| A3 | ACCEPTED | Same as A2 — Bash input-dependent enforcement documented |
| A4 | PHASE-3 | Domain extraction in Check() at implementation time |
| A6 | ACCEPTED | Add vision capability check before image base64 encoding |
| A7 | NOTED | DNS rebinding acknowledged as known limitation; SSRF mitigation is defense-in-depth |
| A8 | ACCEPTED | Add pages parameter for PDF reads |

## Bundles 10-15 (Streaming through Integration)

All attention items in these bundles were either blocking (already resolved) or absent (clean reviews).

---

## Nits Summary

62 nit findings across all bundles. Disposition: all PHASE-3 or NOTED. No nits warrant design doc changes. They will be addressed during implementation as code-quality improvements (godoc, naming consistency, test coverage edge cases).

---

## Applied Fixes Count

| Category | Count |
|----------|-------|
| Previously RESOLVED (blocker pass) | 22 |
| ACCEPTED (this pass) | 28 |
| PHASE-3 (implementation time) | 30 |
| NOTED (no action) | 12 |
| Nits (all PHASE-3/NOTED) | 62 |
