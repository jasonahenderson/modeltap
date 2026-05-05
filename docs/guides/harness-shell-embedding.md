# Harness Shell Embedding Guide

This guide is the canonical developer-facing how-to for embedding the
extracted modeltap conversation shell into a Bubble Tea program. It
covers the package layout, the action / event boundary, the four host
integration flows, command routing, and the migration story for moving
the reusable shell into its own repository later.

The behavior contract this guide describes is
[`FEAT-0014`](../features/0014-harness-conversation-shell.md). The
extraction policy is [`PATCH-0015`](../patches/0015-harness-shell-component-api.md).
The shell-side API design is
[`WU-098`](../releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md);
the host-adapter design is
[`WU-099`](../releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md).

## Architecture Overview

After v0.2.1 the harness conversation experience is composed of three
packages, layered as follows:

```
                 ┌────────────────────────────────────────────┐
                 │           Bubble Tea host program          │
                 │   (top-level: composes shell + adapter     │
                 │    + runtime services / sidebar / status)  │
                 └────────────────────────────────────────────┘
                                  ▲      │
                            host  │      │ shell
                           events │      │ actions
                                  │      ▼
   ┌──────────────────────┐   ┌─────────────────────────────┐
   │ internal/harnessshell│◀──│   internal/harnesshost      │
   │  (reusable, no       │   │   (modeltap-specific glue,  │
   │   modeltap imports)  │──▶│    consumes actions, emits  │
   │                      │   │    events, owns mid-stream  │
   │                      │   │    pause/buffer)            │
   └──────────────────────┘   └─────────────────────────────┘
            ▲                                │
            │                                ▼
            │              ┌─────────────────────────────────┐
            │              │  modeltap runtime services      │
            │              │  (ConnSurface, tool dispatcher, │
            │              │   ContextManager, permission    │
            │              │   enforcer, ...)                │
            │              └─────────────────────────────────┘
            │
            │              ┌─────────────────────────────────┐
            └──────────────│   internal/harnessdemo          │
                           │   (fake-runtime adapter; same   │
                           │    Runtime contract as          │
                           │    harnesshost; powers the      │
                           │    `modeltap shell-demo` CLI)   │
                           └─────────────────────────────────┘
```

Three rules pin this layering down:

1. The reusable shell package (`internal/harnessshell`) does not import
   modeltap runtime, protocol, connection-manager, or tool packages.
   This is what makes promotion into a separate repository possible.
2. The modeltap host adapter (`internal/harnesshost`) is the only
   package in the repo that imports both `internal/harnessshell` and
   the modeltap runtime services.
3. The fake/demo runtime (`internal/harnessdemo`) implements the same
   `harnesshost.Runtime` contract as the production adapter but does
   not call real services. It powers the `modeltap shell-demo` CLI
   command, which replaced the legacy `modeltap harness-spike` CLI in
   WU-100 Stage E.

> **Note.** The valid post-extraction packages are `internal/harnessshell`,
> `internal/harnesshost`, and `internal/harnessdemo`. **`internal/harnessspike`
> was deleted at the end of v0.2.1** per WU-099 §"Stage 5" and WU-100
> Stage E. There is no `harnessspike` in the post-extraction architecture.

## Ownership and Boundary Rules

The shell component owns interaction semantics and rendering. The host
owns runtime effects and external data access. The boundary is
action/event based: the shell emits typed actions, and the host emits
typed events back into the shell.

### Shell-owned

- transcript rendering
- composer rendering
- viewport, focus, and selection state
- queue state and queue-release behavior
- permission UI state and pending-permission navigation
- token display and inline paste expansion
- shell-local key handling
- shell-native commands (`/clear`, transcript-local view actions)

### Host-owned

- turn submission to the runtime / BFF
- stream lifecycle and result delivery
- host-native command execution
- preview / file inspection loading
- production permission request origination and persistence
- policy state beyond shell-local UI concerns
- direct filesystem, provider, or modeltap runtime interaction
- mid-stream pause/resume of stream-delta forwarding while a permission
  is pending (adapter-driven; see "Permission Flow" below)

### Anti-patterns

The reusable shell package must not:

- call provider logic directly
- import modeltap runtime / protocol / tool packages
- expose callback-shaped API at the package boundary
  (no `OnApprove func()`, no `OnPreview func(...)`, no
  `Submit(..., onDelta func(...), ...)` — see PATCH-0015 §8)
- read files directly for preview loading

## Minimal Embedding Example

