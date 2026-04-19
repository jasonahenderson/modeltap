package harness

// StatusBar is the bottom-of-screen status line. Stub for WU-068; the
// full rendering / formatting work lands in WU-069.
type StatusBar struct {
	state *AppState
	width int
}

// NewStatusBar constructs a StatusBar bound to the given shared state.
func NewStatusBar(state *AppState) StatusBar {
	return StatusBar{state: state}
}

// SetWidth informs the status bar of the current terminal width so it
// can budget segment space.
func (s *StatusBar) SetWidth(w int) { s.width = w }

// Width returns the current width.
func (s *StatusBar) Width() int { return s.width }

// Height is always 1 — the status bar is a single line by spec.
func (s *StatusBar) Height() int { return 1 }

// View renders the bar. WU-069 fleshes this out with real segments;
// the WU-068 placeholder shows the current state name only.
func (s *StatusBar) View() string {
	if s.state == nil {
		return ""
	}
	return s.state.ConnState.State
}
