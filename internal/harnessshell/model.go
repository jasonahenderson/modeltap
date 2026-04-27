package harnessshell

import (
	tea "github.com/charmbracelet/bubbletea"
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
// Stage A introduces the type with zero-behavior Update and View methods so
// later stages can land rendering, action emission, and event intake against
// a stable target.
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
	m := Model{
		state: state{
			focus:                 FocusInput,
			historyIndex:          -1,
			activePermissionIndex: 0,
			selectedToken:         -1,
			selectedTranscriptRef: -1,
			statusKind:            StatusReady,
		},
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// Init returns the initial Bubble Tea command for the shell. Stage A returns
// nil; later stages will return textarea.Blink and any other initial commands.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles a Bubble Tea message and returns the next model and
// command. Stage A is a no-op: the message is ignored and the model is
// returned unchanged. Later stages route key input, host events, and queue
// the outbound action stream.
func (m Model) Update(_ tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// View renders the current shell. Stage A returns an empty string; rendering
// lands in Stage B.
func (m Model) View() string {
	return ""
}
