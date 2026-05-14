package harnesshost

// ProductionRuntime is the modeltap-internal Runtime implementation for
// the production conversation-shell entrypoint. It wraps the surviving
// `internal/harness` plumbing (ConnectionManager, ProtocolClient,
// ToolDispatcher, ContextManager, MCP) into the harnesshost.Runtime
// contract per WU-099.
//
// WU-104a lands SubmitTurn + the supporting scaffolding (constructor,
// deferredSender, AttachProgram, runtimeState, permission promise map).
// WU-104b adds LoadPreview, ResolvePermission, InterruptRun.
// WU-104c adds DispatchCommand, SummarizePaste, MCP lazy-start.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harness/tools"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// ProductionRuntimeConfig configures a [NewProductionRuntime] call.
// All fields are owned by the caller (typically the CLI entrypoint).
type ProductionRuntimeConfig struct {
	ConnConfig   harness.ConnectionConfig
	ProjectRoot  string
	Registration *protocol.CapabilitiesRegister

	// MCPAutoStart, when true, launches the MCP server processes during
	// [ProductionRuntime.Start]. When false (default), MCP launches on
	// the first /mcp command or first MCP-namespaced tool call (WU-104c).
	MCPAutoStart bool

	// PermissionTimeout bounds the time a tool execution may block on
	// the user's permission decision. Zero defers to a 5-minute
	// default. Tests pass a small value to drive timeout paths.
	PermissionTimeout time.Duration
}

// deferredSender wraps an atomic.Pointer[tea.Program] so the
// ProductionRuntime can be constructed before the tea.Program exists.
// Once [ProductionRuntime.AttachProgram] lands the program reference,
// every Send forwards to tea.Program.Send. Before that, Send is a
// no-op — the AttachProgram + Start ordering rule (in WU-105's CLI
// snippet) ensures no events fire before the program is attached.
type deferredSender struct {
	program atomic.Pointer[tea.Program]
	onSend  func(tea.Msg)
}

// Send satisfies harness.ProgramSender. Drops messages silently when
// no program is attached.
func (d *deferredSender) Send(msg tea.Msg) {
	if d.onSend != nil {
		d.onSend(msg)
	}
	if p := d.program.Load(); p != nil {
		p.Send(msg)
	}
}

// runtimeState tracks the modeltap-side session state that the
// harness's existing AppState used to hold. It implements
// harness.ModeReader so harness.ToolDispatcher can read the current
// execution mode.
type runtimeState struct {
	mu          sync.Mutex
	mode        protocol.Mode
	sessionID   string
	activeRunID string
	sequence    int
	label       string // current model label
}

func newRuntimeState() *runtimeState {
	return &runtimeState{mode: protocol.ModeBuild}
}

// CurrentMode satisfies harness.ModeReader.
func (s *runtimeState) CurrentMode() protocol.Mode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mode
}

// SetMode updates the current execution mode.
func (s *runtimeState) SetMode(m protocol.Mode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = m
}

// SessionID returns the current session ID (empty before the server
// assigns one).
func (s *runtimeState) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

// SetSessionID records the server-assigned session ID. Subsequent
// turns on this session reuse it.
func (s *runtimeState) SetSessionID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionID = id
}

func (s *runtimeState) ActiveRunID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeRunID
}

func (s *runtimeState) SetActiveRunID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeRunID = id
}

// NextSequence returns the next sequence counter for this session.
func (s *runtimeState) NextSequence() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	return s.sequence
}

// Label returns the current model/agent label.
func (s *runtimeState) Label() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.label
}

// SetLabel updates the model/agent label.
func (s *runtimeState) SetLabel(l string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.label = l
}

// ProductionRuntime is the modeltap-internal Runtime implementation.
// It satisfies harnesshost.Runtime and is owned by a single CLI program
// for the lifetime of that program's tea.Program.
type ProductionRuntime struct {
	cfg ProductionRuntimeConfig

	sender     *deferredSender
	cm         *harness.ConnectionManager
	tracker    *tools.FileTracker
	registry   *tools.Registry
	perms      *tools.PermissionEnforcer
	executor   *tools.Executor
	plan       *harness.PlanAccumulator
	mode       *runtimeState
	dispatcher *harness.ToolDispatcher
	ctxMgr     *harness.ContextManager

	// Per-call coordination.

	// permPromises bridges the executor's PromptCallback (running on
	// the dispatcher goroutine) to ResolvePermission (running on the
	// adapter's tea.Cmd goroutine). Keys are runtime-generated request
	// IDs (the harness.PermissionPromptMsg.ToolCallID field carries
	// them through the adapter's projection layer).
	permPromises sync.Map // map[string]chan harnessshell.PermissionDecision
	permCounter  atomic.Uint64
}

