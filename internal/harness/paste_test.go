package harness

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func paste(t *testing.T, content string) PasteDetectedMsg {
	t.Helper()
	lines := strings.Split(content, "\n")
	preview := content
	if len(lines) > 5 {
		preview = strings.Join(lines[:5], "\n")
	}
	return PasteDetectedMsg{
		Content:   content,
		ByteSize:  len(content),
		LineCount: len(lines),
		Preview:   preview,
	}
}

func drainCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func clip(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func TestPasteHandler_HandlePaste_SetsActive(t *testing.T) {
	h := NewPasteHandler()
	msg := paste(t, strings.Repeat("x", 4096))
	cmd := h.HandlePaste(msg)
	if cmd == nil {
		t.Fatal("expected banner cmd")
	}
	b, ok := drainCmd(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, "[s]") || !strings.Contains(b.Text, "[f]") ||
		!strings.Contains(b.Text, "[t]") || !strings.Contains(b.Text, "[c]") {
		t.Errorf("banner should list choices; got %q", b.Text)
	}
	if b.Duration != 0 {
		t.Errorf("paste banner should be persistent; got %v", b.Duration)
	}
	if !h.Active() {
		t.Error("handler should be active after HandlePaste")
	}
}

func TestPasteHandler_ChooseFull(t *testing.T) {
	h := NewPasteHandler()
	content := strings.Repeat("data\n", 500)
	h.HandlePaste(paste(t, content))

	msg := drainCmd(h.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}))
	res, ok := msg.(PasteResolvedMsg)
	if !ok {
		t.Fatalf("expected PasteResolvedMsg, got %T", msg)
	}
	if res.Strategy != PasteStrategyFull {
		t.Errorf("Strategy = %q, want full", res.Strategy)
	}
	if res.Content != content {
		t.Errorf("full should preserve content; got len=%d want %d", len(res.Content), len(content))
	}
	if h.Active() {
		t.Error("handler should be inactive after resolution")
	}
}

func TestPasteHandler_ChooseTruncate(t *testing.T) {
	h := NewPasteHandler()
	h.SetTruncateLimit(32)
	content := strings.Repeat("A", 256)
	h.HandlePaste(paste(t, content))

	msg := drainCmd(h.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}))
	res, ok := msg.(PasteResolvedMsg)
	if !ok {
		t.Fatalf("expected PasteResolvedMsg, got %T", msg)
	}
	if res.Strategy != PasteStrategyTruncate {
		t.Errorf("Strategy = %q, want truncate", res.Strategy)
	}
	if len(res.Content) != 32 {
		t.Errorf("truncate length = %d, want 32", len(res.Content))
	}
	if h.Active() {
		t.Error("handler should be inactive after resolution")
	}
}

func TestPasteHandler_ChooseCancel(t *testing.T) {
	h := NewPasteHandler()
	h.HandlePaste(paste(t, "some paste"))

	msg := drainCmd(h.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}))
	res, ok := msg.(PasteResolvedMsg)
	if !ok {
		t.Fatalf("expected PasteResolvedMsg, got %T", msg)
	}
	if res.Strategy != PasteStrategyCancel {
		t.Errorf("Strategy = %q, want cancel", res.Strategy)
	}
	if res.Content != "" {
		t.Errorf("cancel should have empty content; got %q", res.Content)
	}
	if h.Active() {
		t.Error("handler should be inactive after cancel")
	}
}

func TestPasteHandler_ChooseSummarize_EmitsRequest(t *testing.T) {
	h := NewPasteHandler()
	content := "the big paste body"
	h.HandlePaste(paste(t, content))

	msg := drainCmd(h.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}))
	req, ok := msg.(PasteSummarizeRequestMsg)
	if !ok {
		t.Fatalf("expected PasteSummarizeRequestMsg, got %T", msg)
	}
	if req.Content != content {
		t.Errorf("summarize content = %q, want %q", req.Content, content)
	}
	// Handler stays active until a PasteResolvedMsg with
	// PasteStrategySummarize arrives (manager round-trips through BFF).
	if !h.Active() {
		t.Error("handler should remain active while summarize is in flight")
	}
}

func TestPasteHandler_Ignores_UnrelatedKey(t *testing.T) {
	h := NewPasteHandler()
	h.HandlePaste(paste(t, "content"))

	msg := drainCmd(h.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}))
	if msg != nil {
		t.Errorf("unrelated key should not resolve; got %T", msg)
	}
	if !h.Active() {
		t.Error("handler should stay active")
	}
}

func TestPasteHandler_EscCancels(t *testing.T) {
	h := NewPasteHandler()
	h.HandlePaste(paste(t, "content"))

	msg := drainCmd(h.HandleKey(tea.KeyMsg{Type: tea.KeyEsc}))
	res, ok := msg.(PasteResolvedMsg)
	if !ok {
		t.Fatalf("expected PasteResolvedMsg from Esc, got %T", msg)
	}
	if res.Strategy != PasteStrategyCancel {
		t.Errorf("Esc should cancel; got strategy %q", res.Strategy)
	}
}

