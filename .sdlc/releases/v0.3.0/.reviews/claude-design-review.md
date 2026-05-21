# v0.3.0 Design Review — Claude

**Reviewer:** Claude Opus 4.7 (1M context), in-conversation peer review delegated to a research subagent for the cross-doc read
**Date:** 2026-05-05
**Phase reviewed:** Phase 2 — Review
**Scope:** v0.3.0 WU-108 through WU-117 designs and ADR-0015 against
FEAT-0015 (umbrella), FEAT-0016 (managed codegen run pipeline), and the
FEAT-0017 foundation slice. Cross-checked against the Codex design review
(`codex-design-review.md`) and its 5 accepted dispositions.

## Verdict

Proceed after revisions. Designs are substantially aligned with FEAT-0016
and the FEAT-0017 foundation slice. Vocabulary (statuses, stages,
attachment states, control verbs, `workflow_type`) is consistent across
all six docs. The five issues Codex found are accepted and applied
(spot-checked by grep against the design files). The single remaining
risk is the **FEAT-0015 liveness / observability axis**: trace IDs,
heartbeats, stage deadlines, stuck-stage detection, fsync boundaries,
and run-family budget composition are required by the umbrella feature
but are essentially absent from every v0.3.0 design and not explicitly
deferred in the plan. A secondary risk is two protocol-shape gaps that
Codex did not surface: a missing `run.create` method (so `queued` is a
status no caller can produce) and `run.blocked`/`run.unblocked` event
naming that diverges from FEAT-0016.

## Method

The review delegated the cross-doc read to a fresh subagent with no
conversation context, given only the file list and the foundation-scope
constraints (FEAT-0017 v0.3.3 deferrals; `context_plan` / `validation`
/ `artifact_capture` no-op for v0.3.0). The subagent produced a
~1,500-word coverage matrix; I then filtered findings against the
already-dispositioned Codex review to surface only net-new items.

## Codex review reconciliation

Codex's five Phase 2 findings (F1–F5) were accepted; spot-check via
grep confirms the dispositions are reflected in the design files:

| Codex finding | Accepted disposition | Verified present in |
|---|---|---|
| F1 — `run.permissions` / `run.resolve_permission` missing | Add methods, add tests | `2026-05-05-design-run-protocol-110.md`, `2026-05-05-design-runtime-foundation-verification-117.md` |
| F2 — run/turn persistence transaction boundary | One durable pre-dispatch transaction | `2026-05-05-design-bff-run-runtime-111-113.md` |
| F3 — tool/model call idempotency | Add `run_model_calls`, `run_tool_results` tables + tests | `2026-05-05-design-bff-run-runtime-111-113.md`, `2026-05-05-design-run-storage-109.md` |
| F4 — attachment authority/transaction rule | `runs.attachment_state` is summary projection, `run_attachments` is lease detail, same transaction | `2026-05-05-design-run-storage-109.md` |
| F5 — stuck/input-required summary semantics | `last_advanced_at`, `input_required`, `stuck` defined | `2026-05-05-design-run-protocol-110.md`, `2026-05-05-design-run-storage-109.md`, `2026-05-05-design-harness-run-surface-114-116.md` |

Findings below are **net-new** relative to Codex.

## Findings

### F6 — FEAT-0015 observability axis is essentially undesigned

Severity: significant

FEAT-0015 §Observability requires a trace ID per run propagated across
BFF / harness / model-provider calls; heartbeats from harness/executor;
stuck-stage detection; per-stage failure rates; queue depth metrics;
warning thresholds and hard deadlines per stage; an explicit
`stage_timeout` failure reason. None of the six v0.3.0 designs carry
any of this. WU-109's `runs` schema has no `trace_id` column. WU-110
has no heartbeat method and no stage-deadline event. WU-111-113
enumerates forward stage transitions but never a deadline-driven
`failed` transition. Codex F5 added `stuck` summary fields but the
underlying threshold/deadline mechanics are not specified.

The plan does not explicitly defer these, so by the plan's "does not
cover" rule they are in scope.

