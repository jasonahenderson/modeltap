package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/jasonahenderson/modeltap/internal/harness/theme"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

var (
	errNoConnection = errors.New("no connection wired")
	errNotConnected = errors.New("not connected to BFF")
)

// AppOptions configures App construction.
type AppOptions struct {
	// SubmitKey is one of the SubmitKey* constants; empty defaults to
	// SubmitKeyCtrlEnter.
	SubmitKey string
	// InitialMode sets the starting execution mode. Empty defaults to
	// protocol.ModeBuild.
	InitialMode protocol.Mode
	// Conn, when non-nil, wires the App's slash commands and
	// PasteSummarizeRequestMsg path through to a ConnectionManager.
	// Left nil (e.g. in tests) the App degrades gracefully: /status
	// reports "no connection", /reconnect is a no-op banner, and
	// paste-summarize resolves as a cancel with an error banner.
	Conn ConnSurface

	// Attacher resolves SubmitMsg.Attachments (raw @file refs) into
	// typed protocol.Attachment values before turn.submit runs. When
	// nil the App passes no attachments along — raw refs are
	// effectively dropped, which is safe for a server that ignores
	// them.
	Attacher FileAttacher

	// MCP, when non-nil, powers /mcp status and /mcp reconnect. Tool
	// discovery / registration happens through MCPManager.Launch at
	// construction or post-construct.
	MCP *MCPManager
}

// App is the top-level Bubbletea model for the modeltap harness. It
// owns the AppState and the three child components (status bar, input
// area, conversation viewport) and routes Bubbletea messages between
// them. The protocol client (WU-073) and connection manager (WU-074)
// produce the streaming / connection / model messages this Update
// handler reacts to.
type App struct {
	state     *AppState
	statusBar StatusBar
	input     InputArea
	viewport  ConversationViewport
	keys      KeyMap
	connUX    *ConnectionUX
	paste     *PasteHandler
	conn      ConnSurface
	history   *HistoryController
	attacher  FileAttacher
	mcp       *MCPManager
	plan      *PlanAccumulator
	compact   *CompactHandler

	width  int
	height int

	// bannerExpiresAt marks when the current Banner should be cleared
	// by the runtime. Zero when no auto-clear is scheduled.
	bannerExpiresAt time.Time
}

// NewApp constructs a fresh App ready for tea.NewProgram.
func NewApp(opts AppOptions) App {
	state := NewAppState()
	if opts.InitialMode != "" {
		state.Mode = opts.InitialMode
	}
	app := App{
		state:     state,
		statusBar: NewStatusBar(state),
		input:     NewInputArea(state),
		viewport:  NewConversationViewport(state),
		keys:      DefaultKeyMap(opts.SubmitKey),
		connUX:    NewConnectionUX(state),
		paste:     NewPasteHandler(),
		conn:      opts.Conn,
		attacher:  opts.Attacher,
		mcp:       opts.MCP,
		plan:      NewPlanAccumulator(),
		compact:   NewCompactHandler(),
	}
	app.history = NewHistoryController(opts.Conn)
	app.input.SetHistorySource(app.history)
	return app
}

// SetConn wires an App to a ConnSurface after construction. Primary
// callers: CLI launch paths that build the App first and bolt on the
// manager once its config is resolved, and tests that swap in a fake.
// Swapping conn also rebinds the history controller so arrow-up
// history traversal starts hitting the new BFF.
func (a *App) SetConn(c ConnSurface) {
	a.conn = c
	a.history = NewHistoryController(c)
	a.input.SetHistorySource(a.history)
}

// SetTheme propagates the active theme to all child components.
func (a *App) SetTheme(t theme.Theme) {
	a.statusBar.SetTheme(t)
	a.viewport.SetTheme(t)
	a.input.SetTheme(t)
}

// History exposes the history controller (tests and integration wiring).
func (a App) History() *HistoryController { return a.history }

// Plan exposes the plan accumulator so the CLI launch path can wire
// it into the ToolDispatcher. Returns nil when no plan accumulator
// has been attached (tests that don't exercise plan mode).
func (a App) Plan() *PlanAccumulator { return a.plan }

