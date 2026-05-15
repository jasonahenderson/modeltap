# WU-103: `internal/harness` Audit and Salvage Report

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Purpose

This document is the WU-103 deliverable. It walks every file in
`internal/harness/` (and its subpackages) and categorizes each as
**keep**, **refactor**, or **delete** for the post-v0.2.1 production
wiring work. Each entry lists the file's role, the
`harnesshost.Runtime` method it serves (or doesn't), and the
rationale for the categorization.

The audit is informed by:

- the `harnesshost.Runtime` contract from
  [WU-099](2026-04-25-design-host-adapter-integration-099.md)
  (six methods: `SubmitTurn`, `InterruptRun`, `DispatchCommand`,
  `ResolvePermission`, `LoadPreview`, `SummarizePaste`)
- WU-100 §"Definite scope rule for the reusable package" (sidebar,
  command palette, agent overlays, etc. are out of scope)
- the v0.2.1 deletion of the legacy `modeltap harness` CLI command
  and its App wiring (`internal/cli/harness.go`)
- the user's report that the App was so broken that the underlying
  plumbing's correctness is unknown until proven by a working
  `Runtime` impl

The audit deliberately errs on the side of **keep** for plumbing
files that compile and have unit tests, and **delete** for files
whose sole purpose was driving the deleted App.

## Summary

| Category | Count | Files |
|---|---|---|
| **Keep** | 11 | Pure plumbing / Runtime-impl building blocks |
| **Refactor** | 8 | Mixed plumbing + App-coupling; need reshaping |
| **Delete** | 14 + subpackages | App-only chrome unreachable post-v0.2.1 |

## Per-file categorization

### Keep — pure plumbing, no App coupling

These compile against tea but only via the narrow `ProgramSender` /
`tea.Cmd` patterns; the new production wiring uses them as-is.

| File | Role | Runtime method served |
|------|------|-----------------------|
| `app_conn.go` | `ConnSurface` + `ConnProtocolClient` interfaces (the narrow App-facing view of `ConnectionManager`/`ProtocolClient`). The new production `Runtime` impl wraps `ConnSurface` directly. | All Runtime methods that hit the BFF |
| `client.go` | `ProtocolClient` — JSON-RPC 2.0 client over unix socket / TLS. Pure protocol; no App imports. | `SubmitTurn`, `InterruptRun`, `LoadPreview` (via `content.transform`-ish RPCs), `SummarizePaste` (via `content.transform`) |
| `connection.go` | `ConnectionManager` — connection lifecycle, auto-start, reconnect, event bridge. Uses `ProgramSender` for outbound msgs. | `SubmitTurn` (delegates to `Client()`), `InterruptRun` |
| `tool_dispatcher.go` | Tool execution coordinator, plan-mode interception, permission gating. Depends only on the narrow `ModeReader` interface (any state holder can implement). | `SubmitTurn` (tool call branch), `ResolvePermission` (gate), `DispatchCommand` |
| `context.go` | `ContextManager` / `FileAttacher` — `@file` resolution into `protocol.Attachment`. Uses `tools/` Read/Glob. | `SubmitTurn` (attachment resolution), `LoadPreview` (file content fetch) |
| `mcp.go` | MCP server orchestrator (start, retry, registration). Uses `tools/` registry. | `DispatchCommand` (`/mcp` and tool-call routing); ambient |
| `mcp_client.go` | MCP stdio JSON-RPC client. Pure protocol. | `DispatchCommand` (transitively) |
| `mcp_tool.go` | MCP-tool→`tools.Tool` adapter. | `DispatchCommand`, `ResolvePermission` |
| `tools/` (subpackage) | Built-in tool implementations (Read/Write/Edit/Bash/Git/Glob/Grep/WebFetch/WebSearch + Registry + Executor + PermissionEnforcer + FileTracker). Pure go; no tea imports. | `SubmitTurn` (tool call branch), `ResolvePermission`, `LoadPreview` |
| `messages.go` | `tea.Msg` types (`StreamTokenMsg`, `StreamCompleteMsg`, `TurnSubmittedMsg`, `StatusUpdateMsg`, etc.). Already consumed by `internal/harnesshost/projection.go`. **Some types in this file are App-only and should be trimmed during refactor (see below)** but the runtime-event types stay. | All Runtime methods (the projection layer) |
| `app_conn_test.go`, `client_test.go`, `connection_test.go`, `context_test.go`, `tool_dispatcher_test.go`, `mcp_test.go`, `mcp_client_test.go` | Unit tests for the kept files. All currently pass. | — |

### Refactor — mixed plumbing + App coupling

These contain reusable runtime/protocol logic but currently emit
App-shaped `tea.Cmd` / `tea.Msg` values into the App's update loop.
The new production wiring needs them to expose Go-native methods that
the `Runtime` impl wraps in `tea.Cmd` itself.

| File | Role | Refactor task |
|------|------|---------------|
| `model.go` | `AppState` struct + supporting types. **Keep (move to `types.go`):** `ConnStateInfo`, `TokenInfo`, plus any other type referenced by `internal/harnesshost/projection.go` or by the kept plumbing. **Delete:** `AppState`, `FocusZone`, `DisplayMessage` (large struct used only by the deleted `viewport.go`/`app.go`; verified by grep — no Keep or Refactor file imports it). | Split per the lists above. |
| `messages.go` | Mixed. **Explicit keep list (13 runtime-event types — every type imported by `internal/harnesshost/projection.go`):** `StreamTokenMsg`, `StreamCompleteMsg`, `TurnSubmittedMsg`, `StatusUpdateMsg`, `BranchStartedMsg`, `BranchCompleteMsg`, `BranchErrorMsg`, `ToolActivityMsg`, `ToolActivityPhase` (the typed string + its constants), `PermissionPromptMsg`, `ConnStateMsg`, `ModelUpdateMsg`, `ContextUpdateMsg`, `CostUpdateMsg`. **Explicit delete list (9 App-internal types):** `SubmitMsg`, `BannerMsg`, `BannerClearMsg`, `ModeChangeMsg`, `PasteDetectedMsg`, `PasteResolvedMsg`, `PasteSummarizeRequestMsg`, `historyLoadedMsg`, `TickMsg`. The split is mechanical: any type referenced from `internal/harnesshost/projection.go` survives; everything else goes. | Apply the keep/delete lists above. |
| `compact.go` | `/compact` slash-command handler. Wraps `protocol.CompactPlan` / `CompactApplyResponse` round-trips and emits `CompactPlanLoadedMsg` / `CompactAppliedMsg` for the App to render. | Keep the RPC-call helpers; replace tea.Cmd return signature with sync return; the new Runtime's `DispatchCommand("/compact")` wraps them. |
| `history.go` | `/history` slash-command handler with command-history cache. Pages through BFF history records. | Same shape: keep the cache and RPC helpers; rewire the tea.Cmd surface so `DispatchCommand("/history")` calls them. |
| `models.go` | `/model` / `/models` slash commands. Loads the model catalog and applies switches. | Same shape — sync helpers + emit results as `HostStatusEvent` or transcript events through the Adapter rather than App-banner messages. |
| `sessions.go` | `/session` / `/sessions` slash commands. Wraps session list/resume/clear/fork RPCs. | Same shape. |
| `plan.go` | `PlanAccumulator` for plan-mode interception. The accumulator itself is reusable; the App-banner emission is the coupling. | Keep the accumulator; the `ModeReader` integration with `tool_dispatcher.go` continues to work because the new state holder implements `ModeReader`. |
| `tool_dispatcher_test.go` (already in keep above) | — | — |

### Delete — App-only chrome unreachable post-v0.2.1

These exist only to drive the deleted App's TUI. The reusable shell
(`internal/harnessshell`) owns the equivalent rendering surfaces;
nothing else imports these files.

| File | Role | Why delete |
|------|------|------------|
| `app.go` | The App `tea.Model`. | The user-visible TUI was reported as broken; v0.2.1 deleted the CLI entry. The new production CLI in WU-105 constructs `harnessshell.Model` + `harnesshost.Adapter` directly. |
| `app_test.go` | App tests. | Tests the deleted App. |
| `input.go` | TUI input area (textarea wrapper, paste detection). | `harnessshell.Model` owns its own composer; FEAT-0014 explicitly. |
| `input_test.go` | Tests for input.go. | — |
| `keys.go` | App keybindings (Quit, Submit, ToggleMode, etc.). | `harnessshell.Model` owns its own key handling. |
| `markdown.go` | TUI markdown wrapper (Glamour). | `harnessshell` renders raw text inline; markdown rendering becomes a v0.2.2+ chrome enhancement at the host level if needed. |
| `markdown_test.go` | Tests for markdown.go. | — |
| `viewport.go` | TUI viewport above the input area. | `harnessshell.Model` owns its own viewport. |
| `viewport_test.go` | Tests for viewport.go. | — |
| `statusbar.go` | TUI status line. | The shell renders its own footer; production CLI can wrap with additional chrome if needed but not via this file. |
| `statusbar_test.go` | Tests for statusbar.go. | — |
| `connux.go` | Connection-state → banner UX translator. Bound to `AppState`, emits `BannerMsg`. | The Adapter's projection layer already maps `ConnStateMsg` → `HostStatusEvent`; no banner translator needed. |
| `connux_test.go` | Tests for connux.go. | — |
| `paste.go` | TUI paste-handler modal overlay (s/f/t/c/Esc disposition). | Paste handling is now shell-local: `harnessshell.handleInputMutation` captures pastes >=120 chars as paste tokens automatically. The TUI modal is replaced by the shell's inline-token model. |
| `paste_test.go` | Tests for paste.go. | — |
| `permission_prompt.go` | TUI permission-prompt modal banner. | Replaced by `harnessshell`'s composer-hosted permission UI + the host adapter's projection of `PermissionPromptMsg`. |
| `theme/` (subpackage) | TUI theme system (light/dark detection, color palette). | `harnessshell/styles.go` is theme-neutral lipgloss; the shell does not import theme. WU-098 §"Theme/style import boundary" forbids it. **Verified by grep against the post-Stage-E tree:** only `app.go`, `input.go`, `viewport.go`, `statusbar.go`, `app_test.go`, and `internal/cli/harness.go` (now-deleted) imported `internal/harness/theme` — all in the delete column. No Keep or Refactor file imports it. |
| `styles/` (subpackage) | TUI lipgloss styles for the App. | Replaced by `harnessshell/styles.go`. **Same grep verification as `theme/`:** only delete-column files reference `internal/harness/styles`. |

## Runtime method → harness service mapping

| Runtime method | Backing services |
|---|---|
| `SubmitTurn(ctx, req SubmitRequest) (SubmitAccepted, error)` | `ContextManager.Resolve` (req.Tokens → protocol.Attachment) → `ConnSurface.SubmitTurn(ctx, payload)` → wait for `TurnSubmittedMsg` (or block via the TurnID-correlation channel that the new Runtime will own). |
| `InterruptRun(ctx, runID string) error` | `ConnSurface.Client().SendInterrupt(ctx, runID)` (provisional — the existing client may not have an explicit interrupt RPC; if not, `client.go` gains one or `Runtime.InterruptRun` returns "not supported" and the user sees a `RunFailedEvent` with a clear message). |
| `DispatchCommand(ctx, cmd HostCommand) error` | Per-command routing: `/model` → `models.go` helpers; `/session` → `sessions.go`; `/context` → `context.go`; `/compact` → `compact.go`; `/history` → `history.go`; `/mcp` → `mcp.go`; `/plan`, `/build`, `/auto` → `plan.go`/`AppState.Mode` setter (which moves to a new state holder). |
| `ResolvePermission(ctx, requestID string, decision PermissionDecision) error` | `tool_dispatcher.go` permission gate calls `tools/PermissionEnforcer` to record the decision and unblock the gated tool call. The decision flows through the dispatcher's existing channel-based wait. |
| `LoadPreview(ctx, req PreviewRequest) (PreviewPayload, error)` | `ContextManager.Resolve` for the path → `tools/Read.Execute` for the content. The Runtime impl converts the tool result into `harnessshell.PreviewPayload`. |
| `SummarizePaste(ctx, raw string) (string, error)` | `ConnSurface.Client().ContentTransform(ctx, raw, "summarize")` — the existing `content.transform` RPC. Pass-through is acceptable in the demo runtime. |

## Recommended WU-104 implementation order

1. Stand up a new state holder type (small struct with `CurrentMode()`,
   `SetMode()`, etc.) so `ModeReader`-coupled files don't need to
   import a deleted `AppState`.
2. Implement `Runtime.SubmitTurn`. This is the foundational call;
   exercises `ContextManager` + `ConnSurface` + correlation. Leaves
   the rest of `Runtime` as `panic("not implemented")` stubs initially.
3. Implement `Runtime.LoadPreview`. Smallest functional next method.
4. Implement `Runtime.ResolvePermission`. Wires the
   `tool_dispatcher.go` permission gate into the new Adapter pause
   buffer.
5. Implement `Runtime.InterruptRun`. May require a new RPC in
   `client.go`.
6. Implement `Runtime.DispatchCommand`. Router that fans out to
   `models.go` / `sessions.go` / `context.go` / `compact.go` /
   `history.go` / `mcp.go` after each is refactored to expose
   sync entry points.
7. Implement `Runtime.SummarizePaste`. Pass-through for now.

## Recommended WU-106 cleanup order

1. Trim `messages.go` (delete App-only msg types).
2. Split `model.go` → `types.go` (minimal shared types) + delete the
   rest.
3. Delete every file in the **delete** column.
4. Delete `theme/` and `styles/` subpackages.
5. Verify `internal/harness/` still builds clean and the projection
   layer in `internal/harnesshost` still imports successfully.

## Risks identified during the audit

- **R1 — interrupt RPC might not exist.** `client.go`'s
  `ConnProtocolClient` interface didn't show an explicit Interrupt
  method during the survey. WU-104's `Runtime.InterruptRun`
  implementation either adds one (server-side change too?) or
  surfaces `RunFailedEvent{Message: "interrupt unsupported"}`.
- **R2 — `compact.go` / `models.go` / `sessions.go` may have
  awkward async shapes.** They emit tea.Cmds today; the refactor to
  sync helpers might surface assumptions the App was hiding.
- **R3 — MCP autostart side effects.** `mcp.go` orchestrates
  external MCP processes. The new Runtime's `Init` (or equivalent)
  must trigger MCP start at the right time; otherwise a
  `DispatchCommand("/mcp status")` returns "no servers" until the
  process loop kicks in.

## Acceptance criteria for WU-103

- ✅ Every file in `internal/harness/` (and subpackages) is in one of
  the three categories.
- ✅ Each Runtime method has at least one backing service identified.
- ✅ The order in which WU-104 should land Runtime methods is
  documented.
- ✅ The order in which WU-106 should delete files is documented.
- ✅ Risks identified during the survey are recorded for WU-104.

WU-103 deliverable is this document. No code changes ship under
WU-103.
