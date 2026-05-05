# 2026-04-25 — Design: Host Adapter and Integration (WU-099)

## Scope

This design covers **WU-099 only**:

- modeltap host adapter package layout for the extracted shell
- minimal host interface between `internal/harnessshell` and modeltap
- action consumption and host-event production
- command routing split between shell-native and host-native behavior
- submission, stream, permission, and preview integration points
- migration path away from the spike/demo runtime
- placement of the fake/demo runtime relative to the reusable shell package

This design does **not** implement the extraction (`WU-100`) or write the
embedding/docs package content (`WU-101`).

## Context

`WU-097` fixed the migration rule: the behavioral source of truth is
`FEAT-0014` plus `internal/harnessspike/`, while the later `v0.2.0` harness
line is integration inventory only. `WU-098` then fixed the shell-side
contract: `internal/harnessshell` owns the Bubble Tea conversation shell and
speaks across the boundary in typed actions and host events, with no
callback-shaped API.

`WU-099` defines the modeltap-specific side of that boundary.

The integration inventory below was captured from `internal/harness/` on the
`spike/scrolling-surface-eval` branch — the canonical home of the v0.2.0
harness/runtime line. The Phase 1 design work was authored on the
`feature/harness-shell-componentization` worktree, which did not yet contain
that line, so the inventory is recorded here in full. Phase 3 implementation
should rely on this in-doc inventory rather than reaching across local
checkouts.

## Inventory From The Later Harness Line

The sibling checkout's later harness line provides the concrete integration
surfaces the adapter must either reuse directly or translate behind a narrower
shell-facing boundary.

### Existing runtime and protocol surfaces

From `internal/harness/` in the sibling checkout:

- `app_conn.go`
  defines `ConnSurface` and `ConnProtocolClient`, already narrowed to the App's
  direct RPC needs:
  - `SubmitTurn`
  - `ContentTransform`
  - `HistoryList`
  - `ModelList` / `ModelSwitch`
  - `SessionList` / `SessionResume` / `SessionClear` / `SessionFork`
  - `ContextList`
  - `SessionCompact` / `CompactApply`
- `connection.go`
  owns connection lifecycle, registration, reconnect, and event bridging from
  protocol notifications into Bubble Tea messages
- `client.go`
  owns typed JSON-RPC calls and the protocol client
- `tool_dispatcher.go`
  owns local tool execution policy, plan-mode interception, and `tool.result`
  posting back to the server
- `permission_prompt.go`
  owns the current per-tool approval bridge between local tool execution and
  UI keystroke decisions
- `context.go`
  owns `@file`/glob resolution into `protocol.Attachment` and the `/context`
  read path

### Existing App-facing message inventory

From `internal/harness/messages.go` and the harness App:

- submit lifecycle:
  - `SubmitMsg`
  - `TurnSubmittedMsg`
- stream lifecycle:
  - `StreamTokenMsg`
  - `StreamCompleteMsg`
  - `StatusUpdateMsg`
  - `BranchStartedMsg`
  - `BranchCompleteMsg`
  - `BranchErrorMsg`
- connection and session/model state:
  - `ConnStateMsg`
  - `ModelUpdateMsg`
  - `ContextUpdateMsg`
  - `CostUpdateMsg`
- tool and permission surfaces:
  - `ToolActivityMsg`
  - `PermissionRequestMsg`
- content staging:
  - `PasteDetectedMsg`
  - `PasteResolvedMsg`
  - `PasteSummarizeRequestMsg`
- session/model slash-command responses:
  - `SessionListLoadedMsg`
  - `SessionResumedMsg`
  - `SessionClearedMsg`
  - `SessionForkedMsg`
  - `ModelListLoadedMsg`
  - `ModelSwitchedMsg`
  - `ContextListLoadedMsg`

### Existing command and submission behavior

From `internal/harness/app.go`, `sessions.go`, `models.go`, and `context.go`:

- slash commands currently handled at the host App layer:
  - `/status`
  - `/reconnect`
  - `/history`
  - `/model`, `/models`
  - `/session`, `/sessions`
  - `/context`
  - `/plan`, `/build`, `/auto`
  - `/mcp`
  - `/compact`
  - `/help`
- free-form submit path currently does this in one App:
  - append the user message immediately
  - resolve `@file` refs into attachments
  - assign session and turn identifiers
  - call `turn.submit`
  - correlate stream events back into transcript rows

### Consequence for WU-099

The adapter design should **reuse the later harness line's runtime inventory**
but **must not expose that whole inventory directly to `internal/harnessshell`**.

