package harness

import (
	tea "github.com/charmbracelet/bubbletea"
)

// InputArea is the multi-line input above the status bar. Stub for
// WU-068; WU-070 fleshes out command parsing, @file detection, paste
// handling, and history keybindings.
type InputArea struct {
	state  *AppState
	width  int
	height int
	value  string
}

// NewInputArea constructs an empty input area bound to the shared
// state.
func NewInputArea(state *AppState) InputArea {
	return InputArea{state: state, height: 1}
}

// SetWidth informs the area of the available width.
func (i *InputArea) SetWidth(w int) { i.width = w }

// Height returns the current rendered height in rows. The App uses
// this to compute the viewport's height each layout pass.
func (i *InputArea) Height() int {
	if i.height < 1 {
		return 1
	}
	return i.height
}

// CursorAtTop reports whether the cursor is on the first line of the
// input — used by the App to decide whether arrow-up should switch
// focus to the viewport rather than scroll history.
func (i *InputArea) CursorAtTop() bool { return true }

// Value returns the raw input contents. WU-070 will replace the
// internal storage with a bubbles/textarea instance.
func (i *InputArea) Value() string { return i.value }

// SetValue replaces the contents (used by tests and by command
// completion in WU-070).
func (i *InputArea) SetValue(v string) { i.value = v }

// Update is the Bubbletea update entrypoint. Returns the new state and
// any commands to dispatch. The WU-068 placeholder simply records key
// runes into Value so the App's Update can produce a SubmitMsg from a
// captured value when the user hits the submit chord.
func (i InputArea) Update(msg tea.Msg) (InputArea, tea.Cmd) {
	return i, nil
}

// View renders the input area. WU-070 will produce the real bordered
// textarea; the WU-068 placeholder shows the current Value.
func (i *InputArea) View() string {
	if i.value == "" {
		return "> "
	}
	return "> " + i.value
}
