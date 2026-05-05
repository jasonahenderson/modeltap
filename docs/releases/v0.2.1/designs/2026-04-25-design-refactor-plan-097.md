# 2026-04-25 — Design: Refactor Plan and Migration Sequencing (WU-097)

## Scope

This design covers **WU-097 only**:

- refactor-plan strategy for shell componentization
- migration sequencing from the spike shell to the extracted component
- keep/adapt/replace classification for the current code surfaces
- cutover plan and risk containment

This design does **not** define the detailed component API (`WU-098`) or the
modeltap host adapter integration contract (`WU-099`).

## Context

`FEAT-0014` fixes the target shell behavior. `PATCH-0015` authorizes extracting
that shell into a reusable component with a clean host/runtime boundary.

The complication is that the accepted shell behavior lives in
`internal/harnessspike/`, while the existing production harness line that will
eventually host it is outside this branch and is already known to be the wrong
UX authority for the conversation surface.

So this WU must plan around two truths:

1. `internal/harnessspike/` is the current source of truth for shell behavior
2. the later `v0.2.0` harness line remains the source of command/runtime
   integration inventory that the eventual host adapter must reuse or replace

This WU therefore produces the migration strategy first, before detailed API
and integration designs are written.

This design intentionally references `PATCH-0015` for the already-accepted
ownership split and boundary policy. Its value-add is the seam map,
entanglement audit, migration sequence, and cutover rules specific to the
current `internal/harnessspike/` implementation.

## Goals

1. Replace the current harness conversation UI with the `FEAT-0014` shell model
   rather than incrementally polishing the old UI.
2. Preserve accepted shell behavior exactly during extraction.
3. Separate reusable shell concerns from modeltap-specific runtime concerns.
4. Keep the migration incremental enough that parity can be checked at each
   step.
5. Leave a package structure that can later move into its own project without
   redesigning the contract.

## Non-Goals

- redesigning shell UX during extraction
- defining final provider/BFF wire contracts
- redefining the production permission object model
- implementing the extraction in this WU
- writing the final exported Go API in this WU

## Source-of-Truth Rules

### Behavioral source of truth

The behavioral source of truth is:

- `FEAT-0014`
- `internal/harnessspike/`
- associated spike history docs captured during the shell spike

### Integration source of truth

The integration source of truth is the later harness/runtime line associated
with `v0.2.0`, even though that code is not present in this worktree.

That line should be consulted in `WU-099` for:

- existing slash-command inventory
- runtime submission flow
- session/runtime state surfaces
- connection/status surfaces
- existing protocol/client boundaries

### Explicit parity rule

Parity is measured against the accepted spike behavior captured in `FEAT-0014`.
Parity is **not** measured against the currently broken production conversation
UI.

## Keep / Adapt / Replace Classification

## Seam Map And Entanglement Audit

### Current extraction surfaces in `internal/harnessspike/`

The current spike is concentrated in:

- `internal/harnessspike/app.go`
  mixed shell state, rendering, fake runtime flow, permission demo flow, token
  handling, queue behavior, and overlays
- `internal/harnessspike/styles.go`
  purely presentational shell styling
- `internal/harnessspike/app_test.go`
  current behavioral oracle for shell extraction

### Extract to reusable shell package

These seams should move into the reusable shell package during `WU-100`:

- transcript rendering and transcript-local selection state
- composer rendering, input rules, and command-history behavior
- token creation/display/inline expansion rules
- queue state, merge behavior, and queue-release rules
- permission UI navigation and composer-hosted approval controls
- viewport, focus, follow-tail, and scroll-preservation rules
- shell-local overlays that remain part of the generic shell contract
- styling from `styles.go`

### Move behind host or demo adapters

These concerns should leave the reusable shell package:

- fake streaming generation and timing
- demo slash-command behavior used only to exercise the spike
- fake permission grant/deny result text
- runtime-backed preview loading for external references
- modeltap-specific command routing and session/runtime state projection

### Entanglement hotspots

The main current entanglements to unwind in `WU-100` are:

- `submit()` mixing shell-native commands, permission demo, queue release, and
  fake runtime start behavior
- `beginSubmission()` creating shell transcript rows and starting fake runtime
  ticks in one path
- `beginPermissionDemo()` mixing transcript event creation with demo policy and
  fake response continuation
- `pauseStreamingForPermission()` and `grantPermission()` mixing shell-local UI
  state with fake runtime pause/resume mechanics
- transcript preview logic where paste-token preview is local but file preview
  will become host-backed

### Keep

These categories should survive conceptually and likely remain below the shell
boundary:

- runtime/provider submission mechanics
- session and connection management concepts
- command inventory and command semantics
- file preview/loading capabilities
- permission origination from real runtime/tool activity

These are not shell-owned; they become host responsibilities.

### Adapt

These categories should be retained but moved behind the shell boundary:

- turn submission entrypoints
- streaming event delivery
- command dispatch
- preview/file inspection loading
- permission request origination and permission decision application
- session/mode/model state projected into shell-visible data

These become part of the host adapter defined in `WU-099`.

### Replace

These categories should be treated as replacement scope:

- current conversation transcript layout
- current composer layout/placement assumptions
- current queue rendering/interaction
- current permission-surface UX
- current shell-local scrolling/focus behavior
- current shell-local state organization where it directly encodes the old UI

