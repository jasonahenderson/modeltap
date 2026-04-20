package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// SessionLockTTL is how long a session lock remains valid after
// acquisition. Refreshed on ping. Shorter than FEAT-0008's 40s grace
// math but long enough for the lock to outlive a single heartbeat gap.
const SessionLockTTL = 5 * time.Minute

// SessionManager owns the in-memory lifecycle of active sessions for a
// single BFF Server. Persistence is delegated to storage.Store; this
// struct tracks only the volatile (conversation, ongoing turn) portion
// of session state.
type SessionManager struct {
	store storage.Store

	mu     sync.Mutex
	active map[string]*ActiveSession
}

// verifySessionAccess returns a "session not found" TransportError
// when the connection's principal doesn't own the given session.
// WU-094 H-10 / H-11 / H-12: today every connection runs as
// SoloUserID so the check is a no-op, but it lands in every
// session-scoped handler now so the auth WU doesn't have to
// retrofit dozens of call sites. Deliberately leaks no "forbidden"
// distinction — existence and ownership failures look identical.
func verifySessionAccess(conn *Connection, sess *storage.Session) error {
	if sess == nil {
		return &TransportError{Code: CodeSessionNotFound, Message: "session not found"}
	}
	if sess.UserID != "" && conn.UserID() != "" && sess.UserID != conn.UserID() {
		return &TransportError{
			Code:    CodeSessionNotFound,
			Message: fmt.Sprintf("session %q not found", sess.ID),
		}
	}
	return nil
}

// ActiveSession is the in-memory footprint of a session currently bound
// to a connection. Fields beyond those needed by WU-050 handlers are
// added by downstream WUs (turn state, branch manager, etc.).
type ActiveSession struct {
	ID           string
	UserID       string
	Project      string
	ConnID       string
	Conversation *Conversation

	// ModelOverride is set by model.switch (WU-059).
	ModelOverride string

	// Running totals. Updated as turns complete (WU-053/056).
	TotalCost         float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	ContextPct        float64
}

// NewSessionManager constructs a SessionManager over the given store.
func NewSessionManager(store storage.Store) *SessionManager {
	return &SessionManager{
		store:  store,
		active: make(map[string]*ActiveSession),
	}
}

// Register attaches the session handlers to a Dispatcher. Called by
// Server during construction.
func (sm *SessionManager) Register(d *Dispatcher) {
	d.Register(protocol.MethodSessionResume, handleSessionResume)
	d.Register(protocol.MethodSessionList, handleSessionList)
	d.Register(protocol.MethodSessionDetails, handleSessionDetails)
	d.Register(protocol.MethodSessionClear, handleSessionClear)
	d.Register(protocol.MethodSessionFork, handleSessionFork)
}

// GetActiveSession returns the in-memory session state, or nil when the
// session is not currently active. Exported for WU-064 (session.sync).
func (sm *SessionManager) GetActiveSession(sessionID string) *ActiveSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.active[sessionID]
}

// EnsureActive registers (or returns an existing) ActiveSession for the
// given session ID bound to the given connection. The connection's
// sessionID must already be set by the caller.
func (sm *SessionManager) EnsureActive(sessionID string, conn *Connection) *ActiveSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if as, ok := sm.active[sessionID]; ok {
		as.ConnID = conn.ID()
		return as
	}
	as := &ActiveSession{
		ID:           sessionID,
		ConnID:       conn.ID(),
		Project:      conn.Capabilities().ProjectContext().Root,
		Conversation: NewConversation(sessionID),
	}
	sm.active[sessionID] = as
	return as
}

// Deactivate drops the in-memory state for a session. Called when the
// session's lock is released (disconnect, manual unlock) so subsequent
// reconnections rehydrate from storage.
func (sm *SessionManager) Deactivate(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.active, sessionID)
}

