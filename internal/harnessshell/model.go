package harnessshell

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Option configures shell-local defaults at construction time. Per WU-098,
// options must not include callback hooks (submit handlers, preview loaders,
// permission resolvers, stream writers); those concerns belong to the typed
// action/event boundary.
type Option func(*Model)

// WithTitle sets the initial shell title for the chrome surface.
func WithTitle(title string) Option {
	return func(m *Model) {
		m.state.title = title
	}
}

// WithLabel sets the initial host-fed display label (e.g. model or agent
// label) shown in chrome.
func WithLabel(label string) Option {
	return func(m *Model) {
		m.state.label = label
	}
}

// WithPlaceholder sets the composer placeholder text. The shell otherwise
// owns its own placeholder default.
func WithPlaceholder(placeholder string) Option {
	return func(m *Model) {
		m.placeholder = placeholder
	}
}

// WithSidebarOpen sets the initial sidebar open/closed state.
func WithSidebarOpen(open bool) Option {
	return func(m *Model) {
		m.state.sidebarOpen = open
	}
}

// Model is the reusable conversation-shell Bubble Tea model. It satisfies
// [tea.Model] and is intended to be embedded in a host program that relays
// host events back through Update.
//
// Stage C wires shell-local key handling (focus cycle, history recall,
// composer-token selection, transcript-token movement) and projects shell-
// owned state into [RenderInput] for [Render]. Outbound actions accumulate
// on the private state action queue and are forwarded as [ActionMsg] via
// [tea.Cmd] returned from Update.
type Model struct {
	state       state
	placeholder string
}

// Compile-time check that Model satisfies tea.Model.
var _ tea.Model = Model{}

// New constructs a [Model] with the given options applied. Callers should
// then forward [tea.WindowSizeMsg], [tea.KeyMsg], and [HostEvent] values
// through Update.
func New(opts ...Option) Model {
	ta := textarea.New()
	ta.Prompt = "▎ "
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6F86A3"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6F86A3"))
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF")).Bold(true)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#4A5668"))
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5F7FB"))
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF"))
	ta.Focus()

	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	m := Model{
		state: state{
			focus:                 FocusInput,
			historyIndex:          -1,
			activePermissionIndex: 0,
			selectedToken:         -1,
			selectedTranscriptRef: -1,
			statusKind:            StatusReady,
			input:                 ta,
			transcript:            vp,
		},
	}
	for _, opt := range opts {
		opt(&m)
	}
	if m.placeholder != "" {
		m.state.input.Placeholder = m.placeholder
	}
	return m
}

