package harness

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

func plainViewport(state *AppState) ConversationViewport {
	v := NewConversationViewport(state)
	zero := lipgloss.NewStyle()
	v.SetStyle(ViewportStyle{
		UserPrefix: zero, UserContent: zero,
		AssistantHead: zero, AssistantFoot: zero,
		ToolCall: zero, ToolResult: zero, System: zero,
	})
	return v
}

func TestViewport_RenderUserMessage(t *testing.T) {
	state := NewAppState()
	state.Messages = []DisplayMessage{
		{Role: RoleUser, Content: "hello"},
	}
	v := plainViewport(state)
	v.SetSize(80, 10)
	out := v.View()
	if !strings.Contains(out, "> hello") {
		t.Errorf("user message not rendered:\n%s", out)
	}
}

func TestViewport_RenderAssistant_HeaderAndFooter(t *testing.T) {
	state := NewAppState()
	state.Messages = []DisplayMessage{
		{
			Role: RoleAssistant, Content: "Some answer.",
			Model: "claude-sonnet-4-6", Routing: "coding",
			Tokens: TokenInfo{Input: 100, Output: 200},
			Cost:   0.05, Duration: 4 * time.Second,
		},
	}
	v := plainViewport(state)
	v.SetSize(80, 10)
	out := v.View()
	if !strings.Contains(out, "claude-sonnet-4-6") || !strings.Contains(out, "routing: coding") {
		t.Errorf("missing assistant header:\n%s", out)
	}
	if !strings.Contains(out, "100 in / 200 out") || !strings.Contains(out, "$0.0500") {
		t.Errorf("missing assistant footer:\n%s", out)
	}
}

func TestViewport_RenderAssistant_StreamingHidesFooter(t *testing.T) {
	state := NewAppState()
	state.Messages = []DisplayMessage{
		{
			Role: RoleAssistant, Content: "thinking...",
			Model: "claude-sonnet-4-6",
			Tokens: TokenInfo{Input: 50}, Cost: 0.01,
			Streaming: true,
		},
	}
	v := plainViewport(state)
	v.SetSize(80, 10)
	out := v.View()
	if strings.Contains(out, "$0.0100") {
		t.Errorf("footer rendered while streaming")
	}
}

func TestViewport_RenderToolCallAndResult(t *testing.T) {
	state := NewAppState()
	state.Messages = []DisplayMessage{
		{Role: RoleToolCall, Content: `Read(path="/x")`},
		{Role: RoleToolResult, Content: "Read: 42 lines"},
	}
	v := plainViewport(state)
	v.SetSize(80, 10)
	out := v.View()
	if !strings.Contains(out, "⚙ Read") {
		t.Errorf("tool call missing:\n%s", out)
	}
	if !strings.Contains(out, "✓ Read") {
		t.Errorf("tool result missing:\n%s", out)
	}
}

func TestViewport_AutoScroll_DefaultEnabled(t *testing.T) {
	state := NewAppState()
	v := plainViewport(state)
	v.SetSize(80, 10)
	if !v.AutoScrollEnabled() {
		t.Errorf("auto-scroll should be enabled by default")
	}
}

func TestViewport_AutoScroll_DisabledOnArrowUp(t *testing.T) {
	state := NewAppState()
	for i := 0; i < 50; i++ {
		state.Messages = append(state.Messages, DisplayMessage{Role: RoleUser, Content: "msg"})
	}
	v := plainViewport(state)
	v.SetSize(80, 5)

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyUp})
	v = updated
	if v.AutoScrollEnabled() {
		t.Errorf("auto-scroll should be disabled after arrow up")
	}
}

func TestViewport_View_EmptyMessages(t *testing.T) {
	state := NewAppState()
	v := plainViewport(state)
	v.SetSize(80, 10)
	if got := v.View(); strings.TrimSpace(got) != "" {
		t.Errorf("empty state should render empty viewport, got %q", got)
	}
}
