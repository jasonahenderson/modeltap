# v0.2.1 Changelog

**Status:** released (tagged on branch `spike/scrolling-surface-eval`)

v0.2.1 extracts the modeltap conversation-shell experience from the
spike implementation into a reusable Bubble Tea component plus a
modeltap-specific host adapter, with a fake/demo runtime powering a
stand-alone CLI for evaluation. The legacy `modeltap harness` command
and its TUI App were scrapped in this release; production provider
integration through the new adapter is the v0.2.2 headline.

For a detailed work-unit-level breakdown see `status.md` and the
per-WU commit messages.

## Headline additions

### Reusable conversation shell (`internal/harnessshell`)

A reusable Bubble Tea conversation-shell component that owns the single
scrolling transcript surface, the tail-mounted composer, the queued
follow-up lifecycle, the composer-hosted permission UI, and inline
token rendering for paste and file references — the FEAT-0014 behavior
contract. The package has no modeltap runtime, protocol, or
filesystem-access imports; it is structured for future promotion into
a separate repository.

The shell speaks across the package boundary in typed actions and host
events, with no callback-shaped API. The single `tea.Msg` envelope
`harnessshell.ActionMsg{Action Action}` carries outbound actions; the
host returns concrete `HostEvent` values (`SubmissionAcceptedEvent`,
`RunDeltaEvent`, `RunCompletedEvent`, `PermissionRequestedEvent`,
`PreviewLoadedEvent`, etc.). Optimistic transcript rendering, FIFO
queue invariants, two-step Esc-arms-then-stops interrupt, paste-token
inline expansion, drag-dropped file capture, slash-command-not-
classified-as-path detection, and history recall are all preserved
exactly per FEAT-0014.

### Modeltap host adapter (`internal/harnesshost`)

A `tea.Model` decorator that wraps `harnessshell.Model` and bridges
the action/event boundary to a narrow modeltap-internal `Runtime`
interface (`SubmitTurn`, `InterruptRun`, `DispatchCommand`,
`ResolvePermission`, `LoadPreview`, `SummarizePaste`).

Three internal halves:

1. **Action consumer.** Every shell `Action` dispatches to the
   corresponding `Runtime` call inside a `tea.Cmd`, with success
   producing a typed `HostEvent` and failure producing the documented
   failure variant.
2. **Event projector.** `internal/harness` runtime tea.Msgs
   (`StreamTokenMsg`, `StreamCompleteMsg`, `TurnSubmittedMsg`,
   `StatusUpdateMsg`, `BranchStarted/Complete/ErrorMsg`,
   `ToolActivityMsg`, `PermissionPromptMsg`, `ConnStateMsg`,
   `ModelUpdateMsg`, `ContextUpdateMsg`, `CostUpdateMsg`) project to
   typed `HostEvent` values. Multi-model branches flatten into per-
   branch run events (`RunID = "TurnID:BranchID"`) so the FEAT-0014
   single-transcript model is preserved.
3. **Mid-stream pause buffer.** `PermissionRequestedEvent` registers
   a pending request; `RunDeltaEvent` forwarding buffers internally
   while any permission is pending; `PermissionResolvedEvent` (the
   last pending one) drains the buffer in arrival order before
   resuming live forwarding. The shell pauses without needing a
   `Runtime.PauseRun` method, per the WU-099 mid-stream design.

### Fake-runtime demo (`internal/harnessdemo`)

A `harnesshost.Runtime` implementation backed by canned fake replies,
configurable per-chunk stream timing, and a `/perm`-prefixed
permission demo. A `Driver` `tea.Model` wraps the adapter and
orchestrates fake stream emission via tea.Tick scheduling — the demo
delivers realistic stream/complete lifecycle events and
permission-pause/resume cycles end-to-end without a real BFF.

### `modeltap shell-demo` CLI

The new conversation-shell entrypoint. Constructs `harnessshell` +
`harnessdemo` + `harnesshost.Adapter` and runs as a tea.Program.
Replaces the legacy `modeltap harness-spike` command (deleted in
WU-100 Stage E) and the `modeltap harness` command (deleted as part
of the v0.2.1 cleanup that scrapped the unmaintained legacy TUI).

### Documentation: embedding guide

`docs/guides/harness-shell-embedding.md` is the canonical developer-
facing how-to for embedding the shell, covering package layout, the
action/event boundary, the four host-integration flows (submit,
permission, preview, command routing), and the migration story for
moving the reusable shell into its own repository later. The guide
matches the implementation names verbatim — no provisional
placeholders remain.

## Removals

- **`internal/harnessspike/`** — the spike package and its ~3,400-line
  `App` implementation deleted in WU-100 Stage E-3. Spike-only chrome
  (sidebar, command palette, agent overlays, background-agent state,
  presets) was out of scope for the reusable shell per WU-100
  §"Definite scope rule" and is not in the post-extraction
  architecture. Any future modeltap top-level harness package can
  reintroduce equivalents without touching `internal/harnessshell`.
- **`modeltap harness-spike` CLI command** — replaced by `modeltap
  shell-demo`.
- **`modeltap harness` CLI command** — the v0.2.0 legacy TUI entry
  point, reported as broken. The wiring layer at
  `internal/cli/harness.go` was deleted; bare `modeltap` falls back
  to cobra default help.
- **Bare-`modeltap`-runs-harness behavior** — root command's RunE
  removed; `--model` / `--resume` / `--socket` orphan flags gone from
  the root command.

## Test coverage

Layer 1 reusable-shell parity tests cover queue FIFO, FEAT-0014 SC4
end-to-end (queue → interrupt → idle Enter releases), paste token
capture, dropped path → file token, slash-command-not-path
classification, submitted paste starts expanded, transcript-Enter
toggle, permission lifecycle (request, navigate, decide, resolve),
y/n/Left/Right/Up/Down composer key paths, run lifecycle (submit,
accept, fail, started, delta, complete, stopped, failed).

Layer 2 host adapter tests cover every shell action's dispatch to
Runtime, success and failure paths, runtime-message projection for
every relevant `internal/harness` tea.Msg type, and the mid-stream
pause buffer including multi-permission-still-pending and arrival-
order-preserved replay.

Layer 3 integration tests cover the adapter-side end-to-end pipeline
(submit-stream-complete with correlation table population) and mid-
stream permission pause integration with full pumpAdapter cmd-chain
chasing.

## Known gaps

- **Stage D-4 production wiring** — the v0.2.2 headline. Implements a
  concrete `Runtime` impl backed by salvageable parts of
  `internal/harness/` (connection / protocol / tool dispatcher /
  context manager / MCP) and a new production CLI entrypoint that
  wraps the shell. The legacy harness package files are unreachable
  from any CLI in v0.2.1 but still compile; v0.2.2 audits and
  salvages.
- **WU-102 SC3 follow-up** — manual scroll preservation has no direct
  automated assertion because viewport state is private inside
  `Model.View`'s local copy. A test-only accessor or refactor would
  close the gap. Not blocking.

## Branch

Tagged on `spike/scrolling-surface-eval`. Branch retarget to a release
branch is pending TPM decision and may happen before publishing the
tag.
