# 2026-04-25 — Design: Behavior-Preserving Shell Extraction Implementation (WU-100)

## Scope

This design covers **WU-100 only**:

- implementation sequencing for extracting the current shell from
  `internal/harnessspike/`
- package and file movement into reusable shell and modeltap host layers
- intermediate compatibility stages during the extraction
- cutover strategy from spike-owned shell logic to the extracted packages
- explicit invariants to preserve during implementation
- rollback and risk controls for the extraction work

This design does **not** redefine:

- the shell boundary from `WU-098`
- the modeltap host adapter contract from `WU-099`
- shell UX behavior already accepted in `FEAT-0014`

## Purpose

Implement the extraction of the current shell behavior from
`internal/harnessspike/` into a reusable shell package plus a modeltap-specific
host adapter, without changing the user-visible `FEAT-0014` interaction
contract.

## Source Baseline

The current extraction baseline is the `internal/harnessspike` package:

- `app.go` contains the state machine, Bubble Tea update loop, rendering, fake
  runtime behavior, token handling, queue handling, and permission demo logic
- `styles.go` contains shell styling constants
- `app_test.go` captures the current behavior contract and is the best
  executable inventory of parity requirements

The implementation must treat the tested spike behavior as the extraction
oracle until `WU-102` replaces the spike-local tests with the new package-level
and integration-level test layout.

## Extraction Outcome

At the end of `WU-100`, the codebase should have three distinct layers and
**no `internal/harnessspike` package**:

1. `internal/harnessshell`
   Owns reusable shell state, Bubble Tea interaction rules, transcript/composer
   rendering, queue UI behavior, token display behavior, permission UI state,
   and typed shell actions / host events.
2. `internal/harnesshost`
   Owns modeltap-specific integration between `internal/harnessshell` and the
   actual harness/runtime surfaces described in `WU-099`.
3. `internal/harnessdemo`
   Owns the fake/demo runtime that drives the shell with synthetic events for
   examples and test fixtures. This is the only post-extraction home for
   shell-with-fake-data behavior.

`internal/harnessspike` is deleted as part of this WU (Stage E). Its current
roles split:

- canonical shell logic → `internal/harnessshell`
- fake reply generation, session presets, demo commands → `internal/harnessdemo`
- the existing CLI demo entrypoint will be replaced/renamed during cutover;
  the final command name is a Phase 3 implementation detail flagged in
  `FEAT-0014`'s CLI section
- cutover-only tests → `internal/harnesshost` integration tests per WU-102

Per `WU-099`, fake/demo runtime behavior must not remain embedded in the
reusable shell package. After Stage E the repo contains no package called
"spike."

## Behavior Invariants To Preserve

The extraction is successful only if these remain true:

- transcript and composer remain one scrolling surface
- composer stays tail-mounted rather than becoming a fixed footer
- input focus remains after submit
- mouse/touchpad transcript scrolling does not steal composer focus
- manual scroll offset is preserved when the user is not following tail
- empty `Enter` while idle releases queued work
- queued work stays FIFO
- queued work auto-releases only after normal completion, not after interrupt
- first `Esc` arms interrupt; second `Esc` stops the active stream
- stop does not auto-resume the stopped run
- large pasted content is compacted into tokens before submit
- submitted paste tokens render inline and start expanded in the transcript
- file references stay as compact tokens and preview on demand
- slash commands do not get misclassified as file tokens
- single-line history recall behavior on `Up` / `Down` remains intact
- permission requests remain visible as transcript events
- active permission controls remain in the composer
- repeated session-approved tools still surface a visible request row with
  remembered policy state
- multiple pending permissions remain navigable in the composer
- mid-stream permission requests pause the active response immediately and
  resume only after approval

These invariants are derived from `FEAT-0014`, `PATCH-0015`, and the existing
`internal/harnessspike/app_test.go`.

## Package And File Movement Plan

### New reusable package