// State exposes the shared state pointer for tests and for downstream
// integration code (connection manager, protocol client).
func (a App) State() *AppState { return a.state }

// Init satisfies tea.Model. Returns the initial command (a single
// Tick to drive the call-duration display in the status bar).
func (a App) Init() tea.Cmd {
	return tickCmd()
}

// tickCmd produces a one-shot 250ms ticker. The App reschedules itself
// on every TickMsg so the status bar's call-duration display stays
// fresh during streaming.
func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return TickMsg(t) })
}

// Update is the single entrypoint for all Bubbletea messages. Order:
// global key handling (quit / submit / mode toggle) first, then route
// the message to the focused child, then handle protocol/connection
// messages last so streaming state is reflected immediately on the
// frame the View call produces.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.recalculateLayout()
		return a, nil

	case tea.KeyMsg:
		// Global keys take precedence regardless of focus.
		if key.Matches(msg, a.keys.Quit) {
			return a, tea.Quit
		}
		// Pending paste choice captures keystrokes before the input area
		// or focus-routing see them (WU-083 overlay semantics).
		if a.paste != nil && a.paste.Active() {
			if cmd := a.paste.HandleKey(msg); cmd != nil {
				return a, cmd
			}
			// Unrelated key — swallow it so the input area doesn't
			// grow while the user is still deciding.
			return a, nil
		}
		// Compact handler captures keys with the same modal semantics.
		if a.compact != nil && a.compact.Active() {
			if cmd := a.compact.HandleKey(msg); cmd != nil {
				return a, cmd
			}
			return a, nil
		}
		if key.Matches(msg, a.keys.ToggleMode) {
			return a, a.setMode(a.cycleMode())
		}
		// Newline shortcuts: checked BEFORE submit so they work
		// regardless of the submit-key binding. Alt+Enter and Ctrl+J
		// are hardcoded cross-terminal ways to insert a newline when
		// Enter is the submit key.
		if a.state.Focus == InputFocus && isNewlineShortcut(msg) {
			a.input.InsertNewline()
			return a, nil
		}
		if key.Matches(msg, a.keys.Submit) && a.state.Focus == InputFocus {
			cmd := a.dispatchSubmit()
			return a, cmd
		}
		// Focus switching for arrow keys at edges.
		a.maybeSwitchFocus(msg)
		// Route to the focused child.
		switch a.state.Focus {
		case InputFocus:
			next, cmd := a.input.Update(msg)
			a.input = next
			return a, cmd
		case ViewportFocus:
			next, cmd := a.viewport.Update(msg)
			a.viewport = next
			return a, cmd
		}

	case TickMsg:
		// Auto-clear banners when their TTL expires.
		if a.state.Banner != "" && !a.bannerExpiresAt.IsZero() && !time.Time(msg).Before(a.bannerExpiresAt) {
			a.state.Banner = ""
			a.bannerExpiresAt = time.Time{}
			a.recalculateLayout()
		}
		return a, tickCmd()

	case BannerMsg:
		a.state.Banner = msg.Text
		if msg.Duration > 0 {
			a.bannerExpiresAt = time.Now().Add(msg.Duration)
		} else {
			a.bannerExpiresAt = time.Time{}
		}
		a.recalculateLayout()
		return a, nil

	case BannerClearMsg:
		a.state.Banner = ""
		a.bannerExpiresAt = time.Time{}
		a.recalculateLayout()
		return a, nil

	case ConnStateMsg:
		a.state.ConnState = msg.Info
		if a.connUX != nil {
			return a, a.connUX.HandleConnState(msg)
		}
		return a, nil

	case ModelUpdateMsg:
		a.state.ModelName = msg.Name
		a.state.ModelOverride = msg.Override
		a.state.ModelRouting = msg.Routing
		return a, nil

	case ContextUpdateMsg:
		a.state.ContextPct = msg.Pct
		a.state.ContextUsed = msg.Used
		a.state.ContextMax = msg.Max
		return a, nil

	case CostUpdateMsg:
		a.state.SessionCost = msg.Total
		return a, nil

	case ModeChangeMsg:
		a.state.Mode = msg.Mode
		return a, nil

	case StreamTokenMsg:
		// Append to the streaming buffer so the viewport renders the
		// growing assistant message in real time.
		a.appendStreamingDelta(msg.TurnID, msg.Delta)
		return a, nil

	case StreamCompleteMsg:
		a.finalizeStreaming(msg)
		return a, nil

	case PasteDetectedMsg:
		if a.paste != nil {
			return a, a.paste.HandlePaste(msg)
		}
		return a, nil

	case PasteResolvedMsg:
		a.input.ReplacePaste(msg.Original, msg.Content)
		if a.paste != nil {
			a.paste.Complete()
		}
		return a, func() tea.Msg { return BannerClearMsg{} }

	case PasteSummarizeRequestMsg:
		return a, a.dispatchPasteSummarize(msg)

	case SubmitMsg:
		return a, a.handleSubmit(msg)

	case TurnSubmittedMsg:
		if msg.Err != nil {
			return a, func() tea.Msg {
				return BannerMsg{
					Text:     "Turn submit failed: " + msg.Err.Error(),
					Duration: 4 * time.Second,
				}
			}
		}
		return a, nil

	case HistoryRefreshedMsg:
		return a, func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("History scope: %s (%d entries)", msg.Scope, msg.Count),
				Duration: 3 * time.Second,
			}
		}

	case HistoryErrMsg:
		return a, func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("History %s: %v", msg.Scope, msg.Err),
				Duration: 5 * time.Second,
			}
		}

	case ModelListLoadedMsg:
		return a, func() tea.Msg {
			return BannerMsg{Text: formatModelList(msg.Response), Duration: 8 * time.Second}
		}

	case ModelSwitchedMsg:
		if msg.Response != nil {
			a.state.ModelOverride = msg.Response.OverrideSet
			if msg.Response.Model != "" {
				a.state.ModelName = msg.Response.Model
			}
		}
		return a, func() tea.Msg {
			return BannerMsg{Text: formatModelSwitched(msg.Response), Duration: 4 * time.Second}
		}

	case ModelErrMsg:
		return a, func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("/%s failed: %v", msg.Command, msg.Err),
				Duration: 5 * time.Second,
			}
		}

	case SessionListLoadedMsg:
		return a, func() tea.Msg {
			return BannerMsg{Text: formatSessionList(msg.Response), Duration: 10 * time.Second}
		}

	case SessionResumedMsg:
		if msg.Response != nil {
			a.state.SessionID = msg.Response.SessionID
			if msg.Response.Model != "" {
				a.state.ModelName = msg.Response.Model
			}
			a.state.ModelOverride = msg.Response.ModelOverride != ""
		}
		return a, func() tea.Msg {
			id := ""
			if msg.Response != nil {
				id = msg.Response.SessionID
			}
			return BannerMsg{Text: "Resumed session " + id, Duration: 4 * time.Second}
		}

	case SessionClearedMsg:
		cleared := 0
		if msg.Response != nil {
			cleared = msg.Response.ClearedTurns
		}
		return a, func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("Session context cleared (%d turns dropped).", cleared),
				Duration: 4 * time.Second,
			}
		}

	case SessionForkedMsg:
		newID := ""
		if msg.Response != nil {
			newID = msg.Response.NewSessionID
		}
		if newID != "" {
			a.state.SessionID = newID
		}
		return a, func() tea.Msg {
			return BannerMsg{
				Text:     "Forked session — now on " + nonEmpty(newID, "(unknown)"),
				Duration: 4 * time.Second,
			}
		}

	case SessionErrMsg:
		return a, func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("/%s failed: %v", msg.Command, msg.Err),
				Duration: 5 * time.Second,
			}
		}

	case ContextListLoadedMsg:
		return a, func() tea.Msg {
			return BannerMsg{Text: formatContextList(msg.Response), Duration: 10 * time.Second}
		}

	case ContextErrMsg:
		return a, func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("/context failed: %v", msg.Err),
				Duration: 5 * time.Second,
			}
		}

	case MCPStatusLoadedMsg:
		return a, func() tea.Msg {
			return BannerMsg{Text: formatMCPStatus(msg.Servers), Duration: 8 * time.Second}
		}

	case MCPReconnectedMsg:
		return a, func() tea.Msg {
			return BannerMsg{Text: "Reconnecting MCP server " + msg.Name + "…", Duration: 4 * time.Second}
		}

	case MCPErrMsg:
		return a, func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("/%s failed: %v", msg.Command, msg.Err),
				Duration: 5 * time.Second,
			}
		}

	case CompactPlanLoadedMsg:
		return a, a.compact.Stage(msg.Plan)

	case compactApplyRequestMsg:
		return a, a.dispatchCompactApply(msg.actions)

	case CompactAppliedMsg:
		a.compact.Complete()
		return a, tea.Batch(
			func() tea.Msg { return BannerClearMsg{} },
			func() tea.Msg {
				return BannerMsg{Text: formatCompactApplied(msg.Response), Duration: 6 * time.Second}
			},
		)

	case CompactErrMsg:
		a.compact.Complete()
		return a, tea.Batch(
			func() tea.Msg { return BannerClearMsg{} },
			func() tea.Msg {
				return BannerMsg{
					Text:     fmt.Sprintf("/%s failed: %v", msg.Command, msg.Err),
					Duration: 5 * time.Second,
				}
			},
		)

	case pasteSummarizeFailureMsg:
		// Expand into (banner, cancel-resolve). Batch so both fire in
		// the same tick and the overlay clears before the next key.
		banner := BannerMsg{
			Text:     "Summarize failed: " + msg.reason,
			Duration: 5 * time.Second,
		}
		resolve := PasteResolvedMsg{Strategy: PasteStrategyCancel, Original: msg.original}
		return a, tea.Batch(
			func() tea.Msg { return banner },
			func() tea.Msg { return resolve },
		)
	}
	return a, nil
}