Recommendation: ADR-0015 declares the v0.3.0 observability scope:

- Reserve `runs.trace_id TEXT` (nullable, opaque) in WU-109.
- Add a `heartbeat` lifecycle protocol method (or extend
  `connection.health`) in WU-110, scoped to attached runs.
- Define `stage_warning_threshold` / `stage_hard_deadline` per stage in
  the ADR, with v0.3.0 implementing the deadline → `failed` transition
  with reason `stage_timeout` for at least the `model_call` and
  `tool_loop` stages.
- Add WU-117 tests for trace-ID propagation, heartbeat liveness, and
  one stage-timeout transition.

If the scope is too large for v0.3.0, the plan's "does not cover" list
should add observability axis explicitly and FEAT-0015 should record
which release owns it.

### F7 — No `run.create` / `run.start` protocol method; `queued` is unreachable

Severity: significant

WU-110 §Methods has `list / details / attach / detach / cancel /
retry / continue / fork / events / permissions / resolve_permission`
plus the `turn.submit` compatibility surface. There is no run-native
creation method. ADR-0015 says "every `turn.submit` creates or
continues a foreground run" and WU-111-113 says creation goes
`queued → preflight`. But no caller can produce a `queued` run
without flowing through `turn.submit`, which auto-advances to
`preflight`.

This is consistent today (no non-turn callers exist) but locks the
extension point: forks and synthesis runs are server-side creations
triggered by control methods, not the same as a client-driven create.
FEAT-0017 §Run Queue describes queued-but-not-yet-running entries as
a user-visible state. Without `run.create`, the only way to surface a
`queued` run to a user is server-side scheduling, which is a v0.3.3
concern.

Recommendation: either (a) add `run.create` to WU-110 with
`status="queued"` as the initial state and explicit transition to
`preflight` only when a caller starts the run, or (b) drop `queued`
from the v0.3.0 status enum and note it as a v0.3.3 addition tied to
background scheduling.

### F8 — `run.blocked` / `run.unblocked` events vs. `run.status_changed` framing

Severity: advisory

FEAT-0016 §Pipeline Events lists "run completed, failed, cancelled, or
**blocked**" as top-level event categories. WU-110 §Events does not
include `run.blocked`; instead it relies on `run.status_changed`
carrying `waiting_permission` or `waiting_user`. This is a reasonable
design call but is silent drift from the feature spec.

Recommendation: pick one and align. Either add explicit
`run.blocked` / `run.unblocked` events to WU-110 (cleaner consumer
ergonomics, harness can route blocked-run UI without parsing the
status enum), or revise FEAT-0016's event list to match the
`status_changed` framing.

### F9 — fsync boundaries and N-1 checkpoint compatibility not declared

Severity: advisory

FEAT-0015 §Observability calls for "exact fsync boundaries" and
"checkpoint-format compatibility across at least the previous minor
version" as part of the run-runtime ADR. ADR-0015 §Checkpoint
Semantics says "Checkpoint writes are part of the same SQLite
transaction as the lifecycle event that advances the run state" —
that covers transactional atomicity but not fsync semantics, and not
the N-1 compatibility window. WU-109 carries `schema_version` columns
on `runs` and `run_checkpoints` but no rule about reading rows
written under previous versions.

Recommendation: ADR-0015 declares (a) fsync occurs at SQLite
transaction commit boundaries with WAL mode (the existing storage
contract), and (b) v0.3.0 readers must accept `schema_version = 1`
records; subsequent minor releases must read at minimum v0.3.0
records. Add a WU-117 test that loads a v1 record under a future
schema version.

### F10 — Run-family budgets, deadlines, and cancellation cascade not designed

Severity: advisory

FEAT-0015 §Run Family says budgets and deadlines compose downward
from parent to child runs, and cancellation cascades to children
unless explicitly opted out. WU-109 has `parent_run_id` but no budget
or deadline columns and no cascade behavior. ADR-0015 lists no
cancellation-cascade rule. Today no runs have children, so this is a
forward-compatibility concern, not a live behavior gap.

