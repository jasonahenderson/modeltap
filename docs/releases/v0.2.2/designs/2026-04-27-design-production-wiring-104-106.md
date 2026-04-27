# 2026-04-27 — Design: Production Wiring (WU-104 + WU-105 + WU-106)

## Scope

This design covers three WUs that share a contract surface:

- **WU-104** — concrete `harnesshost.Runtime` implementation backed
  by surviving `internal/harness/` plumbing
- **WU-105** — new production CLI entrypoint that wraps
  `harnessshell.Model` + `harnesshost.Adapter` + the WU-104 Runtime
- **WU-106** — deletion of plumbing files the
  [WU-103 audit](2026-04-27-design-harness-audit-103.md) categorized
  as **delete**, plus the refactors it categorized as **refactor**

This design does **not** redefine:

- the `harnesshost.Runtime` interface from
  [WU-099](../../v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md);
  WU-104 implements WU-099 verbatim
- the `harnessshell` action/event types from
  [WU-098](../../v0.2.1/designs/2026-04-25-design-shell-component-api-098.md)
- the audit's keep / refactor / delete categorization from
  [WU-103](2026-04-27-design-harness-audit-103.md); WU-106 only
  mechanically applies it

## Purpose

Land the production conversation-shell experience on top of the
post-extraction architecture so v0.2.2 ships an end-user-reachable
shell that talks to a real BFF / runtime / tool dispatcher / context
manager / MCP. The shell, the adapter, and the demo runtime already
ship in v0.2.1; this design is the recipe for replacing the deleted
`modeltap harness` command with a working production equivalent.

## Architecture overview

```
                ┌──────────────────────────────────────────────────┐
                │            modeltap CLI (production)              │
                │     `modeltap shell` (or chosen final name)        │
                │                                                    │
                │   builds: harnessshell.Model                       │
                │           harnesshost.Adapter(shell, runtime)      │
                │           runtime = harnesshost.NewProductionRuntime│
                │                                                    │
                │   runs: tea.NewProgram(adapter, AltScreen, Mouse)  │
                └──────────────────────────────────────────────────┘
                                       │
                                       ▼
       ┌─────────────────────────────────────────────────────────┐
       │              harnesshost.Adapter (v0.2.1)               │
       │   ActionMsg dispatch → Runtime calls                     │
       │   harness tea.Msg → HostEvent projection                 │
       │   mid-stream pause buffer                                │
       └─────────────────────────────────────────────────────────┘
                                       │ Runtime methods
                                       ▼
       ┌─────────────────────────────────────────────────────────┐
       │     internal/harnesshost.ProductionRuntime (NEW)        │
       │                                                          │
       │   Wraps surviving internal/harness plumbing as the       │
       │   concrete Runtime impl per WU-099. Each method maps     │
       │   to existing or refactored services.                    │
       └─────────────────────────────────────────────────────────┘
            │           │            │           │           │
            ▼           ▼            ▼           ▼           ▼
       ┌────────┐  ┌──────────┐  ┌────────┐  ┌──────────┐  ┌──────────┐
       │Client  │  │Connection│  │Context │  │ Tool     │  │   MCP    │
       │(JSON-RPC)│ │ Manager  │  │Manager │  │Dispatcher│  │ Manager  │
       └────────┘  └──────────┘  └────────┘  └──────────┘  └──────────┘
```

The `ProductionRuntime` is the only component that imports the
surviving `internal/harness` plumbing. The CLI entrypoint and the
adapter never import `internal/harness`; the boundary stays clean
even after this wiring lands.

## WU-104: Concrete `harnesshost.Runtime` implementation

### Where the impl lives

`internal/harnesshost/production_runtime.go` (new file, alongside
`runtime.go` which holds the interface).

Rationale: `harnesshost` is already the modeltap-specific adapter
package per WU-099. Adding the production impl in the same package
keeps the production wiring colocated with the contract it
implements. An alternative — a sibling package
`internal/harnessrun` — was considered and rejected because it
would force a circular package boundary (the impl needs to construct
`harnesshost.Adapter` only at the CLI level, not at the impl level,
so colocating is fine).

### Constructor

