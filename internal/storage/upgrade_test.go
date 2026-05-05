package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

// v1SchemaSQL is the complete v1 schema for constructing test databases.
const v1SchemaSQL = `
CREATE TABLE requests (
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
CREATE INDEX idx_requests_timestamp ON requests(timestamp);
CREATE INDEX idx_requests_provider  ON requests(provider);
CREATE INDEX idx_requests_model     ON requests(model);

CREATE TABLE hourly_usage (
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

CREATE TABLE daily_usage (
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

// openRawDB opens an in-memory SQLite database with FK and WAL pragmas,
// suitable for manually constructing a v1 database before calling migrate().
func openRawDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("db.Ping: %v", err)
	}
	return db
}

// seedV1Data inserts realistic v1 data: requests, hourly_usage, and daily_usage rows.
func seedV1Data(t *testing.T, db *sql.DB) {
	t.Helper()

	// Insert requests across multiple providers and models.
	requests := []struct {
		id, ts, provider, model string
		status                  int
		inputTok, outputTok     int
		latency                 int
		cost                    float64
	}{
		{"req-001", "2026-01-15T10:00:00Z", "anthropic", "claude-sonnet-4-20250514", 200, 500, 200, 450, 0.003},
		{"req-002", "2026-01-15T10:05:00Z", "anthropic", "claude-sonnet-4-20250514", 200, 1000, 400, 800, 0.007},
		{"req-003", "2026-01-15T11:00:00Z", "openai", "gpt-4o", 200, 300, 150, 350, 0.002},
		{"req-004", "2026-01-15T11:30:00Z", "openai", "gpt-4o", 500, 200, 0, 100, 0.001},
		{"req-005", "2026-01-16T09:00:00Z", "anthropic", "claude-opus-4-20250514", 200, 2000, 1000, 1200, 0.06},
	}

	const insertReq = `INSERT INTO requests (id, timestamp, provider, model, method, url, response_status,
		input_tokens, output_tokens, latency_ms, estimated_cost_usd)
		VALUES (?, ?, ?, ?, 'POST', 'https://api.example.com/v1/messages', ?, ?, ?, ?, ?)`

	for _, r := range requests {
		if _, err := db.Exec(insertReq, r.id, r.ts, r.provider, r.model,
			r.status, r.inputTok, r.outputTok, r.latency, r.cost); err != nil {
			t.Fatalf("inserting request %s: %v", r.id, err)
		}
	}

	// Insert hourly_usage rows.
	const insertHourly = `INSERT INTO hourly_usage (hour, provider, model, request_count, input_tokens,
		output_tokens, estimated_cost_usd, total_latency_ms, error_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	hourlyRows := []struct {
		hour, provider, model                   string
		count, inTok, outTok, latency, errCount int
		cost                                    float64
	}{
		{"2026-01-15T10:00:00Z", "anthropic", "claude-sonnet-4-20250514", 2, 1500, 600, 1250, 0, 0.010},
		{"2026-01-15T11:00:00Z", "openai", "gpt-4o", 2, 500, 150, 450, 1, 0.003},
		{"2026-01-16T09:00:00Z", "anthropic", "claude-opus-4-20250514", 1, 2000, 1000, 1200, 0, 0.060},
	}
	for _, h := range hourlyRows {
		if _, err := db.Exec(insertHourly, h.hour, h.provider, h.model,
			h.count, h.inTok, h.outTok, h.cost, h.latency, h.errCount); err != nil {
			t.Fatalf("inserting hourly_usage: %v", err)
		}
	}

	// Insert daily_usage rows.
	const insertDaily = `INSERT INTO daily_usage (day, provider, model, request_count, input_tokens,
		output_tokens, estimated_cost_usd, total_latency_ms, error_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	dailyRows := []struct {
		day, provider, model                    string
		count, inTok, outTok, latency, errCount int
		cost                                    float64
	}{
		{"2026-01-15", "anthropic", "claude-sonnet-4-20250514", 2, 1500, 600, 1250, 0, 0.010},
		{"2026-01-15", "openai", "gpt-4o", 2, 500, 150, 450, 1, 0.003},
		{"2026-01-16", "anthropic", "claude-opus-4-20250514", 1, 2000, 1000, 1200, 0, 0.060},
	}
	for _, d := range dailyRows {
		if _, err := db.Exec(insertDaily, d.day, d.provider, d.model,
			d.count, d.inTok, d.outTok, d.cost, d.latency, d.errCount); err != nil {
			t.Fatalf("inserting daily_usage: %v", err)
		}
	}
}

func TestUpgrade_V1ToV2_DataPreserved(t *testing.T) {
	db := openRawDB(t)
	t.Cleanup(func() { db.Close() })

	// Create v1 schema and seed data.
	if _, err := db.Exec(v1SchemaSQL); err != nil {
		t.Fatalf("creating v1 schema: %v", err)
	}
	seedV1Data(t, db)

	// Verify v1 state before migration.
	var reqCountBefore int
	if err := db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&reqCountBefore); err != nil {
		t.Fatalf("counting requests before migration: %v", err)
	}
	if reqCountBefore != 5 {
		t.Fatalf("expected 5 requests before migration, got %d", reqCountBefore)
	}

	var hourlyCountBefore int
	if err := db.QueryRow("SELECT COUNT(*) FROM hourly_usage").Scan(&hourlyCountBefore); err != nil {
		t.Fatalf("counting hourly_usage before migration: %v", err)
	}

	var dailyCountBefore int
	if err := db.QueryRow("SELECT COUNT(*) FROM daily_usage").Scan(&dailyCountBefore); err != nil {
		t.Fatalf("counting daily_usage before migration: %v", err)
	}

	// Run v1→v2 migration.
	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify schema version is now 3.
	version, err := store.currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion: %v", err)
	}
	if version != 3 {
		t.Errorf("schema version = %d, want 3", version)
	}

	// Verify all v1 data is intact.
	var reqCountAfter int
	if err := db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&reqCountAfter); err != nil {
		t.Fatalf("counting requests after migration: %v", err)
	}
	if reqCountAfter != reqCountBefore {
		t.Errorf("requests count changed: before=%d, after=%d", reqCountBefore, reqCountAfter)
	}

	var hourlyCountAfter int
	if err := db.QueryRow("SELECT COUNT(*) FROM hourly_usage").Scan(&hourlyCountAfter); err != nil {
		t.Fatalf("counting hourly_usage after migration: %v", err)
	}
	if hourlyCountAfter != hourlyCountBefore {
		t.Errorf("hourly_usage count changed: before=%d, after=%d", hourlyCountBefore, hourlyCountAfter)
	}

	var dailyCountAfter int
	if err := db.QueryRow("SELECT COUNT(*) FROM daily_usage").Scan(&dailyCountAfter); err != nil {
		t.Fatalf("counting daily_usage after migration: %v", err)
	}
	if dailyCountAfter != dailyCountBefore {
		t.Errorf("daily_usage count changed: before=%d, after=%d", dailyCountBefore, dailyCountAfter)
	}

	// Spot-check a specific request row for data integrity.
	var provider, model string
	var inputTokens int
	if err := db.QueryRow("SELECT provider, model, input_tokens FROM requests WHERE id = 'req-005'").
		Scan(&provider, &model, &inputTokens); err != nil {
		t.Fatalf("querying specific request: %v", err)
	}
	if provider != "anthropic" || model != "claude-opus-4-20250514" || inputTokens != 2000 {
		t.Errorf("request data corrupted: provider=%q model=%q input_tokens=%d", provider, model, inputTokens)
	}

	// Verify new v2 tables exist and are empty.
	v2Tables := []string{"sessions", "turns", "session_events", "command_history"}
	for _, table := range v2Tables {
		var name string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
			t.Errorf("v2 table %q not found after migration: %v", table, err)
			continue
		}
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Errorf("counting rows in %q: %v", table, err)
		} else if count != 0 {
			t.Errorf("v2 table %q should be empty after migration, got %d rows", table, count)
		}
	}
}

func TestUpgrade_V1ToV2_SchemaComplete(t *testing.T) {
	db := openRawDB(t)
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(v1SchemaSQL); err != nil {
		t.Fatalf("creating v1 schema: %v", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify all expected tables exist.
	expectedTables := []string{
		"requests", "hourly_usage", "daily_usage",
		"sessions", "turns", "session_events", "command_history",
	}
	for _, table := range expectedTables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}

	// Verify columns for each v2 table using PRAGMA table_info.
	tableColumns := map[string][]string{
		"sessions": {
			"id", "user_id", "project", "summary", "active_model",
			"model_override", "routing_overrides", "pinned_items",
			"compaction_state", "total_cost", "total_input_tokens",
			"total_output_tokens", "context_pct", "status",
			"lock_owner", "lock_expires_at", "created_at", "updated_at",
		},
		"turns": {
			"id", "session_id", "sequence", "role", "content",
			"model", "provider", "input_tokens", "output_tokens",
			"cost", "latency_ms", "tool_calls", "files_touched",
			"files_modified", "compacted", "compacted_summary",
			"original_turns", "created_at",
		},
		"session_events": {
			"id", "session_id", "type", "detail", "payload", "at",
		},
		"command_history": {
			"id", "user_id", "project", "session_id", "content", "created_at",
		},
	}

	for table, expectedCols := range tableColumns {
		rows, err := db.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			t.Errorf("PRAGMA table_info(%s): %v", table, err)
			continue
		}

		foundCols := make(map[string]bool)
		for rows.Next() {
			var cid int
			var name, typ string
			var notNull, pk int
			var dfltValue *string
			if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
				t.Errorf("scanning table_info row for %s: %v", table, err)
				continue
			}
			foundCols[name] = true
		}
		rows.Close()

		for _, col := range expectedCols {
			if !foundCols[col] {
				t.Errorf("table %q missing column %q", table, col)
			}
		}
	}

	// Verify expected indexes exist using PRAGMA index_list.
	expectedIndexes := map[string][]string{
		"sessions": {
			"idx_sessions_user_id", "idx_sessions_user_active",
			"idx_sessions_project", "idx_sessions_status",
		},
		"turns": {
			"idx_turns_session_id", "idx_turns_session_seq",
		},
		"session_events": {
			"idx_session_events_session_id",
		},
		"command_history": {
			"idx_command_history_user_recent",
			"idx_command_history_project",
			"idx_command_history_session",
		},
	}

	for table, expectedIdxs := range expectedIndexes {
		rows, err := db.Query("PRAGMA index_list(" + table + ")")
		if err != nil {
			t.Errorf("PRAGMA index_list(%s): %v", table, err)
			continue
		}

		foundIdxs := make(map[string]bool)
		for rows.Next() {
			var seq int
			var name, origin string
			var unique, partial int
			if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
				t.Errorf("scanning index_list row for %s: %v", table, err)
				continue
			}
			foundIdxs[name] = true
		}
		rows.Close()

		for _, idx := range expectedIdxs {
			if !foundIdxs[idx] {
				t.Errorf("table %q missing index %q", table, idx)
			}
		}
	}
}

func TestUpgrade_V1ToV2_ForeignKeys(t *testing.T) {
	db := openRawDB(t)
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(v1SchemaSQL); err != nil {
		t.Fatalf("creating v1 schema: %v", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify foreign_keys pragma is enabled.
	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fkEnabled)
	}

	// Inserting a turn with a non-existent session_id should fail.
	_, err := db.Exec(`INSERT INTO turns (id, session_id, sequence, role, content, created_at)
		VALUES ('turn-1', 'nonexistent-session', 1, 'user', 'hello', '2026-01-15T10:00:00Z')`)
	if err == nil {
		t.Fatal("inserting turn with non-existent session_id should fail due to FK constraint")
	}

	// Inserting a session_event with a non-existent session_id should also fail.
	_, err = db.Exec(`INSERT INTO session_events (session_id, type, detail, at)
		VALUES ('nonexistent-session', 'test', 'test detail', '2026-01-15T10:00:00Z')`)
	if err == nil {
		t.Fatal("inserting session_event with non-existent session_id should fail due to FK constraint")
	}

	// Verify FK cascade: create a session, add turns and events, then delete the session.
	_, err = db.Exec(`INSERT INTO sessions (id, user_id, status, created_at, updated_at)
		VALUES ('sess-1', 'user-1', 'active', '2026-01-15T10:00:00Z', '2026-01-15T10:00:00Z')`)
	if err != nil {
		t.Fatalf("inserting session: %v", err)
	}

	_, err = db.Exec(`INSERT INTO turns (id, session_id, sequence, role, content, created_at)
		VALUES ('turn-1', 'sess-1', 1, 'user', 'hello', '2026-01-15T10:00:00Z')`)
	if err != nil {
		t.Fatalf("inserting turn: %v", err)
	}

	_, err = db.Exec(`INSERT INTO session_events (session_id, type, detail, at)
		VALUES ('sess-1', 'test', 'test detail', '2026-01-15T10:00:00Z')`)
	if err != nil {
		t.Fatalf("inserting session_event: %v", err)
	}

	// Delete the session — turns and events should cascade.
	if _, err := db.Exec("DELETE FROM sessions WHERE id = 'sess-1'"); err != nil {
		t.Fatalf("deleting session: %v", err)
	}

	var turnCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM turns WHERE session_id = 'sess-1'").Scan(&turnCount); err != nil {
		t.Fatalf("counting turns after cascade: %v", err)
	}
	if turnCount != 0 {
		t.Errorf("expected 0 turns after cascade delete, got %d", turnCount)
	}

	var eventCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM session_events WHERE session_id = 'sess-1'").Scan(&eventCount); err != nil {
		t.Fatalf("counting events after cascade: %v", err)
	}
	if eventCount != 0 {
		t.Errorf("expected 0 events after cascade delete, got %d", eventCount)
	}
}

func TestUpgrade_V1ToV2_WALPreserved(t *testing.T) {
	db := openRawDB(t)
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(v1SchemaSQL); err != nil {
		t.Fatalf("creating v1 schema: %v", err)
	}

	// Verify WAL before migration.
	var journalBefore string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalBefore); err != nil {
		t.Fatalf("PRAGMA journal_mode before: %v", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify WAL after migration.
	var journalAfter string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalAfter); err != nil {
		t.Fatalf("PRAGMA journal_mode after: %v", err)
	}

	// In-memory DBs may report "memory" instead of "wal".
	if journalBefore != journalAfter {
		t.Errorf("journal_mode changed: before=%q, after=%q", journalBefore, journalAfter)
	}
	if journalAfter != "wal" && journalAfter != "memory" {
		t.Errorf("journal_mode = %q, want wal or memory", journalAfter)
	}
}

func TestUpgrade_V2Idempotent(t *testing.T) {
	db := openRawDB(t)
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(v1SchemaSQL); err != nil {
		t.Fatalf("creating v1 schema: %v", err)
	}
	seedV1Data(t, db)

	store := &SQLiteStore{db: db}

	// First migration: v1 → v2.
	if err := store.migrate(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	version1, err := store.currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion after first migrate: %v", err)
	}

	// Count tables after first migration.
	var tableCount1 int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount1); err != nil {
		t.Fatalf("counting tables after first migrate: %v", err)
	}

	// Second migration: should be a no-op.
	if err := store.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	version2, err := store.currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion after second migrate: %v", err)
	}

	if version1 != version2 {
		t.Errorf("schema version changed: first=%d, second=%d", version1, version2)
	}
	if version2 != 3 {
		t.Errorf("schema version = %d, want 3", version2)
	}

	// Table count should not change.
	var tableCount2 int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount2); err != nil {
		t.Fatalf("counting tables after second migrate: %v", err)
	}
	if tableCount1 != tableCount2 {
		t.Errorf("table count changed: first=%d, second=%d", tableCount1, tableCount2)
	}

	// V1 data should still be intact.
	var reqCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&reqCount); err != nil {
		t.Fatalf("counting requests: %v", err)
	}
	if reqCount != 5 {
		t.Errorf("requests count = %d after idempotent migration, want 5", reqCount)
	}
}

func TestUpgrade_FutureVersion_Rejected(t *testing.T) {
	db := openRawDB(t)
	t.Cleanup(func() { db.Close() })

	// Set user_version to a future version.
	if _, err := db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatalf("setting user_version: %v", err)
	}

	store := &SQLiteStore{db: db}

	err := store.migrate()
	if err == nil {
		t.Fatal("migrate() should fail on future schema version")
	}

	// Verify the error message mentions the version and upgrade guidance.
	errMsg := err.Error()
	if !strings.Contains(errMsg, "99") {
		t.Errorf("error should mention version 99: %v", err)
	}
	if !strings.Contains(errMsg, "upgrade modeltap") {
		t.Errorf("error should mention 'upgrade modeltap': %v", err)
	}
}

func TestUpgrade_V1ToV2_ConcurrentAccess(t *testing.T) {
	// Use a shared-cache in-memory DB so all pool connections see the same data.
	db, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Force a single connection for setup to avoid pool races with empty DB.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(v1SchemaSQL); err != nil {
		t.Fatalf("creating v1 schema: %v", err)
	}
	seedV1Data(t, db)

	// Allow multiple connections for the concurrent phase.
	db.SetMaxOpenConns(4)

	store := &SQLiteStore{db: db}

	// Run migration in the background while performing concurrent reads.
	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	// Start concurrent readers before and during migration.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Read from v1 tables during migration.
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&count); err != nil {
				errCh <- err
				return
			}
			if count < 0 {
				errCh <- fmt.Errorf("unexpected negative count: %d", count)
			}
		}()
	}

	// Run migration.
	if err := store.migrate(); err != nil {
		t.Fatalf("migrate during concurrent access: %v", err)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent read error during migration: %v", err)
	}

	// Verify migration completed successfully.
	version, err := store.currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion: %v", err)
	}
	if version != 3 {
		t.Errorf("schema version = %d after concurrent migration, want 3", version)
	}

	// Verify v1 data is intact after concurrent access + migration.
	var reqCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&reqCount); err != nil {
		t.Fatalf("counting requests: %v", err)
	}
	if reqCount != 5 {
		t.Errorf("requests count = %d after concurrent migration, want 5", reqCount)
	}

	// Verify v2 tables exist and are queryable.
	for _, table := range []string{"sessions", "turns", "session_events", "command_history"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Errorf("querying v2 table %q after concurrent migration: %v", table, err)
		}
	}
}
