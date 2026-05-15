# 2026-04-18 — Session: Bundle 8 (Sessions & Conversation) partial

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Topic

Continued autonomous Phase 3 implementation. Landed Bundle 8's WU-050
(session management) and WU-051 (Conversation canonical format).
Deferred WU-052 (provider format translation and dispatch) because the
design depends on the ProviderRegistry introduced by WU-057 (Bundle 9)
which has not been built yet.

## Work Completed

### WU-051 — Conversation

`internal/bff/conversation.go`:

- Canonical `Conversation` type with 1-based monotonic sequence.
- `AppendUserTurn` validates submit.Sequence, converts protocol
  attachments / tool results into `provider.Message` form, returns a
  `storage.Turn`.
- `AppendAssistantTurn` paired for the model response, carrying
  metrics (model, provider, tokens, cost, latency).
- `RestoreFromTurns` rebuilds state from persisted turns on
  `session.resume`.
- `Reset` clears in-memory state for `session.clear`.
- `PendingToolCalls` / `MatchToolResult` implement the tool
  correlation contract: results on subsequent user turns resolve
  calls from the last assistant turn.
- `messageToTurn` / `turnToMessage` round-trip per design D3.6
  (`Turn.Content` is exactly `json.Marshal(provider.Message)`).

Commit `fb8f320`.

### WU-050 — Session management

`internal/bff/session.go`:

- `SessionManager` tracks `ActiveSession` in-memory state.
- `handleSessionResume`: GetSession → AcquireSessionLock (with
  `MT-CONN-008` on contention, `CodeSessionNotFound` on missing) →
  project-context refresh → `RestoreFromTurns` → `EnsureActive` →
  bind connection → cancel pending grace-period release.
- `handleSessionList`: scoped by connection `UserID` + project.
- `handleSessionDetails`: full detail with turns, events, files,
  pinned items.
- `handleSessionClear`: `Conversation.Reset` + `manual_clear` event.
- `handleSessionFork`: copies turns with new IDs, resets cost/tokens,
  appends `fork` event.
- Registered the five handlers via `SessionManager.Register` from
  `NewServer`.

Also added `SoloUserID = "local"` and `Connection.UserID()` returning
it — placeholder until auth lands.

Commit `f5b5628`.

## Files Created or Modified

Created:
- `internal/bff/conversation.go`
- `internal/bff/conversation_test.go`
- `internal/bff/session.go`
- `internal/bff/session_test.go`
- `.sdlc/history/2026-04-18-session-bundle-8-sessions-conversation.md`

Modified:
- `internal/bff/server.go` — `sessions` field, `Sessions()` accessor,
  registration call from `NewServer`.
- `internal/bff/connection.go` — `SoloUserID`, `Connection.UserID()`.
- `.sdlc/releases/v0.2.0/status.md`

## Deferred

**WU-052 (dispatch)** depends on `ProviderRegistry` from WU-057. A
stub now would duplicate work and risk design drift. Build WU-057
first, then WU-052.

**`handleTurnSubmit`** (design D4.5) is deferred for the same reason
— it orchestrates Session → Conversation → Dispatcher → streaming
relay.

## Next / Open Items

1. **Bundle 9 (Model config & routing, WU-057–060).** Unblocks WU-052.
2. **Track B scaffold (Bundle 5, WU-068–072).** Bubbletea/status bar/
   input area/viewport/Glamour. Depends only on WU-039 (done). Ideal
   for a worktree-isolated agent.
3. **WU-052 + WU-053 streaming relay** after Bundle 9.

## Notes

- `SessionManager.EnsureActive` is the common binding point for
  resume/clear/fork handlers.
- `session.fork` copies turns in a loop (no transaction). Acceptable
  for solo profile; revisit if performance matters.
- Default list limit is 50 per design D2.4; harness pagination comes
  with WU-084 (session explorer).