Create `internal/harnessshell/` as the canonical implementation package.

Expected file split:

- `model.go`
  Core shell model, state structs, typed actions/events, and ownership-safe
  public types
- `update.go`
  Bubble Tea update handling, key routing, action emission, host-event intake,
  queue handling, and permission state transitions
- `view.go`
  Transcript/composer/sidebar/overlay rendering and transcript ref selection
- `tokens.go`
  paste/file token normalization, token summaries, token selection, preview
  request generation
- `queue.go`
  queued submission merge/release logic and helper invariants
- `permissions.go`
  pending permission navigation, action selection, and permission lifecycle
- `styles.go`
  reusable visual style definitions that are shell-owned rather than
  modeltap-host-specific

The exact file count may vary, but the split must separate model/update/view
concerns well enough that host integration changes do not require reopening the
rendering and queue logic.

### New modeltap host package

Create `internal/harnesshost/` per `WU-099`.

Expected file split:

- `adapter.go`
  wiring between runtime-facing modeltap services and shell actions/events
- `commands.go`
  host-native command routing and shell-native/host-native split enforcement
- `runtime_events.go`
  mapping of runtime events into shell host events
- `submission.go`
  submit/interrupt/permission/preview orchestration against modeltap runtime

### Spike package deletion

`internal/harnessspike/` is deleted at the end of this WU. There is no
"narrowed" or "compatibility" variant in the final tree.

Expected end state:

- `internal/harnessspike/` does not exist
- fake/demo behavior (presets, fake replies, demo commands, fake stream
  generation) lives in `internal/harnessdemo`
- the demo CLI entrypoint imports `internal/harnessshell` + `internal/harnessdemo`
  directly; the entrypoint name is a Phase 3 implementation detail
- cutover-only tests live in `internal/harnesshost` integration tests per WU-102
- spike test file is removed at the same time as the spike package; no aliases,
  no forwards

### Movement rules

- do not move modeltap runtime logic into `internal/harnessshell`
- do not move fake reply generation into `internal/harnessshell`
- do not keep callback-valued API seams between shell and host
- do not preserve `internal/harnessspike` past Stage E for any reason — if a
  capability needs to survive, it lives in `internal/harnessdemo`,
  `internal/harnesshost`, or `internal/harnessshell` (whichever owns it)

## Extraction Sequence

Implementation should proceed in these ordered steps.

### Step 1: Establish reusable shell types without changing behavior

- introduce `internal/harnessshell` with the reusable state and type aliases
  that mirror the current spike structures:
  - message/transcript row model
  - token model
  - queued submission model
  - pending permission model
  - transcript selection refs
- rename or reshape internal types only where needed to align with `WU-098`
  typed action/event terminology
- keep semantics unchanged during this step

Exit criteria:

- shell-owned state exists in the new package
- no modeltap runtime or fake runtime logic is required to compile these types

### Step 2: Move pure shell rendering and layout

- move layout calculation and rendering code from `internal/harnessspike` into
  `internal/harnessshell`
- keep the single scrolling transcript/composer surface intact
- preserve current style-driven visual grouping and transcript token rendering

#### Definite scope rule for the reusable package

`internal/harnessshell` may contain only the conversation-surface chrome that
FEAT-0014 defines:

- transcript and queued-row rendering
- composer rendering (including permission controls hosted in the composer)
- shell-local preview dialog for paste-token payloads
- token rendering (paste and file/reference)
- shell-local status/footer presentation

`internal/harnessshell` must **not** contain:

- sidebar, command palette, agent list, agent detail overlay
- session explorer, model catalog, history catalog surfaces
- any background-agent or multi-agent orchestration UI

Those surfaces are spike/demo or modeltap-specific app chrome. They stay in
the spike wrapper or the modeltap top-level harness package, not in the
reusable shell. FEAT-0014 narrows scope to the conversation shell; WU-099
keeps sidebar/session-explorer surfaces top-level. This rule is binding for
WU-100 and is the criterion WU-102 should use to flag scope creep.

