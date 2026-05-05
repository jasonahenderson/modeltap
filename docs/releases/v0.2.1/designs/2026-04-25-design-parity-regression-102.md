# 2026-04-25 — Design: Parity and Regression Verification (WU-102)

## Scope

This design covers **WU-102 only**:

- parity and regression verification strategy for the extracted shell
- test-layer split between reusable shell tests, host adapter tests, and
  cutover/integration coverage
- migration of the current spike behavior oracle into the post-extraction test
  layout
- explicit mapping from `FEAT-0014` success criteria and `harnessspike`
  behavior tests to the extracted package structure

This design does **not** implement the tests themselves, and it does not
redefine the shell boundary or extraction plan from `WU-098` through `WU-100`.

## Context

`WU-102` intentionally runs after `WU-100`. The current spike behavior is the
parity oracle, so the regression sweep is written against the extracted shell
boundary rather than as a standard tester-first red phase.

The verification input set is:

- `FEAT-0014` success criteria
- `PATCH-0015` extraction invariants
- `WU-097` parity rules
- `WU-098` queue, permission, token, and scroll invariants
- `WU-099` host adapter responsibilities
- `WU-100` extraction guardrails and cutover sequence
- `internal/harnessspike/app_test.go` as the executable behavior inventory

## Goals

1. Prove that the extracted shell preserves `FEAT-0014` behavior.
2. Relocate spike-local behavior checks into the correct post-extraction test
   layers instead of leaving parity trapped in `internal/harnessspike`.
3. Catch regressions in queue, permission, token, interrupt, and scroll
   behavior during and after cutover.
4. Verify the adapter boundary without re-coupling runtime concerns back into
   the reusable shell package.
5. Leave a verification structure that remains useful after `internal/harnessspike`
   stops being the canonical shell implementation.

## Non-Goals

- full end-to-end verification of unrelated later harness surfaces such as
  session explorer, model catalog, or MCP
- adding new behavior beyond the accepted shell contract
- preserving spike-only demo chrome that is outside `FEAT-0014`

## Verification Model

Post-extraction verification should be split into three layers.

### Layer 1: Reusable shell unit/parity tests

Target package:

- `internal/harnessshell`

Purpose:

- verify shell-owned behavior without modeltap runtime dependencies
- treat host events and shell actions as the only external seam

This layer owns:

- transcript/composer surface behavior
- queue release and merge semantics
- token creation, expansion, and preview intent behavior
- permission UI navigation and decision emission
- interrupt arming and stop-state behavior
- focus and scroll preservation behavior
- command-history behavior that is shell-local

### Layer 2: Host adapter tests

Target package:

- `internal/harnesshost`

Purpose:

- verify action consumption and host-event projection against modeltap-facing
  services
- ensure the adapter preserves the shell contract while reusing later harness
  runtime inventory

This layer owns:

- submit action to runtime request mapping
- stream/runtime events projected back into shell host events
- permission request/decision routing
- preview/file-load routing
- host-native slash-command routing
- correlation between shell IDs and runtime IDs

### Layer 3: Cutover and integration tests

Target packages:

- `internal/harnesshost` integration tests, or
- a narrow top-level harness integration test if that yields clearer
  ownership

Purpose:

- verify that modeltap's composition of `internal/harnessshell` +
  `internal/harnesshost` (+ `internal/harnessdemo` for the demo CLI) wires
  correctly end-to-end
- ensure the extracted shell is the canonical implementation and no spike-
  shaped surface remains as a behavior authority

This layer does **not** live in `internal/harnessspike` — that package is
deleted as part of WU-100 Stage E. Layer 3 should stay small; its job is
not to duplicate Layer 1 and Layer 2 coverage, but to confirm the
composition path is wired correctly.

## Oracle Migration Plan

The current `internal/harnessspike/app_test.go` file is the migration checklist.
`WU-102` should redistribute its assertions rather than deleting them blindly.

### Tests that move to `internal/harnessshell`

These should become shell package parity tests:

- composer rendered inside transcript surface
- submit keeps input focus
- `alt+enter` and `Ctrl+J` newline handling
- mouse scroll does not steal input focus
- queue visibility during streaming
- queue merge/release FIFO order
- empty submit releases queue when idle
- interrupt arming and stop behavior
- transcript token selection and inline expansion
- submitted paste starts expanded
- file tokens remain compact and previewable on demand
- slash commands do not become file tokens
- scroll offset preservation while scrolled up
- history recall and draft restore
- permission request visible in transcript + composer
- composer approval/deny behavior
- multiple pending permission navigation
- repeated session approval still shows visible permission UI
- mid-stream permission pause and resume behavior

### Tests that move to `internal/harnesshost`

These should become host adapter tests:

- submit action produces the correct runtime request shape
- submit acceptance/failure maps back into shell host events
- stream delta and completion projection from runtime inventory
- interrupt requests map to runtime stop/cancel path
- preview requests call host preview services instead of shell-local file IO
- permission decisions map to runtime policy application
- host-native command actions route to the correct underlying services

