package bff

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// newServerWithRealStore returns a Server backed by an in-memory SQLite
// store so session tests can exercise real session/turn persistence.
func newServerWithRealStore(t *testing.T) *Server {
	t.Helper()
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open sqlite :memory:: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := shortServerConfig("")
	return NewServer(store, cfg)
}

// seedSession inserts a Session and optionally attaches turns. Returns
// the session ID.
func seedSession(t *testing.T, store storage.Store, userID, project string, content string) string {
	t.Helper()
	ctx := context.Background()
	sess := &storage.Session{
		ID:      "sess-" + project + "-" + content,
		UserID:  userID,
		Project: project,
		Summary: content,
		Status:  "active",
	}
	if err := store.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return sess.ID
}

func newReadyConnection(t *testing.T, srv *Server) *Connection {
	t.Helper()
	c := NewConnection("conn-test", NewFrameTransport(nopConn()), srv, false)
	c.setStateForTest(ConnReady)
	c.Capabilities().UpdateProjectContext(protocol.ProjectContext{
		Root:       "/tmp/proj",
		ConfigFile: "modeltap.yaml",
	})
	return c
}

// PATCH-0028: session.create mints a fresh session, persists it,
// acquires the lock, and binds the connection.
func TestSessionCreate_Success(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)

	params, _ := json.Marshal(&protocol.SessionCreate{
		Project: protocol.ProjectContext{Root: "/tmp/proj-new"},
	})

	raw, err := handleSessionCreate(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleSessionCreate: %v", err)
	}
	resp, ok := raw.(*protocol.SessionCreateResponse)
	if !ok {
		t.Fatalf("response type = %T, want *SessionCreateResponse", raw)
	}
	if resp.SessionID == "" {
		t.Fatalf("expected non-empty SessionID")
	}
	if c.SessionID() != resp.SessionID {
		t.Errorf("connection not bound to new session: conn=%q resp=%q", c.SessionID(), resp.SessionID)
	}

	// Storage row should exist with Project from request.
	got, err := srv.store.GetSession(context.Background(), resp.SessionID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got == nil {
		t.Fatal("session row not persisted")
	}
	if got.Project != "/tmp/proj-new" {
		t.Errorf("persisted project = %q, want %q", got.Project, "/tmp/proj-new")
	}
	if got.Status != "active" {
		t.Errorf("persisted status = %q, want active", got.Status)
	}

	// Lock acquired by this connection.
	expiry := time.Now().Add(time.Second)
	acquired, owner, err := srv.store.AcquireSessionLock(context.Background(), resp.SessionID, "different-conn", expiry)
	if err != nil {
		t.Fatalf("AcquireSessionLock: %v", err)
	}
	if acquired {
		t.Errorf("lock should be held by conn-test; another conn acquired it")
	}
	if owner != c.ID() {
		t.Errorf("lock owner = %q, want %q", owner, c.ID())
	}

	// Active session entry exists.
	active := srv.sessions.GetActiveSession(resp.SessionID)
	if active == nil {
		t.Errorf("active session not registered for %q", resp.SessionID)
	}

	// Project context on the connection updated.
	if got := c.Capabilities().ProjectContext().Root; got != "/tmp/proj-new" {
		t.Errorf("connection project = %q, want %q", got, "/tmp/proj-new")
	}
}

func TestSessionCreate_DecodeError(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)

	_, err := handleSessionCreate(context.Background(), c, json.RawMessage(`{not-json`))
	if err == nil {
		t.Fatal("expected decode error")
	}
	var te *TransportError
	if !errors.As(err, &te) {
		t.Fatalf("error type = %T, want *TransportError", err)
	}
	if te.Code != CodeInvalidParams {
		t.Errorf("code = %d, want %d", te.Code, CodeInvalidParams)
	}
}

func TestSessionResume_Success(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "hello world")

	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.SessionResume{
		SessionID: sid,
		Project:   protocol.ProjectContext{Root: "/tmp/proj-updated"},
	})

	raw, err := handleSessionResume(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleSessionResume: %v", err)
	}
	resp, ok := raw.(*protocol.SessionResumeResponse)
	if !ok {
		t.Fatalf("response type = %T", raw)
	}
	if resp.SessionID != sid {
		t.Errorf("SessionID = %q, want %q", resp.SessionID, sid)
	}
	if c.SessionID() != sid {
		t.Errorf("connection not bound: got %q", c.SessionID())
	}
	// Project context refreshed from params.
	if c.Capabilities().ProjectContext().Root != "/tmp/proj-updated" {
		t.Errorf("project not refreshed: %+v", c.Capabilities().ProjectContext())
	}
}

