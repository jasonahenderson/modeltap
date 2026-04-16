# Track 0: Shared Prerequisites

**Release:** v0.2.0
**WU Range:** WU-039 through WU-045, WU-093, WU-096 (9 work units)
**Gates:**
- Track A (BFF Server): all of Track 0 foundation (WU-039–WU-045) must complete before Track A foundation WUs begin. WU-093 and WU-096 are additive and parallel with Track A; WU-093 is a **prerequisite for WU-067** (Track A integration) and WU-087 (Track B integration).
- Track B (Terminal Harness): harness-local work (WU-068–WU-072, WU-075–WU-079) may begin after WU-039 alone is stable. Track B work that touches the protocol client or streaming/session payloads (WU-073, WU-074, WU-080+) additionally requires WU-040 and WU-041. WU-087 additionally requires WU-093 (protocol fixtures).

## WU-039: Protocol Types — Core Messages and Framing

**Size:** Medium | **Dependencies:** None | **Review tier:** C (protocol contract + cross-track)

Implements `internal/protocol/`:
- `protocol.go` — protocol version constants, NDJSON framing helpers, `Mode` type (`plan`/`build`/`auto`), canonical field name documentation
- `messages.go` — all harness→server request types: `TurnSubmit`, `TurnCancel`, `ToolResult`, `ContentTransform`, `SessionResume`, `SessionList`, `SessionDetails`, `SessionCompact`, `CompactApply`, `SessionClear`, `SessionFork`, `SessionSync`, `ModelSwitch`, `ModelList`, `ContextList`, `CapabilitiesRegister`, `CapabilitiesUpdate`, `ConnectionPing`, `ConnectionHealth`, `ConnectionReady`
- `protocol_test.go` — round-trip marshal/unmarshal for every message type

**Definition of done:** `go build ./...` passes. Round-trip serialization tests pass for every message type. The package contains only types and serialization — no business logic.

---

## WU-040: Protocol Types — Streaming Events

**Size:** Medium | **Dependencies:** WU-039 | **Parallelizes with:** WU-041, WU-042 | **Review tier:** C (protocol contract + cross-track)

Implements `internal/protocol/events.go`:
- All server→harness streaming event types: `TokenDelta`, `BranchStarted`, `BranchComplete`, `BranchError`, `ToolCall`, `StatusUpdate`, `KnowledgeHit`, `CostUpdate`, `CompactPlan`, `CompactSuggest`, `CompactNotice`, `TurnComplete`, `ModelSelected`, `Error`
- All server→harness non-streaming types: `CapabilitiesRequest`, `ConnectionPong`
- Round-trip tests for all event types

**Definition of done:** All event types have round-trip serialization tests. `go build ./...` passes.

---

## WU-041: Protocol Types — Tools, Sessions, Models, Health, Errors

**Size:** Medium | **Dependencies:** WU-039 | **Parallelizes with:** WU-040, WU-042 | **Review tier:** C (protocol contract + cross-track)

Implements:
- `tools.go` — `ToolDefinition` (name, namespace, description, input_schema, output_envelope, risk_level, capabilities_required), `ToolCatalog`
- `sessions.go` — `SessionSummary`, `SessionDetail`, `TurnSummary`, `ServerEvent` payloads
- `models.go` — `ModelInfo`, `ModelListResponse`, `ModelSelectedEvent` payloads
- `health.go` — `HealthResponse`, `ReadyResponse`, `ProviderStatus`, dependency statuses
- `errors.go` — `DiagnosticCode` constants (`MT-CONN-001` through `MT-CONN-012`), `Diagnostic` struct
- `compact.go` — `CompactCategory`, `CompactPlanResponse`, `CompactApplyRequest`
- Round-trip tests for all types

**Definition of done:** All types have round-trip serialization tests. `go build ./...` passes.

---

## WU-042: ADR-0006 Amendment — Provider Outbound Formatting Interface

**Size:** Small | **Dependencies:** WU-039 | **Parallelizes with:** WU-040, WU-041 | **Review tier:** C (ADR + shared interface)

Implements:
- ADR amendment document: `docs/adr/0006-amendment-001-outbound-formatting.md`
- Extend `Provider` interface in `internal/provider/provider.go` with:
  - Canonical `Message` type (role, content, tool_calls, tool_results, attachments, metadata)
  - `FormatMessages(canonical []Message, systemPrompt string, windowSize int) ([]byte, error)`
  - `FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error)`
- Stub implementations in Anthropic and OpenAI adapters that return `ErrNotImplemented`

**Definition of done:** ADR amendment written. `Provider` interface extended. Existing tests still pass (stubs satisfy interface). `go build ./...` passes.

---

## WU-043: Anthropic Outbound Formatting

**Size:** Medium | **Dependencies:** WU-042 | **Parallelizes with:** WU-044, WU-045 | **Review tier:** C (shared interface implementation; cross-provider contract)