### Tests that move to host adapter / top-level integration

After Stage E these become `internal/harnesshost` integration tests (or
focused top-level harness integration tests):

- the demo CLI program (built on `internal/harnessshell` + `internal/harnessdemo`)
  launches and renders a shell-backed program correctly
- `internal/harnessdemo` can drive the extracted shell with synthetic events
  for examples and parity fixtures
- no duplicate shell logic survives anywhere outside `internal/harnessshell`
- nothing in the repo imports a path containing "harnessspike"

## Required Parity Coverage Areas

### 1. Transcript and composer surface

Required assertions:

- transcript and composer remain one scrolling surface
- composer remains tail-mounted, not fixed
- transcript lines wrap within viewport width
- **incremental `RunDeltaEvent` output wraps within viewport width during
  active streaming** (not just static transcript lines)
- submitted user rows and queued rows preserve the shared `▎` marker model

Primary sources:

- `FEAT-0014` success criteria 1 and 2
- existing spike transcript/composer rendering tests

### 2. Scroll and focus behavior

Required assertions:

- manual scroll position is preserved when not following tail
- preserving input focus does not force follow-tail scrolling
- mouse/touchpad scroll does not steal composer focus

Primary sources:

- `FEAT-0014` success criterion 3
- `app_test.go` scroll and focus tests

### 3. Queue behavior

Required assertions:

- submit while streaming queues follow-up work
- queued work stays visible
- queue merge preserves FIFO order
- multi-item queue release preserves FIFO across the
  `pendingSubmissions` merge buffer (per WU-098 queue invariants)
- empty `Enter` while idle releases queued work
- interrupted runs do not auto-release queued work
- normal completion does auto-release queued work
- **end-to-end queue+interrupt sequence**: queue a follow-up during an active
  stream, interrupt the stream, then idle-empty-`Enter` releases the queued
  work (matches FEAT-0014 success criterion 4 verbatim)

Primary sources:

- `FEAT-0014` success criterion 4
- queue tests in `app_test.go`
- queue invariants in `WU-098`

### 4. Interrupt behavior

Required assertions:

- first `Esc` arms interrupt without stopping the run
- second `Esc` stops the active stream
- stopped run remains visible
- queued work remains queued after interrupt

Primary sources:

- `FEAT-0014` stop behavior section
- `app_test.go` interrupt tests

### 5. Token and preview behavior

Required assertions:

- large pastes compact into tokens before submit
- submitted paste tokens start expanded inline
- transcript `Enter` toggles paste-token expansion inline
- file/reference tokens stay compact and request preview on demand
- slash commands are not reclassified as file tokens

Primary sources:

- `FEAT-0014` submitted artifact model
- token tests in `app_test.go`

### 6. Permission behavior

Required assertions:

- permission requests remain visible in transcript history
- active permission controls live in the composer
- `Left` / `Right` switch actions
- `Enter` applies the selected action
- `y` / `n` only work when the composer buffer is empty
- multiple pending permissions are navigable with `Up` / `Down`
- repeated session-approved tools still surface a visible request
- mid-stream permission requests pause the active stream and resume or end
  cleanly

#### Mid-stream permission verification approach

Mid-stream pause/resume is now adapter-driven (see WU-099 Mid-stream pause and
stream buffering). A pure shell unit test cannot trigger the pause without a
fake host. WU-102 therefore requires:

- the test fake host must support **injecting `PermissionRequestedEvent`
  while `RunDeltaEvent` deltas are actively in-flight**, so Layer 1 tests can
  verify shell pause/render/resume state transitions
- Layer 1 tests assert the shell pauses delta application and renders
  composer permission controls
- Layer 2 (host adapter) tests assert the adapter actually buffers
  `RunDeltaEvent` forwarding while a permission is pending and replays the
  buffer in arrival order on resolution

Primary sources:

- `FEAT-0014` success criteria 5, 6, and 7
- permission tests in `app_test.go`
- permission invariants in `WU-098`
- adapter pause/resume contract in `WU-099`

### 7. Host integration contract

Required assertions:

- shell emits actions instead of invoking runtime effects directly
- adapter translates actions into runtime/service calls
- runtime/service outputs project back into host events without leaking
  transport-specific shapes into the shell
- preview and permission lifecycles remain host-owned across the boundary

Primary sources:

- `PATCH-0015`
- `WU-098`
- `WU-099`

## Test Fake Host Capability Matrix

The shell unit tests in Layer 1 depend on a fake host that can drive every
shell-visible behavior the action/event boundary supports. WU-102's fake
host must implement at least:

