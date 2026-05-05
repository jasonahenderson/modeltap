# Storage Bundle (WU-045 + WU-091 + WU-096) Pre-Review Lint — Claude Subagent

**Reviewer:** Claude subagent (fresh context, same-model pre-review — not Tier C peer review)
**Date:** 2026-04-16
**Subject:** `docs/history/2026-04-16-design-storage-045-091-096.md`
**Bundle:** WU-045 (sessions/turns/session_events schema + Store extensions) + WU-091 (command_history schema and protocol handlers) + WU-096 (v1→v2 migration tests from real fixtures)

## Reviewer caveat

Same-model lint: shares the Designer's training distribution, tokenizer, and reasoning heuristics. Most likely to catch mechanical drift (missed spec fields, typename inconsistency, SQL-pragma interactions that are Go-idiomatic blind spots) and cross-WU inconsistency against the protocol-types and provider-formatting bundles. Least likely to catch Claude-characteristic reasoning blind spots (uncritical acceptance of plausible-looking symmetry; rehearsal of the design's own framing). Does not substitute for a cross-model Tier-C peer review.

## Summary

The bundled design covers the three specs materially and introduces sensible structures for the session lock, migration stepping, and cursor pagination. Four blocking findings: (B-01) the schema is missing the `session_id` column on the `turns` table's FK declaration relative to the WU-045 track-0 spec — actually it is present, but the design omits any persisted **branch state** table or column used by `session.sync` for multi-model recovery (FEAT-0008 §"Branch-aware session.sync" requires branch state to survive disconnects; the design makes no provision for it anywhere in storage); (B-02) the `migrate()` code shape violates WAL/transaction semantics because `migrateToV1` is `CREATE TABLE IF NOT EXISTS` with no transaction, meaning a crash mid-way through v1-retrofit leaves a *different* partial state than the `partial_v1.db` fixture simulates, so `TestMigration_V1_to_V2_PartialV1` does not model a real failure mode; (B-03) `PRAGMA foreign_keys = ON` is a **per-connection** setting and `database/sql` maintains a connection pool whose size the design never pins to 1 nor configures via `_pragma=foreign_keys(1)` in the DSN — with the current `SetMaxOpenConns` unspecified (default unlimited), a freshly-dialed pool connection will have FK enforcement **off**, silently breaking the `ON DELETE CASCADE` contract the design relies on; (B-04) cross-bundle naming inconsistency — the protocol-types bundle defines `protocol.ServerSessionEvent` with fields `{type, at, freed_tokens, detail}` (no `payload` field, no `id`), while this storage bundle defines `storage.ServerSessionEvent` with `{id, session_id, type, detail, payload, at}` and asserts they differ — but the difference goes beyond "timestamps are strings vs. time.Time": the storage shape has `payload json.RawMessage` whereas the protocol shape has a flat `freed_tokens int`, meaning the BFF converter (WU-050) must synthesize protocol fields from blob JSON with **no documented key schema** for `payload`.

Attention items include: (A-01) `SessionSummary.LastActive` is spec'd but there is no `turns.sequence UNIQUE` and no query plan for computing `last_turn_summary` — the design says the column is a projection but never specifies how it is derived; (A-02) the lock-acquisition SQL uses the `?5` parameter for "now" which is supplied by Go-side `time.Now()` and is therefore subject to harness-clock skew when the same harness acquires + checks expiry; (A-03) the command-history session-scope query `WHERE session_id = ? AND user_id = ?` cannot use the composite index `(session_id, created_at DESC)` efficiently because `user_id` is not in that index — the index definition and the scope query contradict each other; (A-04) the design does not define `SessionFilter.Status` enum validation (string typed, invalid values silently return empty); (A-05) `CommandHistoryFilter` has no `Scope` field but the handler spec expects `scope = "user"|"project"|"session"` — the storage filter shape cannot distinguish "scope=user with project empty" from "scope=project with project unintentionally empty"; (A-06) `SessionFilter.UserID` is declared required in Go but the design enforces that in prose only, with no panic / validation / ErrMissingUserID sentinel; (A-07) pagination cursor serialization is ISO8601 but the design never addresses timestamp-collision pagination (two entries at identical `created_at`); (A-08) migration v2 runs all table creates **plus** data DML in one tx per the design, but the design's v2 migration body has no DML — which is fine now but the doc says "each migrateToVN runs in a single SQLite transaction" without noting that SQLite's DDL-in-transaction + WAL has specific checkpoint implications for the `heavy.db.gz` <30s budget; (A-09) the downgrade guard is enforced at `NewSQLiteStore` but after `migrate()` runs — i.e., if an older binary opens a v2 DB, `migrate()` is called before the guard returns, which is the wrong order for an abort path; the doc's code snippet orders the guard correctly but the surrounding prose and existing `sqlite.go` structure make it easy to misimplement; (A-10) `ListTurns` is spec'd with `UNIQUE(session_id, sequence)` rejected, but without uniqueness, `ORDER BY sequence` on a compaction-compacted turn set has ties that the design doesn't disambiguate (does a compacted row sort at `sequence=4` or `sequence=7` for `original_turns=[4,5,6,7]`?).

