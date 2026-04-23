package harness

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/jasonahenderson/modeltap/internal/harness/theme"
)

// StatusBar renders the bottom status line per Bundle 5 design D3:
// connection indicator, mode, model, context%, cost, optional call
// timer — left-to-right.
type StatusBar struct {
	state   *AppState
	width   int
	style   StatusBarStyle
	spinner spinner.Model
}

// StatusBarStyle holds the lipgloss styles each status segment uses.
// Defaults are produced by DefaultStatusBarStyle; tests pass plain
// styles to avoid ANSI noise in expected-string assertions.
type StatusBarStyle struct {
	ConnReady        lipgloss.Style
	ConnDegraded     lipgloss.Style
	ConnReconnecting lipgloss.Style
	ConnFailed       lipgloss.Style
	Mode             lipgloss.Style
	Model            lipgloss.Style
	Context          lipgloss.Style
	ContextWarning   lipgloss.Style
	ContextCritical  lipgloss.Style
	Cost             lipgloss.Style
	Timer            lipgloss.Style
	Separator        lipgloss.Style
	Spinner          lipgloss.Style
}

// DefaultStatusBarStyle returns ANSI-coloured styles for an
// interactive terminal. Tests use a plain (zero-value) style to keep
// View output deterministic.
func DefaultStatusBarStyle() StatusBarStyle {
	return StatusBarStyle{
		ConnReady:        lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // green
		ConnDegraded:     lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // yellow
		ConnReconnecting: lipgloss.NewStyle(),
		ConnFailed:       lipgloss.NewStyle().Foreground(lipgloss.Color("9")), // red
		Mode:             lipgloss.NewStyle().Bold(true),
		Model:            lipgloss.NewStyle(),
		Context:          lipgloss.NewStyle(),
		ContextWarning:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		ContextCritical:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		Cost:             lipgloss.NewStyle(),
		Timer:            lipgloss.NewStyle().Faint(true),
		Separator:        lipgloss.NewStyle().Faint(true),
		Spinner:          lipgloss.NewStyle(),
	}
}

// ThemedStatusBarStyle builds styles from the active theme.
func ThemedStatusBarStyle(t theme.Theme) StatusBarStyle {
	if t == nil {
		return DefaultStatusBarStyle()
	}
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("·"))
	return StatusBarStyle{
		ConnReady:        lipgloss.NewStyle().Foreground(t.Success()),
		ConnDegraded:     lipgloss.NewStyle().Foreground(t.Warning()),
		ConnReconnecting: lipgloss.NewStyle().Foreground(t.Info()),
		ConnFailed:       lipgloss.NewStyle().Foreground(t.Error()),
		Mode:             lipgloss.NewStyle().Bold(true).Foreground(t.Accent()).Background(t.BackgroundPanel()),
		Model:            lipgloss.NewStyle().Foreground(t.Text()),
		Context:          lipgloss.NewStyle().Foreground(t.TextMuted()),
		ContextWarning:   lipgloss.NewStyle().Foreground(t.Warning()),
		ContextCritical:  lipgloss.NewStyle().Foreground(t.Error()),
		Cost:             lipgloss.NewStyle().Foreground(t.TextMuted()),
		Timer:            lipgloss.NewStyle().Foreground(t.TextMuted()).Faint(true),
		Separator:        sep,
		Spinner:          lipgloss.NewStyle().Foreground(t.Accent()),
	}
}

// NewStatusBar constructs a StatusBar with default styles bound to
// shared state.
func NewStatusBar(state *AppState) StatusBar {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return StatusBar{state: state, style: DefaultStatusBarStyle(), spinner: s}
}

// SetStyle overrides the StatusBarStyle (used by tests for plain
// rendering and by themed UIs in the future).
func (s *StatusBar) SetStyle(style StatusBarStyle) { s.style = style }

// SetTheme rebuilds styles from the given theme.
func (s *StatusBar) SetTheme(t theme.Theme) {
	s.style = ThemedStatusBarStyle(t)
}

