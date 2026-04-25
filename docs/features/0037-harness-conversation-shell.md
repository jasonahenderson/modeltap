---
feature: FEAT-0037
title: Harness Conversation Shell
status: accepted
date: 2026-04-24
depends-on:
  - FEAT-0008: BFF Server
  - FEAT-0009: Terminal Harness
adr-constraints:
  - ADR-0001: Go as primary language
  - ADR-0003: Cobra CLI framework
  - ADR-0013: Terminal UI framework (Bubbletea from day one)
---

# FEAT-0037: Harness Conversation Shell

## Problem

FEAT-0009 defines the terminal harness at the product level, but it does not
yet pin down the concrete interaction model for the main conversation shell.
The harness spike has now established a specific shell shape: a single
scrolling transcript surface, a composer rendered at the tail of that surface,
queued follow-up handling, inline transcript artifacts, and a composer-driven
permission flow.

Without a feature-level contract for that shell behavior, the current spike
risks remaining an implementation artifact instead of becoming a stable product
surface. The project needs one behavior-scoped document that captures the shell
particulars clearly enough to guide packaging, extraction, and later
implementation work.

## Solution

Define the harness conversation shell as a single-surface terminal interface
with an inline transcript, a tail-mounted composer, non-modal queue handling,
and composer-hosted permission controls. The transcript remains the primary
reading surface. The composer is where the user types, reviews pending actions,
and resolves permission requests. Rich submitted artifacts are kept compact in
the transcript and expanded or previewed on demand.

This feature does not redefine the whole terminal harness. It narrows in on the
main shell interaction model that the harness presents once a session is open.
`FEAT-0009` remains the broader terminal-harness feature; this document is the
canonical refinement for the harness conversation-shell UX and interaction
semantics.

The first implementation goal under this feature is behavior-preserving
componentization: extract the current spike shell into a reusable component with
a clear API boundary while preserving the current shell behavior exactly. The
componentization step is not a UX redesign pass. Backlog items are reviewed
separately and should only be pulled into extraction if they are required to
make the component boundary sane or stable.

## Extraction Goal

Extract the current harness conversation shell into a reusable component with a
well-defined API while preserving current spike behavior exactly.

Extraction rules:

- preserve current transcript / composer / queue / permission behavior
- preserve current keyboard interactions and scroll behavior
- preserve the current visual structure unless extraction requires a mechanical
  layout change
- do not fold backlog improvements into the extraction unless they are required
  to define or stabilize the component API
- treat the current spike behavior as the reference contract for the first
  componentized version

## Key Capabilities

### Single Scrolling Surface

- The transcript and composer share one scrolling surface.
- The composer is rendered at the tail of the transcript content rather than as
  a permanently fixed bottom slab.
- In tight vertical layouts, the composer may scroll out of view when the user
  scrolls upward.
- Input focus must not force the viewport back to the bottom just because the
  composer is focused.

### Composer

- Single-line by default
- `alt+enter` and `Ctrl+J` insert newline
- Shell-style command history on `Up` / `Down` when editing a single-line
  buffer
- `▎` prompt marker for the input line
- Input focus is preserved after submit
- Mouse/touchpad scrolling does not steal input focus

### Transcript Rendering

- Flat transcript rows on the terminal-default main surface
- Streaming assistant output rendered inline in the transcript
- User and queued entries use the shared `▎` marker
- Long transcript lines wrap to the transcript width
- Transcript scrolling preserves manual scroll position unless the user is
  already following the tail

### Submitted Artifact Model

- Large pasted content is compacted into tokens before submit
- Submitted paste tokens expand inline in the transcript
- File references are represented as path/reference tokens in the transcript
- File tokens are inspected on demand rather than expanded inline by default
- Preview/open affordances remain available for token inspection

### Queue Handling

- A user may submit follow-up messages while a run is active
- Follow-ups are shown in the transcript as queued work
- Queued work remains FIFO
- If the current run completes normally, queued work is released automatically
- If the current run is interrupted, queued work remains queued
- When idle, pressing `Enter` on an empty composer releases queued work

### Stop Behavior

- Active streaming may be interrupted with a two-step `Esc` flow
- First `Esc` arms the interrupt and makes the stop affordance explicit
- Second `Esc` stops the current stream
- Stopped work remains visible in the transcript
- Interrupt does not automatically resume the stopped run

### Permission Flow

- Permission requests are represented in the transcript as event rows
- The active permission controls live in the composer, not in a modal
- The transcript preserves the durable request history; the composer hosts the
  current approval surface
- Composer permission details include:
  - tool label
  - target
  - short request summary
- Permission actions:
  - `Approve once`
  - `Allow for session`
  - `Deny`
- `Left` / `Right` choose the active permission action
- `Enter` applies the active permission action
- `y` / `n` remain optional fallback shortcuts while the composer is empty
- Repeated requests for a session-approved tool still surface a visible
  permission request, with remembered policy state shown in the composer
- Multiple pending permissions may coexist
- `Up` / `Down` switch which pending permission the composer is currently
  controlling
- A permission request that appears during streaming pauses the active stream
  immediately and surfaces the permission instead of queueing it as a normal
  follow-up

### Navigation Surfaces

- Sidebar remains available for session/model/actions navigation
- Command palette remains available for fast access to common actions
- Sidebar starts closed by default
- Inline transcript expansion is the default detail model; a split-view
  inspector is not required for this feature

## Component Boundary

The shell component owns interaction semantics and rendering. It should not own
real file access, provider transport, or app-specific command execution.

