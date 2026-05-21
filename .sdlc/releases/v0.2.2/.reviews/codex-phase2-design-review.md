# Codex Phase 2 Design Review: v0.2.2

Review scope:

- `.sdlc/releases/v0.2.2/plan.md`
- `.sdlc/releases/v0.2.2/status.md`
- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-harness-audit-103.md`
- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-production-wiring-104-106.md`
- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-viewport-state-accessor-107.md`

Reference contracts:

- `.sdlc/features/0014-harness-conversation-shell.md`
- `.sdlc/patches/0015-harness-shell-component-api.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md`

## Findings

### 1. WU-105 starts before its declared WU-104 dependency is complete

Severity: blocking

Location:

- `.sdlc/releases/v0.2.2/plan.md` — Work units
- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-production-wiring-104-106.md` — Implementation order across WUs

What's wrong:

WU-105 is planned to start after only partial WU-104 completion, but the
release plan says WU-105 depends on WU-104. That violates dependency-legal
Phase 3 execution.

Suggested fix:

Either change WU-105's dependency to "WU-104 SubmitTurn slice" by splitting
WU-104, or require all WU-104 Runtime methods to land before WU-105 starts.

### 2. Permission prompt deletion removes the concrete permission bridge

Severity: blocking

Location:

- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-harness-audit-103.md` — Delete / `permission_prompt.go`
- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-production-wiring-104-106.md` — `ResolvePermission`

What's wrong:

The plan deletes `permission_prompt.go`, but WU-104 assumes an existing
channel-based permission gate can be resolved by `ToolDispatcher.ResolvePermission`.
In current code, the blocking prompt bridge/channel behavior lives in the
permission prompt path, not as a `ToolDispatcher.ResolvePermission` API. This
leaves no concrete path for originating shell permission requests or unblocking
tool execution.

Suggested fix:

Keep/refactor the non-UI permission broker out of `permission_prompt.go` before
deleting modal UI chrome. Define how `PromptCallback` emits
`PermissionRequestedEvent` and how `Runtime.ResolvePermission(requestID,
decision)` resolves the matching channel.

### 3. Production preview loading has no path-resolution source

Severity: significant

Location:

- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-production-wiring-104-106.md` — `LoadPreview`

What's wrong:

`LoadPreview` requires `req.Path`, but the existing shell `LoadPreviewAction`
carries only token identity/source, and the adapter currently builds
`PreviewRequest` without a path. Production preview cannot work for composer
tokens unless the host can map token ID back to payload/path.

Suggested fix:

Define the preview-resolution seam explicitly: either include token payload/path
in `LoadPreviewAction` for file tokens, or have `harnesshost.Adapter` maintain a
tokenID-to-attachment/path table from submissions and composer state.

### 4. Interrupt design invents a new method despite existing turn cancellation

Severity: significant

Location:

- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-production-wiring-104-106.md` — `InterruptRun`
- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-harness-audit-103.md` — Runtime method to harness service mapping

What's wrong:

The design invents `Client.SendInterrupt` / `turn.interrupt`, but the current
protocol/client already has `turn.cancel` / `CancelTurn`. Treating interrupt as
missing may create unnecessary server/API work and inconsistent semantics.

Suggested fix:

First map `Runtime.InterruptRun` to existing `ProtocolClient.CancelTurn` /
`protocol.MethodTurnCancel`; only add a new protocol method if cancellation
semantics are proven insufficient.

### 5. ViewportState lazy cache cannot persist through a value receiver

Severity: significant

Location:

- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-viewport-state-accessor-107.md` — Implementation note: where the state is captured

What's wrong:

The accessor design says `View()` will lazily allocate a pointer cache on
`Model.state`, but `Model.View()` has a value receiver. A lazy assignment to the
copied model will not persist for a subsequent `m.ViewportState()` call.

Suggested fix:

