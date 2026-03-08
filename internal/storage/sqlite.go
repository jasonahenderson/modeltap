package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath, enables WAL
// mode, and runs schema migrations. The dbPath may use "~" for the user's
// home directory. Use ":memory:" for an in-memory database.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dbPath = expandHome(dbPath)

	// Ensure the parent directory exists for file-based databases.
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Verify the connection works.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	s := &SQLiteStore{db: db}

	if err := s.enableWAL(); err != nil {
		db.Close()
		return nil, err
	}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// expandHome expands a leading ~ to the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// enableWAL sets the journal mode to WAL for better concurrent read performance.
func (s *SQLiteStore) enableWAL() error {
	_, err := s.db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("enabling WAL mode: %w", err)
	}
	return nil
}

// migrate creates the schema if it does not already exist.
func (s *SQLiteStore) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS requests (
	id               TEXT PRIMARY KEY,
	timestamp        TEXT NOT NULL,
	provider         TEXT NOT NULL DEFAULT '',
	model            TEXT NOT NULL DEFAULT '',
	method           TEXT NOT NULL DEFAULT '',
	url              TEXT NOT NULL DEFAULT '',
	request_headers  TEXT NOT NULL DEFAULT '',
	request_body     TEXT NOT NULL DEFAULT '',
	response_status  INTEGER NOT NULL DEFAULT 0,
	response_headers TEXT NOT NULL DEFAULT '',
	response_body    TEXT NOT NULL DEFAULT '',
	input_tokens     INTEGER NOT NULL DEFAULT 0,
	output_tokens    INTEGER NOT NULL DEFAULT 0,
	latency_ms       INTEGER NOT NULL DEFAULT 0,
	estimated_cost_usd REAL NOT NULL DEFAULT 0.0
);

CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp);
CREATE INDEX IF NOT EXISTS idx_requests_provider  ON requests(provider);
CREATE INDEX IF NOT EXISTS idx_requests_model     ON requests(model);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("running schema migration: %w", err)
	}
	return nil
}

// SaveRequest inserts a request record. If req.ID is empty, a new UUID is
// generated.
func (s *SQLiteStore) SaveRequest(ctx context.Context, req *Request) error {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now().UTC()
	}

	const query = `
INSERT INTO requests (
	id, timestamp, provider, model, method, url,
	request_headers, request_body,
	response_status, response_headers, response_body,
	input_tokens, output_tokens, latency_ms, estimated_cost_usd
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		req.ID,
		req.Timestamp.Format(time.RFC3339Nano),
		req.Provider,
		req.Model,
		req.Method,
		req.URL,
		req.RequestHeaders,
		req.RequestBody,
		req.ResponseStatus,
		req.ResponseHeaders,
		req.ResponseBody,
		req.InputTokens,
		req.OutputTokens,
		req.LatencyMs,
		req.EstimatedCostUSD,
	)
	if err != nil {
		return fmt.Errorf("inserting request: %w", err)
	}
	return nil
}

// GetRequest retrieves a single request by ID.
func (s *SQLiteStore) GetRequest(ctx context.Context, id string) (*Request, error) {
	const query = `
SELECT id, timestamp, provider, model, method, url,
       request_headers, request_body,
       response_status, response_headers, response_body,
       input_tokens, output_tokens, latency_ms, estimated_cost_usd
FROM requests WHERE id = ?`

	var r Request
	var ts string
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&r.ID, &ts, &r.Provider, &r.Model, &r.Method, &r.URL,
		&r.RequestHeaders, &r.RequestBody,
		&r.ResponseStatus, &r.ResponseHeaders, &r.ResponseBody,
		&r.InputTokens, &r.OutputTokens, &r.LatencyMs, &r.EstimatedCostUSD,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying request %s: %w", id, err)
	}

	r.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, fmt.Errorf("parsing timestamp for request %s: %w", id, err)
	}
	return &r, nil
}

// buildFilterQuery constructs a WHERE clause and arguments from a ListFilter.
func buildFilterQuery(filter ListFilter) (string, []any) {
	var conditions []string
	var args []any

	if filter.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.Since != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, filter.Since.Format(time.RFC3339Nano))
	}
	if filter.Until != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, filter.Until.Format(time.RFC3339Nano))
	}
	if filter.StatusCode != nil {
		conditions = append(conditions, "response_status = ?")
		args = append(args, *filter.StatusCode)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	return where, args
}

// ListRequests returns requests matching the given filter, ordered by
// timestamp descending (newest first).
func (s *SQLiteStore) ListRequests(ctx context.Context, filter ListFilter) ([]Request, error) {
	where, args := buildFilterQuery(filter)

	query := `
SELECT id, timestamp, provider, model, method, url,
       request_headers, request_body,
       response_status, response_headers, response_body,
       input_tokens, output_tokens, latency_ms, estimated_cost_usd
FROM requests` + where + ` ORDER BY timestamp DESC`

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
		return nil, fmt.Errorf("listing requests: %w", err)
	}
	defer rows.Close()

	var results []Request
	for rows.Next() {
		var r Request
		var ts string
		if err := rows.Scan(
			&r.ID, &ts, &r.Provider, &r.Model, &r.Method, &r.URL,
			&r.RequestHeaders, &r.RequestBody,
			&r.ResponseStatus, &r.ResponseHeaders, &r.ResponseBody,
			&r.InputTokens, &r.OutputTokens, &r.LatencyMs, &r.EstimatedCostUSD,
		); err != nil {
			return nil, fmt.Errorf("scanning request row: %w", err)
		}
		r.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmt.Errorf("parsing timestamp: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating request rows: %w", err)
	}
	return results, nil
}

// CountRequests returns the number of requests matching the given filter.
func (s *SQLiteStore) CountRequests(ctx context.Context, filter ListFilter) (int64, error) {
	where, args := buildFilterQuery(filter)

	query := "SELECT COUNT(*) FROM requests" + where

	var count int64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting requests: %w", err)
	}
	return count, nil
}

// DeleteBefore removes all requests with a timestamp before the given time
// and returns the number of deleted rows.
func (s *SQLiteStore) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	const query = "DELETE FROM requests WHERE timestamp < ?"

	result, err := s.db.ExecContext(ctx, query, before.Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("deleting old requests: %w", err)
	}
	return result.RowsAffected()
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
