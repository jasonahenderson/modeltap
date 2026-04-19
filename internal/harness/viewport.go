package harness

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConversationViewport wraps bubbles/viewport with auto-scroll, manual-
// scroll detection, and per-message rendering chrome (header line for
// the assistant, role-specific glyphs, footer with metrics) per Bundle
// 5 design D5.
type ConversationViewport struct {
	state    *AppState
	vp       viewport.Model
	renderer *MarkdownRenderer
	style    ViewportStyle

	width  int
	height int

	autoScroll   bool
	userScrolled bool
}

// ViewportStyle holds the lipgloss styles for each message section.
type ViewportStyle struct {
	UserPrefix    lipgloss.Style
	UserContent   lipgloss.Style
	AssistantHead lipgloss.Style
	AssistantFoot lipgloss.Style
	ToolCall      lipgloss.Style
	ToolResult    lipgloss.Style
	System        lipgloss.Style
}

// DefaultViewportStyle returns coloured styles for an interactive
// terminal. Tests use a plain (zero-value) override.
func DefaultViewportStyle() ViewportStyle {
	return ViewportStyle{
		UserPrefix:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")), // blue
		UserContent:   lipgloss.NewStyle(),
		AssistantHead: lipgloss.NewStyle().Faint(true),
		AssistantFoot: lipgloss.NewStyle().Faint(true),
		ToolCall:      lipgloss.NewStyle().Foreground(lipgloss.Color("13")),  // magenta
		ToolResult:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")),  // green
		System:        lipgloss.NewStyle().Faint(true),
	}
}

// NewConversationViewport constructs a viewport bound to shared state.
// The markdown renderer is created lazily on the first SetSize so we
// know the width.
func NewConversationViewport(state *AppState) ConversationViewport {
	return ConversationViewport{
		state:      state,
		vp:         viewport.New(0, 0),
		style:      DefaultViewportStyle(),
		autoScroll: true,
	}
}

// SetStyle overrides the rendering styles.
func (v *ConversationViewport) SetStyle(style ViewportStyle) { v.style = style }

// SetSize informs the viewport of the available rectangle and creates
// (or resizes) the markdown renderer to match.
func (v *ConversationViewport) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.vp.Width = width
	v.vp.Height = height
	if v.renderer == nil {
		if r, err := NewMarkdownRenderer(width); err == nil {
			v.renderer = r
		}
	} else {
		_ = v.renderer.SetWidth(width)
	}
	v.refreshContent()
}

// AtBottom reports whether the underlying viewport is scrolled to the
// last line — used by the App to decide focus transitions.
func (v *ConversationViewport) AtBottom() bool { return v.vp.AtBottom() }

// Update handles scroll keys and tracks user-initiated scrolling so
// auto-scroll can be re-enabled when the user scrolls back to bottom.
func (v ConversationViewport) Update(msg tea.Msg) (ConversationViewport, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		// Manual scroll-up disables auto-scroll until the user
		// re-reaches the bottom.
		switch {
		case key.Matches(k, key.NewBinding(key.WithKeys("up", "pgup", "k"))):
			v.autoScroll = false
			v.userScrolled = true
		case key.Matches(k, key.NewBinding(key.WithKeys("down", "pgdown", "j"))):
			// Snap-back below.
		}
	}
	updated, cmd := v.vp.Update(msg)
	v.vp = updated
	if v.vp.AtBottom() {
		v.autoScroll = true
		v.userScrolled = false
	}
	return v, cmd
}

// View renders the visible window.
func (v *ConversationViewport) View() string {
	v.refreshContent()
	return v.vp.View()
}

// refreshContent rebuilds the viewport's body text from the current
// AppState messages and applies auto-scroll if enabled.
func (v *ConversationViewport) refreshContent() {
	if v.state == nil {
		return
	}
	body := v.renderMessages()
	v.vp.SetContent(body)
	if v.autoScroll && !v.userScrolled {
		v.vp.GotoBottom()
	}
}

func (v *ConversationViewport) renderMessages() string {
	if v.state == nil || len(v.state.Messages) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, m := range v.state.Messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(v.renderMessage(m))
	}
	return sb.String()
}

func (v *ConversationViewport) renderMessage(m DisplayMessage) string {
	switch m.Role {
	case RoleUser:
		return v.style.UserPrefix.Render("> ") + v.style.UserContent.Render(m.Content)
	case RoleAssistant:
		return v.renderAssistant(m)
	case RoleToolCall:
		return v.style.ToolCall.Render("⚙ " + m.Content)
	case RoleToolResult:
		return v.style.ToolResult.Render("✓ " + m.Content)
	case RoleSystem:
		return v.style.System.Render(m.Content)
	default:
		return m.Content
	}
}

// renderAssistant produces the model-routing header, the rendered
// markdown body, and the per-turn footer with token / cost / latency
// counters once streaming completes.
func (v *ConversationViewport) renderAssistant(m DisplayMessage) string {
	var sb strings.Builder

	// Header: model name + routing reason.
	if m.Model != "" {
		head := "→ " + m.Model
		if m.Routing != "" {
			head += " (routing: " + m.Routing + ")"
		}
		if m.Override {
			head += " *"
		}
		sb.WriteString(v.style.AssistantHead.Render(head))
		sb.WriteString("\n\n")
	}

	// Body: streaming-tolerant render or final clean render.
	body := m.Content
	if v.renderer != nil {
		var rendered string
		var err error
		if m.Streaming {
			rendered, err = v.renderer.RenderStreaming(body)
		} else {
			rendered, err = v.renderer.Render(body)
		}
		if err == nil && rendered != "" {
			body = rendered
		}
	}
	sb.WriteString(body)

	// Footer: only when streaming is complete and metrics are populated.
	if !m.Streaming && (m.Tokens.Input > 0 || m.Tokens.Output > 0 || m.Cost > 0) {
		footer := fmt.Sprintf("--- %s | %d in / %d out | $%.4f | %s ---",
			defaultStr(m.Model, "(unknown)"),
			m.Tokens.Input,
			m.Tokens.Output,
			m.Cost,
			m.Duration.Round(0),
		)
		sb.WriteString("\n\n")
		sb.WriteString(v.style.AssistantFoot.Render(footer))
	}
	return sb.String()
}

// AutoScrollEnabled exposes the current auto-scroll mode for tests.
func (v *ConversationViewport) AutoScrollEnabled() bool { return v.autoScroll }

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