```go
package harnesshost

type ProductionRuntimeConfig struct {
    ConnConfig   harness.ConnectionConfig
    ProjectRoot  string
    ServerBinary string                  // for auto-start
    SubmitKey    string                  // unused for shell; passthrough
    Registration *protocol.CapabilitiesRegister
    // Sender is set by NewProductionRuntime to a deferredSender so
    // the runtime can be constructed before tea.Program exists.
}

func NewProductionRuntime(cfg ProductionRuntimeConfig) (*ProductionRuntime, error) {
    // Build:
    //   - tools.FileTracker / Registry / PermissionEnforcer / Executor
    //   - harness.NewConnectionManager(cfg.ConnConfig, deferredSender)
    //   - harness.NewToolDispatcher(executor, sender, plan, mode)
    //   - harness.NewContextManager(cfg.ProjectRoot, tracker)
    //   - harness.NewMCPManager(...)
    //
    // Returns *ProductionRuntime that is *not yet started*. Caller
    // attaches the tea.Program to the deferred sender once the
    // adapter is wrapped in a tea.Program.
}

func (r *ProductionRuntime) AttachProgram(p *tea.Program)  { /* sets the deferredSender */ }
func (r *ProductionRuntime) Start(ctx context.Context) error { /* cm.ConnectSync */ }
func (r *ProductionRuntime) Close() error                  { /* cm.Disconnect, MCP shutdown */ }
```

### Method-by-method mapping

The order of implementation is `SubmitTurn → LoadPreview →
ResolvePermission → InterruptRun → DispatchCommand → SummarizePaste`,
per the WU-103 audit. Each method below documents inputs, the harness
service it calls, the success/failure event projection, and the
identified risk.

#### `SubmitTurn(ctx, req SubmitRequest) (SubmitAccepted, error)`

1. Resolve attachments: `req.Attachments` (from the adapter's default
   passthrough resolver) carry `TokenID`, `Kind`, `Path`, `Payload`.
   For `TokenKindFile` attachments, call
   `ContextManager.Resolve(ctx, []string{att.Path})` to convert each
   into `protocol.Attachment`. For `TokenKindPaste`, the adapter
   already passed `Payload` through; the runtime wraps it as a
   `protocol.Attachment{Kind: "paste", Content: att.Payload}`.
2. Build a `protocol.TurnSubmitRequest` from the submission text +
   attachments.
3. Call `ConnSurface.SubmitTurn(ctx, req)`. The existing harness
   pattern dispatches and waits for `TurnSubmittedMsg` later; the new
   pattern blocks inside this `tea.Cmd` goroutine until the response
   arrives. **This is the resolution to the v0.2.1-deferred async/sync
   bridge question**: the goroutine running `Runtime.SubmitTurn` waits
   on the response channel that the existing `ConnectionManager`
   already exposes for `TurnSubmittedMsg` callers (or, if no such
   channel exists, the runtime adds a thin promise-style wrapper
   around `Client().SubmitTurn`). Selected because:
   - the existing flow already produces a `TurnSubmittedMsg` with
     `TurnID` + `Err` via the deferred sender; intercepting it before
     it reaches the adapter's Update is straightforward
   - blocking inside the goroutine is safe — `tea.Cmd` runs on a
     dedicated goroutine, no UI thread is held
   - alternative (returning placeholder RunID + relying on projection
     `TurnSubmittedMsg → SubmissionAcceptedEvent`) makes the
     correlation table impossible to populate at submit time
4. On success: return `SubmitAccepted{RunID: turnID, Label: modelName}`.
5. On error: return error; the adapter's existing
   `dispatchSubmit` error path emits `SubmissionFailedEvent`.

**Risk:** the existing `ConnectionManager` may not expose a
synchronous-promise API around `Client.SubmitTurn`. If not, this WU
adds one. Concretely:

```go
// new on ConnectionManager:
func (cm *ConnectionManager) SubmitTurnSync(ctx context.Context, req *protocol.TurnSubmitRequest) (string, error)
```

Implementation: register a per-correlation-id channel, call
`Client.SubmitTurn`, wait on the channel for the matching
`TurnSubmittedMsg`, return `(turnID, err)`. The existing event-bridge
code that emits `TurnSubmittedMsg` to the deferred sender continues
unchanged — the sync wrapper is a new code path, not a refactor.

#### `LoadPreview(ctx, req PreviewRequest) (PreviewPayload, error)`

1. Validate `req.Path` against the project root via
   `ContextManager.Resolve(ctx, []string{req.Path})`. The existing
   resolver handles glob expansion; for preview we expect a single
   path, so reject globs with an error.
2. Run `tools/Read.Execute` on the resolved path to fetch content +
   metadata (line count, byte size, MIME if image).
3. Translate the tool result into `harnessshell.PreviewPayload`:
   `Title = filepath.Base(path)`, `Content = result.Output`,
   `Metadata = {"size": ..., "mime": ...}`.
