<!--
WU-101 structural pass.
This README uses provisional names from WU-098 and WU-099 designs. Anywhere
this file uses a name that may be renamed during WU-100 implementation, an
HTML comment of the form `<!-- provisional: ... -->` flags it for the
reconciliation pass that follows WU-100 cutover.
-->

# `harnesshost` — Modeltap Host Adapter <!-- provisional: subject to WU-100 reconciliation -->

`harnesshost` is the modeltap-specific glue between the reusable
`harnessshell` conversation component <!-- provisional: subject to WU-100 reconciliation -->
and the modeltap runtime services (connection manager, protocol client,
tool dispatcher, attachment/context loader, permission enforcer). It is
not reusable outside modeltap and is not intended for promotion into a
separate repository — it is the layer that lets the reusable shell embed
into modeltap without forcing modeltap concerns back into the shell
package.

## Status

Stage A skeleton at the time of this writing. The adapter is introduced
during WU-100 Stage 3+ as the consumer of shell-emitted actions and the
producer of shell-bound host events. Until cutover, the spike package
`internal/harnessspike` continues to host the App-shaped composition.
After v0.2.1 release, `harnessspike` is deleted and the spike
fake-runtime capability moves to `internal/harnessdemo`.

## Role

The adapter has two jobs:

1. **Consume shell actions.** Drain typed actions emitted by the shell
   (`SubmitTurnAction`, `InterruptRunAction`, <!-- provisional: subject to WU-100 reconciliation -->
   `RunHostCommandAction`, `ResolvePermissionAction`, <!-- provisional: subject to WU-100 reconciliation -->
   `LoadPreviewAction`) <!-- provisional: subject to WU-100 reconciliation -->
   and route them to the appropriate modeltap runtime service.