// NewProductionRuntime constructs a ProductionRuntime ready for
// [AttachProgram] + [Start]. The returned runtime's Bubble Tea
// integration is dormant until AttachProgram lands the program
// reference; methods may be called before Start, but message-emitting
// operations will silently drop until the program is attached.
func NewProductionRuntime(cfg ProductionRuntimeConfig) (*ProductionRuntime, error) {
	if cfg.PermissionTimeout == 0 {
		cfg.PermissionTimeout = 5 * time.Minute
	}

	r := &ProductionRuntime{
		cfg:    cfg,
		sender: &deferredSender{},
		mode:   newRuntimeState(),
	}
	r.sender.onSend = r.observeRuntimeMessage

	// Construction order per WU-104 design (Kimi #10):
	// 1. PlanAccumulator (passed into ToolDispatcher).
	r.plan = harness.NewPlanAccumulator()

	// 2. Tools framework: tracker → registry → permissions → executor.
	r.tracker = tools.NewFileTracker()
	r.registry = tools.NewRegistry()
	registerBuiltinTools(r.registry, cfg.ProjectRoot, r.tracker)
	r.perms = tools.NewPermissionEnforcer(tools.PermDefault)
	r.executor = tools.NewExecutor(r.registry, r.perms)
	r.executor.SetPromptCallback(r.permissionPromptCallback)

	// 3. ConnectionManager — uses the deferredSender so it can be
	//    constructed before tea.Program exists. The event bridge
	//    inside connection.go writes to the deferred sender; the
	//    adapter's projection layer translates harness.* tea.Msgs
	//    into HostEvents.
	r.cm = harness.NewConnectionManager(cfg.ConnConfig, r.sender)

	// 4. ToolDispatcher — uses an internal toolResultSender adapter
	//    around the connection manager so it can re-read the live
	//    ProtocolClient on each call (survives reconnects).
	r.dispatcher = harness.NewToolDispatcher(
		r.executor,
		newToolResultSender(r.cm),
		r.plan,
		r.mode,
	)
	r.cm.SetToolDispatcher(r.dispatcher)

	// 5. ContextManager — depends on tools.FileTracker for Read-before-
	//    mutate semantics.
	r.ctxMgr = harness.NewContextManager(cfg.ProjectRoot, r.tracker)

	return r, nil
}

// AttachProgram binds the ProductionRuntime's outbound message channel
// to the given tea.Program. Must be called before any connection event
// can fire (i.e., before [Start]) — otherwise early ConnStateMsg /
// TurnSubmittedMsg / etc. will be silently dropped.
func (r *ProductionRuntime) AttachProgram(p *tea.Program) {
	r.sender.program.Store(p)
}

// Start begins the connection lifecycle. Returns when the connection
// is established (or fails). The caller typically invokes Start in a
// goroutine so the tea.Program's main loop can begin handling messages
// while the connection is still being set up.
func (r *ProductionRuntime) Start(ctx context.Context) error {
	if err := r.cm.ConnectSync(ctx); err != nil {
		return err
	}
	r.resumeKnownRuns(ctx)
	return nil
}

// Close shuts down the connection.
func (r *ProductionRuntime) Close() error {
	if r.cm != nil {
		r.cm.Disconnect()
	}
	return nil
}

// SubmitTurn implements [harnesshost.Runtime]. Resolves attachments
// via the ContextManager, dispatches turn.submit through the live
// ProtocolClient, and returns the runtime-assigned RunID (the
// server-echoed TurnID, falling back to the harness-assigned one on
// empty echo). The Bubble Tea bridge is intentionally bypassed for
// the submit response — we use the synchronous Client.SubmitTurn
// directly so the ack is correlated at submit time.
func (r *ProductionRuntime) SubmitTurn(ctx context.Context, req SubmitRequest) (SubmitAccepted, error) {
	client := r.cm.Client()
	if client == nil {
		return SubmitAccepted{}, errors.New("no live runtime client")
	}

	content, attachments, err := r.buildSubmitContent(ctx, req)
	if err != nil {
		return SubmitAccepted{}, err
	}

	turnID := "turn-" + uuid.NewString()
	submit := &protocol.TurnSubmit{
		TurnID:      turnID,
		SessionID:   r.mode.SessionID(),
		Sequence:    r.mode.NextSequence(),
		Mode:        r.mode.CurrentMode(),
		Content:     content,
		Attachments: attachments,
	}

	ack, err := client.SubmitTurn(ctx, submit)
	if err != nil {
		return SubmitAccepted{}, err
	}

	finalID := ack.TurnID
	if ack.RunID != "" {
		finalID = ack.RunID
	}
	if finalID == "" {
		finalID = turnID
	}
	if ack.SessionID != "" {
		r.mode.SetSessionID(ack.SessionID)
	}
	r.mode.SetActiveRunID(finalID)

	return SubmitAccepted{
		RunID: finalID,
		Label: r.mode.Label(),
	}, nil
}

// InterruptRun implements [harnesshost.Runtime] using the existing
// ProtocolClient.CancelTurn against protocol.MethodTurnCancel (per
// the Phase 2 review's Codex #4 disposition — no new RPC needed).
//
// On error the runtime synthesizes harnessshell.RunStoppedEvent
// (per Kimi #7) so the shell's transcript shows a clean stop instead
// of a red error when the Runtime doesn't support cancellation.
func (r *ProductionRuntime) InterruptRun(ctx context.Context, runID string) error {
	client := r.cm.Client()
	if client == nil {
		// Synthesize the stop event directly through the sender so
		// the shell sees a clean RunStoppedEvent rather than a
		// failed run from the adapter's default error path.
		r.sender.Send(harnessshell.RunStoppedEvent{
			RunID:   runID,
			Reason:  harnessshell.StopReasonInterrupt,
			Message: "stopped — no live runtime client",
		})
		return nil
	}
	if err := client.CancelRun(ctx, runID); err != nil {
		if turnErr := client.CancelTurn(ctx, runID); turnErr != nil {
			r.sender.Send(harnessshell.RunStoppedEvent{
				RunID:   runID,
				Reason:  harnessshell.StopReasonInterrupt,
				Message: "stopped — backend reported: " + err.Error(),
			})
			return nil
		}
	}
	// Success: the Runtime accepted the cancel. Synthesize the stop
	// event ourselves; the existing harness streaming layer doesn't
	// emit a terminal Run* message on cancel, so we surface one
	// directly to keep the shell's chrome consistent.
	r.sender.Send(harnessshell.RunStoppedEvent{
		RunID:  runID,
		Reason: harnessshell.StopReasonInterrupt,
	})
	return nil
}