func TestPasteHandler_SummarizeCompletion(t *testing.T) {
	// After summarize request, manager eventually sends back
	// PasteResolvedMsg via HandleSummaryReply — handler should clear.
	h := NewPasteHandler()
	h.HandlePaste(paste(t, "original"))
	_ = drainCmd(h.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}))
	if !h.Active() {
		t.Fatal("handler should still be active awaiting summary")
	}
	h.Complete() // connection manager calls this when summary arrives
	if h.Active() {
		t.Error("handler should be inactive after Complete")
	}
}

func TestApp_PasteFlow_TruncateReplacesInputContent(t *testing.T) {
	app := NewApp(AppOptions{})
	// Seed the input area with pre-paste text + the pasted content.
	pasted := strings.Repeat("Z", 3000)
	prefix := "keep this "
	app.input.SetValue(prefix + pasted)
	app.paste.SetTruncateLimit(100)

	// Step 1: App receives PasteDetectedMsg → should emit banner cmd.
	model, cmd := app.Update(PasteDetectedMsg{
		Content:   pasted,
		ByteSize:  len(pasted),
		LineCount: 1,
		Preview:   pasted[:50],
	})
	a, _ := model.(App)
	if !a.paste.Active() {
		t.Fatal("paste handler should be active after PasteDetectedMsg")
	}
	if cmd == nil {
		t.Fatal("expected banner cmd")
	}
	b, ok := cmd().(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, "[t]runcate") {
		t.Errorf("banner should list truncate option: %q", b.Text)
	}

	// Step 2: User presses 't'.
	model, cmd = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	a, _ = model.(App)
	if cmd == nil {
		t.Fatal("expected resolution cmd from 't'")
	}
	res, ok := cmd().(PasteResolvedMsg)
	if !ok {
		t.Fatalf("expected PasteResolvedMsg, got %T", cmd())
	}
	if res.Strategy != PasteStrategyTruncate {
		t.Errorf("Strategy = %q", res.Strategy)
	}

	// Step 3: App consumes the PasteResolvedMsg → replaces paste in input.
	model, _ = a.Update(res)
	a, _ = model.(App)
	got := a.input.Value()
	want := prefix + pasted[:100]
	if got != want {
		t.Errorf("input after truncate:\n got len=%d first 30=%q\nwant len=%d first 30=%q",
			len(got), clip(got, 30),
			len(want), clip(want, 30))
	}
	if a.paste.Active() {
		t.Error("paste handler should be inactive after resolution")
	}
}

func TestApp_PasteFlow_CancelRemovesPaste(t *testing.T) {
	app := NewApp(AppOptions{})
	pasted := strings.Repeat("q", 3000)
	prefix := "before "
	app.input.SetValue(prefix + pasted)

	model, _ := app.Update(PasteDetectedMsg{Content: pasted, ByteSize: len(pasted), LineCount: 1})
	a, _ := model.(App)

	// Esc cancels.
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	res, ok := cmd().(PasteResolvedMsg)
	if !ok {
		t.Fatalf("expected PasteResolvedMsg, got %T", cmd())
	}
	if res.Strategy != PasteStrategyCancel {
		t.Errorf("Strategy = %q, want cancel", res.Strategy)
	}

	model, _ = a.Update(res)
	a, _ = model.(App)
	if got := a.input.Value(); got != prefix {
		t.Errorf("input after cancel = %q, want %q", got, prefix)
	}
}

func TestApp_PasteFlow_IgnoresUnrelatedKeys(t *testing.T) {
	app := NewApp(AppOptions{})
	pasted := strings.Repeat("p", 3000)
	app.input.SetValue(pasted)

	model, _ := app.Update(PasteDetectedMsg{Content: pasted, ByteSize: len(pasted), LineCount: 1})
	a, _ := model.(App)

	before := a.input.Value()
	// Press a random letter — should be swallowed, input untouched.
	model, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	a, _ = model.(App)
	if cmd != nil {
		// Only acceptable if the cmd is not a PasteResolvedMsg.
		if _, ok := cmd().(PasteResolvedMsg); ok {
			t.Error("random key should not resolve paste")
		}
	}
	if a.input.Value() != before {
		t.Errorf("input should be untouched; got %q want %q", a.input.Value(), before)
	}
	if !a.paste.Active() {
		t.Error("handler should still be active")
	}
}

func TestPasteHandler_Preview_IncludesSizeAndLines(t *testing.T) {
	h := NewPasteHandler()
	// Build content >5 lines so preview truncates.
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	content := strings.Join(lines, "\n")
	cmd := h.HandlePaste(paste(t, content))
	b, ok := drainCmd(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, "10 lines") && !strings.Contains(b.Text, "10") {
		t.Errorf("banner should include line count: %q", b.Text)
	}
	if !strings.Contains(b.Text, fmt.Sprintf("%d", len(content))) {
		t.Errorf("banner should include byte size %d: %q", len(content), b.Text)
	}
}
