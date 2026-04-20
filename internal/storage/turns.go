package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CreateTurn inserts a new turn record.
func (s *SQLiteStore) CreateTurn(ctx context.Context, t *Turn) error {
	if t.Content == nil {
		t.Content = json.RawMessage(`""`)
	}
	if t.ToolCalls == nil {
		t.ToolCalls = json.RawMessage(`[]`)
	}
	if t.FilesTouched == nil {
		t.FilesTouched = []string{}
	}
	if t.FilesModified == nil {
		t.FilesModified = []string{}
	}
	if t.OriginalTurns == nil {
		t.OriginalTurns = []int{}
	}

	filesTouchedJSON, err := json.Marshal(t.FilesTouched)
	if err != nil {
		return fmt.Errorf("marshalling files_touched: %w", err)
	}
	filesModifiedJSON, err := json.Marshal(t.FilesModified)
	if err != nil {
		return fmt.Errorf("marshalling files_modified: %w", err)
	}
	originalTurnsJSON, err := json.Marshal(t.OriginalTurns)
	if err != nil {
		return fmt.Errorf("marshalling original_turns: %w", err)
	}

	compacted := 0
	if t.Compacted {
		compacted = 1
	}

	const query = `
INSERT INTO turns (
	id, session_id, sequence, role, content, model, provider,
	input_tokens, output_tokens, cost, latency_ms,
	tool_calls, files_touched, files_modified,
	compacted, compacted_summary, original_turns, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.ExecContext(ctx, query,
		t.ID, t.SessionID, t.Sequence, t.Role, string(t.Content), t.Model, t.Provider,
		t.InputTokens, t.OutputTokens, t.Cost, t.LatencyMs,
		string(t.ToolCalls), string(filesTouchedJSON), string(filesModifiedJSON),
		compacted, t.CompactedSummary, string(originalTurnsJSON),
		t.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("inserting turn: %w", err)
	}
	return nil
}

// GetTurn retrieves a single turn by ID.
func (s *SQLiteStore) GetTurn(ctx context.Context, id string) (*Turn, error) {
	const query = `
SELECT id, session_id, sequence, role, content, model, provider,
       input_tokens, output_tokens, cost, latency_ms,
       tool_calls, files_touched, files_modified,
       compacted, compacted_summary, original_turns, created_at
FROM turns WHERE id = ?`

	var t Turn
	var createdAt string
	var content, toolCalls string
	var filesTouched, filesModified, originalTurns string
	var compacted int

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.SessionID, &t.Sequence, &t.Role, &content, &t.Model, &t.Provider,
		&t.InputTokens, &t.OutputTokens, &t.Cost, &t.LatencyMs,
		&toolCalls, &filesTouched, &filesModified,
		&compacted, &t.CompactedSummary, &originalTurns, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying turn %s: %w", id, err)
	}

	t.Content = json.RawMessage(content)
	t.ToolCalls = json.RawMessage(toolCalls)
	t.Compacted = compacted != 0
	t.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at for turn %s: %w", id, err)
	}

	if err := json.Unmarshal([]byte(filesTouched), &t.FilesTouched); err != nil {
		t.FilesTouched = []string{}
	}
	if err := json.Unmarshal([]byte(filesModified), &t.FilesModified); err != nil {
		t.FilesModified = []string{}
	}
	if err := json.Unmarshal([]byte(originalTurns), &t.OriginalTurns); err != nil {
		t.OriginalTurns = []int{}
	}

	return &t, nil
}

// MaxTurnsPerListCall is the hard ceiling on ListTurns output — a
// long-running session with tens of thousands of turns would
// otherwise stream everything into a slice and OOM the caller
// (WU-094 H-8). Callers needing more should paginate via the
// sequence cursor. Tuned generous: 2000 turns × ~1 KiB/turn ≈ 2 MiB.
const MaxTurnsPerListCall = 2000

// ListTurns returns turns for a session, ordered by sequence,
// capped at MaxTurnsPerListCall. Compacted sessions always fit
// well under the cap; only pathologically long uncompacted
// sessions will trim, and those already demand a /compact pass.
func (s *SQLiteStore) ListTurns(ctx context.Context, sessionID string) ([]Turn, error) {
	const query = `
SELECT id, session_id, sequence, role, content, model, provider,
       input_tokens, output_tokens, cost, latency_ms,
       tool_calls, files_touched, files_modified,
       compacted, compacted_summary, original_turns, created_at
FROM turns WHERE session_id = ? ORDER BY sequence LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, sessionID, MaxTurnsPerListCall)
	if err != nil {
		return nil, fmt.Errorf("listing turns: %w", err)
	}
	defer rows.Close()

	var results []Turn
	for rows.Next() {
		var t Turn
		var createdAt string
		var content, toolCalls string
		var filesTouched, filesModified, originalTurns string
		var compacted int

		if err := rows.Scan(
			&t.ID, &t.SessionID, &t.Sequence, &t.Role, &content, &t.Model, &t.Provider,
			&t.InputTokens, &t.OutputTokens, &t.Cost, &t.LatencyMs,
			&toolCalls, &filesTouched, &filesModified,
			&compacted, &t.CompactedSummary, &originalTurns, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scanning turn row: %w", err)
		}

		t.Content = json.RawMessage(content)
		t.ToolCalls = json.RawMessage(toolCalls)
		t.Compacted = compacted != 0
		t.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}

		if err := json.Unmarshal([]byte(filesTouched), &t.FilesTouched); err != nil {
			t.FilesTouched = []string{}
		}
		if err := json.Unmarshal([]byte(filesModified), &t.FilesModified); err != nil {
			t.FilesModified = []string{}
		}
		if err := json.Unmarshal([]byte(originalTurns), &t.OriginalTurns); err != nil {
			t.OriginalTurns = []int{}
		}

		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating turn rows: %w", err)
	}
	return results, nil
}

// DeleteTurn removes a turn by id. Used by the compaction pipeline
// (WU-061) to delete the originals after replacing them with a
// summary row. Idempotent: deleting an absent id is not an error.
func (s *SQLiteStore) DeleteTurn(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM turns WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting turn %s: %w", id, err)
	}
	return nil
}
