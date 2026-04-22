package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func makeSession(id, userID, project string) *Session {
	now := time.Now().UTC().Truncate(time.Millisecond)
	return &Session{
		ID:               id,
		UserID:           userID,
		Project:          project,
		Summary:          "test session",
		ActiveModel:      "claude-3",
		RoutingOverrides: json.RawMessage(`{}`),
		PinnedItems:      json.RawMessage(`[]`),
		CompactionState:  json.RawMessage(`{}`),
		Status:           "active",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestCreateSession(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got == nil {
		t.Fatal("GetSession returned nil")
	}
	if got.ID != "sess-1" {
		t.Errorf("ID = %q, want %q", got.ID, "sess-1")
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", got.UserID, "user-1")
	}
	if got.Project != "proj-a" {
		t.Errorf("Project = %q, want %q", got.Project, "proj-a")
	}
	if got.Status != "active" {
		t.Errorf("Status = %q, want %q", got.Status, "active")
	}
}

func TestGetSession_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetSession(ctx, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("GetSession(nonexistent) err = %v, want ErrSessionNotFound", err)
	}
}

func TestUpdateSession(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	s.Summary = "updated summary"
	s.Status = "suspended"
	s.TotalCost = 0.42
	s.TotalInputTokens = 1000
	s.TotalOutputTokens = 500
	s.ContextPct = 0.75
	s.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)

	if err := store.UpdateSession(ctx, s); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	got, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Summary != "updated summary" {
		t.Errorf("Summary = %q, want %q", got.Summary, "updated summary")
	}
	if got.Status != "suspended" {
		t.Errorf("Status = %q, want %q", got.Status, "suspended")
	}
	if got.TotalCost != 0.42 {
		t.Errorf("TotalCost = %v, want 0.42", got.TotalCost)
	}
}

func TestUpdateSession_ModelOverride(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	override := "gpt-4"
	s.ModelOverride = &override
	s.UpdatedAt = time.Now().UTC().Truncate(time.Millisecond)
	if err := store.UpdateSession(ctx, s); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	got, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ModelOverride == nil || *got.ModelOverride != "gpt-4" {
		t.Errorf("ModelOverride = %v, want %q", got.ModelOverride, "gpt-4")
	}
}

func TestListSessions(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create sessions for two users
	for _, s := range []*Session{
		makeSession("sess-1", "user-1", "proj-a"),
		makeSession("sess-2", "user-1", "proj-b"),
		makeSession("sess-3", "user-2", "proj-a"),
	} {
		if err := store.CreateSession(ctx, s); err != nil {
			t.Fatalf("CreateSession(%s): %v", s.ID, err)
		}
	}

	// List for user-1 only
	got, err := store.ListSessions(ctx, SessionFilter{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListSessions returned %d sessions, want 2", len(got))
	}
}

func TestListSessions_RequiresUserID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.ListSessions(ctx, SessionFilter{})
	if err != ErrUserIDRequired {
		t.Errorf("ListSessions without UserID: err = %v, want ErrUserIDRequired", err)
	}
}

func TestListSessions_FilterByProject(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for _, s := range []*Session{
		makeSession("sess-1", "user-1", "proj-a"),
		makeSession("sess-2", "user-1", "proj-b"),
	} {
		if err := store.CreateSession(ctx, s); err != nil {
			t.Fatalf("CreateSession(%s): %v", s.ID, err)
		}
	}

	got, err := store.ListSessions(ctx, SessionFilter{UserID: "user-1", Project: "proj-a"})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListSessions returned %d sessions, want 1", len(got))
	}
	if got[0].ID != "sess-1" {
		t.Errorf("session ID = %q, want %q", got[0].ID, "sess-1")
	}
}

