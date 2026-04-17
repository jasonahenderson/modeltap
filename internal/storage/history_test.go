package storage

import (
	"context"
	"testing"
	"time"
)

func TestAppendCommandHistory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	entry := &CommandHistoryEntry{
		UserID:    "user-1",
		Project:   "proj-a",
		Content:   "write tests for storage",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := store.AppendCommandHistory(ctx, entry); err != nil {
		t.Fatalf("AppendCommandHistory: %v", err)
	}
	if entry.ID == 0 {
		t.Error("ID should be auto-assigned after append")
	}
}

func TestListCommandHistory_UserScope(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	for i, content := range []string{"cmd-1", "cmd-2", "cmd-3"} {
		entry := &CommandHistoryEntry{
			UserID:    "user-1",
			Project:   "proj-a",
			Content:   content,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := store.AppendCommandHistory(ctx, entry); err != nil {
			t.Fatalf("AppendCommandHistory: %v", err)
		}
	}

	entries, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID: "user-1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListCommandHistory: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("ListCommandHistory returned %d entries, want 3", len(entries))
	}
	// Should be newest-first (DESC)
	if entries[0].Content != "cmd-3" {
		t.Errorf("entries[0].Content = %q, want %q", entries[0].Content, "cmd-3")
	}
}

func TestListCommandHistory_ProjectScope(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	for i, proj := range []string{"proj-a", "proj-b", "proj-a"} {
		entry := &CommandHistoryEntry{
			UserID:    "user-1",
			Project:   proj,
			Content:   "cmd-" + proj,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := store.AppendCommandHistory(ctx, entry); err != nil {
			t.Fatalf("AppendCommandHistory: %v", err)
		}
	}

	entries, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID:  "user-1",
		Project: "proj-a",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("ListCommandHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListCommandHistory(proj-a) returned %d entries, want 2", len(entries))
	}
}

func TestListCommandHistory_SessionScope(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create session for FK (even though no FK, create for consistency)
	s := makeSession("sess-1", "user-1", "proj-a")
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	sessID := "sess-1"
	for i, sid := range []*string{&sessID, nil, &sessID} {
		entry := &CommandHistoryEntry{
			UserID:    "user-1",
			Project:   "proj-a",
			SessionID: sid,
			Content:   "cmd",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := store.AppendCommandHistory(ctx, entry); err != nil {
			t.Fatalf("AppendCommandHistory: %v", err)
		}
	}

	entries, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID:    "user-1",
		SessionID: "sess-1",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListCommandHistory(session): %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListCommandHistory(session) returned %d entries, want 2", len(entries))
	}
}

func TestListCommandHistory_UserIsolation(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, uid := range []string{"user-1", "user-2"} {
		entry := &CommandHistoryEntry{
			UserID:    uid,
			Project:   "proj-a",
			Content:   "cmd-" + uid,
			CreatedAt: now,
		}
		if err := store.AppendCommandHistory(ctx, entry); err != nil {
			t.Fatalf("AppendCommandHistory: %v", err)
		}
	}

	entries, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID: "user-1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListCommandHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("user-1 should see only 1 entry, got %d", len(entries))
	}
	if entries[0].Content != "cmd-user-1" {
		t.Errorf("wrong entry for user-1: %q", entries[0].Content)
	}
}

func TestListCommandHistory_Pagination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	for i := 0; i < 5; i++ {
		entry := &CommandHistoryEntry{
			UserID:    "user-1",
			Project:   "proj-a",
			Content:   "cmd",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := store.AppendCommandHistory(ctx, entry); err != nil {
			t.Fatalf("AppendCommandHistory: %v", err)
		}
	}

	// First page: newest 2
	page1, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID: "user-1",
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("ListCommandHistory page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page 1 len = %d, want 2", len(page1))
	}

	// Second page: use cursor from last entry of page 1
	lastEntry := page1[len(page1)-1]
	cursor := lastEntry.CreatedAt
	cursorID := lastEntry.ID
	page2, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID:   "user-1",
		Limit:    2,
		Before:   &cursor,
		BeforeID: &cursorID,
	})
	if err != nil {
		t.Fatalf("ListCommandHistory page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page 2 len = %d, want 2", len(page2))
	}

	// Third page: should have 1 remaining
	lastEntry = page2[len(page2)-1]
	cursor = lastEntry.CreatedAt
	cursorID = lastEntry.ID
	page3, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID:   "user-1",
		Limit:    2,
		Before:   &cursor,
		BeforeID: &cursorID,
	})
	if err != nil {
		t.Fatalf("ListCommandHistory page 3: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("page 3 len = %d, want 1", len(page3))
	}
}

func TestCommandHistory_SurvivesSessionDelete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	s := makeSession("sess-1", "user-1", "proj-a")
	s.UpdatedAt = time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Millisecond)
	s.CreatedAt = s.UpdatedAt
	if err := store.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	sessID := "sess-1"
	entry := &CommandHistoryEntry{
		UserID:    "user-1",
		Project:   "proj-a",
		SessionID: &sessID,
		Content:   "my command",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := store.AppendCommandHistory(ctx, entry); err != nil {
		t.Fatalf("AppendCommandHistory: %v", err)
	}

	// Delete the session
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	_, err := store.DeleteSessionsBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteSessionsBefore: %v", err)
	}

	// History should survive (no FK cascade)
	entries, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID: "user-1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListCommandHistory after session delete: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("history should survive session delete, got %d entries", len(entries))
	}
}

func TestListCommandHistory_PaginationSameTimestamp(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// All entries at same timestamp — tie-breaking by ID per A-07
	now := time.Now().UTC().Truncate(time.Millisecond)
	for i := 0; i < 4; i++ {
		entry := &CommandHistoryEntry{
			UserID:    "user-1",
			Project:   "proj-a",
			Content:   "cmd",
			CreatedAt: now,
		}
		if err := store.AppendCommandHistory(ctx, entry); err != nil {
			t.Fatalf("AppendCommandHistory: %v", err)
		}
	}

	page1, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID: "user-1",
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page 1 len = %d, want 2", len(page1))
	}

	// Cursor from last in page 1
	last := page1[len(page1)-1]
	cursor := last.CreatedAt
	cursorID := last.ID
	page2, err := store.ListCommandHistory(ctx, CommandHistoryFilter{
		UserID:   "user-1",
		Limit:    2,
		Before:   &cursor,
		BeforeID: &cursorID,
	})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page 2 len = %d, want 2 (tie-break by ID)", len(page2))
	}
	// Ensure no overlap
	if page2[0].ID == page1[0].ID || page2[0].ID == page1[1].ID {
		t.Error("page 2 should not overlap with page 1")
	}
}
