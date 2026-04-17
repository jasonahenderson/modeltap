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
// mode and foreign keys via DSN pragmas, and runs schema migrations.
// The dbPath may use "~" for the user's home directory.
// Use ":memory:" for an in-memory database.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dbPath = expandHome(dbPath)

	// Ensure the parent directory exists for file-based databases.
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating database directory: %w", err)
		}
	}

	// Use DSN pragmas so every pool connection gets foreign_keys=ON and WAL mode.
	dsn := dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Verify the connection works.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	s := &SQLiteStore{db: db}

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

// currentSchemaVersion reads the database's user_version pragma.
func (s *SQLiteStore) currentSchemaVersion() (int, error) {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	return version, nil
}

// migrate runs stepwise schema migrations from the current version to
// MaxKnownSchemaVersion. A downgrade guard rejects databases newer than
// this binary supports.
func (s *SQLiteStore) migrate() error {
	version, err := s.currentSchemaVersion()
	if err != nil {
		return err
	}

	// Downgrade guard: reject databases from a newer binary.
	if version > MaxKnownSchemaVersion {
		return fmt.Errorf("storage: database schema version %d is newer than this binary supports (max %d); upgrade modeltap",
			version, MaxKnownSchemaVersion)
	}

	// v0 (implicit) with existing tables is treated as v1 (retrofit).
	// v0 with no tables is a fresh database.
	if version < 1 {
		if err := s.migrateToV1(); err != nil {
			return err
		}
	}
	if version < 2 {
		if err := s.migrateToV2(); err != nil {
			return err
		}
	}
	return nil
}

// migrateToV1 creates the v1 schema (requests, hourly_usage, daily_usage).
// Uses CREATE TABLE IF NOT EXISTS for idempotence — existing v0.1.x DBs
// with user_version=0 but v1 tables already present are handled safely.
func (s *SQLiteStore) migrateToV1() error {
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

CREATE TABLE IF NOT EXISTS hourly_usage (
	hour TEXT NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	request_count INTEGER NOT NULL DEFAULT 0,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	estimated_cost_usd REAL NOT NULL DEFAULT 0,
	total_latency_ms INTEGER NOT NULL DEFAULT 0,
	error_count INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (hour, provider, model)
);

CREATE TABLE IF NOT EXISTS daily_usage (
	day TEXT NOT NULL,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	request_count INTEGER NOT NULL DEFAULT 0,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	estimated_cost_usd REAL NOT NULL DEFAULT 0,
	total_latency_ms INTEGER NOT NULL DEFAULT 0,
	error_count INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (day, provider, model)
);

PRAGMA user_version = 1;
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("running v1 schema migration: %w", err)
	}
	return nil
}

// migrateToV2 adds session, turn, event, and command history tables.
// Wrapped in a transaction so crash mid-migration rolls back atomically.
func (s *SQLiteStore) migrateToV2() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning v2 migration transaction: %w", err)
	}
	defer tx.Rollback()

	const v2Schema = `
CREATE TABLE sessions (
	id                   TEXT PRIMARY KEY,
	user_id              TEXT NOT NULL DEFAULT '',
	project              TEXT NOT NULL DEFAULT '',
	summary              TEXT NOT NULL DEFAULT '',
	active_model         TEXT NOT NULL DEFAULT '',
	model_override       TEXT,
	routing_overrides    TEXT NOT NULL DEFAULT '{}',
	pinned_items         TEXT NOT NULL DEFAULT '[]',
	compaction_state     TEXT NOT NULL DEFAULT '{}',
	total_cost           REAL NOT NULL DEFAULT 0.0,
	total_input_tokens   INTEGER NOT NULL DEFAULT 0,
	total_output_tokens  INTEGER NOT NULL DEFAULT 0,
	context_pct          REAL NOT NULL DEFAULT 0.0,
	status               TEXT NOT NULL DEFAULT 'active',
	lock_owner           TEXT,
	lock_expires_at      TEXT,
	created_at           TEXT NOT NULL,
	updated_at           TEXT NOT NULL
);

CREATE INDEX idx_sessions_user_id     ON sessions(user_id);
CREATE INDEX idx_sessions_user_active ON sessions(user_id, updated_at DESC);
CREATE INDEX idx_sessions_project     ON sessions(user_id, project);
CREATE INDEX idx_sessions_status      ON sessions(status);

CREATE TABLE turns (
	id                  TEXT PRIMARY KEY,
	session_id          TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	sequence            INTEGER NOT NULL,
	role                TEXT NOT NULL,
	content             TEXT NOT NULL DEFAULT '',
	model               TEXT NOT NULL DEFAULT '',
	provider            TEXT NOT NULL DEFAULT '',
	input_tokens        INTEGER NOT NULL DEFAULT 0,
	output_tokens       INTEGER NOT NULL DEFAULT 0,
	cost                REAL NOT NULL DEFAULT 0.0,
	latency_ms          INTEGER NOT NULL DEFAULT 0,
	tool_calls          TEXT NOT NULL DEFAULT '[]',
	files_touched       TEXT NOT NULL DEFAULT '[]',
	files_modified      TEXT NOT NULL DEFAULT '[]',
	compacted           INTEGER NOT NULL DEFAULT 0,
	compacted_summary   TEXT NOT NULL DEFAULT '',
	original_turns      TEXT NOT NULL DEFAULT '[]',
	created_at          TEXT NOT NULL
);

CREATE INDEX idx_turns_session_id  ON turns(session_id);
CREATE INDEX idx_turns_session_seq ON turns(session_id, sequence);

CREATE TABLE session_events (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	type       TEXT NOT NULL,
	detail     TEXT NOT NULL DEFAULT '',
	payload    TEXT NOT NULL DEFAULT '{}',
	at         TEXT NOT NULL
);

CREATE INDEX idx_session_events_session_id ON session_events(session_id, at);

CREATE TABLE command_history (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id    TEXT NOT NULL DEFAULT '',
	project    TEXT NOT NULL DEFAULT '',
	session_id TEXT,
	content    TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX idx_command_history_user_recent   ON command_history(user_id, created_at DESC);
CREATE INDEX idx_command_history_project       ON command_history(user_id, project, created_at DESC);
CREATE INDEX idx_command_history_session       ON command_history(user_id, session_id, created_at DESC);
`

	if _, err := tx.Exec(v2Schema); err != nil {
		return fmt.Errorf("running v2 schema migration: %w", err)
	}

	// Bump version inside the transaction so it's atomic.
	if _, err := tx.Exec("PRAGMA user_version = 2"); err != nil {
		return fmt.Errorf("setting user_version to 2: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing v2 migration: %w", err)
	}
	return nil
}

