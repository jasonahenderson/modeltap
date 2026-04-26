package harness

import (
	"errors"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestApp_ShowCurrentSession(t *testing.T) {
	app := NewApp(AppOptions{})
	app.state.SessionID = "" // simulate fresh start with no session
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session"})
	b, ok := drainCmdAny(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, "new session") {
		t.Errorf("banner should say new session when unset: %q", b.Text)
	}

	app.state.SessionID = "sess-9"
	_, cmd = app.Update(SubmitMsg{IsCommand: true, Command: "session"})
	b = drainCmdAny(cmd).(BannerMsg)
	if !strings.Contains(b.Text, "sess-9") {
		t.Errorf("banner should include active session id: %q", b.Text)
	}
}

func TestApp_SessionsList_Success(t *testing.T) {
	fc := &fakeClient{sessionListResp: protocol.SessionListResponse{
		Sessions: []protocol.SessionSummary{
			{ID: "s1", Summary: "Design", ContextPct: 0.12, TotalCost: 0.05, TurnCount: 8},
			{ID: "s2", Summary: "Implement", ContextPct: 0.37, TotalCost: 0.22, TurnCount: 23},
		},
	}}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "sessions"})
	loaded := drainCmdAny(cmd).(SessionListLoadedMsg)
	if len(loaded.Response.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(loaded.Response.Sessions))
	}
	_, bc := app.Update(loaded)
	b := drainCmdAny(bc).(BannerMsg)
	for _, want := range []string{"s1", "s2", "Design", "Implement", "8 turns", "23 turns"} {
		if !strings.Contains(b.Text, want) {
			t.Errorf("banner missing %q:\n%s", want, b.Text)
		}
	}
}

func TestApp_SessionsList_Empty(t *testing.T) {
	fc := &fakeClient{}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "sessions"})
	loaded := drainCmdAny(cmd).(SessionListLoadedMsg)
	_, bc := app.Update(loaded)
	b := drainCmdAny(bc).(BannerMsg)
	if !strings.Contains(strings.ToLower(b.Text), "no sessions") {
		t.Errorf("empty list should say no sessions: %q", b.Text)
	}
}

func TestApp_SessionResume_Success(t *testing.T) {
	fc := &fakeClient{sessionResumeResp: protocol.SessionResumeResponse{
		SessionID:     "s-42",
		Model:         "claude-opus-4-7",
		ModelOverride: "claude-opus-4-7",
	}}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session", CommandArgs: "resume s-42"})
	msg := drainCmdAny(cmd)
	resumed, ok := msg.(SessionResumedMsg)
	if !ok {
		t.Fatalf("expected SessionResumedMsg, got %T", msg)
	}
	if resumed.Response.SessionID != "s-42" {
		t.Errorf("SessionID = %q", resumed.Response.SessionID)
	}
	if len(fc.sessionResumeCalls) != 1 || fc.sessionResumeCalls[0] != "s-42" {
		t.Errorf("resume call mismatch: %v", fc.sessionResumeCalls)
	}

	// Apply to state.
	model, bc := app.Update(resumed)
	a, _ := model.(App)
	if a.state.SessionID != "s-42" {
		t.Errorf("SessionID not set")
	}
	if a.state.ModelName != "claude-opus-4-7" {
		t.Errorf("ModelName not propagated: %q", a.state.ModelName)
	}
	if !a.state.ModelOverride {
		t.Errorf("ModelOverride should be true when server reports one")
	}
	b := drainCmdAny(bc).(BannerMsg)
	if !strings.Contains(b.Text, "s-42") {
		t.Errorf("banner should include session id: %q", b.Text)
	}
}

func TestApp_SessionResume_MissingID(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session", CommandArgs: "resume"})
	b := drainCmdAny(cmd).(BannerMsg)
	if !strings.Contains(b.Text, "Usage") {
		t.Errorf("should show usage hint: %q", b.Text)
	}
}

func TestApp_SessionResume_Error(t *testing.T) {
	fc := &fakeClient{sessionResumeErr: errors.New("not found")}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session", CommandArgs: "resume missing"})
	e := drainCmdAny(cmd).(SessionErrMsg)
	if e.Command != "session resume" {
		t.Errorf("Command = %q", e.Command)
	}
}

func TestApp_SessionClear_Success(t *testing.T) {
	fc := &fakeClient{sessionClearResp: protocol.SessionClearResponse{ClearedTurns: 7}}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.state.SessionID = "s-live"

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session", CommandArgs: "clear"})
	cleared := drainCmdAny(cmd).(SessionClearedMsg)
	if cleared.Response.ClearedTurns != 7 {
		t.Errorf("ClearedTurns = %d", cleared.Response.ClearedTurns)
	}
	if len(fc.sessionClearCalls) != 1 || fc.sessionClearCalls[0] != "s-live" {
		t.Errorf("clear call mismatch: %v", fc.sessionClearCalls)
	}

	_, bc := app.Update(cleared)
	b := drainCmdAny(bc).(BannerMsg)
	if !strings.Contains(b.Text, "7 turns") {
		t.Errorf("banner should show turn count: %q", b.Text)
	}
}

func TestApp_SessionClear_NoActiveSession(t *testing.T) {
	fc := &fakeClient{}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.state.SessionID = "" // simulate no active session

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session", CommandArgs: "clear"})
	e := drainCmdAny(cmd).(SessionErrMsg)
	if !strings.Contains(e.Err.Error(), "no active session") {
		t.Errorf("Err = %v", e.Err)
	}
	// Clear should NOT have hit the BFF.
	if len(fc.sessionClearCalls) != 0 {
		t.Errorf("clear should short-circuit without an active session")
	}
}

func TestApp_SessionFork_Success(t *testing.T) {
	fc := &fakeClient{sessionForkResp: protocol.SessionForkResponse{
		OriginalSessionID: "s-orig",
		NewSessionID:      "s-fork",
	}}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.state.SessionID = "s-orig"

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session", CommandArgs: "fork"})
	forked := drainCmdAny(cmd).(SessionForkedMsg)
	if forked.Response.NewSessionID != "s-fork" {
		t.Errorf("NewSessionID = %q", forked.Response.NewSessionID)
	}
	model, bc := app.Update(forked)
	a, _ := model.(App)
	if a.state.SessionID != "s-fork" {
		t.Errorf("App should switch to forked session; got %q", a.state.SessionID)
	}
	b := drainCmdAny(bc).(BannerMsg)
	if !strings.Contains(b.Text, "s-fork") {
		t.Errorf("banner should name new session: %q", b.Text)
	}
}

func TestApp_SessionCommand_UnknownSub(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session", CommandArgs: "bogus"})
	b, ok := drainCmdAny(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	if !strings.Contains(b.Text, "bogus") {
		t.Errorf("banner should name the unknown subcommand: %q", b.Text)
	}
}

func TestApp_SessionList_Subcommand(t *testing.T) {
	fc := &fakeClient{}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})

	// Both /sessions and /session list hit the same path.
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "session", CommandArgs: "list"})
	if _, ok := drainCmdAny(cmd).(SessionListLoadedMsg); !ok {
		t.Fatalf("expected SessionListLoadedMsg from /session list")
	}
}
