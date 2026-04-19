package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/harness/tools"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func writeContextFile(t *testing.T, root, rel, body string) string {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return abs
}

func TestContextManager_Resolve_SingleFile(t *testing.T) {
	root := t.TempDir()
	writeContextFile(t, root, "a.txt", "hello world")
	cm := NewContextManager(root, tools.NewFileTracker())

	atts, err := cm.Resolve(context.Background(), []string{"a.txt"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(atts))
	}
	if atts[0].Path != "a.txt" {
		t.Errorf("Path = %q", atts[0].Path)
	}
	if !strings.Contains(atts[0].Content, "hello world") {
		t.Errorf("Content missing body: %q", atts[0].Content)
	}
	if atts[0].Transform != "read" {
		t.Errorf("Transform = %q, want read", atts[0].Transform)
	}
}

func TestContextManager_Resolve_GlobExpansion(t *testing.T) {
	root := t.TempDir()
	writeContextFile(t, root, "src/a.go", "package a")
	writeContextFile(t, root, "src/b.go", "package b")
	writeContextFile(t, root, "src/c.txt", "skip")
	cm := NewContextManager(root, tools.NewFileTracker())

	atts, err := cm.Resolve(context.Background(), []string{"src/*.go"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(atts) != 2 {
		t.Fatalf("expected 2 attachments, got %d:\n%+v", len(atts), atts)
	}
	// Two attachments, paths should end in a.go / b.go (order not guaranteed).
	got := map[string]bool{}
	for _, a := range atts {
		if a.Transform == "error" {
			t.Errorf("unexpected error attachment: %+v", a)
		}
		got[filepath.Base(a.Path)] = true
	}
	if !got["a.go"] || !got["b.go"] {
		t.Errorf("missing expected matches; got %v", got)
	}
}

func TestContextManager_Resolve_MissingFile_Errs(t *testing.T) {
	cm := NewContextManager(t.TempDir(), tools.NewFileTracker())
	atts, err := cm.Resolve(context.Background(), []string{"nope.txt"})
	if err != nil {
		t.Fatalf("Resolve should surface per-attachment errors, not fail: %v", err)
	}
	if len(atts) != 1 || atts[0].Transform != "error" {
		t.Errorf("expected error attachment; got %+v", atts)
	}
}

func TestContextManager_Resolve_Empty(t *testing.T) {
	cm := NewContextManager(t.TempDir(), tools.NewFileTracker())
	atts, err := cm.Resolve(context.Background(), nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if atts != nil {
		t.Errorf("empty refs should return nil; got %+v", atts)
	}
}

func TestApp_SubmitTurn_ResolvesAttachments(t *testing.T) {
	root := t.TempDir()
	writeContextFile(t, root, "note.txt", "some context")
	cm := NewContextManager(root, tools.NewFileTracker())

	fc := &fakeClient{submitAck: TurnSubmitAck{TurnID: "t-1"}}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn, Attacher: cm})
	app.state.SessionID = "sess"

	_, cmd := app.Update(SubmitMsg{
		Content:     "explain @note.txt",
		Attachments: []string{"note.txt"},
	})
	msg := drainCmdAny(cmd)
	if tm, ok := msg.(TurnSubmittedMsg); !ok || tm.Err != nil {
		t.Fatalf("submit failed: %+v", msg)
	}
	if len(fc.submitCalls) != 1 {
		t.Fatalf("submit call count = %d", len(fc.submitCalls))
	}
	call := fc.submitCalls[0]
	if len(call.Attachments) != 1 {
		t.Fatalf("attachments on submit = %d, want 1", len(call.Attachments))
	}
	if !strings.Contains(call.Attachments[0].Content, "some context") {
		t.Errorf("attachment content should carry file body; got %q", call.Attachments[0].Content)
	}
}

func TestApp_SubmitTurn_NoAttacher_NoAttachments(t *testing.T) {
	fc := &fakeClient{submitAck: TurnSubmitAck{TurnID: "t-2"}}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn}) // no Attacher

	_, cmd := app.Update(SubmitMsg{
		Content:     "explain @ignored",
		Attachments: []string{"ignored"},
	})
	_ = drainCmdAny(cmd)
	if len(fc.submitCalls) != 1 {
		t.Fatalf("submit call count = %d", len(fc.submitCalls))
	}
	if len(fc.submitCalls[0].Attachments) != 0 {
		t.Errorf("no attacher → no attachments; got %d", len(fc.submitCalls[0].Attachments))
	}
}

func TestApp_ContextCommand_Success(t *testing.T) {
	fc := &fakeClient{contextListResp: protocol.ContextListResponse{
		Files: []protocol.ContextFile{
			{Path: "src/a.go", SizeBytes: 1024, AttachedTurn: 3},
		},
		PinnedItems:   []string{"goals.md"},
		ContextTokens: 4500,
		ContextWindow: 200000,
		ContextPct:    0.0225,
	}}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.state.SessionID = "sess"

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "context"})
	loaded := drainCmdAny(cmd).(ContextListLoadedMsg)
	if loaded.Response.ContextTokens != 4500 {
		t.Errorf("ContextTokens = %d", loaded.Response.ContextTokens)
	}
	_, bc := app.Update(loaded)
	b := drainCmdAny(bc).(BannerMsg)
	for _, want := range []string{"4500", "200000", "src/a.go", "goals.md"} {
		if !strings.Contains(b.Text, want) {
			t.Errorf("banner missing %q:\n%s", want, b.Text)
		}
	}
	if fc.contextListCalls[0] != "sess" {
		t.Errorf("sessionID = %q", fc.contextListCalls[0])
	}
}

func TestApp_ContextCommand_NoSession_Errors(t *testing.T) {
	conn := &fakeConn{client: &fakeClient{}}
	app := NewApp(AppOptions{Conn: conn})
	// no SessionID
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "context"})
	e, ok := drainCmdAny(cmd).(ContextErrMsg)
	if !ok {
		t.Fatalf("expected ContextErrMsg, got %T", cmd())
	}
	if !strings.Contains(e.Err.Error(), "no active session") {
		t.Errorf("Err = %v", e.Err)
	}
}

func TestApp_ContextCommand_WithArgs_Rejected(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "context", CommandArgs: "extra"})
	b := drainCmdAny(cmd).(BannerMsg)
	if !strings.Contains(b.Text, "no arguments") {
		t.Errorf("banner should reject args: %q", b.Text)
	}
}

func TestApp_ContextCommand_RPCError(t *testing.T) {
	fc := &fakeClient{contextListErr: errors.New("boom")}
	conn := &fakeConn{client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.state.SessionID = "sess"

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "context"})
	e := drainCmdAny(cmd).(ContextErrMsg)
	if !strings.Contains(e.Err.Error(), "boom") {
		t.Errorf("Err = %v", e.Err)
	}
}