4. On error: return error; the adapter emits
   `HostStatusEvent{Kind: StatusError}`.

**Risk:** none material. `tools/Read` is well-tested in v0.2.0 and
the audit listed it as keep.

#### `ResolvePermission(ctx, requestID string, decision PermissionDecision) error`

1. Translate `harnessshell.PermissionDecision` →
   `tools.PermissionDecision`: `DecisionApproveOnce` → "allow_once",
   `DecisionApproveSession` → "allow_session", `DecisionDeny` →
   "deny".
2. Call `ToolDispatcher.ResolvePermission(requestID,
   policyDecision)` (refactor on the dispatcher: the existing
   permission gate uses a channel-based wait keyed by `ToolCallID`;
   `ResolvePermission` writes the decision onto the matching
   channel). The dispatcher then continues the gated tool call,
   eventually producing a `ToolActivityMsg` end-event that the
   adapter projects to `HostStatusEvent`.
3. Per WU-099 the adapter's pause-buffer-replay is automatic (it
   forwards `PermissionResolvedEvent` from the runtime, then drains
   the buffer). The Runtime emits `PermissionResolvedEvent` with the
   tool result message.

**Risk:** the existing `ToolDispatcher` permission API is not
currently sync-promise-shaped — it uses callbacks (`PromptCallback`)
keyed on per-call channels. This WU adds a sync `ResolvePermission`
method that writes onto the existing channel; no behavior change to
the gate itself.

#### `InterruptRun(ctx, runID string) error`

1. Call `Client.SendInterrupt(ctx, runID)` — **new method** on
   `ConnProtocolClient` and `ProtocolClient`. The BFF's JSON-RPC
   protocol is expected to expose a `turn.interrupt` method that
   takes `{turn_id}`; this WU adds the wire-format call. If the BFF
   does not yet implement `turn.interrupt`, the runtime returns
   `errors.New("interrupt not yet implemented in BFF")` and the
   adapter surfaces `RunFailedEvent`.
2. On success: the runtime returns `nil`. The BFF is expected to
   emit a terminal lifecycle event (`StreamCompleteMsg` or
   `StatusUpdateMsg` with stop semantics); the adapter projects that
   into `RunStoppedEvent`.

**Risk (R4 from plan.md):** this method may require a BFF protocol
addition. If so, the BFF change is its own work item — call it
WU-104a — and the v0.2.2 release might ship with `InterruptRun`
returning the not-implemented error until WU-104a lands. The
shell's two-step Esc UX still works; the second Esc just produces a
`RunFailedEvent` instead of `RunStoppedEvent`.

#### `DispatchCommand(ctx, cmd HostCommand) error`

Routes the command name to the appropriate refactored service:

| Command | Backing service | Output |
|---------|-----------------|--------|
| `model`, `models` | `harness.models.go` (refactored: sync helpers) | `HostStatusEvent` with the catalog or current model |
| `session`, `sessions` | `harness.sessions.go` (refactored: sync helpers) | `HostStatusEvent` with the list, or transcript event row for `resume`/`fork`/`clear` |
| `context` | `harness.ContextManager.Snapshot` | `HostStatusEvent` |
| `compact` | `harness.compact.go` (refactored: sync helpers) | `HostStatusEvent` with freed-tokens narrative |
| `history` | `harness.history.go` (refactored: sync helpers) | `HostStatusEvent` with the history page |
| `mcp` | `harness.mcp.go` (refactored: sync helpers) | `HostStatusEvent` with server states |
| `plan`, `build`, `auto` | mode setter on the new state holder | `HostStatusEvent` echoing the mode |
| anything else | unrecognized command error | `HostStatusEvent{Kind: StatusError}` |

Each refactored service follows the same pattern: lift the existing
RPC-call helper out of the tea.Cmd-returning shape into a sync
method that returns `(displayText string, err error)`, then the
runtime wraps the result in a `HostStatusEvent`. The original
tea.Cmd-returning shape is deleted under WU-106.

**Risk:** R2 from the audit — the App may have hidden assumptions
about the order of msg arrivals. The refactor surfaces them via
test failures during WU-104 implementation.

#### `SummarizePaste(ctx, raw string) (string, error)`

1. Call `Client.ContentTransform(ctx, raw, "summarize")`. The
   existing harness `paste.go` already used this RPC; only the
   wrapper wraps differently.
2. Return the response text directly. On error, return error and
   the adapter falls back to the shell's built-in paste summary.

**Risk:** `harness/paste.go` is **delete** per WU-103 audit, so the
refactor extracts the RPC-call helper and inlines it here. No
behavior change.