Recommendation: either reserve `parent_run_id`-correlated budget /
deadline columns on `runs` now (low cost, avoids a later migration),
or note in the plan's "does not cover" list that family budgets and
cancellation cascade are deferred to the release that introduces
sub-agent runs (likely v0.3.3 or later).

### F11 — Cost attribution granularity is run-aggregate only

Severity: advisory

FEAT-0016 §Run Outcome Attribution requires cost/usage at three
levels: per `model_call_id`, per `tool_call_id`, per stage, with run
totals derived. WU-109's `runs` table stores aggregate
`input_tokens` / `output_tokens` / `total_cost` only. Codex's F3
added `run_model_calls` and `run_tool_results` tables for
*idempotency* but not for cost columns. Per-stage aggregation does
not exist.

Recommendation: extend `run_model_calls` / `run_tool_results` to
include `input_tokens`, `output_tokens`, `cost`, `latency_ms`, and
`stage`. Optionally derive per-stage aggregation as a query rather
than a table. Either way, WU-117 should assert run totals match
the sum across model + tool calls.

### F12 — State-graph reentry edges from FEAT-0016 are not explicit

Severity: advisory

FEAT-0016 §State Graph defines legal reentry edges
(`tool_loop → prompt_plan`, `validation → model_call`,
`validation → prompt_plan`, `model_call → preflight`, terminal from
any). WU-111-113 enumerates *forward* transitions only. Most reentry
edges target stages that are no-op in v0.3.0 (`validation`,
`context_plan`), so the practical gap is small — but
`tool_loop → prompt_plan` and `model_call → preflight` are reachable
in v0.3.0 (a tool result triggers a new prompt assembly; a transient
provider failure could reset to preflight) and are not asserted.

Recommendation: WU-111-113 enumerates the legal reentry edges
explicitly with an "active in v0.3.0" / "no-op v0.3.0" marker, and
WU-117 adds at least one test exercising the
`tool_loop → prompt_plan` path.

### F13 — `/run context | prompt | policy` subcommands are silently absent

Severity: advisory

FEAT-0016 §Inspection lists `/run context`, `/run prompt`, and
`/run policy` as part of the v0.3.0 inspection surface. WU-114-116
§Slash Commands has only `/run [run-id]`. The data those subcommands
expose (context plan, prompt assembly, policy decisions) is not
populated in v0.3.0, so the natural disposition is "stub them now and
let them surface 'not enabled in v0.3.0'." Today they are simply
missing.

Recommendation: WU-114-116 adds the three subcommands as
discoverable stubs returning a clear "not enabled in v0.3.0; see
v0.3.1+ release plan" message. Avoids drift from the feature and
gives discoverability for the harness slash-command catalog.

### F14 — Tool-loop parallel-call policy is not addressed

Severity: advisory

FEAT-0016 §Tool Loop says: tool calls execute in provider emission
order; non-conflicting calls may resolve in parallel; mutating
operations are serialized unless policy permits parallel. v0.3.0
designs do not address this. The behavior may default to v0.2.x
(single-call sequential), but the plan does not state that.

Recommendation: WU-111-113 states the v0.3.0 policy explicitly —
either "tool loop is sequential in v0.3.0; parallel resolution is a
v0.3.1+ concern under FEAT-0019" or define the v0.3.0 parallelism
rule.

### F15 — Minor consistency notes

Severity: trivial

- WU-109 `run_events` row has columns `stage`, `status`, `reason`,
  `payload_json`, `created_at` — but WU-110 §Events says every payload
  carries `run_id`, `seq`, `session_id`, `turn_id`. The
  `session_id` / `turn_id` projection comes from joins with `runs` /
  `run_turns` at read time, or is duplicated into `payload_json`. Worth
  one sentence in WU-109 stating which.
- ADR-0015's "minimum checkpoint fields" includes `workflow_type` and
  `attachment_state` as part of a checkpoint payload; WU-109's
  `run_checkpoints` columns do not include these directly (they live on
  `runs`). The two should reconcile — either ADR-0015 says checkpoints
  reference `runs` rather than duplicating, or WU-109 adds the columns.