The simplest useful embedding wraps `harnessshell.Model` in
`harnesshost.Adapter`. The adapter is itself a `tea.Model`; the host
runs it as the program's model and the entire action/event boundary
is handled inside.

```go
package main

import (
    tea "github.com/charmbracelet/bubbletea"

    "github.com/jasonahenderson/modeltap/internal/harnessshell"
    "github.com/jasonahenderson/modeltap/internal/harnesshost"
)

func main() {
    shell := harnessshell.New(
        harnessshell.WithLabel("my-model"),
        harnessshell.WithPlaceholder("Type a message and press Enter."),
    )

    var runtime harnesshost.Runtime = newMyRuntime() // your impl
    adapter := harnesshost.New(shell, runtime)

    p := tea.NewProgram(adapter, tea.WithAltScreen(), tea.WithMouseAllMotion())
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
```

The `Adapter.Update` method is the integration point: shell-emitted
`harnessshell.ActionMsg` envelopes are intercepted and dispatched to
`Runtime` calls in `tea.Cmd` goroutines; runtime tea.Msgs from
`internal/harness/messages.go` (e.g., `StreamTokenMsg`,
`PermissionPromptMsg`) are projected to `HostEvent` values and forwarded
to the inner shell.

For non-modeltap hosts that prefer manual action handling without
`harnesshost`, the shell can be embedded directly:

```go
type model struct {
    shell harnessshell.Model
}

func (m model) Init() tea.Cmd { return m.shell.Init() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if am, ok := msg.(harnessshell.ActionMsg); ok {
        // Dispatch am.Action to your runtime service. The shell
        // already removed the action from its queue when it emitted
        // the ActionMsg via tea.Cmd.
        return m, m.handleAction(am.Action)
    }
    inner, cmd := m.shell.Update(msg)
    m.shell = inner.(harnessshell.Model)
    return m, cmd
}

func (m model) View() string { return m.shell.View() }
```

## Submit and Stream Integration

A submit followed by a streaming response runs as follows.

### Sequence

1. The user presses `Enter` with a non-empty composer buffer.
2. The shell appends the user-visible row **and** an assistant
   placeholder row (with `Streaming: true`) to the transcript
   immediately, then emits:

   ```go
   harnessshell.SubmitTurnAction{
       Submission: harnessshell.Submission{
           ID:     "...",                                 // shell-generated
           Entries: []string{"hello world"},
           Text:   "hello world",
           Tokens: nil,
           Source: harnessshell.SubmissionSourceDirect,
           RequestedAt: time.Now(),
       },
   }
   ```
3. The host adapter resolves attachments (via the
   `harnesshost.AttachmentResolver` option, default passthrough), calls
   `Runtime.SubmitTurn`, populates the correlation tables, and forwards:

   ```go
   harnessshell.SubmissionAcceptedEvent{
       SubmissionID: "...",
       RunID:        "...",
   }
   ```

   When `SubmitAccepted.Label` is non-empty, the adapter also synthesizes
   a `RunStartedEvent` so the chrome label updates without waiting for a
   real runtime stream message:

   ```go
   harnessshell.RunStartedEvent{
       SubmissionID: "...",
       RunID:        "...",
       Label:        "claude-opus-4-7",
   }
   ```
4. The runtime emits stream deltas; the adapter projects them as
   `RunDeltaEvent{RunID, Delta}` and the shell appends the text inline
   to the active assistant row.
5. Terminal lifecycle: one of `RunCompletedEvent`, `RunStoppedEvent`,
   `RunFailedEvent`. The shell finalizes visible run state and applies
   queue-release behavior on normal completion. Interrupt does **not**
   auto-release the queue.

### Adapter dispatch (illustrative)

The actual dispatch lives in `internal/harnesshost/adapter.go`; the
shape below mirrors that structure.

```go
func (a Adapter) dispatchSubmit(act harnessshell.SubmitTurnAction) tea.Cmd {
    sub := act.Submission
    ctx := a.ctx()
    attachments := /* a.resolver(...) over sub.Tokens */
    req := harnesshost.SubmitRequest{
        SubmissionID: sub.ID,
        Text:         sub.Text,
        Entries:      sub.Entries,
        Attachments:  attachments,
        Source:       sub.Source,
        RequestedAt:  sub.RequestedAt,
    }
    runtime := a.runtime
    return func() tea.Msg {
        accepted, err := runtime.SubmitTurn(ctx, req)
        if err != nil {
            return harnessshell.SubmissionFailedEvent{
                SubmissionID: sub.ID,
                Message:      err.Error(),
            }
        }
        return submissionAcceptedAdapterMsg{ /* internal correlation msg */ }
    }
}
```

