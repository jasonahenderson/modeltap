package storage

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigration_FreshDB_GetsV4(t *testing.T) {
	store := newTestStore(t)

	version, err := store.currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion: %v", err)
	}
	if version != 4 {
		t.Errorf("schema version = %d, want 4", version)
	}

	// All v2 and v3 tables should exist.
	for _, table := range []string{"requests", "hourly_usage", "daily_usage", "sessions", "turns", "session_events", "command_history", "runs", "run_turns", "run_events", "run_checkpoints", "run_attachments", "run_model_calls", "run_tool_results"} {
		var name string
		err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestMigration_V1DB_MigratesToV4(t *testing.T) {
	// Create a raw v1 DB (user_version=0, v1 tables only)
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	// Create v1 schema manually (simulating existing v0.1.x DB)
	const v1Schema = `
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
	`
	if _, err := db.Exec(v1Schema); err != nil {
		t.Fatalf("creating v1 schema: %v", err)
	}

	// Insert some v1 data
	_, err = db.Exec(`INSERT INTO requests (id, timestamp, provider, model) VALUES ('req-1', '2025-01-01T00:00:00Z', 'anthropic', 'claude-3')`)
	if err != nil {
		t.Fatalf("inserting v1 data: %v", err)
	}

	db.Close()

	// Now open through NewSQLiteStore which runs migrate()
	// Since we can't pass an existing db to NewSQLiteStore, simulate with a raw store
	db2, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	// Re-create v1 with data
	if _, err := db2.Exec(v1Schema); err != nil {
		t.Fatalf("creating v1 schema: %v", err)
	}
	_, err = db2.Exec(`INSERT INTO requests (id, timestamp, provider, model) VALUES ('req-1', '2025-01-01T00:00:00Z', 'anthropic', 'claude-3')`)
	if err != nil {
		t.Fatalf("inserting v1 data: %v", err)
	}

	store := &SQLiteStore{db: db2}
	t.Cleanup(func() { store.Close() })

	if err := store.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Verify version is now 4.
	version, err := store.currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion: %v", err)
	}
	if version != 4 {
		t.Errorf("schema version = %d, want 4", version)
	}

	// V1 data should be preserved
	var id string
	if err := db2.QueryRow("SELECT id FROM requests WHERE id = 'req-1'").Scan(&id); err != nil {
		t.Errorf("v1 request should be preserved: %v", err)
	}
	var runID, traceID string
	if err := db2.QueryRow("SELECT run_id, trace_id FROM requests WHERE id = 'req-1'").Scan(&runID, &traceID); err != nil {
		t.Errorf("v1 request should have correlation columns: %v", err)
	}
	if runID != "" || traceID != "" {
		t.Errorf("new correlation columns = (%q, %q), want empty strings", runID, traceID)
	}

	// V2 and v3 tables should exist.
	for _, table := range []string{"sessions", "turns", "session_events", "command_history", "runs", "run_events", "run_checkpoints", "run_attachments"} {
		var name string
		err := db2.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("v2 table %q not found: %v", table, err)
		}
	}
}

func TestMigration_V4DB_IsNoop(t *testing.T) {
	store := newTestStore(t)

	// Migrate is already run; run it again
	if err := store.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	version, err := store.currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion: %v", err)
	}
	if version != 4 {
		t.Errorf("schema version = %d after double-migrate, want 4", version)
	}
}

func TestMigration_V3DB_AddsRequestCorrelationColumns(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	store := &SQLiteStore{db: db}
	t.Cleanup(func() { store.Close() })
	if err := store.migrateToV1(); err != nil {
		t.Fatalf("migrateToV1: %v", err)
	}
	if err := store.migrateToV2(); err != nil {
		t.Fatalf("migrateToV2: %v", err)
	}
	if err := store.migrateToV3(); err != nil {
		t.Fatalf("migrateToV3: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO requests (id, timestamp, provider, model) VALUES ('req-v3', '2026-05-12T00:00:00Z', 'anthropic', 'claude-3')`); err != nil {
		t.Fatalf("inserting v3 request: %v", err)
	}

	if err := store.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	version, err := store.currentSchemaVersion()
	if err != nil {
		t.Fatalf("currentSchemaVersion: %v", err)
	}
	if version != 4 {
		t.Errorf("schema version = %d, want 4", version)
	}

	var runID, traceID string
	if err := db.QueryRow("SELECT run_id, trace_id FROM requests WHERE id = 'req-v3'").Scan(&runID, &traceID); err != nil {
		t.Fatalf("querying migrated request correlation: %v", err)
	}
	if runID != "" || traceID != "" {
		t.Errorf("migrated correlation = (%q, %q), want empty strings", runID, traceID)
	}
	for _, idx := range []string{"idx_requests_run_id", "idx_requests_trace_id"} {
		var name string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name); err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

func TestMigration_DowngradeGuard(t *testing.T) {
	// Create a DB and set user_version to something higher
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	if _, err := db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatalf("setting user_version: %v", err)
	}

	store := &SQLiteStore{db: db}
	t.Cleanup(func() { store.Close() })

	err = store.migrate()
	if err == nil {
		t.Fatal("migrate() should fail on future schema version")
	}

	// Error message should indicate upgrade needed
	if err.Error() == "" {
		t.Fatal("error message should not be empty")
	}
}

func TestMigration_SchemaVersionConst(t *testing.T) {
	if MaxKnownSchemaVersion != 4 {
		t.Errorf("MaxKnownSchemaVersion = %d, want 4", MaxKnownSchemaVersion)
	}
}

func TestMigration_WALPreserved(t *testing.T) {
	store := newTestStore(t)

	var journalMode string
	if err := store.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	// In-memory DBs may report "memory" for journal_mode, not "wal".
	// For file-based DBs this would be "wal". Accept either for :memory:.
	if journalMode != "wal" && journalMode != "memory" {
		t.Errorf("journal_mode = %q, want wal or memory", journalMode)
	}
}