// View composes the four zones: viewport, optional banner, input area,
// status bar (top-to-bottom).
func (a App) View() string {
	var sb strings.Builder
	sb.WriteString(a.viewport.View())
	sb.WriteString("\n")
	if a.state.Banner != "" {
		sb.WriteString(a.state.Banner)
		sb.WriteString("\n")
	}
	sb.WriteString(a.input.View())
	sb.WriteString("\n")
	sb.WriteString(a.statusBar.View())
	return sb.String()
}

// recalculateLayout assigns each component its share of the terminal
// rectangle per design D2.3.
func (a *App) recalculateLayout() {
	statusBarHeight := 1
	bannerHeight := a.bannerLines()
	inputHeight := a.input.Height()
	if max := a.height / 3; max > 0 && inputHeight > max {
		inputHeight = max
	}
	if inputHeight < 1 {
		inputHeight = 1
	}
	viewportHeight := a.height - statusBarHeight - inputHeight - bannerHeight
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	a.viewport.SetSize(a.width, viewportHeight)
	a.input.SetWidth(a.width)
	a.statusBar.SetWidth(a.width)
}

// bannerLines returns the height in rows that the banner occupies.
// Banners are at most 2 lines per spec.
func (a *App) bannerLines() int {
	if a.state.Banner == "" {
		return 0
	}
	count := strings.Count(a.state.Banner, "\n") + 1
	if count > 2 {
		count = 2
	}
	return count
}

