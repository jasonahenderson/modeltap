# Phase 2 Design Review: v0.2.2 WU-103 through WU-107

**Reviewer:** kimi-k2.6 (cloud)  
**Date:** 2026-04-27  
**Scope:** `plan.md`, `status.md`, `WU-103`, `WU-104/105/106` (bundled), `WU-107`  
**Baseline:** `internal/harness/*.go`, `internal/harnesshost/*.go`, `internal/harnessshell/*.go`, `internal/cli/root_test.go`

---

## Summary

The v0.2.2 designs are well-scoped and correctly identify the salvage-vs-delete boundary. The audit (WU-103) is the strongest document — it is concrete, file-level, and maps cleanly to the WU-099 `Runtime` interface. The production-wiring bundle (WU-104–106) is directionally sound but contains underspecified integration points that risk expanding WU-104's scope during implementation, particularly around permission gating refactor, the deferred-sender lifecycle, and `DispatchCommand` event surfacing. WU-107 is a clean, small design with one conceptual inconsistency about `View()` purity.

---

## Findings

### 1. WU-103 audit omits exact `messages.go` keep list
- **Severity:** significant
- **Location:** `WU-103` §Refactor — `messages.go`
- **What's wrong:** The audit says "Keep the runtime-event types the projection layer consumes" and "Delete the App-only msg types," but it does not enumerate the exact set that must survive. `internal/harnesshost/projection.go` currently imports `harness.StreamTokenMsg`, `StreamCompleteMsg`, `TurnSubmittedMsg`, `StatusUpdateMsg`, `BranchStartedMsg`, `BranchCompleteMsg`, `BranchErrorMsg`, `ToolActivityMsg`, `PermissionPromptMsg`, `ConnStateMsg`, `ModelUpdateMsg`, `ContextUpdateMsg`, and `CostUpdateMsg` — 13 types. `BannerMsg`, `BannerClearMsg`, `ModeChangeMsg`, `PasteDetectedMsg`, `PasteResolvedMsg`, `PasteSummarizeRequestMsg`, `historyLoadedMsg`, `TickMsg`, and `SubmitMsg` are App-only. If WU-106 trims too aggressively, or if a refactor accidentally moves a needed type into the delete pile, `projection.go` breaks with a compile error.
- **Suggested fix:** Add an explicit enumerated keep list to WU-103 (and WU-106) naming every message type that must survive, so the mechanical cleanup doesn't become a guessing game.

### 2. WU-104 `ResolvePermission` misidentifies the integration point
- **Severity:** significant
- **Location:** `WU-104` §`ResolvePermission`; `WU-104` §Method-by-method mapping
- **What's wrong:** The design says `ProductionRuntime.ResolvePermission` calls `ToolDispatcher.ResolvePermission(requestID, policyDecision)`. But `internal/harness/tool_dispatcher.go` does **not** have a `ResolvePermission` method. The existing permission gate works via a `PromptCallback` registered on `tools.PermissionEnforcer`. The `PermissionHandler` in `permission_prompt.go` (Delete) implements this callback: it blocks the tool-dispatch goroutine on a channel, sends a `PermissionRequestMsg` through `ProgramSender`, and waits for `HandleKey` to write back to the channel. In the new architecture, `ProductionRuntime` must supply its own callback that emits `PermissionPromptMsg` (projected by the adapter) and blocks on a channel keyed by `ToolCallID`. `ResolvePermission` then writes to that channel — it is a `ProductionRuntime` concern, not a `ToolDispatcher` method. The design's description will send an implementer looking for a dispatcher method that does not exist.
- **Suggested fix:** Replace the paragraph with the actual architecture: `ProductionRuntime` constructs the `PermissionEnforcer`, registers a custom `PromptCallback` that emits `PermissionPromptMsg` and blocks on a per-`ToolCallID` channel, and implements `ResolvePermission` by writing the decision into that channel. Note that this requires a `sync.Map` or similar map-of-channels inside `ProductionRuntime`.

