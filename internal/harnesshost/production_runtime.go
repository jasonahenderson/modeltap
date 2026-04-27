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
}

// Send satisfies harness.ProgramSender. Drops messages silently when
// no program is attached.
func (d *deferredSender) Send(msg tea.Msg) {
	if p := d.program.Load(); p != nil {
		p.Send(msg)
	}
}

// runtimeState tracks the modeltap-side session state that the
// harness's existing AppState used to hold. It implements
// harness.ModeReader so harness.ToolDispatcher can read the current
// execution mode.
type runtimeState struct {
	mu        sync.Mutex
	mode      protocol.Mode
	sessionID string
	sequence  int
	label     string // current model label
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
	return r.cm.ConnectSync(ctx)
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
		return SubmitAccepted{}, errors.New("no live BFF client")
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
	if finalID == "" {
		finalID = turnID
	}
	if ack.SessionID != "" {
		r.mode.SetSessionID(ack.SessionID)
	}

	return SubmitAccepted{
		RunID: finalID,
		Label: r.mode.Label(),
	}, nil
}

// InterruptRun implements [harnesshost.Runtime]. WU-104a returns
// not-implemented; WU-104b lands the CancelTurn integration.
func (r *ProductionRuntime) InterruptRun(ctx context.Context, runID string) error {
	return errors.New("InterruptRun: not yet implemented (WU-104b)")
}

// DispatchCommand implements [harnesshost.Runtime]. WU-104a returns
// not-implemented; WU-104c lands the per-command routing.
func (r *ProductionRuntime) DispatchCommand(ctx context.Context, cmd HostCommand) error {
	return errors.New("DispatchCommand: not yet implemented (WU-104c)")
}

// ResolvePermission implements [harnesshost.Runtime]. WU-104a returns
// not-implemented; WU-104b lands the channel-based bridge.
func (r *ProductionRuntime) ResolvePermission(ctx context.Context, requestID string, decision harnessshell.PermissionDecision) error {
	return errors.New("ResolvePermission: not yet implemented (WU-104b)")
}

// LoadPreview implements [harnesshost.Runtime]. WU-104a returns
// not-implemented; WU-104b lands the path-resolution + Read tool
// integration.
func (r *ProductionRuntime) LoadPreview(ctx context.Context, req PreviewRequest) (harnessshell.PreviewPayload, error) {
	return harnessshell.PreviewPayload{}, errors.New("LoadPreview: not yet implemented (WU-104b)")
}

// SummarizePaste implements [harnesshost.Runtime]. WU-104a returns
// the raw text unchanged; WU-104c lands the ContentTransform
// integration with passthrough fallback.
func (r *ProductionRuntime) SummarizePaste(ctx context.Context, raw string) (string, error) {
	return raw, nil
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
// loop. For WU-104a's BFF stub Layer 3 tests, no permission prompts
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
		return errors.New("tool result dropped: no live BFF client")
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
