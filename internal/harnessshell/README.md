# `harnessshell` — Reusable Conversation Shell

`harnessshell` is the reusable Bubble Tea conversation-shell component for
the modeltap harness. It owns the single scrolling transcript surface, the
tail-mounted composer, the queued follow-up lifecycle, the composer-hosted
permission UI, and inline token rendering for paste and file references —
the behavior contract defined in
[`.sdlc/features/0014-harness-conversation-shell.md`](../../.sdlc/features/0014-harness-conversation-shell.md)
(FEAT-0014). It does not own provider transport, filesystem access, or
production permission persistence; those remain host responsibilities and
cross the package boundary as typed actions and events per
[`.sdlc/patches/0015-harness-shell-component-api.md`](../../.sdlc/patches/0015-harness-shell-component-api.md)
(PATCH-0015).

## Status

Feature-complete reusable component. `harnessshell.Model` accepts every
`HostEvent` defined by WU-098 and emits every required `Action` reachable
from shell-local key paths. The package is the canonical home of the
extracted shell behavior; the previous spike package
(`internal/harnessspike`) was deleted in WU-100 Stage E.

## Ownership

The shell component owns interaction semantics and rendering. The host owns
runtime effects and external data access. The boundary is action/event
based: the shell **emits actions** and the host **emits events** back into
the shell.

| Concern | Owner |
| --- | --- |
| Transcript rendering | shell |
| Composer rendering | shell |
| Viewport, focus, selection state | shell |
| Queue state and queue-release behavior | shell |
| Permission UI state and pending-permission navigation | shell |
| Token display and inline paste expansion | shell |
| Shell-local key handling | shell |
| Shell-native commands (`/clear`, transcript-local view actions) | shell |
| Turn submission to runtime server | host |
| Stream lifecycle and result delivery | host |
| Host-native command execution | host |
| Preview / file inspection loading | host |
| Production permission request origination and persistence | host |
| Policy state beyond shell-local UI concerns | host |
| Direct filesystem, provider, or modeltap runtime interaction | host |

The shell package must not import modeltap runtime concerns, must not call
provider logic, must not perform filesystem reads for preview loading, and
must not expose function-valued ("callback") fields at the boundary. Those
constraints are inherited from PATCH-0015 §8 ("API-shape guidance") and
PATCH-0015 §"Separation Requirements For Future Extraction".

## Preserved FEAT-0014 Invariants

The first-extraction goal was behavior parity with the spike, not redesign.
The reusable component preserves these shell-level invariants exactly:

- **Single scrolling surface.** The transcript and composer share one
  scrolling viewport; the composer is rendered at the tail of the
  transcript content rather than as a permanently fixed bottom slab. In
  tight vertical layouts the composer may scroll out of view when the user
  scrolls upward.
- **Tail-mounted composer.** Input focus does not force the viewport back
  to the bottom; mouse-wheel transcript scrolling does not steal input
  focus.
- **Queued follow-up release.** Follow-up messages submitted while a run
  is active are shown FIFO in the transcript as queued work. Normal
  completion auto-releases queued work; interrupt does not. **Pressing
  `Enter` on an empty composer while idle releases queued work** — this
  trigger is shell-local; the resulting submission still crosses the
  boundary as a normal `SubmitTurnAction`
  with `Source = SubmissionSourceQueueRelease`.
- **Composer-driven permissions.** Permission requests render durable
  history rows in the transcript and the active approval controls live in
  the composer area, not in a modal. Multiple pending permissions may
  coexist; `Up`/`Down` select which pending request the composer is
  controlling, `Left`/`Right` choose action, `Enter` applies, with
  optional `y`/`n` fallback only while the composer buffer is empty.
- **Preview-on-demand for file references.** Pasted content remains
  inspectable inline in the transcript; file/reference tokens stay compact
  path/reference tokens with on-demand preview. The shell asks for
  preview intent only — the host loads preview payloads.