### 3. WU-104 `DispatchCommand` event-surfacing mechanism is unspecified
- **Severity:** significant
- **Location:** `WU-104` §`DispatchCommand`; `WU-104` §Method-by-method mapping table
- **What's wrong:** The `Runtime` interface defines `DispatchCommand(ctx, cmd HostCommand) error`. But the WU-104 table shows every command producing a `HostStatusEvent` (or transcript event) back to the shell. An `error` return cannot carry a status event. The design says "the runtime wraps the result in a `HostStatusEvent`" but does not explain the wrapping mechanism. Does `ProductionRuntime` hold a reference to the `tea.Program` sender and call `Send(HostStatusEvent{...})` directly? Does the adapter inspect the error and synthesize the event? If the runtime sends directly, it bypasses the adapter's pause buffer and correlation logic. If the adapter inspects the error, it needs typed errors or side channels.
- **Suggested fix:** Define the mechanism explicitly. Recommended: `ProductionRuntime` owns a `ProgramSender` reference (set via `AttachProgram`) and sends `HostStatusEvent` directly for command results. Document that command events are "out-of-band" relative to the action→event cycle because they originate from host-native commands, not from shell actions. Alternatively, change the `Runtime` interface to return a richer type for `DispatchCommand`.

### 4. WU-104 `SubmitTurnSync` may double-notify the event bridge
- **Severity:** significant
- **Location:** `WU-104` §`SubmitTurn`; `WU-104` §Risk paragraph under `SubmitTurn`
- **What's wrong:** The design proposes adding `ConnectionManager.SubmitTurnSync` which registers a channel, calls `Client.SubmitTurn`, waits for the matching `TurnSubmittedMsg`, and returns. But the existing event-bridge code in `connection.go` will *also* emit `TurnSubmittedMsg` to the `ProgramSender` (deferred sender). If `SubmitTurnSync` consumes the message from the bridge, the adapter's `projectRuntimeMessage` will never see it — unless the sync wrapper also re-broadcasts it. If it re-broadcasts, the adapter sees two `TurnSubmittedMsg`s for one submit, causing duplicate `SubmissionAcceptedEvent`s. If it doesn't re-broadcast, the adapter's correlation table and projection layer never record the submission, breaking stream correlation. The design says "the sync wrapper is a new code path, not a refactor" but doesn't address the dual-delivery problem.
- **Suggested fix:** State explicitly that `SubmitTurnSync` intercepts the `TurnSubmittedMsg` from the bridge *before* it reaches the `ProgramSender`, and that `SubmitTurnSync` is responsible for forwarding the message to the `ProgramSender` after it has captured the result. Or, alternatively, use a promise object that the bridge writes to *in addition to* the sender, so both paths receive the message exactly once.

### 5. WU-104 `deferredSender` is undefined
- **Severity:** significant
- **Location:** `WU-104` §Constructor; `WU-104` §`AttachProgram`
- **What's wrong:** `NewProductionRuntime` constructs a `ConnectionManager` with a `deferredSender`, and `AttachProgram` sets it later. There is no `deferredSender` type in the current codebase. The design does not define it. Is it a new wrapper around `ProgramSender` that buffers messages until a program is attached? Is it a channel? If it buffers, what is its capacity and drop policy? If the runtime starts before `AttachProgram` is called, do buffered messages accumulate indefinitely?
- **Suggested fix:** Provide a concrete Go type definition for `deferredSender` (or whatever the implementation names it) and specify its buffering semantics. A simple 256-message buffered channel with a "drop oldest" overflow policy is usually sufficient.

### 6. WU-104 `AttachProgram` + `Start()` ordering risk
- **Severity:** significant
- **Location:** `WU-105` §`runShell` code snippet
- **What's wrong:** The CLI snippet shows:
  ```go
  p := tea.NewProgram(adapter, ...)
  runtime.AttachProgram(p)
  go func() { _ = runtime.Start(ctx) }()
  _, err = p.Run()
  ```
  `runtime.Start` calls `cm.ConnectSync`, which may immediately produce connection events (`ConnStateMsg`). If `AttachProgram` is called *after* `Start` begins (even in the same goroutine due to scheduling), those early events are lost or buffered in the undefined `deferredSender`. The safe order is `AttachProgram` first, then `Start`. But `tea.NewProgram` returns a `*tea.Program` that isn't running until `p.Run()`; attaching before `Run()` is fine, but starting the runtime before `Run()` means events may arrive before the program's message pump is active.
