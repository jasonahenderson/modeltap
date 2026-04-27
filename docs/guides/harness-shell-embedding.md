<!--
WU-101 structural pass.
This guide uses provisional names from WU-098 and WU-099 designs. Anywhere
this file uses a name that may be renamed during WU-100 implementation, an
HTML comment of the form `<!-- provisional: ... -->` flags it for the
reconciliation pass that follows WU-100 cutover.

Source design: docs/releases/v0.2.1/designs/2026-04-25-design-docs-embedding-101.md
-->

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
                 │   (top-level App: composes shell + adapter │
                 │    + runtime services / sidebar / status)  │
                 └────────────────────────────────────────────┘
                                  ▲      │
                            host  │      │ shell
                           events │      │ actions
                                  │      ▼
   ┌──────────────────────┐   ┌─────────────────────────────┐
   │ internal/harnessshell│◀──│   internal/harnesshost      │     <!-- provisional: subject to WU-100 reconciliation -->
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
            └──────────────│   internal/harnessdemo          │     <!-- provisional: subject to WU-100 reconciliation -->
                           │   (fake-runtime adapter; same   │
                           │    action/event contract as     │
                           │    harnesshost; powers the      │
                           │    fake-data CLI)               │
                           └─────────────────────────────────┘
```

Three rules pin this layering down:

1. The reusable shell package (`internal/harnessshell`) does not import
   modeltap runtime, protocol, connection-manager, or tool packages.
   This is what makes promotion into a separate repository possible.
2. The modeltap host adapter (`internal/harnesshost`) is the only
   package in the repo that imports both `internal/harnessshell` and
   the modeltap runtime services.
3. The fake/demo runtime (`internal/harnessdemo`) speaks the same
   action/event contract as `internal/harnesshost` but does not call
   real services. It replaces the spike's compiled-in fake behavior
   when `internal/harnessspike` is deleted at the end of v0.2.1.

> **Note.** The valid post-extraction packages are `internal/harnessshell`,
> `internal/harnesshost`, and `internal/harnessdemo`. **`internal/harnessspike`
> is deleted at the end of v0.2.1** per WU-099 §"Stage 5" and WU-100 Stage E.
> Do not document `harnessspike` as part of the post-extraction architecture.

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

The smallest useful embedding is a Bubble Tea model that holds the shell
state, a host adapter, routes input into the shell, drains shell-emitted
actions to the adapter, and renders the shell as the main conversation
surface. Code follows the WU-098 typed action/event contract.

```go
package main

import (
    tea "github.com/charmbracelet/bubbletea"

    "modeltap/internal/harnessshell" // provisional package path
    "modeltap/internal/harnesshost"  // provisional package path
)

type model struct {
    shell   harnessshell.Model      // shell-owned state
    adapter *harnesshost.Adapter    // host adapter; modeltap-specific
}

func newModel(rt harnesshost.Runtime) model {
    return model{
        shell:   harnessshell.New(),
        adapter: harnesshost.NewAdapter(rt),
    }
}

func (m model) Init() tea.Cmd { return m.shell.Init() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // 1. Forward terminal/UI input AND host events into the shell.
    var shellCmd tea.Cmd
    m.shell, shellCmd = m.shell.Update(msg)

    // 2. Drain shell-emitted actions and route each to the adapter.
    cmds := []tea.Cmd{shellCmd}
    for _, action := range m.shell.DrainActions() {
        cmds = append(cmds, m.adapter.Dispatch(action))
    }
    return m, tea.Batch(cmds...)
}

func (m model) View() string {
    return m.shell.View()
}

