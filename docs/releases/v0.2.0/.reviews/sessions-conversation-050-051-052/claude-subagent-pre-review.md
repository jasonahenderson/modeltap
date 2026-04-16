# Pre-Review: Sessions & Conversation Bundle (WU-050 + WU-051 + WU-052)

**Reviewer:** Claude subagent (pre-review lint)
**Date:** 2026-04-16
**Design doc:** `docs/history/2026-04-16-design-sessions-conversation-050-051-052.md`

**Checked against:**
- FEAT-0008 (`docs/features/0008-bff-server.md`)
- Track A WU descriptions (`docs/releases/v0.2.0/track-a-bff-server.md`)
- BFF foundation design (WU-046/047/048/049)
- Protocol types design (WU-040/041/093)
- Storage design (WU-045/091/096)
- Provider formatting design (WU-042/043/044)

---

## Blocking

### B-01: `Conversation.turns` uses `provider.Message` but storage round-trip uses `storage.Turn`

**Location:** D3.1 vs. D3.6

The `Conversation` struct holds `turns []provider.Message` (D3.1), but `RestoreFromTurns` takes `[]storage.Turn` (D3.1) and the conversion helpers `turnToMessage` / `messageToTurn` (D3.6) bridge between the two types. The issue is that `provider.Message.Content` is a plain `string`, while `storage.Turn.Content` is `json.RawMessage` (per the storage design D3). The `turnToMessage` function must unmarshal the JSON content into something the `provider.Message` can hold, but `provider.Message.Content` is just a string — it cannot hold structured content (tool calls, attachments, metadata are separate fields on `provider.Message`).

The design does not specify how `turnToMessage` decomposes `storage.Turn.Content` (which is "canonical message JSON" per the storage schema) into the separate `provider.Message` fields (`Content`, `ToolCalls`, `ToolResults`, `Attachments`, `Metadata`). Without this, implementers must guess the serialization format, risking a mismatch between what `messageToTurn` writes and what `turnToMessage` reads.

**Fix:** Define the canonical JSON schema stored in `turns.content` explicitly. State whether it is the JSON serialization of `provider.Message` itself, or a separate envelope. Specify the unmarshal target type in `turnToMessage`.

### B-02: `session.resume` handler returns `protocol.SessionDetail` but protocol types define `SessionResumeResponse`

**Location:** D2.3 (line 93) vs. protocol types design (sessions.go)

The design says `session.resume` returns `protocol.SessionDetail`, but the protocol types design defines a dedicated `SessionResumeResponse` type with fields `{session_id, model, model_override, project ProjectContext}` -- a much lighter payload than `SessionDetail` (which includes turns, server_events, files_touched, etc.). Returning the full `SessionDetail` on resume would be unnecessarily heavy and inconsistent with the protocol contract.

**Fix:** Change `handleSessionResume` to return `protocol.SessionResumeResponse`. If the design intends to also load turns for the in-memory `Conversation` during resume, that is an internal operation that does not affect the wire response type.

### B-03: `session.sync` not handled anywhere in this bundle

**Location:** FEAT-0008 lines 455-493 vs. design scope

FEAT-0008 lists `session.sync` as a harness-to-server message. The protocol types design defines `SessionSyncResponse`. The BFF foundation design (D4.4) notes that `session.sync` is "typically the first method" after reconnection in `ConnReady` state. However, this bundle's design does not register or implement a `session.sync` handler, nor does it mention it in the out-of-scope list. The "Interfaces Exported" section references WU-064 for `session.sync`, but WU-064 depends on WU-050 (this bundle). If WU-064 implements the handler, it needs access to `SessionManager` internals (active session state, pending tools, branch state) that are not exported.

**Fix:** Either (a) add a `session.sync` handler stub in WU-050 that returns the current session state (active turn, pending tools), with multi-model state deferred to WU-064; or (b) explicitly list `session.sync` in the out-of-scope section and ensure `SessionManager` exports enough state for WU-064 to implement it externally.

---

## Attention

### A-01: `storage.Turn.Content` type mismatch with `provider.Message.Content`

**Location:** D3.2 (line 208)

`AppendUserTurn` builds a `provider.Message{Role: "user", Content: submit.Content, ...}` and then must produce a `storage.Turn` for persistence. But `storage.Turn.Content` is `json.RawMessage` (per storage design D3) while `submit.Content` is a plain string. The design's `messageToTurn` signature (D3.6) takes a `provider.Message` and produces `*storage.Turn`, but does not specify how `provider.Message` is serialized into the `json.RawMessage` content field. The `TurnMetadata` struct in D3.6 duplicates fields already on `provider.Message` (ToolCalls) and `storage.Turn` (FilesTouched, FilesModified), creating ambiguity about the source of truth during conversion.

