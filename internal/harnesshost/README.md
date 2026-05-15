# `harnesshost` — Modeltap Host Adapter

`harnesshost` is the modeltap-specific glue between the reusable
[`harnessshell`](../harnessshell) conversation component and the modeltap
runtime services (connection manager, protocol client, tool dispatcher,
attachment/context loader, permission enforcer). It is not reusable
outside modeltap and is not intended for promotion into a separate
repository — it is the layer that lets the reusable shell embed into
modeltap without forcing modeltap concerns back into the shell package.

## Status

Feature-complete adapter. The action-consumer half (`Adapter.Update`
intercepting `ActionMsg`) and the event-projection half
(`projectRuntimeMessage` translating `internal/harness` runtime tea.Msgs
into `harnessshell.HostEvent`s) both run end-to-end against the
internal-package fakes. Mid-stream pause buffering per WU-099 §"Mid-stream
Pause" is wired in `Adapter.forwardEvent`. Production wiring against
`internal/harness/app_conn.ConnSurface` (Stage D-4 of v0.2.1's plan)
remains as the only Stage D follow-up.

`internal/harnessspike` was deleted in WU-100 Stage E. The fake/demo
runtime moved to [`internal/harnessdemo`](../harnessdemo); the demo CLI
now runs as `modeltap shell-demo`.

## Role

The adapter has two jobs:

1. **Consume shell actions.** Drain typed actions emitted by the shell
   (`SubmitTurnAction`, `InterruptRunAction`, `RunHostCommandAction`,
   `ResolvePermissionAction`, `LoadPreviewAction`) and route them to the
   appropriate modeltap runtime service.
2. **Produce host events.** Project modeltap connection / runtime / tool
   notifications back into the shell as typed host events
   (`SubmissionAcceptedEvent`, `RunStartedEvent`, `RunDeltaEvent`,
   `RunCompletedEvent`, `RunStoppedEvent`, `RunFailedEvent`,
   `PermissionRequestedEvent`, `PermissionResolvedEvent`,
   `PreviewLoadedEvent`, `HostStatusEvent`).

The adapter is the only package in the repo that imports both
`internal/harnessshell` and the modeltap runtime/protocol/tool packages.

## Runtime Interface

The adapter depends on a narrow, modeltap-internal `Runtime` interface
defined here, **not** the full `ConnProtocolClient` from
`internal/harness/app_conn.go`. The interface deliberately exposes only
what the FEAT-0014 boundary requires:

```go
// Runtime is the modeltap-internal contract that harnesshost depends on.
// It is intentionally narrower than ConnProtocolClient.
type Runtime interface {
    SubmitTurn(ctx context.Context, req SubmitRequest) (SubmitAccepted, error)
    InterruptRun(ctx context.Context, runID string) error
    DispatchCommand(ctx context.Context, cmd HostCommand) error
    ResolvePermission(ctx context.Context, requestID string, decision harnessshell.PermissionDecision) error
    LoadPreview(ctx context.Context, req PreviewRequest) (harnessshell.PreviewPayload, error)
    SummarizePaste(ctx context.Context, raw string) (string, error)
}
```

`ResolvePermission` takes `(ctx, requestID, decision)` because the shell
allows multiple pending permissions to coexist; the request identity is
required for the host to apply the decision to the correct request. This
matches the shell-emitted `ResolvePermissionAction.RequestID` from
WU-098.

A concrete `Runtime` implementation wraps the existing
`internal/harness/app_conn.ConnSurface`, the tool dispatcher, the
context manager, and the permission enforcer. The adapter never calls
those services directly — only through `Runtime`. The fake runtime in
`internal/harnessdemo.FakeRuntime` satisfies the interface for the demo
CLI and integration tests.

The `Runtime` interface intentionally does **not** include
`PauseRun` / `ResumeRun`. Mid-stream permission pause is implemented
inside the adapter as a stream-delta buffering concern; see
"Mid-stream pause" below.

## Action → Operation Mapping

The adapter consumes shell actions and routes each to a runtime
operation or an internal command-routing service.

| Shell action | Adapter operation |
| --- | --- |
| `SubmitTurnAction` | resolve attachments → `Runtime.SubmitTurn` → emit `SubmissionAcceptedEvent` (or `SubmissionFailedEvent`); track shell submission ID → runtime turn/run ID correlation. Source (`SubmissionSourceDirect` vs `SubmissionSourceQueueRelease`) preserved in correlation metadata. |
| `InterruptRunAction` | `Runtime.InterruptRun(ctx, runID)`; on success the runtime is expected to surface a terminal lifecycle event via the projection layer. On failure the adapter emits `RunFailedEvent` so the shell leaves armed-stop deterministically. |
| `RunHostCommandAction` | parse the host-native slash command; dispatch via `Runtime.DispatchCommand` and/or fan out to existing harness command services (model/session/context/compact/MCP/etc.). Failure surfaces as `HostStatusEvent{Kind: StatusError}`. |
| `ResolvePermissionAction` | `Runtime.ResolvePermission(ctx, requestID, decision)`; the buffered `RunDeltaEvent`s drain when the pending permission set empties; on failure the adapter emits `PermissionResolvedEvent{Outcome: OutcomeDenied}` carrying the error message. |
| `LoadPreviewAction` | `Runtime.LoadPreview` (delegates to the existing `ContextManager` for path validation and read); emit `PreviewLoadedEvent`, or `HostStatusEvent{Kind: StatusError}` on failure. |

Shell-native commands (`/clear`, queue release on empty `Enter` while
idle, transcript-local view actions) are **never** observed by the
adapter. They are handled inside the shell's update loop and there is no
`RunShellCommandAction` type. Per WU-098, queue-release is a shell-local
*trigger* whose *effect* still crosses the boundary as a normal
`SubmitTurnAction` with `Source = SubmissionSourceQueueRelease`.

## Runtime Event → Shell Event Mapping

The adapter projects modeltap runtime / connection / tool notifications
into the shell as typed host events. The left column lists the
representative `internal/harness/messages.go` types; the right column
lists the boundary-crossing event delivered into the shell.

| Runtime / harness message | Shell host event |
| --- | --- |
| `TurnSubmittedMsg` (success) | `SubmissionAcceptedEvent` |
| `TurnSubmittedMsg` (Err non-nil) | `SubmissionFailedEvent` |
| `StreamTokenMsg` | `RunDeltaEvent` (gated by mid-stream pause buffer) |
| `StreamCompleteMsg` | `RunCompletedEvent` |
| `StatusUpdateMsg` | `HostStatusEvent{Kind: StatusStreaming}` |
| `BranchStartedMsg` / `BranchCompleteMsg` / `BranchErrorMsg` | flattened into the single-transcript model per FEAT-0014 (each branch projects to a `Run*Event` with `RunID = "<TurnID>:<BranchID>"`) |
| `ToolActivityMsg` | `HostStatusEvent` with phase-aware glyph (⚙ start, ✓/✗/⊘/• end) |
| `PermissionPromptMsg` | `PermissionRequestedEvent` |
| `ConnStateMsg` / `ModelUpdateMsg` / `ContextUpdateMsg` / `CostUpdateMsg` | `HostStatusEvent` with appropriate `StatusKind` |

The adapter holds the correlation table that maps shell submission/run
IDs to runtime turn IDs (and to branch IDs when the server emits parallel
review branches). The shell sees only stable run IDs.

Paste-handler messages (`PasteDetectedMsg`, `PasteResolvedMsg`,
`PasteSummarizeRequestMsg`), banners, ticks, and other host-App-only
runtime messages project to `nil` so they remain owned by the host
program rather than crossing into the reusable shell.

## Mid-stream Pause

`FEAT-0014` requires that a permission request arriving during streaming
pauses the active stream immediately and resumes only after approval. In
the post-extraction architecture this responsibility lives in the
**adapter**, not in the shell and not behind a `Runtime.PauseRun`
method:

- on `PermissionRequestedEvent` while a run is active, the adapter
  registers the request ID in `pendingPermissions` and buffers any
  further `RunDeltaEvent` forwarding internally instead of forwarding
  to the shell
- on `PermissionResolvedEvent` (the last pending one), the adapter
  forwards the resolve event first, then replays buffered deltas in
  arrival order, then resumes live forwarding
- multi-permission case: only when ALL pending permissions resolve does
  the buffer drain. A resolve for one of N pending permissions
  decrements the set but does not drain the buffer
- if the runtime/server itself naturally pauses streaming at the tool
  boundary, the buffer remains empty — but the buffer logic must still
  exist, because nothing in the boundary contract requires server-side
  pausing

The shell stays unaware of streaming-pause mechanics; it simply does not
receive deltas during the pause window.

## Where Demo Behavior Lives

Fake / demo runtime behavior — synthetic stream lifecycle events,
example permission requests, queue/preview exercise without a real
runtime server — lives in [`internal/harnessdemo`](../harnessdemo), **not** in this
package.

The reusable shell has two valid host packages plus inline test fakes:

- `internal/harnesshost` — real modeltap integration (this package).
- `internal/harnessdemo` — examples and fake-runtime CLI capability.
- ad-hoc test fakes constructed inline by shell unit tests.

There is no third "spike" host. The previous `internal/harnessspike`
package was deleted in WU-100 Stage E.

## Related Packages

- [`internal/harnessshell/README.md`](../harnessshell/README.md) — reusable conversation-shell component this adapter drives.
- [`internal/harnessdemo`](../harnessdemo) — fake-runtime adapter for examples and tests.
- [`docs/guides/harness-shell-embedding.md`](../../docs/guides/harness-shell-embedding.md) — canonical embedding guide; includes the full submit / stream / permission / preview integration walkthroughs.
- [`.sdlc/features/0014-harness-conversation-shell.md`](../../.sdlc/features/0014-harness-conversation-shell.md) — behavior contract.
- [`.sdlc/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md`](../../.sdlc/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md) — full host-adapter integration design (WU-099).