#### Theme/style import boundary

Per WU-098, `internal/harnessshell` may not import `internal/harness/theme`
or any other modeltap-specific style constants. Move only theme-neutral
styles into the new `styles.go`. If the spike currently relies on theme
values, leave a temporary translation in the spike wrapper that maps theme
output into shell-local style configuration; the reusable package itself
must remain theme-agnostic.

#### Stage A → Stage B bridge

Step 2 (rendering cutover) depends on shell-owned state being available in
the new package. Stage A (type duplication) introduced the shell-state types
but did not yet move spike state into them. Step 2 must therefore include a
short translation pass: the spike's existing `App` state is projected into
the new shell state structs immediately before the new renderer is invoked.
This bridge is local to the spike package and can be deleted once Stage C
(action/event cutover) lands.

Without this bridge, Step 2 would either reach back into spike-local state
or duplicate state in two places. Both are explicitly disallowed.

Exit criteria:

- `View`, layout, transcript refresh, token rendering, and composer surface can
  run from `internal/harnessshell`
- scroll preservation behavior remains shell-local
- the in-scope chrome list above is the only chrome rendered from
  `internal/harnessshell`

### Step 3: Replace direct fake-runtime transitions with emitted shell actions

- refactor submit, interrupt, preview, and permission paths so the shell emits
  typed actions instead of directly mutating fake runtime state
- replace direct calls that currently start fake streaming or permission-demo
  work with action emission plus shell-side pending state
- ensure shell-native commands remain local:
  - `/clear`
  - queue release behavior
  - transcript-local expand/preview intent
- ensure host-native commands cross the boundary as actions per `WU-098` and
  `WU-099`

Exit criteria:

- `internal/harnessshell` no longer knows how modeltap or fake runtime work
- all external effects are represented as actions leaving the shell

### Step 4: Add host-event intake path

- introduce host event application methods in `internal/harnessshell`
- shell host-event intake must cover:
  - submission accepted / started
  - stream delta
  - run completed
  - run interrupted
  - run failed
  - permission requested
  - permission resolved
  - preview payload ready
- replace current direct stream tick completion logic with host-driven event
  handling, while preserving queue-release semantics

Compatibility rule:

- transient internal helper events are acceptable inside the shell package, but
  the shell/host boundary itself must remain typed action/event based

Exit criteria:

- the shell can be driven entirely by inbound host events plus local key input

### Step 5: Move queue and permission behavior intact

- move queue merge/release logic verbatim in behavior, not verbatim in file
  shape
- preserve the distinction between:
  - visible `queuedSubmissions`
  - transient `pendingSubmissions` merge buffer (per WU-098 queue invariants)
  - paused response state during mid-stream permission gating — but note that
    after extraction the runtime-side pause/resume is owned by the host
    adapter (WU-099), so the shell's residual responsibility is solely the
    UI gating, not the stream-queue replay
- move permission UI behavior as shell-owned state:
  - active permission selection
  - left/right action selection
  - `y`/`n` fallback when composer is empty
  - visible session-policy hinting
- leave production permission identity and runtime pause/resume orchestration in
  the host package (the adapter buffers `RunDeltaEvent` forwarding while a
  permission is pending, per WU-099 Mid-stream pause section)

Exit criteria:

- queue and permission semantics match the spike behavior without shell/runtime
  coupling

### Step 6: Introduce `internal/harnesshost`

- implement the modeltap-specific host adapter described by `WU-099`
- map shell actions onto modeltap runtime operations:
  - submit turn
  - interrupt run
  - route host-native commands
  - request preview/file inspection
  - originate permission requests and apply permission decisions
- map runtime outputs back into shell host events
- make the host adapter responsible for stable IDs, runtime handles, and any
  translation from later `v0.2.0` harness/runtime packages

Exit criteria:

- modeltap can drive the shell with no direct import from `internal/harnessshell`
  into runtime-specific packages

