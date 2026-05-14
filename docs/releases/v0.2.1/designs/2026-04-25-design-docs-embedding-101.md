# 2026-04-25 — Design: Developer Docs and Embedding Examples (WU-101)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Scope

This design covers **WU-101 only**:

- the documentation set that must ship with the extracted shell component
- ownership and boundary documentation for reusable shell versus modeltap host
- the minimal embedding example required for future engineers
- host integration examples for submit, stream, permission, and preview flows
- reconciliation rules between design-time names and final `WU-100`
  implementation names

This design does **not** implement the docs or examples themselves, and it
does not define the extraction implementation (`WU-100`).

## Purpose

Define the documentation set that ships with the extracted harness shell so a
future engineer can embed it in modeltap, or later extract it into another
repo, without reading the old `internal/harnessspike` implementation first.

This WU defines:

- which docs must exist after `WU-100`
- where those docs live
- what each doc must cover
- which examples must be included
- how the docs reconcile with final package/type/function names once `WU-100`
  lands

## Inputs And Constraints

The documentation design is constrained by the accepted release docs:

- `FEAT-0014` is the behavior contract. The docs must describe preserved shell
  behavior, not redesign it.
- `PATCH-0015` requires an action/event boundary and explicitly rejects
  callback-shaped host APIs.
- `WU-099` defines modeltap integration as a host-adapter layer, not as logic
  embedded inside the reusable shell package.
- `WU-101` depends on `WU-098` and `WU-099`, but runs in parallel with
  `WU-100`, so this design must be robust to final naming changes during
  implementation.

## Documentation Goals

The final documentation set must let another engineer answer these questions
directly:

1. What package is reusable and what package is modeltap-specific?
2. What state does the shell own versus the host?
3. How does a Bubble Tea host instantiate and drive the shell?
4. Which shell actions must the host handle?
5. Which host events can be sent back into the shell?
6. How do submit, streaming, permission, and preview flows cross the boundary?
7. Which commands stay local to the shell and which must be routed outward?
8. Which implementation names are canonical after `WU-100`, and which earlier
   design names were provisional?

## Documentation Set

The implementation produced by `WU-100` and documented by this WU must include
the following set.

### 1. Reusable shell package doc

Location:

- `internal/harnessshell/README.md` (package-level developer doc adjacent to
  the extracted reusable shell package)

Purpose:

- establish the shell package as the reusable FEAT-0014 interaction component
- define the component’s role inside a Bubble Tea program
- summarize the action/event boundary in one screenful
- state the parity promise: behavior-preserving extraction from the accepted
  shell spike

Minimum content:

- one-paragraph package purpose
- ownership table: shell-owned vs host-owned responsibilities
- summary of preserved behavior invariants:
  - single scrolling transcript surface
  - tail-mounted composer
  - queued follow-up release on empty `Enter` when idle
  - composer-driven permission handling
  - preview-on-demand for file reference tokens
- explicit note that provider logic, filesystem access, and production
  permission handling remain host responsibilities

### 2. Host adapter package doc

Location:

- `internal/harnesshost/README.md` (package-level developer doc adjacent to
  the modeltap-specific host adapter package)

Purpose:

- explain how modeltap maps runtime, tool, and preview behavior onto the
  reusable shell boundary
- make the reusable package and modeltap-specific adapter separate in the docs,
  not just in code

Minimum content:

- one-paragraph statement that this package is modeltap-specific glue
- description of the runtime-facing interface the adapter depends on
- mapping table from shell actions to host/runtime operations
- mapping table from runtime events back into shell host events
- statement that fake/demo behavior belongs in a separate adapter-style package
  rather than in the reusable shell

### 3. Embedding guide

Location:

- `docs/guides/harness-shell-embedding.md` — the canonical developer-facing
  embedding guide for the extracted shell. The `docs/guides/` directory is
  introduced by this WU if it does not already exist; the guide is the
  primary how-to artifact and should be linked from both package READMEs.

Purpose:

- serve as the main how-to guide for embedding the component into a Bubble Tea
  application

Minimum sections:

- architecture overview
- ownership and boundary rules
- minimal embedding example
- submit and stream integration example
- permission flow integration example
- preview flow integration example
- shell-native vs host-native command routing
- migration note for future extraction into a separate repository

### 4. Example code snippets

Location:

- code snippets embedded in the embedding guide and package docs

Purpose:

- provide enough concrete wiring detail that the docs are executable as a
  development guide even before full examples are promoted to standalone sample
  programs later

Required snippets:

- shell construction in a Bubble Tea model
- action dispatch from shell update output into host adapter
- runtime event projection back into shell state
- permission decision handoff
- preview request/response handoff

## Ownership And Boundary Documentation

The docs must include one canonical ownership table. It should not be spread
across multiple inconsistent descriptions.

Required ownership split:

### Shell-owned

- transcript rendering
- composer rendering
- viewport/focus/selection state
- queue state and queue-release behavior
- permission UI state and pending-permission navigation
- token display and inline paste expansion
- shell-local key handling
- shell-native commands such as `/clear` and transcript-local view actions

### Host-owned

- turn submission to runtime/BFF
- stream lifecycle and result delivery
- host-native command execution
- preview/file inspection loading
- production permission request origination and persistence
- policy state beyond shell-local UI concerns
- any direct filesystem, provider, or modeltap runtime interaction

The docs must also include a short anti-pattern section:

