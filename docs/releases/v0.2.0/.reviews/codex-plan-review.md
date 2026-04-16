# v0.2.0 Plan Review

Reviewed:

- `docs/releases/v0.2.0/plan.md`
- `docs/releases/v0.2.0/track-0-shared.md`
- `docs/releases/v0.2.0/track-a-bff-server.md`
- `docs/releases/v0.2.0/track-b-terminal-harness.md`
- `docs/releases/v0.2.0/track-integration.md`

Cross-checked against:

- `docs/features/0008-bff-server.md`
- `docs/features/0009-terminal-harness.md`
- `docs/history/2026-04-15-feat-0008-connectivity-review.md`
- `docs/history/2026-04-15-feat-0008-0009-interdependency-review.md`

## Findings Summary

- 5 findings total
- 4 blocking
- 1 significant

## Findings

### 1. Blocking — `capabilities.register` work is incomplete in the plan

The release plan does not assign explicit work for protocol-version negotiation or project-context handling during `capabilities.register`, even though FEAT-0008 makes both part of connection establishment and session scoping.

In the current plan, WU-049 is limited to tool-catalog registration and schema validation. Its definition of done does not mention:

- supported-version negotiation
- version-mismatch rejection
- project-root capture
- config-content capture for Layer 4 prompt assembly
- persistence or in-memory ownership of project context per connection/session

That leaves a gap between the accepted feature contract and the release work breakdown.

### 2. Blocking — WU-045 underspecifies the storage schema

WU-045 only commits to generic `sessions` and `turns` tables plus CRUD methods, but FEAT-0008 requires significantly more state to persist and return.

The accepted feature expects session persistence and session-detail payloads to cover at least:

- project association
- routing overrides
- pinned items
- token totals
- compaction state
- files touched and modified
- server events

If those fields are not explicitly part of Track 0, Track A is likely to reopen the schema and migration design midstream.

### 3. Blocking — server CLI coverage is short of the accepted feature

WU-065 includes `modeltap serve`, `modeltap server status`, and `modeltap session unlock`, but FEAT-0008 also defines:

- `modeltap server sessions`
- `modeltap server session <id>`

Those operator-facing commands are part of the feature contract and should have an owning work unit or be folded explicitly into WU-065.

### 4. Blocking — cross-session command history has no owning work unit

FEAT-0009 requires command history traversal sourced from the BFF across sessions/projects. The release plan does not allocate server or harness work for that behavior.

WU-070 mentions local input history traversal, but nothing in Track A or Track B defines:

- a protocol method for history access
- storage or indexing of command history
- server-side history scoping rules
- harness-side fetch and replay behavior

As written, the release plan cannot satisfy that success criterion.

### 5. Significant — Track 0 is treated as a universal gate when part of Track B is harness-local

The top-level plan says Track 0 must complete before Track A or Track B begins, but several Track B work units only depend on WU-039 or on harness-local scaffolding.

That means the current gating rule delays work such as:

- Bubbletea scaffold
- viewport and markdown rendering
- tool framework
- local tool implementations

behind provider formatting and storage work that those units do not actually need. This increases schedule length without obviously reducing risk.

## Recommended Changes

1. Expand WU-049 or add a dedicated WU for protocol negotiation and project-context ownership.
2. Expand WU-045 to name the required persisted session fields and any supporting tables needed for `session.details`.
3. Add the missing server CLI surfaces to WU-065 or a new adjacent work unit.
4. Add a dedicated work unit for BFF-backed command history, or remove that requirement from FEAT-0009 before implementation.
5. Relax the blanket Track 0 gate so harness-local Track B work can begin after the protocol contract is stable enough for that slice.