- **Suggested fix:** Reorder the snippet to `runtime.AttachProgram(p)` before `runtime.Start`. Document that `deferredSender` must buffer until the program's pump is ready, or defer `runtime.Start` until `p.Run()` has initialized (e.g., via a custom `tea.Msg` sent in `Init()`).

### 7. WU-104 `InterruptRun` requires new client method and BFF protocol
- **Severity:** significant
- **Location:** `WU-104` §`InterruptRun`; `plan.md` Risk R4
- **What's wrong:** `internal/harness/client.go` has no `SendInterrupt` method. The BFF protocol may not expose `turn.interrupt`. The design correctly flags this as a risk but then proposes a fallback: return "not yet implemented" and surface `RunFailedEvent`. For a user pressing `Esc` twice to stop a run, seeing a `RunFailedEvent` with message "interrupt unsupported" is poor UX — the shell will render it as a failed run rather than a clean stop. The design should specify how the shell distinguishes a true failure from an unsupported-but-intentional stop.
- **Suggested fix:** Recommend that the adapter synthesize a `RunStoppedEvent` (not `RunFailedEvent`) when `InterruptRun` returns "not supported", so the shell's transcript shows "Run stopped (interrupt not yet supported)" instead of a red error. This preserves the FEAT-0014 stop semantics even when the BFF doesn't support interrupt.

### 8. WU-105 bare `modeltap` behavior change breaks existing test
- **Severity:** significant
- **Location:** `WU-105` §Bare `modeltap` behavior; `internal/cli/root_test.go`
- **What's wrong:** The design says WU-105 "re-introduces `runShell` as the bare-`modeltap` `RunE`". `internal/cli/root_test.go` currently asserts that bare `modeltap` falls back to help ("since the legacy harness CLI was scrapped in v0.2.1"). Changing the root command's `RunE` to launch the shell will break that test. The test also verifies that `shell-demo` is a registered subcommand. Adding `shell` as a subcommand is fine; changing the root `RunE` is a behavior change that needs test updates.
- **Suggested fix:** In WU-105, add a checklist item: update `internal/cli/root_test.go` to expect the new bare-command behavior, or remove the assertion that bare `modeltap` shows help. If the bare-command behavior is conditional (e.g., only when a TTY is detected), document the condition.

### 9. WU-104 test plan for Layer 3 integration tests is optimistic
- **Severity:** significant
- **Location:** `WU-104` §Test coverage
- **What's wrong:** The design calls for Layer 3 integration tests that construct `ProductionRuntime` against "in-memory fake `ConnectionManager`, `ToolDispatcher`, and `ContextManager`". There are no such fakes in the current codebase. `app_test.go` has a fake `ConnSurface` but faking the full `ConnectionManager` (with its reconnect loop, heartbeat, auto-start, and event bridge) is a major undertaking — arguably a WU in itself. If these fakes don't exist, WU-104's test coverage will be blocked or will have to skip Layer 3.
- **Suggested fix:** Either (a) scope Layer 3 tests to a thin end-to-end check using the real `ConnectionManager` against a test BFF stub (e.g., a local net.Listener that speaks JSON-RPC), or (b) create a `testutil` fake package as a prerequisite and document it as part of WU-104's scaffolding phase. Do not assume the fakes exist.

### 10. WU-104 `ModeReader` state holder wiring is incomplete in constructor sketch
- **Severity:** significant
- **Location:** `WU-104` §State holder for `ModeReader`; `WU-104` §Constructor pseudocode
- **What's wrong:** The design shows `runtimeState` implementing `ModeReader` and says `ProductionRuntime` owns one. But `NewProductionRuntime`'s pseudocode says "`harness.NewToolDispatcher(executor, sender, plan, mode)`" — it passes `mode` but doesn't show where `mode` comes from. If `mode` is the `runtimeState`, the constructor must construct it before the dispatcher. The pseudocode is missing this line. More importantly, `plan` (the `PlanAccumulator`) is also passed to `NewToolDispatcher`; the audit categorizes `plan.go` as Refactor. Does `ProductionRuntime` construct a `PlanAccumulator`? The pseudocode doesn't show it.
- **Suggested fix:** Update the constructor pseudocode to show the exact construction order: `runtimeState` → `PlanAccumulator` (if needed) → `ToolDispatcher` with `runtimeState` as the `ModeReader` argument.

