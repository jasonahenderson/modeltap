# 2026-04-25 — Design: Shell Component API and Package Boundary (WU-098)

## Scope

This design covers **WU-098 only**:

- reusable shell package shape
- exported API boundary for shell state, actions, and host events
- shell-owned state and responsibility boundaries
- serialization-friendly type strategy for turns, tokens, previews, and permissions
- package seams required to preserve `FEAT-0014` during extraction

This design does **not** define the modeltap-specific host adapter wiring
(`WU-099`) or the extraction sequence itself (`WU-100`).

## Context

`WU-097` fixed the migration strategy: the accepted shell behavior source of
truth is `FEAT-0014` plus `internal/harnessspike/`, and the later harness line
is integration inventory only.

The current spike proves the shell contract, but it keeps three concerns inside
one `App` type:

1. shell-local interaction and rendering state
2. fake/demo runtime effects
3. modeltap-adjacent concepts that belong on the host side later

`PATCH-0015` requires an action/event contract that keeps the shell replayable,
testable, and movable into its own repository later. The reusable package must
therefore own interaction semantics and rendering while pushing runtime effects
and external data access across an explicit boundary.

This design inherits `PATCH-0015` section 8 as binding policy: callback-shaped
boundary contracts are already out of bounds. `WU-098` therefore defines the
concrete typed structs, IDs, and message flows underneath that policy rather
than reopening the callback-vs-action/event decision.

## Goals

1. Preserve the `FEAT-0014` shell behavior exactly during extraction.
2. Define one reusable shell package that does not import modeltap runtime
   concerns.
3. Use typed actions and host events rather than callback-shaped contracts.
4. Keep the contract deterministic enough for replay, tests, and later
   extraction.
5. Partition current spike state into shell-owned, host-owned, and transient
   translation layers.

## Non-Goals

- defining modeltap runtime package names
- freezing final command inventory beyond the shell-native split
- redesigning shell visuals or interaction semantics
- defining transport or BFF payload schemas
- implementing the package split in this WU

## Source Inputs

Behavioral and contract inputs:

- `docs/features/0014-harness-conversation-shell.md`
- `docs/patches/0015-harness-shell-component-api.md`
- `docs/releases/v0.2.1/designs/2026-04-25-design-refactor-plan-097.md`

Current implementation baseline:

- `internal/harnessspike/app.go`
- `internal/harnessspike/app_test.go`

## Design Summary

The extracted shell should be a Bubble Tea model packaged as a reusable
conversation-shell component with this shape:

1. the shell receives Bubble Tea UI messages and explicit host/runtime events
2. the shell mutates only shell-owned state
3. the shell emits typed outbound actions describing requested host work
4. the host performs real effects and feeds typed events back into the shell

The shell package therefore exposes:

- one exported Bubble Tea model type
- exported immutable-ish data structs for host-fed state and shell actions
- exported host-event messages that can be sent back through `Update`
- exported constructors/options for shell-local configuration only

The shell package does **not** expose function-valued effect hooks at the
boundary. All external effects cross the boundary as data.

## Proposed Package Layout

### Reusable package

Primary target:

- `internal/harnessshell`

This package is the extraction target for the shell component in `v0.2.1`. It
is repo-internal for now, but must be organized so it can later promote out of
the repo with minimal contract churn.

### Internal files within the reusable package

Suggested layout:

- `model.go`
  Bubble Tea model, constructor, update loop entrypoints
- `types.go`
  exported action/event/data types
- `state.go`
  shell-owned state structs and invariants
- `render.go`
  transcript/composer/sidebar rendering
- `input.go`
  shell-local key handling and command-history behavior
- `tokens.go`
  token summarization, selection, inline expansion rules
- `permissions.go`
  permission navigation and decision state
- `queue.go`
  queued follow-up lifecycle and merge rules
- `styles.go`
  styling only

`WU-100` may adjust exact filenames, but this responsibility split should hold.

### Non-reusable packages

Kept outside `internal/harnessshell`:

- modeltap host adapter package defined in `WU-099`
- fake/demo runtime package used by spike/examples/tests

## Exported Surface

### Core type

The reusable package should expose one primary model:

```go
type Model struct {
    // contains shell-owned state only
}
```

Constructor:

```go
func New(opts ...Option) Model
```

The returned type remains a `tea.Model` implementation. Hosts instantiate it,
embed it in their Bubble Tea program, and relay host events back into it via
`Update`.

### Configuration options