func main() {
    rt := /* your modeltap Runtime implementation */
    p := tea.NewProgram(newModel(rt), tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
```

What this example deliberately does *not* show: full app chrome,
sidebar, config loading, network/bootstrap setup, production permission
persistence. This is the **minimal embedding**, not a reference app.

The `Adapter.Dispatch` method returns a `tea.Cmd` so the real runtime
work runs off the UI goroutine; the adapter then delivers a typed host
event back through the program's `Update` loop, and the shell consumes
it as input.

## Submit and Stream Integration

A submit followed by a streaming response runs as follows.

### Sequence

1. The user presses `Enter` with a non-empty composer buffer.
2. The shell appends the user-visible row **and** an assistant
   placeholder row (with `Streaming: true`) to the transcript
   immediately, then emits:

   ```go
   harnessshell.SubmitTurnAction{ // provisional name
       Submission: harnessshell.Submission{ // provisional name
           ID:     "...",                       // shell-generated
           Entries: []string{"hello world"},
           Text:   "hello world",
           Tokens: nil,
           Source: harnessshell.SourceDirect, // provisional name
       },
   }
   ```
3. The host adapter resolves attachments (via the existing
   `ContextManager`), calls `Runtime.SubmitTurn`, and feeds back:

   ```go
   harnessshell.SubmissionAcceptedEvent{ // provisional name
       SubmissionID: "...",
       RunID:        "...",
   }
   ```

   Followed (possibly later) by:

   ```go
   harnessshell.RunStartedEvent{ // provisional name
       SubmissionID: "...",
       RunID:        "...",
       Label:        "claude-3.7-sonnet",
   }
   ```
4. The runtime emits stream deltas; the adapter projects them as
   `RunDeltaEvent{RunID, Delta}` and the shell appends the text inline
   to the active assistant row.
5. Terminal lifecycle: one of `RunCompletedEvent`, `RunStoppedEvent`,
   `RunFailedEvent`. The shell finalizes visible run state and applies
   queue-release behavior on normal completion. Interrupt does **not**
   auto-release the queue.

### Host-side dispatch sketch

```go
func (a *Adapter) Dispatch(action harnessshell.Action) tea.Cmd { // provisional name
    switch act := action.(type) {
    case harnessshell.SubmitTurnAction:
        return a.handleSubmit(act)
    case harnessshell.InterruptRunAction:
        return a.handleInterrupt(act)
    case harnessshell.RunHostCommandAction:
        return a.handleHostCommand(act)
    case harnessshell.ResolvePermissionAction:
        return a.handleResolvePermission(act)
    case harnessshell.LoadPreviewAction:
        return a.handleLoadPreview(act)
    }
    return nil
}

func (a *Adapter) handleSubmit(act harnessshell.SubmitTurnAction) tea.Cmd {
    return func() tea.Msg {
        accepted, err := a.runtime.SubmitTurn(a.ctx, toSubmitRequest(act.Submission))
        if err != nil {
            return harnessshell.SubmissionFailedEvent{
                SubmissionID: act.Submission.ID,
                Message:      err.Error(),
            }
        }
        a.correlate(act.Submission.ID, accepted.RunID, act.Submission.Source)
        return harnessshell.SubmissionAcceptedEvent{
            SubmissionID: act.Submission.ID,
            RunID:        accepted.RunID,
        }
    }
}
```

Stream delta events arrive on a separate channel from the runtime / connection
manager and are projected into shell-bound events by `runtime_events.go` in
`internal/harnesshost`. The adapter pushes those events into the program via
`tea.Program.Send`, so they enter the shell through the same `Update` loop as
synchronous responses.

### Important rules

- **Empty `Enter` while idle is queue release, not a normal submit.** This
  trigger is shell-local; the resulting submission still crosses the boundary
  as a normal `SubmitTurnAction` with `Source = queue_release`. There is no
  second submission API on the host side.
- **Submit handling must not bypass the action/event boundary.** The top-level
  Bubble Tea program never reaches into shell transcript state to insert a
  submit row — the shell renders that row itself in response to the user's
  `Enter` press, before the adapter ever sees the action.
- **Manual scroll position is preserved unless the user is already following
  tail.** The shell takes care of this; host events do not need to manage
  viewport.
- **Interrupt/stop semantics remain those of FEAT-0014.** Two-step `Esc`
  arms then stops; the second `Esc` triggers `InterruptRunAction`. If the
  host cannot interrupt, it must still answer with a terminal lifecycle
  event so the shell leaves the armed state.

## Permission Flow Integration

Permission UI is **composer-hosted**, not modal. Mid-stream pause is
**adapter-driven** via stream-delta buffering, per WU-099. The shell does
not own pause/resume; it simply receives no further deltas during the
pause window.

### Sequence

1. Runtime / tool execution needs approval; the runtime emits a
   permission-origination notification.
2. Host adapter translates the notification into a shell-bound event:

   ```go
   harnessshell.PermissionRequestedEvent{ // provisional name
       Request: harnessshell.PermissionRequest{ // provisional name
           ID:                 "perm-123",
           ToolLabel:          "filesystem.write",
           Target:             "./README.md",
           Summary:            "Append release notes section",
           SessionPolicyState: harnessshell.PolicyNone, // provisional name
       },
   }
   ```
3. **If a run is currently streaming, the adapter stops forwarding
   `RunDeltaEvent`s and starts buffering them internally.** The shell
   only sees the permission request; it does not see further deltas
   from this run until the adapter resumes them.
4. The shell renders a durable permission row in the transcript and
   activates the composer permission controls (`Approve once`, `Allow
   for session`, `Deny`). `Up`/`Down` switch which pending request the
   composer is controlling; `Left`/`Right` choose action; `Enter`
   applies; `y`/`n` work as fallback only while the composer buffer is
   empty.
5. The user makes a choice. The shell emits:

   ```go
   harnessshell.ResolvePermissionAction{ // provisional name
       RequestID: "perm-123",
       Decision:  harnessshell.DecisionApproveOnce, // provisional name
   }
   ```
6. The adapter calls
   `Runtime.ResolvePermission(ctx, "perm-123", DecisionApproveOnce)`.
7. The adapter then:
   - constructs a `Message` from the runtime tool-result payload (or a
     generic granted/denied fallback when the payload is empty or
     structured),
   - emits `PermissionResolvedEvent{RequestID, Outcome, Message}` into
     the shell, <!-- provisional: subject to WU-100 reconciliation -->
   - **replays buffered stream deltas in arrival order** before
     resuming live `RunDeltaEvent` forwarding.

### Adapter sketch for buffered pause

```go
type Adapter struct {
    // ... runtime, correlation, etc.
    pendingPermission map[string]bool        // requestID → active
    bufferedDeltas    map[string][]string    // runID → buffered deltas
}

func (a *Adapter) onRuntimePermissionRequested(req PermissionRequest) {
    if runID := a.activeRunID(); runID != "" {
        a.pendingPermission[req.ID] = true
        // bufferedDeltas[runID] starts collecting on next delta.
    }
    a.send(harnessshell.PermissionRequestedEvent{Request: req})
}

func (a *Adapter) onRuntimeStreamDelta(runID, delta string) {
    if a.streamPaused(runID) {
        a.bufferedDeltas[runID] = append(a.bufferedDeltas[runID], delta)
        return
    }
    a.send(harnessshell.RunDeltaEvent{RunID: runID, Delta: delta})
}

func (a *Adapter) handleResolvePermission(act harnessshell.ResolvePermissionAction) tea.Cmd {
    return func() tea.Msg {
        err := a.runtime.ResolvePermission(a.ctx, act.RequestID, act.Decision)
        // ... build resolution message from runtime result ...
        msgs := []tea.Msg{
            harnessshell.PermissionResolvedEvent{
                RequestID: act.RequestID,
                Outcome:   outcomeFor(act.Decision, err),
                Message:   resolutionText,
            },
        }
        delete(a.pendingPermission, act.RequestID)
        // Replay buffered deltas before resuming live forwarding.
        if runID := a.runIDFor(act.RequestID); runID != "" {
            for _, d := range a.bufferedDeltas[runID] {
                msgs = append(msgs, harnessshell.RunDeltaEvent{RunID: runID, Delta: d})
            }
            delete(a.bufferedDeltas, runID)
        }
        return tea.Batch(send(msgs...))()
    }
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
- **Mid-stream pause is adapter-driven via stream-delta buffering.** The
  shell does not directly pause; it simply does not receive deltas until
  the adapter replays them after `PermissionResolvedEvent`. There is no
  `Runtime.PauseRun` / `Runtime.ResumeRun` method.
- **Runtime pause/resume policy is host-owned**, even though the shell
  owns the UI surface.

## Preview Flow Integration

Paste tokens are owned entirely by the shell — they expand inline in the
transcript without any host round trip. The host is responsible only for
**file/reference token preview loading**.

### Sequence

1. The user selects a file/reference token in the composer or transcript
   and triggers preview.
2. The shell emits:

   ```go
   harnessshell.LoadPreviewAction{ // provisional name
       Target: harnessshell.PreviewTarget{ // provisional name
           TokenID: "tok-42",
           Origin:  harnessshell.PreviewOriginTranscript, // provisional name
       },
   }
   ```
3. The host adapter validates the path, reads the file (via the existing
   `ContextManager` / harness-owned path rules), and loads the preview
   payload.
4. On success, the adapter emits:

   ```go
   harnessshell.PreviewLoadedEvent{ // provisional name
       Target: act.Target,
       Preview: harnessshell.PreviewPayload{ // provisional name
           Title:    "README.md",
           Content:  "# Modeltap\n...",
           Metadata: map[string]string{"size": "12414"},
       },
   }
   ```

   On failure, the adapter emits a preview-failure event so the shell
   can surface the failure in the inline expansion or preview surface.
5. The shell updates inline expansion or preview presentation state.

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
  routes the command to the appropriate runtime service.

### Shell-native commands

Stay inside `internal/harnessshell` and never cross the boundary:

- `/clear`
- empty-`Enter` queue release while idle (the trigger is shell-native;
  the resulting submission crosses the boundary as `SubmitTurnAction`
  with `Source = queue_release`)
- token expansion / collapse and transcript-local inspection toggles
- permission selection / navigation keys

### Host-native commands

Cross the boundary as `RunHostCommandAction`. The host adapter routes <!-- provisional: subject to WU-100 reconciliation -->
each to the existing harness command services:

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
and `internal/harnessdemo` (or their renamed equivalents) and depends on
the new external `harnessshell` module. No changes to the action/event
contract should be required at that point — that is the explicit goal
of the v0.2.1 design.

## Reconciliation With Final WU-100 Names

This guide and the package READMEs use **provisional** names from the
WU-098 and WU-099 designs. WU-100 may rename packages, types, or
functions during implementation; the reconciliation pass after WU-100
cutover updates every doc against the final names.

Provisional placeholders in this guide and the package READMEs are
flagged with the HTML comment
`<!-- provisional: subject to WU-100 reconciliation -->` so they are
greppable.

| Design role | Final implementation name | Notes |
| --- | --- | --- |
| Reusable shell package | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `internal/harnessshell` in design |
| Modeltap host adapter package | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `internal/harnesshost` in design |
| Demo / fake-runtime adapter package | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `internal/harnessdemo` in design |
| Shell outbound action marker type | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | must remain action-oriented; provisionally `Action` |
| Shell inbound host event marker type | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | must remain event-oriented; provisionally `HostEvent` |
| Submit action | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `SubmitTurnAction` |
| Interrupt action | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `InterruptRunAction` |
| Resolve-permission action | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `ResolvePermissionAction` |
| Load-preview action | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `LoadPreviewAction` |
| Run-host-command action | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `RunHostCommandAction` |
| Submission lifecycle events | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `SubmissionAcceptedEvent`, `SubmissionFailedEvent` |
| Run lifecycle events | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `RunStartedEvent`, `RunDeltaEvent`, `RunCompletedEvent`, `RunStoppedEvent`, `RunFailedEvent` |
| Permission events | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `PermissionRequestedEvent`, `PermissionResolvedEvent` |
| Preview event | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `PreviewLoadedEvent` (plus a failure variant) |
| Host status event | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `HostStatusEvent` with `StatusKind` |
| Minimal host adapter interface | `<final>` <!-- provisional: subject to WU-100 reconciliation --> | provisionally `Runtime` (six methods, see `internal/harnesshost/README.md`) |

Reconciliation rules:

- if implementation keeps the `internal/harnesshost` name, the docs
  use it
- if implementation renames packages or types, the docs update all
  narrative and examples to final names before review
- examples must not mix provisional and final names
- if WU-100 merges or splits packages differently than expected, the
  docs preserve the same ownership narrative even if labels change

The goal is **stable concepts first, final names second**.

## Cross-Links

- [`internal/harnessshell/README.md`](../../internal/harnessshell/README.md) — reusable shell package doc.
- [`internal/harnesshost/README.md`](../../internal/harnesshost/README.md) — modeltap host adapter doc.
- [`docs/features/0014-harness-conversation-shell.md`](../features/0014-harness-conversation-shell.md) — FEAT-0014 behavior contract.
- [`docs/patches/0015-harness-shell-component-api.md`](../patches/0015-harness-shell-component-api.md) — PATCH-0015 API-shape policy.
- [`docs/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md`](../releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md) — WU-098 shell-side API design.
- [`docs/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md`](../releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md) — WU-099 host adapter design.
- [`docs/releases/v0.2.1/designs/2026-04-25-design-docs-embedding-101.md`](../releases/v0.2.1/designs/2026-04-25-design-docs-embedding-101.md) — WU-101 docs design (this guide implements it).