The shell contract should remain small and FEAT-0014-shaped:

- conversation submission
- stream/run lifecycle
- permission requests and decisions
- preview requests and results
- shell-native command handling
- host-native command dispatch

Everything else remains modeltap host composition around that boundary.

## Goals

1. Let modeltap embed `internal/harnessshell` without importing modeltap runtime
   concerns back into the reusable package.
2. Reuse the later harness line's connection/protocol/tool infrastructure where
   practical instead of inventing a second runtime stack.
3. Preserve `FEAT-0014` behavior, including queued submit, inline stream
   rendering, composer-hosted permission flow, and on-demand previews.
4. Keep the host side concrete enough that `WU-100` can extract code and
   `WU-101` can document embedding with real names and responsibilities.
5. Move the spike's fake/demo runtime out of the shell package and behind the
   same action/event boundary used by modeltap.

## Non-Goals

- reusing the current `internal/harness` App type as the reusable shell package
- freezing the final server protocol for future product features outside
  `FEAT-0014`
- redesigning the slash-command set during extraction
- replacing the later harness connection manager in this WU

## Design Summary

Modeltap should host the extracted shell through a dedicated adapter layer:

1. `internal/harnessshell` remains the reusable Bubble Tea shell package
2. `internal/harnesshost` becomes the modeltap-specific adapter package
3. the top-level harness App composes shell model + host adapter + existing
   runtime services (`ConnSurface`, tool dispatcher, context/attachment loader,
   permission enforcer)

The adapter has two jobs:

- consume shell-emitted actions and route them to modeltap services
- translate modeltap/runtime events back into shell host events

The adapter is the only package that knows both sides.

## Proposed Package Layout

### Reusable package

- `internal/harnessshell`

As defined by `WU-098`. It owns transcript/composer/queue/permission UI state
and emits typed actions.

### New host adapter package

- `internal/harnesshost`

This package is modeltap-specific and should contain:

- `adapter.go`
  top-level adapter type, constructor, action dispatcher
- `types.go`
  modeltap-specific host-facing data structs used between the adapter and the
  top-level harness App/runtime composition
- `submit.go`
  turn submission, queue-release submission, stream correlation
- `commands.go`
  command routing split and host-native command execution
- `preview.go`
  file preview / token inspection requests
- `permissions.go`
  permission request origination and decision application