Options are limited to shell-local defaults and cosmetic/runtime-neutral setup,
for example:

- initial shell title or model label presentation
- placeholder text
- initial sidebar open/closed state
- shell-native command enablement toggles if needed

Options must not include callback hooks such as:

- submit handlers
- preview loaders
- permission resolvers
- stream writers

Those belong to the host action/event boundary.

## Boundary Model

### Shell receives

The shell receives two categories of inputs:

1. regular Bubble Tea UI messages
2. typed host events injected by the embedding program

Representative host event marker:

```go
type HostEvent interface {
    isHostEvent()
}
```

Each concrete host event is a plain struct so it can be logged, replayed, and
constructed in tests without hidden closures.

### Shell emits

The shell emits typed outbound actions by appending them to an internal action
queue and returning a Bubble Tea command that forwards them to the host
program.

Representative marker:

```go
type Action interface {
    isAction()
}
```

The host loop drains these actions, performs real effects, and sends back host
events. This preserves Bubble Tea ergonomics without coupling the reusable
package to a specific runtime implementation.

## Action Contract

The shell emits actions only for work it does not own.

### Required outbound actions

#### `SubmitTurnAction`

Emitted when the user submits immediately or queued work is released.

```go
type SubmitTurnAction struct {
    Submission Submission
}
```

`Submission` fields:

- `ID string`
  stable shell-generated submission identifier
- `Entries []string`
  exact user-visible entry slices after queue merge
- `Text string`
  normalized merged text payload
- `Tokens []InputToken`
  submitted file/paste tokens in submission order
- `Source SubmissionSource`
  `direct` or `queue_release`
- `RequestedAt time.Time` or host-supplied monotonic sequence later if needed

The host uses `Submission.ID` to correlate lifecycle events.

#### `InterruptRunAction`

Emitted on the second `Esc` when an active run is armed for interrupt.

```go
type InterruptRunAction struct {
    RunID string
}
```

If the host cannot interrupt, it must still answer with a terminal lifecycle
event so the shell leaves the armed state explicitly.

#### `ResolvePermissionAction`

Emitted when the composer applies the selected permission choice.

```go
type ResolvePermissionAction struct {
    RequestID string
    Decision  PermissionDecision
}
```

`PermissionDecision` values:

- `approve_once`
- `approve_session`
- `deny`

#### `LoadPreviewAction`

Emitted when the user requests preview/open for a file/reference token that the
shell does not already fully own.

```go
type LoadPreviewAction struct {
    Target PreviewTarget
}
```

The target identifies whether the preview request originated from:

- composer token selection
- transcript token selection

and includes the referenced token identity.

#### `RunCommandAction`

Emitted for host-native slash commands only.

```go
type RunCommandAction struct {
    Invocation CommandInvocation
}
```

Shell-native commands such as `/clear` remain local and never emit this action.

## Host Event Contract

The host sends events whenever external state changes.

### Required inbound events

#### `RunStartedEvent`

```go
type RunStartedEvent struct {
    SubmissionID string
    RunID        string
    Label        string
}
```

Transitions the shell from submitted/queued state into active streaming state.

#### `RunDeltaEvent`

```go
type RunDeltaEvent struct {
    RunID  string
    Delta  string
}
```

Appends inline assistant output to the active transcript row.

#### `RunCompletedEvent`

```go
type RunCompletedEvent struct {
    RunID string
}
```

Marks the active assistant row complete and triggers queue auto-release.

#### `RunStoppedEvent`

```go
type RunStoppedEvent struct {
    RunID   string
    Reason  StopReason
    Message string
}
```

Used for explicit interrupt or host-side stop conditions. Queue remains queued.

#### `RunFailedEvent`

```go
type RunFailedEvent struct {
    RunID   string
    Message string
}
```

Marks the run terminal and surfaces failure text without inventing retry
semantics.

#### `PermissionRequestedEvent`

```go
type PermissionRequestedEvent struct {
    Request PermissionRequest
}
```

Appends a durable transcript event row and activates composer permission
controls. If a run is active, the host must pause it before or alongside this
event so shell-visible behavior matches `FEAT-0014`.

#### `PermissionResolvedEvent`

```go
type PermissionResolvedEvent struct {
    RequestID string
    Outcome   PermissionOutcome
    Message   string
}
```

Updates transcript event state to granted or denied and clears the active
composer controls for that request.

#### `PreviewLoadedEvent`

```go
type PreviewLoadedEvent struct {
    Target  PreviewTarget
    Preview PreviewPayload
}
```