// DispatchCommand implements [harnesshost.Runtime] by routing each
// host-native command to the appropriate ProtocolClient RPC and
// emitting a HostStatusEvent with the result through the
// deferredSender. Per Kimi #3 the runtime emits status events
// directly (out-of-band relative to the action→event cycle) because
// command results originate from host-native commands rather than
// shell-emitted actions.
//
// Commands the production runtime routes:
//
//	/model, /models      — model.list / model.switch
//	/session, /sessions  — session.list / session.resume / session.clear / session.fork
//	/context             — context.list
//	/compact             — session.compact / compact.apply
//	/history             — history.list
//	/mcp                 — MCP server status (lazy-start)
//	/plan, /build, /auto — runtime state mode setter (no RPC)
//
// Unknown commands return nil and emit StatusError.
func (r *ProductionRuntime) DispatchCommand(ctx context.Context, cmd HostCommand) error {
	switch cmd.Name {
	case "plan":
		return r.handleModeCommand(protocol.ModePlan)
	case "build":
		return r.handleModeCommand(protocol.ModeBuild)
	case "auto":
		return r.handleModeCommand(protocol.ModeAuto)
	case "model":
		return r.handleModelCommand(ctx, cmd.Args)
	case "models":
		return r.handleModelsCommand(ctx)
	case "session", "sessions":
		return r.handleSessionCommand(ctx, cmd.Name, cmd.Args)
	case "context":
		return r.handleContextCommand(ctx)
	case "compact":
		return r.handleCompactCommand(ctx)
	case "history":
		return r.handleHistoryCommand(ctx, cmd.Args)
	case "mcp":
		return r.handleMCPCommand(ctx, cmd.Args)
	case "run":
		return r.handleRunCommand(ctx, cmd.Args)
	case "runs", "jobs":
		return r.handleRunsCommand(ctx)
	case "attach":
		return r.handleRunAttachCommand(ctx, cmd.Args)
	case "detach":
		return r.handleRunDetachCommand(ctx, cmd.Args)
	case "cancel":
		return r.handleRunControlCommand(ctx, protocol.MethodRunCancel, cmd.Args)
	case "retry":
		return r.handleRunControlCommand(ctx, protocol.MethodRunRetry, cmd.Args)
	case "continue":
		return r.handleRunControlCommand(ctx, protocol.MethodRunContinue, cmd.Args)
	case "fork":
		return r.handleRunControlCommand(ctx, protocol.MethodRunFork, cmd.Args)
	case "help":
		return r.handleHelpCommand()
	case "clear":
		return r.handleClearCommand(ctx)
	default:
		r.sender.Send(harnessshell.HostStatusEvent{
			Status: "Unknown command: /" + cmd.Name,
			Kind:   harnessshell.StatusError,
		})
		return nil
	}
}

// handleModeCommand updates the runtime's execution mode and emits a
// confirmation status event. Mode changes are runtime-local; no RPC.
func (r *ProductionRuntime) handleModeCommand(mode protocol.Mode) error {
	r.mode.SetMode(mode)
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "Mode: " + string(mode),
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) handleModelCommand(ctx context.Context, args string) error {
	args = strings.TrimSpace(args)
	if args == "" {
		// Show current model.
		r.sender.Send(harnessshell.HostStatusEvent{
			Status: "Current model: " + nonEmpty(r.mode.Label(), "(unset)"),
			Kind:   harnessshell.StatusReady,
		})
		return nil
	}
	client := r.cm.Client()
	if client == nil {
		return r.statusError("model.switch", errNoLiveClient)
	}
	var resp protocol.ModelSwitchResponse
	if err := client.CallInto(ctx, protocol.MethodModelSwitch, &protocol.ModelSwitch{
		SessionID: r.mode.SessionID(),
		Model:     args,
	}, &resp); err != nil {
		return r.statusError("model.switch", err)
	}
	if resp.Model != "" {
		r.mode.SetLabel(resp.Model)
	}
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "Model switch: " + nonEmpty(resp.Model, args) + " — " + resp.Reason,
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) handleModelsCommand(ctx context.Context) error {
	client := r.cm.Client()
	if client == nil {
		return r.statusError("model.list", errNoLiveClient)
	}
	var resp protocol.ModelListResponse
	if err := client.CallInto(ctx, protocol.MethodModelList, &protocol.ModelList{}, &resp); err != nil {
		return r.statusError("model.list", err)
	}
	if len(resp.Models) == 0 {
		r.sender.Send(harnessshell.HostInfoEvent{Text: "No models available"})
		return nil
	}
	var b strings.Builder
	b.WriteString("Available models:")
	for _, m := range resp.Models {
		b.WriteString("\n  ")
		b.WriteString(m.Name)
		if m.Provider != "" {
			b.WriteString(" (")
			b.WriteString(m.Provider)
			b.WriteString(")")
		}
		if m.Status != "" {
			b.WriteString(" [")
			b.WriteString(m.Status)
			b.WriteString("]")
		}
	}
	r.sender.Send(harnessshell.HostInfoEvent{Text: b.String()})
	return nil
}

func (r *ProductionRuntime) handleSessionCommand(ctx context.Context, name, args string) error {
	args = strings.TrimSpace(args)
	// `/sessions` (alias-form, no args) or `/sessions list` or
	// `/session list` route to list. Sub-commands like `current` and
	// `resume <id>` reach the switch below. PATCH-0038 added
	// `current`; PATCH-0039 will add `delete <id>` / `prune`.
	if args == "" || args == "list" {
		return r.handleSessionList(ctx)
	}
	parts := strings.Fields(args)
	switch parts[0] {
	case "resume":
		if len(parts) < 2 {
			return r.statusError("session.resume", errors.New("session resume requires <id>"))
		}
		return r.handleSessionResume(ctx, parts[1])
	case "clear":
		return r.handleSessionMutation(ctx, protocol.MethodSessionClear)
	case "fork":
		return r.handleSessionMutation(ctx, protocol.MethodSessionFork)
	case "current":
		return r.handleSessionCurrent()
	}
	return r.statusError("session", errors.New("unknown session subcommand: "+parts[0]))
}

