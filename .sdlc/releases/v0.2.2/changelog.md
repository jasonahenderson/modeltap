# v0.2.2 Changelog

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

**Status:** released (tagged on branch `spike/scrolling-surface-eval`)

v0.2.2 lands the production conversation-shell wiring deferred from
v0.2.1. The post-extraction architecture
(`internal/harnessshell` + `internal/harnesshost` +
`internal/harnessdemo`) is now reachable from a real `modeltap shell`
command that connects to the BFF over a unix socket and drives
end-to-end submit / stream / permission / preview flows. The legacy
`internal/harness/` plumbing has been audited, deletely-pruned to the
runtime services that the new `ProductionRuntime` consumes, and
trimmed of the App-coupled tea.Cmd handlers.

For a detailed work-unit-level breakdown see `status.md` and the
per-WU commit messages.

## Headline additions

### `modeltap shell` — production CLI entrypoint

The new `modeltap shell` command launches the production
conversation shell. Constructs `harnessshell.Model` +
`harnesshost.Adapter` + `harnesshost.ProductionRuntime` and runs as
a tea.Program against a real BFF (auto-starting the BFF when the
configured socket is absent). Slash commands route via
`Runtime.DispatchCommand`: `/clear` (shell-local), `/plan` /
`/build` / `/auto` (mode), `/model` / `/models` (catalog), `/session`
/ `/sessions` (lifecycle), `/context` (context window), plus
documented stubs for `/compact` / `/history` / `/mcp` (planned for
follow-up).

Replaces the legacy `modeltap harness` command (deleted in v0.2.1).

### `harnesshost.ProductionRuntime` — modeltap-internal Runtime impl

Concrete `harnesshost.Runtime` implementation backed by the
surviving `internal/harness/` plumbing. Owns the connection
lifecycle (`ConnectionManager`), the JSON-RPC client
(`ProtocolClient`), the tool dispatcher with permission gating, the
context manager for `@file` resolution, and the runtimeState
(session ID, sequence, mode, label) that replaces the deleted
`AppState`.

Method coverage:

- **`SubmitTurn`** — resolves attachments via `ContextManager`
  (file refs) and inlines paste payloads; calls
  `Client.SubmitTurn` synchronously; returns `SubmitAccepted{RunID,
  Label}` with the server-echoed turn ID.
- **`InterruptRun`** — calls the existing `Client.CancelTurn`
  (against `protocol.MethodTurnCancel`); on error or no-live-client,
  synthesizes `RunStoppedEvent{Reason: StopReasonInterrupt}` so the
  shell shows a clean stop instead of a red error.
- **`ResolvePermission`** — bridges the executor's `PromptCallback`
  to the user's decision via a `sync.Map` of per-`ToolCallID`
  channels. Synthesizes `PermissionResolvedEvent` for the adapter's
  pause-buffer drain logic.
- **`LoadPreview`** — validates path via `ContextManager.Resolve`;
  reads content via the registered `tools.Read` tool; returns a
  `PreviewPayload` with title / content / metadata.
- **`DispatchCommand`** — routes `/model`, `/models`, `/session`,
  `/sessions`, `/context`, `/plan`, `/build`, `/auto` via
  ProtocolClient calls and emits `HostStatusEvent` directly through
  the deferredSender (out-of-band relative to the action→event
  cycle).
- **`SummarizePaste`** — calls `content.transform` with
  passthrough-on-error fallback so the shell's built-in summarizer
  takes over when the BFF transform fails.

### Adapter token-attachment table

`harnesshost.Adapter` now maintains a `tokenAttachments` map
(populated on every `dispatchSubmit` from the resolved Attachments)
so `LoadPreviewAction` can populate `PreviewRequest.Path`. Per
Codex #3 in the v0.2.2 Phase 2 review.

### Viewport-state accessor + scroll preservation fix