**Fix:** Clarify that `messageToTurn` marshals the full `provider.Message` into `storage.Turn.Content` as JSON. Remove `ToolCalls` from `TurnMetadata` if it is already on the `provider.Message`, or document why both are needed (e.g., `TurnMetadata.ToolCalls` might carry resolved IDs not present on the canonical message).

### A-02: `DispatchOpts.Tools` uses `protocol.ToolDefinition` but `FormatMessagesOpts.Tools` also uses `protocol.ToolDefinition` -- consistent but undocumented assumption

**Location:** D4.2 (line 316-319) vs. provider design D2

Both `DispatchOpts.Tools` and `FormatMessagesOpts.Tools` are `[]protocol.ToolDefinition`. This is correct and consistent with the provider design. However, the dispatch flow (D4.2 step 2) maps `opts.Tools` directly to `fmOpts.Tools` without filtering by model capabilities. The provider design (D2) includes `Capabilities []string` on `FormatMessagesOpts` for vision gating and capability-based tool filtering, but `DispatchOpts` does not include a `Capabilities` field.

**Fix:** Add a `Capabilities []string` field to `DispatchOpts` (populated from the model registry metadata in WU-058/059) so the dispatch flow can pass it through to `FormatMessagesOpts.Capabilities`. Without this, vision gating and capability-based tool omission will not work when the dispatch path is wired.

### A-03: `handleTurnSubmit` wiring references `store.AppendCommandHistory` but the storage design uses `ctx` as first argument

**Location:** D4.5 (line 393-398) vs. storage design D4

The design shows `conn.server.store.AppendCommandHistory(ctx, &storage.CommandHistoryEntry{...})`. The `CommandHistoryEntry` struct (per storage design D3) has fields `UserID`, `Project`, `SessionID *string`, `Content`, `CreatedAt`. The `handleTurnSubmit` code populates `UserID`, `Project`, `SessionID`, `Content` but omits `CreatedAt`. Since `CreatedAt` is `time.Time` (not a pointer), it will default to the zero value, which is likely incorrect.

**Fix:** Either set `CreatedAt: time.Now()` in the handler code, or document that the storage layer sets `CreatedAt` to `NOW()` if the Go zero value is provided.

### A-04: `updateSummary` accesses `session.store` but `ActiveSession` has no `store` field

**Location:** D2.9 (line 159-168)

The `updateSummary` method calls `session.store.UpdateSession(...)` but the `ActiveSession` struct (D2.1) has no `store` field. The method receiver is `*SessionManager` which does have a `store` field.

**Fix:** Change the call to `sm.store.UpdateSession(...)` to use the `SessionManager`'s store. The `Session` struct passed to `UpdateSession` should also use `session.ID` not the full `storage.Session` literal (since `ActiveSession.ID` is a string, not a `storage.Session`).

### A-05: `session.clear` does not document what `session_events` type value it appends

**Location:** D2.6 (line 126)

The design says clear "appends a `session_events` entry of type `"manual_clear"`" but this event type is not listed in the storage design's payload schema table (D2, `session_events` section), which only defines `auto_compact`, `server_restart`, and `manual_compact`. The converter function `sessionEventToProtocol` (storage design) will map it but the payload schema for `manual_clear` is undefined.

**Fix:** Add `manual_clear` to the storage design's payload schema table, or note that the storage bundle needs amendment before Phase 3.

### A-06: `session.fork` does not copy `compaction_state` from session record

**Location:** D2.7 (line 137-141)

Step 3 says "Copy pinned items and compaction state" but the implementation list (steps 1-6) does not explicitly mention copying `compaction_state`, `routing_overrides`, `active_model`, `model_override`, `total_cost`, `total_input_tokens`, `total_output_tokens`, or `context_pct` from the source session. Some of these should be copied (compaction_state, pinned_items, active_model, model_override, routing_overrides) and some should be reset (total_cost, token counts -- since the fork is a new session with copied history but fresh metrics). The design does not specify which fields are copied vs. reset.

**Fix:** Add an explicit field-by-field copy/reset table for `session.fork`.

### A-07: No handler registration wiring documented

**Location:** D4.5 vs. BFF foundation design D2.3 (Dispatcher)

The design shows individual handler functions (`handleSessionResume`, `handleSessionList`, etc.) but does not document where they are registered with the `Dispatcher` (from WU-046). The BFF foundation design shows handler registration via `dispatcher.Register(method, handler)`. This bundle should document the registration site (presumably in `Server.Start()` or a setup function) and the method constants used (e.g., `protocol.MethodSessionResume`).

**Fix:** Add a handler registration section showing which method constants map to which handlers, or reference the protocol method constants from WU-039.