### Shell-owned responsibilities

- transcript rendering
- composer rendering
- queue state and queue-release behavior
- permission UI state, including multi-pending navigation
- token display and inline expansion state
- viewport, focus, and selection state
- shell-local key handling
- translation of user actions into explicit host requests

### Host-owned responsibilities

- provider/runtime interaction for submitted turns
- command execution for non-shell-native commands
- file inspection and preview loading
- production permission request origination and resolution
- persistence beyond shell-local interaction state
- tool/runtime pause/resume semantics outside the shell surface

### Shell-native vs host-native commands

Shell-native commands may remain internal to the shell component:

- `/clear`
- queue release behavior
- transcript/token-local view actions

Host-native commands must cross the API boundary:

- provider/session commands
- model or mode commands
- production permission commands
- any command that requires runtime, filesystem, or server coordination

### File handling through the API

The shell should render file references as path/reference tokens and allow
inspection on demand, but the host owns actual file access.

The shell owns:

- token display
- token selection
- inline paste expansion
- preview request intent

The host owns:

- reading file contents
- producing preview payloads
- validating referenced paths
- any permission-gated file access

### Provider response handling through the API

The shell should not talk to provider logic directly. It should submit a turn
through the host boundary and receive streamed run events back.

This preserves reusability across:

- the current fake spike runtime
- the future harness runtime
- tests and scripted harness fixtures

### Permission handling through the API

The shell owns:

- displaying permission requests in transcript history
- composer-hosted permission controls
- multi-pending permission navigation
- approve / deny UI state

The host owns:

- stable permission request identity
- real runtime pause / resume behavior
- policy persistence outside shell-local session state
- production tool/runtime effects of approval or denial

## API Expectations

The extracted shell component should expose:

- state inputs
- user-event inputs
- rendered view output
- explicit messages/callbacks for shell actions

At minimum, the component API should support these boundary shapes.

### Submit / run boundary

The shell emits a submit request:

- text
- submitted tokens
- shell context needed to correlate the run

The host returns run lifecycle events:

- run started
- stream delta
- run stopped
- run completed
- run failed

### Preview boundary

The shell emits preview intent for a selected file/reference token.

The host returns preview data:

- title
- rendered content
- any lightweight metadata required for inspection

### Permission boundary

The host emits permission requests with enough metadata for the shell to render:

- request identity
- tool label
- target
- summary
- policy state if any

The shell emits permission decisions:

- approve once
- approve for session
- deny

## CLI / UI Integration

CLI entrypoint:

```text
modeltap harness-spike
```

In-shell behaviors covered by this feature:

- `/clear`
- `/perm`
- `/demo`
- composer submission and queue release
- transcript scrolling
- permission action selection

## Configuration

This feature does not require new persistent configuration keys beyond what the
terminal harness already owns. It does, however, define shell-level interaction
semantics that any future configuration must preserve:

- queued work is not dropped silently
- permissions remain visible and interactive
- transcript artifacts remain compact by default

## Non-Goals

- Defining the production permission object model or request IDs
- Final packaging / extraction seams
- Split-view inspector
- Mouse-based permission interaction
- Retry / branch semantics
- Full session-tree or branch UX
- Multi-agent orchestration or background task execution semantics beyond the
  current shell surface

## Success Criteria

1. The harness shell uses one scrolling transcript surface with the composer
   rendered at the tail rather than as a fixed panel.
2. Transcript content wraps correctly within the viewport width, including
   streaming assistant output. **Test**: run `/demo` in a narrow terminal and
   verify content wraps instead of running off-screen.
3. Manual scroll position is preserved when the user has scrolled upward, even
   if input focus remains in the composer. **Test**: scroll up, type into the
   composer, confirm the viewport does not snap back to the bottom.
4. Queued follow-up messages remain visible and FIFO, and idle empty `Enter`
   releases queued work after an interrupt. **Test**: queue a message during a
   stream, interrupt the stream, then press `Enter` on an empty composer and
   verify the queued work starts.
5. The permission flow is non-modal and composer-driven. **Test**: run `/perm`,
   verify the request appears in the transcript and the approval controls appear
   in the composer.
6. Multiple pending permissions can coexist and be switched with `Up` / `Down`.
   **Test**: submit `/perm` twice and verify the composer can switch between
   pending requests.
7. Mid-stream permissions pause the active stream immediately and resume or end
   cleanly based on approval/denial. **Test**: start `/demo`, trigger `/perm`,
   approve it, and verify streaming resumes.
8. Pasted content remains inspectable inline in the transcript while file
   references remain compact path/reference tokens with on-demand preview.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0001 (Go) | The shell remains a Go implementation. |
| ADR-0003 (Cobra) | The shell is launched via the Cobra CLI entrypoint. |
| ADR-0013 (Bubbletea) | The shell interaction model is built on Bubble Tea terminal primitives. |

## Relationship to Other Features

| Feature | Relationship |
|---------|-------------|
| FEAT-0009 (Terminal Harness) | FEAT-0009 defines the harness at the product/system level. FEAT-0037 defines the canonical interaction model for the primary conversation shell inside that harness. |

## Open Questions

1. Does smoother scrolling require a different incremental rendering strategy
   during streaming, or is the current full-surface refresh acceptable?
2. Should mouse interaction for composer permission actions be added before
   packaging, or deferred entirely?
3. Which backlog items belong in componentization rather than in this shell
   feature itself: slash-command suggestions, tokenized history recall,
   empty/loading/error states, session list depth, side-surface model, broader
   selection model?