// cycleMode implements the WU-080 design D1 Ctrl+P semantics:
//
//	plan  → build
//	build → plan
//	auto  → build  (always drops back to build from auto)
//
// This is intentionally different from a 3-way cycle — plan and build
// are the two "active" states the user switches between, and Ctrl+P
// is the quick toggle. Auto is entered explicitly via /auto and
// leaves via Ctrl+P → build.
func (a *App) cycleMode() protocol.Mode {
	switch a.state.Mode {
	case protocol.ModePlan:
		return protocol.ModeBuild
	case protocol.ModeBuild:
		return protocol.ModePlan
	case protocol.ModeAuto:
		return protocol.ModeBuild
	}
	return protocol.ModeBuild
}

// maybeSwitchFocus implements the edge-driven focus transition rules
// from design D2.5 (now D2.5 of the bundle): Up at input top → viewport,
// Down at viewport bottom → input, any printable rune in viewport →
// input.
func (a *App) maybeSwitchFocus(msg tea.KeyMsg) {
	switch {
	case key.Matches(msg, a.keys.ScrollUp) && a.state.Focus == InputFocus:
		if a.input.CursorAtTop() {
			a.state.Focus = ViewportFocus
		}
	case key.Matches(msg, a.keys.ScrollDown) && a.state.Focus == ViewportFocus:
		if a.viewport.AtBottom() {
			a.state.Focus = InputFocus
		}
	case msg.Type == tea.KeyRunes && a.state.Focus == ViewportFocus:
		a.state.Focus = InputFocus
	}
}