A complete enumeration of preserved invariants lives in WU-098 §"Lifecycle
Invariants"; this README only summarizes the user-visible contract.

## Host Responsibility Boundary

The reusable shell package is intentionally narrow. The following remain
host responsibilities and must cross the action/event boundary as typed
data:

- **Provider logic.** No direct provider/RPC calls from the shell package.
  Submitted turns leave the shell as a `SubmitTurnAction`
  and the host returns lifecycle events (`SubmissionAcceptedEvent`,
  `RunStartedEvent`, `RunDeltaEvent`, `RunCompletedEvent`,
  `RunStoppedEvent`, `RunFailedEvent`).
- **Filesystem access.** Path validation, file reads, and preview payload
  loading happen host-side. The shell emits `LoadPreviewAction`
  and the host returns `PreviewLoadedEvent` (or a failure event).
- **Production permission handling.** Stable permission request identity,
  policy persistence, and runtime pause/resume execution are host-owned.
  The shell renders the request and emits a typed
  `ResolvePermissionAction`
  carrying `RequestID` and `Decision`.
- **Mid-stream permission pause.** Per WU-099, the host adapter is
  responsible for pausing and replaying `RunDeltaEvent`
  forwarding while a permission is pending. The shell does not directly
  pause streams; it simply receives no further deltas until the host
  resumes them after `PermissionResolvedEvent`.

## Action / Event Envelope

Outbound shell actions cross the boundary inside a single `tea.Msg`
envelope: `harnessshell.ActionMsg{Action Action}`. The host program (or
the modeltap host adapter at `internal/harnesshost`) pattern-matches
`ActionMsg` once and dispatches the concrete `Action` to the appropriate
runtime call. Inbound host events are concrete typed values that satisfy
the closed `harnessshell.HostEvent` interface; the host sends them as
`tea.Msg` values and the shell processes them in `Model.Update`.

## Minimal Embedding Example

Most modeltap embeddings drive the shell through
[`internal/harnesshost.Adapter`](../harnesshost/README.md), which wraps
the shell as a `tea.Model` decorator and bridges the action/event
boundary to a `Runtime` implementation. A minimal embedding looks like:

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
    runtime := /* your harnesshost.Runtime implementation */

    // Adapter wraps the shell: Update intercepts ActionMsg and
    // dispatches to runtime; runtime tea.Msgs project to HostEvents.
    adapter := harnesshost.New(shell, runtime)

    p := tea.NewProgram(adapter, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
```

For non-modeltap hosts that prefer to forward shell actions manually
without `harnesshost`, treat the shell as a normal `tea.Model`: forward
`tea.Msg` values to `Model.Update`, and pattern-match `ActionMsg` in your
own outer loop:

```go
inner, cmd := m.shell.Update(msg)
m.shell = inner.(harnessshell.Model)
// In your top-level Update, dispatch ActionMsg to your runtime
// service. The shell never invokes runtime calls directly.
```

For a fuller host integration walkthrough — submit, stream, permission,
and preview flows — see
[`docs/guides/harness-shell-embedding.md`](../../docs/guides/harness-shell-embedding.md).

## Related Packages

- [`internal/harnesshost/README.md`](../harnesshost/README.md) — modeltap-specific host adapter that consumes actions and produces events.
- [`internal/harnessdemo`](../harnessdemo) — fake/demo runtime adapter used by the `modeltap shell-demo` CLI and integration fixtures. Lives outside this package per WU-099.
- [`docs/guides/harness-shell-embedding.md`](../../docs/guides/harness-shell-embedding.md) — canonical embedding guide.
- [`.sdlc/features/0014-harness-conversation-shell.md`](../../.sdlc/features/0014-harness-conversation-shell.md) — behavior contract.
- [`.sdlc/patches/0015-harness-shell-component-api.md`](../../.sdlc/patches/0015-harness-shell-component-api.md) — extraction policy and API-shape rules.