- `runtime_events.go`
  projection from modeltap connection/runtime messages into shell host events
  (filename matches `WU-100`'s package-and-file plan)

`WU-100` may collapse or rename files, but this separation of responsibilities
should stand.

### Existing modeltap packages that remain in place

The host adapter should reuse, not absorb, these sibling-checkout surfaces:

- `internal/harness/app_conn.go`
  connection and RPC client surface
- `internal/harness/connection.go`
  notification/event bridge from protocol to Bubble Tea
- `internal/harness/context.go`
  attachment and preview-capable file resolution support
- `internal/harness/tool_dispatcher.go`
  tool execution path
- `internal/harness/permission_prompt.go`
  current approval bridge, but refactored so the shell owns the permission UI
  and the host adapter owns the runtime-facing request/decision linkage

### Top-level composition package

The existing top-level harness package remains the composition point for:

- process startup
- Bubble Tea program setup
- connection manager lifecycle
- tool registry / MCP setup
- theme/status/sidebar/session explorer surfaces outside the extracted shell

The top-level harness package should not bypass the adapter to manipulate shell
conversation state directly.

## Minimal Host Interface

`WU-098` defines the shell-side exported actions and host events. `WU-099`
defines the minimal modeltap-facing host interface that `internal/harnesshost`
needs in order to satisfy those actions.

Suggested internal interface shape:

```go
type Runtime interface {
    SubmitTurn(ctx context.Context, req SubmitRequest) (SubmitAccepted, error)
    InterruptRun(ctx context.Context, runID string) error
    DispatchCommand(ctx context.Context, cmd HostCommand) error
    ResolvePermission(ctx context.Context, requestID string, decision PermissionDecision) error
    LoadPreview(ctx context.Context, req PreviewRequest) (PreviewPayload, error)
    SummarizePaste(ctx context.Context, raw string) (string, error)
}
```

The `requestID` argument on `ResolvePermission` carries the same identity
used by WU-098's `ResolvePermissionAction.RequestID`. With multiple pending
permissions, the host needs the request identity to apply the decision to
the correct request.

This is intentionally narrower than `ConnProtocolClient`. The adapter may use
`ConnSurface`, `ContextManager`, tool services, and permission enforcement
behind this interface, but the shell boundary should not know those details.

### Why this is the right minimum

It covers exactly the FEAT-0014 boundary needs:

- submit work
- interrupt work
- execute host-native commands
- resolve permissions
- load previews
- summarize large pasted content

It does **not** expose:

- raw RPC client methods
- connection-manager state machine internals
- tool registry internals
- session/model/context catalog shapes except through explicit host events

## Action Consumption

The host adapter must consume the shell actions defined in `WU-098` and route
them to modeltap services as follows.

### `SubmitTurnAction`

Consumed by:

- `internal/harnesshost/submit.go`

Behavior:

- preserve shell-provided submission identity
- translate shell tokens/file refs into `protocol.Attachment` values using the
  existing attachment/context machinery where applicable
- map the shell-visible mode/session context from the outer harness state
- call the underlying runtime submit path
- emit an acceptance/failure host event back to the shell

Important design rule:

The shell already knows when a submission came from direct user submit versus
queue release. The adapter must preserve that source in correlation metadata so
later diagnostics and docs remain accurate, but the server-facing `turn.submit`
call stays one submission path.

### `InterruptRunAction`

Consumed by:

- `internal/harnesshost/submit.go`

Behavior:

- map shell run identity to the current modeltap runtime turn/run identity
- request cancellation/stop from the runtime path
- emit explicit completion/failure host events so the shell can leave its
  armed-stop state deterministically

### Shell-native commands (no action emission)

Shell-native commands do not emit a host-bound action. They are dispatched
inside the shell's update loop and the host adapter is not involved. There
is no `RunShellCommandAction` type.

Examples handled entirely inside `internal/harnessshell`:

- `/clear`
- queue release on empty `Enter` while idle (the trigger is shell-native; the
  resulting submission crosses the boundary as `SubmitTurnAction` with
  `Source = queue_release`, see WU-098)
- transcript/token-local expansion or collapse actions

Design rule:

The adapter must not be asked to execute commands that are already classified
as shell-native by `FEAT-0014`.

### `RunHostCommandAction`

Consumed by:

- `internal/harnesshost/commands.go`

Behavior:

- parse and route host-native slash commands
- fan out to the existing harness command services:
  - connection state/reconnect
  - model list/switch
  - session list/resume/clear/fork
  - context list
  - compact flow
  - MCP status/reconnect
  - history scope where retained
- translate command results into shell-visible transcript rows or banners
  without letting the shell import command-specific runtime code

## Event Production

The host adapter must project modeltap/runtime state changes back into the
shell as typed host events.

### Required host-event families

#### Submission lifecycle

- submission accepted
- submission failed

These events replace the current direct `TurnSubmittedMsg` coupling.

#### Stream lifecycle

- run started
- stream delta
- run completed
- run interrupted
- run failed

These are projected from the later harness line's existing stream inventory:

- `StreamTokenMsg`
- `StreamCompleteMsg`
- `StatusUpdateMsg`
- `BranchStartedMsg`
- `BranchCompleteMsg`
- `BranchErrorMsg`

`FEAT-0014` is single-transcript first, so multi-branch events should be
flattened into transcript sections in the shell event contract rather than
re-exposed as raw protocol notions.

#### Permission lifecycle

- permission requested
- permission updated/repeated-with-policy
- permission resolved
- permission resolution failed

These replace the current modal-banner `PermissionRequestMsg` path with a
composer-hosted shell permission surface.

#### Preview lifecycle

- preview loaded
- preview failed

These are emitted when the user requests inspection of a file/reference token.

#### Host status projection

- connection status changed
- model/mode/session context updated
- cost/context counters updated

These should remain separate from transcript events so the shell can render
status surfaces without coupling status collection to transcript rows.

## Command Routing Split

The command split must follow `FEAT-0014` and the sibling harness inventory.

### Shell-native commands

Handled entirely inside `internal/harnessshell`:

- `/clear`
- empty-`Enter` queue release while idle
- token expansion/collapse and transcript-local inspection toggles
- permission selection/navigation keys

These commands mutate shell-local state only and must not cross the host
boundary.

### Host-native commands

Handled by `internal/harnesshost`:

- `/status`
- `/reconnect`
- `/history` if retained
- `/model`, `/models`
- `/session`, `/sessions`
- `/context`
- `/plan`, `/build`, `/auto`
- `/mcp`
- `/compact`
- `/help`

These commands require runtime, session, connection, filesystem, server, or
tooling coordination.

### Routing rule

The shell performs the first classification pass:

- known shell-native command: execute locally
- anything else beginning with `/`: emit `RunHostCommandAction`

This keeps host-native command parsing centralized in the adapter while
preserving shell-native responsiveness and replayability.

## Submission Integration Points

### Turn submission

The adapter should preserve the current later-harness responsibilities while
moving transcript ownership into the shell:

Current later harness behavior:

- user message appended in the App
- attachments resolved in the App
- `turn.submit` called in the App
- stream messages appended in the App

Target behavior after extraction:

- the shell appends the user-visible row and emits `SubmitTurnAction`
- the adapter resolves attachments and runtime parameters
- the runtime path submits the turn
- the adapter feeds submit/stream lifecycle events back into the shell

This is the key host-adapter cut line.

### Queue release

Queue release remains shell-owned behavior, but its released submission still
goes through the exact same adapter submit path as a direct submit.

The only difference is action metadata:

- `Submission.Source = direct`
- `Submission.Source = queue_release`

No second runtime submission API should be introduced.

### Stream correlation

The adapter must maintain a correlation table between:

- shell submission/run IDs
- runtime turn IDs
- branch IDs when the server emits parallel review branches

This is required because:

- the shell generates stable local IDs for replayability
- the runtime/server may assign or echo canonical IDs later
- stream and tool events arrive asynchronously

## Permission Integration Points

The spike currently mixes permission UI and fake runtime behavior. The later
harness line separately has a runtime permission bridge in
`permission_prompt.go`, but that bridge assumes host-owned modal banners.

The extracted design should replace that modal approach with:

1. runtime/tool layer originates a permission request
2. host adapter translates it into a shell `PermissionRequestedEvent`
3. if a run is currently streaming, the adapter pauses delta projection (see
   "Mid-stream pause and stream buffering" below) before or alongside the
   shell event so shell-visible behavior matches `FEAT-0014`
4. the shell renders the request in transcript history and composer controls
5. user decision emits `ResolvePermissionAction` carrying `RequestID` and
   `Decision`
6. host adapter applies the decision via `Runtime.ResolvePermission(ctx,
   requestID, decision)`
7. host adapter emits `PermissionResolvedEvent` (with `Message`) or a
   resolution-failure event
8. on resolution, the adapter resumes delta projection and replays any
   buffered deltas

### Mid-stream pause and stream buffering

`FEAT-0014` requires that a permission request arriving during streaming
pauses the active stream immediately and resumes only after approval. The
spike implements this with a local stream queue (`pauseStreamingForPermission`
saves remaining stream chunks into `pausedResponse`).

After extraction, the shell no longer drives streaming directly — runtime
deltas arrive via `RunDeltaEvent` projected by the adapter. The adapter is
therefore responsible for the pause/resume effect:

- on `PermissionRequestedEvent` while a run is active, the adapter stops
  forwarding `RunDeltaEvent` to the shell and buffers any further runtime
  deltas internally
- on `PermissionResolvedEvent`, the adapter replays buffered deltas in
  arrival order before resuming live forwarding
- if the runtime/server itself naturally pauses streaming at the tool
  boundary (so no further deltas arrive while the tool is awaiting approval),
  the adapter's buffer remains empty — but the buffer logic must still exist,
  because nothing in the boundary contract requires server-side pausing

This places pause/resume semantics on the adapter rather than on the shell or
the `Runtime` interface, keeping the shell unaware of streaming-pause
mechanics and avoiding new mandatory `Runtime.PauseRun`/`ResumeRun` methods.

### Post-permission message construction

`PermissionResolvedEvent.Message` is the sole text appended to the assistant
row on resolution. The adapter constructs it from the runtime tool result
payload:

- if the runtime returns plain text, the adapter forwards it verbatim
- if the runtime returns a structured payload, the adapter renders a host-side
  text projection (typical: a brief tool-result summary) and uses that
- if the runtime returns no payload, the adapter falls back to a generic
  granted/denied message

The shell does not interpret structured runtime payloads itself; that logic
stays adapter-side.

### Stable placeholder boundary

`WU-098` intentionally keeps the permission object model minimal and stable.
`WU-099` inherits that rule:

- request IDs must be stable enough for repeated requests and composer
  navigation
- request payload should include only the shell-visible fields:
  - request ID
  - tool label
  - target
  - summary
  - remembered-policy indicator
- runtime-only details stay host-side

This lets the shell keep a stable UI contract even if the production permission
model deepens later.

## Preview Integration Points

`FEAT-0014` requires:

- paste tokens expand inline by default
- file/reference tokens remain compact until inspected on demand

The adapter therefore handles only **host-backed preview loading**.

### Paste tokens

- shell-owned
- no host round-trip needed for inline expansion

### File/reference tokens

- shell emits `LoadPreviewAction`
- adapter calls preview/file-load support from the host side
- adapter returns preview payload or failure event

The existing later harness `ContextManager` already knows how to resolve file
refs through harness-owned path and read rules. `WU-100` should reuse that
capability rather than add filesystem reads to `internal/harnessshell`.

## Migration Away From Spike/Demo Behavior

The migration should replace the current spike coupling in this order.

### Stage 1

Keep the spike as the behavioral oracle only.

No new modeltap integration code should be written into `internal/harnessspike`
once `WU-099` is accepted.

### Stage 2

Extract shell-local state and rendering into `internal/harnessshell` per
`WU-098`.

### Stage 3

Introduce `internal/harnesshost` and route all non-shell effects through it:

- submit
- stream
- permissions
- previews
- host-native commands

### Stage 4

Recompose the later harness App around:

- status/sidebar/session/model surfaces from the existing harness package
- conversation shell from `internal/harnessshell`
- adapter bridge from `internal/harnesshost`

### Stage 5

Move any remaining fake/demo behavior into `internal/harnessdemo` and delete
`internal/harnessspike` entirely. Cutover-only tests relocate to
`internal/harnesshost` integration tests per WU-102. After Stage 5 the repo
contains no package called "spike."

## Fake/Demo Runtime Placement

The fake/demo runtime must sit **outside** `internal/harnessshell`.

### Correct placement

Suggested package target:

- `internal/harnessdemo`

Responsibilities:

- produce synthetic stream lifecycle events
- originate fake permission requests for examples/tests
- exercise queue and preview flows without the real BFF/runtime

### Relationship to the reusable shell

`internal/harnessdemo` should consume the same shell action contract and emit
the same host event contract as `internal/harnesshost`.

That gives two valid host packages for the reusable shell, plus pure test
fakes for unit tests:

- `internal/harnesshost` for real modeltap integration
- `internal/harnessdemo` for examples and test fixtures
- test fakes constructed inline by shell unit tests

`internal/harnessspike` is **not** a fourth host. It is deleted as part of
v0.2.1 (see WU-100 Stage E). The shell-with-fake-data CLI capability that
`harnessspike` currently provides moves to `internal/harnessdemo`.

### Anti-goal

The fake/demo runtime must not remain compiled into the reusable shell package
as hidden fallback behavior. Doing so would re-entangle the extraction and make
future repository promotion harder.

## Concrete Guidance For WU-100

`WU-100` should implement against this integration shape:

1. build `internal/harnessshell` as the conversation shell package
2. build `internal/harnesshost` as the modeltap adapter
3. move current App-owned conversation behavior behind shell actions/events
4. keep connection manager, protocol client, tool dispatcher, and context
   loader as host-side services reused by the adapter
5. move fake/demo behavior to a separate adapter-style package

Specific extraction guardrails:

- do not let `internal/harnessshell` import `internal/protocol`
- do not let `internal/harnessshell` import connection-manager or tool
  packages directly
- do not reintroduce callback-style submit/preview/permission hooks
- do not let the top-level App mutate shell transcript state directly once the
  shell package exists

## Concrete Guidance For WU-101

`WU-101` should document the system in three layers:

1. reusable shell package: `internal/harnessshell`
2. modeltap host adapter: `internal/harnesshost`
3. optional demo host: `internal/harnessdemo`

`WU-101` must not document `internal/harnessspike` as part of the post-
extraction architecture; that package is deleted at end of release.

The embedding docs should show:

- how the outer harness program instantiates the shell
- how the adapter consumes actions
- how connection/runtime/tool events are projected back as shell host events
- which slash commands stay local versus cross the boundary

## Open Questions Deferred Past WU-099

- exact final names of every host event struct in `internal/harnessshell`
- whether `/history` remains a first-class host command in the extracted shell
  integration or is folded into another surface
- whether multi-model branch events become first-class shell data types or a
  host-side flattening concern

These are implementation and docs-shape questions, not blockers for the host
adapter design.

## Done Criteria

`WU-099` is complete when:

- the release has an explicit modeltap host adapter package target
- the minimal host interface is defined
- shell action consumption paths are mapped to modeltap services
- host-event production paths are mapped from the later harness/runtime line
- command routing split is explicit
- submit/stream/permission/preview integration points are explicit
- the fake/demo runtime is placed outside the reusable shell package
- `WU-100` and `WU-101` can proceed without reopening the host boundary