Supplies preview data for file/reference tokens. The shell owns rendering of
the preview surface; the host owns the data fetch.

#### `HostStatusEvent`

Optional but recommended:

```go
type HostStatusEvent struct {
    Status string
}
```

Allows the host adapter to update footer/status text without coupling status
rendering to runtime internals.

## Data Types

### Transcript model

The shell owns transcript row layout and selection. The exported row types
should be explicit enough that host events map onto stable transcript states.

Suggested row model:

```go
type TranscriptItem struct {
    ID        string
    Kind      TranscriptItemKind
    Role      Role
    Text      string
    Tokens    []InputToken
    Expanded  map[string]bool
    Event     *EventState
    Streaming bool
}
```

Implementation may use a denser internal form, but the logical model should
remain:

- user rows
- assistant rows
- event rows
- queued submission rows rendered outside the committed transcript list

### Token model

Promote spike `inputToken` into an exported stable data type:

```go
type InputToken struct {
    ID      string
    Kind    TokenKind
    Label   string
    Payload string
}
```

`TokenKind` initially needs only:

- `paste`
- `file`

Rules:

- shell may fully own inline expansion for paste tokens using existing payload
- shell may preview paste tokens locally without host round-trip
- file/reference tokens may require host preview loading
- token identity must remain stable across submission, transcript rendering,
  preview requests, and queue merge

### Permission model

Promote permission metadata into exported host-fed data:

```go
type PermissionRequest struct {
    ID                 string
    ToolLabel          string
    Target             string
    Summary            string
    SessionPolicyState SessionPolicyState
}
```

Shell-local state kept separately:

- active request index
- currently selected action

Host-owned state:

- stable permission request identity
- remembered session/global policy outside the UI
- pause/resume semantics for the runtime

The placeholder permission contract in this WU is intentionally narrow:

- enough metadata to render the composer and transcript correctly
- stable IDs for replay and later runtime substitution
- no production-only permission object semantics baked into the reusable shell

This keeps the boundary stable enough for later production integration without
forcing a second extraction pass solely to replace a demo-shaped permission API.

### Preview model

```go
type PreviewPayload struct {
    Title    string
    Content  string
    Metadata map[string]string
}
```

The shell renders the preview dialog or inline surface. The host provides data
for file/reference targets; the shell may synthesize preview payloads for local
paste tokens.

## Ownership Split

### Shell-owned state

The following current spike state belongs inside the reusable component:

- transcript items and transcript selection
- composer buffer and height
- input token collection and selected token
- queue buffers and queue-release mechanics
- command history and history draft
- permission UI navigation state
- preview dialog visibility and locally available preview content
- viewport, follow-tail, focus, sidebar-open state
- interrupt arming state
- shell-local status/footer presentation

Concrete current fields from `internal/harnessspike.App` that stay shell-owned:

- `focus`
- `input`
- `transcript`
- `status`
- `messages`
- `sidebarItems` only if rendered as shell-local navigation data
- `sidebarIndex`
- `dialog` / `preview` / `agentList` / `agentDetail` / `palette` only if still
  considered shell-local overlays during extraction
- `sidebarOpen`
- `inputTokens`
- `selectedToken`
- `transcriptRefs`
- `selectedTranscriptRef`
- `queuedSubmissions`
- `pendingSubmissions`
- `commandHistory`
- `historyIndex`
- `historyDraft`
- `pendingPermissions`
- `activePermissionIndex`
- `interruptArmed`

### Host-owned state/effects

These move outside the reusable shell package:

- actual provider/runtime submission
- run identity allocation if not shell-generated
- streaming production and completion/failure decisions
- real interrupt execution
- file reads and preview loading for external references
- non-shell-native command execution
- permission request origination
- policy persistence such as session-approved tools
- agent/task orchestration data not required by the generic shell contract

Concrete current spike fields that should not define the reusable package API:

- `streamQueue`
- `streaming` as a fake runtime driver
- `streamDelay`
- `streamPulse` as fake-runtime timing, though visual working-state rendering
  stays shell-owned
- `modelName` as a host-fed display label rather than hardcoded shell state
- `agents` unless later generalized into a separate shell surface
- `sessionAllowedTools`
- `pausedResponse`

## Command Boundary

### Shell-native commands

Remain local in the reusable shell package:

- `/clear`
- empty `Enter` queue release while idle
- transcript token expand/collapse
- local preview open for already-owned paste-token payloads

### Host-native commands

Cross the boundary as `RunCommandAction`:

- provider/session commands
- model/mode commands
- filesystem or server dependent commands
- production permission commands
- any command not fully satisfiable from shell-local state

The shell may still parse slash-command text to classify it, but execution of
host-native commands belongs to the host adapter.

## Lifecycle Invariants

The reusable package must preserve these invariants exactly.

### Queue invariants

- queued submissions remain FIFO as entered
- if multiple queued submissions are released together, merged `Entries`
  preserve per-submit order
- empty `Enter` while idle releases queued work
- interrupted runs do not auto-release queued work
- normal completion auto-releases queued work

### Permission invariants

- permission requests always leave a durable transcript history row
- active permission controls live in the composer area
- multiple pending permissions may coexist
- `Up` / `Down` switch the active pending permission when the composer buffer is
  empty
- `Left` / `Right` switch the selected action when the composer buffer is empty
- `Enter` applies the selected action when the composer buffer is empty
- `y` / `n` are fallback shortcuts only while the composer buffer is empty
- repeated requests after session approval still surface visible permission UI

### Scroll and focus invariants

- input focus is preserved after submit
- mouse-wheel transcript scrolling does not steal input focus
- manual scroll position is preserved unless the user is already following the
  tail
- focusing the composer does not force the viewport back to bottom

### Token invariants

- large pasted content compacts into a token before submit
- submitted paste tokens start expanded inline in the transcript
- transcript `Enter` toggles paste-token expansion inline
- file/reference tokens request or open preview rather than auto-expanding

## Bubble Tea Integration Pattern

The reusable package remains a Bubble Tea child model, but the host adapter
must drive it through explicit action/event relaying:

1. host program forwards ordinary Tea input to `harnessshell.Model.Update`
2. when the model emits an action message, the host adapter handles the effect
3. host adapter later injects the corresponding host event back into the model
4. shell updates its shell-local state and view from that event

This pattern keeps the API repo-native and Bubble Tea friendly without making
the reusable package depend on modeltap runtime packages.

## Migration Mapping From Current Spike

### Mechanical extractions expected in `WU-100`

- extract token types and queue helpers from `app.go` into reusable package
- replace fake streaming ticks with host lifecycle events
- replace `/perm` demo internals with host-fed permission requests
- keep rendering/layout logic largely intact for first extraction pass
- preserve existing tests by rewriting them against the new model/event/action
  boundary rather than changing behavior

### Temporary compatibility allowance

During extraction, a demo or fake-runtime adapter may continue to exist outside
the reusable package to keep current spike scenarios runnable. That adapter may
reproduce the current fake streaming behavior, but none of that timing/runtime
logic should remain inside `internal/harnessshell`.

## Risks And Mitigations

### Risk 1 — The boundary leaks fake runtime assumptions

Cause:

- encoding `streamTickMsg`, timing delays, or fake reply generation in the
  reusable package API

Mitigation:

- allow only host lifecycle events at the API boundary
- keep fake/demo runtime behavior in an adapter package

### Risk 2 — The shell package becomes callback-heavy

Cause:

- using constructor hooks like `OnSubmit`, `OnPreview`, or `OnPermission`

Mitigation:

- represent every external request as a typed outbound action
- represent every host reply as a typed inbound event

### Risk 3 — Queue/permission semantics drift during extraction

Cause:

- flattening current mixed shell/runtime state without preserving invariants

Mitigation:

- preserve the queue, token, and permission rules above as explicit acceptance
  criteria for `WU-100` and `WU-102`

### Risk 4 — Generic package scope balloons

Cause:

- pulling sidebar/agent/palette/demo surfaces into the first reusable contract

Mitigation:

- keep `WU-098` scoped to the conversation shell contract required by
  `FEAT-0014`
- treat optional overlays as extraction-local unless `WU-099` proves they need
  first-class host integration

## Acceptance Criteria

`WU-098` is complete when:

1. the reusable shell package name and role are explicit
2. shell-owned state versus host-owned effects are explicit
3. the action/event contract covers submit, stream, interrupt, preview,
   permission, and host-native commands
4. token, queue, permission, and scroll invariants are captured in writing
5. the design is specific enough for `WU-099` to map modeltap runtime surfaces
   onto this boundary without redesigning the shell

## Next Dependency

`WU-099` should now define the modeltap host adapter in terms of this shell
contract:

- how modeltap consumes outbound actions
- how modeltap runtime emits inbound events
- which current harness/runtime surfaces survive or are translated at the
  adapter layer