The replacement target is the `FEAT-0014` shell behavior.

## Proposed Package Strategy

This WU does not freeze the final package names, but it does fix the package
separation strategy.

### Target package layers

1. **Reusable shell package**
   - owns transcript/composer/queue/permission UI state
   - owns rendering and shell-local input handling
   - emits actions
   - consumes host/runtime events

2. **Modeltap host adapter package**
   - owns translation between modeltap runtime state and shell actions/events
   - owns command routing, preview loading, permission origination, and turn
     submission integration

3. **Optional demo/fake runtime layer**
   - preserves the ability to exercise shell behavior without the real runtime
   - useful for tests, examples, and spike parity checks

### Demo/fake runtime destination

The current fake runtime behavior should not stay inside the reusable shell
package. It should move into the optional demo/fake runtime layer described
above.

That layer may remain repo-internal in `v0.2.1`, but it becomes an adapter
consumer of the extracted shell boundary rather than part of the shell package
itself. `WU-099` should define how modeltap and the fake runtime each implement
the same action/event contract.

### Anti-goals for package structure

- no reusable shell package importing modeltap-specific runtime code directly
- no provider logic inside the shell package
- no filesystem preview loading inside the shell package
- no dependence on spike-only slash commands for normal operation

## Migration Strategy

The migration should proceed in five stages.

### Stage 1 — Freeze the shell behavior contract

Inputs:

- `FEAT-0014`
- spike baseline commits already carried to this branch
- `PATCH-0015`

Output:

- accepted behavior target and release plan in `v0.2.1`

Status:

- complete before this WU

### Stage 2 — Design the shell boundary

Owned by:

- `WU-098`
- `WU-099`

Output:

- component API design
- host adapter design

Key rule:

- no implementation starts until the boundary is explicit enough to prevent
  silent UX redesign during extraction

### Stage 3 — Extract shell-local code behind the new boundary

Owned by:

- `WU-100`

Approach:

- start from `internal/harnessspike/` as the behavioral baseline
- move shell-local state and rendering into the reusable shell package
- replace direct demo/runtime coupling with action/event seams

Key rule:

- preserve visible shell behavior during every extraction step

### Stage 4 — Attach modeltap host integration

Owned by:

- `WU-100`, guided by `WU-099`

Approach:

- translate existing modeltap runtime capabilities into host actions/events
- reuse command/runtime inventory from the `v0.2.0` harness line
- do not import old conversation-UI assumptions into the new shell package

### Stage 5 — Cutover and retire superseded shell code

Owned by:

- `WU-100`
- validated by `WU-102`

Approach:

- switch the harness entrypoint to the extracted shell path
- quarantine or remove superseded shell-local code from the old harness UI
- leave only host/runtime responsibilities outside the reusable shell package

## Sequencing Rules

1. Do not redesign shell UX while moving code.
2. Do not use old harness UI behavior as the parity target.
3. Do not let host-runtime coupling leak into the reusable shell package.
4. Keep a runnable shell-demo path during extraction if it materially reduces
   regression risk.
5. Prefer additive migration and cutover over in-place mutation of the old
   harness UI.

## Risk Inventory

### Risk 1 — Old harness UI assumptions leak back into the new shell

Cause:

- treating current `internal/harness` UI behavior as compatibility scope

Mitigation:

- use `FEAT-0014` as the explicit parity target
- classify old conversation UI as replacement scope

### Risk 2 — Shell package becomes modeltap-specific

Cause:

- direct imports of runtime/provider/filesystem code into the extracted shell

Mitigation:

- defer those capabilities to `WU-099` host adapter design
- keep shell boundary action/event oriented

### Risk 3 — Extraction becomes a silent redesign

Cause:

- changing layout/interaction while moving code

Mitigation:

- treat the spike behavior as fixed during extraction
- push optional UX reconsideration to later releases

### Risk 4 — Missing command/runtime inventory during integration

Cause:

- this branch does not contain the later `v0.2.0` harness implementation line

Mitigation:

- make `WU-099` explicitly inventory the command and runtime surfaces from that
  line before integration design is finalized

### Risk 5 — Cutover leaves duplicate shell implementations alive for too long

Cause:

- half-migrated code path with no clear cutover moment

Mitigation:

- `WU-100` must include an explicit cutover step
- `WU-102` must verify the extracted shell path rather than only the spike/demo
  path

## Deliverables For Later WUs

### Inputs to WU-098

- package-layer strategy
- shell parity target
- keep/adapt/replace classification
- migration-stage sequence

### Inputs to WU-099

- explicit instruction to inventory the `v0.2.0` harness command/runtime line
- explicit instruction that old conversation UI is replacement scope

### Inputs to WU-100

- cutover strategy
- extraction order
- risk constraints

## Acceptance Criteria

This WU is complete when:

1. The migration plan states clearly that the current conversation UI is being
   replaced, not refined in place.
2. The plan defines the keep/adapt/replace split for shell vs host/runtime
   responsibilities.
3. The plan identifies the later `v0.2.0` harness line as integration
   inventory, not behavioral authority.
4. The plan defines the staged extraction/cutover sequence for later WUs.
5. The plan gives `WU-098` and `WU-099` clear downstream design inputs.