// SaveRequest inserts a request record and atomically updates the hourly and
// daily aggregation tables. If req.ID is empty, a new UUID is generated.
func (s *SQLiteStore) SaveRequest(ctx context.Context, req *Request) error {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	const insertRequest = `
INSERT INTO requests (
	id, timestamp, provider, model, method, url,
	request_headers, request_body,
	response_status, response_headers, response_body,
	input_tokens, output_tokens, latency_ms, estimated_cost_usd
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, insertRequest,
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

	// Determine if this is an error response.
	var errorIncrement int64
	if req.ResponseStatus >= 400 {
		errorIncrement = 1
	}

	// Upsert hourly aggregation.
	hour := req.Timestamp.UTC().Truncate(time.Hour).Format(time.RFC3339)
	const upsertHourly = `
INSERT INTO hourly_usage (hour, provider, model, request_count, input_tokens, output_tokens, estimated_cost_usd, total_latency_ms, error_count)
VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?)
ON CONFLICT (hour, provider, model) DO UPDATE SET
	request_count = request_count + 1,
	input_tokens = input_tokens + excluded.input_tokens,
	output_tokens = output_tokens + excluded.output_tokens,
	estimated_cost_usd = estimated_cost_usd + excluded.estimated_cost_usd,
	total_latency_ms = total_latency_ms + excluded.total_latency_ms,
	error_count = error_count + excluded.error_count`

	_, err = tx.ExecContext(ctx, upsertHourly,
		hour, req.Provider, req.Model,
		req.InputTokens, req.OutputTokens, req.EstimatedCostUSD, req.LatencyMs, errorIncrement,
	)
	if err != nil {
		return fmt.Errorf("upserting hourly usage: %w", err)
	}

	// Upsert daily aggregation.
	day := req.Timestamp.UTC().Format("2006-01-02")
	const upsertDaily = `