func TestListSessions_FilterByStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s1 := makeSession("sess-1", "user-1", "proj-a")
	s2 := makeSession("sess-2", "user-1", "proj-a")
	s2.Status = "completed"

	for _, s := range []*Session{s1, s2} {
		if err := store.CreateSession(ctx, s); err != nil {
			t.Fatalf("CreateSession(%s): %v", s.ID, err)
		}
	}

	got, err := store.ListSessions(ctx, SessionFilter{UserID: "user-1", Status: "active"})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListSessions returned %d sessions, want 1", len(got))
	}
	if got[0].ID != "sess-1" {
		t.Errorf("session ID = %q, want %q", got[0].ID, "sess-1")
	}
}

func TestListSessions_InvalidStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.ListSessions(ctx, SessionFilter{UserID: "user-1", Status: "bogus"})
	if err != ErrInvalidStatus {
		t.Errorf("ListSessions with bad status: err = %v, want ErrInvalidStatus", err)
	}
}

func TestDeleteSessionsBefore(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	old := makeSession("sess-old", "user-1", "proj-a")
	old.UpdatedAt = time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Millisecond)
	old.CreatedAt = old.UpdatedAt
	recent := makeSession("sess-new", "user-1", "proj-a")

	for _, s := range []*Session{old, recent} {
		if err := store.CreateSession(ctx, s); err != nil {
			t.Fatalf("CreateSession(%s): %v", s.ID, err)
		}
	}

	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	n, err := store.DeleteSessionsBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteSessionsBefore: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted %d, want 1", n)
	}

	// Old session gone
	_, err = store.GetSession(ctx, "sess-old")
	if err != ErrSessionNotFound {
		t.Errorf("GetSession(old) err = %v, want ErrSessionNotFound", err)
	}

	// Recent session still there
	got, err := store.GetSession(ctx, "sess-new")
	if err != nil {
		t.Fatalf("GetSession(new): %v", err)
	}
	if got == nil {
		t.Fatal("recent session should still exist")
	}
}

