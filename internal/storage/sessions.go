package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CreateSession inserts a new session record.
func (s *SQLiteStore) CreateSession(ctx context.Context, sess *Session) error {
	if sess.RoutingOverrides == nil {
		sess.RoutingOverrides = json.RawMessage(`{}`)
	}
	if sess.PinnedItems == nil {
		sess.PinnedItems = json.RawMessage(`[]`)
	}
	if sess.CompactionState == nil {
		sess.CompactionState = json.RawMessage(`{}`)
	}

	const query = `
INSERT INTO sessions (
	id, user_id, project, summary, active_model, model_override,
	routing_overrides, pinned_items, compaction_state,
	total_cost, total_input_tokens, total_output_tokens, context_pct,
	status, lock_owner, lock_expires_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var lockExpiresAt *string
	if sess.LockExpiresAt != nil {
		s := sess.LockExpiresAt.UTC().Format(time.RFC3339Nano)
		lockExpiresAt = &s
	}

	_, err := s.db.ExecContext(ctx, query,
		sess.ID, sess.UserID, sess.Project, sess.Summary, sess.ActiveModel, sess.ModelOverride,
		string(sess.RoutingOverrides), string(sess.PinnedItems), string(sess.CompactionState),
		sess.TotalCost, sess.TotalInputTokens, sess.TotalOutputTokens, sess.ContextPct,
		sess.Status, sess.LockOwner, lockExpiresAt,
		sess.CreatedAt.UTC().Format(time.RFC3339Nano),
		sess.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("inserting session: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID.
func (s *SQLiteStore) GetSession(ctx context.Context, id string) (*Session, error) {
	const query = `
SELECT id, user_id, project, summary, active_model, model_override,
       routing_overrides, pinned_items, compaction_state,
       total_cost, total_input_tokens, total_output_tokens, context_pct,
       status, lock_owner, lock_expires_at, created_at, updated_at
FROM sessions WHERE id = ?`

	var sess Session
	var routingOverrides, pinnedItems, compactionState string
	var createdAt, updatedAt string
	var lockExpiresAt *string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&sess.ID, &sess.UserID, &sess.Project, &sess.Summary, &sess.ActiveModel, &sess.ModelOverride,
		&routingOverrides, &pinnedItems, &compactionState,
		&sess.TotalCost, &sess.TotalInputTokens, &sess.TotalOutputTokens, &sess.ContextPct,
		&sess.Status, &sess.LockOwner, &lockExpiresAt,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying session %s: %w", id, err)
	}

	sess.RoutingOverrides = json.RawMessage(routingOverrides)
	sess.PinnedItems = json.RawMessage(pinnedItems)
	sess.CompactionState = json.RawMessage(compactionState)

	sess.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at for session %s: %w", id, err)
	}
	sess.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing updated_at for session %s: %w", id, err)
	}
	if lockExpiresAt != nil {
		t, err := time.Parse(time.RFC3339Nano, *lockExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("parsing lock_expires_at for session %s: %w", id, err)
		}
		sess.LockExpiresAt = &t
	}

	return &sess, nil
}

// UpdateSession updates an existing session record.
func (s *SQLiteStore) UpdateSession(ctx context.Context, sess *Session) error {
	if sess.RoutingOverrides == nil {
		sess.RoutingOverrides = json.RawMessage(`{}`)
	}
	if sess.PinnedItems == nil {
		sess.PinnedItems = json.RawMessage(`[]`)
	}
	if sess.CompactionState == nil {
		sess.CompactionState = json.RawMessage(`{}`)
	}

	const query = `
UPDATE sessions SET
	user_id = ?, project = ?, summary = ?, active_model = ?, model_override = ?,
	routing_overrides = ?, pinned_items = ?, compaction_state = ?,
	total_cost = ?, total_input_tokens = ?, total_output_tokens = ?, context_pct = ?,
	status = ?, lock_owner = ?, lock_expires_at = ?, updated_at = ?
WHERE id = ?`

	var lockExpiresAt *string
	if sess.LockExpiresAt != nil {
		s := sess.LockExpiresAt.UTC().Format(time.RFC3339Nano)
		lockExpiresAt = &s
	}

	result, err := s.db.ExecContext(ctx, query,
		sess.UserID, sess.Project, sess.Summary, sess.ActiveModel, sess.ModelOverride,
		string(sess.RoutingOverrides), string(sess.PinnedItems), string(sess.CompactionState),
		sess.TotalCost, sess.TotalInputTokens, sess.TotalOutputTokens, sess.ContextPct,
		sess.Status, sess.LockOwner, lockExpiresAt,
		sess.UpdatedAt.UTC().Format(time.RFC3339Nano),
		sess.ID,
	)
	if err != nil {
		return fmt.Errorf("updating session: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// ListSessions returns sessions matching the filter, ordered by updated_at DESC.
func (s *SQLiteStore) ListSessions(ctx context.Context, filter SessionFilter) ([]Session, error) {
	if filter.UserID == "" {
		return nil, ErrUserIDRequired
	}
	if !ValidSessionStatuses[filter.Status] {
		return nil, ErrInvalidStatus
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "user_id = ?")
	args = append(args, filter.UserID)

	if filter.Project != "" {
		conditions = append(conditions, "project = ?")
		args = append(args, filter.Project)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Since != nil {
		conditions = append(conditions, "updated_at >= ?")
		args = append(args, filter.Since.UTC().Format(time.RFC3339Nano))
	}

	where := " WHERE " + strings.Join(conditions, " AND ")
	query := `
SELECT id, user_id, project, summary, active_model, model_override,
       routing_overrides, pinned_items, compaction_state,
       total_cost, total_input_tokens, total_output_tokens, context_pct,
       status, lock_owner, lock_expires_at, created_at, updated_at
FROM sessions` + where + ` ORDER BY updated_at DESC`

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var results []Session
	for rows.Next() {
		var sess Session
		var routingOverrides, pinnedItems, compactionState string
		var createdAt, updatedAt string
		var lockExpiresAt *string

		if err := rows.Scan(
			&sess.ID, &sess.UserID, &sess.Project, &sess.Summary, &sess.ActiveModel, &sess.ModelOverride,
			&routingOverrides, &pinnedItems, &compactionState,
			&sess.TotalCost, &sess.TotalInputTokens, &sess.TotalOutputTokens, &sess.ContextPct,
			&sess.Status, &sess.LockOwner, &lockExpiresAt,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning session row: %w", err)
		}

		sess.RoutingOverrides = json.RawMessage(routingOverrides)
		sess.PinnedItems = json.RawMessage(pinnedItems)
		sess.CompactionState = json.RawMessage(compactionState)

		sess.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}
		sess.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", err)
		}
		if lockExpiresAt != nil {
			t, err := time.Parse(time.RFC3339Nano, *lockExpiresAt)
			if err != nil {
				return nil, fmt.Errorf("parsing lock_expires_at: %w", err)
			}
			sess.LockExpiresAt = &t
		}

		results = append(results, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session rows: %w", err)
	}
	return results, nil
}

// DeleteSessionsBefore removes sessions updated before the given time.
// FK CASCADE deletes associated turns and session_events.
// Command history is NOT cascaded (no FK).
func (s *SQLiteStore) DeleteSessionsBefore(ctx context.Context, before time.Time) (int64, error) {
	const query = "DELETE FROM sessions WHERE updated_at < ?"
	result, err := s.db.ExecContext(ctx, query, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("deleting old sessions: %w", err)
	}
	return result.RowsAffected()
}

// AcquireSessionLock atomically acquires a session lock.
// Returns (true, "", nil) on success.
// Returns (false, currentOwner, nil) if the lock is held by another owner.
// Includes self-reacquire support per attention A-02.
func (s *SQLiteStore) AcquireSessionLock(ctx context.Context, sessionID, owner string, expiresAt time.Time) (bool, string, error) {
	now := time.Now().UTC()

	const acquireSQL = `
UPDATE sessions
   SET lock_owner = ?,
       lock_expires_at = ?,
       updated_at = ?
 WHERE id = ?
   AND (lock_owner IS NULL OR lock_expires_at < ? OR lock_owner = ?)`

	result, err := s.db.ExecContext(ctx, acquireSQL,
		owner,
		expiresAt.UTC().Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
		sessionID,
		now.Format(time.RFC3339Nano),
		owner,
	)
	if err != nil {
		return false, "", fmt.Errorf("acquiring session lock: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return false, "", fmt.Errorf("checking rows affected: %w", err)
	}

	if n == 1 {
		return true, "", nil
	}

	// Lock not acquired — find out who holds it.
	var currentOwner *string
	err = s.db.QueryRowContext(ctx, "SELECT lock_owner FROM sessions WHERE id = ?", sessionID).Scan(&currentOwner)
	if err == sql.ErrNoRows {
		return false, "", ErrSessionNotFound
	}
	if err != nil {
		return false, "", fmt.Errorf("querying current lock owner: %w", err)
	}

	ownerStr := ""
	if currentOwner != nil {
		ownerStr = *currentOwner
	}
	return false, ownerStr, nil
}

// ReleaseSessionLock releases a lock held by the specified owner.
func (s *SQLiteStore) ReleaseSessionLock(ctx context.Context, sessionID, owner string) error {
	now := time.Now().UTC()

	const query = `
UPDATE sessions
   SET lock_owner = NULL, lock_expires_at = NULL, updated_at = ?
 WHERE id = ? AND lock_owner = ?`

	_, err := s.db.ExecContext(ctx, query, now.Format(time.RFC3339Nano), sessionID, owner)
	if err != nil {
		return fmt.Errorf("releasing session lock: %w", err)
	}
	return nil
}

// ForceReleaseSessionLock releases a session lock regardless of owner (admin override).
func (s *SQLiteStore) ForceReleaseSessionLock(ctx context.Context, sessionID string) error {
	now := time.Now().UTC()

	const query = `
UPDATE sessions
   SET lock_owner = NULL, lock_expires_at = NULL, updated_at = ?
 WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, now.Format(time.RFC3339Nano), sessionID)
	if err != nil {
		return fmt.Errorf("force-releasing session lock: %w", err)
	}
	return nil
}

