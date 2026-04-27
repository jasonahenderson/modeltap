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
| 100 | Behavior-Preserving Shell Extraction Implementation | L | In progress | Stages A+B+C+E done; Stage D adapter complete (D-1 `07528ba`, D-2 `b03604a`, D-3 `caf93b7`); Stage D-4 production wiring (`internal/harness/app.go` integration) is the only remaining WU-100 work |
| 101 | Developer Documentation and Embedding Examples | M | Done | Structural pass `a40e4b9`; reconciliation pass `23dff2d` |
| 102 | Parity and Regression Test Sweep | M | Done | Layer 1 / Layer 2 / Layer 3 tests landed (`1f1e5b9`); FEAT-0014 success criteria SC1, SC2, SC4–SC7 covered. SC3 (manual scroll preservation) flagged as a follow-up gap |

## Phase 3 dependency graph

```
WU-100 (extraction)  ──→  WU-102 (parity tests)  ✅
       ║                                          ✅
       ║  parallel                                ✅
       ║                                          ✅
WU-101 (docs)                                     ✅
```

WU-097, WU-098, WU-099 produced design docs in Phase 1; their Phase 3
deliverables are already present. WU-100 Stages A through E and the
Stage D adapter (D-1 / D-2 / D-3) are complete; only Stage D-4 remains.

## Up next

- **WU-100 Stage D-4** — production wiring. Implement a concrete
  `Runtime` backed by `internal/harness.ConnSurface`,
  `ToolDispatcher`, `ContextManager`, and the permission enforcer.
  Modify `internal/harness/app.go` to embed the conversation surface
  through `harnesshost.Adapter` instead of (or in parallel with) the
  existing direct flow.

  Two design questions need to be resolved before implementation:

  1. **App composition.** The existing `internal/harness/app.go` has
     its own `tea.Model` with statusbar, multi-line input, viewport,
     markdown wrapper, etc. Wrapping that App in `harnesshost.Adapter`
     conflicts with the FEAT-0014 single-scrolling-surface invariant
     (the shell wants to own the conversation surface end-to-end).
     Options: replace App's conversation surface with `harnessshell.
     Model` + Adapter (preserving sidebar/statusbar around it), or
     stand up a parallel entrypoint that uses Adapter natively while
     the legacy path runs alongside through one release.
  2. **`Runtime.SubmitTurn` async/sync bridge.** The harness's
     existing flow dispatches via `ConnSurface.SubmitTurn` (returns
     immediately) and the response arrives later as `TurnSubmittedMsg`.
     `harnesshost.Runtime.SubmitTurn` is sync — it returns
     `(SubmitAccepted, error)`. Two viable approaches: dispatch async
     and return a placeholder `RunID` (relying on the projection
     layer's `TurnSubmittedMsg → SubmissionAcceptedEvent` translation
     to deliver the real correlation later), or block inside the
     `tea.Cmd` goroutine until `TurnSubmittedMsg` arrives.

  These are architectural decisions worth a small TPM design pass
  before any code change.

## In progress

(none — Stage D-4 is queued for dispatch)

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
- WU-100 Stage E-1 (`f80661a`) — `internal/harnessdemo` package with
  `FakeRuntime` (implements `harnesshost.Runtime`) and `Driver`
  (`tea.Model` orchestrator that emits fake stream tokens). Pure
  additive; no spike changes.
- WU-100 Stage E-2 (`f8aa1cc`) — `modeltap shell-demo` CLI command.
  Constructs `harnessshell` + `harnessdemo` + `harnesshost.Adapter` as
  a thin demo client; runs alongside the legacy `harness-spike` for
  side-by-side comparison.
- WU-100 Stage E-3 (`88e91ba`) — deleted `internal/harnessspike`
  (~3,400 lines) and the `harness-spike` CLI command. Post-extraction
  architecture is now the only home for conversation-shell behaviors.
- WU-101 structural pass (`a40e4b9`) — `internal/harnessshell/README.md`,
  `internal/harnesshost/README.md`, `docs/guides/harness-shell-embedding.md`
  with 85 provisional placeholders.
- WU-101 reconciliation (`23dff2d`) — final names sweep. All 85
  placeholders resolved against the implementation; embedding example,
  flow walkthroughs, and final-names table updated to match the
  shipped `harnessshell` / `harnesshost` / `harnessdemo` API.
- WU-102 (`1f1e5b9`) — parity and regression test sweep. Closed one
  shell-side gap (transcript Enter toggles paste-token expansion / file
  preview); added Layer 1 parity tests in
  `internal/harnessshell/queue_test.go` and
  `internal/harnessshell/tokens_test.go`; added Layer 3 integration
  tests in `internal/harnesshost/integration_test.go`. Layer 2 host
  adapter tests already covered by Stage D commits. FEAT-0014 success
  criteria SC1, SC2, SC4–SC7 have direct automated assertions; SC3
  (manual scroll preservation) is flagged as the only remaining
  coverage gap and would benefit from a viewport-state accessor.

## WU-100 done condition

The Stage extraction design lists 6 done-condition items. After Stages
A through E:

- ✅ `internal/harnessshell` is the canonical home of the extracted
  shell behavior
- ✅ `internal/harnesshost` owns modeltap-specific integration (sans
  the production `Runtime` impl, which is Stage D-4)
- ✅ `internal/harnessdemo` owns fake/demo runtime behavior
- ✅ `internal/harnessspike` has been deleted
- ✅ Listed FEAT-0014 invariants are preserved by the extracted
  pipeline (verified in Stages C-3 through C-6 + the host adapter
  pause buffer in Stage D-3 + WU-102 parity tests)
- ✅ Concrete enough that WU-102 has written parity and regression
  coverage against the new package structure without redesign

The done condition is structurally satisfied. Stage D-4 production
wiring is the operational gate before tagging v0.2.1 — the modeltap
production harness must actually use `harnesshost.Adapter` for the
release to deliver user-facing benefit beyond the `shell-demo` CLI.

## Open items

- **Stage D-4 architectural decisions** (App composition + async/sync
  bridge) noted under "Up next" above.
- **WU-102 SC3 follow-up.** Manual scroll preservation has no direct
  automated assertion because the viewport state lives inside
  `Model.View`'s local copy. A test-only accessor or a `View() (string,
  ViewportState)` variant would close the gap.
- **Branch retarget.** `spike/scrolling-surface-eval` should retarget
  to a release branch before tagging — pending TPM decision.
- **Post-release.** `internal/harnessshell` is repo-internal during
  v0.2.1 but the boundary is clean enough for future repository
  extraction per PATCH-0015 §"Separation Requirements For Future
  Extraction".