func TestDeleteSession_CascadeTurnsAndEvents(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	s.UpdatedAt = time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Millisecond)
	s.CreatedAt = s.UpdatedAt
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Add a turn
	turn := &Turn{
		ID:        "turn-1",
		SessionID: "sess-1",
		Sequence:  1,
		Role:      "user",
		Content:   json.RawMessage(`"hello"`),
		ToolCalls: json.RawMessage(`[]`),
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := store.CreateTurn(ctx, turn); err != nil {
		t.Fatalf("CreateTurn: %v", err)
	}

	// Add an event
	evt := &ServerSessionEvent{
		SessionID: "sess-1",
		Type:      "auto_compact",
		Detail:    "freed 500 tokens",
		Payload:   json.RawMessage(`{"freed_tokens": 500}`),
		At:        time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := store.AppendServerEvent(ctx, evt); err != nil {
		t.Fatalf("AppendServerEvent: %v", err)
	}

	// Delete the session
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	_, err := store.DeleteSessionsBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteSessionsBefore: %v", err)
	}

	// Turn should be gone (cascade)
	got, err := store.GetTurn(ctx, "turn-1")
	if err != nil {
		t.Fatalf("GetTurn after cascade: %v", err)
	}
	if got != nil {
		t.Error("turn should have been cascaded on session delete")
	}

	// Events should be gone (cascade)
	events, err := store.ListServerEvents(ctx, "sess-1")
	if err != nil {
		t.Fatalf("ListServerEvents after cascade: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("events should be empty after cascade, got %d", len(events))
	}
}

// --- Session Lock Tests ---

func TestAcquireSessionLock(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	expires := time.Now().UTC().Add(5 * time.Minute)
	acquired, owner, err := store.AcquireSessionLock(ctx, "sess-1", "harness-1", expires)
	if err != nil {
		t.Fatalf("AcquireSessionLock: %v", err)
	}
	if !acquired {
		t.Error("expected lock to be acquired")
	}
	if owner != "" {
		t.Errorf("currentOwner should be empty on success, got %q", owner)
	}

	// Verify lock is persisted
	got, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.LockOwner == nil || *got.LockOwner != "harness-1" {
		t.Errorf("LockOwner = %v, want harness-1", got.LockOwner)
	}
}

func TestAcquireSessionLock_Contention(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	expires := time.Now().UTC().Add(5 * time.Minute)
	acquired, _, err := store.AcquireSessionLock(ctx, "sess-1", "harness-1", expires)
	if err != nil {
		t.Fatalf("AcquireSessionLock (1st): %v", err)
	}
	if !acquired {
		t.Fatal("first acquire should succeed")
	}

	// Second harness tries to acquire — should fail
	acquired2, currentOwner, err := store.AcquireSessionLock(ctx, "sess-1", "harness-2", expires)
	if err != nil {
		t.Fatalf("AcquireSessionLock (2nd): %v", err)
	}
	if acquired2 {
		t.Error("second acquire should fail due to contention")
	}
	if currentOwner != "harness-1" {
		t.Errorf("currentOwner = %q, want %q", currentOwner, "harness-1")
	}
}

func TestAcquireSessionLock_SelfReacquire(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	expires := time.Now().UTC().Add(5 * time.Minute)
	acquired, _, err := store.AcquireSessionLock(ctx, "sess-1", "harness-1", expires)
	if err != nil {
		t.Fatalf("AcquireSessionLock (1st): %v", err)
	}
	if !acquired {
		t.Fatal("first acquire should succeed")
	}

	// Same owner re-acquires (extend lock) — should succeed per A-02
	newExpires := time.Now().UTC().Add(10 * time.Minute)
	acquired2, _, err := store.AcquireSessionLock(ctx, "sess-1", "harness-1", newExpires)
	if err != nil {
		t.Fatalf("AcquireSessionLock (re-acquire): %v", err)
	}
	if !acquired2 {
		t.Error("self re-acquire should succeed")
	}
}

func TestAcquireSessionLock_ExpiredLock(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Acquire with an already-expired time
	pastExpires := time.Now().UTC().Add(-1 * time.Minute)
	acquired, _, err := store.AcquireSessionLock(ctx, "sess-1", "harness-1", pastExpires)
	if err != nil {
		t.Fatalf("AcquireSessionLock (1st): %v", err)
	}
	if !acquired {
		t.Fatal("first acquire should succeed")
	}

	// Second harness tries — should succeed because lock is expired
	futureExpires := time.Now().UTC().Add(5 * time.Minute)
	acquired2, _, err := store.AcquireSessionLock(ctx, "sess-1", "harness-2", futureExpires)
	if err != nil {
		t.Fatalf("AcquireSessionLock (2nd): %v", err)
	}
	if !acquired2 {
		t.Error("acquire over expired lock should succeed")
	}

	// Verify new owner
	got, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.LockOwner == nil || *got.LockOwner != "harness-2" {
		t.Errorf("LockOwner = %v, want harness-2", got.LockOwner)
	}
}

func TestReleaseSessionLock(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	expires := time.Now().UTC().Add(5 * time.Minute)
	store.AcquireSessionLock(ctx, "sess-1", "harness-1", expires)

	if err := store.ReleaseSessionLock(ctx, "sess-1", "harness-1"); err != nil {
		t.Fatalf("ReleaseSessionLock: %v", err)
	}

	got, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.LockOwner != nil {
		t.Errorf("LockOwner should be nil after release, got %v", got.LockOwner)
	}
}

func TestForceReleaseSessionLock(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	expires := time.Now().UTC().Add(5 * time.Minute)
	store.AcquireSessionLock(ctx, "sess-1", "harness-1", expires)

	// Force release by admin (no owner check)
	if err := store.ForceReleaseSessionLock(ctx, "sess-1"); err != nil {
		t.Fatalf("ForceReleaseSessionLock: %v", err)
	}

	got, err := store.GetSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.LockOwner != nil {
		t.Errorf("LockOwner should be nil after force release, got %v", got.LockOwner)
	}
}

// --- Turn Tests ---

func TestCreateTurn(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	turn := &Turn{
		ID:            "turn-1",
		SessionID:     "sess-1",
		Sequence:      1,
		Role:          "user",
		Content:       json.RawMessage(`"hello"`),
		Model:         "claude-3",
		Provider:      "anthropic",
		InputTokens:   100,
		OutputTokens:  50,
		Cost:          0.01,
		LatencyMs:     200,
		ToolCalls:     json.RawMessage(`[]`),
		FilesTouched:  []string{"/foo.go"},
		FilesModified: []string{"/foo.go"},
		CreatedAt:     time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := store.CreateTurn(ctx, turn); err != nil {
		t.Fatalf("CreateTurn: %v", err)
	}

	got, err := store.GetTurn(ctx, "turn-1")
	if err != nil {
		t.Fatalf("GetTurn: %v", err)
	}
	if got == nil {
		t.Fatal("GetTurn returned nil")
	}
	if got.Role != "user" {
		t.Errorf("Role = %q, want %q", got.Role, "user")
	}
	if got.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", got.InputTokens)
	}
	if len(got.FilesTouched) != 1 || got.FilesTouched[0] != "/foo.go" {
		t.Errorf("FilesTouched = %v, want [\"/foo.go\"]", got.FilesTouched)
	}
}

func TestGetTurn_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	got, err := store.GetTurn(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetTurn: %v", err)
	}
	if got != nil {
		t.Error("GetTurn should return nil for nonexistent turn")
	}
}

