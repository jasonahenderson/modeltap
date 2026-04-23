package harness

import "github.com/charmbracelet/bubbles/key"

// SubmitKeyDefault is the default submit-key configuration.
const (
	SubmitKeyCtrlEnter = "ctrl+enter"
	SubmitKeyEnter     = "enter"
	SubmitKeyEscEnter  = "esc-enter"
)

// KeyMap defines the App's global key bindings. Component-local
// bindings (textarea cursor movement, viewport scroll, etc.) are
// handled within each component.
type KeyMap struct {
	Quit        key.Binding
	Submit      key.Binding
	ToggleMode  key.Binding
	ScrollUp    key.Binding
	ScrollDown  key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	HistoryUp   key.Binding
	HistoryDown key.Binding
}

// DefaultKeyMap returns the canonical KeyMap. submitKey selects the
// configured submit chord; unknown values fall back to enter.
func DefaultKeyMap(submitKey string) KeyMap {
	if submitKey == "" {
		submitKey = SubmitKeyEnter
	}
	var submitBinding key.Binding
	switch submitKey {
	case SubmitKeyEnter:
		submitBinding = key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "submit"),
		)
	case SubmitKeyEscEnter:
		submitBinding = key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc enter", "submit"),
		)
	default:
		submitBinding = key.NewBinding(
			key.WithKeys("ctrl+@"), // bubbletea reports Ctrl+Enter as "ctrl+@" on most terminals
			key.WithHelp("ctrl+enter", "submit"),
		)
	}
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Submit: submitBinding,
		ToggleMode: key.NewBinding(
			key.WithKeys("ctrl+p", "tab"),
			key.WithHelp("ctrl+p / tab", "toggle mode"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "scroll down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
		HistoryUp: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "history prev"),
		),
		HistoryDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "history next"),
		),
	}
}
