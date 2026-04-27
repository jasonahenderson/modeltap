# v0.2.1 — Status

**Current phase:** Phase 3 — Implementation
**Branch:** `spike/scrolling-surface-eval` (should retarget to a release branch
before tagging; pending TPM decision)
**Phase 1 closed:** 2026-04-26
**Phase 2 closed:** 2026-04-26

## Phase 3 work units

| WU | Title | Size | State | Notes |
|----|-------|------|-------|-------|
| 097 | Refactor Plan and Migration Sequencing | M | Done (Phase 1 design) | Deliverable is the design doc |
| 098 | Shell Component API and Package-Boundary Design | M | Done (Phase 1 design) | Deliverable is the design doc |
| 099 | Modeltap Host Adapter and Integration Design | M | Done (Phase 1 design) | Deliverable is the design doc |
| 100 | Behavior-Preserving Shell Extraction Implementation | L | In progress | Stages A+B+C done; Stage D adapter complete (D-1 `07528ba`, D-2 `b03604a`, D-3 `caf93b7`); Stage D-4 production wiring + Stage E (delete spike + `internal/harnessdemo` + CLI rename) still ahead |
| 101 | Developer Documentation and Embedding Examples | M | In progress | Structural pass done (`a40e4b9`); reconciliation pass after WU-100 cutover |
| 102 | Parity and Regression Test Sweep | M | Up next | Starts after WU-100 cutover |

## Phase 3 dependency graph

```
WU-100 (extraction)  ──→  WU-102 (parity tests)
       ║
       ║  parallel
       ║
WU-101 (docs)
```

WU-097, WU-098, WU-099 produced design docs in Phase 1; their Phase 3
deliverables are already present.

## Up next

- **WU-100 Stage D-4** — production wiring: implement a concrete
  `Runtime` backed by `internal/harness.ConnSurface`,
  `ToolDispatcher`, `ContextManager`, `PermissionEnforcer`. Modify the
  existing `internal/harness/app.go` (or build a parallel entrypoint)
  to wrap its `tea.Model` in `harnesshost.Adapter`. The async/sync
  bridging for `Runtime.SubmitTurn` (existing flow dispatches via
  `ConnSurface.SubmitTurn` and gets `TurnSubmittedMsg` later) needs a
  small design call: either dispatch async + return a placeholder
  `RunID` and rely on the projection layer's `TurnSubmittedMsg`
  translation, or block on the response inside the `tea.Cmd` goroutine.
- **WU-100 Stage E** — extract `internal/harnessdemo` (fake replies,
  presets, /perm demo, stream timing). Delete `internal/harnessspike`
  entirely. Replace/rename the `modeltap harness-spike` CLI entrypoint
  with a thin client of `harnessshell` + `harnessdemo`.
- **WU-101 reconciliation** — fill in the 85 placeholders in the
  embedding docs against the post-extraction architecture (now
  partially possible; final sweep after Stage E settles
  `harnessdemo` + CLI names).
- **WU-102 parity sweep** — migrate spike test coverage into
  shell-unit and host-integration test layouts per WU-102 design;
  blocked on Stage E spike deletion.

## In progress

(none — Stage D-4 is queued for dispatch; Stage E is queued behind it)

## Done this phase

- WU-100 Stage A (`1cb1eb4`) — `internal/harnessshell` skeleton with action/event
  boundary types, state structs, and styles. Build clean, no
  `internal/harness/theme` import.
- WU-100 Stage B (`1e32b57`) — rendering cutover. Pure-function `Render(RenderInput)
  → RenderResult` value-type bridge. Spike's `refreshTranscript` now delegates
  to `harnessshell.Render`. Sidebar/palette/agent overlays remain in spike.
  Spike tests + build clean.
- WU-100 Stage C scaffolding (`19a546b`) — shell-owned helpers for composer
  history, paste/dropped-path detection, queue merge (FIFO + transient
  pendingSubmissions buffer), and pending-permission lifecycle. Pure additive;
  no behavior change. Stage A out-of-scope chrome fields removed from the
  state struct per WU-100 §"Definite scope rule".
- WU-100 Stage C wire-up (`d106fc6`) — `Model.New`/`Init`/`Update`/`View`
  wired for shell-local keys; resolves the WU-098-deferred envelope choice
  with `ActionMsg{Action Action}` (single envelope; concrete dispatch at
  the host adapter).
- WU-100 Stage C-3 (`89571e4`) — Submit action emission with optimistic
  transcript rendering; run-lifecycle event intake (SubmissionAccepted/
  Failed, RunStarted/Delta/Completed/Stopped/Failed); auto-release queue
  on RunCompleted only (FEAT-0014 invariant).
- WU-100 Stage C-4 (`8368e7c`) — Interrupt action emission. First Esc
  arms; second Esc emits `InterruptRunAction` with the active RunID.
