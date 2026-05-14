---
patch: "PATCH-0029"
title: "Fix bootstrapSession race that overwrites turn-assigned session id"
status: "approved"
date: "2026-05-08"
related:
  - "PATCH-0028 (session.create RPC + harness auto-call)"
  - "docs/releases/v0.3.0/retrospective.md (Finding F11)"
branch: "patch/0029-bootstrap-session-race"
---

# PATCH-0029: Fix bootstrapSession race that overwrites turn-assigned session id

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

`turn.submit` returns `-32602 turn.submit sequence 2 does not follow
current 0` on the second user turn after a fresh shell launch.
Tracing:

1. Shell connects; harness reaches `ConnStateReady` and PATCH-0028
   spawns `bootstrapSession` as a background goroutine.
2. User types and submits a turn before `session.create` returns.
   `turn.submit` is sent with `session_id=""`, so the BFF
   auto-creates session **B** and the response sets
   `r.mode.SessionID(B)`. The conversation for B advances to
   sequence 1.
3. `bootstrapSession`'s `session.create` response lands a moment
   later carrying session **A**; the goroutine calls
   `r.mode.SetSessionID(A)`, overwriting B.
4. The user's next turn submits against session A. Harness sequence
   counter is at 2 (incremented locally). BFF reads A's conversation
   (sequence 0). Validation rejects: `sequence 2 does not follow
   current 0`.

Recorded as Finding F11 in `docs/releases/v0.3.0/retrospective.md`.

## Scope

1. **Re-check `r.mode.SessionID()` inside `bootstrapSession` before
   setting it.** The existing early-return at the top of the
   function is insufficient because the session id can become
   non-empty during the RPC roundtrip (turn.submit auto-creates a
   session and stores its id). After `session.create` returns,
   only adopt the new id when the harness still has none.

2. **Best-effort: do not error or surface the discarded
   bootstrap session.** It becomes an orphaned but harmless row in
   storage. A future patch may clean these up (or reuse them via
   stale-session GC), but that's out of scope here.

3. **Test:** new unit test in
   `internal/harnesshost/production_runtime_test.go` that uses the
   BFFStub to:
   - Construct a runtime, drive it through `ConnStateReady`.
   - Set `r.mode.sessionID` synthetically (simulating a fast
     turn.submit) before bootstrapSession completes.
   - Verify bootstrapSession does not overwrite the existing id.

## Out of Scope

- **Synchronous bootstrap (block shell startup until
  `session.create` completes).** Cleaner contract but adds latency
  to shell launch. Re-check is the smaller fix and addresses the
  actual user-visible failure.
- **Cleanup of orphaned bootstrap sessions in storage.** Worth a
  background sweep eventually; not now.
- **Sequence-counter recovery.** If the user *did* hit the race
  before this patch and is on a session with mismatched sequence,
  this patch does not retroactively fix that — but a fresh shell
  start avoids it.

## Checklist

- [ ] `bootstrapSession` re-checks `r.mode.SessionID()` after the
  RPC and skips the Set when non-empty
- [ ] Unit test covers the race shape (existing id wins)
- [ ] `go test ./...` passes
- [ ] Smoke verification: launch shell, type quickly, verify no
  `sequence does not follow` error on the second turn
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/retrospective.md` Finding F11 status
  updated to "Fixed in PATCH-0029"