| Capability | Why it's needed |
|---|---|
| Acknowledge submissions (`SubmissionAcceptedEvent`) | Verifies optimistic assistant-row reconciliation |
| Reject submissions (`SubmissionFailedEvent`) | Verifies placeholder removal and failure rendering |
| Start runs (`RunStartedEvent`) | Drives transition from submitted to streaming state |
| Emit stream deltas (`RunDeltaEvent`) | Drives streaming-output wrap and follow-tail behavior |
| Complete runs (`RunCompletedEvent`) | Drives queue auto-release path |
| Stop runs (`RunStoppedEvent`) | Verifies interrupt-driven non-release of queue |
| Fail runs (`RunFailedEvent`) | Verifies failure rendering without retry semantics |
| Inject permissions mid-stream (`PermissionRequestedEvent`) | Required for FEAT-0014 success criterion 7 |
| Resolve permissions (`PermissionResolvedEvent`) | Drives composer-controls clear and assistant-row update |
| Provide preview payloads (`PreviewLoadedEvent`) | Drives file-token preview rendering |
| Update host status (`HostStatusEvent` with `Kind`) | Drives chrome-state assertions |

Capability anti-pattern: the fake host must not reach into shell-internal
state to assert behavior. Tests should assert via shell action emissions and
rendered view output only.

## Test Layout Proposal

### Reusable shell tests

Suggested files:

- `internal/harnessshell/model_test.go`
  construction defaults, transcript/composer surface, focus
- `internal/harnessshell/queue_test.go`
  queue submit/release/merge behavior
- `internal/harnessshell/tokens_test.go`
  paste/file token behavior and preview intent
- `internal/harnessshell/permissions_test.go`
  permission navigation, decisions, repeated requests, mid-stream pause effects
- `internal/harnessshell/viewport_test.go`
  scroll preservation and wrapping behavior

### Host adapter tests

Suggested files:

- `internal/harnesshost/submit_test.go`
- `internal/harnesshost/commands_test.go`
- `internal/harnesshost/preview_test.go`
- `internal/harnesshost/permissions_test.go`
- `internal/harnesshost/projection_test.go`

### Cutover / integration tests

Suggested files:

- `internal/harnesshost/integration_test.go`
  end-to-end composition checks for `internal/harnessshell` +
  `internal/harnesshost`
- `internal/harnessdemo/demo_test.go` (if needed)
  end-to-end composition checks for the demo CLI's shell + demo runtime
  composition

The deleted `internal/harnessspike/app_test.go` does not survive — its
assertions are redistributed into Layer 1 (shell), Layer 2 (host adapter),
and Layer 3 (integration) per the migration plan.

## Verification Sequence

`WU-102` implementation should proceed in this order:

1. inventory and label existing spike tests by target layer
2. port shell-owned tests into `internal/harnessshell`
3. add host adapter tests for action/event and runtime projection behavior
4. add `internal/harnesshost` (and if needed `internal/harnessdemo`)
   integration tests for the small set of cutover / composition checks
5. delete `internal/harnessspike/app_test.go` (and the rest of the
   `harnessspike` package) at the same time as WU-100 Stage E
6. run the parity sweep against the extracted package structure
7. confirm that every `FEAT-0014` success criterion has at least one direct
   automated assertion

### Test compatibility during cutover

WU-100's "Test compatibility during cutover" rule is binding for WU-102:

- the spike `app_test.go` is treated as a migration checklist, not as a
  continuously-passing suite during cutover stages
- new shell-package tests must be passing before any spike test is deleted
- there are no type aliases bridging spike test names to shell-package names
- the spike test file is allowed to be compile-broken on a per-stage basis

WU-102's verification job is to confirm that, by Stage E completion, the
`internal/harnessspike` package and its test file are deleted, and that all
migrated assertions are passing in their new shell-package, host-adapter,
or host-integration homes.

## Regression Gates

Before Phase 1 is considered complete at the design layer, the verification
design must define these release gates for Phase 3 implementation:

- no spike behavior invariant may be dropped without an explicit design reason
- `FEAT-0014` success criteria must map to automated tests
- no test may validate shell behavior by reaching through modeltap runtime
  internals when the shell boundary can be exercised directly
- no adapter test may reintroduce callback-shaped coupling as a convenience

## Risks And Mitigations

### Risk 1 — parity remains trapped in spike-only tests

Cause:

- leaving `internal/harnessspike/app_test.go` as the only thorough behavior
  oracle after extraction

Mitigation:

- redistribute tests by ownership layer (Layer 1 shell, Layer 2 host adapter,
  Layer 3 host-integration)
- delete `internal/harnessspike` and its test file once redistribution is
  complete; do not leave a residual spike test suite

### Risk 2 — host adapter regressions are missed

Cause:

- porting only UI tests and assuming runtime projection is covered elsewhere

Mitigation:

- add explicit host adapter tests for action consumption and event projection

### Risk 3 — success criteria drift from automated coverage

Cause:

- porting tests mechanically without mapping back to `FEAT-0014`

Mitigation:

- treat `FEAT-0014` success criteria as the canonical coverage checklist

## Acceptance Criteria

`WU-102` is complete when:

1. the post-extraction test layout is explicit by package/layer
2. `internal/harnessspike/app_test.go` has been treated as a migration
   checklist rather than an incidental test file
3. queue, permission, token, interrupt, transcript, and scroll behavior all
   have designated parity coverage targets
4. host adapter action/event behavior has designated regression coverage targets
5. every `FEAT-0014` success criterion maps to automated verification
