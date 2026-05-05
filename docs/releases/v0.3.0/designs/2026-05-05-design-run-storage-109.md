# 2026-05-05 - Design: Run Schema, Storage, and Migration (WU-109)

## Scope

This design covers WU-109:

- SQLite schema for durable runs, events, checkpoints, attachments, and
  run/turn links
- idempotency boundaries for model calls and tool results
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
- `trace_id TEXT NOT NULL`
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
- `last_advanced_at TEXT NOT NULL`
- `model TEXT NOT NULL DEFAULT ''`
- `provider TEXT NOT NULL DEFAULT ''`
- `input_tokens INTEGER NOT NULL DEFAULT 0`
- `output_tokens INTEGER NOT NULL DEFAULT 0`
- `total_cost REAL NOT NULL DEFAULT 0`
- `last_event_seq INTEGER NOT NULL DEFAULT 0`
- `last_checkpoint_id TEXT NOT NULL DEFAULT ''`
- `extension_json TEXT NOT NULL DEFAULT '{}'`
- `retention_class TEXT NOT NULL DEFAULT 'standard'`
- `expires_at TEXT NULL`
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
- `payload_schema_version INTEGER NOT NULL DEFAULT 1`
- `created_at TEXT NOT NULL`
- primary key `(run_id, seq)`

The store assigns `seq = runs.last_event_seq + 1` inside the same transaction
that updates `runs.last_event_seq`. `session_id` for protocol event payloads is
projected from `runs` at read/emit time. `turn_id` is either copied into
`payload_json.turn_id` for turn-correlated events or projected from `run_turns`
when the event references a known run-turn link.

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

`run_attachments` is the authoritative lease-detail row.
`runs.attachment_state`, `runs.attached_connection_id`, and
`runs.attachment_grace_deadline` are denormalized list/detail summary fields.
They must be updated in the same transaction through one storage method. Code
outside storage must not update only one side of the attachment state.

### `run_model_calls`

Idempotent model-call accounting.

Columns:

- `model_call_id TEXT PRIMARY KEY`
- `run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE`
- `provider TEXT NOT NULL`
- `model TEXT NOT NULL`
- `stage TEXT NOT NULL DEFAULT 'model_call'`
- `status TEXT NOT NULL`
- `input_tokens INTEGER NOT NULL DEFAULT 0`
- `output_tokens INTEGER NOT NULL DEFAULT 0`
- `total_cost REAL NOT NULL DEFAULT 0`
- `latency_ms INTEGER NOT NULL DEFAULT 0`
- `payload_json TEXT NOT NULL DEFAULT '{}'`
- `created_at TEXT NOT NULL`
- `updated_at TEXT NOT NULL`

The primary key is the idempotency boundary. Re-reporting the same
`model_call_id` updates only missing terminal fields and must not double-count
usage in `runs`.

### `run_tool_results`

Idempotent tool-result delivery.

Columns:

- `tool_call_id TEXT PRIMARY KEY`
- `run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE`
- `tool TEXT NOT NULL`
- `namespace TEXT NOT NULL DEFAULT ''`
- `stage TEXT NOT NULL DEFAULT 'tool_loop'`
- `status TEXT NOT NULL`
- `result_id TEXT NOT NULL DEFAULT ''`
- `duration_ms INTEGER NOT NULL DEFAULT 0`
- `estimated_cost REAL NOT NULL DEFAULT 0`
- `payload_json TEXT NOT NULL DEFAULT '{}'`
- `created_at TEXT NOT NULL`
- `updated_at TEXT NOT NULL`

The primary key is the idempotency boundary for tool result delivery. Duplicate
`tool_call_id` reports return the stored result summary and do not re-enter the
model loop.

Run totals are derived from `run_model_calls` and `run_tool_results`. Per-stage
aggregation is a query over the `stage` columns in those tables and
`run_events`; no separate stage aggregate table is introduced in v0.3.0.

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
func (s *SQLiteStore) RecordRunModelCall(ctx context.Context, call RunModelCall) (created bool, err error)
func (s *SQLiteStore) RecordRunToolResult(ctx context.Context, result RunToolResult) (created bool, err error)
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

Retention metadata is intentionally minimal in v0.3.0: `retention_class` and
`expires_at` reserve the run-record side of the FEAT-0015 retention envelope.
Artifact/blob-specific retention remains deferred to FEAT-0020.

## Tests

- migration v2 to v3 creates all run tables and preserves v2 data
- downgrade guard updates `MaxKnownSchemaVersion` to 3
- `CreateRun` enforces idempotency by `(user_id, project, idempotency_key)`
- event append returns contiguous per-run sequence numbers
- `run_turns` rejects linking one turn to two runs
- checkpoint creation and latest checkpoint pointer are transactional
- duplicate `model_call_id` does not double-count run usage
- duplicate `tool_call_id` does not re-deliver a tool result
- unknown `workflow_type` is rejected at run creation
- v0.3.0 readers accept `run_checkpoints.schema_version = 1`
- run totals equal the sum of model-call/tool-result accounting fields