Nits: three deviations section ships with TPM action items ("TPM should update the spec to list the derivation helpers..."), but two of these (`ListTurns` vs. `GetSessionTurns`, `SessionFilter.UserID` required) are unambiguous improvements that should be committed as spec amendments, not merely flagged; file-layout block omits `internal/storage/migrate.go` even though the design introduces a `migrate()` step-wise function split; `MaxKnownSchemaVersion` is declared `const` in D1 but placed in `sqlite.go` implicitly — a package-level public const in `store.go` is more discoverable for tests and tools.

## Blocking findings

### B-01. Branch state for `session.sync` has no persistence surface anywhere in the schema

- **What:** FEAT-0008 §"Branch-aware session.sync" (lines 836-839) and §"In-Flight Turn Recovery" (lines 478-493) require that after reconnect, `session.sync` returns per-branch state for a multi-model turn — completed branches with their summaries, streaming branches with token counts, failed with error detail. The `ActiveTurnState`/`MultiModelState`/`ReviewerState` types in the protocol bundle (`docs/history/2026-04-16-design-protocol-types-040-041-093.md:292-296`) describe the wire shape. But this storage design has nowhere to persist that state. `turns` rows are **terminal** — created with `compacted`/`cost`/`tokens` already known. There is no table for in-flight turn state, no `branches` table, no `branch_state` column on `sessions`. If the BFF crashes during a multi-model turn, the per-branch recovery needed for `session.sync` cannot be reconstructed from storage.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:81-107` — `sessions` schema has no column for active-turn or branch state.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:117-141` — `turns` schema has `role`/`content`/`cost` fields that describe completed turns; no `status = streaming|pending_tool_result` or branch correlation column.
  - `docs/features/0008-bff-server.md:478-493` — the `session.sync` multi-model payload that must be reconstructable on server restart.
  - `docs/features/0008-bff-server.md:1197` (Success Criterion #9) — "The server handles concurrent harness connections".
  - `docs/releases/v0.2.0/track-a-bff-server.md:184-189` — WU-064 depends on `session.sync` returning "active turn status, pending tools, completed tokens, branch states" — which would need to come from storage if the BFF itself restarted.
  - The design's D10 notes "In-flight turn state" is out of scope (implicitly by not mentioning it) and line 486 calls out that long-session streaming iteration is "deferred", but does not mention multi-model branch persistence at all.
- **Why blocking:** WU-050 (session management) and WU-064 (in-flight recovery) need to know whether to keep branch state in-memory only (losing it on BFF restart — an acceptable design choice, but one that must be explicit) or persist it here. The design implicitly assumes the former but does not say so. If the decision is "in-memory only, `session.sync` after BFF restart returns `active_turn: null`", that needs to be ratified in this design because it shapes the `sessions.status` semantics and the WU-064 test matrix. If the decision is "persist branch state", then a table is missing from migration v2 — adding it in a later migration after WU-050 has shipped is a much more invasive migration for users than bundling it now.
- **Suggested fix:** Add D11 "In-flight turn state: in-memory only (persistence out of scope for v0.2.0)" with two consequences spelled out: (a) `session.sync` after a BFF cold start returns `active_turn: null` for any session whose turn was mid-flight at shutdown, and (b) the FEAT-0008 `session.sync` recovery story depends on **harness reconnect inside the grace window** (40s), not on server restart. Alternatively, add a `branches` table now. Either choice, but not silent.

### B-02. `migrateToV1` is not transactional, contradicting D1 "each migrateToVN runs in a single SQLite transaction" and invalidating the `partial_v1.db` crash model

- **What:** D1 says "Each `migrateToVN` runs in a single SQLite transaction (so crashes roll back cleanly) and bumps `user_version` as the last statement in the transaction." But `migrateToV1` is described as "existing CREATE TABLE IF NOT EXISTS block + PRAGMA user_version = 1" (line 59). The existing `sqlite.go:83-138` executes its schema as a single multi-statement `db.Exec(schema)` call with **no transaction wrapping**. Further, `PRAGMA user_version = N` inside an active transaction in SQLite **has special semantics** — it is allowed, but the `user_version` change does not become visible to other connections until `COMMIT`, which is fine for single-connection migration, but the existing code pattern (multi-statement `Exec`) is not automatically transactional in `modernc.org/sqlite`'s default autocommit mode.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:59` — "`migrateToV1() error  // existing CREATE TABLE IF NOT EXISTS block + PRAGMA user_version = 1`"
  - `docs/history/2026-04-16-design-storage-045-091-096.md:63` — "Each `migrateToVN` runs in a single SQLite transaction".
  - `internal/storage/sqlite.go:83-138` — existing `migrate()` uses `s.db.Exec(schema)` with no transaction. The design retrofits this without amending it.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:389` — `partial_v1.db` description: "crash-mid-migration simulation: `requests` table present, `hourly_usage` and `daily_usage` tables missing." This is the **autocommit** partial-state, which is only possible if the v1 migration was non-transactional.
- **Why blocking:** Two contradictions: (a) the design asserts atomic v1 migration, but retrofits an already-shipped non-atomic `Exec(schema)` block and claims that is transactional. (b) The `partial_v1.db` fixture is explicitly a non-atomic partial state; if v1 migration is now atomic, no real user DB can be in that state — and the test is testing a scenario that cannot occur. Either the fixture models real historical user DBs (v0.1.x was non-atomic → fixture is correct → D1 claim is wrong) or the fixture models a hypothetical that will not occur (new atomic migration → fixture is theater). The design needs to pick a lane.
- **Suggested fix:** (a) Wrap `migrateToV1` body in an explicit `BeginTx` → `COMMIT`. (b) Document that `partial_v1.db` models the pre-existing v0.1.x state where migration was `Exec(schema)` autocommit, and that the v2 migrate path must be robust to this specific partial state (treating "requests present, aggregation tables missing" as "re-run v1, then v2" rather than erroring). (c) Explicitly verify that `PRAGMA user_version = 1` inside `BeginTx` is honored by `modernc.org/sqlite` (it is for pure-Go sqlite, but this is worth a one-line test assertion). (d) Add a comment in the migration function that the `partial_v1.db` scenario is specifically the pre-atomic failure mode, not a general "any interruption" model — `TestMigration_V1_to_V2_PartialV1` should not be used to reason about v2 interruption.

### B-03. `PRAGMA foreign_keys = ON` is a per-connection setting; `database/sql` pool semantics can silently disable FK cascade

- **What:** The design states (line 482) that `NewSQLiteStore` "sets it [PRAGMA foreign_keys = ON]" and that this is sufficient because "all consumers go through `NewSQLiteStore`." This is wrong in a subtle but consequential way. `database/sql` maintains a connection pool, and `s.db.Exec("PRAGMA foreign_keys = ON")` executes on *one* connection from the pool. The next connection the pool dials is a fresh SQLite connection with FK enforcement set to the library default (OFF for SQLite). The current `sqlite.go:74-80` `enableWAL()` has the same latent issue, but WAL mode is DB-file-level and persists across connections, so the bug is invisible there. FK enforcement is NOT DB-file-level — it is per-connection, and it must be re-applied on every dialed connection. This is a documented `database/sql` vs. SQLite semantics mismatch.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:145-146` — "Foreign key: `session_id REFERENCES sessions(id) ON DELETE CASCADE`. Requires `PRAGMA foreign_keys = ON` (SQLite default is OFF)."
  - `docs/history/2026-04-16-design-storage-045-091-096.md:482` — "`NewSQLiteStore` sets it, and all consumers go through `NewSQLiteStore`. Add a test that confirms the pragma is set after open."
  - `internal/storage/sqlite.go:74-80` — `enableWAL()` pattern that the design intends to mimic. But WAL is per-file; foreign_keys is per-connection.
  - `internal/storage/sqlite.go:150,464` — the code uses `BeginTx`, which may dial a new connection. Any connection dialed lazily during a transaction will default to `foreign_keys = OFF`.
  - Go `database/sql` pool default: `SetMaxOpenConns(0)` means unlimited. The design does not set a limit.
- **Why blocking:** `DELETE FROM sessions WHERE ...` will silently **not** cascade to `turns` or `session_events` on any connection that hasn't had `PRAGMA foreign_keys = ON` applied. The D4 retention helper `DeleteSessionsBefore` depends on this cascade. The test in the risk bullet ("a test that confirms the pragma is set after open") only verifies the pragma on a single connection — it does not verify the invariant across all pool connections. The production bug would be: retention prunes sessions but orphans turns, which then fail future queries with dangling FK references (or, worse, appear as ghost data).
- **Suggested fix:** Use one of three standard patterns: (a) pass `?_pragma=foreign_keys(1)` (and `?_pragma=journal_mode(WAL)` for symmetry) in the DSN so `modernc.org/sqlite` applies them on every dialed connection; (b) call `db.SetMaxOpenConns(1)` to pin the pool to one connection (simplest; acceptable for modeltap's embedded single-writer workload); (c) use `sql.OpenDB(connector)` with a custom `driver.Connector` that applies both pragmas on every `Connect()` call. Option (a) is the idiomatic modernc.org/sqlite approach. Option (b) is the simplest and aligns with SQLite's single-writer model; it also removes a class of concurrency bugs around multi-connection WAL checkpoints. Either way, the design must state which. Add a test that opens, forces multiple pool connections (e.g., by holding an open read transaction while opening a write), and verifies FK cascade from both.

### B-04. `storage.ServerSessionEvent` vs. `protocol.ServerSessionEvent` shape mismatch has no documented converter schema

- **What:** The protocol-types bundle defines `protocol.ServerSessionEvent` with fields `{type string (R), at string (R, ISO8601), freed_tokens int (O, omitempty), detail string (R)}` — a flat structure with explicit `freed_tokens` at the top level (`docs/history/2026-04-16-design-protocol-types-040-041-093.md:291`). This storage bundle defines `storage.ServerSessionEvent` with fields `{ID int64, SessionID string, Type string, Detail string, Payload json.RawMessage, At time.Time}` (`docs/history/2026-04-16-design-storage-045-091-096.md:246-253`). The design's naming note (line 334) says "A converter lives in `internal/bff/session.go` (WU-050)", but the converter has to promote `Payload[".freed_tokens"]` to the protocol's top-level `freed_tokens` field — and the key schema inside `payload` is never defined in either bundle.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:156-162` — SQL schema: `payload TEXT NOT NULL DEFAULT '{}' -- JSON object (freed_tokens, etc.)`. The comment hints at the key but doesn't commit.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:244-253` — Go type has `Payload json.RawMessage`.
  - `docs/history/2026-04-16-design-protocol-types-040-041-093.md:291` — protocol type has `freed_tokens int` as a first-class column, not nested in a payload blob.
  - `docs/features/0008-bff-server.md:942` — FEAT-0008 example: `{ "type": "auto_compact", "at": "2026-04-15T03:00:00Z", "freed_tokens": 12800, "detail": "..." }` — the wire shape is flat.
- **Why blocking:** The blob-to-flat promotion is not a timestamp-conversion or JSON-raw detail. It's a schema translation that requires a registry of `type` → payload key promotion rules. Without that registry:
  1. `auto_compact.payload = {"freed_tokens": 12800}` — converter knows to promote.
  2. `server_restart.payload = {...}` — what keys? Are there any promoted fields? The protocol type doesn't model any, but the design says "Payload TEXT NOT NULL DEFAULT '{}' -- JSON object (freed_tokens, etc.)" leaves "etc." undefined.
  3. New event types (a future `provider_outage` event, for example) would need to update both the schema `payload` key set AND the protocol type definition — a two-file change the design doesn't call out.
  
  Worse: if the protocol type's `freed_tokens` is only relevant to `auto_compact`, putting it at the top level of the protocol struct is itself a design mistake (it should be nested in the payload) — but that's a protocol-bundle issue, and blocking here because the storage design can't resolve the cross-bundle inconsistency on its own.
- **Suggested fix:** Add a subsection "Event payload schema" to D2 (or in D4 near the type definition) that enumerates the known event `type` values and their expected `payload` key set:
  - `auto_compact` → `{"freed_tokens": int}`
  - `server_restart` → `{}` (no keys; `detail` carries the message)
  - Future types register here, not in code.
  
  Then document the converter rule: "The BFF converter reads `storage.ServerSessionEvent.Payload` and promotes known keys per the registry above; unknown keys are preserved in a `extra` map on `protocol.ServerSessionEvent` (pending protocol amendment) or dropped (current behavior — document the tradeoff)." Coordinate with the protocol-types bundle to either (a) add `Extra map[string]any` to `protocol.ServerSessionEvent` or (b) remove `freed_tokens` from the top level and use a `payload json.RawMessage` on the wire. Option (b) keeps the two types isomorphic.

## Attention findings

### A-01. `SessionSummary.LastTurnSummary` projection has no defined query path

- **What:** `storage.SessionSummary` (line 317-332) includes `LastTurnSummary string`, which the FEAT-0008 `session.list` example shows as `"last_turn_summary": "backend agent completed, reviewer found 2 issues"` (FEAT-0008:908). The design says this is computed by `SessionSummaries(ctx, filter)` and the prose says computation "requires JOIN + aggregation logic that belongs in storage" — but no turn has a `summary` column in the schema. The `turns` table has `content` (canonical message JSON) and `compacted_summary` (only populated when compacted). The last non-compacted turn has no summary.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:328` — `LastTurnSummary string`.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:117-141` — `turns` schema: no `summary` column.
  - `docs/features/0008-bff-server.md:908` — FEAT-0008 wire example shows short descriptive strings; these must come from somewhere.
  - `docs/history/2026-04-16-design-protocol-types-040-041-093.md:290` — `TurnSummary.summary string (R)` is the wire shape; not a DB column.
- **Recommended disposition:** Add either (a) a `summary TEXT NOT NULL DEFAULT ''` column to `turns` that the BFF populates via cheap-model summarization (mirrors `sessions.summary` generation; consistent with FEAT-0008:947-948 "after the first 2-3 turns, the server prompts a cheap model to produce a short session title"), or (b) a documented projection rule (e.g., "truncate `content` to 80 chars; compacted turns use `compacted_summary`"). Option (a) is more faithful to FEAT-0008; option (b) is simpler but user-hostile. Whichever, commit to it — the design leaves the reader to guess.

### A-02. Lock-acquisition SQL takes clock from Go-side; expired-self-owned lock mechanics are undefined

- **What:** D5 passes `?5 = time.Now()` from the Go caller for the stale-lock check. Two edge cases the design does not specify:
  1. **Expired self-owned lock:** Harness A holds the lock, but its expiry has passed. Harness A reconnects within the grace window (WU-048: 40s) and calls `AcquireSessionLock` with its own owner ID. The current SQL `WHERE id = ?1 AND (lock_owner IS NULL OR lock_expires_at < ?5)` treats this as "expired → acquirable", and the `UPDATE` succeeds — but now the lock timestamp is renewed. Is that the intended behavior? WU-048 §"Reconnect inside the grace window keeps the lock" suggests yes, but the mechanism is undocumented.
  2. **Self-owned fresh lock:** Harness A already holds a fresh (non-expired) lock and calls `AcquireSessionLock` again for the same `owner`. The `WHERE` clause fails (lock_owner is not NULL, expiry is in future), `RowsAffected = 0`, the handler then returns "contested by `ownerA`" — i.e., a harness gets `MT-CONN-008` against itself. The design needs an `OR lock_owner = ?2` branch in the WHERE.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:340-347` — acquisition SQL.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:464-465` — test plan says "acquire, re-acquire same owner, contest other owner, stale lock claim, force-release" — re-acquire-same-owner is called out as a test case, but the SQL doesn't support it.
  - `docs/features/0008-bff-server.md:420` — "after heartbeat timeout ... plus grace period, the server releases the active session lock."
  - `docs/releases/v0.2.0/track-a-bff-server.md:72` — WU-050 "Lock survives reconnection inside the grace window."
- **Recommended disposition:** Amend the SQL to `WHERE id = ?1 AND (lock_owner IS NULL OR lock_owner = ?2 OR lock_expires_at < ?5)` so a harness re-acquiring its own (fresh or expired) lock succeeds. Add a test case for the three cases: re-acquire-same-owner-fresh, re-acquire-same-owner-expired, contend-other-owner. Additionally, document the clock-source decision: Go-side `time.Now().UTC()` is fine for single-BFF deployments but will misbehave in the FEAT-0010 multi-node future — flag this as a v0.3.x concern.

### A-03. Command-history session-scope index does not match the session-scope query

- **What:** The session-scope query is `WHERE session_id = ? AND user_id = ? ORDER BY created_at DESC` (per D6). The index is `CREATE INDEX idx_command_history_session ON command_history(session_id, created_at DESC)`. `user_id` is not in this index, so SQLite will scan the `session_id`-matching range and filter `user_id` per-row. For a well-scoped session (one user_id per session), this is fine — but the design explicitly says "user_id enforced even in session scope so cross-user leakage is impossible at the query level." If enforcement is at the query level, it should be in the index too, otherwise a pathological session with multiple users (impossible by design, but testable) would surface the asymmetry.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:180` — `idx_command_history_session ON command_history(session_id, created_at DESC)`.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:376` — `scope = "session"` → `WHERE session_id = ? AND user_id = ?`.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:380-381` — "The `user_id` filter is applied **always**, including under session scope."
- **Recommended disposition:** Either (a) change the index to `idx_command_history_session ON command_history(user_id, session_id, created_at DESC)` so the query is fully index-covered, or (b) leave the index as-is and document that session-scope queries rely on `session_id` cardinality being low (one user per session) and are expected to filter <10 rows after the initial seek. Option (a) is mechanically safer and uses the same pattern as `idx_command_history_project`.

### A-04. `SessionFilter.Status` has no validation; invalid values silently match nothing

- **What:** `SessionFilter.Status` is `string` with comment "optional — active | suspended | completed". No enum type, no validation, no documented behavior for invalid values. A misspelled `"actvie"` returns zero rows with no error.
- **Evidence:** `docs/history/2026-04-16-design-storage-045-091-096.md:239`.
- **Recommended disposition:** Either introduce a `SessionStatus` typed string with the three valid values as package consts (and a validator), or explicitly document that invalid `Status` returns empty results silently (matching the existing `ListFilter.Provider` behavior at `store.go:30-38`). The latter is consistent with existing code but error-prone — a typed enum is worth the 8 lines.

### A-05. `CommandHistoryFilter` shape cannot express the "scope" dimension cleanly

- **What:** The WU-091 protocol handler takes `{ scope: "user"|"project"|"session", limit, before }`. The Go filter type is `{UserID, Project, SessionID, Limit, Before}` — a flat struct. This cannot distinguish "scope=user, project accidentally empty" from "scope=project, project='' (wants project-less entries)". The design's D6 rules say `user` scope → `WHERE user_id = ?`, `project` scope → `WHERE user_id = ? AND project = ?`. If project is empty the two reduce to the same query — silently.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:265-273` — `CommandHistoryFilter` type.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:372-376` — scope rules.
  - `docs/releases/v0.2.0/track-a-bff-server.md:233-234` — protocol handler spec: `scope` is an explicit enum parameter.
- **Recommended disposition:** Add an explicit `Scope CommandHistoryScope` field (typed enum with three constants) to `CommandHistoryFilter`, and document that `Project`/`SessionID` are only consulted when scope requires. The protocol handler maps `scope=session` with empty `session_id` to an invalid-params error, which the storage layer should also refuse.

### A-06. `SessionFilter.UserID` is "required" only in prose; no validation

- **What:** Comment on `SessionFilter.UserID` says `// required — no cross-user listing`. There is no Go-side enforcement. A caller passing `SessionFilter{}` (empty UserID) issues `WHERE user_id = ''`, which in FEAT-0010's future (where some sessions legitimately have empty `user_id` for single-user deployments) would leak single-user-mode sessions cross-tenant.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:236-237` — comment "required".
  - `docs/releases/v0.2.0/track-0-shared.md:113` — spec field: `user_id` "Owner (FEAT-0010 isolation)".
- **Recommended disposition:** Either (a) add an `ErrMissingUserID` sentinel and return it from `ListSessions`/`SessionSummaries` when `UserID == ""` (simple and defensive), or (b) accept the empty case as "single-user deployment / admin" and document it explicitly. Do not leave it as "required by convention" — that is exactly the kind of contract the FEAT-0010 security review will flag.

### A-07. Pagination cursor doesn't disambiguate timestamp ties

- **What:** D6 pagination uses `ORDER BY created_at DESC` with a `Before` cursor (exclusive). SQLite's `created_at` is RFC3339 TEXT. Two entries with identical `created_at` (a plausible case for bulk-append tests or very fast harness typing) would either both be returned twice or both be skipped depending on the cursor boundary semantics. FEAT-0009 expects stable pagination.
- **Evidence:** `docs/history/2026-04-16-design-storage-045-091-096.md:378-380`.
- **Recommended disposition:** Use a compound cursor `(created_at, id)` — `ORDER BY created_at DESC, id DESC` with `WHERE created_at < ? OR (created_at = ? AND id < ?)`. Serialize cursor as a JSON tuple or an opaque base64 string that the server decodes. The design says the cursor is "Opaque to the harness — the server decides the cursor format" (line 378), so this change is internal. Add a test with two entries at identical `created_at` that pagination returns both exactly once.

### A-08. DDL-in-transaction under WAL has specific checkpoint behavior affecting the heavy-fixture budget

- **What:** `migrateToV2` runs `CREATE TABLE ... CREATE INDEX ...` inside a single transaction under WAL mode. SQLite under WAL writes DDL changes to the -wal file, and a checkpoint runs on COMMIT. For an empty fresh DB this is trivial. For the `heavy.db` fixture (100k rows of capture data + WAL state), the checkpoint on migration COMMIT will merge the existing WAL into the main DB, which can inflate migration time well beyond "DDL cost." The <30s CI budget assumes the migration is DDL-bound; a dirty WAL at the time of migration is DB-size-bound.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:399` — `TestMigration_V1_to_V2_Heavy ... <30s CI`.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:404` — `TestMigration_WALPreserved` — only verifies `journal_mode` is still `wal`, not that WAL size/checkpoint behavior is sane.
  - FEAT-0008 doesn't specify, but SQLite's `PRAGMA wal_checkpoint(TRUNCATE)` is the standard "reset WAL" op; a non-truncated WAL after migration could leave a large -wal sidecar file confusing users.
- **Recommended disposition:** Document that the `heavy.db.gz` fixture is created with a **clean** WAL (`PRAGMA wal_checkpoint(TRUNCATE)` before gzipping) so the migration time budget is predictable. Add a test assertion that post-migration WAL size is < 10MB (proxy for "checkpoint happened"). Optionally, run `PRAGMA wal_checkpoint(PASSIVE)` at the end of `migrateToV2`.

### A-09. Downgrade guard ordering can trip on the existing `migrate()` call site if not refactored

- **What:** `NewSQLiteStore` (existing `sqlite.go:46-58`) calls `enableWAL()` then `migrate()`. The design's D1 guard is "`NewSQLiteStore` rejects DBs whose `user_version` is higher..." — this is correct IF the guard runs before `migrate()`. Existing code runs `migrate()` directly without checking version. A Backend Implementer reading D1 may interpret "downgrade guard in `NewSQLiteStore`" as "somewhere in NewSQLiteStore" and inadvertently add it *after* `migrate()` — at which point `migrate()` has already run a no-op (v2 code sees `user_version=2` and does nothing) but also has not verified the DB is safe to use. The guard must be placed between `enableWAL` and `migrate`, AND `migrate` itself must be able to bail early if `currentSchemaVersion() > MaxKnownSchemaVersion`.
- **Evidence:**
  - `internal/storage/sqlite.go:46-58` — existing call order.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:65-73` — downgrade guard code.
  - `docs/history/2026-04-16-design-storage-045-091-096.md:454` — File Layout note says "downgrade guard in `NewSQLiteStore`, `PRAGMA foreign_keys = ON` after open" — ordering relative to `migrate()` unspecified.
- **Recommended disposition:** Explicitly order the steps in D1 / File Layout: `Open` → `Ping` → `enableWAL` → `enableFK` → `currentSchemaVersion()` → **guard check** (abort if > max) → `migrate(version)`. The guard check is duplicated at the top of `migrate()` for defense-in-depth. Show this as pseudocode in the design rather than leaving the order to the implementer.

### A-10. `turns.sequence` ordering for compacted rows is ambiguous without uniqueness

- **What:** D2 rejects `UNIQUE(session_id, sequence)` because compacted rows span multiple sequence numbers in `original_turns`. But `ListTurns(sessionID)` is spec'd to return rows "ordered by sequence" (line 298). For a session with turns [1, 2, 3, 4, 5, 6, 7] where 4-7 are compacted into a row with `sequence=4, original_turns=[4,5,6,7]`, `ORDER BY sequence` places the compacted row after turn 3. But the compacted row's logical position spans 4-7 — the next non-compacted turn (say, turn 8) sorts after it with `sequence=8`, which is correct. However, if a *second* compaction collapses the compacted-4 row plus turns 8-10 into a new row, what `sequence` does the new row get? The design says `sequence` reflects "where the collapsed block *starts*" — so the new compacted row has `sequence=4` again, colliding with the prior compacted row.
- **Evidence:**
  - `docs/history/2026-04-16-design-storage-045-091-096.md:143-144` — "compacted turns store multiple sequence numbers ... and the `sequence` column on a compacted row reflects where the collapsed block *starts*."
  - `docs/history/2026-04-16-design-storage-045-091-096.md:298` — `ListTurns` "ordered by sequence".
- **Recommended disposition:** Either (a) add a secondary sort column `created_at` to disambiguate (`ORDER BY sequence, created_at`), which gives stable ordering when two rows share a `sequence`, or (b) re-compact strategy: prior compacted rows are deleted and replaced by a single new compacted row that subsumes them, so no collision occurs. Option (b) matches the FEAT-0008 implication (compaction is lossless in storage, but live-context replacement is the unit) but deleting the prior compacted row loses the two-level summary trail. Option (a) is simpler and preserves compaction history. Pick one.

## Nit findings

### N-01. Deviations-from-spec section contains action items that should be committed

- **What:** D "Deviations" section lists three items as "TPM should update spec" action items. Two of these (`ListTurns` vs. `GetSessionTurns`, `ServerEvent` → `ServerSessionEvent`) are editorial consequences of the protocol-types rename that already happened in the sibling bundle. They should be landed as spec amendments alongside this design, not flagged as pending work.
- **Evidence:** `docs/history/2026-04-16-design-storage-045-091-096.md:488-494`.
- **Recommended disposition:** File a follow-on commit to `docs/releases/v0.2.0/track-0-shared.md:109` that reflects the final method names and type names. No need to wait for the Implementer.

### N-02. File Layout omits `internal/storage/migrate.go`

- **What:** The design introduces a materially new migration infrastructure (`currentSchemaVersion`, `migrateToV1`, `migrateToV2`, `MaxKnownSchemaVersion`) but the "Modified (WU-045 + WU-091)" section lists only `sqlite.go`. A separate `migrate.go` file is a natural home for all this new surface and will make WU-096 tests easier to import.
- **Evidence:** `docs/history/2026-04-16-design-storage-045-091-096.md:442-454`.
- **Recommended disposition:** Add `internal/storage/migrate.go` to the File Layout new-files list. Keep `sqlite.go:NewSQLiteStore` thin — it calls into `migrate.go`.

### N-03. `MaxKnownSchemaVersion` placement

- **What:** Declared in D1 as `const` but the surrounding context is the migration function. A package-level public const (`storage.MaxKnownSchemaVersion`) is more useful for downstream tools and tests.
- **Recommended disposition:** Place it in `store.go` as `const MaxKnownSchemaVersion = 2` so Go-doc tooling picks it up and the value is importable. Reference it from `migrate.go`.

### N-04. Storage spec doesn't cover the `turn.submit` history-append hook

- **What:** WU-091 spec (track-a-bff-server.md:236) says "Hook `session.resume` and `turn.submit` in WU-050/WU-051 so every user turn is captured." This design mentions it once (line 21) as deferred. The `command_history.session_id` being nullable suggests draft-capture, but the *automatic append on turn.submit* path is invisible here. A one-liner in the Scope section clarifying that the storage primitive is sufficient for both paths (automatic-via-turn-submit-in-WU-050, and manual-via-history.append) would close the loop.
- **Recommended disposition:** Add a Scope bullet: "WU-050 will call `AppendCommandHistory` automatically on each `turn.submit`; the `history.append` handler is the manual path for drafts. Both use the same storage method with the same idempotency (by `turn_id`-equivalent, plus duplicate-append tolerance at the application layer)."

### N-05. D1 `MaxKnownSchemaVersion = 2` but future v3 migration needs a stepwise path not shown

- **What:** D1's version-step pattern handles v0 → v1 → v2 via `if version < N` branches. A future v3 migration adds `if version < 3 { migrateToV3() }`. The design doesn't say so, but it implies that every future migration bumps `MaxKnownSchemaVersion` in lock-step. This is a convention worth stating once rather than discovering later.
- **Recommended disposition:** Add one sentence to D1: "Future migrations add a `migrateToVN` function, an `if version < N` branch in `migrate()`, and bump `MaxKnownSchemaVersion` to N. No squashing of prior migrations."

## Coverage Table

| Area checked | Finding status | Refs |
|---|---|---|
| WU-045 `sessions` columns vs track-0 spec | Complete (17/17) | spec lines 113-131 |
| WU-045 `turns` columns vs track-0 spec | Complete (17/17) | spec lines 133-152 |
| WU-045 `session_events` columns vs track-0 spec | Complete (6/6); payload-key schema undefined | B-04 |
| WU-091 `command_history` columns vs track-a spec | Complete (6/6) | spec line 231 |
| Store interface method coverage vs track-0 spec | Superset; 3 deviations (N-01) | spec line 109 |
| Go type fields for `Session`, `Turn` | Complete | D3 |
| `SessionFilter` vs consumer needs (WU-050) | `Status` typing weak (A-04); `UserID` required only in prose (A-06) | — |
| `CommandHistoryFilter` vs protocol handler | Missing explicit `Scope` (A-05) | — |
| `SessionSummary` projection completeness | `LastTurnSummary` query path undefined (A-01); otherwise complete | D4 |
| Migration v1 retrofit correctness | Non-atomic retrofit contradicts D1 atomicity claim (B-02) | — |
| Migration v2 fresh-DB correctness | Appears correct | — |
| WAL / transaction interaction | Heavy-fixture checkpoint not addressed (A-08) | — |
| `PRAGMA foreign_keys` pool semantics | Broken (B-03) | — |
| Downgrade guard correctness | Code snippet correct; ordering risk (A-09) | — |
| Lock SQL: contention, stale | Covered | D5 |
| Lock SQL: self re-acquire (same owner, fresh) | Not supported (A-02) | — |
| Lock SQL: self re-acquire (same owner, expired) | Works by accident | A-02 |
| Force-release w/ active streaming turn | Mentioned at spec level, not enforced in storage layer (by design; admin handler gate is in WU-050) | Scope line 21 |
| Command history user_id leakage | Enforced in query (D6); index mismatch (A-03) | — |
| Command history pagination cursor | Timestamp ties unresolved (A-07) | — |
| Protocol ↔ storage type consistency: `SessionSummary` | Documented divergence; converter not blocked | D4 note |
| Protocol ↔ storage type consistency: `SessionDetail` | Implicit via `SessionSummaries` + `SessionFilesTouched`/`Modified` | — |
| Protocol ↔ storage type consistency: `ServerSessionEvent` | Shape mismatch with no converter schema (B-04) | — |
| In-flight / multi-model branch persistence | Unaddressed (B-01) | — |
| `turns` ordering for compaction passes | Ambiguous (A-10) | — |
| Retention `DeleteSessionsBefore` cascade | Depends on FK fix (B-03) | D9 |
| WU-096 fixture provenance (generator) | Covered | D7 |
| WU-096 scenarios | 8 tests enumerated; `TestMigration_InterruptedMidV2` crash-simulation approach questioned (relies on intentionally-aborted tx, feasible) | D8 |
| WU-096 crash-model for `partial_v1.db` | Model contradiction with B-02 | — |
| Error sentinels | `ErrSessionNotFound`, `ErrSessionLockContended`, `ErrSchemaTooNew`; `ErrMissingUserID` missing (A-06) | D10 |

## What I did NOT review

- **Actual `internal/bff/session.go` converter code** — WU-050's scope. The B-04 finding flags that the converter's input schema is undefined; the converter's correctness is for the WU-050 design review.
- **End-to-end session.sync semantics** — FEAT-0008 ambiguity around BFF-restart recovery is flagged (B-01) but the full state-machine belongs to the WU-064 design.
- **Retention configuration** — D9 defers to FEAT-0008 configuration; I assumed the existing `retention.sessions: 90d` convention holds and did not verify config wiring.
- **`sqlite_test.go` helper changes** — the new migration tests will require a way to open a DB at a specific starting `user_version`; the existing `newTestStore(t)` always ends at max. Implementation detail, not design.
- **Concurrent-write correctness under `SetMaxOpenConns`** — if B-03 is resolved via `SetMaxOpenConns(1)`, deadlock risk from long-running reads blocking writes is a separate question. Not in scope for a schema bundle.
- **Secrets / SQL injection in new queries** — all queries use parameters (`?`). Glanced at them; nothing looked suspicious. A formal security-tier review is a separate pass.
- **The `ADR-0002` scoring matrix or storage-provider-alternative story** — ADR is accepted and the design honors it; I did not re-derive.
- **Fixture generator reproducibility** — D7 asserts determinism via fixed seeds. I trust this claim; verifying would require running the generator.
- **CLI interactions** (WU-065, `modeltap session unlock`) — out of scope for this bundle.
- **Cross-bundle deviations in the protocol-types design or provider-formatting design** — I referenced them for consistency checks only; any findings on those bundles belong in their own reviews.
