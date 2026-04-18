package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// handleHistoryAppend records a user-typed entry in cross-session
// command history per WU-091. The server also auto-appends on every
// turn.submit (in handleTurnSubmit); this method covers unsent drafts.
func handleHistoryAppend(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.HistoryAppend
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode history.append: " + err.Error()}
	}
	if req.Content == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "content is required"}
	}
	entry := &storage.CommandHistoryEntry{
		UserID:    conn.UserID(),
		Project:   conn.Capabilities().ProjectContext().Root,
		Content:   req.Content,
		CreatedAt: time.Now().UTC(),
	}
	if req.SessionID != "" {
		sid := req.SessionID
		entry.SessionID = &sid
	}
	if err := conn.server.store.AppendCommandHistory(ctx, entry); err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "append history: " + err.Error()}
	}
	return &protocol.HistoryAppendResponse{Accepted: true}, nil
}

// handleHistoryList returns history entries scoped by user / project /
// session per the request's Scope field. Pagination uses an opaque
// before-cursor encoding the timestamp + id of the last entry the
// client received.
func handleHistoryList(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.HistoryList
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode history.list: " + err.Error()}
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}

	filter := storage.CommandHistoryFilter{
		UserID: conn.UserID(),
		Limit:  limit + 1, // pull one extra to compute HasMore
	}
	switch req.Scope {
	case "user":
		// no extra filter
	case "project":
		filter.Project = conn.Capabilities().ProjectContext().Root
	case "session":
		filter.SessionID = conn.SessionID()
		if filter.SessionID == "" {
			return nil, &TransportError{Code: CodeInvalidParams, Message: "scope=session requires an active session bound to the connection"}
		}
	case "":
		filter.Project = conn.Capabilities().ProjectContext().Root
	default:
		return nil, &TransportError{Code: CodeInvalidParams, Message: fmt.Sprintf("unknown scope %q", req.Scope)}
	}

	if req.Before != "" {
		ts, id, err := decodeBeforeCursor(req.Before)
		if err != nil {
			return nil, &TransportError{Code: CodeInvalidParams, Message: "invalid before cursor"}
		}
		filter.Before = &ts
		filter.BeforeID = &id
	}

	entries, err := conn.server.store.ListCommandHistory(ctx, filter)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "list history: " + err.Error()}
	}

	hasMore := false
	if len(entries) > limit {
		hasMore = true
		entries = entries[:limit]
	}

	out := make([]protocol.HistoryEntry, 0, len(entries))
	for _, e := range entries {
		he := protocol.HistoryEntry{
			Content:   e.Content,
			Timestamp: e.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		if e.SessionID != nil {
			he.SessionID = *e.SessionID
		}
		out = append(out, he)
	}

	resp := &protocol.HistoryListResponse{
		Entries: out,
		HasMore: hasMore,
	}
	if hasMore && len(entries) > 0 {
		last := entries[len(entries)-1]
		resp.Cursor = encodeBeforeCursor(last.CreatedAt, last.ID)
	}
	return resp, nil
}

// encodeBeforeCursor emits an opaque "TIMESTAMP|ID" cursor. The format
// is internal but stable across server restarts as long as the storage
// schema doesn't reassign IDs.
func encodeBeforeCursor(ts time.Time, id int64) string {
	return ts.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(id, 10)
}

// decodeBeforeCursor is the inverse of encodeBeforeCursor.
func decodeBeforeCursor(cursor string) (time.Time, int64, error) {
	parts := strings.SplitN(cursor, "|", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor timestamp: %w", err)
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor id: %w", err)
	}
	return ts, id, nil
}
