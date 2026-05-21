# v0.3.0 Design Review — Codex

**Reviewer:** Codex
**Date:** 2026-05-05
**Phase reviewed:** Phase 2 — Review
**Scope:** v0.3.0 WU-108 through WU-117 designs and ADR-0015 against
FEAT-0015, FEAT-0016, FEAT-0017, and ADR-0014.

## Verdict

Proceed after revisions. The design set has the right BFF-owned runtime shape
and preserves the existing `turn.submit` compatibility path, but five issues
need disposition before Phase 2 can close.

## Findings

### F1 — Permission resolution is not a run protocol surface

Severity: significant

FEAT-0015 and FEAT-0017 require pending permission/user-input state to be
run-correlated, and FEAT-0017 lists "resolve pending permission for a run" as an
expected protocol/API surface. WU-110 defines run list/details/control/replay,
but no permission list or resolution method. WU-113 mentions
`waiting_permission`, but the design does not say how a foreground run exits
that state.

Recommendation: add `run.permissions` and `run.resolve_permission`, or
explicitly bind run permission resolution to existing `tool.result`/permission
plumbing with run ID correlation. Include tests.

### F2 — `turn.submit` run/turn persistence order can become inconsistent

Severity: significant

WU-113's flow links a user turn to the run before appending the user turn to
storage. If user-turn persistence fails after run creation/linking, the store
can retain a run link to a missing turn, or an accepted run without the
initiating turn. The design says run creation must happen before provider
dispatch, but it needs a transaction boundary for run creation, turn creation,
and run-turn linking.

Recommendation: make the first durable transaction create the run, initial run
event/checkpoint, user turn, and run-turn link before provider dispatch.

### F3 — Idempotency boundaries are incomplete

Severity: significant

ADR-0015 and WU-113 cover run idempotency keys, but FEAT-0015 also requires
tool-result delivery to be idempotent per `tool_call_id` and model-call
accounting to be idempotent per `model_call_id`. WU-109 has
`model_call_ids_json` and `pending_tool_call_ids_json`, but no model-call or
tool-result uniqueness boundary.

Recommendation: add run-scoped `run_model_calls` and `run_tool_results` (or
equivalent unique keys) to the storage design, and require BFF tests for
duplicate model accounting/tool result delivery.

### F4 — Attachment state is duplicated without an authority invariant

Severity: advisory

WU-109 stores attachment state in both `runs` and `run_attachments`. This can be
reasonable if one is a denormalized summary, but the design does not identify
the authority or transaction rule. Drift would break attach/list behavior.

Recommendation: state that `runs.attachment_state` is the list/detail summary
projection and `run_attachments` is the lease detail, updated in the same
transaction through one storage method.

### F5 — Run queue "stuck" and input-required semantics are not represented

Severity: advisory

FEAT-0017 asks run queue rows to show whether user input is required and a
`stuck` badge when no stage/event advance occurs for a configurable interval.
WU-110 says summaries include whether input is required, but no stuck/deadline
fields. WU-114 says `/runs` renders rows but not how the badge is derived.

Recommendation: add `last_advanced_at`, `input_required`, and `stuck` summary
semantics, with v0.3.0 deriving stuck from last event/stage timestamp and a
default threshold.

## Disposition

Processed in `WU-110: process v0.3.0 design review`.

| Finding | Disposition |
|---|---|
| F1 | Accepted; WU-110 now defines `run.permissions` and `run.resolve_permission`, and WU-117 adds permission-resolution tests. |
| F2 | Accepted; WU-113 now requires one durable pre-dispatch transaction for run creation, user turn persistence, run-turn linking, initial event, and checkpoint. |
| F3 | Accepted; WU-109 now adds `run_model_calls` and `run_tool_results` uniqueness boundaries, with WU-117 tests. |
| F4 | Accepted; WU-109 now states the attachment summary/lease invariant and same-transaction update rule. |
| F5 | Accepted; WU-110/WU-114 now define input-required and stuck summary semantics, with WU-117 tests. |
