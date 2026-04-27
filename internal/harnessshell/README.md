<!--
WU-101 structural pass.
This README uses provisional names from WU-098 and WU-099 designs. Anywhere
this file uses a name that may be renamed during WU-100 implementation, an
HTML comment of the form `<!-- provisional: ... -->` flags it for the
reconciliation pass that follows WU-100 cutover.
-->

# `harnessshell` — Reusable Conversation Shell <!-- provisional: subject to WU-100 reconciliation -->

`harnessshell` is the reusable Bubble Tea conversation-shell component for
the modeltap harness. It owns the single scrolling transcript surface, the
tail-mounted composer, the queued follow-up lifecycle, the composer-hosted
permission UI, and inline token rendering for paste and file references —
the behavior contract defined in
[`docs/features/0014-harness-conversation-shell.md`](../../docs/features/0014-harness-conversation-shell.md)
(FEAT-0014). It does not own provider transport, filesystem access, or
production permission persistence; those remain host responsibilities and
cross the package boundary as typed actions and events per
[`docs/patches/0015-harness-shell-component-api.md`](../../docs/patches/0015-harness-shell-component-api.md)
(PATCH-0015).

## Status

Stage A skeleton at the time of this writing. Mirror types from
`internal/harnessspike` are being landed without behavior change; runtime
effects move out behind the action/event boundary in later stages of WU-100.
`internal/harnessspike` is deleted at end of release v0.2.1.

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
| Turn submission to runtime / BFF | host |
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

The first-extraction goal is behavior parity with the spike, not redesign.
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
  boundary as a normal `SubmitTurnAction` <!-- provisional: subject to WU-100 reconciliation -->
  with `Source = queue_release`.
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
  Submitted turns leave the shell as a `SubmitTurnAction` <!-- provisional: subject to WU-100 reconciliation -->
  and the host returns lifecycle events (`SubmissionAcceptedEvent`, <!-- provisional: subject to WU-100 reconciliation -->
  `RunStartedEvent`, `RunDeltaEvent`, `RunCompletedEvent`, <!-- provisional: subject to WU-100 reconciliation -->
  `RunStoppedEvent`, `RunFailedEvent`). <!-- provisional: subject to WU-100 reconciliation -->
- **Filesystem access.** Path validation, file reads, and preview payload
  loading happen host-side. The shell emits `LoadPreviewAction` <!-- provisional: subject to WU-100 reconciliation -->
  and the host returns `PreviewLoadedEvent` (or a failure event). <!-- provisional: subject to WU-100 reconciliation -->
- **Production permission handling.** Stable permission request identity,
  policy persistence, and runtime pause/resume execution are host-owned.
  The shell renders the request and emits a typed
  `ResolvePermissionAction` <!-- provisional: subject to WU-100 reconciliation -->
  carrying `RequestID` and `Decision`.
- **Mid-stream permission pause.** Per WU-099, the host adapter is
  responsible for pausing and replaying `RunDeltaEvent` <!-- provisional: subject to WU-100 reconciliation -->
  forwarding while a permission is pending. The shell does not directly
  pause streams; it simply receives no further deltas until the host
  resumes them after `PermissionResolvedEvent`. <!-- provisional: subject to WU-100 reconciliation -->

## Minimal Embedding Example

The following sketch shows the smallest useful Bubble Tea host. It is
illustrative; example code follows the WU-098 typed action/event contract
and does not need to compile against the Stage A skeleton.

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
    // 1. Forward terminal/UI input and host events into the shell.
    var cmd tea.Cmd
    m.shell, cmd = m.shell.Update(msg)
    cmds := []tea.Cmd{cmd}

    // 2. Drain shell-emitted actions and route them to the host adapter.
    for _, action := range m.shell.DrainActions() {
        cmds = append(cmds, m.adapter.Dispatch(action))
    }

    return m, tea.Batch(cmds...)
}

func (m model) View() string { return m.shell.View() }

func main() {
    rt := /* your modeltap Runtime implementation */
    p := tea.NewProgram(newModel(rt), tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
```

The host adapter's `Dispatch` returns a `tea.Cmd` that performs the real
runtime effect off the UI goroutine and delivers the resulting host event
back through the program's `Update` loop, where the shell consumes it as
input.

For a fuller host integration walkthrough — submit, stream, permission,
and preview flows — see
[`docs/guides/harness-shell-embedding.md`](../../docs/guides/harness-shell-embedding.md).

## Related Packages

- [`internal/harnesshost/README.md`](../harnesshost/README.md) — modeltap-specific host adapter that consumes actions and produces events. <!-- provisional: subject to WU-100 reconciliation -->
- `internal/harnessdemo` — fake/demo runtime adapter used by the shell-with-fake-data CLI and integration fixtures. Lives outside this package per WU-099. <!-- provisional: subject to WU-100 reconciliation -->
- [`docs/guides/harness-shell-embedding.md`](../../docs/guides/harness-shell-embedding.md) — canonical embedding guide.
- [`docs/features/0014-harness-conversation-shell.md`](../../docs/features/0014-harness-conversation-shell.md) — behavior contract.
- [`docs/patches/0015-harness-shell-component-api.md`](../../docs/patches/0015-harness-shell-component-api.md) — extraction policy and API-shape rules.

## Reconciliation

Names in this README that end with the HTML comment
`<!-- provisional: subject to WU-100 reconciliation -->` are drawn from the
WU-098 / WU-099 designs and may be renamed when WU-100 lands. The
reconciliation pass performs a final sweep against the implemented names
before release v0.2.1 ships; see
[`docs/guides/harness-shell-embedding.md`](../../docs/guides/harness-shell-embedding.md)
§"Reconciliation With Final WU-100 Names" for the canonical mapping table.
