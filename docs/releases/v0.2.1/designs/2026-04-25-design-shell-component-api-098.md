# 2026-04-25 — Design: Shell Component API and Package Boundary (WU-098)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

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

#### Theme/style import boundary

The reusable shell package must not import `internal/harness/theme` or any
modeltap-specific style constants. The package owns its own theme-neutral
style definitions in `styles.go`, exposed as `lipgloss` styles or simple
configuration values. Theme integration with a host program happens via
shell-local options, not via cross-package style imports. This rule is
binding for `WU-100` and is one of the separation requirements PATCH-0015
defines for future repository promotion.

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

#### Test package layout note

The `isHostEvent()` marker is unexported, so external test packages
(`package harnessshell_test`) cannot satisfy `HostEvent` directly. Tests that
need to construct custom host events must either live in `package harnessshell`
or use the exported concrete event types. This is intentional: the boundary
is closed-typed, and ad-hoc host events would defeat replay/serialization.

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

#### Concrete forwarding shape

The exact `tea.Msg` envelope used to deliver actions from the shell into the
host program is intentionally deferred to `WU-100`. Either an `ActionMsg`
struct (`type ActionMsg struct { Action Action }`) or a per-action concrete
message type is acceptable, provided the host loop can pattern-match on
exported types and forward to the adapter without a callback hook. The
boundary contract here is the action set itself, not the Bubble Tea wrapper.

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

##### Optimistic transcript rendering

When emitting `SubmitTurnAction`, the shell appends both the user-visible row
**and** the placeholder assistant row (with `Streaming: true`) to the
transcript before the host has acknowledged the submission. The user must
never see a state in which the user row exists without an accompanying
assistant placeholder. `RunStartedEvent` (below) carries no UX requirement
to insert that row — its only jobs are run-ID correlation and signaling that
streaming is now active. If the host fails the submission, `SubmissionFailedEvent`
removes the placeholder assistant row and surfaces failure text in its place.

This preserves the spike's `beginSubmission()` behavior, where the assistant
row exists immediately on submit and is gradually filled by stream deltas.

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

`PermissionDecision` is a closed string type:

```go
type PermissionDecision string

const (
    DecisionApproveOnce    PermissionDecision = "approve_once"
    DecisionApproveSession PermissionDecision = "approve_session"
    DecisionDeny           PermissionDecision = "deny"
)
```

The string form keeps payloads serialization-friendly for replay and logging.

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

#### `RunHostCommandAction`

Emitted for host-native slash commands only.

```go
type RunHostCommandAction struct {
    Invocation CommandInvocation
}
```

Shell-native commands such as `/clear` remain local and never emit this action.
There is no `RunShellCommandAction` — shell-native commands are dispatched
inside the shell's update loop without crossing the boundary.

## Host Event Contract

The host sends events whenever external state changes.

### Required inbound events

#### `SubmissionAcceptedEvent`

```go
type SubmissionAcceptedEvent struct {
    SubmissionID string
    RunID        string
}
```

Confirms that the host has accepted a submission and assigned a `RunID`. The
shell uses this event to correlate the optimistically-rendered assistant row
with the host run. `SubmissionAcceptedEvent` may arrive before, with, or
after the first `RunStartedEvent`; if it arrives after, the shell must
reconcile run-ID correlation against the placeholder row.

#### `SubmissionFailedEvent`

```go
type SubmissionFailedEvent struct {
    SubmissionID string
    Message      string
}
```

Indicates that the host could not accept the submission. The shell removes
the placeholder assistant row, surfaces failure text in its place, and
re-enables composer input. Queue state is not auto-released on submission
failure.

#### `RunStartedEvent`

```go
type RunStartedEvent struct {
    SubmissionID string
    RunID        string
    Label        string
}
```

Transitions the shell from submitted/queued state into active streaming state.
This event does not create or move the assistant transcript row — see
`SubmitTurnAction`'s optimistic-rendering rule. It carries `Label` (e.g., model
or agent label) for display, plus the run-ID correlation needed to scope
delta/completion events.

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
composer controls for that request. `Message` is the sole text appended to
the assistant row on resolution. The shell does not synthesize this text —
the host adapter constructs `Message` from the runtime's tool-result payload,
or from a generic granted/denied fallback when the payload is empty or
structured (see WU-099 Permission Integration Points). This keeps the
shell free of runtime-specific result interpretation.

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
    Kind   StatusKind
}

type StatusKind string

const (
    StatusReady             StatusKind = "ready"
    StatusStreaming         StatusKind = "streaming"
    StatusInterruptArmed    StatusKind = "interrupt_armed"
    StatusPermissionPending StatusKind = "permission_pending"
    StatusError             StatusKind = "error"
)
```

`Status` carries display text supplied by the host. `Kind` lets the shell make
chrome decisions (e.g., pulsing dot during streaming, interrupt-armed
styling, permission-pending highlight) without parsing the display string.
`Kind` may be left empty for backward-compatible status updates that should
not affect chrome behavior.

This split keeps the host's freedom to choose display text while giving the
shell enough signal to drive its own status surfaces.

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

#### Empty-Enter queue release: trigger vs effect

Empty `Enter` queue release is "shell-native" in the sense that the **trigger
condition** is evaluated locally: only the shell knows that the composer
buffer is empty, the run state is idle, and queued submissions are present.
But the **effect** of that release — submitting the queued work — crosses the
boundary as a normal `SubmitTurnAction` with `Source = queue_release`. There
is no second submission path.

Concretely:

1. shell observes empty Enter while idle with non-empty queue
2. shell promotes the head of `queuedSubmissions` into `pendingSubmissions`
3. shell emits `SubmitTurnAction{Submission.Source = queue_release}`
4. shell transitions to optimistic-rendering state on submit per the
   SubmitTurnAction contract above
5. host acknowledges via `SubmissionAcceptedEvent` / `SubmissionFailedEvent`
   and runs the lifecycle as for a direct submit

This avoids the host needing two intake paths while preserving the spike's
property that queue-release is initiated by shell-local logic, not by an
external command.

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
- the shell maintains two queue buffers internally:
  - `queuedSubmissions` — the visible queue rendered in the transcript
  - `pendingSubmissions` — a transient merge buffer used during release; this
    buffer holds submissions that have been promoted out of `queuedSubmissions`
    but have not yet been emitted as `SubmitTurnAction` (typically because a
    merge is in progress)

The pending merge buffer is shell-owned internal state. It is not exposed at
the action/event boundary. `WU-100` must encode this distinction in shell
state and `WU-102` must verify multi-item queue release preserves FIFO
across the merge.

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
- preserve `pendingSubmissions` as shell-owned state alongside
  `queuedSubmissions` per the queue invariants above
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
