# 2026-04-26 — Session: WU-100 Stage C complete

## Scope

Continuation of the WU-100 Stage C checkpoint session
(`2026-04-26-session-wu-100-stage-c-checkpoint.md`). Drove Stage C
through to a clean, end-to-end-functional reusable shell: every required
outbound `Action` is emitted, every defined `HostEvent` is consumed, all
FEAT-0014 invariants the shell can verify are upheld in the shell-owned
pipeline.

## Commits landed

```
89571e4 WU-100: Stage C-3 — submit emission and run-lifecycle event intake
8368e7c WU-100: Stage C-4 — interrupt action emission
6115549 WU-100: Stage C-5 — permission action emission and intake
661dc25 WU-100: Stage C-6 — preview, status events, and Esc precedence
```

## Capability per commit

### Stage C-3 (`89571e4`) — submit + run lifecycle

- Enter routing: direct submit / queue follow-up / queue release /
  shell-native `/clear`
- `beginSubmission` appends optimistic user + assistant placeholder
  rows per WU-098 §"Optimistic transcript rendering"
- `releaseQueuedSubmission` merges and emits with `Source =
  queue_release`
- All 7 run-lifecycle events: `SubmissionAccepted`,
  `SubmissionFailed`, `RunStarted`, `RunDelta`, `RunCompleted`,
  `RunStopped`, `RunFailed`
- `RunCompletedEvent` auto-releases the queue; `RunStoppedEvent` and
  `RunFailedEvent` do not (FEAT-0014)
- `state.now` injectable clock for deterministic test stamping

### Stage C-4 (`8368e7c`) — interrupt

- First Esc arms (`StatusInterruptArmed`); second Esc emits
  `InterruptRunAction{RunID}`. The host's `RunStoppedEvent` /
  `RunFailedEvent` clears streaming chrome (intake from C-3)

### Stage C-5 (`6115549`) — permissions

- Enter (empty buffer + permission active) emits
  `ResolvePermissionAction` with selectedAction → decision mapping
- y/Y/n/N shortcuts emit approve-once/deny
- Left/Right walks the action selector (clamps [0,2])
- Up/Down (empty) navigates between multiple pending permissions
  before falling through to history recall
- `PermissionRequestedEvent` appends transcript event row + registers
  pending; `PermissionResolvedEvent` updates row status (granted/
  denied) and pops the pending entry. Status reverts to "Resuming
  run" while streaming with no remaining pending permissions

### Stage C-6 (`661dc25`) — preview + status

- Ctrl+O on composer paste-token previews locally
- Ctrl+O on composer file-token emits `LoadPreviewAction` with
  `Source: "composer"`
- Ctrl+O on transcript ref same logic with `Source: "transcript"` +
  MessageIndex/TokenIndex
- `PreviewLoadedEvent` paints the shell-local `PreviewDialog`
- `HostStatusEvent` applies status string + `StatusKind` (chrome
  decisions driven by Kind, not by parsing the string)
- Esc closes preview before reaching the interrupt-arm branch

## End-state of the shell after Stage C

`internal/harnessshell/Model` is a complete, embeddable Bubble Tea
component:

- **Inputs accepted:** `tea.WindowSizeMsg`, `tea.KeyMsg`, `tea.MouseMsg`,
  all 10 `HostEvent` concrete types defined in WU-098
- **Outbound actions:** `SubmitTurnAction`, `InterruptRunAction`,
  `ResolvePermissionAction`, `LoadPreviewAction` (all 4 of the 4
  reachable from shell-local key paths)
- **Not yet emitted:** `RunHostCommandAction` — there is no shell-local
  trigger for it yet because the only shell-native command (`/clear`)
  is handled locally. Wires up when host-native command routing lands
  during Stage E with the new CLI entrypoint
- **Closed-typed boundary:** `ActionMsg{Action Action}` envelope on the
  outbound side; concrete `HostEvent` types on the inbound side. No
  callback hooks, no untyped messages crossing the boundary

## What the spike still owns (until Stage D + E)

- Its own `App.Update` event loop, key handling, and submit pipeline
- Sidebar / command palette / agent overlays (out of scope for the
  reusable shell per WU-100 §"Definite scope rule")
- Background-agent state, presets, fake reply generation, fake stream
  timing, /perm demo (move to `internal/harnessdemo` in Stage E)
- Scroll-preservation logic in `App.refreshTranscript` (will move into
  `Model.View` or a host-driven refresh path during Stage E cutover)

The shell-owned pipeline added across Stages C-3 through C-6 is
**parallel and dormant** in the spike — nothing in spike calls
`Model.Update` yet. Spike tests still pass; this is exactly the state
the WU-100 §"Test compatibility during cutover" rule allows.

## What's next

- **Stage D (`internal/harnesshost`)** — modeltap-specific adapter that
  consumes `ActionMsg`, calls `Runtime` ops (a narrow modeltap-internal
  interface), and projects runtime messages into `HostEvent`s. Includes
  the mid-stream pause buffer per FEAT-0014 (runtime-side pause/resume
  per WU-099). The concrete `Runtime` impl wraps the existing
  `internal/harness/app_conn.ConnSurface`.
- **Stage E** — extract `internal/harnessdemo`; delete
  `internal/harnessspike`; replace/rename the `modeltap harness-spike`
  CLI entrypoint as a thin `harnessshell` + `harnessdemo` client.

## Open items

- The `RunHostCommandAction` emission path is undefined until the new
  CLI entrypoint lands in Stage E. Currently no shell-local trigger
  exists; the existing spike route for host-native commands runs
  through the spike's own update loop, not through `Model`.
- Scroll-preservation rewiring is deferred to Stage E because it
  needs the spike narrowing to expose where the followTail/YOffset
  logic should live (probably on `RunDeltaEvent` intake inside the
  shell).
- Branch retarget for `spike/scrolling-surface-eval` still pending TPM
  decision per `status.md`.
