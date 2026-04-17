# Phase 1 Cross-Flow Audit

**Date:** 2026-04-17
**Scope:** All 16 design docs, 10 end-to-end flows, protocol coverage, test completeness

## Findings Summary

| Severity | Found | Resolved |
|----------|-------|----------|
| CRITICAL | 2 | 2 |
| IMPORTANT | 11 | 11 |
| MINOR | 12 | documented for Phase 3 |

## CRITICAL (both resolved)

1. **#18**: `history.list` and `history.append` not in protocol methods. **Fixed**: added to messages.go (now 22 methods) with `HistoryAppend` and `HistoryList` types.

2. **#9**: `ModelSwitch.Action` field doesn't exist in protocol type. **Fixed**: WU-059 design updated to use `model: "auto"` sentinel (matching WU-039 and FEAT-0009).

## IMPORTANT (all resolved)

3. **#1**: No orchestration handler for `SubmitMsg → turn.submit`. **Fixed**: added `handleSubmit` method to App in WU-068 scaffold design with full flow (attachments, sequence, mode, timer, ProtocolClient call).

4. **#2**: `TurnSubmitAck` drops `Sync` field. **Fixed**: added `Sync json.RawMessage` field to match protocol.TurnSubmitResponse.

5. **#4**: No `ToolCallMsg` Bubbletea message. **Fixed**: added `ToolCallMsg`, `ToolResultMsg` to WU-068 message catalog.

6. **#6**: Permission prompt UI unspecified. **Fixed**: added `PermissionPromptMsg` and `PermissionResponseMsg` to WU-068.

7. **#7**: `SessionList` expects params that don't exist. **Fixed**: design updated — handler derives filter from connection state (UserID from auth, Project from capabilities). No params needed.

8. **#11**: `CompactSuggest` requires `turn_id` but emitter doesn't pass it. **Fixed**: `CheckPressure` gains `turnID` parameter.

9. **#13**: No branch lifecycle Msg types. **Fixed**: added `BranchStartedMsg`, `BranchCompleteMsg`, `BranchErrorMsg` to WU-068.

10. **#15**: Harness reconnection timing (75s) vs server lock release (40s). **Fixed**: documented dual detection — read loop EOF triggers immediate reconnection (milliseconds), missed pongs are the slow backup path for degraded-but-open connections.

11. **#22/#23/#24**: BFF handlers for connection.ready/health/ping not explicitly registered. **Fixed**: added `registerHandlers()` method to Server listing all 22 handlers.

12. **#25**: `status.update` event has no Bubbletea Msg. **Fixed**: added `StatusUpdateMsg`.

## MINOR (documented, deferred to Phase 3)

- #3: EventHandler→Msg dispatch mapping implicit (WU-074 ConnectionManager.HandleEvent covers this)
- #5: Tool name `tool` vs `name` — cosmetic
- #8: No `SessionDetails` helper on ProtocolClient — use raw Call()
- #10: `model.selected` outside turn has no turn_id — make optional in implementation
- #12: Compaction summarize dispatch needs raw turns, not Conversation — implementation detail
- #14: Per-branch cost via CostUpdate.branch_id correlation — implementation detail
- #16: Restart vs reconnection distinction — implementation detail
- #17: capabilities.update handler behavior — already covered by WU-049 D5.3
- #19: No typed ProtocolClient helper for history.list — use raw Call()
- #20: PasteDetectedMsg now in catalog (fixed with other Msg types)
- #21: max_output_tokens vs MaxLength — implementation will use protocol field name
- #27/#28: Unsolicited compact.plan and capabilities.request harness handlers — Phase 3
