# 2026-05-05 - Design: Run Schema, Storage, and Migration (WU-109)

## Scope

This design covers WU-109:

- SQLite schema for durable runs, events, checkpoints, attachments, and
  run/turn links
- Go storage types and methods
- schema-version and migration plan
- cross-release extension points for v0.3.1 through v0.3.4

It does not implement artifact payload storage, validation evidence, policy
audit enrichment, durable memory, or quality routing. Those releases consume
extension fields reserved here.

## Current Baseline

Storage is `internal/storage` with SQLite migrations at schema version 2.
Existing conversation state is session/turn oriented:

- `sessions`
- `turns`
- `session_events`
- `command_history`

v0.3.0 adds schema version 3.

## New Tables

### `runs`

One row per durable workflow execution.

Required columns:

- `id TEXT PRIMARY KEY`
- `idempotency_key TEXT NOT NULL`
- `user_id TEXT NOT NULL`
- `project TEXT NOT NULL`
- `session_id TEXT NOT NULL REFERENCES sessions(id)`
- `parent_run_id TEXT NULL REFERENCES runs(id)`
- `initiator_type TEXT NOT NULL`
- `title TEXT NOT NULL DEFAULT ''`
- `workflow_type TEXT NOT NULL DEFAULT 'implementation'`
- `status TEXT NOT NULL`
- `stage TEXT NOT NULL`
- `attachment_state TEXT NOT NULL`
- `attached_connection_id TEXT NOT NULL DEFAULT ''`
- `attachment_grace_deadline TEXT NULL`
- `summary TEXT NOT NULL DEFAULT ''`
- `model TEXT NOT NULL DEFAULT ''`
- `provider TEXT NOT NULL DEFAULT ''`
- `input_tokens INTEGER NOT NULL DEFAULT 0`
- `output_tokens INTEGER NOT NULL DEFAULT 0`
- `total_cost REAL NOT NULL DEFAULT 0`
- `last_event_seq INTEGER NOT NULL DEFAULT 0`
- `last_checkpoint_id TEXT NOT NULL DEFAULT ''`
- `extension_json TEXT NOT NULL DEFAULT '{}'`
- `schema_version INTEGER NOT NULL DEFAULT 1`
- `created_at TEXT NOT NULL`
- `updated_at TEXT NOT NULL`
- `terminal_at TEXT NULL`

Indexes:

- unique `(user_id, project, idempotency_key)`
- `(user_id, project, updated_at DESC)`
- `(session_id, updated_at DESC)`
- `(status, updated_at DESC)`

### `run_turns`

Links ordered session turns to one run.

Columns:

- `run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE`
- `turn_id TEXT NOT NULL`
- `sequence INTEGER NOT NULL`
- `role TEXT NOT NULL`
- `created_at TEXT NOT NULL`
- primary key `(run_id, turn_id)`
- unique `(turn_id)`

The unique `turn_id` rule enforces FEAT-0015: a turn belongs to at most one run.

### `run_events`

Append-only event log.

Columns:

- `run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE`
- `seq INTEGER NOT NULL`
- `type TEXT NOT NULL`
- `stage TEXT NOT NULL DEFAULT ''`
- `status TEXT NOT NULL DEFAULT ''`
- `reason TEXT NOT NULL DEFAULT ''`
- `payload_json TEXT NOT NULL DEFAULT '{}'`
- `created_at TEXT NOT NULL`
- primary key `(run_id, seq)`

The store assigns `seq = runs.last_event_seq + 1` inside the same transaction
that updates `runs.last_event_seq`.

### `run_checkpoints`

Point-in-time recovery metadata.

Columns:

- `id TEXT PRIMARY KEY`
- `run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE`
- `seq INTEGER NOT NULL`
- `stage TEXT NOT NULL`
- `status TEXT NOT NULL`
- `reason TEXT NOT NULL DEFAULT ''`
- `turn_ids_json TEXT NOT NULL DEFAULT '[]'`
- `model_call_ids_json TEXT NOT NULL DEFAULT '[]'`
- `pending_tool_call_ids_json TEXT NOT NULL DEFAULT '[]'`
- `summary TEXT NOT NULL DEFAULT ''`
- `payload_json TEXT NOT NULL DEFAULT '{}'`
- `schema_version INTEGER NOT NULL DEFAULT 1`
- `created_at TEXT NOT NULL`

`payload_json` reserves nested keys: `context`, `artifacts`, `policy`,
`workspace`, `memory`, and `routing`.

### `run_attachments`

Attachment lease and observer metadata.

Columns:

- `run_id TEXT PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE`
- `state TEXT NOT NULL`
- `attached_connection_id TEXT NOT NULL DEFAULT ''`
- `attached_host_fingerprint TEXT NOT NULL DEFAULT ''`
- `grace_deadline TEXT NULL`
- `updated_at TEXT NOT NULL`

Observers are intentionally not represented here. They are transient connection
subscriptions until a later multi-client feature requires persistence.

## Storage API

Add `internal/storage/runs.go` with:

```go
type Run struct { ... }
type RunEvent struct { ... }
type RunCheckpoint struct { ... }
type RunFilter struct { UserID, Project, SessionID, Status string; Limit, Offset int }

func (s *SQLiteStore) CreateRun(ctx context.Context, run *Run, initial RunEvent, cp RunCheckpoint) error
func (s *SQLiteStore) GetRun(ctx context.Context, id string) (*Run, error)
func (s *SQLiteStore) ListRuns(ctx context.Context, filter RunFilter) ([]Run, error)
func (s *SQLiteStore) AppendRunEvent(ctx context.Context, runID string, ev RunEvent, update RunStateUpdate) (int64, error)
func (s *SQLiteStore) CreateRunCheckpoint(ctx context.Context, cp RunCheckpoint) error
func (s *SQLiteStore) ListRunEvents(ctx context.Context, runID string, afterSeq int64, limit int) ([]RunEvent, error)
func (s *SQLiteStore) LinkTurnToRun(ctx context.Context, runID, turnID, role string, seq int) error
```

`CreateRun` and `AppendRunEvent` are transaction boundaries. Code outside
storage must not read-modify-write `last_event_seq`.

## Cross-Release Compatibility Check

v0.3.1 context planner stores context-plan references in checkpoint
`payload_json.context` and may later add normalized context tables. v0.3.0 must
not bake context-file schemas into `runs`.

v0.3.2 artifacts use `runs.id`, `run_events.seq`, and checkpoint extension JSON
as stable source references. Artifact payload tables are deferred.

v0.3.3 policy/workspace stores effective policy and workspace mode references
under `payload_json.policy` and `payload_json.workspace`; executable workspace
creation is not modeled in v0.3.0.

v0.3.4 memory/routing stores source links to run ID, artifact ID, and outcome.
v0.3.0 only needs durable `workflow_type`, terminal outcome, model/provider,
cost, and token totals.

## Tests

- migration v2 to v3 creates all run tables and preserves v2 data
- downgrade guard updates `MaxKnownSchemaVersion` to 3
- `CreateRun` enforces idempotency by `(user_id, project, idempotency_key)`
- event append returns contiguous per-run sequence numbers
- `run_turns` rejects linking one turn to two runs
- checkpoint creation and latest checkpoint pointer are transactional