// handleSessionCurrent prints the active session id. PATCH-0038.
// Helpful for cross-referencing with /run / /runs output when
// multiple sessions exist for the project.
func (r *ProductionRuntime) handleSessionCurrent() error {
	id := r.mode.SessionID()
	if id == "" {
		r.sender.Send(harnessshell.HostStatusEvent{
			Status: "No active session",
			Kind:   harnessshell.StatusReady,
		})
		return nil
	}
	r.sender.Send(harnessshell.HostInfoEvent{
		Text: "Current session: " + id,
	})
	return nil
}

// handleClearCommand implements /clear's PATCH-0038 redefinition:
// /clear starts a fresh conversation. Calls session.create to mint a
// new session id, switches the harness's active session to it, and
// emits TranscriptClearEvent so the shell wipes its visible
// transcript only on success.
//
// Refuses while a run is streaming — mid-stream clear would require
// cancelling the active run first; surface a clear error instead.
func (r *ProductionRuntime) handleClearCommand(ctx context.Context) error {
	if r.mode.ActiveRunID() != "" {
		return r.statusError("clear", errors.New("cannot start new conversation while a run is in flight; press Esc twice to cancel first"))
	}
	client := r.cm.Client()
	if client == nil {
		return r.statusError("clear", errNoLiveClient)
	}
	var resp protocol.SessionCreateResponse
	if err := client.CallInto(ctx, protocol.MethodSessionCreate, &protocol.SessionCreate{
		Project: protocol.ProjectContext{Root: r.cfg.ProjectRoot},
	}, &resp); err != nil {
		return r.statusError("clear", err)
	}
	r.mode.SetSessionID(resp.SessionID)
	r.mode.SetActiveRunID("")
	r.sender.Send(harnessshell.TranscriptClearEvent{})
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "Started new conversation: " + resp.SessionID,
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) handleSessionList(ctx context.Context) error {
	client := r.cm.Client()
	if client == nil {
		return r.statusError("session.list", errNoLiveClient)
	}
	var resp protocol.SessionListResponse
	if err := client.CallInto(ctx, protocol.MethodSessionList, &protocol.SessionList{}, &resp); err != nil {
		return r.statusError("session.list", err)
	}
	if len(resp.Sessions) == 0 {
		r.sender.Send(harnessshell.HostInfoEvent{Text: "No sessions"})
		return nil
	}
	var b strings.Builder
	b.WriteString("Sessions:")
	for _, s := range resp.Sessions {
		b.WriteString("\n  ")
		b.WriteString(s.ID)
		if s.Summary != "" {
			b.WriteString(" — ")
			b.WriteString(s.Summary)
		}
	}
	r.sender.Send(harnessshell.HostInfoEvent{Text: b.String()})
	return nil
}

func (r *ProductionRuntime) handleSessionResume(ctx context.Context, id string) error {
	client := r.cm.Client()
	if client == nil {
		return r.statusError("session.resume", errNoLiveClient)
	}
	var resp protocol.SessionResumeResponse
	if err := client.CallInto(ctx, protocol.MethodSessionResume, &protocol.SessionResume{
		SessionID: id,
		Project:   protocol.ProjectContext{Root: r.cfg.ProjectRoot},
	}, &resp); err != nil {
		return r.statusError("session.resume", err)
	}
	r.mode.SetSessionID(resp.SessionID)
	r.resumeKnownRuns(ctx)
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "Resumed session: " + resp.SessionID,
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) observeRuntimeMessage(msg tea.Msg) {
	m, ok := msg.(harness.ConnStateMsg)
	if !ok || m.Info.State != harness.ConnStateReady {
		return
	}
	go r.bootstrapSession(context.Background())
	go r.resumeKnownRuns(context.Background())
}

// bootstrapSession ensures the harness has an active session id on
// ConnStateReady so session-scoped RPCs (model.switch, context.list,
// session.clear, etc.) work before the user has submitted any turn.
// PATCH-0028 added the bootstrap; PATCH-0029 fixed a race against a
// fast turn.submit; PATCH-0038 changes the policy from "always create
// new" to "auto-resume the most-recent session, or create one only
// when none exist for this user/project."
//
// Net effect for the user: shell launches pick up where the previous
// launch left off (Claude Code-style); `/clear` is the way to start a
// fresh conversation, not relaunching the shell. Cuts down on
// orphan-session accumulation (Finding F23).
//
// Failures of session.list fall back to the original create path so
// users on a broken list endpoint still get a usable shell.
//
// Emits a HostInfoEvent welcome message naming the active session id
// and how to start fresh.
func (r *ProductionRuntime) bootstrapSession(ctx context.Context) {
	if r.mode.SessionID() != "" {
		return
	}
	client := r.cm.Client()
	if client == nil {
		return
	}

	// Try resume-most-recent first.
	var list protocol.SessionListResponse
	listErr := client.CallInto(ctx, protocol.MethodSessionList, &protocol.SessionList{}, &list)
	if listErr == nil && len(list.Sessions) > 0 {
		// Sessions are returned newest-first per storage's
		// updated_at DESC ordering.
		mostRecent := list.Sessions[0]
		var resumeResp protocol.SessionResumeResponse
		if err := client.CallInto(ctx, protocol.MethodSessionResume, &protocol.SessionResume{
			SessionID: mostRecent.ID,
			Project:   protocol.ProjectContext{Root: r.cfg.ProjectRoot},
		}, &resumeResp); err == nil {
			// PATCH-0029 race re-check.
			if r.mode.SessionID() != "" {
				return
			}
			r.mode.SetSessionID(resumeResp.SessionID)
			r.sender.Send(harnessshell.HostInfoEvent{
				Text: "Resumed session " + resumeResp.SessionID + ". Type /clear to start a new conversation, /sessions list to see all sessions.",
			})
			return
		}
		// Resume failed — fall through to create.
	}

	// No sessions to resume, or list/resume failed: create fresh.
	var createResp protocol.SessionCreateResponse
	if err := client.CallInto(ctx, protocol.MethodSessionCreate, &protocol.SessionCreate{
		Project: protocol.ProjectContext{Root: r.cfg.ProjectRoot},
	}, &createResp); err != nil {
		return
	}
	// PATCH-0029 race re-check.
	if r.mode.SessionID() != "" {
		return
	}
	r.mode.SetSessionID(createResp.SessionID)
	r.sender.Send(harnessshell.HostInfoEvent{
		Text: "New session " + createResp.SessionID + ". Type /help for commands.",
	})
}