func TestListTurns_OrderedBySequence(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Insert turns out of order
	for _, seq := range []int{3, 1, 2} {
		turn := &Turn{
			ID:        "turn-" + string(rune('0'+seq)),
			SessionID: "sess-1",
			Sequence:  seq,
			Role:      "user",
			Content:   json.RawMessage(`"msg"`),
			ToolCalls: json.RawMessage(`[]`),
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		}
		if err := store.CreateTurn(ctx, turn); err != nil {
			t.Fatalf("CreateTurn(%d): %v", seq, err)
		}
	}

	turns, err := store.ListTurns(ctx, "sess-1")
	if err != nil {
		t.Fatalf("ListTurns: %v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("ListTurns returned %d turns, want 3", len(turns))
	}
	for i, turn := range turns {
		if turn.Sequence != i+1 {
			t.Errorf("turns[%d].Sequence = %d, want %d", i, turn.Sequence, i+1)
		}
	}
}

func TestTurnCompaction(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	turn := &Turn{
		ID:               "turn-compact",
		SessionID:        "sess-1",
		Sequence:         1,
		Role:             "assistant",
		Content:          json.RawMessage(`"compacted content"`),
		ToolCalls:        json.RawMessage(`[]`),
		Compacted:        true,
		CompactedSummary: "summarized 3 turns",
		OriginalTurns:    []int{1, 2, 3},
		CreatedAt:        time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := store.CreateTurn(ctx, turn); err != nil {
		t.Fatalf("CreateTurn: %v", err)
	}

	got, err := store.GetTurn(ctx, "turn-compact")
	if err != nil {
		t.Fatalf("GetTurn: %v", err)
	}
	if !got.Compacted {
		t.Error("Compacted should be true")
	}
	if got.CompactedSummary != "summarized 3 turns" {
		t.Errorf("CompactedSummary = %q", got.CompactedSummary)
	}
	if len(got.OriginalTurns) != 3 {
		t.Errorf("OriginalTurns len = %d, want 3", len(got.OriginalTurns))
	}
}

// --- Session Summaries ---

func TestSessionSummaries(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	s.TotalCost = 1.23
	s.ContextPct = 0.5
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Add turns for count
	for i := 1; i <= 3; i++ {
		turn := &Turn{
			ID:        "turn-" + string(rune('0'+i)),
			SessionID: "sess-1",
			Sequence:  i,
			Role:      "user",
			Content:   json.RawMessage(`"msg"`),
			ToolCalls: json.RawMessage(`[]`),
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		}
		if err := store.CreateTurn(ctx, turn); err != nil {
			t.Fatalf("CreateTurn: %v", err)
		}
	}

	summaries, err := store.SessionSummaries(ctx, SessionFilter{UserID: "user-1"})
	if err != nil {
		t.Fatalf("SessionSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("SessionSummaries returned %d, want 1", len(summaries))
	}
	if summaries[0].TurnCount != 3 {
		t.Errorf("TurnCount = %d, want 3", summaries[0].TurnCount)
	}
	if summaries[0].TotalCost != 1.23 {
		t.Errorf("TotalCost = %v, want 1.23", summaries[0].TotalCost)
	}
}

func TestSessionSummaries_RequiresUserID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.SessionSummaries(ctx, SessionFilter{})
	if err != ErrUserIDRequired {
		t.Errorf("SessionSummaries without UserID: err = %v, want ErrUserIDRequired", err)
	}
}

// --- Session Events ---

func TestAppendAndListServerEvents(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	evt := &ServerSessionEvent{
		SessionID: "sess-1",
		Type:      "auto_compact",
		Detail:    "freed 500 tokens",
		Payload:   json.RawMessage(`{"freed_tokens": 500}`),
		At:        time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := store.AppendServerEvent(ctx, evt); err != nil {
		t.Fatalf("AppendServerEvent: %v", err)
	}

	events, err := store.ListServerEvents(ctx, "sess-1")
	if err != nil {
		t.Fatalf("ListServerEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ListServerEvents returned %d events, want 1", len(events))
	}
	if events[0].Type != "auto_compact" {
		t.Errorf("event Type = %q, want %q", events[0].Type, "auto_compact")
	}
	if events[0].ID == 0 {
		t.Error("event ID should be auto-assigned")
	}
}

// --- Files Touched / Modified ---

func TestSessionFilesTouched(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	for i, files := range [][]string{
		{"/a.go", "/b.go"},
		{"/b.go", "/c.go"},
	} {
		turn := &Turn{
			ID:           "turn-" + string(rune('1'+i)),
			SessionID:    "sess-1",
			Sequence:     i + 1,
			Role:         "user",
			Content:      json.RawMessage(`"msg"`),
			ToolCalls:    json.RawMessage(`[]`),
			FilesTouched: files,
			CreatedAt:    time.Now().UTC().Truncate(time.Millisecond),
		}
		if err := store.CreateTurn(ctx, turn); err != nil {
			t.Fatalf("CreateTurn: %v", err)
		}
	}

	touched, err := store.SessionFilesTouched(ctx, "sess-1")
	if err != nil {
		t.Fatalf("SessionFilesTouched: %v", err)
	}
	// Should return distinct files: /a.go, /b.go, /c.go
	if len(touched) != 3 {
		t.Errorf("SessionFilesTouched returned %d files, want 3: %v", len(touched), touched)
	}
}

func TestSessionFilesModified(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	turn := &Turn{
		ID:            "turn-1",
		SessionID:     "sess-1",
		Sequence:      1,
		Role:          "user",
		Content:       json.RawMessage(`"msg"`),
		ToolCalls:     json.RawMessage(`[]`),
		FilesModified: []string{"/x.go", "/y.go"},
		CreatedAt:     time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := store.CreateTurn(ctx, turn); err != nil {
		t.Fatalf("CreateTurn: %v", err)
	}

	modified, err := store.SessionFilesModified(ctx, "sess-1")
	if err != nil {
		t.Fatalf("SessionFilesModified: %v", err)
	}
	if len(modified) != 2 {
		t.Errorf("SessionFilesModified returned %d files, want 2", len(modified))
	}
}

// --- Foreign Key Pragma ---

func TestForeignKeys_OnAllPoolConnections(t *testing.T) {
	store := newTestStore(t)

	store.db.SetMaxOpenConns(4)

	// Force multiple connections and verify each has FK enabled
	for i := 0; i < 4; i++ {
		var fk int
		if err := store.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatalf("PRAGMA foreign_keys query %d: %v", i, err)
		}
		if fk != 1 {
			t.Errorf("connection %d: foreign_keys = %d, want 1", i, fk)
		}
	}
}