// SessionSummaries returns projected session summaries with turn counts.
func (s *SQLiteStore) SessionSummaries(ctx context.Context, filter SessionFilter) ([]SessionSummary, error) {
	if filter.UserID == "" {
		return nil, ErrUserIDRequired
	}
	if !ValidSessionStatuses[filter.Status] {
		return nil, ErrInvalidStatus
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "s.user_id = ?")
	args = append(args, filter.UserID)

	if filter.Project != "" {
		conditions = append(conditions, "s.project = ?")
		args = append(args, filter.Project)
	}
	if filter.Status != "" {
		conditions = append(conditions, "s.status = ?")
		args = append(args, filter.Status)
	}

	where := " WHERE " + strings.Join(conditions, " AND ")

	query := `
SELECT s.id, s.project, s.status, s.summary, s.updated_at, s.context_pct,
       s.total_cost, s.active_model, s.model_override, s.pinned_items,
       COALESCE((SELECT COUNT(*) FROM turns t WHERE t.session_id = s.id), 0) as turn_count,
       COALESCE((SELECT t2.compacted_summary FROM turns t2 WHERE t2.session_id = s.id ORDER BY t2.sequence DESC LIMIT 1), '') as last_turn_summary
FROM sessions s` + where + ` ORDER BY s.updated_at DESC`

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying session summaries: %w", err)
	}
	defer rows.Close()

	var results []SessionSummary
	for rows.Next() {
		var ss SessionSummary
		var updatedAt string
		var pinnedItems string

		if err := rows.Scan(
			&ss.ID, &ss.Project, &ss.Status, &ss.Summary, &updatedAt, &ss.ContextPct,
			&ss.TotalCost, &ss.Model, &ss.ModelOverride, &pinnedItems,
			&ss.TurnCount, &ss.LastTurnSummary,
		); err != nil {
			return nil, fmt.Errorf("scanning session summary row: %w", err)
		}

		ss.LastActive, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", err)
		}

		// Count pinned items from JSON array
		var pinned []json.RawMessage
		if err := json.Unmarshal([]byte(pinnedItems), &pinned); err == nil {
			ss.PinnedCount = len(pinned)
		}

		results = append(results, ss)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session summary rows: %w", err)
	}
	return results, nil
}