// handleSessionResume implements session.resume per design D2.3.
func handleSessionResume(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.SessionResume
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode session.resume: " + err.Error()}
	}
	if req.SessionID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "session_id is required"}
	}

	srv := conn.server
	sess, err := srv.store.GetSession(ctx, req.SessionID)
	if err != nil || sess == nil {
		return nil, &TransportError{
			Code:    CodeSessionNotFound,
			Message: fmt.Sprintf("session %q not found", req.SessionID),
		}
	}
	if err := verifySessionAccess(conn, sess); err != nil {
		return nil, err
	}

	expiry := time.Now().Add(SessionLockTTL)
	acquired, owner, err := srv.store.AcquireSessionLock(ctx, req.SessionID, conn.ID(), expiry)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "acquire lock: " + err.Error()}
	}
	if !acquired {
		diag := protocol.Diagnostic{
			Code:     protocol.DiagSessionLocked,
			Category: "session",
			Cause:    fmt.Sprintf("session locked by %q", owner),
		}
		diagRaw, _ := json.Marshal(diag)
		return nil, &TransportError{
			Code:    CodeSessionLocked,
			Message: fmt.Sprintf("session %q is locked by another owner", req.SessionID),
			Data:    json.RawMessage(diagRaw),
		}
	}

	// Refresh project context with what the harness supplied.
	conn.Capabilities().UpdateProjectContext(req.Project)

	// Rehydrate conversation.
	turns, err := srv.store.ListTurns(ctx, req.SessionID)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "list turns: " + err.Error()}
	}
	active := srv.sessions.EnsureActive(req.SessionID, conn)
	if err := active.Conversation.RestoreFromTurns(turns); err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "restore conversation: " + err.Error()}
	}
	active.UserID = sess.UserID
	active.Project = sess.Project
	if sess.ModelOverride != nil {
		active.ModelOverride = *sess.ModelOverride
	}
	active.TotalCost = sess.TotalCost
	active.TotalInputTokens = sess.TotalInputTokens
	active.TotalOutputTokens = sess.TotalOutputTokens
	active.ContextPct = sess.ContextPct

	conn.SetSessionID(req.SessionID)
	// Rescue any pending grace-period release (harness reconnected).
	conn.cancelGracePeriodRelease()

	resp := &protocol.SessionResumeResponse{
		SessionID: req.SessionID,
		Model:     sess.ActiveModel,
		Project:   conn.Capabilities().ProjectContext(),
	}
	if sess.ModelOverride != nil {
		resp.ModelOverride = *sess.ModelOverride
	}
	return resp, nil
}

// handleSessionList implements session.list per design D2.4.
func handleSessionList(ctx context.Context, conn *Connection, _ json.RawMessage) (any, error) {
	filter := storage.SessionFilter{
		UserID:  conn.UserID(),
		Project: conn.Capabilities().ProjectContext().Root,
		Limit:   50,
	}
	summaries, err := conn.server.store.SessionSummaries(ctx, filter)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "session summaries: " + err.Error()}
	}
	out := make([]protocol.SessionSummary, 0, len(summaries))
	for _, s := range summaries {
		ps := protocol.SessionSummary{
			ID:              s.ID,
			Project:         s.Project,
			Status:          s.Status,
			Summary:         s.Summary,
			LastActive:      s.LastActive.UTC().Format(time.RFC3339),
			ContextPct:      s.ContextPct,
			TotalCost:       s.TotalCost,
			TurnCount:       s.TurnCount,
			Model:           s.Model,
			LastTurnSummary: s.LastTurnSummary,
			FilesTouched:    s.FilesTouched,
			PinnedCount:     s.PinnedCount,
		}
		if s.ModelOverride != nil {
			ps.ModelOverride = *s.ModelOverride
		}
		out = append(out, ps)
	}
	return &protocol.SessionListResponse{Sessions: out}, nil
}

// handleSessionDetails implements session.details per design D2.5.
func handleSessionDetails(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.SessionDetails
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode session.details: " + err.Error()}
	}
	if req.SessionID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "session_id is required"}
	}

	srv := conn.server
	sess, err := srv.store.GetSession(ctx, req.SessionID)
	if err != nil || sess == nil {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: fmt.Sprintf("session %q not found", req.SessionID)}
	}
	if err := verifySessionAccess(conn, sess); err != nil {
		return nil, err
	}

	turns, err := srv.store.ListTurns(ctx, req.SessionID)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "list turns: " + err.Error()}
	}
	turnSummaries := make([]protocol.TurnSummary, 0, len(turns))
	for _, t := range turns {
		turnSummaries = append(turnSummaries, protocol.TurnSummary{
			Sequence:      t.Sequence,
			Summary:       turnSummary(t),
			Compacted:     t.Compacted,
			OriginalTurns: t.OriginalTurns,
			Model:         t.Model,
			Cost:          t.Cost,
		})
	}

	events, err := srv.store.ListServerEvents(ctx, req.SessionID)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "list events: " + err.Error()}
	}
	evOut := make([]protocol.ServerSessionEvent, 0, len(events))
	for _, e := range events {
		evOut = append(evOut, protocol.ServerSessionEvent{
			Type:   e.Type,
			At:     e.At.UTC().Format(time.RFC3339),
			Detail: e.Detail,
		})
	}

	filesTouched, _ := srv.store.SessionFilesTouched(ctx, req.SessionID)
	filesModified, _ := srv.store.SessionFilesModified(ctx, req.SessionID)

	resp := &protocol.SessionDetail{
		ID:            sess.ID,
		Summary:       sess.Summary,
		CreatedAt:     sess.CreatedAt.UTC().Format(time.RFC3339),
		LastActive:    sess.UpdatedAt.UTC().Format(time.RFC3339),
		Model:         sess.ActiveModel,
		ContextPct:    sess.ContextPct,
		TotalCost:     sess.TotalCost,
		Turns:         turnSummaries,
		FilesTouched:  filesTouched,
		FilesModified: filesModified,
		ServerEvents:  evOut,
	}
	if sess.ModelOverride != nil {
		resp.ModelOverride = *sess.ModelOverride
	}
	if len(sess.PinnedItems) > 0 {
		// PinnedItems is a JSON array of strings in storage; unmarshal.
		var pinned []string
		if err := json.Unmarshal(sess.PinnedItems, &pinned); err == nil {
			resp.PinnedItems = pinned
		}
	}
	return resp, nil
}