Stream delta events arrive on a separate channel from the runtime /
connection manager. The adapter's `projectRuntimeMessage` translates
`harness.StreamTokenMsg` into `harnessshell.RunDeltaEvent` and routes
through `Adapter.forwardEvent`, which applies pause-buffer gating
before forwarding to the inner shell.

### Important rules

- **Empty `Enter` while idle is queue release, not a normal submit.**
  This trigger is shell-local; the resulting submission still crosses
  the boundary as a normal `SubmitTurnAction` with
  `Source = SubmissionSourceQueueRelease`. There is no second
  submission API on the host side.
- **Submit handling must not bypass the action/event boundary.** The
  top-level Bubble Tea program never reaches into shell transcript
  state to insert a submit row — the shell renders that row itself in
  response to the user's `Enter` press, before the adapter ever sees
  the action.
- **Manual scroll position is preserved unless the user is already
  following tail.** The shell takes care of this; host events do not
  need to manage viewport.
- **Interrupt/stop semantics remain those of FEAT-0014.** Two-step
  `Esc` arms then stops; the second `Esc` triggers
  `InterruptRunAction`. If the host cannot interrupt, it must still
  answer with a terminal lifecycle event so the shell leaves the armed
  state.

## Permission Flow Integration

Permission UI is **composer-hosted**, not modal. Mid-stream pause is
**adapter-driven** via stream-delta buffering, per WU-099. The shell
does not own pause/resume; it simply receives no further deltas during
the pause window.

### Sequence

1. Runtime / tool execution needs approval; the runtime emits a
   permission-origination notification (e.g.,
   `harness.PermissionPromptMsg`).
2. The adapter projects the notification into a shell-bound event:

   ```go
   harnessshell.PermissionRequestedEvent{
       Request: harnessshell.PermissionRequest{
           ID:                 "perm-123",
           ToolLabel:          "filesystem.write",
           Target:             "./README.md",
           Summary:            "Append release notes section",
           SessionPolicyState: harnessshell.SessionPolicyState{},
       },
   }
   ```
3. **If a run is currently streaming, the adapter stops forwarding
   `RunDeltaEvent`s and starts buffering them internally.** The shell
   only sees the permission request; it does not see further deltas
   from this run until the adapter resumes them. The buffer is keyed by
   the `pendingPermissions` set and drains in arrival order on
   `PermissionResolvedEvent`.
4. The shell renders a durable permission row in the transcript and
   activates the composer permission controls (`Approve once`, `Allow
   for session`, `Deny`). `Up`/`Down` switch which pending request the
   composer is controlling; `Left`/`Right` choose action; `Enter`
   applies; `y`/`n` work as fallback only while the composer buffer is
   empty.
5. The user makes a choice. The shell emits:

   ```go
   harnessshell.ResolvePermissionAction{
       RequestID: "perm-123",
       Decision:  harnessshell.DecisionApproveOnce,
   }
   ```
6. The adapter calls
   `Runtime.ResolvePermission(ctx, "perm-123", DecisionApproveOnce)`.
7. The adapter then:
   - emits `PermissionResolvedEvent{RequestID, Outcome, Message}` into
     the shell,
   - **replays buffered stream deltas in arrival order** before
     resuming live `RunDeltaEvent` forwarding,
   - keeps buffering if other pending permissions remain (multi-pending
     case: the buffer drains only when ALL pending resolve).

### Adapter sketch for buffered pause

The actual buffer logic lives in `Adapter.forwardEvent`:

```go
func (a Adapter) forwardEvent(evt harnessshell.HostEvent) (Adapter, tea.Cmd) {
    switch e := evt.(type) {
    case harnessshell.PermissionRequestedEvent:
        a.pendingPermissions[e.Request.ID] = struct{}{}
    case harnessshell.PermissionResolvedEvent:
        delete(a.pendingPermissions, e.RequestID)
        if len(a.pendingPermissions) == 0 && len(a.pauseBuffer) > 0 {
            // Forward resolve first, then replay buffered deltas.
            inner, cmd := a.shell.Update(evt)
            a.shell = inner.(harnessshell.Model)
            for _, d := range a.pauseBuffer {
                inner, _ = a.shell.Update(d)
                a.shell = inner.(harnessshell.Model)
            }
            a.pauseBuffer = nil
            return a, cmd
        }
    case harnessshell.RunDeltaEvent:
        if len(a.pendingPermissions) > 0 {
            a.pauseBuffer = append(a.pauseBuffer, e)
            return a, nil
        }
    }
    inner, cmd := a.shell.Update(evt)
    a.shell = inner.(harnessshell.Model)
    return a, cmd
}
```