Implements `FormatMessages` and `FormatToolDefinitions` for the Anthropic adapter:
- Translate canonical message history → Anthropic Messages API format
- `messages` array with `role`/`content` blocks
- `tool_use`/`tool_result` block types
- System prompt as `system` parameter
- Tool definitions as `tools` parameter
- Context window truncation (drop oldest turns, preserve system prompt + recent)

**Definition of done:** Table-driven tests cover: simple text, multi-turn, tool call/result round-trips, system prompt injection, context window truncation, messages with attachments. All tests pass.

---

## WU-044: OpenAI Outbound Formatting

**Size:** Medium | **Dependencies:** WU-042 | **Parallelizes with:** WU-043, WU-045 | **Review tier:** C (shared interface implementation; cross-provider contract)

Implements `FormatMessages` and `FormatToolDefinitions` for the OpenAI adapter:
- Translate canonical messages → OpenAI Chat Completions format
- `messages` array with `role`/`content`
- `tool_calls`/`function` format
- System prompt as system role message
- Tool definitions as `tools` parameter
- Context window truncation

**Definition of done:** Table-driven tests matching WU-043 scope but for OpenAI format. All tests pass.

---

## WU-045: Session and Turn Storage Schema

**Size:** Large | **Dependencies:** WU-039 | **Parallelizes with:** WU-043, WU-044 | **Review tier:** C (stable on-disk schema + cross-track session contract)

Implements:
- New `sessions` and `turns` tables in `internal/storage/sqlite.go` (migration v2)
- New types in `internal/storage/store.go`: `Session`, `Turn`, `SessionFilter`, `ServerEvent`
- New `Store` interface methods: `CreateSession`, `GetSession`, `UpdateSession`, `ListSessions`, `CreateTurn`, `GetTurn`, `ListTurns`, `GetSessionTurns`, `AppendServerEvent`, `ListServerEvents`, `DeleteSessionsBefore`
- SQLite implementation for all new methods

**`sessions` table** (fields required to satisfy FEAT-0008 `session.list` and `session.details` payloads):

| Column | Type | Purpose |
|--------|------|---------|
| `id` | TEXT PK | Session UUID |
| `user_id` | TEXT | Owner (FEAT-0010 isolation) |
| `project` | TEXT | Project root / identifier (session scoping) |
| `summary` | TEXT | Auto-generated short title |
| `active_model` | TEXT | Currently selected model |
| `model_override` | TEXT NULL | Session-level model override, if set |
| `routing_overrides` | JSON | Per-session routing overrides |
| `pinned_items` | JSON | User-pinned context items |
| `compaction_state` | JSON | Current compaction state (categorized buckets, thresholds hit, last auto-compact summary) |
| `total_cost` | REAL | Running session cost |
| `total_input_tokens` | INTEGER | Running input token total |
| `total_output_tokens` | INTEGER | Running output token total |
| `context_pct` | REAL | Last observed context usage ratio (for `session.list`) |
| `status` | TEXT | active, suspended, completed |
| `lock_owner` | TEXT NULL | Harness holding the session lock |
| `lock_expires_at` | TIMESTAMP NULL | Lock grace deadline |
| `created_at` | TIMESTAMP | Session creation time |
| `updated_at` | TIMESTAMP | Last activity time |

**`turns` table** (per FEAT-0008 Session and Turn Storage Model):

| Column | Type | Purpose |
|--------|------|---------|
| `id` | TEXT PK | Turn UUID |
| `session_id` | TEXT FK | Parent session |
| `sequence` | INTEGER | Turn order within session |
| `role` | TEXT | user, assistant |
| `content` | TEXT (JSON) | Canonical message content |
| `model` | TEXT | Model used for this turn |
| `provider` | TEXT | Provider used |
| `input_tokens` | INTEGER | Input tokens for this turn |
| `output_tokens` | INTEGER | Output tokens for this turn |
| `cost` | REAL | Cost for this turn |
| `latency_ms` | INTEGER | Provider response latency |
| `tool_calls` | JSON | Tool calls made in this turn |
| `files_touched` | JSON | File paths read in this turn |
| `files_modified` | JSON | File paths written/edited in this turn |
| `compacted` | BOOLEAN | Whether this turn has been compacted |
| `compacted_summary` | TEXT | Summary if compacted |
| `original_turns` | JSON | Sequence IDs collapsed into a compacted turn |
| `created_at` | TIMESTAMP | Turn timestamp |

**`session_events` table** (supports `session.details.server_events` and resume notifications):

| Column | Type | Purpose |
|--------|------|---------|
| `id` | INTEGER PK | Event ID |
| `session_id` | TEXT FK | Parent session |
| `type` | TEXT | `auto_compact`, `server_restart`, etc. |
| `detail` | TEXT | Human-readable description |
| `payload` | JSON | Type-specific fields (e.g., `freed_tokens`) |
| `at` | TIMESTAMP | Event time |

