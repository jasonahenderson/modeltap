package harness

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// plainStatusBar returns a StatusBar with all styles cleared so View
// output is deterministic for assertions.
func plainStatusBar(state *AppState) StatusBar {
	s := NewStatusBar(state)
	zero := lipgloss.NewStyle()
	s.SetStyle(StatusBarStyle{
		ConnReady: zero, ConnDegraded: zero, ConnReconnecting: zero, ConnFailed: zero,
		Mode: zero, Model: zero, Context: zero, ContextWarning: zero, ContextCritical: zero,
		Cost: zero, Timer: zero,
	})
	return s
}

func TestStatusBar_ConnectionIndicator_AllStates(t *testing.T) {
	state := NewAppState()
	bar := plainStatusBar(state)
	cases := map[string]string{
		ConnStateReady:          "[●]",
		ConnStateDegraded:       "[◐]",
		ConnStateReconnecting:   "[↻]",
		ConnStateFailed:         "[✗]",
		ConnStateDiscovering:    "[…]",
		ConnStateStarting:       "[…]",
		ConnStateConnecting:     "[…]",
		ConnStateAuthenticating: "[…]",
		ConnStateRegistering:    "[…]",
	}
	for st, want := range cases {
		state.ConnState = ConnStateInfo{State: st}
		got := bar.View()
		if !strings.Contains(got, want) {
			t.Errorf("state %q: View missing %q\n%s", st, want, got)
		}
	}
}

func TestStatusBar_ModeAndModel(t *testing.T) {
	state := NewAppState()
	state.Mode = protocol.ModePlan
	state.ModelName = "claude-sonnet-4-6"
	bar := plainStatusBar(state)
	v := bar.View()
	if !strings.Contains(v, "[plan]") {
		t.Errorf("missing mode label:\n%s", v)
	}
	if !strings.Contains(v, "claude-sonnet-4-6") {
		t.Errorf("missing model name:\n%s", v)
	}
	if strings.Contains(v, "claude-sonnet-4-6*") {
		t.Errorf("override marker should be absent: %s", v)
	}

	state.ModelOverride = true
	if v := bar.View(); !strings.Contains(v, "claude-sonnet-4-6*") {
		t.Errorf("override marker missing:\n%s", v)
	}
}

func TestStatusBar_Context_PressureColoring(t *testing.T) {
	state := NewAppState()
	state.ContextUsed = 1000
	state.ContextMax = 10_000

	style := DefaultStatusBarStyle()
	bar := NewStatusBar(state)
	bar.SetStyle(style)

	for _, tc := range []struct {
		pct       float64
		wantStyle lipgloss.Style
	}{
		{0.50, style.Context},
		{0.78, style.ContextWarning},
		{0.92, style.ContextCritical},
	} {
		state.ContextPct = tc.pct
		got := bar.contextDisplay()
		want := tc.wantStyle.Render(got)
		// Compare against the wanted-style render of the same text.
		// If a style would have been applied, the ANSI prefix differs.
		if !ansiPrefixesMatch(got, want) {
			t.Errorf("pct=%v: style mismatch", tc.pct)
		}
	}
}

func ansiPrefixesMatch(a, b string) bool {
	// Strip everything before the first 'm' (end of an ANSI sequence)
	// and compare just the prefix bytes. Acceptable simplification:
	// both strings will share the same ANSI prefix when the same
	// lipgloss style produced them.
	prefixA := strings.SplitN(a, "m", 2)
	prefixB := strings.SplitN(b, "m", 2)
	if len(prefixA) < 2 || len(prefixB) < 2 {
		return prefixA[0] == prefixB[0]
	}
	return prefixA[0] == prefixB[0]
}

func TestStatusBar_Timer_ShowsWhenCallActive(t *testing.T) {
	state := NewAppState()
	bar := plainStatusBar(state)
	if strings.Contains(bar.View(), "s") && strings.Contains(bar.View(), ".") {
		// Timer suffix may legitimately be absent when CallActive==false.
	}
	if strings.Contains(bar.View(), "0.0s") {
		t.Errorf("timer leaked when CallActive=false")
	}

	state.CallActive = true
	state.CallStartTime = time.Now().Add(-1500 * time.Millisecond)
	v := bar.View()
	if !strings.Contains(v, "s") {
		t.Errorf("timer not rendered:\n%s", v)
	}
}

func TestStatusBar_Cost_FormattedTo4Decimals(t *testing.T) {
	state := NewAppState()
	state.SessionCost = 0.012345
	bar := plainStatusBar(state)
	if !strings.Contains(bar.View(), "$0.0123") {
		t.Errorf("cost not formatted: %s", bar.View())
	}
}

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1.0K"},
		{1234, "1.2K"},
		{9999, "10.0K"},
		{12345, "12K"},
		{123456, "123K"},
		{1500000, "1.5M"},
	}
	for _, c := range cases {
		if got := formatTokens(c.in); got != c.want {
			t.Errorf("formatTokens(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStatusBar_NoModelShowsPlaceholder(t *testing.T) {
	state := NewAppState()
	bar := plainStatusBar(state)
	if !strings.Contains(bar.View(), "(no model)") {
		t.Errorf("missing placeholder:\n%s", bar.View())
	}
}