WU-107 added a read-only `harnessshell.Model.ViewportState()`
accessor (`{YOffset, AtBottom, Width, Height}`) and surfaced a
hidden bug while implementing the SC3 parity test: the original
shell's `Model.View` did SetContent on a local viewport copy that
didn't persist, so mouse-wheel scroll was a no-op and AtBottom
always reported true. Per Prime Directive #4, the design was
revised explicitly: the persistent `state.transcript` viewport now
gets `SetContent` applied via a `state.refresh()` helper called at
the end of every `Model.Update` tick, with proper followTail /
saved-YOffset preservation. View became pure (no projection or
content mutation). FEAT-0014 SC3 ("manual scroll preserved when not
following tail") is now actually implemented and asserted.

### Test-stub BFF for Layer 3 integration

`internal/harnesshost/testutil/bffstub.go` ships a minimal
unix-socket JSON-RPC server speaking `capabilities.register`,
`turn.submit`, `turn.cancel`, and `ping`. Replaces the WU-099-
proposed "in-memory `ConnectionManager` fake" with a real
`net.Listener`-backed BFF (per Kimi #9). Used by
`production_runtime_test.go` to exercise the full
`ConnectionManager → ProtocolClient → unix socket → server` path.

## Removals

- **`internal/harness/app.go`** and the entire TUI App layer —
  ~3,000 lines covering input area, viewport, statusbar, markdown
  wrapper, modal dialogs (paste, permission prompt, compact),
  banner UX, theme system, lipgloss styles. The reusable
  `harnessshell` owns the conversation surface; the production CLI
  in `internal/cli/shell.go` is the new entrypoint.
- **`internal/harness/{compact,history,models,sessions}.go`** —
  the App-coupled slash-command handlers. ProductionRuntime
  inlines the underlying RPC calls in `DispatchCommand`.
- **`internal/harness/permission_prompt.go`** — the modal
  permission-prompt handler. Broker logic moved to
  `ProductionRuntime.permissionPromptCallback` per Kimi #2.
- **App-only `tea.Msg` types** — `SubmitMsg`, `BannerMsg`,
  `BannerClearMsg`, `ModeChangeMsg`, `PasteDetectedMsg`,
  `PasteResolvedMsg`, `PasteSummarizeRequestMsg`, `historyLoadedMsg`,
  `TickMsg`. The 13 runtime-event types the projection layer
  consumes survive verbatim.
- **`AppState`, `FocusZone`, `DisplayMessage`, `RoleUser/Assistant
  /etc.`** — App-internal state types in `model.go`. The kept
  shared types (`ConnStateInfo`, `TokenInfo`, `ConnState*`
  constants) survive in a trimmed `model.go`.
- **Connection event-bridge `BannerMsg` sends** in `connection.go`
  — replaced with `StatusUpdateMsg` so the projection layer routes
  them to `HostStatusEvent`.
- **App-driven integration tests** in `internal/integration/`.
  Replaced by the WU-104 BFF stub-driven Layer 3 tests in
  `internal/harnesshost`.

Net delta of WU-106: 74 files changed, 113 insertions, **14,048
deletions**.

## Test coverage

- **Shell parity (Layer 1)** — viewport-state accessor + SC3
  manual-scroll-preservation parity assertion (WU-107). All FEAT-
  0014 success criteria now have direct automated coverage.
- **Adapter behavior (Layer 2)** — existing pause-buffer,
  projection, and dispatch tests from v0.2.1 plus new
  ProductionRuntime tests covering SubmitTurn (live BFF),
  ResolvePermission unblock, LoadPreview readfile, mode-change
  DispatchCommand, all WU-104b/c stubs.
- **Runtime integration (Layer 3)** — `BFFStub`-backed
  end-to-end SubmitTurn pipeline (`turn.submit` ack, content
  round-trip, server-assigned RunID, sequence increment, session-
  ID persistence across submits).

All tests pass: `internal/harness`, `internal/harness/tools`,
`internal/harnessshell`, `internal/harnesshost`,
`internal/harnessdemo`, `internal/integration`. Pre-existing
`internal/cli` failures (config / dashboard tests need API key env
vars) unchanged from prior baseline; not caused by this release.

## Known gaps

- **`/compact`, `/history`, `/mcp`** are documented stubs that
  return "not yet wired" status events. Full integration scoped
  for v0.2.3.
- **MCP lazy-start** — the constructor builds tool framework state
  but `MCPManager` is not yet constructed; `/mcp` returns the
  stub message. Real MCP wiring is a v0.2.3 enhancement.
- **`InterruptRun` requires the BFF to support `turn.cancel`** —
  if the BFF returns an error, the runtime synthesizes
  `RunStoppedEvent` so the UX is preserved, but the underlying
  cancellation may not fire server-side. Verify the deployed BFF
  implements `protocol.MethodTurnCancel`.
- **ApproveOnce vs ApproveSession** — the existing
  `tools.PermissionEnforcer` collapses both decisions into
  "approved for this session" (calling `Approve(toolName)` after
  any approval). The shell exposes the 3-way choice but the
  production runtime treats `ApproveOnce` and `ApproveSession`
  identically. Distinguishing them at the executor level requires
  a small refactor of `framework.go`.

## Branch

Tagged on `spike/scrolling-surface-eval`. Branch retarget pending
TPM decision; per `.agents/process.md` §"Tag update policy" an
unpublished tag may be moved post-retarget.
