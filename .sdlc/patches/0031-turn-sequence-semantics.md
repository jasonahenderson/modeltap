---
patch: "PATCH-0031"
title: "Align BFF Conversation.sequence with harness user-turn semantics"
status: "proposed"
date: "2026-05-11"
related:
  - "PATCH-0029 (bootstrapSession race — same error shape, different root cause)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F13)"
branch: "patch/0031-turn-sequence-semantics"
---

# PATCH-0031: Align BFF Conversation.sequence with harness user-turn semantics

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

`turn.submit` returns `-32602 turn.submit sequence 2 does not follow
current 2` on the **second** user turn after a clean session (no race,
session id correct, conversation state intact).

Tracing:

1. The harness (`internal/harnesshost/production_runtime.go`,
   `runtimeState.NextSequence` at line 134) increments its counter by
   **1 per user submit**: 1, 2, 3, ...
   The existing unit test
   `TestProductionRuntimeSubmitTurnRecordsServerSession` explicitly
   asserts `second submit sequence = 2`.

2. The BFF (`internal/bff/conversation.go`,
   `AppendUserTurn` line 108 and `AppendAssistantTurn` line 167)
   increments `c.sequence` on **every** message — user **and**
   assistant. So after one accepted turn round-trip, `c.sequence == 2`.

3. On the user's second turn the harness sends `sequence=2`. The BFF
   expects `c.sequence+1 == 3`. Validation rejects:
   `sequence 2 does not follow current 2`.

The bug went undetected because the BFF stub used by the harness's
unit tests (`internal/harnesshost/testutil/bffstub.go`,
`turn.submit` handler around line 170) accepts any sequence value
without validating it. The real BFF rejects it.

Distinct from PATCH-0029, which fixed the same error message but a
different root cause (session id race; current was 0, not 2). Both
patches need to be in place to make the two-turn flow work.

## Scope

1. **Split user-turn validation from storage ordering in
   `internal/bff/conversation.go`.** Introduce a separate
   `userSequence int` counter. The wire-contract validation
   (`submit.Sequence == userSequence+1`) uses `userSequence`; the
   storage `Turn.Sequence` continues to advance per-message so
   `ListTurns ORDER BY sequence` keeps user→assistant ordering.

2. **Update producers.** `AppendUserTurn` increments both
   `userSequence` and `sequence` (and assigns the message
   `sequence`). `AppendAssistantTurn` increments `sequence` only.

3. **Update `Sequence()` accessor.** The public `Sequence()` getter
   reports `userSequence` (the wire concept). A new internal
   accessor exposes the storage counter for tests that need it.

4. **Fix `RestoreFromTurns`.** Recompute `userSequence` from the
   count of `role=user` turns in the restored history; recompute
   `sequence` from the max `Turn.Sequence`.

5. **Update affected unit tests.**
   `internal/bff/conversation_test.go`,
   `internal/bff/streaming_test.go`,
   `internal/bff/sync_test.go` reset their post-assistant sequence
   expectations to match user-turn semantics.

6. **Add regression test.** New case in `conversation_test.go`
   covering the two-user-turns flow: user1(seq=1) → assistant →
   user2(seq=2) must succeed; user2(seq=3) must fail.

## Out of Scope

- **Tightening the harness BFF stub.** The stub in
  `internal/harnesshost/testutil/bffstub.go` could be taught to
  validate sequence per real-BFF rules, but that's a separate
  hardening task. This patch only fixes the production contract;
  the real BFF unit test added in (6) is the definitive guard.

- **Storage migration.** Existing `turns.sequence` rows are
  per-message ordering and stay that way. No data shape changes.

- **Protocol doc revision.** `protocol.TurnSubmit.Sequence` is
  already documented as the user submission number in the
  conversation_test contract; the doc comment in
  `internal/bff/conversation.go` is updated to match.

## Checklist

- [ ] `Conversation` carries both `userSequence` and `sequence`
- [ ] `AppendUserTurn` validates against `userSequence` and
  advances both counters
- [ ] `AppendAssistantTurn` advances `sequence` only
- [ ] `RestoreFromTurns` recomputes both counters from persisted
  history
- [ ] BFF unit tests updated (`conversation_test.go`,
  `streaming_test.go`, `sync_test.go`)
- [ ] Regression test covers two-user-turns flow
- [ ] `go build ./...`, `go vet ./...`, `go test ./internal/bff/...
  ./internal/harnesshost/...` pass
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` gets a Finding F13
  entry pointing at this patch

## Fix Detail

Producer changes (sketched):

```go
// AppendUserTurn
if submit.Sequence != c.userSequence+1 {
    return nil, &TransportError{ ... }
}
c.userSequence = submit.Sequence
c.sequence++
turn := messageToTurn(c.sessionID, c.sequence, msg, TurnMetadata{})

// AppendAssistantTurn (unchanged shape; sequence still per-message)
c.sequence++
turn := messageToTurn(c.sessionID, c.sequence, msg, meta)
```

`Sequence()` returns `c.userSequence` (the contract value callers
care about). `RestoreFromTurns` walks the restored turns and counts
roles instead of just taking the max sequence.

Why split the counters rather than collapse to one user-turn
counter? `storage.Turn.Sequence` is the message-ordering field
read by `ListTurns ... ORDER BY sequence` and by
`RestoreFromTurns` itself. Reusing it for the wire contract would
either require user and assistant rows to share sequence numbers
(loses ordering) or require collisions on duplicate values. Two
counters keeps both semantics clean with no storage migration.