### A-08: `Conversation` mutex granularity may cause contention with streaming

**Location:** D3.1 (line 178)

`Conversation` uses a single `sync.RWMutex`. During streaming (WU-053), `AppendAssistantTurn` will take a write lock. Meanwhile, `Messages()` takes a read lock for system prompt assembly (WU-054/055). If streaming and prompt assembly happen on different goroutines for the same session, the write lock during `AppendAssistantTurn` will block `Messages()`. This is likely acceptable for correctness but could introduce latency if the prompt engine reads messages mid-stream. Since `AppendAssistantTurn` is called only at stream completion (not on every delta), contention is low.

**No fix required** -- flagging for implementer awareness. The current design is correct; the note is informational.

### A-09: `TurnDispatcher` holds `providers map[string]provider.Provider` but provider lifecycle is managed elsewhere

**Location:** D4.1

`TurnDispatcher` takes a static `map[string]provider.Provider` at construction time. WU-057 (provider endpoints) manages provider health and discovery, which means the provider map may change at runtime (new Ollama models discovered, endpoints going unavailable). The dispatcher will not see these changes unless the map is replaced or shared via a pointer/interface.

**Fix:** Document that WU-057 is responsible for providing a thread-safe provider registry that the dispatcher queries at dispatch time, rather than a static map. This may require changing `TurnDispatcher` to accept a `ProviderRegistry` interface instead of a bare map.

### A-10: `handleTurnSubmit` missing `turn_id` generation for implicit session creation

**Location:** D4.5 (line 382-383)

The handler calls `ValidateTurnSubmit(params)` which returns a `*protocol.TurnSubmit`. The `TurnSubmit` type (per protocol WU-039) has a `TurnID` field that is client-generated. However, during implicit session creation (step 2), the design does not validate that `submit.TurnID` is non-empty. FEAT-0008 says `turn_id` is a "client-generated UUID, idempotency key" -- if the client omits it, the server should reject with `CodeInvalidParams`.

**Fix:** Add `turn_id` presence validation to `ValidateTurnSubmit` (or note that WU-046 transport validation already covers this).

---

## Nits

### N-01: `AssistantResponse.Cost` field name

**Location:** D3.3 (line 233)

`AssistantResponse` has both `Cost float64` and per-token counts. The field name `Cost` is ambiguous -- is it per-turn cost or cumulative? Suggest `TurnCost` for clarity. Minor naming concern.

### N-02: Missing `session.details` handler method constant

**Location:** D2.5

The handler function name is `handleSessionDetails` but FEAT-0008 protocol table lists the method as `session.details`. The design should reference the protocol constant (e.g., `protocol.MethodSessionDetails`) for grep-traceability.

### N-03: `DispatchOpts.WindowSize` naming vs. FEAT-0008

**Location:** D4.2

FEAT-0008 uses `context_window` consistently. The design uses `WindowSize`. The provider design uses `WindowSize` too, so this is internally consistent, but diverges from the spec vocabulary. Minor.

### N-04: `handleTurnSubmit` step 9 starts goroutine without error handling path

**Location:** D4.5 (line 410)

`go conn.server.streamRelay(...)` is fire-and-forget. If `streamRelay` panics or encounters an unrecoverable error, there is no path to notify the connection. WU-053 owns the streaming relay, but this design should note that `streamRelay` must send a `turn.complete` or `error` event in all exit paths.

### N-05: `Conversation.PendingToolCalls()` return type

**Location:** D3.4

Returns `[]provider.ToolCall` but the protocol event `tool.call` (WU-040) uses `protocol.ToolCall` (which is a different type from `provider.ToolCall`). The conversion between the two is handled by `convert.go` (WU-042), but the design does not mention the conversion point for pending tool calls.

---

## Summary

| Severity | Count |
|----------|-------|
| Blocking | 3 |
| Attention | 10 |
| Nit | 5 |

The design is well-structured and covers the session/conversation/dispatch pipeline thoroughly. The three blocking issues are:

1. **B-01** -- The canonical JSON schema for `turns.content` is unspecified, creating a gap between what `messageToTurn` writes and what `turnToMessage` reads. This is a data integrity issue.
2. **B-02** -- `session.resume` returns the wrong protocol type (`SessionDetail` vs. `SessionResumeResponse`). Wire contract mismatch with the protocol types design.
3. **B-03** -- `session.sync` is not handled or explicitly deferred, leaving a gap in the reconnection flow that downstream WU-064 cannot fill without `SessionManager` internals.

Attention items are mostly about missing field specifications, undocumented assumptions about downstream integration points (provider capabilities, provider registry lifecycle, command history timestamps), and the `session.fork` field copy/reset ambiguity. All are fixable during implementation but should be clarified in the design to prevent divergent implementations.