- no direct provider calls from the shell package
- no direct modeltap runtime imports from the reusable shell package
- no callback-shaped API examples at the package boundary
- no file reads performed directly by the reusable shell for preview loading

## Minimal Embedding Example

The docs must include one minimal example that demonstrates the smallest useful
host embedding. It is intentionally not a full app.

The minimal example must show:

- a Bubble Tea model that holds:
  - shell component state/model
  - host adapter reference
  - any command queue needed to bridge action/event flow
- an `Update` loop that:
  - routes terminal input into the shell
  - captures outbound shell actions
  - forwards those actions to the host adapter
  - routes returned host events back into the shell
- a `View` function that renders the shell as the main conversation surface

The minimal example does not need to show:

- full app chrome
- sidebar implementation
- config loading
- network/bootstrap setup
- production permission persistence details

The example must be documented as “minimal embedding”, not as “reference app”.
That keeps the docs focused on the reusable boundary.

## Host Integration Examples

The docs must include four separate flow examples. Each example should show:

- initiating shell action
- host-side handling
- event(s) sent back into the shell
- visible user-facing outcome

### 1. Submit flow

Required example sequence:

1. user presses `Enter` with a non-empty composer buffer
2. shell emits a submit action with normalized input payload
3. host adapter forwards the turn to modeltap runtime
4. host sends run-started event back to shell
5. shell shows active run state and preserves follow-up queue semantics

The example must also state:

- empty `Enter` while idle is queue release, not a normal submit
- submit handling must not bypass the action/event boundary

### 2. Stream flow

Required example sequence:

1. runtime produces stream delta events
2. host adapter projects them into shell host events
3. shell appends inline transcript output
4. runtime emits completion, failure, or interrupt terminal event
5. shell finalizes visible run state and queue-release behavior

The example must explicitly call out:

- manual scroll position is preserved unless the user is already following tail
- interrupt/stop semantics remain those defined by `FEAT-0014`

### 3. Permission flow

Required example sequence:

1. runtime/tool execution needs approval
2. host emits a permission-request event with stable identity and summary fields
3. shell renders durable transcript history plus active composer controls
4. user selects `Approve once`, `Allow for session`, or `Deny`
5. shell emits a permission-decision action
6. host applies the decision and resumes or terminates runtime work as needed
7. host emits resolution event back to shell so transcript state becomes durable

The example must explicitly call out:

- permission UI is composer-hosted, not modal
- repeated session-approved tools still surface a visible permission request
- runtime pause/resume policy is host-owned even though the shell owns the UI

### 4. Preview flow

Required example sequence:

1. user selects a file/reference token or preview affordance
2. shell emits preview-request action
3. host validates path/reference and loads preview payload
4. host emits preview-ready or preview-failed event
5. shell updates inline expansion or preview presentation state

The example must explicitly call out:

- the reusable shell asks for preview intent only
- path validation and file access stay outside the shell package

## Reconciliation With Final WU-100 Names

`WU-101` is being designed before `WU-100` finalizes package/type/function
names, so the docs must include a controlled reconciliation rule rather than
hard-coding unstable names from pre-implementation design.

Implementation rule:

- every doc produced from this design must treat package and type names from
  `WU-098` and `WU-099` as provisional until `WU-100` lands
- before release Phase 1 is marked complete, the doc author must perform one
  reconciliation pass against the implemented names

Required reconciliation table in the final docs:

| Design role | Final implementation name | Notes |
| --- | --- | --- |
| Reusable shell package | `<final>` | previously “reusable shell package” in design |
| Modeltap host adapter package | `<final>` | previously `internal/harnesshost` in design |
| Shell outbound action type | `<final>` | must remain action-oriented |
| Shell inbound host event type | `<final>` | must remain event-oriented |
| Minimal host adapter interface | `<final>` | runtime-facing adapter surface |

Reconciliation rules:

- if implementation keeps the `internal/harnesshost` name, the docs use it
- if implementation renames packages or types, the docs update all narrative and
  examples to final names before review
- examples must not mix provisional and final names
- if `WU-100` merges or splits packages differently than expected, the docs
  preserve the same ownership narrative even if labels change

The goal is stable concepts first, final names second.

## Documentation Structure For Another Engineer

The final docs must be ordered so a new engineer can read them linearly:

1. package purpose and preserved behavior
2. ownership boundary
3. minimal embedding example
4. host integration flows
5. command split and extension guidance
6. implementation-name reconciliation notes

The docs must avoid sending the reader into the old spike branch for answers.
If a behavior is required for embedding, it belongs in the new docs.

## Non-Goals

This WU does not define:

- a separate public website or external docs portal
- a standalone sample application outside the repo
- implementation of the package docs or examples
- any UX redesign beyond documented FEAT-0014 parity behavior
- production permission-object redesign beyond the stable shell boundary

## Acceptance Criteria

`WU-101` is complete when another engineer could write the final docs without
opening `internal/harnessspike/` and without reverse-engineering the old
prototype.

That requires:

- the documentation set is explicitly enumerated with named file paths:
  `internal/harnessshell/README.md`, `internal/harnesshost/README.md`,
  and `docs/guides/harness-shell-embedding.md`
- ownership and boundary rules are unambiguous
- one minimal embedding example is defined
- submit, stream, permission, and preview examples are defined
- reconciliation rules against final `WU-100` names are defined
- the doc structure distinguishes reusable-shell documentation from
  modeltap-specific host-adapter documentation