// SetSpinnerStyle updates the spinner style.
func (s *StatusBar) SetSpinnerStyle(st lipgloss.Style) {
	s.spinner.Style = st
}

// SetWidth informs the bar of the available terminal width.
func (s *StatusBar) SetWidth(w int) { s.width = w }

// Width returns the current width.
func (s *StatusBar) Width() int { return s.width }

// Height is always 1.
func (s *StatusBar) Height() int { return 1 }

// View renders the bar. Returns "" when the bound state is nil so the
// stub-construction path doesn't panic.
func (s *StatusBar) View() string {
	if s.state == nil {
		return ""
	}
	parts := []string{
		s.connectionIndicator(),
		s.modeDisplay(),
		s.modelDisplay(),
	}
	parts = append(parts, s.sep(), s.contextDisplay(), s.sep(), s.costDisplay())
	if t := s.timerDisplay(); t != "" {
		parts = append(parts, s.sep(), t)
	}
	if s.state.CallActive {
		parts = append(parts, s.sep(), s.spinner.View())
	}
	return strings.Join(parts, " ")
}

// Tick advances the spinner frame. Returns a tea.Cmd suitable for
// the app's Update loop.
func (s *StatusBar) Tick() {
	_ = s.spinner.Tick
}

func (s *StatusBar) sep() string {
	return s.style.Separator.Render("·")
}

// connectionIndicator renders a colored badge per FEAT-0009.
func (s *StatusBar) connectionIndicator() string {
	switch s.state.ConnState.State {
	case ConnStateReady:
		return s.style.ConnReady.Render("[●]")
	case ConnStateDegraded:
		return s.style.ConnDegraded.Render("[◐]")
	case ConnStateReconnecting:
		return s.style.ConnReconnecting.Render("[↻]")
	case ConnStateFailed:
		return s.style.ConnFailed.Render("[✗]")
	case ConnStateDiscovering, ConnStateStarting, ConnStateConnecting,
		ConnStateAuthenticating, ConnStateRegistering:
		return s.style.ConnReconnecting.Render("[…]")
	default:
		return "[?]"
	}
}

func (s *StatusBar) modeDisplay() string {
	mode := string(s.state.Mode)
	if mode == "" {
		mode = "build"
	}
	return s.style.Mode.Render("[" + mode + "]")
}

func (s *StatusBar) modelDisplay() string {
	name := s.state.ModelName
	if name == "" {
		name = "(no model)"
	}
	if s.state.ModelOverride {
		name += "*"
	}
	return s.style.Model.Render(name)
}

func (s *StatusBar) contextDisplay() string {
	pct := s.state.ContextPct * 100
	used := formatTokens(s.state.ContextUsed)
	max := formatTokens(s.state.ContextMax)
	text := fmt.Sprintf("%.0f%% context (%s/%s)", pct, used, max)
	switch {
	case pct >= 92:
		return s.style.ContextCritical.Render(text)
	case pct >= 78:
		return s.style.ContextWarning.Render(text)
	default:
		return s.style.Context.Render(text)
	}
}

func (s *StatusBar) costDisplay() string {
	return s.style.Cost.Render(fmt.Sprintf("$%.4f", s.state.SessionCost))
}

func (s *StatusBar) timerDisplay() string {
	if !s.state.CallActive || s.state.CallStartTime.IsZero() {
		return ""
	}
	elapsed := time.Since(s.state.CallStartTime)
	return s.style.Timer.Render(fmt.Sprintf("%.1fs", elapsed.Seconds()))
}

// formatTokens turns a raw count into a human-friendly short form.
//
//	1234   →  "1.2K"
//	12345  →  "12K"
//	123456 →  "123K"
//	1234567 → "1.2M"
//
// Zero collapses to "0" rather than "0K".
func formatTokens(n int) string {
	switch {
	case n == 0:
		return "0"
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 10_000:
		return fmt.Sprintf("%.1fK", float64(n)/1000.0)
	case n < 1_000_000:
		return fmt.Sprintf("%dK", n/1000)
	default:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000.0)
	}
}
