package harness

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestCompactHandler_Stage_RendersBanner(t *testing.T) {
	h := NewCompactHandler()
	plan := &protocol.CompactPlan{
		Categories: []protocol.CompactCategory{
			{Name: "older_turns", TokenCount: 4000, SuggestedAction: "summarize"},
			{Name: "recent", TokenCount: 800, SuggestedAction: "keep"},
		},
		EstimatedTokensFreed: 3600,
		ContextPctBefore:     0.42,
		ContextPctAfter:      0.08,
	}
	cmd := h.Stage(plan)
	if cmd == nil {
		t.Fatal("Stage should return a banner cmd")
	}
	b, ok := cmd().(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	for _, want := range []string{"older_turns", "summarize", "recent", "[a]pply", "[c]ancel", "3600"} {
		if !strings.Contains(b.Text, want) {
			t.Errorf("banner missing %q:\n%s", want, b.Text)
		}
	}
	if b.Duration != 0 {
		t.Errorf("compact plan banner should be persistent; got %v", b.Duration)
	}
	if !h.Active() {
		t.Error("handler should be active after Stage")
	}
}

func TestCompactHandler_Stage_EmptyPlan(t *testing.T) {
	h := NewCompactHandler()
	cmd := h.Stage(&protocol.CompactPlan{})
	b := cmd().(BannerMsg)
	if !strings.Contains(strings.ToLower(b.Text), "nothing to compact") {
		t.Errorf("banner should indicate empty plan: %q", b.Text)
	}
}

func TestCompactHandler_ApplyAllEmitsRequest(t *testing.T) {
	h := NewCompactHandler()
	plan := &protocol.CompactPlan{
		Categories: []protocol.CompactCategory{
			{Name: "older_turns", SuggestedAction: "summarize"},
			{Name: "recent", SuggestedAction: "keep"},
		},
	}
	_ = h.Stage(plan)

	cmd := h.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected cmd on [a]")
	}
	msg := cmd()
	req, ok := msg.(compactApplyRequestMsg)
	if !ok {
		t.Fatalf("expected compactApplyRequestMsg, got %T", msg)
	}
	if req.actions["older_turns"] != "summarize" || req.actions["recent"] != "keep" {
		t.Errorf("actions did not mirror suggested: %+v", req.actions)
	}
}

func TestCompactHandler_CancelClearsOverlay(t *testing.T) {
	h := NewCompactHandler()
	_ = h.Stage(&protocol.CompactPlan{
		Categories: []protocol.CompactCategory{{Name: "older_turns", SuggestedAction: "summarize"}},
	})

	cmd := h.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd on Esc")
	}
	msgs := flattenBatch(cmd())
	var sawClear, sawBanner bool
	for _, m := range msgs {
		if _, ok := m.(BannerClearMsg); ok {
			sawClear = true
		}
		if b, ok := m.(BannerMsg); ok {
			if strings.Contains(strings.ToLower(b.Text), "cancel") {
				sawBanner = true
			}
		}
	}
	if !sawClear {
		t.Error("cancel should emit BannerClearMsg")
	}
	if !sawBanner {
		t.Error("cancel should emit a cancellation banner")
	}
	if h.Active() {
		t.Error("handler should be inactive after cancel")
	}
}

func TestCompactHandler_IgnoresUnrelatedKeys(t *testing.T) {
	h := NewCompactHandler()
	_ = h.Stage(&protocol.CompactPlan{
		Categories: []protocol.CompactCategory{{Name: "older_turns", SuggestedAction: "summarize"}},
	})

	if cmd := h.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}); cmd != nil {
		t.Errorf("unrelated key should be swallowed; got cmd=%v", cmd())
	}
	if !h.Active() {
		t.Error("handler should stay active after unrelated key")
	}
}

func TestApp_CompactCommand_NoSession(t *testing.T) {
	app := NewApp(AppOptions{})
	// no session id
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "compact"})
	b, ok := drainCmdAny(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, "active session") {
		t.Errorf("banner should mention active session; got %q", b.Text)
	}
}

func TestApp_CompactCommand_FullRoundTrip(t *testing.T) {
	fc := &fakeClient{
		sessionCompactResp: protocol.CompactPlan{
			Categories: []protocol.CompactCategory{
				{Name: "older_turns", TokenCount: 2000, SuggestedAction: "summarize"},
				{Name: "recent", TokenCount: 500, SuggestedAction: "keep"},
			},
			EstimatedTokensFreed: 1800,
		},
		compactApplyResp: protocol.CompactApplyResponse{
			Applied:     true,
			TokensFreed: 1800,
			Summary:     "summarized 4 older turns",
		},
	}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.State().SessionID = "sess-c"

	// Step 1: /compact dispatches session.compact → CompactPlanLoadedMsg.
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "compact"})
	msg := drainCmdAny(cmd)
	loaded, ok := msg.(CompactPlanLoadedMsg)
	if !ok {
		t.Fatalf("expected CompactPlanLoadedMsg, got %T (%+v)", msg, msg)
	}
	if loaded.Plan == nil || len(loaded.Plan.Categories) != 2 {
		t.Fatalf("unexpected plan: %+v", loaded.Plan)
	}

	// Step 2: App.Update(loaded) stages the handler + emits a banner.
	model, stageCmd := app.Update(loaded)
	app2 := model.(App)
	if !app2.compact.Active() {
		t.Fatal("compact handler should be active after stage")
	}
	b := drainCmdAny(stageCmd).(BannerMsg)
	if !strings.Contains(b.Text, "[a]pply") {
		t.Errorf("stage banner should prompt apply: %q", b.Text)
	}

	// Step 3: user presses 'a' → handler emits compactApplyRequestMsg.
	_, keyCmd := app2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	keyMsg := drainCmdAny(keyCmd)
	req, ok := keyMsg.(compactApplyRequestMsg)
	if !ok {
		t.Fatalf("expected compactApplyRequestMsg from 'a'; got %T", keyMsg)
	}

	// Step 4: App.Update(req) dispatches compact.apply → CompactAppliedMsg.
	_, applyCmd := app2.Update(req)
	appliedMsg := drainCmdAny(applyCmd)
	applied, ok := appliedMsg.(CompactAppliedMsg)
	if !ok {
		t.Fatalf("expected CompactAppliedMsg, got %T", appliedMsg)
	}
	if !applied.Response.Applied || applied.Response.TokensFreed != 1800 {
		t.Errorf("unexpected apply response: %+v", applied.Response)
	}
	if len(fc.compactApplyCalls) != 1 {
		t.Fatalf("expected 1 compact.apply call; got %d", len(fc.compactApplyCalls))
	}
	if fc.compactApplyCalls[0].actions["older_turns"] != "summarize" {
		t.Errorf("actions map not forwarded: %+v", fc.compactApplyCalls[0].actions)
	}

	// Step 5: App.Update(applied) clears handler + emits success banner.
	model, finalCmd := app2.Update(applied)
	app3 := model.(App)
	if app3.compact.Active() {
		t.Error("handler should be cleared after apply")
	}
	finalMsgs := flattenBatch(finalCmd())
	var sawClear, sawFinal bool
	for _, m := range finalMsgs {
		if _, ok := m.(BannerClearMsg); ok {
			sawClear = true
		}
		if b, ok := m.(BannerMsg); ok {
			if strings.Contains(b.Text, "1800") && strings.Contains(b.Text, "summarized") {
				sawFinal = true
			}
		}
	}
	if !sawClear || !sawFinal {
		t.Errorf("expected BannerClear + success banner; clear=%v final=%v", sawClear, sawFinal)
	}
}