// dispatchSubmit converts the input area's value into a SubmitMsg,
// clears the input, and returns the command. The actual protocol
// transmission happens in the connection manager (WU-074); this
// helper produces the message it consumes.
func (a *App) dispatchSubmit() tea.Cmd {
	value := a.input.Value()
	a.input.SetValue("")
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	msg := SubmitMsg{Content: value}
	if strings.HasPrefix(value, "/") {
		msg.IsCommand = true
		parts := strings.SplitN(strings.TrimPrefix(value, "/"), " ", 2)
		msg.Command = parts[0]
		if len(parts) == 2 {
			msg.CommandArgs = parts[1]
		}
	}
	return func() tea.Msg { return msg }
}

// dispatchPasteSummarize issues a content.transform call for the
// pasted content via the wired ConnSurface. The returned tea.Cmd
// emits a PasteResolvedMsg with PasteStrategySummarize and the
// server-returned summary on success, or a BannerMsg + cancel
// resolution on failure. When no ConnSurface is wired the summarize
// path degrades to a cancel resolution with an explanatory banner —
// the app remains usable even without a running BFF.
func (a *App) dispatchPasteSummarize(msg PasteSummarizeRequestMsg) tea.Cmd {
	conn := a.conn
	return func() tea.Msg {
		if conn == nil {
			return pasteSummarizeFail(msg.Content, "no connection wired")
		}
		client := conn.Client()
		if client == nil {
			return pasteSummarizeFail(msg.Content, "not connected to BFF")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		raw, err := client.ContentTransform(ctx, &protocol.ContentTransform{
			Transform:   "summarize",
			RawContent:  msg.Content,
			ContentType: "text/plain",
		})
		if err != nil {
			return pasteSummarizeFail(msg.Content, err.Error())
		}
		var resp protocol.ContentTransformResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return pasteSummarizeFail(msg.Content, "decode: "+err.Error())
		}
		return PasteResolvedMsg{
			Strategy: PasteStrategySummarize,
			Content:  resp.Content,
			Original: msg.Content,
		}
	}
}

// pasteSummarizeFail is the uniform failure envelope for the summarize
// path — it resolves the overlay (cancel strategy) and surfaces the
// failure via a banner the runtime queues right after the resolve.
// Returning a single tea.Msg isn't enough (we need two), so this
// returns a BatchedPasteFailureMsg sentinel the Update loop expands.
func pasteSummarizeFail(original, reason string) tea.Msg {
	return pasteSummarizeFailureMsg{original: original, reason: reason}
}

// pasteSummarizeFailureMsg is the internal marker expanded by Update
// into (BannerMsg, PasteResolvedMsg{cancel}). Keeps the fallback path
// in one place.
type pasteSummarizeFailureMsg struct {
	original string
	reason   string
}

// handleSubmit routes a SubmitMsg: slash commands go through the
// conn-backed command handler; free-form text dispatches a turn.submit
// via the wired ConnSurface. When no conn is wired, slash commands
// emit informational banners and free-form submits error out.
func (a *App) handleSubmit(msg SubmitMsg) tea.Cmd {
	if msg.IsCommand {
		return a.handleCommand(msg)
	}
	return a.dispatchTurnSubmit(msg)
}

