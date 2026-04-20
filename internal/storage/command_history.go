package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MaxCommandHistoryPerListCall is the hard ceiling on ListCommandHistory
// output — a user's full history across months is potentially tens
// of thousands of rows; dumping everything on every /history scroll
// is a real DoS vector (WU-094 H-9). Clients page via Before /
// BeforeID to walk older entries.
const MaxCommandHistoryPerListCall = 500

// AppendCommandHistory inserts a command history entry.
func (s *SQLiteStore) AppendCommandHistory(ctx context.Context, entry *CommandHistoryEntry) error {
	const query = `
INSERT INTO command_history (user_id, project, session_id, content, created_at)
VALUES (?, ?, ?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query,
		entry.UserID, entry.Project, entry.SessionID, entry.Content,
		entry.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("inserting command history: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting history entry ID: %w", err)
	}
	entry.ID = id
	return nil
}

// ListCommandHistory returns command history entries matching the filter.
// Scope is determined implicitly per A-05:
//   - SessionID non-empty -> user + session scope
//   - Project non-empty -> user + project scope
//   - Otherwise -> user scope
//
// Pagination uses compound cursor (created_at, id) per A-07.
// Limit is clamped to MaxCommandHistoryPerListCall — callers that
// previously passed 0 (zero value) used to retrieve the full
// history; now they get a bounded page.
func (s *SQLiteStore) ListCommandHistory(ctx context.Context, filter CommandHistoryFilter) ([]CommandHistoryEntry, error) {
	var conditions []string
	var args []any

	// User ID always applied
	conditions = append(conditions, "user_id = ?")
	args = append(args, filter.UserID)

	// Scope: session > project > user
	if filter.SessionID != "" {
		conditions = append(conditions, "session_id = ?")
		args = append(args, filter.SessionID)
	} else if filter.Project != "" {
		conditions = append(conditions, "project = ?")
		args = append(args, filter.Project)
	}

	// Compound cursor pagination (A-07)
	if filter.Before != nil && filter.BeforeID != nil {
		conditions = append(conditions, "(created_at < ? OR (created_at = ? AND id < ?))")
		ts := filter.Before.UTC().Format(time.RFC3339Nano)
		args = append(args, ts, ts, *filter.BeforeID)
	} else if filter.Before != nil {
		conditions = append(conditions, "created_at < ?")
		args = append(args, filter.Before.UTC().Format(time.RFC3339Nano))
	}

	where := " WHERE " + strings.Join(conditions, " AND ")
	query := "SELECT id, user_id, project, session_id, content, created_at FROM command_history" +
		where + " ORDER BY created_at DESC, id DESC"

	// Default + cap the limit (WU-094 H-9). filter.Limit=0 used to
	// mean "unlimited" — a bug that dumped the user's full history
	// on any caller that left the field zero-valued.
	limit := filter.Limit
	if limit <= 0 {
		limit = MaxCommandHistoryPerListCall
	}
	if limit > MaxCommandHistoryPerListCall {
		limit = MaxCommandHistoryPerListCall
	}
	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing command history: %w", err)
	}
	defer rows.Close()

	var results []CommandHistoryEntry
	for rows.Next() {
		var e CommandHistoryEntry
		var createdAt string

		if err := rows.Scan(&e.ID, &e.UserID, &e.Project, &e.SessionID, &e.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning command history row: %w", err)
		}

		e.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}

		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating command history rows: %w", err)
	}
	return results, nil
}