**Definition of done:** Migration creates all three tables without breaking existing schema. All enumerated fields are present. Store methods cover CRUD for sessions, turns, and server events, including derivation helpers for `session.list` (summary + context_pct + files_touched aggregate) and `session.details` (timeline including compacted turns, pinned items, files touched/modified, server events). All new methods have table-driven tests. Existing storage tests still pass. `go build ./...` passes.

---

## WU-093: Protocol Contract — Shared Golden Fixtures and Cross-Track Conformance

**Size:** Medium | **Dependencies:** WU-039, WU-040, WU-041 | **Parallelizes with:** WU-042, WU-043, WU-044, WU-045 | **Review tier:** C (protocol contract + cross-track)

Satisfies FEAT-0008 §"Interface Definition" ("the protocol should be extracted into a standalone interface definition ... to enable automated contract testing"). Round-trip tests inside WU-039/040/041 catch *self-consistency*; this WU catches *drift* between Tracks A and B.

Implements:

- NEW `internal/protocol/fixtures/` — golden JSON/NDJSON files for every protocol message, event, and payload. One file per type (e.g., `turn_submit.json`, `token_delta.json`, `session_list_response.json`, `diagnostic_mt_conn_008.json`).
- NEW `internal/protocol/conformance_test.go` — `go:embed`s every fixture and runs:
  - **Forward conformance:** unmarshal fixture → canonical Go struct → marshal → byte-equivalent (after normalized key ordering) with the fixture.
  - **Schema conformance:** fixture fails validation if it has unknown fields or is missing required fields (uses a strict decoder).
  - **Reverse conformance:** synthesize representative instances in code → marshal → compare against the fixture (catches accidental field renames on the Go side).
  - **Coverage assertion:** a registry test fails if a protocol type exported from `internal/protocol/` has no corresponding fixture.
- NEW `internal/protocol/fixtures/README.md` — update procedure, why these exist, the protocol-freeze contract, pointer to FEAT-0008 "Canonical Field Names".
- Both Track A (WU-067) and Track B (WU-087) integration suites MUST include these fixtures as test inputs. WU-067/WU-087 gain an explicit dependency on WU-093.
- Each fixture carries a comment header with `protocol_version` and the FEAT-0008 section that defines it.

**Coverage requirements:**
- Every harness→server request type from WU-039
- Every server→harness streaming event from WU-040
- Every tool, session, model, health, error, compact type from WU-041
- At minimum one happy-path variant per type plus one error/diagnostic variant where applicable

**Definition of done:** Fixtures exist for 100% of protocol types exported from `internal/protocol/` (coverage test asserts this via reflection over the type registry). Forward, schema, and reverse conformance all pass. `go test ./internal/protocol/...` green. `go build ./...` passes. History log records the fixture set and the freeze policy.

---

## WU-096: Storage Migration v1→v2 Upgrade Tests from Real Fixtures

**Size:** Medium | **Dependencies:** WU-045 | **Parallelizes with:** WU-043, WU-044, any Track A WU after WU-045 | **Review tier:** C (on-disk schema correctness; migration is a one-shot contract)

WU-045's DoD covers *creating* the v2 schema on a fresh DB. This WU covers *upgrading an existing v1 database* with live capture data, metrics state, and retention state — the actual user-upgrade path. A bricked user DB on upgrade is a worst-case v0.2.0 failure mode.

Implements `internal/storage/migrate_test.go` (or extension of existing migration tests) with:

- **Fixture v1 databases** under `internal/storage/testdata/migrations/v1/`:
  - `empty.db` — v1 schema, no rows
  - `typical.db` — ~1k captures, realistic metrics aggregation state, retention pruning state mid-cycle
  - `heavy.db.gz` — ~100k captures, gzipped at rest, uncompressed at test time (target < 5MB committed)
  - `partial_v1.db` — simulated mid-migration crash (e.g., `user_version` unset, partial table present)
- **Test scenarios:**
  - Upgrade each fixture → v2 schema present, **all v1 data preserved**, new v2 tables exist and are empty (sessions/turns/session_events/command_history).
  - Idempotency: running migration twice is a no-op on the second run.
  - Interrupted migration: recovery run completes cleanly after simulated crash.
  - Downgrade guard: a v2 DB opened by pre-migration code path refuses to operate (PRAGMA `user_version` check prevents silent data loss).
  - WAL mode preserved across migration.
  - Upgrade time budget on `heavy.db`: < 30s on CI, tightened to < 5s once WU-095 lands a reference machine.
- **Fixture provenance:** a small generator (`internal/storage/testdata/migrations/gen.go`, build-tagged `//go:build migrationgen`) produces v1 DBs deterministically so fixtures can be regenerated. Committed fixtures are the canonical source; the generator is the safety net.

**Definition of done:** All fixtures exist and commit cleanly (heavy fixture gzipped, < 5MB). All scenarios pass. Migration is idempotent and crash-safe. Downgrade guard prevents silent v2-read. `go test -race ./internal/storage/...` passes in < 30s on CI.