2. **Produce host events.** Project modeltap connection / runtime / tool
   notifications back into the shell as typed host events
   (`SubmissionAcceptedEvent`, `RunStartedEvent`, `RunDeltaEvent`, <!-- provisional: subject to WU-100 reconciliation -->
   `RunCompletedEvent`, `RunStoppedEvent`, `RunFailedEvent`, <!-- provisional: subject to WU-100 reconciliation -->
   `PermissionRequestedEvent`, `PermissionResolvedEvent`, <!-- provisional: subject to WU-100 reconciliation -->
   `PreviewLoadedEvent`, `HostStatusEvent`). <!-- provisional: subject to WU-100 reconciliation -->

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
    ResolvePermission(ctx context.Context, requestID string, decision PermissionDecision) error
    LoadPreview(ctx context.Context, req PreviewRequest) (PreviewPayload, error)
    SummarizePaste(ctx context.Context, raw string) (string, error)
}
```

`ResolvePermission` takes `(ctx, requestID, decision)` because the shell
allows multiple pending permissions to coexist; the request identity is
required for the host to apply the decision to the correct request. This
matches the shell-emitted `ResolvePermissionAction.RequestID` from
WU-098. <!-- provisional: subject to WU-100 reconciliation -->

A concrete `Runtime` implementation wraps the existing
`internal/harness/app_conn.ConnSurface`, the tool dispatcher, the
context manager, and the permission enforcer. The adapter never calls
those services directly — only through `Runtime`.

The `Runtime` interface intentionally does **not** include
`PauseRun` / `ResumeRun`. Mid-stream permission pause is implemented
inside the adapter as a stream-delta buffering concern; see
"Mid-stream pause" below.

## Action → Operation Mapping

The adapter consumes shell actions and routes each to a runtime
operation or an internal command-routing service. <!-- provisional: subject to WU-100 reconciliation -->

| Shell action | Adapter operation |
| --- | --- |
| `SubmitTurnAction` | resolve attachments → `Runtime.SubmitTurn` → emit `SubmissionAcceptedEvent` (or `SubmissionFailedEvent`); track shell submission ID → runtime turn/run ID correlation. Source (`direct` vs `queue_release`) preserved in correlation metadata. |
| `InterruptRunAction` | `Runtime.InterruptRun(ctx, runID)`; on success emit `RunStoppedEvent`; on failure emit a terminal lifecycle event so the shell can leave armed-stop deterministically. |
| `RunHostCommandAction` | parse the host-native slash command; dispatch via `Runtime.DispatchCommand` and/or fan out to existing harness command services (model/session/context/compact/MCP/etc.); translate result into a transcript-visible event or a `HostStatusEvent`. |
| `ResolvePermissionAction` | `Runtime.ResolvePermission(ctx, requestID, decision)`; replay any buffered `RunDeltaEvent`s; emit `PermissionResolvedEvent` whose `Message` is constructed from the runtime tool-result payload (or a granted/denied fallback). |
| `LoadPreviewAction` | `Runtime.LoadPreview` (delegates to the existing `ContextManager` for path validation and read); emit `PreviewLoadedEvent` (or a preview-failure event). |

Shell-native commands (`/clear`, queue release on empty `Enter` while
idle, transcript-local view actions) are **never** observed by the
adapter. They are handled inside the shell's update loop and there is no
`RunShellCommandAction` type. Per WU-098, queue-release is a shell-local
*trigger* whose *effect* still crosses the boundary as a normal
`SubmitTurnAction` with `Source = queue_release`. <!-- provisional: subject to WU-100 reconciliation -->

## Runtime Event → Shell Event Mapping

The adapter projects modeltap runtime / connection / tool notifications
into the shell as typed host events. The left column lists representative
existing harness `messages.go` types; the right column lists the
boundary-crossing event delivered into the shell. <!-- provisional: subject to WU-100 reconciliation -->

| Runtime / harness message | Shell host event |
| --- | --- |
| `TurnSubmittedMsg` | `SubmissionAcceptedEvent` (or `SubmissionFailedEvent`) |
| `StreamTokenMsg` | `RunDeltaEvent` (gated by mid-stream pause buffer) |
| `StreamCompleteMsg` | `RunCompletedEvent` |
| `StatusUpdateMsg` (interrupt / stop) | `RunStoppedEvent` |
| `BranchStartedMsg` / `BranchCompleteMsg` / `BranchErrorMsg` | flattened into the single-transcript model per FEAT-0014 (combined with run started / completed / failed events) |
| Tool / runtime permission origination | `PermissionRequestedEvent` |
| Tool result after `Runtime.ResolvePermission` | `PermissionResolvedEvent` (with `Message` constructed by the adapter) |
| Preview/file payload from `ContextManager` | `PreviewLoadedEvent` (or preview-failure event) |
| `ConnStateMsg` / `ModelUpdateMsg` / `ContextUpdateMsg` / `CostUpdateMsg` | `HostStatusEvent` with appropriate `StatusKind` |

The adapter holds the correlation table that maps shell submission/run
IDs to runtime turn IDs (and to branch IDs when the server emits parallel
review branches). The shell sees only stable run IDs.

## Mid-stream Pause

`FEAT-0014` requires that a permission request arriving during streaming
pauses the active stream immediately and resumes only after approval. In
the post-extraction architecture this responsibility lives in the
**adapter**, not in the shell and not behind a `Runtime.PauseRun`
method:

- on `PermissionRequestedEvent` while a run is active, the adapter stops <!-- provisional: subject to WU-100 reconciliation -->
  forwarding `RunDeltaEvent` <!-- provisional: subject to WU-100 reconciliation -->
  to the shell and buffers any further runtime deltas internally
- on `PermissionResolvedEvent`, the adapter replays buffered deltas in <!-- provisional: subject to WU-100 reconciliation -->
  arrival order before resuming live forwarding
- if the runtime/server itself naturally pauses streaming at the tool
  boundary, the buffer remains empty — but the buffer logic must still
  exist, because nothing in the boundary contract requires server-side
  pausing

The shell stays unaware of streaming-pause mechanics; it simply does not
receive deltas during the pause window.

## Where Demo Behavior Lives

Fake / demo runtime behavior — synthetic stream lifecycle events,
example permission requests, queue/preview exercise without a real
BFF — lives in `internal/harnessdemo`, **not** in this package. <!-- provisional: subject to WU-100 reconciliation -->

The reusable shell has two valid host packages plus inline test fakes:

- `internal/harnesshost` — real modeltap integration (this package). <!-- provisional: subject to WU-100 reconciliation -->
- `internal/harnessdemo` — examples and fake-runtime CLI capability. <!-- provisional: subject to WU-100 reconciliation -->
- ad-hoc test fakes constructed inline by shell unit tests.

`internal/harnessspike` is **not** a third host. It is deleted as part
of v0.2.1 release per WU-099 §"Stage 5" and the WU-100 Stage E plan.

## Related Packages

- [`internal/harnessshell/README.md`](../harnessshell/README.md) — reusable conversation-shell component this adapter drives. <!-- provisional: subject to WU-100 reconciliation -->
- `internal/harnessdemo` — fake-runtime adapter for examples and tests. <!-- provisional: subject to WU-100 reconciliation -->
- [`docs/guides/harness-shell-embedding.md`](../../docs/guides/harness-shell-embedding.md) — canonical embedding guide; includes the full submit / stream / permission / preview integration walkthroughs.
- [`docs/features/0014-harness-conversation-shell.md`](../../docs/features/0014-harness-conversation-shell.md) — behavior contract.
- [`docs/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md`](../../docs/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md) — full host-adapter integration design (WU-099).

## Reconciliation

Names in this README that end with the HTML comment
`<!-- provisional: subject to WU-100 reconciliation -->` are drawn from the
WU-098 / WU-099 designs and may be renamed when WU-100 lands. The
reconciliation pass performs a final sweep against the implemented names
before release v0.2.1 ships; see
[`docs/guides/harness-shell-embedding.md`](../../docs/guides/harness-shell-embedding.md)
§"Reconciliation With Final WU-100 Names" for the canonical mapping table.
