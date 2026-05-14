package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

func TestHistoryAppend_Success(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)

	params, _ := json.Marshal(&protocol.HistoryAppend{
		Content:   "git status",
		SessionID: "sess-x",
	})
	resp, err := handleHistoryAppend(context.Background(), c, params)
	if err != nil {
		t.Fatalf("history.append: %v", err)
	}
	if !resp.(*protocol.HistoryAppendResponse).Accepted {
		t.Errorf("Accepted should be true")
	}

	got, err := srv.store.ListCommandHistory(context.Background(), storage.CommandHistoryFilter{
		UserID: SoloUserID, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Content != "git status" {
		t.Errorf("storage = %+v", got)
	}
}

func TestHistoryAppend_MissingContent(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.HistoryAppend{})
	_, err := handleHistoryAppend(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected error for empty content")
	}
}

func TestHistoryList_ScopeUser(t *testing.T) {
	srv := newServerWithRealStore(t)
	for i, content := range []string{"one", "two", "three"} {
		_ = srv.store.AppendCommandHistory(context.Background(), &storage.CommandHistoryEntry{
			UserID:    SoloUserID,
			Project:   "/tmp/proj",
			Content:   content,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Millisecond).UTC(),
		})
	}
	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.HistoryList{Scope: "user", Limit: 10})

	resp, err := handleHistoryList(context.Background(), c, params)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	hr := resp.(*protocol.HistoryListResponse)
	if len(hr.Entries) != 3 {
		t.Errorf("Entries = %d, want 3", len(hr.Entries))
	}
	if hr.HasMore {
		t.Errorf("HasMore should be false")
	}
}

func TestHistoryList_ScopeSession_RequiresActive(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	// no SetSessionID
	params, _ := json.Marshal(&protocol.HistoryList{Scope: "session", Limit: 10})
	_, err := handleHistoryList(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected error for scope=session without bound session")
	}
}

func TestHistoryList_HasMoreAndCursor(t *testing.T) {
	srv := newServerWithRealStore(t)
	for i := 0; i < 5; i++ {
		_ = srv.store.AppendCommandHistory(context.Background(), &storage.CommandHistoryEntry{
			UserID: SoloUserID, Project: "/tmp/proj",
			Content:   "cmd-" + string(rune('a'+i)),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Millisecond).UTC(),
		})
	}
	c := newReadyConnection(t, srv)

	params, _ := json.Marshal(&protocol.HistoryList{Scope: "project", Limit: 2})
	resp, err := handleHistoryList(context.Background(), c, params)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	hr := resp.(*protocol.HistoryListResponse)
	if !hr.HasMore || hr.Cursor == "" {
		t.Errorf("expected HasMore=true with cursor, got %+v", hr)
	}
}

func TestHistoryList_BeforeCursor_Pagination(t *testing.T) {
	srv := newServerWithRealStore(t)
	for i := 0; i < 4; i++ {
		_ = srv.store.AppendCommandHistory(context.Background(), &storage.CommandHistoryEntry{
			UserID: SoloUserID, Project: "/tmp/proj",
			Content:   "cmd-" + string(rune('a'+i)),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Millisecond).UTC(),
		})
	}
	c := newReadyConnection(t, srv)

	first, err := handleHistoryList(context.Background(), c, jsonMust(&protocol.HistoryList{Scope: "project", Limit: 2}))
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	hr1 := first.(*protocol.HistoryListResponse)
	if hr1.Cursor == "" {
		t.Fatalf("expected cursor for further pagination")
	}

	second, err := handleHistoryList(context.Background(), c, jsonMust(&protocol.HistoryList{
		Scope: "project", Limit: 10, Before: hr1.Cursor,
	}))
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	hr2 := second.(*protocol.HistoryListResponse)
	if len(hr2.Entries) == 0 {
		t.Errorf("expected more entries after cursor; got %d", len(hr2.Entries))
	}
}

func TestHistoryList_InvalidScope(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.HistoryList{Scope: "wrong", Limit: 5})
	_, err := handleHistoryList(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected error for unknown scope")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeInvalidParams {
		t.Errorf("expected CodeInvalidParams, got %T %v", err, err)
	}
}

func TestEncodeDecodeBeforeCursor_RoundTrip(t *testing.T) {
	now := time.Now().UTC()
	cursor := encodeBeforeCursor(now, 42)
	ts, id, err := decodeBeforeCursor(cursor)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !ts.Equal(now) || id != 42 {
		t.Errorf("round-trip = (%v, %d)", ts, id)
	}
}

func jsonMust(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