func (r *ProductionRuntime) resumeKnownRuns(ctx context.Context) {
	client := r.cm.Client()
	if client == nil {
		return
	}
	sessionID := r.mode.SessionID()
	if sessionID == "" {
		return
	}
	var list protocol.RunListResponse
	if err := client.CallInto(ctx, protocol.MethodRunList, &protocol.RunList{SessionID: sessionID, Limit: 20}, &list); err != nil {
		return
	}
	activeRunID := r.mode.ActiveRunID()
	if activeRunID != "" {
		var events protocol.RunEventsResponse
		if err := client.CallInto(ctx, protocol.MethodRunEvents, &protocol.RunEvents{RunID: activeRunID, AfterSeq: 0, Limit: 100}, &events); err == nil {
			r.projectRunReplay(events.Events)
			r.sender.Send(harnessshell.HostStatusEvent{
				Status: "Run replay: " + activeRunID + " (" + events.Fidelity + ")",
				Kind:   harnessshell.StatusReady,
			})
			return
		}
	}
	if len(list.Runs) > 0 {
		r.sender.Send(harnessshell.HostStatusEvent{
			Status: fmt.Sprintf("Recovered %d recent run(s)", len(list.Runs)),
			Kind:   harnessshell.StatusReady,
		})
	}
}

func (r *ProductionRuntime) projectRunReplay(events []protocol.RunEventPayload) {
	for _, ev := range events {
		switch ev.Type {
		case protocol.EventRunProgress:
			var payload struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(ev.Payload, &payload); err == nil && payload.Type == "token_delta" && payload.Text != "" {
				r.sender.Send(harnessshell.RunDeltaEvent{RunID: ev.RunID, Delta: payload.Text})
			}
		case protocol.EventRunCompleted:
			r.sender.Send(harnessshell.RunCompletedEvent{RunID: ev.RunID})
		case protocol.EventRunFailed:
			r.sender.Send(harnessshell.RunFailedEvent{RunID: ev.RunID, Message: ev.Reason})
		case protocol.EventRunCancelled:
			r.sender.Send(harnessshell.RunStoppedEvent{RunID: ev.RunID, Reason: harnessshell.StopReasonHost, Message: ev.Reason})
		case protocol.EventRunBlocked:
			status := "Run blocked"
			if ev.Reason != "" {
				status += ": " + ev.Reason
			}
			r.sender.Send(harnessshell.HostStatusEvent{Status: status, Kind: harnessshell.StatusPermissionPending})
		}
	}
}

func (r *ProductionRuntime) handleSessionMutation(ctx context.Context, method string) error {
	sessionID := r.mode.SessionID()
	if sessionID == "" {
		return r.statusError(method, errors.New("no active session"))
	}
	client := r.cm.Client()
	if client == nil {
		return r.statusError(method, errNoLiveClient)
	}
	if _, err := client.Call(ctx, method, struct {
		SessionID string `json:"session_id"`
	}{SessionID: sessionID}); err != nil {
		return r.statusError(method, err)
	}
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: method + " ok: " + sessionID,
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) handleContextCommand(ctx context.Context) error {
	client := r.cm.Client()
	if client == nil {
		return r.statusError("context.list", errNoLiveClient)
	}
	resp, err := client.Call(ctx, protocol.MethodContextList, struct {
		SessionID string `json:"session_id"`
	}{SessionID: r.mode.SessionID()})
	if err != nil {
		return r.statusError("context.list", err)
	}
	r.sender.Send(harnessshell.HostInfoEvent{
		Text: "Context:\n" + string(resp),
	})
	return nil
}

func (r *ProductionRuntime) handleCompactCommand(ctx context.Context) error {
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "/compact: not yet wired in v0.2.2 (planned for follow-up)",
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) handleHistoryCommand(ctx context.Context, args string) error {
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "/history: not yet wired in v0.2.2 (planned for follow-up)",
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) handleRunCommand(ctx context.Context, args string) error {
	args = strings.TrimSpace(args)
	switch args {
	case "context", "prompt":
		r.sender.Send(harnessshell.HostStatusEvent{
			Status: "/run " + args + ": not enabled in v0.3.0 (planned for v0.3.1)",
			Kind:   harnessshell.StatusReady,
		})
		return nil
	case "policy":
		r.sender.Send(harnessshell.HostStatusEvent{
			Status: "/run policy: not enabled in v0.3.0 (planned for v0.3.3)",
			Kind:   harnessshell.StatusReady,
		})
		return nil
	}
	runID := args
	if runID == "" {
		runID = r.mode.ActiveRunID()
	}
	if runID == "" {
		return r.statusError("run.details", errors.New("no active run"))
	}
	client := r.cm.Client()
	if client == nil {
		return r.statusError("run.details", errNoLiveClient)
	}
	var resp protocol.RunDetailsResponse
	if err := client.CallInto(ctx, protocol.MethodRunDetails, &protocol.RunDetails{RunID: runID}, &resp); err != nil {
		return r.statusError("run.details", err)
	}
	r.sender.Send(harnessshell.HostInfoEvent{
		Text: formatRunDetails(resp.Run),
	})
	return nil
}