// handleCommand handles the slash commands this patch wires:
// /status (show connection state) and /reconnect (force reconnect).
// Unknown commands produce a "unknown command" banner.
func (a *App) handleCommand(msg SubmitMsg) tea.Cmd {
	switch strings.ToLower(msg.Command) {
	case "status":
		state := "no connection wired"
		if a.conn != nil {
			state = a.conn.State()
		}
		banner := BannerMsg{Text: "Connection: " + state, Duration: 4 * time.Second}
		return func() tea.Msg { return banner }

	case "reconnect":
		if a.conn == nil {
			return func() tea.Msg {
				return BannerMsg{Text: "Cannot reconnect: no connection wired", Duration: 4 * time.Second}
			}
		}
		reconnectCmd := a.conn.Reconnect()
		announce := BannerMsg{Text: "Reconnecting…", Duration: 0}
		return tea.Batch(
			func() tea.Msg { return announce },
			reconnectCmd,
		)

	case "history":
		scope := strings.TrimSpace(strings.ToLower(msg.CommandArgs))
		if scope == "" {
			scope = a.history.Scope()
			banner := BannerMsg{
				Text:     fmt.Sprintf("Current history scope: %s (%d entries)", scope, a.history.Len()),
				Duration: 4 * time.Second,
			}
			return func() tea.Msg { return banner }
		}
		return a.history.SetScope(scope)

	case "model", "models":
		return a.handleModelCommand(msg)

	case "session", "sessions":
		return a.handleSessionCommand(msg)

	case "context":
		return a.handleContextCommand(msg)

	case "plan", "build", "auto":
		return a.handleModeCommand(msg)

	case "mcp":
		return a.handleMCPCommand(msg)

	case "help":
		return func() tea.Msg {
			return BannerMsg{
				Text: "Available commands:\n" +
					"  /help           show this help\n" +
					"  /status         show connection state\n" +
					"  /reconnect      force reconnect\n" +
					"  /plan           plan mode (Ctrl+P/Tab toggles)\n" +
					"  /build          build mode\n" +
					"  /auto           auto mode\n" +
					"  /model <name>   override model\n" +
					"  /model auto     clear override\n" +
					"  /models         list models\n" +
					"  /sessions       list sessions\n" +
					"  /session        show current session\n" +
					"  /session resume <id>\n" +
					"  /session fork   fork session\n" +
					"  /session clear  clear context\n" +
					"  /context        show context\n" +
					"  /compact        compact context\n" +
					"  /mcp [status]   MCP state\n" +
					"  @file           attach file\n" +
					"  Enter            submit\n" +
					"  Ctrl+J           insert newline\n" +
					"  Ctrl+P/Tab       toggle mode",
				Duration: 10 * time.Second,
			}
		}

	case "compact":
		return a.handleCompactCommand(msg)
	}
	unknown := BannerMsg{
		Text:     "Unknown command: /" + msg.Command,
		Duration: 3 * time.Second,
	}
	return func() tea.Msg { return unknown }
}

// dispatchTurnSubmit translates a free-form SubmitMsg into a
// turn.submit RPC. Runs the call on a background goroutine and emits
// TurnSubmittedMsg with the ack's TurnID (or the error). Streaming
// events for the turn flow in separately via the ConnectionManager's
// event bridge.
func (a *App) dispatchTurnSubmit(msg SubmitMsg) tea.Cmd {
	if a.conn == nil {
		return func() tea.Msg {
			return TurnSubmittedMsg{Err: errNoConnection}
		}
	}
	client := a.conn.Client()
	if client == nil {
		return func() tea.Msg {
			return TurnSubmittedMsg{Err: errNotConnected}
		}
	}
	sessionID := a.state.SessionID
	seq := a.state.NextSequence()
	content := msg.Content
	refs := msg.Attachments
	attacher := a.attacher
	mode := a.state.Mode
	// TurnIDs are harness-assigned per protocol contract; the server
	// echoes them back so both sides can correlate streaming events.
	turnID := "turn-" + uuid.NewString()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var attachments []protocol.Attachment
		if attacher != nil && len(refs) > 0 {
			resolved, err := attacher.Resolve(ctx, refs)
			if err != nil {
				return TurnSubmittedMsg{Err: err}
			}
			attachments = resolved
		}

		ack, err := client.SubmitTurn(ctx, &protocol.TurnSubmit{
			TurnID:      turnID,
			SessionID:   sessionID,
			Sequence:    seq,
			Mode:        mode,
			Content:     content,
			Attachments: attachments,
		})
		if err != nil {
			return TurnSubmittedMsg{Err: err}
		}
		// Prefer the server-echoed TurnID; fall back to ours if omitted.
		finalID := ack.TurnID
		if finalID == "" {
			finalID = turnID
		}
		return TurnSubmittedMsg{TurnID: finalID}
	}
}