### State holder for `ModeReader`

`harness/tool_dispatcher.go` depends on a narrow `ModeReader`
interface (`CurrentMode() protocol.Mode`). The existing implementer
is `*AppState`, which is delete per WU-103. WU-104 adds:

```go
package harnesshost

type runtimeState struct {
    mu   sync.Mutex
    mode protocol.Mode
}

func (s *runtimeState) CurrentMode() protocol.Mode { /* ... */ }
func (s *runtimeState) SetMode(m protocol.Mode)    { /* ... */ }
```

`ProductionRuntime` owns one `runtimeState`. `DispatchCommand` for
`/plan`, `/build`, `/auto` mutates it via `SetMode`.

### Test coverage

Layer 2 host-adapter tests already cover dispatch + projection; WU-104
adds Layer 2 tests against an in-memory fake `ConnectionManager`,
`ToolDispatcher`, and `ContextManager`:

- `SubmitTurn`: success returns SubmitAccepted with correct RunID;
  failure returns error with message preserved
- `LoadPreview`: file existing/non-existing; binary file produces a
  reasonable Title; oversized file truncates per `tools/Read` policy
- `ResolvePermission`: each decision resolves the matching channel;
  unknown requestID returns nil (idempotent)
- `InterruptRun`: success path; not-implemented path
- `DispatchCommand`: each command name routes to the right service;
  unknown command returns StatusError
- `SummarizePaste`: passthrough on failure

Layer 3 integration tests under `internal/harnesshost/` add an
end-to-end check that constructs `ProductionRuntime` against the
fakes, wraps in `Adapter`, drives a synthetic submit-stream-complete
sequence, and asserts the shell view contains the streamed text.

## WU-105: Production CLI entrypoint

### Command name

Final name is decided during this WU. Provisional candidates:

- `modeltap shell` — concise, parallels `shell-demo`
- `modeltap chat` — user-facing, matches industry convention
- `modeltap session` — describes the artifact rather than the action

**Recommendation: `modeltap shell`.** Parallels `shell-demo` (fake
runtime) and `shell` (real runtime); the user can compare them
side-by-side without learning a new vocabulary.

### File location

`internal/cli/shell.go` — mirrors `shell_demo.go`'s file naming.

### Command shape

```go
func newShellCommand() *cobra.Command {
    var flags shellFlags  // similar to legacy harnessFlags
    cmd := &cobra.Command{
        Use:   "shell",
        Short: "Launch the modeltap conversation shell against the BFF",
        Long: `Launch the production conversation shell. Connects to the
modeltap BFF over a local socket (auto-starting the BFF when absent),
constructs harnessshell.Model + harnesshost.Adapter +
harnesshost.ProductionRuntime, and runs as a tea.Program.

Slash commands:
  /model, /models    model catalog and switch
  /session, /sessions session list / resume / clear / fork
  /context           context window breakdown
  /compact           compaction
  /history           command history
  /mcp               MCP server state
  /plan, /build, /auto  execution mode
  /clear             shell-local transcript clear`,
        Example: `  modeltap shell
  modeltap shell --resume 7d9f...
  modeltap shell --model claude-opus-4-7
  modeltap shell --project /abs/path`,
        RunE: func(cmd *cobra.Command, args []string) error {
            return runShell(cmd, &flags)
        },
    }
    bindShellFlags(cmd, &flags)
    return cmd
}

func runShell(cmd *cobra.Command, flags *shellFlags) error {
    cfg, _, err := config.LoadWithViper("")
    // ... resolve socket path, project root, server binary ...
    runtime, err := harnesshost.NewProductionRuntime(harnesshost.ProductionRuntimeConfig{...})
    if err != nil { return err }
    defer runtime.Close()

    shell := harnessshell.New(
        harnessshell.WithLabel(cfg.DefaultModel),
        harnessshell.WithPlaceholder("Type a message and press Enter."),
    )
    adapter := harnesshost.New(shell, runtime)
    p := tea.NewProgram(adapter, tea.WithAltScreen(), tea.WithMouseAllMotion())
    runtime.AttachProgram(p)

    go func() { _ = runtime.Start(context.Background()) }()
    _, err = p.Run()
    return err
}
```

### Flags

Mirror the legacy harness flags: `--socket`, `--resume`, `--project`,
`--model`. No `--no-auto-start` — auto-start is the default and the
legacy flag was a developer escape hatch; the new shell omits it
unless WU-105 implementation surfaces a need.

### Bare `modeltap` behavior