- WU-100 Stage C-5 (`6115549`) — Permission action emission and intake.
  Enter (empty buffer + permission active) emits
  `ResolvePermissionAction`; y/n/Y/N shortcuts; Left/Right walks action
  selector; Up/Down navigates between multiple pending permissions.
  PermissionRequestedEvent / PermissionResolvedEvent intake updates
  transcript event row and pending list.
- WU-100 Stage C-6 (`661dc25`) — Ctrl+O preview routing (paste tokens
  preview locally; file tokens emit `LoadPreviewAction`).
  PreviewLoadedEvent and HostStatusEvent intake. Esc closes preview
  before any other Esc handler.
- WU-100 Stage D-1 (`07528ba`) — `internal/harnesshost` action-consumer
  half. `Runtime` interface per WU-099 (narrower than
  `ConnProtocolClient`, intentionally omits `PauseRun`/`ResumeRun`).
  `Adapter` wraps `harnessshell.Model` as a `tea.Model` decorator and
  dispatches every shell `Action` (Submit/Interrupt/ResolvePermission/
  LoadPreview/RunHostCommand) to the corresponding `Runtime` method via
  `tea.Cmd`. Correlation table populated on submit-accepted. Eleven
  tests cover success and error paths with a fake `Runtime`.
- WU-100 Stage D-2 (`b03604a`) — runtime message → `HostEvent`
  projection. `projectRuntimeMessage` translates every relevant
  `internal/harness` runtime tea.Msg (`StreamTokenMsg`,
  `StreamCompleteMsg`, `TurnSubmittedMsg`, `StatusUpdateMsg`,
  `BranchStarted/Complete/ErrorMsg`, `ToolActivityMsg`,
  `PermissionPromptMsg`, `ConnStateMsg`, `ModelUpdateMsg`,
  `ContextUpdateMsg`, `CostUpdateMsg`) into the corresponding
  shell-bound HostEvent. Multi-model branches flatten into per-branch
  `Run*Event`s (RunID = "TurnID:BranchID"). Paste-handler msgs project
  to nil so they remain host-App-owned.
- WU-100 Stage D-3 (`caf93b7`) — mid-stream pause buffer per WU-099
  §"Mid-stream Pause". `Adapter` registers `PermissionRequestedEvent`
  IDs in a pending set; while non-empty, `RunDeltaEvent` forwarding
  buffers internally instead of flowing to the shell. On
  `PermissionResolvedEvent`, when the pending set drains to empty the
  resolve forwards first then buffered deltas replay in arrival order.
  Multi-permission case: only when ALL pending are resolved does the
  buffer drain. The shell pauses without needing a `PauseRun` Runtime
  method.
- WU-101 structural pass (`a40e4b9`) — `internal/harnessshell/README.md`,
  `internal/harnesshost/README.md`, `docs/guides/harness-shell-embedding.md`
  with 85 provisional placeholders to be reconciled after WU-100 cutover.

## Stage C exit checklist

The shell is now functionally complete in isolation:

- ✅ `internal/harnessshell.Model` accepts `tea.WindowSizeMsg`,
  `tea.KeyMsg`, `tea.MouseMsg`, all 10 `HostEvent` types
- ✅ Emits 4 of 5 outbound actions (`SubmitTurnAction`,
  `InterruptRunAction`, `ResolvePermissionAction`,
  `LoadPreviewAction`). `RunHostCommandAction` still pending — wires
  up when host-native command routing lands during Stage E (no
  shell-native commands besides `/clear` exist yet)
- ✅ Single envelope `ActionMsg{Action Action}` per WU-098 deferred
  decision
- ✅ Optimistic transcript rendering per WU-098 §"Optimistic
  transcript rendering"
- ✅ FEAT-0014 invariants preserved by the shell-owned pipeline:
  composer-stays-tail-mounted, single-scroll surface, queue FIFO,
  RunCompleted auto-releases queue, RunStopped does not, mid-stream
  permission pause UI gating (runtime-side pause/resume now belongs
  to the host adapter per WU-099)
- ✅ Spike tests still pass; spike continues to drive its own
  pipeline. The shell-owned pipeline is parallel and dormant until
  Stage D wires it through `internal/harnesshost`

## Open items

- CLI entrypoint rename for `modeltap harness-spike` is a Phase 3
  implementation decision; final command name TBD.
- Demo runtime package `internal/harnessdemo` does not yet exist; will be
  introduced during WU-100 Stage 6 / Stage E.
- Phase 2 disposition tables in `docs/releases/v0.2.1/.reviews/` should be
  re-checked once implementation reveals any unforeseen issues; the
  `revise the design explicitly` rule from `.agents/process.md` applies.