Initialize the cache pointer in `New()` so `View()` mutates the shared pointed-to
snapshot, or change the design to derive the snapshot from persisted viewport
state in `Update` rather than lazy assignment in `View`.

### 6. Bare `modeltap` shell launch is an uncalled-out user-visible behavior change

Severity: advisory

Location:

- `.sdlc/releases/v0.2.2/designs/2026-04-27-design-production-wiring-104-106.md` — WU-105 / Bare `modeltap` behavior

What's wrong:

The design restores bare `modeltap` to launch the shell, replacing current help
behavior. That is a user-visible CLI behavior change, not just production
wiring, and it may unexpectedly auto-start the BFF.

Suggested fix:

Make this an explicit Phase 2 decision in the plan/status and add tests for bare
invocation behavior, or defer bare `modeltap` shell launch until after the
production shell is proven stable.

## Disposition

| Finding | Severity | Status | Disposition |
| --- | --- | --- | --- |
| 1 — WU-105 starts before WU-104 dependency | blocking | accepted | Split WU-104 into three slices: WU-104a (SubmitTurn), WU-104b (LoadPreview + ResolvePermission + InterruptRun), WU-104c (DispatchCommand + SummarizePaste). WU-105's dependency tightens to WU-104a only — once SubmitTurn lands, the CLI is wireable end-to-end and the remaining methods light up command paths as they ship. Plan + production-wiring design updated. |
| 2 — permission_prompt deletion removes permission bridge | blocking | accepted | Adopting Kimi #2's more-detailed architecture: the permission broker logic moves into `harnesshost.ProductionRuntime` (a `sync.Map` of `chan PermissionDecision` keyed by `ToolCallID`); the runtime registers a custom `PromptCallback` on `tools.PermissionEnforcer` that emits `harness.PermissionPromptMsg` (which the projection layer translates to `PermissionRequestedEvent`) and blocks on the per-call channel. `ResolvePermission` writes the decision into the matching channel. `permission_prompt.go` deletion is fine after the broker logic relocates. |
| 3 — LoadPreview has no path-resolution source | significant | accepted | `harnesshost.Adapter` gains a tokenID→`Attachment` table populated on every `SubmitTurnAction` dispatch (from `Submission.Tokens`) and on composer-token state injection (host-side helpers). When `LoadPreviewAction` arrives, the adapter looks up the attachment to populate `PreviewRequest.Path` before calling `Runtime.LoadPreview`. Production-wiring design updated. |
| 4 — Interrupt invents a new method despite existing turn.cancel | significant | accepted | `Runtime.InterruptRun` calls `ProtocolClient.CancelTurn(ctx, runID)` against the existing `protocol.MethodTurnCancel`. No new client method or BFF protocol change. If `CancelTurn` returns an unsupported-feature error the adapter synthesizes `RunStoppedEvent` (per Kimi #7) so the UX matches a clean stop rather than a red error. Production-wiring design updated. |
| 5 — ViewportState lazy cache cannot persist through value receiver | significant | accepted | `New()` preallocates a `*ViewportState` and stores it on `state.viewportCache`. `View()` mutates the pointed-to value (which works through the value-receiver because the field is a pointer). `ViewportState()` reads the snapshot. The "purity" claim in the design is reworded per Kimi #13: View intentionally mutates the lazy cache via a pointer field; the cache is not part of semantic state. Viewport-state-accessor design updated. |
| 6 — bare `modeltap` shell launch is uncalled-out CLI behavior change | advisory | rejected | Decided NOT to restore bare-`modeltap`-launches-shell. v0.2.1 deliberately stripped that behavior (with the broken legacy harness); restoring it now would surprise users with an auto-BFF-start path that hasn't been exercised in v0.2.2 yet. Bare `modeltap` continues to fall back to cobra default help. The user can add the bare-launch behavior in a future release if desired. Production-wiring design + status.md "Up next" updated to record the decision. Kimi #8 (test breakage) is moot under this disposition. |