func (r *ProductionRuntime) handleRunsCommand(ctx context.Context) error {
	client := r.cm.Client()
	if client == nil {
		return r.statusError("run.list", errNoLiveClient)
	}
	var resp protocol.RunListResponse
	if err := client.CallInto(ctx, protocol.MethodRunList, &protocol.RunList{SessionID: r.mode.SessionID(), Limit: 20}, &resp); err != nil {
		return r.statusError("run.list", err)
	}
	if len(resp.Runs) == 0 {
		r.sender.Send(harnessshell.HostInfoEvent{Text: "No runs"})
		return nil
	}
	var b strings.Builder
	b.WriteString("Runs:")
	for _, run := range resp.Runs {
		b.WriteString("\n  ")
		b.WriteString(run.RunID)
		b.WriteString(" ")
		b.WriteString(run.Status)
		b.WriteString("/")
		b.WriteString(run.Stage)
		if run.InputRequired {
			b.WriteString(" input-required")
		}
		if run.Stuck {
			b.WriteString(" stuck")
		}
		if run.Title != "" {
			b.WriteString(" — ")
			b.WriteString(truncate(run.Title, 60))
		}
	}
	r.sender.Send(harnessshell.HostInfoEvent{Text: b.String()})
	return nil
}

func (r *ProductionRuntime) handleRunAttachCommand(ctx context.Context, args string) error {
	runID := strings.TrimSpace(args)
	if runID == "" {
		return r.statusError("run.attach", errors.New("attach requires <run-id>"))
	}
	client := r.cm.Client()
	if client == nil {
		return r.statusError("run.attach", errNoLiveClient)
	}
	var resp protocol.RunAttachResponse
	if err := client.CallInto(ctx, protocol.MethodRunAttach, &protocol.RunAttach{RunID: runID}, &resp); err != nil {
		// PATCH-0033: the Runtime rejects attaching to terminal runs by
		// design. Surface a friendlier hint pointing at /run for
		// read-only inspection instead of leaking the JSON-RPC error.
		var rpcErr *harness.RPCError
		if errors.As(err, &rpcErr) && strings.Contains(rpcErr.Message, "cannot attach terminal run") {
			r.sender.Send(harnessshell.HostStatusEvent{
				Status: "run.attach failed: run " + runID +
					" is already complete — use /run " + runID + " to inspect it",
				Kind: harnessshell.StatusError,
			})
			return nil
		}
		return r.statusError("run.attach", err)
	}
	r.mode.SetActiveRunID(resp.Run.RunID)
	r.sender.Send(harnessshell.RunStartedEvent{RunID: resp.Run.RunID})
	r.projectRunReplay(resp.Events)
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "Attached run: " + resp.Run.RunID + " (" + resp.Fidelity + " replay)",
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) handleRunDetachCommand(ctx context.Context, args string) error {
	runID := strings.TrimSpace(args)
	if runID == "" {
		runID = r.mode.ActiveRunID()
	}
	if runID == "" {
		return r.statusError("run.detach", errors.New("no active run"))
	}
	client := r.cm.Client()
	if client == nil {
		return r.statusError("run.detach", errNoLiveClient)
	}
	var resp protocol.RunDetachResponse
	if err := client.CallInto(ctx, protocol.MethodRunDetach, &protocol.RunDetach{RunID: runID}, &resp); err != nil {
		return r.statusError("run.detach", err)
	}
	if r.mode.ActiveRunID() == runID {
		r.mode.SetActiveRunID("")
		r.sender.Send(harnessshell.RunStoppedEvent{
			RunID:   runID,
			Reason:  harnessshell.StopReasonHost,
			Message: "Detached",
		})
	}
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "Detached run: " + resp.RunID,
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func (r *ProductionRuntime) handleRunControlCommand(ctx context.Context, method, args string) error {
	runID := strings.TrimSpace(args)
	if runID == "" && method == protocol.MethodRunCancel {
		runID = r.mode.ActiveRunID()
	}
	if runID == "" {
		return r.statusError(method, errors.New("requires <run-id>"))
	}
	client := r.cm.Client()
	if client == nil {
		return r.statusError(method, errNoLiveClient)
	}
	var resp protocol.RunControlResponse
	if err := client.CallInto(ctx, method, &protocol.RunControl{RunID: runID}, &resp); err != nil {
		return r.statusError(method, err)
	}
	status := method + " "
	if resp.Accepted {
		status += "accepted"
	} else {
		status += "not accepted"
	}
	if resp.Message != "" {
		status += ": " + resp.Message
	}
	r.sender.Send(harnessshell.HostStatusEvent{Status: status, Kind: harnessshell.StatusReady})
	return nil
}

// handleHelpCommand prints the host slash-command surface to the
// transcript. Per-command argument detail (`/help <name>`) is out of
// scope for v0.3.0 and tracked under FEAT-0024. PATCH-0037.
func (r *ProductionRuntime) handleHelpCommand() error {
	r.sender.Send(harnessshell.HostInfoEvent{Text: helpText})
	return nil
}

const helpText = `Available commands:

  modes:     /plan  /build  /auto
  model:     /model <name>  /models
  session:   /sessions [list|resume <id>|clear|fork|current]
  context:   /context  /compact
  history:   /history
  mcp:       /mcp
  runs:      /run [<id>]  /runs  /jobs
  lifecycle: /attach <id>  /detach  /cancel  /retry  /continue  /fork
  shell:     /clear (new conversation)  /select  /help  /quit  /exit`

// handleMCPCommand is the MCP lazy-start integration point. v0.2.2
// ships with MCP wiring deferred to a follow-up release; the command
// returns a clear "not yet configured" status rather than crashing.
func (r *ProductionRuntime) handleMCPCommand(ctx context.Context, args string) error {
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: "/mcp: MCP not configured in this v0.2.2 build (planned for follow-up)",
		Kind:   harnessshell.StatusReady,
	})
	return nil
}

