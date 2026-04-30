# v0.3.0 — Plan Review (Claude Opus 4.7)

**Reviewer:** Claude Opus 4.7 (1M context), in-conversation peer review
**Date:** 2026-04-30
**Phase reviewed:** Planning draft (Phase 1 not yet opened)
**Scope:** v0.3.0 Run Runtime Foundation plan vs FEAT-0015 (umbrella), FEAT-0016, FEAT-0017
**Companion reviews:** v0.3.1, v0.3.2, v0.3.3, v0.3.4 — `claude-plan-review.md` in each

## Verdict

The v0.3.0 plan is structurally sound and traces to FEAT-0016 success criteria
cleanly. Two FEAT-0016 items are under-specified and three FEAT-0017
deferrals are implicit. None require restructuring; all are tractable
clarifications before Phase 1 opens.

## Success Criteria Trace

### FEAT-0016 (Managed Codegen Run Pipeline)

| SC | Trace | Status |
|---|---|---|
| #1 stable run ID before model dispatch | WU-111, WU-112 | covered |
| #2 BFF records and emits stage transitions | WU-111, WU-113 | covered |
| #3 tool calls, **model selection, cost**, terminal outcome correlated to run ID | WU-113 | partial — see Finding 1 |
| #4 harness renders stage/status, inspects metadata | WU-114 | covered |
| #5 interrupt/retry/continue/fork on run ID | WU-115 | covered (plan acknowledges shallow retry/continue) |
| #6 simple chat compatible | WU-112 | covered |

### FEAT-0017 (Durable Runs and Background Agents — foundation slice)

| SC | Trace | Status |
|---|---|---|
| #1 start/detach/list/reattach | WU-115, WU-116 | covered |
| #2 detached run has separate transcript | WU-114/WU-115 | implicit only — see Finding 2 |
| #3 background pause/auto-deny on unapproved side effects | deferred to v0.3.3 WU-144 | not stated explicitly — see Finding 3 |
| #4 blocked-run inbox | deferred to v0.3.3 WU-144 | not stated explicitly — see Finding 3 |
| #5 harness restart preserves runs | WU-116 | covered |
| #6 shared lifecycle/artifact model | foundation only | covered |

## Findings (release-local)

### Finding 1 — cost/usage/model-selection not enumerated in WU-113

FEAT-0016 SC#3 requires *"tool calls, model selection, cost, and terminal
outcome correlated with the run ID."* WU-113 is described as
*"stage/status emission and checkpoint metadata"* and does not call out
cost, token usage, or model selection capture. Without these, FEAT-0016
SC#3 is not met when v0.3.0 closes.

**Recommendation:** amend WU-113 description to include cost/usage and model
selection capture, or split a new WU-113b for usage/cost recording.

### Finding 2 — detached transcripts is a UX invariant without an explicit owner

FEAT-0017 §"Detached Transcripts" requires that returning to the main
conversation does not merge background-run chatter into the foreground
surface. v0.3.0 plan covers attached projection (WU-114) and run commands
(WU-115), but does not state where this invariant is tested.