### Step 7: Delete `internal/harnessspike`

- move any remaining fake reply generation, session presets, and demo-only
  commands from the spike into `internal/harnessdemo`
- relocate the cutover-only tests identified by WU-102 to
  `internal/harnesshost` integration tests
- replace or rename the existing `modeltap harness-spike` CLI entrypoint;
  the demo CLI program becomes a thin client of `internal/harnessshell` +
  `internal/harnessdemo` with a name decided during Phase 3 implementation
- delete the `internal/harnessspike/` directory

Exit criteria:

- `internal/harnessspike` does not exist on disk
- the demo CLI capability is preserved under a new entrypoint that imports
  `internal/harnessshell` + `internal/harnessdemo` directly
- no test or runtime file imports a path containing "harnessspike"

## Intermediate Compatibility Steps

The implementation should not attempt a single-shot replacement. Use these
compatibility stages:

### Stage A: Type duplication with behavior lock

- introduce new shell types and translation helpers
- keep spike using its current implementation
- allow temporary adapters between old spike-local structs and new shell types
- this stage's translation helpers are what unblocks Stage B; the new
  renderer cannot be invoked without them

### Stage B: Rendering cutover

- switch transcript/composer rendering to the new shell package first
- the spike calls Stage A's translation helpers immediately before invoking
  the new renderer so the spike's `App` state is projected into the new
  shell state structs at each render
- keep submit/runtime behavior temporarily bridged if needed

Reason:

- rendering and scroll behavior are high-risk and easy to regress visually
- isolating them first makes later action/event cutover easier to inspect

Caveat:

- this is the order with the highest test-migration risk because the spike
  test file still tests against the old `App` type. See "Test compatibility
  during cutover" below.

### Stage C: Action/event cutover

- move submit/interrupt/permission/preview boundaries to typed shell actions
- keep the spike host as a translation shim while modeltap host integration is
  still landing

### Stage D: Host adapter cutover

- bind the shell to `internal/harnesshost`
- route all runtime effects through the host adapter

### Stage E: Spike package deletion

- migrate any surviving demo wiring (presets, fake replies, demo commands)
  from the spike into `internal/harnessdemo`
- delete `internal/harnessspike/` entirely once nothing imports it
- relocate Layer 3 cutover-only tests to `internal/harnesshost` per WU-102
- replace or rename the `modeltap harness-spike` CLI entrypoint; the new
  demo program imports `internal/harnessshell` + `internal/harnessdemo`
  directly

At each stage, the code must compile and the still-relevant parity tests must
continue passing.

### Test compatibility during cutover

The spike's `internal/harnessspike/app_test.go` tests the monolithic `App`
struct and references types (`queuedSubmission`, `inputToken`,
`pendingPermission`, etc.) that move into `internal/harnessshell` during this
WU. We deliberately do **not** keep type aliases in `internal/harnessspike`
that forward to the new package — that re-entangles the layers we are trying
to separate.

Test-migration rule:

1. Treat `internal/harnessspike/app_test.go` as a **migration checklist**, not
   as a continuously-passing test suite, during Stages B–D.
2. WU-102's new tests in `internal/harnessshell/*_test.go` must be passing
   before any spike test is deleted.
3. When a spike test moves to a new shell-package home, delete it from the
   spike test file at the same time.
4. The spike test file is allowed to have its compile broken during cutover
   on a per-stage basis, provided each commit explicitly states which stage
   is in progress and which spike tests are migrated/pending.
5. Stage E ends with `app_test.go` reduced to compatibility-only checks per
   WU-102's "Tests that remain as thin cutover checks" list.

This rule is binding for WU-100 and is the migration contract WU-102 will
verify.

## Cutover Strategy

Use a branch-local hard cutover with temporary compatibility shims, not a
long-lived dual implementation.

### Canonical package cutover

- once `internal/harnessshell` can render and process shell interactions
  end-to-end with host events, it becomes the canonical implementation
