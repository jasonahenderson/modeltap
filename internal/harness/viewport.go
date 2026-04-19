package harness

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ConversationViewport is the scrollable conversation area above the
// input. Stub for WU-068; WU-071 wraps a bubbles/viewport.Model and
// adds auto-scroll, snap-back, and metadata chrome around each
// DisplayMessage.
type ConversationViewport struct {
	state  *AppState
	width  int
	height int
	atBot  bool
}

// NewConversationViewport returns an empty viewport bound to shared
// state.
func NewConversationViewport(state *AppState) ConversationViewport {
	return ConversationViewport{state: state, atBot: true}
}

// SetSize informs the viewport of the available rectangle.
func (v *ConversationViewport) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// AtBottom reports whether the scroll position is at the latest
// message — used by the App to decide whether arrow-down should
// transfer focus back to the input.
func (v *ConversationViewport) AtBottom() bool { return v.atBot }

// Update is the Bubbletea update entrypoint. The placeholder routes
// scroll keys but does not yet manage a buffer.
func (v ConversationViewport) Update(msg tea.Msg) (ConversationViewport, tea.Cmd) {
	return v, nil
}

// View renders the visible window. The WU-068 placeholder concatenates
// each message's Content joined by blank lines; WU-071/072 replace this
// with a real scroll-aware viewport plus Glamour rendering.
func (v *ConversationViewport) View() string {
	if v.state == nil || len(v.state.Messages) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, m := range v.state.Messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(m.Content)
	}
	return sb.String()
}