### Important rules

- **Permission UI is composer-hosted, not modal.** The transcript holds
  the durable history row; the composer holds the active controls.
- **Repeated session-approved tools still surface a visible permission
  request.** Remembered policy state shows in the composer; it does not
  suppress the request.
- **Multiple pending permissions may coexist.** The shell handles
  multi-pending navigation; the adapter just emits and resolves them by
  `RequestID`.
- **Mid-stream pause is adapter-driven via stream-delta buffering.**
  The shell does not directly pause; it simply does not receive deltas
  until the adapter replays them after `PermissionResolvedEvent`. There
  is no `Runtime.PauseRun` / `Runtime.ResumeRun` method.
- **Runtime pause/resume policy is host-owned**, even though the shell
  owns the UI surface.

## Preview Flow Integration

Paste tokens are owned entirely by the shell — they expand inline in the
transcript without any host round trip. The host is responsible only
for **file/reference token preview loading**.

### Sequence

1. The user selects a file/reference token in the composer or transcript
   and presses `Ctrl+O` (or `Enter` on a transcript ref).
2. The shell handles paste tokens locally (toggles inline expansion or
   opens the shell-local preview dialog) and emits
   `LoadPreviewAction` only for file/reference tokens:

   ```go
   harnessshell.LoadPreviewAction{
       Target: harnessshell.PreviewTarget{
           TokenID:      "tok-42",
           Source:       "transcript",   // "composer" or "transcript"
           MessageIndex: 3,
           TokenIndex:   1,
       },
   }
   ```
3. The host adapter validates the path, reads the file (via the
   existing `ContextManager` / harness-owned path rules), and loads the
   preview payload.
4. On success, the adapter emits:

   ```go
   harnessshell.PreviewLoadedEvent{
       Target: act.Target,
       Preview: harnessshell.PreviewPayload{
           Title:    "README.md",
           Content:  "# Modeltap\n...",
           Metadata: map[string]string{"size": "12414"},
       },
   }
   ```

   On failure, the adapter emits `HostStatusEvent{Kind: StatusError}`
   with the failure message; the shell surfaces it in the status
   footer.
5. The shell paints the loaded payload into the shell-local
   `PreviewDialog`. `Esc` dismisses the preview before reaching any
   other Esc handler.

### Important rules

- **The reusable shell asks for preview intent only.** It does not
  validate paths and does not read files.
- **Path validation and file access stay host-side.** Any
  permission-gated file access also stays host-side.
- **Paste tokens never round-trip to the host for inline expansion.**
  The shell owns the payload from compaction onward.

## Shell-native vs Host-native Command Routing

The shell performs the first classification pass on slash-command text:

- if the text matches a **shell-native** command, the shell executes it
  locally and the adapter is not involved.
- otherwise, the shell emits `RunHostCommandAction` and the adapter
  routes the command to the appropriate runtime service via
  `Runtime.DispatchCommand`.

### Shell-native commands

Stay inside `internal/harnessshell` and never cross the boundary:

- `/clear`
- empty-`Enter` queue release while idle (the trigger is shell-native;
  the resulting submission crosses the boundary as `SubmitTurnAction`
  with `Source = SubmissionSourceQueueRelease`)
- token expansion / collapse and transcript-local inspection toggles
- permission selection / navigation keys

### Host-native commands

Cross the boundary as `RunHostCommandAction`. The host adapter routes
each to the existing harness command services via
`Runtime.DispatchCommand`:

- `/status`, `/reconnect` — connection state
- `/history` (if retained)
- `/model`, `/models` — model list and switch
- `/session`, `/sessions` — session list/resume/clear/fork
- `/context` — context list and `@file` resolution
- `/plan`, `/build`, `/auto` — mode commands
- `/mcp` — MCP status / reconnect
- `/compact` — context compaction
- `/help`

Anything else beginning with `/` that the shell does not recognize as
shell-native is emitted as `RunHostCommandAction`; the adapter is the
authority on command parsing for host-native commands.

## Demo CLI

`modeltap shell-demo` is the canonical demo CLI for the post-extraction
architecture. It composes `harnessshell` + `harnesshost.Adapter` +
`harnessdemo.FakeRuntime` (via `harnessdemo.Driver`) to drive the shell
end-to-end against a synthetic backend. Useful for evaluating shell
layout, streaming behavior, queue follow-ups, and the `/perm`
permission demo without a real BFF.

The legacy `modeltap harness-spike` command was removed in WU-100
Stage E.