// Init returns the initial Bubble Tea command for the shell. Stage C returns
// [textarea.Blink] so the composer cursor blinks as soon as the host program
// starts the model.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles a Bubble Tea message and returns the next model and
// command. Stage C wires shell-local key handling: window-size adjustments,
// Tab focus cycling, single-line Up/Down history recall, Ctrl+P/Ctrl+N
// composer-token selection, and Up/Down/j/k transcript-token movement when
// the transcript zone is focused. Submit/interrupt/permission/preview key
// paths and host-event intake are intentionally left for subsequent Stage C
// commits; the spike continues to drive those paths through its own event
// loop.
//
// Outbound actions queued on the shell state are drained at the end of the
// update tick and forwarded as [ActionMsg] via [tea.Cmd]. The host program
// pattern-matches on [ActionMsg] in its own update loop and dispatches the
// concrete [Action] to the host adapter.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.state.width = msg.Width
		m.state.height = msg.Height
		composerInner := msg.Width - 4
		if composerInner < 1 {
			composerInner = 1
		}
		m.state.input.SetWidth(composerInner)
		m.state.transcript.Width = msg.Width
		m.state.transcript.Height = msg.Height

	case tea.KeyMsg:
		if handled, next, cmd := m.handleKey(msg); handled {
			cmds = append(cmds, cmd)
			m = next
			break
		}
		var passthrough tea.Cmd
		m, passthrough = m.routeKeyToFocus(msg)
		if passthrough != nil {
			cmds = append(cmds, passthrough)
		}

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.state.transcript, cmd = m.state.transcript.Update(msg)
		cmds = append(cmds, cmd)

	default:
		// Forward other messages to the focused widget so cursor blink
		// timers, paste events, and similar bubble-internal traffic still
		// reach their owners.
		switch m.state.focus {
		case FocusInput:
			var cmd tea.Cmd
			m.state.input, cmd = m.state.input.Update(msg)
			cmds = append(cmds, cmd)
		case FocusTranscript:
			var cmd tea.Cmd
			m.state.transcript, cmd = m.state.transcript.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if drainCmd := m.drainPendingActions(); drainCmd != nil {
		cmds = append(cmds, drainCmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKey routes a [tea.KeyMsg] through shell-local handlers (focus
// cycle, token selection, transcript-ref movement, single-line history
// recall). It returns (true, next, cmd) when the key was consumed; on a
// miss it returns (false, m, nil) so the caller can pass the key through
// to the focused widget.
func (m Model) handleKey(msg tea.KeyMsg) (bool, Model, tea.Cmd) {
	if msg.Type == tea.KeyTab {
		m.state.focus = nextFocus(m.state.focus, m.state.sidebarOpen)
		if m.state.focus == FocusInput {
			m.state.input.Focus()
		} else {
			m.state.input.Blur()
		}
		return true, m, nil
	}

	switch strings.ToLower(msg.String()) {
	case "ctrl+n":
		if m.state.focus == FocusInput && len(m.state.inputTokens) > 0 {
			m.state.moveTokenSelection(1)
			return true, m, nil
		}
	case "ctrl+p":
		if m.state.focus == FocusInput && len(m.state.inputTokens) > 0 {
			m.state.moveTokenSelection(-1)
			return true, m, nil
		}
	}

	switch m.state.focus {
	case FocusTranscript:
		switch msg.Type {
		case tea.KeyUp:
			if len(m.state.transcriptRefs) > 0 {
				m.state.moveTranscriptRef(-1)
				return true, m, nil
			}
		case tea.KeyDown:
			if len(m.state.transcriptRefs) > 0 {
				m.state.moveTranscriptRef(1)
				return true, m, nil
			}
		}
		switch msg.String() {
		case "k":
			if len(m.state.transcriptRefs) > 0 {
				m.state.moveTranscriptRef(-1)
				return true, m, nil
			}
		case "j":
			if len(m.state.transcriptRefs) > 0 {
				m.state.moveTranscriptRef(1)
				return true, m, nil
			}
		}
	case FocusInput:
		// Single-line Up/Down recall the command history. Multi-line
		// buffers fall through to the textarea so cursor navigation
		// still works inside the composer.
		if msg.Type == tea.KeyUp && !strings.Contains(m.state.input.Value(), "\n") {
			m.state.recallPreviousCommand()
			return true, m, nil
		}
		if msg.Type == tea.KeyDown && !strings.Contains(m.state.input.Value(), "\n") {
			m.state.recallNextCommand()
			return true, m, nil
		}
	}

	return false, m, nil
}

// routeKeyToFocus forwards an unhandled [tea.KeyMsg] to the focused
// widget. Composer mutations (paste capture, dropped-path detection,
// dynamic textarea height) run through the shell helpers immediately
// after the textarea consumes the key.
func (m Model) routeKeyToFocus(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch m.state.focus {
	case FocusInput:
		prev := m.state.input.Value()
		var cmd tea.Cmd
		m.state.input, cmd = m.state.input.Update(msg)
		m.state.handleInputMutation(prev, m.state.input.Value())
		m.state.syncInputHeight()
		return m, cmd
	case FocusTranscript:
		var cmd tea.Cmd
		m.state.transcript, cmd = m.state.transcript.Update(msg)
		return m, cmd
	}
	return m, nil
}

// drainPendingActions empties the shell's outbound action queue into a
// [tea.Cmd] that emits one [ActionMsg] per queued action. Returns nil
// when the queue is empty.
func (m *Model) drainPendingActions() tea.Cmd {
	if len(m.state.pendingActions) == 0 {
		return nil
	}
	actions := m.state.pendingActions
	m.state.pendingActions = nil
	cmds := make([]tea.Cmd, len(actions))
	for i, a := range actions {
		action := a
		cmds[i] = func() tea.Msg { return ActionMsg{Action: action} }
	}
	return tea.Batch(cmds...)
}

// View renders the current shell. Stage C projects shell-owned state into
// [RenderInput], calls [Render], pipes the result string into a local copy
// of the transcript viewport, and returns the viewport view. The viewport
// copy isolates the per-frame [SetContent] mutation from the persisted
// state in `m.state.transcript`, so View remains pure (no observable
// mutation of `m`) per the [tea.Model] convention.
func (m Model) View() string {
	in := m.toRenderInput()
	result := Render(in)
	vp := m.state.transcript
	vp.SetContent(result.Content)
	return vp.View()
}

// toRenderInput projects shell-owned state into the [RenderInput] consumed
// by [Render]. Per the WU-100 Stage A→B bridge note, this projection
// replaces the spike's `toShellRenderInput` once the spike-shim cutover
// lands.
func (m Model) toRenderInput() RenderInput {
	in := RenderInput{
		Width:                 m.state.transcript.Width,
		ModelLabel:            m.state.label,
		InputView:             m.state.input.View(),
		SelectedToken:         m.state.selectedToken,
		SelectedTranscriptRef: m.state.selectedTranscriptRef,
		Focus:                 toRenderFocus(m.state.focus),
		Streaming:             m.state.streaming,
		StreamPulse:           m.state.streamPulse,
		InterruptArmed:        m.state.interruptArmed,
		QueuedCount:           len(m.state.queuedSubmissions),
	}
	if len(m.state.transcriptItems) > 0 {
		in.Messages = make([]RenderMessage, 0, len(m.state.transcriptItems))
		for _, item := range m.state.transcriptItems {
			in.Messages = append(in.Messages, transcriptItemToRender(item))
		}
	}
	if len(m.state.queuedSubmissions) > 0 {
		in.Queued = make([]RenderQueued, len(m.state.queuedSubmissions))
		for i, q := range m.state.queuedSubmissions {
			in.Queued[i] = RenderQueued{
				Content: q.Text,
				Tokens:  inputTokensToRender(q.Tokens),
				Entries: q.Entries,
			}
		}
	}
	if len(m.state.inputTokens) > 0 {
		in.InputTokens = inputTokensToRender(m.state.inputTokens)
	}
	if p := pendingPermissionView(&m.state); p != nil {
		in.PendingPermission = &RenderPendingPermission{
			ToolLabel:       p.Request.ToolLabel,
			ToolTarget:      p.Request.Target,
			ToolSummary:     p.Request.Summary,
			SelectedAction:  p.SelectedAction,
			SessionPolicyOn: p.Request.SessionPolicyState.SessionApproved,
			PendingTotal:    len(m.state.pendingPermissions),
			ActiveIndex:     m.state.activePermissionIndex,
		}
		in.PermissionComposerActive = m.state.focus == FocusInput && m.state.input.Value() == ""
	}
	return in
}

// pendingPermissionView returns the active pending permission for read-only
// rendering use. It mirrors `state.currentPendingPermission` but skips the
// clamp side effect so View remains pure.
func pendingPermissionView(s *state) *PendingPermission {
	if len(s.pendingPermissions) == 0 {
		return nil
	}
	idx := s.activePermissionIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.pendingPermissions) {
		idx = len(s.pendingPermissions) - 1
	}
	return &s.pendingPermissions[idx]
}

// transcriptItemToRender translates a shell-owned [TranscriptItem] into the
// renderer's [RenderMessage] shape.
func transcriptItemToRender(item TranscriptItem) RenderMessage {
	out := RenderMessage{
		Role:      string(item.Role),
		Content:   item.Text,
		Streaming: item.Streaming,
		Entries:   item.Entries,
		Expanded:  expandedToIndexMap(item.Expanded, item.Tokens),
	}
	if item.Event != nil {
		out.EventState = item.Event.Status
	}
	if len(item.Tokens) > 0 {
		out.Tokens = inputTokensToRender(item.Tokens)
	}
	return out
}

// expandedToIndexMap converts the [TranscriptItem.Expanded] keyed-by-token-ID
// map into the index-keyed map shape the renderer consumes.
func expandedToIndexMap(expanded map[string]bool, tokens []InputToken) map[int]bool {
	if len(expanded) == 0 || len(tokens) == 0 {
		return nil
	}
	out := make(map[int]bool, len(tokens))
	for i, tok := range tokens {
		if expanded[tok.ID] {
			out[i] = true
		}
	}
	return out
}

// inputTokensToRender converts a slice of [InputToken] into the renderer's
// [RenderToken] shape.
func inputTokensToRender(tokens []InputToken) []RenderToken {
	if len(tokens) == 0 {
		return nil
	}
	out := make([]RenderToken, len(tokens))
	for i, t := range tokens {
		out[i] = RenderToken{
			ID:      t.ID,
			Kind:    string(t.Kind),
			Label:   t.Label,
			Payload: t.Payload,
		}
	}
	return out
}

// toRenderFocus maps a [FocusZone] to the renderer's [RenderFocus] enum.
func toRenderFocus(f FocusZone) RenderFocus {
	switch f {
	case FocusTranscript:
		return RenderFocusTranscript
	case FocusSidebar:
		return RenderFocusSidebar
	default:
		return RenderFocusInput
	}
}