// AppendServerEvent appends a server event to a session's event log.
func (s *SQLiteStore) AppendServerEvent(ctx context.Context, e *ServerSessionEvent) error {
	const query = `
INSERT INTO session_events (session_id, type, detail, payload, at)
VALUES (?, ?, ?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query,
		e.SessionID, e.Type, e.Detail, string(e.Payload),
		e.At.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("inserting session event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting event ID: %w", err)
	}
	e.ID = id
	return nil
}

// ListServerEvents returns all events for a session, ordered by time.
func (s *SQLiteStore) ListServerEvents(ctx context.Context, sessionID string) ([]ServerSessionEvent, error) {
	const query = `
SELECT id, session_id, type, detail, payload, at
FROM session_events WHERE session_id = ? ORDER BY at`

	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("listing session events: %w", err)
	}
	defer rows.Close()

	var results []ServerSessionEvent
	for rows.Next() {
		var e ServerSessionEvent
		var payload string
		var at string

		if err := rows.Scan(&e.ID, &e.SessionID, &e.Type, &e.Detail, &payload, &at); err != nil {
			return nil, fmt.Errorf("scanning session event row: %w", err)
		}

		e.Payload = json.RawMessage(payload)
		e.At, err = time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return nil, fmt.Errorf("parsing event at: %w", err)
		}

		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating session event rows: %w", err)
	}
	return results, nil
}

// SessionFilesTouched returns distinct files touched across all turns in a session.
func (s *SQLiteStore) SessionFilesTouched(ctx context.Context, sessionID string) ([]string, error) {
	return s.sessionFiles(ctx, sessionID, "files_touched")
}

// SessionFilesModified returns distinct files modified across all turns in a session.
func (s *SQLiteStore) SessionFilesModified(ctx context.Context, sessionID string) ([]string, error) {
	return s.sessionFiles(ctx, sessionID, "files_modified")
}

func (s *SQLiteStore) sessionFiles(ctx context.Context, sessionID, column string) ([]string, error) {
	query := fmt.Sprintf("SELECT %s FROM turns WHERE session_id = ?", column)
	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("querying session %s: %w", column, err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var result []string

	for rows.Next() {
		var filesJSON string
		if err := rows.Scan(&filesJSON); err != nil {
			return nil, fmt.Errorf("scanning %s: %w", column, err)
		}
		var files []string
		if err := json.Unmarshal([]byte(filesJSON), &files); err != nil {
			continue // skip malformed JSON
		}
		for _, f := range files {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating %s rows: %w", column, err)
	}
	return result, nil
}