// handleSessionClear implements session.clear per design D2.6.
func handleSessionClear(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.SessionClear
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode session.clear: " + err.Error()}
	}
	if req.SessionID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "session_id is required"}
	}

	srv := conn.server
	sess, err := srv.store.GetSession(ctx, req.SessionID)
	if err != nil || sess == nil {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: fmt.Sprintf("session %q not found", req.SessionID)}
	}
	if err := verifySessionAccess(conn, sess); err != nil {
		return nil, err
	}

	active := srv.sessions.EnsureActive(req.SessionID, conn)
	cleared := active.Conversation.Reset()

	// Record the manual clear event.
	_ = srv.store.AppendServerEvent(ctx, &storage.ServerSessionEvent{
		SessionID: req.SessionID,
		Type:      "manual_clear",
		Detail:    fmt.Sprintf("cleared %d in-memory turns", cleared),
		At:        time.Now().UTC(),
	})

	return &protocol.SessionClearResponse{
		ClearedTurns:      cleared,
		RetainedInStorage: true,
	}, nil
}

// handleSessionFork implements session.fork per design D2.7.
func handleSessionFork(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.SessionFork
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode session.fork: " + err.Error()}
	}
	if req.SessionID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "session_id is required"}
	}

	srv := conn.server
	src, err := srv.store.GetSession(ctx, req.SessionID)
	if err != nil || src == nil {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: fmt.Sprintf("session %q not found", req.SessionID)}
	}
	if err := verifySessionAccess(conn, src); err != nil {
		return nil, err
	}

	// Create the new session with copied fields but reset cost/tokens.
	now := time.Now().UTC()
	newID := uuid.NewString()
	newSess := &storage.Session{
		ID:              newID,
		UserID:          src.UserID,
		Project:         src.Project,
		Summary:         src.Summary,
		ActiveModel:     src.ActiveModel,
		PinnedItems:     src.PinnedItems,
		CompactionState: src.CompactionState,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := srv.store.CreateSession(ctx, newSess); err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "create forked session: " + err.Error()}
	}

	// Copy turns with new session_id; preserve sequence numbers.
	srcTurns, err := srv.store.ListTurns(ctx, src.ID)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "list turns: " + err.Error()}
	}
	for _, t := range srcTurns {
		copied := t
		copied.ID = uuid.NewString()
		copied.SessionID = newID
		copied.CreatedAt = now
		if err := srv.store.CreateTurn(ctx, &copied); err != nil {
			return nil, &TransportError{Code: CodeInternalError, Message: "copy turn: " + err.Error()}
		}
	}

	// Record fork event on the source.
	_ = srv.store.AppendServerEvent(ctx, &storage.ServerSessionEvent{
		SessionID: src.ID,
		Type:      "fork",
		Detail:    fmt.Sprintf("forked to %s", newID),
		At:        now,
	})

	return &protocol.SessionForkResponse{
		NewSessionID:      newID,
		OriginalSessionID: src.ID,
	}, nil
}

// turnSummary derives the short label shown in SessionDetail.Turns.
func turnSummary(t storage.Turn) string {
	// Prefer the compacted summary when present.
	if t.Compacted && t.CompactedSummary != "" {
		return t.CompactedSummary
	}
	var msg struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(t.Content, &msg); err == nil && msg.Content != "" {
		if len(msg.Content) > 80 {
			return msg.Content[:80] + "..."
		}
		return msg.Content
	}
	return t.Role
}

// Ensure uuid import is referenced even if every future codepath uses
// it from another file — defensive to keep go vet clean.
var _ = uuid.NewString
var _ = errors.New