// appendStreamingDelta records a streamed token chunk into the
// in-progress assistant message. The viewport reads
// AppState.Messages on every View call so this update is visible on
// the next frame.
func (a *App) appendStreamingDelta(turnID, delta string) {
	if turnID == "" {
		return
	}
	if a.state.StreamingTurnID != turnID {
		a.state.StreamingTurnID = turnID
		a.state.StreamingBuf.Reset()
		a.state.Messages = append(a.state.Messages, DisplayMessage{
			Role:      RoleAssistant,
			TurnID:    turnID,
			Streaming: true,
		})
	}
	a.state.StreamingBuf.WriteString(delta)
	if i := len(a.state.Messages) - 1; i >= 0 && a.state.Messages[i].TurnID == turnID {
		a.state.Messages[i].Content = a.state.StreamingBuf.String()
	}
}

// finalizeStreaming stamps the final metadata on the in-progress
// assistant message and clears the streaming flag.
func (a *App) finalizeStreaming(msg StreamCompleteMsg) {
	for i := range a.state.Messages {
		if a.state.Messages[i].TurnID == msg.TurnID {
			a.state.Messages[i].Streaming = false
			a.state.Messages[i].Tokens = msg.Tokens
			a.state.Messages[i].Cost = msg.Cost
			a.state.Messages[i].Duration = msg.Duration
			a.state.Messages[i].Model = msg.Model
		}
	}
	a.state.StreamingTurnID = ""
	a.state.StreamingBuf.Reset()
	a.state.CallActive = false
}

// isNewlineShortcut reports whether a key event should insert a newline
// in the input area rather than trigger the submit key binding. These
// are hardcoded cross-terminal shortcuts: Alt+Enter (works on terminals
// that support keyboard disambiguation) and Ctrl+J (ASCII linefeed,
// works everywhere). They are evaluated before the submit-key check so
// they always insert a newline regardless of the configured submit key.
func isNewlineShortcut(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyEnter && msg.Alt || msg.Type == tea.KeyCtrlJ
}

// TerminalResponseFilter drops KeyMsg values that are terminal response
// sequences leaking through the input stream (OSC background/foreground
// queries, cursor-position reports, ST terminators, etc.). It is intended
// for use with tea.WithFilter so the garbage is removed before Update runs.
func TerminalResponseFilter(_ tea.Model, msg tea.Msg) tea.Msg {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return msg
	}
	if isTerminalGarbage(km) {
		return nil
	}
	return msg
}

func isTerminalGarbage(msg tea.KeyMsg) bool {
	// ST terminator fragment parsed as Alt+\.
	if msg.Alt && len(msg.Runes) == 1 && msg.Runes[0] == '\\' {
		return true
	}
	s := string(msg.Runes)
	if strings.HasPrefix(s, "\x1b") {
		return true
	}
	if strings.HasPrefix(s, "]") && len(s) > 1 {
		c := s[1]
		if c >= '0' && c <= '9' {
			return true // OSC response (]10;…, ]11;…)
		}
	}
	if strings.HasPrefix(s, "[") && len(s) > 1 {
		c := s[1]
		if c >= '0' && c <= '9' {
			return true // CPR or similar CSI response
		}
		if c == 'I' || c == 'O' {
			return true // focus-event responses
		}
	}
	return false
}