### 11. WU-106 `model.go` split doesn't account for `DisplayMessage`
- **Severity:** advisory
- **Location:** `WU-103` §Refactor — `model.go`; `WU-106` §Applies the refactor column
- **What's wrong:** The audit says split `model.go` into `types.go` (keep `ConnStateInfo`, `TokenInfo`, etc.) and delete the rest (`AppState`, `FocusZone`). `DisplayMessage` is also in `model.go`. It is used only by `viewport.go` and `app.go` (both Delete), but it is a large struct. If any Keep or Refactor file references it, the split must preserve it. A quick check shows no Keep/Refactor files import `DisplayMessage`, but the audit should state this explicitly rather than leaving it implicit.
- **Suggested fix:** Add `DisplayMessage` to the explicit delete list in WU-106, with rationale: "Used only by deleted App/viewport components."

### 12. WU-106 deletion of `theme/` and `styles/` is safe but unverified in docs
- **Severity:** advisory
- **Location:** `WU-103` §Delete; `WU-106` §Deletes
- **What's wrong:** The audit categorizes `theme/` and `styles/` as Delete. A grep confirms only App-only files (`app.go`, `input.go`, `viewport.go`, `statusbar.go`, `app_test.go`) import these subpackages. No Keep or Refactor file imports them. This is good, but the audit doesn't cite the verification method, so a future editor might doubt it.
- **Suggested fix:** Add a one-line verification note: "Verified by grep: no Keep or Refactor file imports `internal/harness/theme` or `internal/harness/styles`."

### 13. WU-107 `ViewportState` cache breaks `View()` purity claim
- **Severity:** advisory
- **Location:** `WU-107` §Implementation note
- **What's wrong:** The design claims "Preserve the existing rule that `Model.View` is pure: View must not mutate `Model` state observable to the caller." It then proposes that `View()` mutate a pointer field inside `Model.state` to cache the viewport snapshot. This is technically a side effect — `View()` mutates heap-allocated state that `ViewportState()` later reads. While acceptable in practice (Bubble Tea value receivers with pointer fields are common), the claim of purity is inaccurate and could mislead future maintainers.
- **Suggested fix:** Replace the purity claim with an explicit acknowledgment: "View mutates a lazy-allocated internal cache through a pointer field. This is intentional: the cache is not part of the shell's semantic state, only a snapshot of the last rendered frame, and it is only observable through the read-only `ViewportState()` accessor."

### 14. WU-107 test requires `package harnessshell` (not `_test`)
- **Severity:** advisory
- **Location:** `WU-107` §Test: SC3 parity assertion
- **What's wrong:** The test accesses `m.state.transcriptItems`, `m.state.focus`, and `m.ViewportState()`. `state` is an unexported field. This means the test file must be in `package harnessshell`, not `package harnessshell_test`. The design doesn't mention this constraint, which matters for test-package conventions.
- **Suggested fix:** Add a note: "The test lives in `package harnessshell` because it reads unexported `state` fields. If the project prefers `*_test` packages, the test would need exported test helpers or the `ViewportState` accessor would need to expose enough signal for a black-box test."

### 15. WU-105 `--socket` and `--project` flags default behavior is unspecified
- **Severity:** advisory
- **Location:** `WU-105` §Flags
- **What's wrong:** The design says flags "mirror the legacy harness flags: `--socket`, `--resume`, `--project`, `--model`" but does not state their defaults. The legacy `AppOptions` had `Conn ConnSurface` and `Attacher FileAttacher`. The new CLI needs to resolve socket path and project root from config or flags. If `--socket` defaults to the viper config value and `--project` defaults to `cwd`, that should be explicit.
- **Suggested fix:** Add a flag defaults table: `--socket` → viper `socket_path`, `--project` → `cwd`, `--model` → viper `default_model`, `--resume` → empty (new session).

