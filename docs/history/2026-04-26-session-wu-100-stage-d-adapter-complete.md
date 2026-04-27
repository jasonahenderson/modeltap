# 2026-04-26 — Session: WU-100 Stage D adapter complete (D-1 through D-3)

## Scope

Continuation of the same long session that landed Stage C. Built out
`internal/harnesshost` to the point where it is a complete, well-tested
bridge between the reusable shell and any `Runtime` implementation.
Production wiring against `internal/harness/app_conn.ConnSurface`
(Stage D-4) is the only remaining Stage D step.

## Commits landed

```
07528ba WU-100: Stage D-1 — internal/harnesshost adapter and Runtime contract
b03604a WU-100: Stage D-2 — runtime message → HostEvent projection
caf93b7 WU-100: Stage D-3 — mid-stream pause buffer
```

## Capability per commit

### Stage D-1 (`07528ba`) — action-consumer half

- `Runtime` interface per WU-099: SubmitTurn / InterruptRun /
  DispatchCommand / ResolvePermission / LoadPreview / SummarizePaste.
  Intentionally narrower than `ConnProtocolClient` and intentionally
  omits PauseRun / ResumeRun (mid-stream pause is adapter-internal)
- Supporting types: `SubmitRequest`, `SubmitAccepted`, `Attachment`,
  `HostCommand`, `PreviewRequest`. `Attachment` carries adapter-
  resolved data; the runtime never sees raw shell tokens
- `Adapter` is a `tea.Model` that wraps `harnessshell.Model` as a
  decorator. Dispatches every shell Action to a Runtime call via
  `tea.Cmd`; success path produces an internal correlation msg
  (SubmitTurn) or `nil` (others); error paths produce the documented
  failure HostEvent (e.g., `SubmissionFailedEvent`,
  `RunFailedEvent`, `HostStatusEvent{Kind: StatusError}`)
- `Option` pattern: `WithAttachmentResolver`, `WithContextSource`. No
  callback hooks per WU-098
- Correlation tables `submissionToRun` / `runToSubmission` populated
  on submit-accepted

### Stage D-2 (`b03604a`) — event projection

- `projectRuntimeMessage(msg)` returns the corresponding `HostEvent`
  (or nil for messages that stay host-App-owned)
- All relevant runtime msgs from `internal/harness/messages.go` covered
- Multi-model branches flatten into per-branch `Run*Event`s with
  `RunID = "<TurnID>:<BranchID>"` so the FEAT-0014 single-transcript
  invariant holds without losing branch identity
- Tool activity events project to `HostStatusEvent` with phase-aware
  glyphs (⚙ start, ✓/✗/⊘/• end)
- Permission prompts project to `PermissionRequestedEvent` with a
  conservatively-truncated target description from the input JSON
- Adapter.Update gains a `projectRuntimeMessage` call before the
  shell-passthrough path

### Stage D-3 (`caf93b7`) — mid-stream pause buffer

- Adapter state: `pendingPermissions map[string]struct{}` and
  `pauseBuffer []RunDeltaEvent`
- `forwardEvent` helper handles all HostEvents flowing into the
  shell, applying the pause/buffer logic:
  - PermissionRequestedEvent: register in pending set, forward
  - RunDeltaEvent: buffer if pending set non-empty, otherwise forward
  - PermissionResolvedEvent: remove from pending; when set drains
    to empty, forward the resolve then replay buffered deltas in
    arrival order
- Multi-permission case: buffer drains only when ALL pending resolve
- Non-RunDelta events (status, preview, etc.) flow through during a
  pause so chrome updates stay live

## End-state of harnesshost

After Stage D-3, `internal/harnesshost.Adapter` is feature-complete
for any `Runtime` implementation:

- **Inputs accepted:** every Bubble Tea message the shell handles,
  every `internal/harness` runtime msg the projection covers, plus
  direct `HostEvent` injection for tests / synthetic events
- **Outputs:** shell `Action`s flow to `Runtime` calls; runtime
  responses + projected runtime msgs flow back to the shell as
  typed `HostEvent`s; mid-stream pause buffering is transparent to
  both the shell and the Runtime impl
- **Correlation:** SubmissionID ↔ RunID tables maintained inside
  the adapter; the Runtime impl provides RunIDs, the shell sees only
  stable run IDs
- **Tests:** 19 tests across `adapter_test.go`, `projection_test.go`,
  `pause_test.go`. The shell-unit tests in `internal/harnessshell`
  still all pass. The spike tests still all pass.

## What's NOT in Stage D yet

- **Stage D-4 production wiring** — concrete `Runtime` impl backed
  by the modeltap `ConnSurface` / `ToolDispatcher` / `ContextManager`
  / `PermissionEnforcer`. The `Runtime.SubmitTurn` async/sync bridge
  needs a small design call: existing harness flow dispatches via
  `ConnSurface.SubmitTurn` (returns immediately) and the response
  arrives later as `TurnSubmittedMsg`. Either dispatch-and-return with
  a placeholder RunID + rely on the projection layer's
  `TurnSubmittedMsg → SubmissionAcceptedEvent` translation, or block
  inside the `tea.Cmd` goroutine until `TurnSubmittedMsg` arrives.
- The existing `internal/harness/app.go` is unchanged — Stage D-4
  decides whether to wrap it in `harnesshost.Adapter` or build a
  parallel entrypoint that uses `Adapter` natively while the legacy
  path runs alongside through one release.

## Open items

- See `docs/releases/v0.2.1/status.md` "Up next" for the remaining
  stages (D-4, E, WU-101 reconciliation, WU-102 parity sweep) and
  their inter-dependencies.
- Branch retarget for `spike/scrolling-surface-eval` still pending
  TPM decision.

## Cumulative progress in this long session

```
19a546b WU-100: Stage C scaffolding — shell-state helpers
d106fc6 WU-100: Stage C wire-up — Model API for shell-local keys
056d7c6 ADMIN: log WU-100 Stage C checkpoint and update v0.2.1 status
89571e4 WU-100: Stage C-3 — submit emission and run-lifecycle event intake
8368e7c WU-100: Stage C-4 — interrupt action emission
6115549 WU-100: Stage C-5 — permission action emission and intake
661dc25 WU-100: Stage C-6 — preview, status events, and Esc precedence
e331ae7 ADMIN: log WU-100 Stage C completion and update v0.2.1 status
07528ba WU-100: Stage D-1 — internal/harnesshost adapter and Runtime contract
b03604a WU-100: Stage D-2 — runtime message → HostEvent projection
caf93b7 WU-100: Stage D-3 — mid-stream pause buffer
```

11 implementation commits + 2 admin commits. Net: `internal/harnessshell`
went from "Stage A skeleton" to "feature-complete reusable shell";
`internal/harnesshost` went from "stub README" to "feature-complete
adapter (sans production runtime impl)".
