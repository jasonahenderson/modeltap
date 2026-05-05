# 2026-05-05 - Design: Run Runtime ADR (WU-108)

## Scope

This design covers WU-108 only: the run-runtime ADR that constrains v0.3.0
implementation and the downstream v0.3.x release train.

The ADR draft lives at `docs/adr/0015-run-runtime.md`.

## Inputs

- `docs/features/0015-professional-harness-runtime.md`
- `docs/features/0016-managed-codegen-run-pipeline.md`
- `docs/features/0017-durable-runs-and-background-agents.md`
- `docs/adr/0014-harness-base-strategy.md`
- `internal/bff/turn.go`
- `internal/protocol/messages.go`
- `internal/protocol/events.go`
- `internal/harnesshost/runtime.go`
- `internal/harnessshell/types.go`

## Decisions Captured

ADR-0015 decides:

- BFF is authoritative for run identity, lifecycle, stage, attachment state,
  checkpoint, and event sequencing.
- Harness is authoritative for local facts and executor availability, but those
  facts become canonical only after BFF integration.
- `turn.submit` remains a compatibility method and creates a foreground run
  before provider dispatch.
- New control and inspection surfaces use `run.*`.
- Local side effects require a connected harness/local executor in v0.3.0.
- Detached or disconnected runs may continue only through BFF-safe stages.
- Run events are append-only and monotonically sequenced per run.
- Checkpoints are written with lifecycle transitions and reserve extension JSON
  for context, artifacts, policy, workspace, memory, and routing.
- `workflow_type` is introduced on run records with the FEAT-0015 enum.

## Implementation Constraints for Later WUs

WU-109 must map the ADR vocabulary directly into SQLite tables and Go structs.
Status, stage, attachment state, workflow type, event type, and checkpoint schema
must be closed-string constants in `internal/protocol` or a BFF-owned runtime
package before storage code uses them.

WU-110 must keep `turn.submit` wire compatibility. A v0.2.x harness may submit a
turn and receive the existing accept response, while a v0.3.0 harness can
observe the related run through `run.*` methods and events.

WU-111 through WU-113 must write the run row, first lifecycle event, and initial
checkpoint before provider dispatch. If this fails, `turn.submit` fails before
any model call starts.

WU-114 through WU-116 must treat shell `RunID` as the BFF run ID rather than a
local placeholder. Detached run transcript streams must not append directly to
the active foreground transcript.

## Acceptance Checks

- ADR status is `proposed` during Phase 1 and must become `accepted` before
  Phase 3 starts.
- The ADR has explicit answers for executor availability, checkpoint minimums,
  event ordering, status/stage/attachment vocabulary, and workflow type.
- No Phase 3 implementation WU may introduce a lifecycle state or stage that is
  not in ADR-0015 without revising the ADR first.