- WU-109 says `idempotency_key TEXT NOT NULL UNIQUE` per
  `(user_id, project, idempotency_key)`. WU-111-113 says the BFF
  synthesizes a key as `turn:<session>:<turn>` for `turn.submit`
  callers. If F7's recommendation introduces `run.create`, the
  idempotency-key contract for non-turn callers needs to be stated.
- ADR-0015 says "Unknown workflow types are rejected at run creation
  in v0.3.0" but WU-109 `Tests` and WU-117 do not include a test for
  that rejection. Add one.

## Disposition

Processed in `WU-110: process remaining v0.3.0 design review`.

| Finding | Disposition |
|---|---|
| F6 (observability) | Accepted with v0.3.0 minimum scope. ADR-0015 now defines trace IDs, heartbeats, default `model_call`/`tool_loop` deadlines, and `stage_timeout`; WU-109/WU-110/WU-111-113/WU-117 reserve schema/protocol/runtime/tests. |
| F7 (`run.create`) | Accepted. WU-110 now defines `run.create`, and WU-111-113 defines queued run creation without provider dispatch. |
| F8 (`run.blocked` events) | Accepted. WU-110 now adds `run.blocked` and `run.unblocked` events while retaining `run.status_changed`. |
| F9 (fsync / N-1) | Accepted. ADR-0015 now declares SQLite transaction commit as the fsync boundary and requires v0.3.0 checkpoint schema-version compatibility. |
| F10 (run-family budgets) | Accepted and deferred. ADR-0015 and the v0.3.0 plan now defer budget inheritance, inherited deadlines, and cancellation cascade until child/sub-agent execution lands. |
| F11 (cost granularity) | Accepted. WU-109 now extends model-call/tool-result accounting and states per-stage aggregation is derived by query; WU-117 adds tests. |
| F12 (reentry edges) | Accepted. WU-111-113 now enumerates legal reentry edges and marks v0.3.0-active versus inactive edges. |
| F13 (`/run` subcommands) | Accepted. WU-114-116 now adds discoverable `/run context`, `/run prompt`, and `/run policy` stubs. |
| F14 (parallel tool calls) | Accepted. WU-111-113 now states v0.3.0 uses sequential provider-emission-order tool-loop processing. |
| F15 (consistency) | Accepted. WU-109, ADR-0015, and WU-117 now clarify protocol event projection, checkpoint/run field ownership, non-turn idempotency, workflow-type rejection tests, and event payload schema versions. |

## Forward-compatibility check (FEAT-0018/0019/0020/0021/0022)

Generally clean:

- **FEAT-0018 (context planner):** `payload_json.context` reserved in
  `run_checkpoints`; `context_plan` stage enumerated and no-op.
- **FEAT-0019 (validation):** `validation` stage no-op;
  `payload_json.policy` / `workspace` reserved.
- **FEAT-0020 (artifacts):** `runs.id`, `run_events.seq` stable;
  `run.artifact_recorded` event reserved. Risk: no retention metadata
  on `runs` rows; FEAT-0017 retention envelope ("memory can outlive
  artifacts") needs `expires_at` / `retention_class` reservation.
- **FEAT-0021 (policy):** `payload_json.policy` reserved;
  `decision_id` / `policy_version` from FEAT-0015 identity table not
  pre-reserved. Acceptable — later release adds tables.
- **FEAT-0022 (memory/routing):** Run records carry `workflow_type`,
  terminal outcome, model/provider, cost, token totals — all that
  FEAT-0022 declares it needs.

Direction-closing risk: `run_events.payload_json` is one TEXT blob
with no per-event-type schema. v0.3.0 declines to enumerate per-event
schemas. Reasonable, but FEAT-0020 / FEAT-0021 will need committed
event-payload schemas — there is no per-event-version field today.
Worth a Phase 2 note even if not actioned in v0.3.0.
