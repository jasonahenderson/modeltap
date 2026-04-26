# PATCH-0015: Harness Shell Component API

**Status:** approved
**Date:** 2026-04-24
**Related:** FEAT-0009, FEAT-0014
**Branch:** patch/0015-harness-shell-component-api
**Release:** v0.2.1

## Problem

The harness spike shell now has a stable interaction model, but it still lives
as a spike package with runtime behavior, file handling, command behavior, and
permission flow entangled in one implementation. Before it can be packaged or
extracted cleanly, the shell needs a well-defined component API that preserves
the current behavior exactly while separating UI ownership from host/runtime
ownership.

Without that API boundary, extraction will either freeze the spike internals in
place or accidentally redesign behavior during refactoring.

## Scope

- Define a repo-native shell component contract for the harness conversation
  shell currently implemented in `internal/harnessspike/`
- Define the extracted component as a cleanly separated unit that can later be
  moved into its own project with minimal contract churn
- Preserve current spike behavior during extraction:
  - single scrolling transcript surface
  - tail-mounted composer
  - queued follow-up handling
  - stop behavior
  - composer-driven permission flow
  - inline paste expansion and file reference tokens
- Define shell-owned responsibilities vs host-owned responsibilities
- Define the component boundary for:
  - submitted turns
  - streamed run lifecycle events
  - preview/file inspection requests
  - permission requests and permission decisions
  - shell-native vs host-native commands
- Prefer serialization-friendly typed actions/events over callback-oriented API
  shapes
- Produce detailed developer documentation for using the component from a host
  application
- Produce a stable enough contract to guide packaging / extraction work

## Out of Scope

- Production permission request IDs and final runtime object model
- BFF wire protocol changes
- New UX behavior beyond what the spike already does
- Retry / branch semantics
- Mouse interaction for permission controls
- Revisiting backlog items unless extraction requires them
- Publishing the component as a separate project during this patch

## Checklist

- [ ] Define the extracted shell as a component with a clear host boundary
- [ ] Define shell-owned responsibilities
- [ ] Define host-owned responsibilities
- [ ] Define shell-native commands vs host-native commands
- [ ] Define typed outbound shell actions
- [ ] Define typed inbound host/runtime events
- [ ] Define preview/file-inspection boundary
- [ ] Define permission request/decision boundary
- [ ] Define queue, run, and permission lifecycle invariants
- [ ] Ensure the API shape avoids callback/closure-based boundary contracts
- [ ] Keep the contract aligned with FEAT-0014 behavior
- [ ] Document how a host application instantiates, drives, and consumes the component
- [ ] Document extraction seams required for moving the component into its own project later
- [ ] Keep package boundaries clean enough that the component can be promoted out of the repo with minimal rewiring

## Contract Outline

### 1. Boundary model

The shell component owns interaction semantics and rendering. The host owns
runtime effects and external data access.

The boundary should be action/event based:

- shell emits **actions**
- host/runtime emits **events**

This keeps the API:

- testable
- replayable
- serialization-friendly
- extensible as the harness grows
- suitable for later extraction into a separate project without redefining the
  interaction contract

### 2. Shell-owned responsibilities

- transcript rendering
- composer rendering
- viewport, focus, and selection state
- queue state and queue-release behavior
- permission UI state
- token rendering and inline paste expansion
- keyboard handling and shell-local interaction rules

### 3. Host-owned responsibilities

- turn submission to the real runtime/BFF side
- streaming lifecycle and result delivery
- command execution for host-native commands
- file inspection / preview loading
- production permission origination and resolution
- persistence and policy state beyond shell-local UI state

### 4. Command split

Shell-native commands may remain local:

- `/clear`
- queue release behavior
- transcript-local view actions

Host-native commands must cross the component boundary:

- provider/session commands
- model/mode commands
- production permission commands
- anything requiring server, runtime, or filesystem coordination

### 5. File handling boundary

The shell should render:

- paste tokens with inline expansion
- file tokens as path/reference artifacts

The host should provide:

- preview payloads
- file-content loading
- path validation
- any permission-gated file access

### 6. Provider/run boundary

The shell must not call provider logic directly.

The host accepts a submitted turn and returns run lifecycle events:

- run started
- stream delta
- run stopped
- run completed
- run failed

### 7. Permission boundary

The host emits permission requests with:

- request identity
- tool label
- target
- summary
- optional remembered-policy state

The shell emits permission decisions:

- approve once
- approve for session
- deny

### 8. API-shape guidance

Preferred boundary style:

- typed action structs
- typed event structs
- stable IDs
- no function-valued fields at the component boundary
- no callback-driven preview/permission/run contracts

Avoid API shapes like:

- `OnApprove func()`
- `OnPreview func(...)`
- `Submit(..., onDelta func(...), onDone func(...))`

Those shapes make replay, logging, persistence, and testing harder as the
project grows.

### 9. Invariants to preserve during extraction

- queued work remains FIFO
- empty `Enter` while idle releases queued work
- interrupt does not auto-resume stopped work
- permission requests remain visible in transcript history
- input focus does not force follow-tail scrolling
- inline paste expansion remains the default detail model

## Developer Documentation Requirements

The extracted component must ship with developer-facing documentation that is
good enough for another engineer to embed it without reading the entire shell
implementation first.

At minimum, the documentation must describe:

- what package exposes the reusable shell component
- which package(s) remain host/runtime-specific
- how to construct the component
- which actions the host must accept
- which events the host may emit back into the component
- how queue handling, permission handling, and preview loading are expected to
  work across the boundary
- how to wire the component into a Bubble Tea program
- what behavior is intentionally preserved from the spike
- which behaviors are shell-local vs host-defined

The documentation should include:

- a high-level architecture diagram or equivalent ownership table
- a minimal embedding example
- a host-integration example for:
  - turn submission
  - streaming updates
  - permission request / decision flow
  - file preview flow

## Separation Requirements For Future Extraction

The extraction should be organized so the shell component can later move into
its own repository with minimal contract churn.

That requires:

- no direct dependency from the reusable shell package onto modeltap-specific
  runtime packages
- no direct provider logic inside the shell component
- no direct filesystem reads in the shell component beyond purely local UI
  concerns
- no hard dependency on spike-only demo commands for normal operation
- host integration through exported types/interfaces, not hidden package-local
  coupling
- transport-agnostic action/event contracts

Preferred package shape inside this repo:

- reusable shell package
- host adapter package for modeltap-specific runtime integration
- optional demo/fake runtime package for spike/test harness behavior

The reusable shell package should be able to move out of this repository
without carrying:

- BFF-specific transport code
- provider adapters
- modeltap-specific config loading
- repo-specific command wiring

## Expected Deliverables

The implementation under this patch should eventually produce:

- a reusable shell package with exported API types
- host adapter code for modeltap-specific integration
- developer documentation for embedding the component
- examples or tests demonstrating the integration contract

## Fix Detail

This patch is intentionally API-first. The first extraction goal is not to
improve behavior, but to preserve the spike shell exactly while moving runtime
effects behind a well-defined boundary. Backlog items should be reconsidered
only after the extraction boundary is clear.