INSERT INTO daily_usage (day, provider, model, request_count, input_tokens, output_tokens, estimated_cost_usd, total_latency_ms, error_count)
VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?)
ON CONFLICT (day, provider, model) DO UPDATE SET
	request_count = request_count + 1,
	input_tokens = input_tokens + excluded.input_tokens,
	output_tokens = output_tokens + excluded.output_tokens,
	estimated_cost_usd = estimated_cost_usd + excluded.estimated_cost_usd,
	total_latency_ms = total_latency_ms + excluded.total_latency_ms,
	error_count = error_count + excluded.error_count`

	_, err = tx.ExecContext(ctx, upsertDaily,
		day, req.Provider, req.Model,
		req.InputTokens, req.OutputTokens, req.EstimatedCostUSD, req.LatencyMs, errorIncrement,
	)
	if err != nil {
		return fmt.Errorf("upserting daily usage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
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

// QueryHourlyMetrics returns aggregated hourly usage metrics matching the filter.
func (s *SQLiteStore) QueryHourlyMetrics(ctx context.Context, filter MetricsFilter) ([]UsageMetrics, error) {
	return s.queryMetrics(ctx, "hourly_usage", "hour", filter)
}

// QueryDailyMetrics returns aggregated daily usage metrics matching the filter.
func (s *SQLiteStore) QueryDailyMetrics(ctx context.Context, filter MetricsFilter) ([]UsageMetrics, error) {
	return s.queryMetrics(ctx, "daily_usage", "day", filter)
}

// queryMetrics is a shared helper for querying hourly or daily aggregation tables.
func (s *SQLiteStore) queryMetrics(ctx context.Context, table, periodCol string, filter MetricsFilter) ([]UsageMetrics, error) {
	var conditions []string
	var args []any

	if filter.Since != nil {
		conditions = append(conditions, periodCol+" >= ?")
		if periodCol == "hour" {
			args = append(args, filter.Since.UTC().Truncate(time.Hour).Format(time.RFC3339))
		} else {
			args = append(args, filter.Since.UTC().Format("2006-01-02"))
		}
	}
	if filter.Until != nil {
		conditions = append(conditions, periodCol+" <= ?")
		if periodCol == "hour" {
			args = append(args, filter.Until.UTC().Truncate(time.Hour).Format(time.RFC3339))
		} else {
			args = append(args, filter.Until.UTC().Format("2006-01-02"))
		}
	}
	if filter.Provider != "" {
		conditions = append(conditions, "provider = ?")
		args = append(args, filter.Provider)
	}
	if filter.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, filter.Model)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
SELECT %s, provider, model,
       SUM(request_count), SUM(input_tokens), SUM(output_tokens),
       SUM(estimated_cost_usd), SUM(total_latency_ms), SUM(error_count)
FROM %s%s
GROUP BY %s, provider, model
ORDER BY %s, provider, model`,
		periodCol, table, where, periodCol, periodCol)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying %s metrics: %w", table, err)
	}
	defer rows.Close()

	var results []UsageMetrics
	for rows.Next() {
		var m UsageMetrics
		var totalLatency int64
		if err := rows.Scan(
			&m.Period, &m.Provider, &m.Model,
			&m.RequestCount, &m.InputTokens, &m.OutputTokens,
			&m.EstimatedCost, &totalLatency, &m.ErrorCount,
		); err != nil {
			return nil, fmt.Errorf("scanning %s metrics row: %w", table, err)
		}
		if m.RequestCount > 0 {
			m.AvgLatencyMs = totalLatency / m.RequestCount
		}
		results = append(results, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating %s metrics rows: %w", table, err)
	}
	return results, nil
}

// RebuildMetrics deletes all aggregation data and recomputes it from the
// raw requests table.
func (s *SQLiteStore) RebuildMetrics(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning rebuild transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM hourly_usage"); err != nil {
		return fmt.Errorf("clearing hourly_usage: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM daily_usage"); err != nil {
		return fmt.Errorf("clearing daily_usage: %w", err)
	}

	const rebuildHourly = `
INSERT INTO hourly_usage (hour, provider, model, request_count, input_tokens, output_tokens, estimated_cost_usd, total_latency_ms, error_count)
SELECT
	strftime('%%Y-%%m-%%dT%%H:00:00Z', timestamp),
	provider,
	model,
	COUNT(*),
	SUM(input_tokens),
	SUM(output_tokens),
	SUM(estimated_cost_usd),
	SUM(latency_ms),
	SUM(CASE WHEN response_status >= 400 THEN 1 ELSE 0 END)
FROM requests
GROUP BY strftime('%%Y-%%m-%%dT%%H:00:00Z', timestamp), provider, model`

	if _, err := tx.ExecContext(ctx, rebuildHourly); err != nil {
		return fmt.Errorf("rebuilding hourly_usage: %w", err)
	}

	const rebuildDaily = `
INSERT INTO daily_usage (day, provider, model, request_count, input_tokens, output_tokens, estimated_cost_usd, total_latency_ms, error_count)
SELECT
	strftime('%%Y-%%m-%%d', timestamp),
	provider,
	model,
	COUNT(*),
	SUM(input_tokens),
	SUM(output_tokens),
	SUM(estimated_cost_usd),
	SUM(latency_ms),
	SUM(CASE WHEN response_status >= 400 THEN 1 ELSE 0 END)
FROM requests
GROUP BY strftime('%%Y-%%m-%%d', timestamp), provider, model`

	if _, err := tx.ExecContext(ctx, rebuildDaily); err != nil {
		return fmt.Errorf("rebuilding daily_usage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing rebuild transaction: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