- from that point forward, new shell behavior fixes must land in
  `internal/harnessshell`, not in the spike wrapper

### Host cutover

- wire production-oriented modeltap integration only through
  `internal/harnesshost`
- prevent direct runtime imports from creeping back into the reusable shell

### Spike cutover

- once the shell package is canonical, the spike package has no remaining
  responsibilities beyond what `internal/harnessdemo` and `internal/harnesshost`
  cover
- the spike is deleted at Stage E end; there is no surviving "thin demo"
  variant

### Deletion rule

- the spike package may be deleted only after:
  - `internal/harnessshell` is the canonical implementation and compiles
  - `internal/harnessdemo` provides the fake-runtime capability the spike
    previously did
  - `internal/harnesshost` integration tests cover what WU-102 Layer 3
    requires
  - the replacement CLI entrypoint exists or has been removed deliberately
- if any of those preconditions fail, the spike removal is paused at the
  appropriate stage rollback boundary, but the design intent remains
  deletion (not retention)

## Risk Controls And Rollback Plan

### Primary risks

- scroll-follow and manual-scroll preservation regressions
- queue merge/release regressions during event-boundary cutover
- permission flow regressions, especially mid-stream pause/resume behavior
- accidental promotion of fake demo semantics into the reusable shell contract
- reopening `WU-098` by sneaking callback APIs back into the host boundary
- reopening `WU-099` by letting modeltap runtime details leak into the shell

### Controls

- preserve spike tests until equivalent coverage is intentionally relocated
- move one concern at a time: types, then render/layout, then actions/events,
  then host wiring, then spike thinning
- keep explicit ownership tables in code comments or package docs at each new
  package boundary
- reject any helper that couples `internal/harnessshell` to modeltap runtime
  packages or fake runtime state
- preserve queue and permission data structures until the new flow is proven,
  then rename only if still necessary

### Rollback

If a cutover stage destabilizes behavior:

- keep the new package files in place
- route the spike wrapper back to the previous stable implementation layer
- revert only the latest cutover step, not the already-extracted pure shell
  types/rendering work

Rollback boundaries should align to the compatibility stages:

- Stage B rollback: return spike rendering to old code while retaining new
  shell types
- Stage C rollback: keep new rendering but route runtime behavior through the
  previous shim
- Stage D rollback: keep reusable shell canonical but swap back to a temporary
  host translation layer instead of `internal/harnesshost`

## Implementation Guardrails

- no behavior redesign during extraction
- no new public shell boundary concepts beyond those already defined in
  `WU-098` and `WU-099`
- no callback-oriented API exceptions for convenience
- no direct file reads, provider calls, or runtime orchestration in
  `internal/harnessshell`
- no fake/demo command handling in `internal/harnessshell` unless the command is
  explicitly shell-native and behavior-local
- no partial cutover that leaves both spike and shell packages as competing
  sources of truth

## Done Condition For WU-100

`WU-100` is complete when:

- `internal/harnessshell` is the canonical home of the extracted shell behavior
- `internal/harnesshost` owns modeltap-specific integration
- `internal/harnessdemo` owns fake/demo runtime behavior
- `internal/harnessspike` has been deleted
- the extraction preserves the listed invariants
- the implementation is concrete enough that `WU-102` can write parity and
  regression coverage against the new package structure without redesign work

## Notes For WU-101 And WU-102

This implementation design intentionally leaves two follow-on expectations:

- `WU-101` should document the final post-extraction architecture
  (`internal/harnessshell` + `internal/harnesshost` + `internal/harnessdemo`),
  not the temporary Stage B/C compatibility shims and not the deleted
  `internal/harnessspike`
- `WU-102` should treat the current spike tests as the migration checklist
  and redistribute them into shell-unit and host-integration coverage once
  extraction lands. Layer 3 tests live in `internal/harnesshost` integration
  tests after Stage E, not in the deleted spike package