Currently bare `modeltap` falls back to cobra's help. WU-105
re-introduces `runShell` as the bare-`modeltap` `RunE`, restoring
the v0.2.0 convenience (running `modeltap` with no args launches the
shell against the BFF).

### Tests

`internal/cli/root_test.go` updates to register `shell` in the three
subcommand tables; help output verification.

## WU-106: Plumbing cleanup

Mechanical application of the WU-103 audit's **delete** column. After
WU-104 + WU-105 have landed and tests are green, this WU:

1. Deletes the **delete** files:
   - `internal/harness/{app,app_test}.go`
   - `internal/harness/{input,input_test}.go`
   - `internal/harness/keys.go`
   - `internal/harness/{markdown,markdown_test}.go`
   - `internal/harness/{viewport,viewport_test}.go`
   - `internal/harness/{statusbar,statusbar_test}.go`
   - `internal/harness/{connux,connux_test}.go`
   - `internal/harness/{paste,paste_test}.go`
   - `internal/harness/permission_prompt.go`
   - `internal/harness/theme/` (subpackage)
   - `internal/harness/styles/` (subpackage)
2. Applies the **refactor** column:
   - Split `model.go`: keep `ConnStateInfo`, `TokenInfo`, and any
     other type that's referenced from `internal/harnesshost/projection.go`
     in a new file `internal/harness/types.go`. Delete the rest
     (`AppState`, `FocusZone`, App-internal helpers).
   - Trim `messages.go`: delete App-only msg types
     (`SubmitMsg`, `BannerMsg`, `BannerClearMsg`, `ModeChangeMsg`,
     `PasteDetectedMsg`, `PasteResolvedMsg`, `PasteSummarizeRequestMsg`,
     `historyLoadedMsg`, `TickMsg`). Keep the runtime-event types
     the projection layer consumes.
   - `compact.go`, `history.go`, `models.go`, `sessions.go`,
     `plan.go`: delete the tea.Cmd-returning App handlers; the sync
     helpers that WU-104 extracted stay.
3. Verifies `go build ./...` clean and all tests pass.
4. Updates `docs/guides/harness-shell-embedding.md` if any names
   changed during the refactor.

WU-106 is the lowest-risk part of this design because it only mutates
files that have either been covered by WU-104's tests (refactor) or
are unreachable from any CLI entry (delete).

## Implementation order across WUs

```
WU-104 starts → SubmitTurn lands → LoadPreview lands → ResolvePermission lands
                                                          │
                                                          ▼
                                              WU-105 starts (CLI uses
                                              partial Runtime; methods
                                              not-yet-implemented return
                                              clear errors that the
                                              adapter surfaces as
                                              HostStatusEvent)
                                                          │
                                                          ▼
                                              WU-104 continues:
                                              InterruptRun, DispatchCommand,
                                              SummarizePaste
                                                          │
                                                          ▼
                                              WU-105 finalizes (all
                                              Runtime methods backed)
                                                          │
                                                          ▼
                                              WU-106 deletes the
                                              audit-delete files,
                                              splits model.go,
                                              trims messages.go,
                                              applies refactor column
```

WU-105 does not block on WU-104 being feature-complete. As soon as
SubmitTurn lands, the CLI is wireable end-to-end (for users who don't
use slash commands). The remaining Runtime methods light up the
corresponding command paths as they land.

## Acceptance criteria (Phase 1 scope)

This design is accepted for Phase 2 review when:

- ✅ Every `Runtime` method is mapped to a backing service.
- ✅ The async/sync bridge for `SubmitTurn` is decided
  (block-inside-tea.Cmd-via-sync-promise-on-ConnectionManager).
- ✅ The `ModeReader` story for the deleted `AppState` is decided
  (`runtimeState` lives in `harnesshost`).
- ✅ The CLI command name is recommended (`shell`) with rationale.
- ✅ Implementation order across WUs is sequenced.
- ✅ Risks identified during the audit (R1–R5 on plan.md) are
  addressed in the design.

## Notes for Phase 2 review

The biggest single decision in this design is **R4** — the
`InterruptRun` BFF protocol method may not exist. Reviewers should
confirm:

1. Whether the BFF already implements `turn.interrupt` (or
   equivalent). If yes, the runtime just calls it. If no, this WU
   either adds a server-side change (split into WU-104a) or accepts
   the not-implemented error path.
2. Whether `ConnectionManager` already exposes a sync-promise wrapper
   around `Client.SubmitTurn`. The design assumes this WU adds one;
   reviewers may know of an existing one.
3. The CLI command name — `shell` is the recommendation; alternatives
   are `chat` or keeping `harness`.