func TestSessionResume_NotFound(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)

	params, _ := json.Marshal(&protocol.SessionResume{SessionID: "nonexistent"})
	_, err := handleSessionResume(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected not-found error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeSessionNotFound {
		t.Errorf("expected CodeSessionNotFound, got %T %v", err, err)
	}
}

func TestSessionResume_Locked(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "locked session")

	// Pre-acquire the lock with a different owner.
	acquired, _, err := srv.store.AcquireSessionLock(context.Background(), sid, "other-conn", time.Now().Add(10*time.Minute))
	if err != nil || !acquired {
		t.Fatalf("pre-acquire: acquired=%v err=%v", acquired, err)
	}

	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.SessionResume{SessionID: sid})

	_, err = handleSessionResume(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected locked error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeSessionLocked {
		t.Errorf("expected CodeSessionLocked, got %T %v", err, err)
	}
}

func TestSessionList_Basic(t *testing.T) {
	srv := newServerWithRealStore(t)
	seedSession(t, srv.store, SoloUserID, "/tmp/proj", "session-a")
	seedSession(t, srv.store, SoloUserID, "/tmp/proj", "session-b")
	// Different project; must NOT appear.
	seedSession(t, srv.store, SoloUserID, "/tmp/other", "session-other")

	c := newReadyConnection(t, srv)
	raw, err := handleSessionList(context.Background(), c, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handleSessionList: %v", err)
	}
	resp := raw.(*protocol.SessionListResponse)
	if len(resp.Sessions) != 2 {
		t.Errorf("got %d sessions, want 2: %+v", len(resp.Sessions), resp.Sessions)
	}
	for _, s := range resp.Sessions {
		if s.Project != "/tmp/proj" {
			t.Errorf("unexpected project in list: %q", s.Project)
		}
	}
}

func TestSessionDetails_Basic(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "detail test")

	// Add a server event.
	if err := srv.store.AppendServerEvent(context.Background(), &storage.ServerSessionEvent{
		SessionID: sid,
		Type:      "manual_clear",
		Detail:    "test-event",
		At:        time.Now().UTC(),
	}); err != nil {
		t.Fatalf("AppendServerEvent: %v", err)
	}

	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.SessionDetails{SessionID: sid})

	raw, err := handleSessionDetails(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleSessionDetails: %v", err)
	}
	detail := raw.(*protocol.SessionDetail)
	if detail.ID != sid {
		t.Errorf("ID = %q", detail.ID)
	}
	if len(detail.ServerEvents) != 1 {
		t.Errorf("ServerEvents = %d, want 1", len(detail.ServerEvents))
	}
}

func TestSessionDetails_NotFound(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.SessionDetails{SessionID: "missing"})
	_, err := handleSessionDetails(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected not-found error")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeSessionNotFound {
		t.Errorf("expected CodeSessionNotFound, got %T %v", err, err)
	}
}

func TestSessionClear_ResetsConversation(t *testing.T) {
	srv := newServerWithRealStore(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "clear test")

	c := newReadyConnection(t, srv)
	c.SetSessionID(sid)

	// Simulate an active conversation by registering the session in the
	// manager with a few messages.
	active := srv.sessions.EnsureActive(sid, c)
	active.Conversation.appendMessageForTest("user", "hi")
	active.Conversation.appendMessageForTest("assistant", "hello")

	before := active.Conversation.TurnCount()
	if before != 2 {
		t.Fatalf("pre-clear TurnCount = %d, want 2", before)
	}

	params, _ := json.Marshal(&protocol.SessionClear{SessionID: sid})
	raw, err := handleSessionClear(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleSessionClear: %v", err)
	}
	resp := raw.(*protocol.SessionClearResponse)
	if resp.ClearedTurns != 2 {
		t.Errorf("ClearedTurns = %d, want 2", resp.ClearedTurns)
	}
	if !resp.RetainedInStorage {
		t.Errorf("RetainedInStorage should be true")
	}
	if active.Conversation.TurnCount() != 0 {
		t.Errorf("TurnCount after clear = %d, want 0", active.Conversation.TurnCount())
	}

	// manual_clear event should have been appended.
	events, err := srv.store.ListServerEvents(context.Background(), sid)
	if err != nil {
		t.Fatalf("ListServerEvents: %v", err)
	}
	found := false
	for _, e := range events {
		if e.Type == "manual_clear" {
			found = true
		}
	}
	if !found {
		t.Errorf("manual_clear event not recorded: %+v", events)
	}
}