func formatRunDetails(run protocol.RunSummary) string {
	var b strings.Builder
	b.WriteString("Run ")
	b.WriteString(run.RunID)
	b.WriteString(": ")
	b.WriteString(run.Status)
	b.WriteString("/")
	b.WriteString(run.Stage)
	if run.AttachmentState != "" {
		b.WriteString(" ")
		b.WriteString(run.AttachmentState)
	}
	if run.Model != "" {
		b.WriteString(" ")
		b.WriteString(run.Model)
	}
	if run.TotalCost > 0 {
		b.WriteString(fmt.Sprintf(" $%.4f", run.TotalCost))
	}
	if run.Summary != "" {
		b.WriteString("\n")
		b.WriteString(run.Summary)
	}
	return b.String()
}

// statusError emits a HostStatusEvent{Kind: StatusError} with the
// command + error message and returns nil so the adapter doesn't
// double-emit a RunFailedEvent.
//
// PATCH-0033: unwraps *harness.RPCError so the JSON-RPC wire framing
// (`rpc error -%d: %s`) doesn't leak into user-facing status text.
// Surfaces just the inner message for transport errors.
func (r *ProductionRuntime) statusError(op string, err error) error {
	msg := err.Error()
	var rpcErr *harness.RPCError
	if errors.As(err, &rpcErr) {
		msg = rpcErr.Message
	}
	r.sender.Send(harnessshell.HostStatusEvent{
		Status: op + " failed: " + msg,
		Kind:   harnessshell.StatusError,
	})
	return nil
}

// errNoLiveClient is the canonical "no live runtime client" error; helper
// constants reuse it to keep the user-visible message stable.
var errNoLiveClient = errors.New("no live runtime client")

// nonEmpty returns s if non-empty, else fallback.
func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// ResolvePermission implements [harnesshost.Runtime]. Writes the
// user's decision into the per-ToolCallID channel that the
// permissionPromptCallback is blocking on. Idempotent: unknown
// requestIDs are no-ops (the gate may have already resolved or
// timed out).
func (r *ProductionRuntime) ResolvePermission(ctx context.Context, requestID string, decision harnessshell.PermissionDecision) error {
	raw, ok := r.permPromises.Load(requestID)
	if !ok {
		return nil
	}
	promise := raw.(chan harnessshell.PermissionDecision)
	select {
	case promise <- decision:
		// Synthesize PermissionResolvedEvent into the projection
		// stream so the adapter's pause buffer drains and the
		// transcript event row flips to granted/denied.
		r.sender.Send(harnessshell.PermissionResolvedEvent{
			RequestID: requestID,
			Outcome:   outcomeFromDecision(decision),
		})
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Channel full: the gate already received a decision.
		return nil
	}
}

// LoadPreview implements [harnesshost.Runtime]. Path comes from the
// adapter's tokenAttachments map (populated on submit); the runtime
// validates via ContextManager.Resolve and reads via the Read tool.
func (r *ProductionRuntime) LoadPreview(ctx context.Context, req PreviewRequest) (harnessshell.PreviewPayload, error) {
	if req.Path == "" {
		return harnessshell.PreviewPayload{}, errors.New("preview: path unresolved for token " + req.TokenID)
	}

	// Validate the path against the project root via Resolve. We
	// expect a single non-glob path; reject globs to keep preview
	// scoped to one file.
	resolved, err := r.ctxMgr.Resolve(ctx, []string{req.Path})
	if err != nil {
		return harnessshell.PreviewPayload{}, err
	}
	if len(resolved) == 0 {
		return harnessshell.PreviewPayload{}, errors.New("preview: path resolved to no files")
	}
	att := resolved[0]

	// Run the Read tool to produce the rendered text. The tool
	// applies its own size cap and format detection.
	read := r.registry.Get(tools.ToolNameRead)
	if read == nil {
		return harnessshell.PreviewPayload{}, errors.New("preview: Read tool not registered")
	}
	input, _ := json.Marshal(map[string]any{"file_path": att.Path})
	result, err := read.Execute(ctx, input)
	if err != nil {
		return harnessshell.PreviewPayload{}, err
	}
	if result.Status != tools.StatusSuccess {
		msg := result.Error
		if msg == "" {
			msg = result.Reason
		}
		return harnessshell.PreviewPayload{}, errors.New("preview: " + msg)
	}

	title := att.Path
	if i := lastSlash(title); i >= 0 && i+1 < len(title) {
		title = title[i+1:]
	}
	return harnessshell.PreviewPayload{
		Title:   title,
		Content: result.Output,
		Metadata: map[string]string{
			"path": att.Path,
			"type": result.OutputType,
		},
	}, nil
}

// outcomeFromDecision translates a shell-side PermissionDecision
// into the adapter-side PermissionOutcome the shell expects on
// PermissionResolvedEvent.
func outcomeFromDecision(d harnessshell.PermissionDecision) harnessshell.PermissionOutcome {
	switch d {
	case harnessshell.DecisionApproveOnce:
		return harnessshell.OutcomeApprovedOnce
	case harnessshell.DecisionApproveSession:
		return harnessshell.OutcomeApprovedSession
	default:
		return harnessshell.OutcomeDenied
	}
}

// lastSlash returns the index of the last '/' or '\\' in s, or -1.
func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' || s[i] == '\\' {
			return i
		}
	}
	return -1
}

// SummarizePaste implements [harnesshost.Runtime] via the existing
// content.transform RPC. Per Kimi #17 the fallback on RPC error is
// a passthrough — the shell already has its own built-in paste
// summarizer, so a missing/failing Runtime transform doesn't break
// paste capture.
func (r *ProductionRuntime) SummarizePaste(ctx context.Context, raw string) (string, error) {
	client := r.cm.Client()
	if client == nil {
		return raw, nil
	}
	resp, err := client.Call(ctx, "content.transform", &protocol.ContentTransform{
		Transform:   "summarize",
		RawContent:  raw,
		ContentType: "text/plain",
	})
	if err != nil {
		return raw, nil
	}
	var result struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(resp, &result); err != nil || result.Content == "" {
		return raw, nil
	}
	return result.Content, nil
}

