package harnessshell

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Stage C wire-up smoke tests. WU-102 owns the parity/regression test sweep
// per the v0.2.1 plan; the tests here only assert that the public Model API
// plumbs into the shell-state helpers correctly. They are not a substitute
// for the parity coverage WU-102 will land.

func TestNewDefaults(t *testing.T) {
	m := New(WithLabel("test-model"), WithPlaceholder("Type here"))

	if m.state.focus != FocusInput {
		t.Fatalf("focus = %v, want FocusInput", m.state.focus)
	}
	if m.state.label != "test-model" {
		t.Fatalf("label = %q, want %q", m.state.label, "test-model")
	}
	if m.state.input.Placeholder != "Type here" {
		t.Fatalf("placeholder = %q, want %q", m.state.input.Placeholder, "Type here")
	}
	if !m.state.input.Focused() {
		t.Fatalf("input should be focused after New")
	}
	if m.state.historyIndex != -1 {
		t.Fatalf("historyIndex = %d, want -1", m.state.historyIndex)
	}
	if m.state.statusKind != StatusReady {
		t.Fatalf("statusKind = %v, want StatusReady", m.state.statusKind)
	}
}

func TestInitReturnsBlinkCmd(t *testing.T) {
	m := New()
	if cmd := m.Init(); cmd == nil {
		t.Fatalf("Init returned nil; expected textarea.Blink cmd")
	}
}

func TestUpdateWindowSizePropagates(t *testing.T) {
	m := New()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	mu := updated.(Model)

	if mu.state.width != 100 || mu.state.height != 30 {
		t.Fatalf("size = (%d,%d), want (100,30)", mu.state.width, mu.state.height)
	}
	if mu.state.transcript.Width != 100 || mu.state.transcript.Height != 30 {
		t.Fatalf("transcript size = (%d,%d), want (100,30)", mu.state.transcript.Width, mu.state.transcript.Height)
	}
	// textarea.SetWidth(n) reserves space for the prompt internally, so
	// .Width() reports a value strictly less than the requested width.
	// We assert the textarea was sized into the composer envelope rather
	// than asserting an exact internal arithmetic.
	if w := mu.state.input.Width(); w <= 0 || w >= 100 {
		t.Fatalf("input width = %d, want 0 < w < 100", w)
	}
}

func TestEmptyShellViewShowsWelcomeBlock(t *testing.T) {
	m := New(WithTitle("modeltap"), WithLabel("test-model"), WithPlaceholder("Type here"))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mu := updated.(Model)

	view := mu.View()
	if !strings.Contains(view, "modeltap") {
		t.Fatalf("empty shell view should include product title; view = %q", view)
	}
	if !strings.Contains(view, "Conversation shell") {
		t.Fatalf("empty shell view should include welcome subtitle; view = %q", view)
	}
	if !strings.Contains(view, "test-model") {
		t.Fatalf("empty shell view should include current label; view = %q", view)
	}
}

func TestTabCyclesFocusSidebarClosed(t *testing.T) {
	m := New()
	if m.state.focus != FocusInput {
		t.Fatalf("starting focus = %v, want FocusInput", m.state.focus)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m1 := updated.(Model)
	if m1.state.focus != FocusTranscript {
		t.Fatalf("after first Tab focus = %v, want FocusTranscript", m1.state.focus)
	}
	if m1.state.input.Focused() {
		t.Fatalf("input should be blurred when focus moves off composer")
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated.(Model)
	if m2.state.focus != FocusInput {
		t.Fatalf("after second Tab focus = %v, want FocusInput", m2.state.focus)
	}
	if !m2.state.input.Focused() {
		t.Fatalf("input should refocus when focus returns to composer")
	}
}

func TestSingleLineUpRecallsHistory(t *testing.T) {
	m := New()
	m.state.commandHistory = []string{"first", "second"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m1 := updated.(Model)
	if got := m1.state.input.Value(); got != "second" {
		t.Fatalf("after Up input = %q, want %q", got, "second")
	}

	updated, _ = m1.Update(tea.KeyMsg{Type: tea.KeyUp})
	m2 := updated.(Model)
	if got := m2.state.input.Value(); got != "first" {
		t.Fatalf("after second Up input = %q, want %q", got, "first")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyDown})
	m3 := updated.(Model)
	if got := m3.state.input.Value(); got != "second" {
		t.Fatalf("after Down input = %q, want %q", got, "second")
	}
}

func TestMultilineUpDoesNotTriggerHistory(t *testing.T) {
	m := New()
	m.state.commandHistory = []string{"older"}
	m.state.input.SetValue("line one\nline two")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	mu := updated.(Model)
	if got := mu.state.input.Value(); !strings.Contains(got, "line one") {
		t.Fatalf("multi-line Up should not invoke history recall; input = %q", got)
	}
}

func TestActionDrainEmitsActionMsg(t *testing.T) {
	m := New()
	expected := SubmitTurnAction{Submission: Submission{ID: "sub-1", Text: "hello"}}
	m.state.pendingActions = []Action{expected}

	// Drive an irrelevant message through Update so the drain runs at the
	// end of the tick. tea.WindowSizeMsg{} works as a benign trigger.
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mu := updated.(Model)
	if len(mu.state.pendingActions) != 0 {
		t.Fatalf("pendingActions should be drained; got %d", len(mu.state.pendingActions))
	}
	if cmd == nil {
		t.Fatalf("Update returned nil cmd; expected drain to emit ActionMsg")
	}

	msg := cmd()
	got, ok := collectActionMsg(msg)
	if !ok {
		t.Fatalf("drain msg was not an ActionMsg or batch of them: %T", msg)
	}
	if len(got) != 1 {
		t.Fatalf("got %d ActionMsgs, want 1", len(got))
	}
	sub, ok := got[0].Action.(SubmitTurnAction)
	if !ok {
		t.Fatalf("ActionMsg.Action = %T, want SubmitTurnAction", got[0].Action)
	}
	if sub.Submission.ID != "sub-1" || sub.Submission.Text != "hello" {
		t.Fatalf("submission round-trip mismatch: %+v", sub)
	}
}

// collectActionMsg unwraps a tea.Msg into its constituent ActionMsg values.
// Cmds returned by tea.Batch resolve into a tea.BatchMsg which is itself a
// slice of cmds; for single-cmd cases the msg is already an ActionMsg.
func collectActionMsg(msg tea.Msg) ([]ActionMsg, bool) {
	switch m := msg.(type) {
	case ActionMsg:
		return []ActionMsg{m}, true
	case tea.BatchMsg:
		var out []ActionMsg
		for _, c := range m {
			if c == nil {
				continue
			}
			sub, ok := collectActionMsg(c())
			if !ok {
				return nil, false
			}
			out = append(out, sub...)
		}
		return out, true
	default:
		return nil, false
	}
}