**Recommendation:** add an explicit bullet to WU-114 ("project events for
detached runs into a separate per-run transcript stream") or WU-117 test
("foreground surface remains free of detached run events").

### Finding 3 — implicit FEAT-0017 deferrals should be explicit

FEAT-0017 SC#3 (background pause/auto-deny) and SC#4 (blocked-run inbox)
land in v0.3.3 WU-144. v0.3.0 plan §"This release does not cover" lists
"full background local-tool execution while no harness/executor is
connected" but does not explicitly defer SC#3 and SC#4.

**Recommendation:** add a sentence to v0.3.0 plan §"Scope" or a new
"Deferred FEAT success criteria" subsection naming v0.3.3 WU-144 as the
home for FEAT-0017 SC#3 and SC#4.

### Finding 4 — pipeline stage scope ambiguity

FEAT-0016 enumerates 9 stages. WU-113 emits *"preflight, prompt/model/tool
stages, completion, failure, cancellation, and checkpoint."* This implicitly
defers `context_plan` (v0.3.1), `validation` (v0.3.2), and
`artifact_capture` (v0.3.2) stage events.

**Recommendation:** state explicitly in WU-113 that `context_plan`,
`validation`, and `artifact_capture` stages emit no-op or absent events in
v0.3.0 and become functional in v0.3.1/v0.3.2.

### Finding 5 — `waiting_user` distinction needs an implementation owner

FEAT-0015 distinguishes `waiting_permission` from `waiting_user`. WU-108 ADR
will codify the status enum, but no implementation WU explicitly handles
`waiting_user` (non-permission user input pause). The harness command set
(WU-115) needs to surface both.

**Recommendation:** add a one-line note to WU-111 (lifecycle store) and
WU-115 (commands) confirming both statuses are first-class.

### Finding 6 — v0.2.x prerequisite framing

v0.3.0 §"Context" says *"v0.2.x established the production conversation
shell and BFF-backed harness plumbing."* Per `docs/releases/README.md`,
only v0.2.2 is `released`; v0.2.0 and v0.2.1 are still `planning`.

**Recommendation:** add a "Prerequisites" subsection naming v0.2.0,
v0.2.1, and v0.2.2 explicitly as Phase 3 gates, or replace the existing
sentence with a specific shipped-version reference.

## Cross-cutting concerns affecting v0.3.0

### `workflow_type` has no introduction WU

FEAT-0015 SC#6 requires workflow types to drive tool, validation, artifact,
and permission defaults. The 8 workflow types are referenced in v0.3.2
(WU-129 validation), v0.3.3 (WU-139 policy), and v0.3.4 (WU-153 profile
registry, gated). **No WU establishes the workflow_type field on a run nor
populates the enum.** The natural home is v0.3.0 (run lifecycle metadata).

**Recommendation:** add a small WU to v0.3.0 — e.g., "WU-118½: workflow_type
field on run records, default `implementation`, enum populated per
FEAT-0015 §Workflow Contracts."

### Workflow slash commands `/explore`, `/feature`, `/adr`, `/release`, `/implement`, `/debug`, `/docs`, `/devops`

Listed in FEAT-0015 §"Terminal UI" as part of the umbrella behavior contract
but not assigned to any v0.3.x release. Likely belong to FEAT-0012 (Skills)
or a future patch.

**Recommendation:** add an entry to v0.3.0 §"This release does not cover"
naming workflow slash commands and pointing to a future home (or v0.3.4
WU-153).

## Process notes

- The strict three-phase release cycle is honored.
- WU-108 ADR placement at the head of Track A is correct.
- The risk register correctly anticipates R2 (turn.submit compatibility) and
  R3 (disconnected execution) as ADR-driving questions.
- No process violations were found.

## Recommended pre-Phase-1 edits (priority order)

1. Add workflow_type introduction WU (cross-cutting Finding above).
2. Amend WU-113 to include cost/usage/model-selection capture (Finding 1).
3. State FEAT-0017 SC#3 and SC#4 deferrals explicitly (Finding 3).
4. Clarify pipeline stage scope in WU-113 (Finding 4).
5. Add Prerequisites subsection naming v0.2.x dependencies (Finding 6).
6. Add detached-transcript invariant test to WU-114 or WU-117 (Finding 2).
7. Add `waiting_user` callout in WU-111/WU-115 (Finding 5).
8. Add workflow slash command deferral note (cross-cutting).

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Finding | Disposition |
|---|---|
| Workflow type has no introduction WU | Accepted; folded into WU-109/WU-111 scope and v0.3.0 DoD without renumbering later WUs. |
| WU-113 missing cost/usage/model-selection capture | Accepted; WU-113 and DoD now require cost/token usage and model-selection metadata. |
| FEAT-0017 SC#3/SC#4 deferral unclear | Accepted; plan now explicitly defers background approval/blocked-operation criteria to v0.3.3 WU-144. |
| Pipeline stage activation ambiguity | Accepted; WU-113 now defines inactive/no-op downstream stages and points to v0.3.1/v0.3.2 activation. |
| v0.2.x prerequisite framing | Accepted; added v0.2.0/v0.2.1/v0.2.2 prerequisites and status gate. |
| Detached transcript invariant | Accepted; WU-114 and WU-117 now require the invariant and tests. |
| `waiting_user` missing | Accepted; WU-111/WU-115 now distinguish `waiting_permission` and `waiting_user`. |
| Workflow slash commands unassigned | Accepted as deferral; plan points command alignment to v0.3.4 WU-153 or a later split. |