// permissionPromptCallback runs on the tool-dispatch goroutine. It
// generates a runtime-internal request ID, emits a
// harness.PermissionPromptMsg through the deferred sender (which the
// adapter's projection layer translates into
// harnessshell.PermissionRequestedEvent), and blocks on a channel
// keyed by the request ID. ResolvePermission writes the decision
// onto the channel.
//
// WU-104a stub: the channel is registered but ResolvePermission can't
// fulfill it yet (returns not-implemented). WU-104b completes the
// loop. For WU-104a's Runtime stub Layer 3 tests, no permission prompts
// fire so this stub is exercised only by WU-104b's tests.
func (r *ProductionRuntime) permissionPromptCallback(ctx context.Context, tool tools.Tool, input json.RawMessage) bool {
	requestID := fmt.Sprintf("perm-%d", r.permCounter.Add(1))
	promise := make(chan harnessshell.PermissionDecision, 1)
	r.permPromises.Store(requestID, promise)
	defer r.permPromises.Delete(requestID)

	r.sender.Send(harness.PermissionPromptMsg{
		ToolCallID:  requestID,
		ToolName:    tool.Name(),
		RiskLevel:   string(tool.RiskLevel()),
		Description: summarizeToolCall(tool.Name(), input),
		Input:       input,
	})

	select {
	case decision := <-promise:
		switch decision {
		case harnessshell.DecisionApproveOnce, harnessshell.DecisionApproveSession:
			return true
		default:
			return false
		}
	case <-ctx.Done():
		return false
	case <-time.After(r.cfg.PermissionTimeout):
		return false
	}
}

// buildSubmitContent splits shell-emitted [Attachment] values into
// (final content text, protocol-shaped attachments). File attachments
// resolve through the ContextManager (which honors the tools/Read
// project-root scope and converts paths into protocol.Attachment).
// Paste attachments are concatenated inline into the content text;
// the shell already keeps paste tokens visually expanded in the
// transcript, so inlining the payload here matches what the user
// expected to send.
func (r *ProductionRuntime) buildSubmitContent(ctx context.Context, req SubmitRequest) (string, []protocol.Attachment, error) {
	if len(req.Attachments) == 0 {
		return req.Text, nil, nil
	}

	var refs []string
	var pasteParts []string
	for _, a := range req.Attachments {
		switch a.Kind {
		case harnessshell.TokenKindFile:
			if a.Path != "" {
				refs = append(refs, a.Path)
			}
		case harnessshell.TokenKindPaste:
			if a.Payload != "" {
				pasteParts = append(pasteParts, a.Payload)
			}
		}
	}

	content := req.Text
	if len(pasteParts) > 0 {
		var b strings.Builder
		b.WriteString(strings.TrimSpace(content))
		for _, p := range pasteParts {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(p)
		}
		content = b.String()
	}

	var attachments []protocol.Attachment
	if len(refs) > 0 {
		resolved, err := r.ctxMgr.Resolve(ctx, refs)
		if err != nil {
			return "", nil, err
		}
		attachments = resolved
	}

	return content, attachments, nil
}

// toolResultSender adapts *ConnectionManager into the
// harness.ToolResultSender interface the dispatcher expects. Reads the
// live ProtocolClient on every call so the dispatcher survives
// reconnects.
type toolResultSender struct {
	cm *harness.ConnectionManager
}

func newToolResultSender(cm *harness.ConnectionManager) *toolResultSender {
	return &toolResultSender{cm: cm}
}

// SendToolResult satisfies harness.ToolResultSender.
func (t *toolResultSender) SendToolResult(ctx context.Context, result *protocol.ToolResult) error {
	client := t.cm.Client()
	if client == nil {
		return errors.New("tool result dropped: no live runtime client")
	}
	return client.SendToolResult(ctx, result)
}

// registerBuiltinTools wires the in-tree tool implementations into
// the registry. Mirrors the legacy CLI's registration list. WebSearch
// is intentionally omitted because it requires an API key the runtime
// can't read at construction time; users wanting WebSearch wire it
// explicitly through MCP or a future config surface.
func registerBuiltinTools(registry *tools.Registry, projectRoot string, tracker *tools.FileTracker) {
	registry.Register(tools.NewReadTool(projectRoot, tracker))
	registry.Register(tools.NewWriteTool(projectRoot, tracker))
	registry.Register(tools.NewEditTool(projectRoot, tracker))
	registry.Register(tools.NewBashTool(projectRoot))
	registry.Register(tools.NewGitTool(projectRoot))
	registry.Register(tools.NewGlobTool(projectRoot))
	registry.Register(tools.NewGrepTool(projectRoot))
	registry.Register(tools.NewWebFetchTool())
}

// summarizeToolCall produces a one-liner for the
// harness.PermissionPromptMsg.Description field. Mirrors
// harness.defaultPlanSummary but lives in this package because the
// helper there is unexported.
func summarizeToolCall(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return toolName
	}
	// Best-effort: try to decode common single-key shapes (path, url,
	// command); fall back to the raw JSON.
	var generic map[string]any
	if err := json.Unmarshal(input, &generic); err != nil {
		return toolName
	}
	for _, key := range []string{"path", "url", "command", "pattern", "file"} {
		if v, ok := generic[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return toolName + ": " + truncate(s, 80)
			}
		}
	}
	return toolName
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "…"
}

// Compile-time check that ProductionRuntime satisfies Runtime.
var _ Runtime = (*ProductionRuntime)(nil)