func TestSessionFork_CopiesAndResets(t *testing.T) {
	srv := newServerWithRealStore(t)

	// Seed a source session with a turn and cost data.
	src := &storage.Session{
		ID:                "sess-src",
		Project:           "/tmp/proj",
		Summary:           "fork-source",
		Status:            "active",
		TotalCost:         1.23,
		TotalInputTokens:  100,
		TotalOutputTokens: 200,
	}
	if err := srv.store.CreateSession(context.Background(), src); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	turn := &storage.Turn{
		ID:        "turn-1",
		SessionID: src.ID,
		Sequence:  1,
		Role:      "user",
		Content:   json.RawMessage(`{"role":"user","content":"hi"}`),
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateTurn(context.Background(), turn); err != nil {
		t.Fatalf("CreateTurn: %v", err)
	}

	c := newReadyConnection(t, srv)
	c.SetSessionID(src.ID)

	params, _ := json.Marshal(&protocol.SessionFork{SessionID: src.ID})
	raw, err := handleSessionFork(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleSessionFork: %v", err)
	}
	resp := raw.(*protocol.SessionForkResponse)
	if resp.OriginalSessionID != src.ID {
		t.Errorf("OriginalSessionID = %q", resp.OriginalSessionID)
	}
	if resp.NewSessionID == "" || resp.NewSessionID == src.ID {
		t.Errorf("NewSessionID suspicious: %q", resp.NewSessionID)
	}

	// New session should have summary copied, cost reset, turn copied.
	newSess, err := srv.store.GetSession(context.Background(), resp.NewSessionID)
	if err != nil {
		t.Fatalf("GetSession(new): %v", err)
	}
	if newSess.Summary != "fork-source" {
		t.Errorf("new.Summary = %q, want fork-source", newSess.Summary)
	}
	if newSess.TotalCost != 0 || newSess.TotalInputTokens != 0 || newSess.TotalOutputTokens != 0 {
		t.Errorf("cost/tokens not reset: %+v", newSess)
	}

	newTurns, err := srv.store.ListTurns(context.Background(), resp.NewSessionID)
	if err != nil {
		t.Fatalf("ListTurns: %v", err)
	}
	if len(newTurns) != 1 {
		t.Errorf("new session turns = %d, want 1", len(newTurns))
	}

	// fork event recorded on source.
	events, _ := srv.store.ListServerEvents(context.Background(), src.ID)
	foundFork := false
	for _, e := range events {
		if e.Type == "fork" {
			foundFork = true
		}
	}
	if !foundFork {
		t.Errorf("fork event missing: %+v", events)
	}
}

func TestSessionManager_GetActiveSession(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	c.SetSessionID("sess-x")

	srv.sessions.EnsureActive("sess-x", c)
	active := srv.sessions.GetActiveSession("sess-x")
	if active == nil {
		t.Fatalf("GetActiveSession returned nil")
	}
	if active.ID != "sess-x" || active.ConnID != c.ID() {
		t.Errorf("active = %+v", active)
	}

	if srv.sessions.GetActiveSession("missing") != nil {
		t.Errorf("GetActiveSession(missing) should be nil")
	}
}

func TestServer_RegistersSessionHandlers(t *testing.T) {
	srv := newServerWithRealStore(t)
	for _, m := range []string{
		protocol.MethodSessionResume,
		protocol.MethodSessionList,
		protocol.MethodSessionDetails,
		protocol.MethodSessionClear,
		protocol.MethodSessionFork,
	} {
		if _, ok := srv.dispatcher.handlers[m]; !ok {
			t.Errorf("session handler for %q not registered", m)
		}
	}
}
