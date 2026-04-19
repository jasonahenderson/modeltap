package harness

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// AppOptions configures App construction.
type AppOptions struct {
	// SubmitKey is one of the SubmitKey* constants; empty defaults to
	// SubmitKeyCtrlEnter.
	SubmitKey string
	// InitialMode sets the starting execution mode. Empty defaults to
	// protocol.ModeBuild.
	InitialMode protocol.Mode
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
	return App{
		state:     state,
		statusBar: NewStatusBar(state),
		input:     NewInputArea(state),
		viewport:  NewConversationViewport(state),
		keys:      DefaultKeyMap(opts.SubmitKey),
		connUX:    NewConnectionUX(state),
		paste:     NewPasteHandler(),
	}
}

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
		if key.Matches(msg, a.keys.ToggleMode) {
			a.cycleMode()
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

// cycleMode advances Mode through plan → build → auto → plan…
// per design D2.5 (Ctrl+P toggle). The full mode-toggle UX (banner
// hint, status update) lands in WU-080.
func (a *App) cycleMode() {
	switch a.state.Mode {
	case protocol.ModePlan:
		a.state.Mode = protocol.ModeBuild
	case protocol.ModeBuild:
		a.state.Mode = protocol.ModeAuto
	case protocol.ModeAuto:
		a.state.Mode = protocol.ModePlan
	default:
		a.state.Mode = protocol.ModeBuild
	}
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