### 16. Plan says adapter never imports `internal/harness` — false
- **Severity:** advisory
- **Location:** `plan.md` §Architecture overview; `WU-104` §Architecture overview
- **What's wrong:** The plan's architecture box says "The CLI entrypoint and the adapter never import `internal/harness`; the boundary stays clean." But `internal/harnesshost/projection.go` already imports `internal/harness` (for `harness.StreamTokenMsg`, `harness.ConnStateMsg`, etc.). This statement is factually incorrect and will confuse reviewers.
- **Suggested fix:** Correct the statement: "The CLI entrypoint and the `ProductionRuntime` impl are the only packages that import `internal/harness`; the `harnesshost.Adapter` itself does not import `internal/harness` directly — it imports it only via `projection.go` for the runtime-message → HostEvent translation layer."

### 17. WU-104 `SummarizePaste` uses `ContentTransform` but `content.transform` may be BFF-only
- **Severity:** advisory
- **Location:** `WU-104` §`SummarizePaste`
- **What's wrong:** The design says `SummarizePaste` calls `Client.ContentTransform(ctx, raw, "summarize")`. `app_conn.go` lists `ContentTransform` in `ConnProtocolClient`. But this is a BFF RPC method. If the BFF doesn't implement `content.transform` for "summarize", the runtime returns an RPC error. The design says "passthrough is acceptable" but doesn't specify the fallback.
- **Suggested fix:** Document the fallback: if `ContentTransform` returns an RPC error, `SummarizePaste` returns `(raw, nil)` so the shell falls back to its built-in paste summary.

### 18. WU-104 MCP lazy-start design is good but not reflected in constructor
- **Severity:** advisory
- **Location:** `WU-104` §Constructor; `plan.md` Risk R5
- **What's wrong:** The plan's risk register says MCP should be lazy-started. But the constructor pseudocode says "`harness.NewMCPManager(...)`" inside `NewProductionRuntime`, which implies eager construction. Lazy-start means the manager is constructed but its processes aren't spawned until the first `/mcp` command or MCP tool call. The design doesn't show how to achieve this.
- **Suggested fix:** In the constructor pseudocode, add a comment: "`NewMCPManager` is constructed but not started here; `ProductionRuntime.Start` optionally triggers `mcp.Launch()` only if the config enables MCP auto-start, or it is deferred until `DispatchCommand("/mcp ...")`."

---

## Cross-Cutting Concerns

### Permission gating architecture change
The most consequential implementation work hidden inside WU-104 is not any single `Runtime` method but the refactor of the permission gating flow. The existing code uses a synchronous callback (`PromptCallback`) that blocks the tool-dispatch goroutine and emits `PermissionRequestMsg` through `ProgramSender`. The new architecture must replace this with:
1. A callback registered by `ProductionRuntime` that emits `PermissionPromptMsg` and blocks on a per-`ToolCallID` channel.
2. `ResolvePermission` writing to that channel.
3. The adapter projecting `PermissionPromptMsg` → `PermissionRequestedEvent` and managing the pause buffer.

This is a three-package coordination problem (`harnesshost`, `harness/tools`, `harnessshell`) and the design's misleading description ("call `ToolDispatcher.ResolvePermission`") could send an implementer down the wrong path for hours.

### `SubmitTurnSync` double-delivery
The second most consequential issue is the async/sync bridge for `SubmitTurn`. The existing `ConnectionManager` event bridge sends `TurnSubmittedMsg` to `ProgramSender`. If `SubmitTurnSync` adds a parallel channel-based consumer, both consumers must agree on who owns the message. If `SubmitTurnSync` intercepts and consumes it, the adapter never sees the correlation message. If both see it, the adapter gets a duplicate. The design must resolve this explicitly before Phase 3.

### Test coverage realism
The Layer 3 integration test plan is optimistic about fake `ConnectionManager` availability. Given that `ConnectionManager` has complex lifecycle behavior (auto-start, reconnect, heartbeat), building a faithful fake is likely larger than WU-104's "Layer 3" test paragraph suggests. Recommend scoping Layer 3 to a real `net.Listener` stub BFF instead.

