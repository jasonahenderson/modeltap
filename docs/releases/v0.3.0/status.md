# v0.3.0 — Status

**Current phase:** Phase 3 — Implementation

**Branch:** `release/v0.3.0`  
**Scope:** Run runtime foundation for FEAT-0016 and the first slice of
FEAT-0017

Phase 1 opened on 2026-05-05 by explicit `ADMIN:` commit and completed after all
WU-108 through WU-117 designs were drafted. Phase 2 opened on 2026-05-05 by
explicit `ADMIN:` commit. Phase 2 design findings:

- `docs/releases/v0.3.0/.reviews/codex-design-review.md` — 5 findings (F1–F5)
  accepted and applied to the designs.
- `docs/releases/v0.3.0/.reviews/claude-design-review.md` — 10 net-new findings
  (F6–F15), accepted and applied or explicitly deferred.

The complete release design index for user review is
`docs/releases/v0.3.0/designs/README.md`.

Phase 3 opened on 2026-05-05 by explicit `ADMIN:` commit after accepting
FEAT-0015, FEAT-0016, the FEAT-0017 foundation scope, and ADR-0015. The v0.2.x
prerequisite BFF/harness surfaces were confirmed reachable on `release/v0.3.0`
before the phase transition.

## Work Units

| WU | Title | Size | State | Design |
|---|---|---|---|---|
| 108 | Run runtime ADR | M | accepted | [designs/2026-05-05-design-run-runtime-adr-108.md](designs/2026-05-05-design-run-runtime-adr-108.md), [ADR-0015](../../adr/0015-run-runtime.md) |
| 109 | Run schema, storage, and migration design | M | implemented | [designs/2026-05-05-design-run-storage-109.md](designs/2026-05-05-design-run-storage-109.md) |
| 110 | Run protocol methods and event taxonomy | M | implemented | [designs/2026-05-05-design-run-protocol-110.md](designs/2026-05-05-design-run-protocol-110.md) |
| 111 | BFF run registry and lifecycle store | L | implemented, hardening pending | [designs/2026-05-05-design-bff-run-runtime-111-113.md](designs/2026-05-05-design-bff-run-runtime-111-113.md) |
| 112 | `turn.submit` to foreground-run integration | L | implemented | [designs/2026-05-05-design-bff-run-runtime-111-113.md](designs/2026-05-05-design-bff-run-runtime-111-113.md) |
| 113 | Pipeline stage/status emission and checkpoint metadata | M | implemented, coverage expanding | [designs/2026-05-05-design-bff-run-runtime-111-113.md](designs/2026-05-05-design-bff-run-runtime-111-113.md) |
| 114 | Harness run projection and active `/run` surface | M | implemented, coverage expanding | [designs/2026-05-05-design-harness-run-surface-114-116.md](designs/2026-05-05-design-harness-run-surface-114-116.md) |
| 115 | Run list, attach/detach/cancel/retry/continue/fork commands | L | implemented, hardening pending | [designs/2026-05-05-design-harness-run-surface-114-116.md](designs/2026-05-05-design-harness-run-surface-114-116.md) |
| 116 | Reconnect/resume behavior for active and detached runs | M | partially implemented | [designs/2026-05-05-design-harness-run-surface-114-116.md](designs/2026-05-05-design-harness-run-surface-114-116.md) |
| 117 | Runtime foundation tests and docs | M | in progress | [designs/2026-05-05-design-runtime-foundation-verification-117.md](designs/2026-05-05-design-runtime-foundation-verification-117.md) |

## Gates

- Phase 1 started by explicit `ADMIN:` release-open commit on 2026-05-05 with a
  design-against-draft exception for FEAT-0015/0016/0017 foundation scope.
- v0.3.0 Phase 1 design may depend on the committed BFF/harness contracts in
  `internal/protocol`, `internal/bff`, `internal/harness`,
  `internal/harnesshost`, `internal/harnessshell`, `internal/cli`, and
  `internal/storage` at the `release/v0.3.0` branch point.
- Phase 1 closes only after every WU has a design artifact.
- Phase 2 design reviews are recorded and findings are dispositioned.
- Phase 3 began by explicit Phase 2 -> Phase 3 `ADMIN:` commit after accepted
  FEAT-0015/0016/0017 foundation scope, accepted ADR-0015, and reachable
  v0.2.x prerequisite surfaces.

## Implementation Progress

- Added SQLite schema version 3 with durable run, event, checkpoint,
  attachment, model-call, tool-result, and run-turn link tables.
- Added run storage APIs and tests for workflow validation, event sequencing,
  replay, and idempotent model-call accounting.
- Added `run.*` protocol request/response types, run event payloads, and
  optional `run_id` on `turn.submit` responses.
- Wired BFF foreground `turn.submit` into durable runs with lifecycle events,
  checkpoints, run IDs, model-call accounting, and run cancellation.
- Added BFF `run.*` handlers for create, list, details, attach, detach,
  cancel, retry, continue, fork, events, permissions, permission resolution,
  and heartbeat with conservative v0.3.0 retry/continue behavior.
- Added production harness command routing for `/run`, `/runs`/`/jobs`,
  `/attach`, `/detach`, `/cancel`, `/retry`, `/continue`, and `/fork`.
- Added optional `run_id` correlation on legacy turn stream events so durable
  run IDs do not break foreground transcript projection.
- Added attach-conflict handling, exact detach lease clearing, startup/session
  run recovery summaries, and detached-run delta regression coverage.
- Updated user and embedding docs for run commands and run projection.

Validation: `go test ./...` passes.

## Open Items

- Persist and expose richer permission-blocker details.
- Complete full reconnect/resume replay projection and detached transcript
  buffer behavior for WU-116.
- Expand harness projection tests for attach replay and summary-only fidelity.
- Finalize release changelog and readiness review after all WUs close.
