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
| 100 | Behavior-Preserving Shell Extraction Implementation | L | In progress | Stages A+B+C done (`1cb1eb4`, `1e32b57`, `19a546b`, `d106fc6`, `89571e4`, `8368e7c`, `6115549`, `661dc25`); Model owns full action emission + every HostEvent intake. Stage D (`internal/harnesshost`) and Stage E (delete spike + `internal/harnessdemo` + CLI rename) ahead |
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

- **WU-100 Stage D** — introduce `internal/harnesshost`. Build the
  `Runtime` interface per WU-099, the action-consumer side of the adapter
  (drains `ActionMsg`, calls runtime ops), and the event-projection side
  (translates runtime messages into `HostEvent`s). Implement the
  mid-stream pause buffer per FEAT-0014. Wrap the existing
  `internal/harness/app_conn.ConnSurface` as a concrete `Runtime`
  implementation and wire the production harness path to use the new
  adapter.
- **WU-100 Stage E** — extract `internal/harnessdemo` (fake replies,
  presets, /perm demo, stream timing). Delete `internal/harnessspike`
  entirely. Replace/rename the `modeltap harness-spike` CLI entrypoint
  with a thin client of `harnessshell` + `harnessdemo`.
- **WU-101 reconciliation** — fill in the 57 placeholders in the
  embedding docs against the post-extraction architecture.
- **WU-102 parity sweep** — migrate spike test coverage into shell-unit
  and host-integration test layouts per WU-102 design.

## In progress

(none — Stage D is queued for dispatch)

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
- WU-101 structural pass (`a40e4b9`) — `internal/harnessshell/README.md`,
  `internal/harnesshost/README.md`, `docs/guides/harness-shell-embedding.md`
  with 57 provisional placeholders to be reconciled after WU-100 cutover.

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