---

## Verdict

The designs are **conditionally ready for Phase 3** after disposition of findings #1, #2, #3, #4, and #7. Findings #2 and #4 are the most likely to cause implementation thrash if left unaddressed. Advisory findings should be dispositioned at the implementer's discretion but are low-cost to fix.

## Disposition

| Finding | Severity | Status | Disposition |
| --- | --- | --- | --- |
| 1 — `messages.go` keep list omitted | significant | accepted | Audit + WU-106 cleanup pseudocode now enumerate the 13 runtime-event types that must survive (`StreamTokenMsg`, `StreamCompleteMsg`, `TurnSubmittedMsg`, `StatusUpdateMsg`, `BranchStartedMsg`, `BranchCompleteMsg`, `BranchErrorMsg`, `ToolActivityMsg`, `PermissionPromptMsg`, `ConnStateMsg`, `ModelUpdateMsg`, `ContextUpdateMsg`, `CostUpdateMsg`) and the 9 App-only types that must be removed (`SubmitMsg`, `BannerMsg`, `BannerClearMsg`, `ModeChangeMsg`, `PasteDetectedMsg`, `PasteResolvedMsg`, `PasteSummarizeRequestMsg`, `historyLoadedMsg`, `TickMsg`). |
| 2 — `ResolvePermission` integration point | significant | accepted | Authoritative architecture (supersedes Codex #2): `ProductionRuntime` owns a `sync.Map[string]chan PermissionDecision` keyed by `ToolCallID`. The runtime registers a `PromptCallback` on `tools.PermissionEnforcer` that creates a per-call channel, emits `harness.PermissionPromptMsg`, and blocks on the channel. `ResolvePermission` writes into the matching channel. The adapter projects `PermissionPromptMsg` → `PermissionRequestedEvent` and the existing pause buffer handles mid-stream pause/replay. Production-wiring design rewrites the `ResolvePermission` section accordingly. |
| 3 — `DispatchCommand` event-surfacing mechanism | significant | accepted | `ProductionRuntime` owns a `ProgramSender` reference (set via `AttachProgram`) and emits `HostStatusEvent` directly via `sender.Send` for command results. Documented as out-of-band relative to the action→event cycle because command results originate from host-native commands, not shell-emitted actions. Production-wiring design updated. |
| 4 — `SubmitTurnSync` may double-notify | significant | accepted | Promise pattern: the existing event bridge in `connection.go` continues to send `TurnSubmittedMsg` to `ProgramSender` AND, in addition, writes the result to a per-correlation-ID promise channel that `SubmitTurnSync` waits on. Both consumers receive the message exactly once: the adapter via the bridge → projection layer, and `SubmitTurnSync`'s caller via the promise. Production-wiring design's `SubmitTurn` section rewrites the bridge integration. |
| 5 — `deferredSender` undefined | significant | accepted | Concrete Go type: `harnesshost.deferredSender` wraps `atomic.Pointer[tea.Program]`. `Send` no-ops when the program isn't attached; once attached, every subsequent `Send` forwards to `tea.Program.Send`. No buffer needed for the early-message case because connection events arrive after `runtime.AttachProgram(p)` per the new ordering (Kimi #6). Production-wiring design updated. |
| 6 — `AttachProgram` + `Start` ordering risk | significant | accepted | New documented ordering in WU-105: `tea.NewProgram(adapter)` → `runtime.AttachProgram(p)` → `go runtime.Start(ctx)` → `p.Run()`. `AttachProgram` runs synchronously before any `Start` work spawns. Production-wiring design's CLI snippet updated. |
| 7 — `InterruptRun` poor-UX fallback | significant | accepted (with revision) | Codex #4 supersedes the "new method" concern — `Runtime.InterruptRun` uses the existing `ProtocolClient.CancelTurn`. The fallback recommendation is adopted: when `CancelTurn` returns an unsupported-feature or transport error, the runtime synthesizes `harnessshell.RunStoppedEvent{Reason: StopReasonInterrupt, Message: "stopped — backend reported: <err>"}` rather than `RunFailedEvent`. Preserves FEAT-0014 stop UX. Production-wiring design updated. |
| 8 — bare `modeltap` test breakage | significant | moot | Codex #6 dispositioned: bare `modeltap` does NOT change to launch shell. `internal/cli/root_test.go`'s help-fallback assertion stays valid. No test changes needed. |
| 9 — Layer 3 fake unrealistic | significant | accepted | Layer 3 integration scope changes from "in-memory fake `ConnectionManager`" to "real `ConnectionManager` against a `net.Listener`-backed test BFF stub that speaks the JSON-RPC subset the integration test exercises." A `testutil` BFF stub becomes part of WU-104a's scaffolding (small — just enough to ack `turn.submit` and feed back a synthetic `StreamTokenMsg`/`StreamCompleteMsg` pair). Production-wiring design's "Test coverage" section updated. |
| 10 — `ModeReader` constructor wiring incomplete | significant | accepted | Constructor pseudocode updated to show the explicit construction order: `runtimeState` (implements `ModeReader`) → `PlanAccumulator` → `tools.FileTracker`/`Registry`/`PermissionEnforcer`/`Executor` → `harness.NewToolDispatcher(executor, sender, plan, modeReader)` → `harness.NewConnectionManager(connCfg, deferredSender)` → `harness.NewContextManager(projectRoot, tracker)` → `harness.NewMCPManager(...)` (deferred-start per K18). |
| 11 — `model.go` split: `DisplayMessage` | advisory | accepted | Audit's delete column for `model.go` now explicitly lists `DisplayMessage`. Verified: no Keep or Refactor file references it (used only by deleted `viewport.go` / `app.go`). |
| 12 — theme/styles deletion verification | advisory | accepted | Audit's delete column adds the verification note: "Confirmed by grep against the post-Stage-E tree: only `app.go`, `input.go`, `viewport.go`, `statusbar.go`, `app_test.go` import `internal/harness/theme` or `internal/harness/styles` — all in the delete column." |
| 13 — ViewportState purity claim | advisory | accepted | Subsumed by the Codex #5 fix: viewport-state-accessor design now reads "View intentionally mutates a lazy-allocated internal cache through a pointer field. The cache is not part of the shell's semantic state — only a snapshot of the last rendered frame — and is only observable through the read-only `ViewportState()` accessor." |
| 14 — test-package convention | advisory | accepted | Viewport-state-accessor design adds the note: "The test lives in `package harnessshell` because it reads unexported `state` fields. If a future external test needs the same coverage without internal access, the `ViewportState` accessor already exposes enough signal for a black-box test." |
| 15 — `--socket` / `--project` flag defaults | advisory | accepted | Production-wiring design's WU-105 section adds a flag-defaults table: `--socket` → viper `bff.socket_path` (then `config.DefaultBFFSocketPath()`), `--project` → `os.Getwd()`, `--model` → viper `default_model`, `--resume` → empty (new session). |
| 16 — adapter-imports-internal/harness statement | advisory | accepted | Plan + production-wiring architecture overview corrected: "The CLI entrypoint and the `ProductionRuntime` impl are the packages that import `internal/harness` directly. The `harnesshost.Adapter` itself imports `internal/harness` only via `projection.go` for the runtime-message → HostEvent translation layer; the action-consumer half of `Adapter.Update` does not." |
| 17 — `SummarizePaste` fallback | advisory | accepted | Production-wiring design's `SummarizePaste` section adds: "If `Client.ContentTransform` returns an RPC error (method-not-implemented, transport, server-error), `SummarizePaste` returns `(raw, nil)` so the shell falls back to its built-in paste summary." |
| 18 — MCP lazy-start in constructor | advisory | accepted | Constructor pseudocode adds the comment: "`harness.NewMCPManager(...)` is constructed but the manager's processes are NOT spawned here. `ProductionRuntime.Start` may opt-in to early MCP launch when `cfg.MCPAutoStart` is true; otherwise MCP lifts on the first `DispatchCommand(\"/mcp ...\")` or first MCP-namespaced tool call." |