## Migration Note: Future Extraction Into A Separate Repository

`internal/harnessshell` is repo-internal during v0.2.1, but its
boundary is intentionally clean enough that it can later move into its
own repository with minimal contract churn. The package separation
requirements from PATCH-0015 §"Separation Requirements For Future
Extraction" are binding for that future move:

- no direct dependency from the reusable shell package onto modeltap-
  specific runtime packages
- no direct provider logic inside the shell component
- no direct filesystem reads in the shell component beyond purely local
  UI concerns
- no hard dependency on spike-only demo commands for normal operation
- host integration through exported types/interfaces, not hidden
  package-local coupling
- transport-agnostic action/event contracts

When a future repo split happens, modeltap keeps `internal/harnesshost`
and `internal/harnessdemo` (or their renamed equivalents) and depends
on the new external `harnessshell` module. No changes to the
action/event contract should be required at that point — that is the
explicit goal of the v0.2.1 design.

## Final Names (post-WU-100 reconciliation)

The provisional name tables from earlier drafts of this guide have been
reconciled to the actual implementation names used in v0.2.1. There
are no provisional placeholders remaining.

| Concept | Final name |
| --- | --- |
| Reusable shell package | `internal/harnessshell` |
| Modeltap host adapter package | `internal/harnesshost` |
| Demo / fake-runtime adapter package | `internal/harnessdemo` |
| Shell outbound action marker | `harnessshell.Action` |
| Shell action envelope (`tea.Msg`) | `harnessshell.ActionMsg{Action Action}` |
| Shell inbound host event marker | `harnessshell.HostEvent` |
| Submit action | `harnessshell.SubmitTurnAction` |
| Interrupt action | `harnessshell.InterruptRunAction` |
| Resolve-permission action | `harnessshell.ResolvePermissionAction` |
| Load-preview action | `harnessshell.LoadPreviewAction` |
| Run-host-command action | `harnessshell.RunHostCommandAction` |
| Submission lifecycle events | `harnessshell.SubmissionAcceptedEvent`, `harnessshell.SubmissionFailedEvent` |
| Run lifecycle events | `harnessshell.RunStartedEvent`, `harnessshell.RunDeltaEvent`, `harnessshell.RunCompletedEvent`, `harnessshell.RunStoppedEvent`, `harnessshell.RunFailedEvent` |
| Permission events | `harnessshell.PermissionRequestedEvent`, `harnessshell.PermissionResolvedEvent` |
| Preview event | `harnessshell.PreviewLoadedEvent` (failures surface as `HostStatusEvent{Kind: StatusError}`) |
| Host status event | `harnessshell.HostStatusEvent` with `harnessshell.StatusKind` |
| Host adapter type | `harnesshost.Adapter` |
| Minimal host adapter interface | `harnesshost.Runtime` (six methods, see [`internal/harnesshost/README.md`](../../internal/harnesshost/README.md)) |
| Submission source enum | `harnessshell.SubmissionSourceDirect`, `harnessshell.SubmissionSourceQueueRelease` |
| Permission decisions | `harnessshell.DecisionApproveOnce`, `harnessshell.DecisionApproveSession`, `harnessshell.DecisionDeny` |
| Permission outcomes | `harnessshell.OutcomeApprovedOnce`, `harnessshell.OutcomeApprovedSession`, `harnessshell.OutcomeDenied` |
| Demo CLI command | `modeltap shell-demo` |

The `harnesshell.Source*` and similar enum-style constants are exported
typed string constants (`SubmissionSource`, `PermissionDecision`,
`PermissionOutcome`, `StatusKind`, `TokenKind`, `StopReason`).

## Cross-Links

- [`internal/harnessshell/README.md`](../../internal/harnessshell/README.md) — reusable shell package doc.
- [`internal/harnesshost/README.md`](../../internal/harnesshost/README.md) — modeltap host adapter doc.
- [`docs/features/0014-harness-conversation-shell.md`](../features/0014-harness-conversation-shell.md) — FEAT-0014 behavior contract.
- [`docs/patches/0015-harness-shell-component-api.md`](../patches/0015-harness-shell-component-api.md) — PATCH-0015 API-shape policy.
- [`docs/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md`](../releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md) — WU-098 shell-side API design.
- [`docs/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md`](../releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md) — WU-099 host adapter design.
- [`docs/releases/v0.2.1/designs/2026-04-25-design-docs-embedding-101.md`](../releases/v0.2.1/designs/2026-04-25-design-docs-embedding-101.md) — WU-101 docs design (this guide implements it).
