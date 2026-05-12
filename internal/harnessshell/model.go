package harnessshell

import (
	"fmt"
	"strings"
	"time"

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

// WithStreamTick overrides the factory used to schedule the 1Hz
// elapsed-seconds tick (PATCH-0035). Production code uses the
// internal default (tea.Tick(time.Second, ...)). Tests that drive
// Update synchronously via a pump loop should pass a no-op factory
// (e.g. `func() tea.Cmd { return nil }`) so the real Tick's 1-second
// sleep does not multiply their step budget. Passing nil leaves the
// default in place.
func WithStreamTick(factory func() tea.Cmd) Option {
	return func(m *Model) {
		if factory != nil {
			m.state.streamTick = factory
		}
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
			mouseCaptureDisabled:  true,
			input:                 ta,
			transcript:            vp,
			streamTick:            streamTickCmd, // PATCH-0035; tests may override
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

	case HostEvent:
		m.state.applyHostEvent(msg)

	case streamTickMsg:
		// PATCH-0035: 1Hz elapsed-seconds ticker. Reschedule while a
		// run is still streaming; let the loop expire on terminal
		// events (applyRunCompleted / Stopped / Failed clear
		// runStartedAt and streaming, so the next tick is a no-op).
		if m.state.streaming && !m.state.runStartedAt.IsZero() && m.state.streamTick != nil {
			if cmd := m.state.streamTick(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

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

	// PATCH-0035: drain shell-internal pending tea.Cmds (e.g., the
	// streamTickCmd queued by applyRunStarted).
	if len(m.state.pendingCmds) > 0 {
		cmds = append(cmds, m.state.pendingCmds...)
		m.state.pendingCmds = nil
	}

	// FEAT-0014 SC3: keep the persistent viewport's content in sync
	// with the latest transcript state and apply followTail/scroll-
	// preservation. View() then just renders m.state.transcript.View().
	m.state.refresh()

	return m, tea.Batch(cmds...)
}

// handleKey routes a [tea.KeyMsg] through shell-local handlers (focus
// cycle, token selection, transcript-ref movement, single-line history
// recall). It returns (true, next, cmd) when the key was consumed; on a
// miss it returns (false, m, nil) so the caller can pass the key through
// to the focused widget.
func (m Model) handleKey(msg tea.KeyMsg) (bool, Model, tea.Cmd) {
	// Esc dismisses an open shell-local preview before reaching any
	// other Esc handler. Mirrors the spike's overlay-dismiss
	// precedence for the in-scope dialog.
	if msg.Type == tea.KeyEsc && m.state.preview != nil {
		m.state.preview = nil
		m.state.status = "Preview closed"
		return true, m, nil
	}

	if msg.Type == tea.KeyEsc && m.state.streaming {
		if m.state.interruptArmed {
			// Second Esc emits the interrupt; the host's RunStoppedEvent
			// (or RunFailedEvent) intake clears streaming chrome and
			// preserves the queue per FEAT-0014 ("stop does not
			// auto-resume the stopped run").
			m.state.pendingActions = append(m.state.pendingActions, InterruptRunAction{RunID: m.state.activeRunID})
			m.state.interruptArmed = false
			m.state.status = "Stopping run"
			m.state.statusKind = StatusStreaming
		} else {
			m.state.interruptArmed = true
			m.state.status = "Press Esc again to interrupt"
			m.state.statusKind = StatusInterruptArmed
		}
		return true, m, nil
	}

	if msg.Type == tea.KeyTab {
		m.state.focus = nextFocus(m.state.focus, m.state.sidebarOpen)
		if m.state.focus == FocusInput {
			m.state.input.Focus()
		} else {
			m.state.input.Blur()
		}
		return true, m, nil
	}

	// PATCH-0034: focus-agnostic transcript scroll. PgUp/PgDn always
	// page the transcript regardless of which zone has focus; Alt+Up
	// / Alt+Down nudge by one line. This restores a discoverable
	// scroll path in the default (input-focused, no /select) state
	// where PATCH-0030 turned off mouse capture for native selection.
	switch msg.Type {
	case tea.KeyPgUp:
		m.state.transcript.PageUp()
		return true, m, nil
	case tea.KeyPgDown:
		m.state.transcript.PageDown()
		return true, m, nil
	}
	if msg.Alt {
		switch msg.Type {
		case tea.KeyUp:
			m.state.transcript.LineUp(1)
			return true, m, nil
		case tea.KeyDown:
			m.state.transcript.LineDown(1)
			return true, m, nil
		}
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
	case "ctrl+o":
		// Composer-token preview when input focused with tokens;
		// transcript-token preview when transcript focused with refs.
		if m.state.focus == FocusInput && len(m.state.inputTokens) > 0 {
			m.state.previewSelectedComposerToken()
			return true, m, nil
		}
		if m.state.focus == FocusTranscript && len(m.state.transcriptRefs) > 0 {
			m.state.previewSelectedTranscriptRef()
			return true, m, nil
		}
	}

	switch m.state.focus {
	case FocusTranscript:
		switch msg.Type {
		case tea.KeyEnter:
			if len(m.state.transcriptRefs) > 0 {
				m.state.activateSelectedTranscriptRef()
				return true, m, nil
			}
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
		// Enter (without Alt) submits unless the textarea has been
		// instructed to insert a newline via Alt+Enter / Ctrl+J. The
		// shell-native /clear command, queue release, queue follow-up,
		// and permission-resolve paths are all routed through
		// emitSubmitOnEnter.
		if msg.Type == tea.KeyEnter && !msg.Alt {
			if isShellNativeQuitCommand(m.state.input.Value()) && len(m.state.inputTokens) == 0 {
				m.state.input.Reset()
				m.state.syncInputHeight()
				m.state.status = "Exiting"
				return true, m, tea.Quit
			}
			if isShellNativeSelectCommand(m.state.input.Value()) && len(m.state.inputTokens) == 0 {
				updated, cmd := m.toggleSelectMode()
				return true, updated, cmd
			}
			if m.state.emitSubmitOnEnter() {
				return true, m, nil
			}
		}
		if msg.Type == tea.KeyEnter && msg.Alt {
			m.state.input.InsertRune('\n')
			m.state.syncInputHeight()
			return true, m, nil
		}
		if msg.Type == tea.KeyCtrlJ {
			m.state.input.InsertRune('\n')
			m.state.syncInputHeight()
			return true, m, nil
		}

		// Permission shortcuts when the composer is empty: y/Y approves
		// once, n/N denies, Left/Right walks the action selector,
		// Up/Down (also empty) walks between multiple pending
		// permissions before falling through to history recall.
		if m.state.input.Value() == "" && m.state.currentPendingPermission() != nil {
			switch msg.String() {
			case "y", "Y":
				m.state.resolveActivePermission(DecisionApproveOnce)
				return true, m, nil
			case "n", "N":
				m.state.resolveActivePermission(DecisionDeny)
				return true, m, nil
			}
			switch msg.Type {
			case tea.KeyLeft:
				if m.state.movePermissionAction(-1) {
					return true, m, nil
				}
			case tea.KeyRight:
				if m.state.movePermissionAction(1) {
					return true, m, nil
				}
			}
		}
		if msg.Type == tea.KeyUp && m.state.input.Value() == "" {
			if m.state.movePendingPermission(-1) {
				return true, m, nil
			}
		}
		if msg.Type == tea.KeyDown && m.state.input.Value() == "" {
			if m.state.movePendingPermission(1) {
				return true, m, nil
			}
		}
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

// toggleSelectMode flips the /select toggle (PATCH-0030). Selection mode is
// the default: the terminal handles mouse natively so click-drag selection,
// copy, and paste shortcuts keep working. Toggling into chat mode enables
// Bubble Tea mouse capture for mouse-wheel viewport scrolling; toggling back
// disables capture again. The composer is reset so /select itself doesn't echo
// back as user content on the next submit.
func (m Model) toggleSelectMode() (Model, tea.Cmd) {
	m.state.input.Reset()
	m.state.syncInputHeight()
	m.state.mouseCaptureDisabled = !m.state.mouseCaptureDisabled
	if !m.state.mouseCaptureDisabled {
		m.state.status = "Chat mode - mouse captured for scroll; type /select for terminal selection"
		m.state.statusKind = StatusReady
		return m, tea.EnableMouseAllMotion
	}
	m.state.status = "Selection mode - terminal handles mouse; type /select for mouse scroll"
	m.state.statusKind = StatusReady
	return m, tea.DisableMouse
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

// View renders the current shell. Returns the transcript viewport's
// rendered string. [Model.Update] is responsible for keeping the
// viewport's content in sync with the shell's transcript state via
// [refresh] — View itself does no projection or content mutation, so
// it remains pure under the [tea.Model] value-receiver convention.
func (m Model) View() string {
	return m.state.transcript.View()
}

// ViewportState is the read-only snapshot of the shell's transcript
// viewport state. It reflects the scroll position the user would see
// at this point in the Bubble Tea update loop.
type ViewportState struct {
	// YOffset is the current top-line offset into the rendered content.
	// Zero when scrolled all the way to the top.
	YOffset int
	// AtBottom reports whether the viewport is currently following tail
	// (additional content would auto-scroll into view).
	AtBottom bool
	// Width and Height are the viewport's current dimensions.
	Width  int
	Height int
}

// ViewportState returns a snapshot of the transcript viewport's current
// scroll state. Reads from the persistent viewport that [refresh]
// mutates each [Model.Update] tick.
func (m Model) ViewportState() ViewportState {
	vp := m.state.transcript
	return ViewportState{
		YOffset:  vp.YOffset,
		AtBottom: vp.AtBottom(),
		Width:    vp.Width,
		Height:   vp.Height,
	}
}

// refresh re-projects shell state into [RenderInput], calls [Render],
// and applies the resulting content to the persistent transcript
// viewport with FEAT-0014 SC3 followTail semantics: if the user was
// at the bottom (or no content existed), the viewport auto-scrolls
// to the new bottom; otherwise the YOffset is preserved.
//
// refresh is called at the end of [Model.Update] so every state
// change (transcript items, queue state, permission state, mouse
// scroll, window resize) lands in the viewport before the next
// [Model.View] call.
func (s *state) refresh() {
	in := stateToRenderInput(s)
	result := Render(in)

	// Save followTail and YOffset before SetContent so the
	// invariants survive content size changes.
	followTail := s.transcript.AtBottom() || s.transcript.TotalLineCount() == 0
	savedOffset := s.transcript.YOffset

	s.transcript.SetContent(result.Content)
	s.transcriptRefs = nil
	for _, ref := range result.TranscriptRefs {
		s.transcriptRefs = append(s.transcriptRefs, TranscriptRef(ref))
	}

	if followTail {
		s.transcript.GotoBottom()
	} else {
		s.transcript.SetYOffset(savedOffset)
	}
}

// stateToRenderInput is the package-level analogue of [Model.toRenderInput]
// used by [refresh] (which operates on the state pointer). Both call into
// the same projection logic; the Model-receiver wrapper exists for
// callers that already have a Model in hand.
func stateToRenderInput(s *state) RenderInput {
	return Model{state: *s}.toRenderInput()
}

// toRenderInput projects shell-owned state into the [RenderInput] consumed
// by [Render]. Per the WU-100 Stage A→B bridge note, this projection
// replaces the spike's `toShellRenderInput` once the spike-shim cutover
// lands.
func (m Model) toRenderInput() RenderInput {
	in := RenderInput{
		Width:                 m.state.transcript.Width,
		Title:                 m.state.title,
		ModelLabel:            m.state.label,
		InputView:             m.state.input.View(),
		SelectedToken:         m.state.selectedToken,
		SelectedTranscriptRef: m.state.selectedTranscriptRef,
		Focus:                 toRenderFocus(m.state.focus),
		Streaming:             m.state.streaming,
		StreamPulse:           m.state.streamPulse,
		InterruptArmed:        m.state.interruptArmed,
		QueuedCount:           len(m.state.queuedSubmissions),
		Status:                composeStatusWithElapsed(&m.state),
		StatusKind:            m.state.statusKind,
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

// composeStatusWithElapsed appends "(Ns)" to the status when a run is
// streaming and runStartedAt is set, so the user sees live elapsed
// time alongside the static "Streaming response" / "Working" text.
// PATCH-0035 v0.3.0 placeholder; FEAT-0024 will replace this with a
// proper structured streaming-status surface.
func composeStatusWithElapsed(s *state) string {
	if !s.streaming || s.runStartedAt.IsZero() {
		return s.status
	}
	elapsed := s.nowOrDefault().Sub(s.runStartedAt).Round(time.Second)
	if elapsed <= 0 {
		return s.status
	}
	secs := int(elapsed.Seconds())
	if s.status == "" {
		return fmt.Sprintf("(%ds)", secs)
	}
	return fmt.Sprintf("%s (%ds)", s.status, secs)
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
