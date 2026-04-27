# 2026-04-27 — Design: Production Wiring (WU-104a/b/c + WU-105 + WU-106)

> **Phase 2 review applied.** This document was revised on
> 2026-04-27 after processing the Codex and Kimi Phase 2 reviews.
> See `.reviews/codex-phase2-design-review.md` and
> `.reviews/kimi-phase2-design-review.md` for the full disposition
> tables. The major changes from the Phase 1 draft:
>
> - WU-104 split into three slices (a / b / c) so WU-105 can begin
>   on a stable WU-104a foundation (Codex #1).
> - Permission gating architecture rewritten — the broker logic
>   moves into `ProductionRuntime`; `ToolDispatcher` is unchanged
>   (Kimi #2 / Codex #2).
> - `Runtime.InterruptRun` calls the existing `CancelTurn` RPC
>   instead of inventing a new method; not-supported fallback
>   synthesizes `RunStoppedEvent` rather than `RunFailedEvent`
>   (Codex #4 / Kimi #7).
> - `LoadPreview` path resolution added: the adapter maintains a
>   tokenID → attachment table (Codex #3).
> - `SubmitTurnSync` double-notification problem resolved with a
>   promise-pattern that the existing event bridge writes to in
>   addition to `ProgramSender` (Kimi #4).
> - `deferredSender` concretely defined; `AttachProgram` ordering
>   tightened (Kimi #5, Kimi #6).
> - `DispatchCommand` event-surfacing mechanism specified
>   (out-of-band send via the runtime's `ProgramSender` reference)
>   (Kimi #3).
> - Bare `modeltap` does NOT change to launch shell (Codex #6
>   rejected; user-visible CLI behavior change deemed too
>   surprising for v0.2.2 scope).
> - Layer 3 test scope changes from "in-memory `ConnectionManager`
>   fake" to "real `ConnectionManager` against a `net.Listener`-
>   backed test BFF stub" (Kimi #9).
> - Other advisory findings applied (flag defaults, MCP lazy-start
>   in constructor, content.transform fallback, etc.).

## Scope

This design covers four WUs that share a contract surface:

- **WU-104a** — `Runtime.SubmitTurn` plus the supporting
  `ProductionRuntime` scaffolding (constructor, `deferredSender`,
  `AttachProgram`, correlation tables, BFF promise integration)
- **WU-104b** — `Runtime.LoadPreview`, `Runtime.ResolvePermission`,
  and `Runtime.InterruptRun`. WU-104b lands once 104a's scaffolding
  is in place.
- **WU-104c** — `Runtime.DispatchCommand` and
  `Runtime.SummarizePaste`. Lands last because its fan-out into the
  refactored harness services depends on the keep-and-refactor
  files surviving WU-106.
- **WU-105** — new production CLI entrypoint that wraps
  `harnessshell.Model` + `harnesshost.Adapter` + the WU-104a
  Runtime (and the b/c methods as they ship)
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
  mechanically applies it (with the K1 keep-list and K11 delete-list
  enumeration the Phase 2 review made explicit)

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
                │     `modeltap shell` (WU-105)                      │
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
       │   tokenID → Attachment table (NEW per Codex #3)          │
       └─────────────────────────────────────────────────────────┘
                                       │ Runtime methods
                                       ▼
       ┌─────────────────────────────────────────────────────────┐
       │     internal/harnesshost.ProductionRuntime (NEW)        │
       │                                                          │
       │   Wraps surviving internal/harness plumbing as the       │
       │   concrete Runtime impl per WU-099.                      │
       │   Owns: deferredSender, ProgramSender ref,               │
       │         submission promise channels (SubmitTurnSync),    │
       │         permission decision channels (sync.Map),         │
       │         runtimeState (ModeReader), PlanAccumulator       │
       └─────────────────────────────────────────────────────────┘
            │           │            │           │           │
            ▼           ▼            ▼           ▼           ▼
       ┌────────┐  ┌──────────┐  ┌────────┐  ┌──────────┐  ┌──────────┐
       │Client  │  │Connection│  │Context │  │ Tool     │  │   MCP    │
       │(JSON-RPC)│ │ Manager  │  │Manager │  │Dispatcher│  │ Manager  │
       └────────┘  └──────────┘  └────────┘  └──────────┘  └──────────┘
```

The CLI entrypoint (`internal/cli/shell.go`) and the
`ProductionRuntime` impl (`internal/harnesshost/production_runtime.go`)
are the packages that import `internal/harness` directly. The
`harnesshost.Adapter` itself imports `internal/harness` only via
`projection.go` for the runtime-message → HostEvent translation
layer; the action-consumer half of `Adapter.Update` does not
(corrects Kimi #16 from the Phase 1 wording).

## WU-104a: `SubmitTurn` and `ProductionRuntime` scaffolding

### Where the impl lives

`internal/harnesshost/production_runtime.go` (new file, alongside
`runtime.go` which holds the interface).

### Constructor

```go
package harnesshost

import (
    "context"
    "sync"
    "sync/atomic"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/jasonahenderson/modeltap/internal/harness"
    "github.com/jasonahenderson/modeltap/internal/harness/tools"
    "github.com/jasonahenderson/modeltap/internal/protocol"
    "github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// ProductionRuntimeConfig configures a NewProductionRuntime call.
// All fields are owned by the caller (typically the CLI entrypoint).
type ProductionRuntimeConfig struct {
    ConnConfig   harness.ConnectionConfig
    ProjectRoot  string
    SubmitKey    string                  // passthrough; not read by runtime
    Registration *protocol.CapabilitiesRegister
    MCPAutoStart bool                    // false → MCP lazy-start
}

// deferredSender wraps an atomic.Pointer[tea.Program] so the
// ProductionRuntime can be constructed before the tea.Program exists.
// Once AttachProgram lands the program reference, every Send forwards
// to tea.Program.Send. Before that, Send is a no-op (NOT a buffer):
// the AttachProgram + Start ordering rule (see WU-105) ensures no
// connection events fire before the program is attached.
type deferredSender struct {
    program atomic.Pointer[tea.Program]
}

func (d *deferredSender) Send(msg tea.Msg) {
    if p := d.program.Load(); p != nil {
        p.Send(msg)
    }
}

// ProductionRuntime is the modeltap-internal Runtime implementation.
// It satisfies harnesshost.Runtime and is owned by a single CLI
// program for the lifetime of that program's tea.Program.
type ProductionRuntime struct {
    cfg ProductionRuntimeConfig

    // Plumbing constructed in NewProductionRuntime.
    sender     *deferredSender
    cm         *harness.ConnectionManager
    tracker    *tools.FileTracker
    registry   *tools.Registry
    perms      *tools.PermissionEnforcer
    executor   *tools.Executor
    plan       *harness.PlanAccumulator
    mode       *runtimeState // implements ModeReader
    dispatcher *harness.ToolDispatcher
    ctxMgr     *harness.ContextManager
    mcp        *harness.MCPManager   // constructed but not started

    // Per-call coordination.
    submitPromises sync.Map // map[turnCorrelationID]chan turnSubmitResult
    permPromises   sync.Map // map[ToolCallID]chan harnessshell.PermissionDecision
}

func NewProductionRuntime(cfg ProductionRuntimeConfig) (*ProductionRuntime, error) {
    r := &ProductionRuntime{cfg: cfg, sender: &deferredSender{}}

    // Construction order (per Kimi #10):
    // 1. runtimeState — implements harness.ModeReader; passed into
    //    ToolDispatcher.
    r.mode = &runtimeState{mode: protocol.ModeBuild}

    // 2. PlanAccumulator — passed into ToolDispatcher; survives
    //    refactor per WU-103 audit.
    r.plan = harness.NewPlanAccumulator()

    // 3. tools framework — Registry, FileTracker, PermissionEnforcer,
    //    Executor. The PermissionEnforcer is constructed with a
    //    PromptCallback that integrates with the runtime's
    //    permission-promise map (see ResolvePermission below).
    r.tracker = tools.NewFileTracker()
    r.registry = tools.NewRegistry()
    registerBuiltinTools(r.registry, cfg.ProjectRoot, r.tracker)
    r.perms = tools.NewPermissionEnforcer(tools.PermDefault)
    r.perms.SetPromptCallback(r.permissionPromptCallback)
    r.executor = tools.NewExecutor(r.registry, r.perms)

    // 4. ToolDispatcher — uses sender for tool-result + activity
    //    notifications, mode for plan-mode interception.
    r.dispatcher = harness.NewToolDispatcher(
        r.executor, r.toolResultSender(), r.plan, r.mode,
    )

    // 5. ConnectionManager — uses deferredSender so it can be
    //    constructed before tea.Program exists. The event bridge
    //    inside connection.go writes to both the deferred sender
    //    AND to any registered submit/correlation promise (see
    //    SubmitTurn below).
    r.cm = harness.NewConnectionManager(cfg.ConnConfig, r.sender)
    r.cm.SetToolDispatcher(r.dispatcher)
    r.cm.SetSubmitPromiseRouter(r.routeTurnSubmittedToPromise) // NEW hook

    // 6. ContextManager — depends on tools.FileTracker for the
    //    Read-before-mutate rule.
    r.ctxMgr = harness.NewContextManager(cfg.ProjectRoot, r.tracker)

    // 7. MCPManager — constructed but processes are NOT spawned
    //    here. ProductionRuntime.Start may opt-in to early launch
    //    when cfg.MCPAutoStart is true; otherwise MCP lifts on the
    //    first DispatchCommand("/mcp ...") or first MCP-namespaced
    //    tool call (per WU-104c).
    r.mcp = harness.NewMCPManager(r.registry, r.tracker, /* config */)

    return r, nil
}

func (r *ProductionRuntime) AttachProgram(p *tea.Program) {
    r.sender.program.Store(p)
}

func (r *ProductionRuntime) Start(ctx context.Context) error {
    if r.cfg.MCPAutoStart {
        // Best-effort, non-blocking — MCP launch failure does not
        // abort the connection.
        go r.mcp.Launch(ctx)
    }
    return r.cm.ConnectSync(ctx)
}

func (r *ProductionRuntime) Close() error {
    if r.mcp != nil {
        _ = r.mcp.Shutdown()
    }
    if r.cm != nil {
        r.cm.Disconnect()
    }
    return nil
}
```

**WU-104a scaffolding additions to existing harness code:**

- `harness.ConnectionManager.SetSubmitPromiseRouter(routeFn)`: a new
  method that registers a callback the event bridge invokes when a
  `TurnSubmittedMsg` is about to fire. The callback takes the same
  `(turnID, sessionID, err)` triple. The event bridge continues
  delivering the message to `ProgramSender` after the callback
  returns. **This is the resolution to Kimi #4 (no double-notify):**
  both consumers see the message exactly once because the bridge
  writes to both the promise router AND the sender as a single
  unit.

- `runtimeState` (in `harnesshost`): a small struct with `CurrentMode()`
  and `SetMode()` that satisfies `harness.ModeReader`. Replaces the
  deleted `*AppState`.

- `routeTurnSubmittedToPromise(turnID, sessionID, err)`: looks up
  the per-correlation-ID channel in `submitPromises` and forwards
  the result. If no channel is registered (the message arrived
  before `SubmitTurnSync` started waiting), the routing is a no-op
  and the bridge's normal `ProgramSender` path delivers the
  message to the projection layer.

### `SubmitTurn(ctx, req SubmitRequest) (SubmitAccepted, error)`

```go
func (r *ProductionRuntime) SubmitTurn(ctx context.Context, req SubmitRequest) (SubmitAccepted, error) {
    // 1. Resolve attachments from req.Attachments → []protocol.Attachment.
    //    File tokens go through ContextManager (which respects the
    //    tools/Read project-root scope); paste tokens are wrapped
    //    inline.
    attachments, err := r.resolveAttachments(ctx, req.Attachments)
    if err != nil {
        return SubmitAccepted{}, err
    }

    // 2. Build the protocol payload + a correlation ID.
    correlationID := generateCorrelationID() // crypto/rand.Reader-backed
    rpcReq := &protocol.TurnSubmitRequest{
        SessionID:     /* from runtimeState.SessionID, set by /session resume */
        Text:          req.Text,
        Attachments:   attachments,
        CorrelationID: correlationID,
    }

    // 3. Register a promise channel BEFORE calling SubmitTurn so
    //    the event bridge cannot race past us.
    promise := make(chan turnSubmitResult, 1)
    r.submitPromises.Store(correlationID, promise)
    defer r.submitPromises.Delete(correlationID)

    // 4. Dispatch the RPC.
    if err := r.cm.Client().SubmitTurn(ctx, rpcReq); err != nil {
        return SubmitAccepted{}, err
    }

    // 5. Block on the promise. The event bridge's promise router
    //    will write to this channel when TurnSubmittedMsg fires;
    //    the bridge continues delivering the same message to the
    //    ProgramSender so the projection layer also sees it
    //    (resolves Kimi #4).
    select {
    case res := <-promise:
        if res.err != nil {
            return SubmitAccepted{}, res.err
        }
        return SubmitAccepted{
            RunID: res.turnID,
            Label: res.modelName, // populated by the runtime when known
        }, nil
    case <-ctx.Done():
        return SubmitAccepted{}, ctx.Err()
    }
}
```

The `harness.ConnectionManager.SubmitPromiseRouter` hook is the only
new surface in `internal/harness` that this WU adds; all other
`SubmitTurn` plumbing (the existing `Client.SubmitTurn` RPC, the
`TurnSubmittedMsg` event bridge) is reused unchanged.

### Test coverage for WU-104a

Layer 2 host-adapter tests already cover dispatch + projection;
WU-104a adds tests against an in-memory fake `ConnSurface` /
`ProtocolClient`:

- `SubmitTurn`: success path returns `SubmitAccepted` with correct
  RunID; failure path propagates the error.
- `SubmitTurnSync` no-double-notify: the test fake bridge fires
  `TurnSubmittedMsg` once; the promise wakes up; the projection
  layer also sees the message exactly once.
- `deferredSender`: Send before `AttachProgram` is a no-op; Send
  after forwards correctly.
- Correlation lifecycle: the promise map is empty after
  `SubmitTurn` returns (success and error paths).

**Layer 3 integration scope** (per Kimi #9): rather than faking the
full `ConnectionManager`, WU-104a stands up a `testutil` BFF stub —
a `net.Listener`-backed JSON-RPC server that speaks the subset the
integration test exercises (`turn.submit` ack + a synthetic
`StreamTokenMsg`/`StreamCompleteMsg` pair). The stub lives at
`internal/harnesshost/testutil/bffstub.go`. WU-104a is responsible
for landing it; subsequent slices reuse it.

## WU-104b: `LoadPreview`, `ResolvePermission`, `InterruptRun`

### `LoadPreview(ctx, req PreviewRequest) (PreviewPayload, error)`

```go
func (r *ProductionRuntime) LoadPreview(ctx context.Context, req PreviewRequest) (harnessshell.PreviewPayload, error) {
    // req.Path comes from the adapter's tokenID → Attachment table
    // (populated on every SubmitTurnAction dispatch and on composer-
    // token state injection — see Adapter changes below).
    if req.Path == "" {
        return harnessshell.PreviewPayload{}, errors.New("preview: path unresolved for token " + req.TokenID)
    }

    resolved, err := r.ctxMgr.ResolveOne(ctx, req.Path) // single-path Resolve helper
    if err != nil {
        return harnessshell.PreviewPayload{}, err
    }

    readResult, err := r.runReadTool(ctx, resolved.Path)
    if err != nil {
        return harnessshell.PreviewPayload{}, err
    }

    return harnessshell.PreviewPayload{
        Title:    filepath.Base(resolved.Path),
        Content:  readResult.Output,
        Metadata: readResult.Metadata,
    }, nil
}
```

#### Adapter change (Codex #3): tokenID → Attachment table

The `harnesshost.Adapter` gains:

```go
type Adapter struct {
    // ... existing fields ...

    // tokenAttachments correlates shell-emitted token IDs to the
    // resolved Attachment so LoadPreviewAction can populate
    // PreviewRequest.Path. Populated on dispatchSubmit (from
    // Submission.Tokens) and via a host-side helper for composer
    // tokens injected outside a submission lifecycle.
    tokenAttachments map[string]Attachment
}
```

`dispatchSubmit` writes every resolved attachment into
`tokenAttachments` keyed by `Attachment.TokenID`.
`dispatchLoadPreview` reads the table to build
`PreviewRequest{TokenID, Path: tokenAttachments[TokenID].Path,
Source}`. If the lookup misses, the runtime returns the
"path unresolved" error and the adapter projects it as
`HostStatusEvent{Kind: StatusError}`.

**Cleanup:** `tokenAttachments` entries persist for the lifetime of
the Adapter. Memory growth is bounded because the shell's
`InputToken.ID`s are scoped per-session-of-shell and a typical
session never accumulates more than ~hundreds of tokens. If this
turns out to be a leak in practice, a future refactor can
garbage-collect entries on `RunCompletedEvent`.

### `ResolvePermission(ctx, requestID string, decision PermissionDecision) error`

**Architecture (per Kimi #2 — supersedes the Phase 1 design):** the
permission broker logic moves into `ProductionRuntime`.
`ToolDispatcher` is unchanged; the existing `PermissionEnforcer`
callback mechanism is reused.

```go
// permissionPromptCallback is registered on tools.PermissionEnforcer
// in NewProductionRuntime. It runs on the tool-dispatch goroutine
// and blocks until the user resolves the permission via
// Runtime.ResolvePermission.
func (r *ProductionRuntime) permissionPromptCallback(req tools.PermissionRequest) tools.PermissionDecision {
    promise := make(chan harnessshell.PermissionDecision, 1)
    r.permPromises.Store(req.ToolCallID, promise)
    defer r.permPromises.Delete(req.ToolCallID)

    // Emit harness.PermissionPromptMsg through the deferred sender.
    // The adapter's projection layer translates it to
    // harnessshell.PermissionRequestedEvent and forwards it through
    // forwardEvent, which registers the request in pendingPermissions
    // and starts buffering RunDeltaEvents.
    r.sender.Send(harness.PermissionPromptMsg{
        ToolCallID:  req.ToolCallID,
        ToolName:    req.ToolName,
        RiskLevel:   req.RiskLevel,
        Description: req.Summary,
        Input:       req.Input,
    })

    // Block until ResolvePermission writes the decision.
    decision := <-promise
    return shellDecisionToToolDecision(decision)
}

// ResolvePermission is the Runtime method the adapter calls when the
// shell emits ResolvePermissionAction.
func (r *ProductionRuntime) ResolvePermission(ctx context.Context, requestID string, decision harnessshell.PermissionDecision) error {
    raw, ok := r.permPromises.Load(requestID)
    if !ok {
        // Idempotent: unknown requestID means the gate already
        // resolved (or never existed). Not an error.
        return nil
    }
    promise := raw.(chan harnessshell.PermissionDecision)
    select {
    case promise <- decision:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Channel full means the gate already received a decision.
        // Idempotent.
        return nil
    }
}
```

The adapter's existing pause buffer (Stage D-3 in v0.2.1) handles
`RunDeltaEvent` buffering automatically once
`PermissionRequestedEvent` flows through `forwardEvent`. When the
user resolves the permission, the shell emits
`ResolvePermissionAction` → adapter dispatches
`Runtime.ResolvePermission` → the dispatch goroutine wakes up →
`ToolDispatcher` continues the gated tool call → the runtime emits
the tool-result `ToolActivityMsg` end-event → projection layer
synthesizes `PermissionResolvedEvent` (the runtime constructs the
`Message` from the tool result) → adapter drains the pause buffer.

### `InterruptRun(ctx, runID string) error`

**Use existing `turn.cancel` (per Codex #4):** `internal/harness/client.go`
already implements `CancelTurn(ctx, turnID)` against
`protocol.MethodTurnCancel`.

```go
func (r *ProductionRuntime) InterruptRun(ctx context.Context, runID string) error {
    err := r.cm.Client().CancelTurn(ctx, runID)
    if err != nil {
        // Per Kimi #7: synthesize RunStoppedEvent (not RunFailedEvent)
        // so the shell's transcript shows "Run stopped" instead of a
        // red error when the BFF doesn't support cancellation.
        // The adapter's existing dispatchInterrupt error path expects
        // RunFailedEvent; we override here by sending the synthetic
        // event directly through the deferred sender.
        r.sender.Send(harnessshell.RunStoppedEvent{
            RunID:   runID,
            Reason:  harnessshell.StopReasonInterrupt,
            Message: "stopped — backend reported: " + err.Error(),
        })
        // Return nil so dispatchInterrupt doesn't ALSO emit
        // RunFailedEvent (the synthetic event already covers UX).
        return nil
    }
    // Success path: turn.cancel succeeded. The BFF is expected to
    // emit a terminal lifecycle event which the projection layer
    // translates into RunStoppedEvent.
    return nil
}
```

### Test coverage for WU-104b

- `LoadPreview`: existing path, missing path, oversized path
  truncation per `tools/Read` policy, `tokenAttachments` table
  population on submit + lookup on preview.
- `ResolvePermission`: each decision (`ApproveOnce`,
  `ApproveSession`, `Deny`) wakes the matching channel and
  resolves the gate; unknown requestID is a no-op (idempotent);
  context cancellation propagates.
- `InterruptRun`: `CancelTurn` success returns nil; `CancelTurn`
  error path emits synthetic `RunStoppedEvent` via the sender and
  returns nil (test asserts the sender received the event).
- Pause-buffer integration: a permission requested mid-stream
  buffers deltas; `ResolvePermission` resolves the gate and the
  buffered deltas replay (already covered by
  `internal/harnesshost/pause_test.go`; a thin smoke test is added
  here against `ProductionRuntime` to confirm the integration).

## WU-104c: `DispatchCommand`, `SummarizePaste`

### `DispatchCommand(ctx, cmd HostCommand) error`

**Event-surfacing mechanism (per Kimi #3):** `ProductionRuntime`
emits `HostStatusEvent` directly via `r.sender.Send` for command
results. Documented as **out-of-band** relative to the
action-event cycle because command results originate from
host-native commands, not from shell-emitted actions, and the
adapter's correlation tables and pause buffer don't need to
process them.

```go
func (r *ProductionRuntime) DispatchCommand(ctx context.Context, cmd HostCommand) error {
    switch cmd.Name {
    case "model":
        return r.handleModelCommand(ctx, cmd.Args)
    case "models":
        return r.handleModelsCommand(ctx)
    case "session":
        return r.handleSessionCommand(ctx, cmd.Args)
    case "sessions":
        return r.handleSessionsCommand(ctx)
    case "context":
        return r.handleContextCommand(ctx)
    case "compact":
        return r.handleCompactCommand(ctx)
    case "history":
        return r.handleHistoryCommand(ctx, cmd.Args)
    case "mcp":
        return r.handleMCPCommand(ctx, cmd.Args) // lazy-launches MCP if needed
    case "plan", "build", "auto":
        return r.handleModeCommand(cmd.Name)
    default:
        r.sender.Send(harnessshell.HostStatusEvent{
            Status: "Unknown command: /" + cmd.Name,
            Kind:   harnessshell.StatusError,
        })
        return nil
    }
}
```

Each `handle*Command` calls into the refactored harness service
(`harness.compact.go`, `harness.history.go`, etc. — sync helpers
extracted under WU-106's refactor pass) and emits the result as
`HostStatusEvent`. A representative one:

```go
func (r *ProductionRuntime) handleModelsCommand(ctx context.Context) error {
    catalog, err := harness.ListModelsSync(ctx, r.cm.Client())
    if err != nil {
        r.sender.Send(harnessshell.HostStatusEvent{
            Status: "Models unavailable: " + err.Error(),
            Kind:   harnessshell.StatusError,
        })
        return nil
    }
    r.sender.Send(harnessshell.HostStatusEvent{
        Status: formatModelCatalog(catalog),
        Kind:   harnessshell.StatusReady,
    })
    return nil
}
```

The `Runtime.DispatchCommand` interface returns `error`; here the
return value indicates whether the dispatch *itself* failed
(unknown command, panic-style failure). Command-result errors are
emitted as `HostStatusEvent{Kind: StatusError}` and the method
returns nil because the command was successfully dispatched even
though the underlying RPC failed.

### `SummarizePaste(ctx, raw string) (string, error)`

```go
func (r *ProductionRuntime) SummarizePaste(ctx context.Context, raw string) (string, error) {
    summary, err := r.cm.Client().ContentTransform(ctx, raw, "summarize")
    if err != nil {
        // Per Kimi #17: fall back to passthrough on RPC error so
        // the shell uses its built-in paste summary instead of
        // failing the paste capture.
        return raw, nil
    }
    return summary, nil
}
```

### Test coverage for WU-104c

- `DispatchCommand`: each command name routes to the right service;
  unknown command returns `nil` and emits `HostStatusEvent{Kind:
  StatusError}` via the sender (test asserts the sender received).
- `SummarizePaste`: success returns transformed text; RPC error
  returns `(raw, nil)` per fallback.
- MCP lazy-start: `DispatchCommand("/mcp status")` triggers
  `MCPManager.Launch` if not already started; subsequent calls are
  idempotent.

## WU-105: Production CLI entrypoint

### Command name

Final name: **`modeltap shell`** (parallels `shell-demo`).

### File location

`internal/cli/shell.go`.

### Command shape

```go
func newShellCommand() *cobra.Command {
    var flags shellFlags
    cmd := &cobra.Command{
        Use:   "shell",
        Short: "Launch the modeltap conversation shell against the BFF",
        Long: `Launch the production conversation shell ...`,
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
    if err != nil {
        return fmt.Errorf("loading config: %w", err)
    }

    runtime, err := harnesshost.NewProductionRuntime(harnesshost.ProductionRuntimeConfig{
        ConnConfig:   buildConnConfig(cfg, flags, cmd),
        ProjectRoot:  resolveProjectRoot(flags.project),
        SubmitKey:    cfg.Harness.SubmitKey,
        Registration: buildRegistration(cmd),
        MCPAutoStart: cfg.MCP.AutoStart, // false by default; lazy
    })
    if err != nil {
        return err
    }
    defer runtime.Close()

    shell := harnessshell.New(
        harnessshell.WithLabel(resolveLabel(cfg, flags)),
        harnessshell.WithPlaceholder("Type a message and press Enter."),
    )
    adapter := harnesshost.New(shell, runtime)

    p := tea.NewProgram(adapter, tea.WithAltScreen(), tea.WithMouseAllMotion())

    // Ordering (per Kimi #6): AttachProgram BEFORE Start so the
    // deferredSender's program reference is non-nil before any
    // connection event fires.
    runtime.AttachProgram(p)
    go func() { _ = runtime.Start(context.Background()) }()

    _, err = p.Run()
    return err
}
```

### Flag defaults (per Kimi #15)

| Flag | Default | Source |
|------|---------|--------|
| `--socket` | viper `bff.socket_path` then `config.DefaultBFFSocketPath()` | command flag → config → built-in |
| `--resume` | empty (new session) | command flag only |
| `--project` | `os.Getwd()` | command flag → cwd |
| `--model` | viper `default_model` (empty when unset) | command flag → config |

### Bare `modeltap` behavior

**Bare `modeltap` does NOT change** (per Codex #6 disposition).
v0.2.1 deliberately stripped the bare-launch behavior when the
broken legacy harness was scrapped; restoring it now would surprise
users with an auto-BFF-start path that hasn't been exercised in
v0.2.2 yet. Bare `modeltap` continues to fall back to cobra's
default help. The `internal/cli/root_test.go` help-fallback
assertion stays valid.

A future release may reintroduce bare-launch as a deliberate UX
decision; that decision will get its own ADMIN entry rather than
being smuggled in here.

### Tests

`internal/cli/root_test.go` updates:

- `TestSubcommandsRegistered`: `shell` is registered.
- `TestSubcommandsAcceptHelp`: `shell --help` succeeds.
- `TestHelpListsAllSubcommands`: `shell` appears in help output.

End-to-end `internal/cli/shell_test.go` exercises the `runShell`
construction up to (but not running) `p.Run` against the
`testutil/bffstub` from WU-104a. Asserts `runtime.AttachProgram(p)`
runs before `runtime.Start` and that early connection events
(synthetic) reach the projection layer via the deferred sender.

## WU-106: Plumbing cleanup

Mechanical application of the WU-103 audit's keep / refactor /
delete columns. After WU-104 + WU-105 have landed and tests are
green, this WU:

1. Deletes the **delete** files. Explicit list (per Kimi #11 and
   Kimi #12 verification):
   - `internal/harness/{app,app_test}.go`
   - `internal/harness/{input,input_test}.go`
   - `internal/harness/keys.go`
   - `internal/harness/{markdown,markdown_test}.go`
   - `internal/harness/{viewport,viewport_test}.go`
   - `internal/harness/{statusbar,statusbar_test}.go`
   - `internal/harness/{connux,connux_test}.go`
   - `internal/harness/{paste,paste_test}.go`
   - `internal/harness/permission_prompt.go` (broker logic moved
     to `ProductionRuntime` in WU-104b)
   - `internal/harness/theme/` (subpackage; verified-grep no
     surviving references)
   - `internal/harness/styles/` (subpackage; verified-grep no
     surviving references)
2. Applies the **refactor** column:
   - **Split `model.go`:** keep `ConnStateInfo`, `TokenInfo`, and
     other types referenced from `internal/harnesshost/projection.go`
     in a new file `internal/harness/types.go`. Delete `AppState`,
     `FocusZone`, **`DisplayMessage`** (per Kimi #11), and other
     App-internal helpers.
   - **Trim `messages.go`:** apply the WU-103 audit's enumerated
     keep list (13 types) and delete list (9 types). Build will
     fail the moment a needed type is dropped because
     `internal/harnesshost/projection.go` will not compile.
   - **`compact.go` / `history.go` / `models.go` / `sessions.go`:**
     delete the tea.Cmd-returning App handlers; the sync helpers
     extracted under WU-104c (`ListModelsSync`, `ListSessionsSync`,
     `CompactPlanSync`, `HistoryPageSync`, etc.) stay.
   - **`plan.go`:** keep `PlanAccumulator` (used by
     `ProductionRuntime`); delete the App-banner emission code.
3. Verifies `go build ./...` clean and all tests pass.
4. Updates `docs/guides/harness-shell-embedding.md` if any names
   changed during the refactor.

WU-106 is the lowest-risk part of this design because it only
mutates files that have either been covered by WU-104's tests
(refactor) or are unreachable from any CLI entry (delete).

## Implementation order across WUs

```
WU-104a (SubmitTurn + ProductionRuntime scaffolding + testutil/bffstub)
   │
   ├──→ WU-105 (CLI uses 104a Runtime; b/c methods not-yet-implemented
   │             return clear errors that surface as HostStatusEvent)
   │
   └──→ WU-104b (LoadPreview + ResolvePermission + InterruptRun)
            │
            └──→ WU-104c (DispatchCommand + SummarizePaste +
                          MCP lazy-start integration)
                      │
                      └──→ WU-106 (cleanup: delete + refactor)
```

WU-105 starts as soon as WU-104a's `ProductionRuntime` scaffolding
+ `SubmitTurn` lands. The CLI is then wireable end-to-end (for
users who only submit text and read responses); the b/c methods
light up the corresponding command paths as they ship. This is
the resolution to Codex #1 — WU-105 has a stable WU-104a
foundation rather than relying on an in-progress 104.

## Acceptance criteria (Phase 1 scope, post-Phase-2 review)

This design is accepted for Phase 3 implementation when:

- ✅ Every `Runtime` method is mapped to a backing service.
- ✅ The async/sync `SubmitTurn` bridge is decided
  (promise-pattern with the existing event bridge writing to
  both `ProgramSender` and the per-correlation promise).
- ✅ Permission gating architecture is decided (broker logic in
  `ProductionRuntime`; `ToolDispatcher` unchanged; `sync.Map` of
  per-`ToolCallID` channels).
- ✅ `InterruptRun` uses existing `CancelTurn`; not-supported
  fallback synthesizes `RunStoppedEvent`.
- ✅ `LoadPreview` path resolution defined (adapter
  tokenID→Attachment table).
- ✅ `DispatchCommand` event-surfacing mechanism defined
  (out-of-band sender).
- ✅ `deferredSender` concretely defined.
- ✅ `AttachProgram` ordering rule documented.
- ✅ Bare `modeltap` does NOT change to launch shell.
- ✅ Layer 3 test scope realistic (`net.Listener` BFF stub).
- ✅ Construction order in `NewProductionRuntime` explicit.
- ✅ Implementation order across WU-104a/b/c + WU-105 + WU-106
  sequenced.

## Notes for follow-on review (if any)

The design now resolves the Phase 2 reviews' blocking and
significant findings. Advisory findings #11 (DisplayMessage),
#15 (flag defaults), #16 (adapter-imports statement), #17
(SummarizePaste fallback), and #18 (MCP lazy-start) are
incorporated above. Any further review should focus on the new
surfaces this revision introduced:

- `harness.ConnectionManager.SetSubmitPromiseRouter` — the
  smallest possible new surface; reviewers should confirm this is
  preferable to alternative bridge integrations.
- `tokenAttachments` lifetime in `harnesshost.Adapter` — the
  unbounded map is acceptable for v0.2.2 scope; reviewers should
  flag if shell-demo or shell are expected to run indefinitely.
- `ProductionRuntime.permissionPromptCallback`'s blocking semantics
  — the dispatch goroutine blocks indefinitely on the channel; if
  the user never resolves a permission, the tool call hangs.
  WU-104b adds a timeout (default 5 minutes, configurable) plus a
  cancellation path triggered by `Runtime.Close`.
