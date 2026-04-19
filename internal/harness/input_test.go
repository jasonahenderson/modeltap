package harness

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeHistory is a deterministic HistorySource for tests.
type fakeHistory struct {
	entries []string // entries[0] is the most recent
}

func (h *fakeHistory) Entry(i int) (string, bool) {
	if i < 0 || i >= len(h.entries) {
		return "", false
	}
	return h.entries[i], true
}
func (h *fakeHistory) Len() int { return len(h.entries) }

func typeRune(t *testing.T, ia InputArea, r rune) InputArea {
	t.Helper()
	updated, _ := ia.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated
}

func TestInput_TypeAndValue(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetWidth(40)
	ia = typeRune(t, ia, 'h')
	ia = typeRune(t, ia, 'i')
	if ia.Value() != "hi" {
		t.Errorf("Value = %q", ia.Value())
	}
}

func TestInput_Submit_ReturnsSubmitMsgAndResets(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetValue("hello world")
	cmd := ia.Submit()
	if cmd == nil {
		t.Fatalf("nil cmd")
	}
	msg := cmd().(SubmitMsg)
	if msg.Content != "hello world" || msg.IsCommand {
		t.Errorf("submit = %+v", msg)
	}
	if ia.Value() != "" {
		t.Errorf("not reset")
	}
}

func TestInput_Submit_ParsesCommand(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetValue("/model claude-haiku-4-5")
	msg := ia.Submit()().(SubmitMsg)
	if !msg.IsCommand || msg.Command != "model" || msg.CommandArgs != "claude-haiku-4-5" {
		t.Errorf("submit = %+v", msg)
	}
}

func TestInput_Submit_ExtractsFileRefs(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetValue("review @internal/bff/turn.go and @docs/features/0008-bff-server.md please")
	msg := ia.Submit()().(SubmitMsg)
	want := []string{"internal/bff/turn.go", "docs/features/0008-bff-server.md"}
	if !reflect.DeepEqual(msg.Attachments, want) {
		t.Errorf("attachments = %v, want %v", msg.Attachments, want)
	}
}

func TestInput_Submit_BlankReturnsNil(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetValue("   \n\t  ")
	if cmd := ia.Submit(); cmd != nil {
		t.Errorf("expected nil cmd for whitespace input")
	}
}

func TestInput_HistoryUp_TraversesEntries(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetHistorySource(&fakeHistory{entries: []string{"latest", "older", "oldest"}})
	ia.SetValue("draft")

	// Up at top → first entry.
	updated, cmd := ia.Update(tea.KeyMsg{Type: tea.KeyUp})
	ia = updated
	if cmd == nil {
		t.Fatalf("expected history cmd")
	}
	if _, ok := cmd().(historyAdvancedMsg); !ok {
		t.Fatalf("cmd type = %T", cmd())
	}
	if ia.Value() != "latest" {
		t.Errorf("Value after up = %q", ia.Value())
	}

	// Up again → second entry.
	updated, _ = ia.Update(tea.KeyMsg{Type: tea.KeyUp})
	ia = updated
	if ia.Value() != "older" {
		t.Errorf("Value after second up = %q", ia.Value())
	}

	// Down → back to latest.
	updated, _ = ia.Update(tea.KeyMsg{Type: tea.KeyDown})
	ia = updated
	if ia.Value() != "latest" {
		t.Errorf("Value after down = %q", ia.Value())
	}

	// Down again → restore draft.
	updated, _ = ia.Update(tea.KeyMsg{Type: tea.KeyDown})
	ia = updated
	if ia.Value() != "draft" {
		t.Errorf("Value after down to draft = %q", ia.Value())
	}
}

func TestInput_HistoryUp_NoSourceIsNoop(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetValue("draft")
	updated, cmd := ia.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		// Up without history may still be consumed by the textarea
		// (cursor up); just assert no history advance happened.
		if _, ok := cmd().(historyAdvancedMsg); ok {
			t.Errorf("history should not advance without source")
		}
	}
	_ = updated
}

func TestInput_PasteDetected(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetWidth(40)
	ia.SetPasteThreshold(10)

	// Simulate a "paste" by sending a single KeyRunes message larger
	// than the threshold. bubbletea's textarea processes Runes as a
	// single insertion.
	pasted := strings.Repeat("x", 50)
	updated, cmd := ia.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(pasted)})
	ia = updated
	if cmd == nil {
		t.Fatalf("expected paste cmd")
	}
	// Drain commands looking for PasteDetectedMsg.
	found := false
	if pm, ok := cmd().(PasteDetectedMsg); ok {
		found = true
		if pm.ByteSize < 50 {
			t.Errorf("ByteSize = %d", pm.ByteSize)
		}
	} else if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, c := range batch {
			if pm, ok := c().(PasteDetectedMsg); ok {
				found = true
				if pm.ByteSize < 50 {
					t.Errorf("ByteSize = %d", pm.ByteSize)
				}
			}
		}
	}
	if !found {
		t.Errorf("PasteDetectedMsg not produced")
	}
}

func TestExtractFileRefs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"no refs here", nil},
		{"@a.txt", []string{"a.txt"}},
		{"hello @file.go world", []string{"file.go"}},
		{"@a @b @c", []string{"a", "b", "c"}},
		{"path: @/abs/path", []string{"/abs/path"}},
		{"glob: @*.md", []string{"*.md"}},
		{"email: user@example.com", nil}, // no leading whitespace before @
	}
	for _, c := range cases {
		got := ExtractFileRefs(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ExtractFileRefs(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDetectDragDrop(t *testing.T) {
	cases := []struct {
		in        string
		wantPaths []string
		wantOK    bool
	}{
		{"", nil, false},
		{"plain text", nil, false},
		{"/abs/path/file.txt", []string{"/abs/path/file.txt"}, true},
		{"~/notes/list.md", []string{"~/notes/list.md"}, true},
		{"/a /b /c", []string{"/a", "/b", "/c"}, true},
		{"/a\n/b\n/c", []string{"/a", "/b", "/c"}, true},
		{"/abs hello", nil, false},
	}
	for _, c := range cases {
		paths, ok := DetectDragDrop(c.in)
		if ok != c.wantOK {
			t.Errorf("DetectDragDrop(%q) ok = %v, want %v", c.in, ok, c.wantOK)
		}
		if !reflect.DeepEqual(paths, c.wantPaths) {
			t.Errorf("DetectDragDrop(%q) paths = %v, want %v", c.in, paths, c.wantPaths)
		}
	}
}

func TestInput_CursorAtTop_FirstLine(t *testing.T) {
	state := NewAppState()
	ia := NewInputArea(state)
	ia.SetValue("one")
	if !ia.CursorAtTop() {
		t.Errorf("single-line input should report cursor at top")
	}
}
