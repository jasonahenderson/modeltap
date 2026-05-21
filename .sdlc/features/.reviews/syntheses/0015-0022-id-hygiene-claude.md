# FEAT-0015 through FEAT-0022 — Identifier Hygiene Review

**Reviewer:** Claude Opus 4.7 (1M context)
**Date:** 2026-04-30
**Scope:** Identifier discipline across the Professional Harness Runtime umbrella (FEAT-0015) and its seven member features (FEAT-0016–0022).
**Status:** All specs are `draft`.

## Top-line summary

| total_findings | blocking | significant | advisory | top_line |
|---|---|---|---|---|
| 9 | 0 | 5 | 4 | `run_id` discipline is strong; session/turn/branch/tool-call/decision IDs need explicit naming and cardinality before Phase 1 opens. |

## Verdict

`run_id` is well-handled — every run-control verb across the eight features uses run identity correctly and never collapses run into session. The umbrella's command set (`/run`, `/runs`, `/attach <run-id>`, `/detach`, `/cancel <run-id>`, `/continue <run-id>`, `/retry <run-id>`, `/fork <run-id>`, `/artifacts <run-id>`) is internally consistent.

Five real ID-hygiene gaps remain. None are restructuring; all are clarification edits to the FEAT specs.

## ID inventory across the eight features

| ID | F-0015 | F-0016 | F-0017 | F-0018 | F-0019 | F-0020 | F-0021 | F-0022 |
|---|---|---|---|---|---|---|---|---|
| `session_id` | — | — | — | — | — | — | — | — |
| `turn_id` | — | implicit (OQ#1, OQ#3) | — | — | — | — | — | — |
| `run_id` | explicit | explicit | explicit | "run ID correlation" | implicit | `<run-id>` cmds | "by run" | "source run/artifact" |
| `branch_id` | — | — | — | — | — | — | — | — |
| `tool_call_id` | implicit | implicit | — | — | — | — | "request ID" (ambiguous) | — |
| permission request id | implicit | — | implicit (`/permissions`) | — | — | — | "request ID" (conflated) | — |
| `artifact_id` | implicit | "list run artifacts" | — | — | implicit | explicit "artifact IDs" | "execution result reference" | "source-artifact links" |

## What's clean (do not change)

- **No FEAT implies `run_id == session_id`.** The umbrella separates them by silence, and the run-control vocabulary stays out of session-control territory.
- **Run controls use `run_id`** consistently across FEAT-0015 (Terminal UI), FEAT-0016 (run-aware protocol), FEAT-0017 (attach/detach/cancel/continue/retry/fork), FEAT-0020 (`/artifacts <run-id>`, `/diff <run-id>`, `/evidence <run-id>`).
- **Session operations stay out of FEAT-0015–0022.** Resume/clear/compact/list-sessions remain owned by FEAT-0008/0009/0014. No verb conflicts.
- **Artifact IDs are first-class** in FEAT-0020 ("Final assistant summaries reference artifact IDs") and FEAT-0022 ("memory candidate schema and source-artifact links"). Memory items reference their **source run/artifact** (artifacts → runs, not the reverse).
- **Fork-as-new-run is implied** by FEAT-0015's `/fork <run-id>` description ("creates an independent continuation"). Modeltap is not using a branch-within-a-run model.

## Findings

### F1 — `session_id ↔ run_id` relationship is never stated in any FEAT — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0016 §"Run Lifecycle"; FEAT-0017 §"Attachment Semantics", §"Detached Transcripts", §"Resume After Restart"

None of FEAT-0015–0022 says whether a run belongs to a session, can span sessions, or whether a session can exist without a run. The v0.3.0 plan (WU-111) declares "runs are scoped by user/project/session," which is the right answer — but a plan note is not a feature contract. FEAT-0016 OQ#3 implies the relationship matters but does not state it.

FEAT-0017 §"Resume After Restart" says "the user can list active runs and reattach." Listed scoped to which session? FEAT-0017 §"Detached Transcripts" says "Background runs keep separate transcripts" — meaning the run owns its transcript. But the session/run transcript composition is undefined.

**Recommendation:** add to FEAT-0016 §"Run Lifecycle" or FEAT-0017 §"Attachment Semantics":

> Every run belongs to exactly one session. A session may contain zero or more runs. Run-level operations (`run_id`) and session-level operations (`session_id`) are non-overlapping verb spaces. Run transcript events do not append to the session transcript; the harness composes the foreground view from the active session transcript plus, when attached, the active run's transcript stream.

### F2 — `turn_id ↔ run_id` cardinality not stated — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0016 §"Run Lifecycle", OQ#1, OQ#3; FEAT-0019 §"Repair Attempts"

FEAT-0016 OQ#1 ("Should `turn.submit` become run-aware or should new `run.*` methods wrap turn submission?") punts to ADR. The v0.3.0 plan says "linking turn IDs to run IDs," confirming separateness but not cardinality:

- 1 turn → 1 run?
- 1 run → many turns (repair turns, validation triage turns, clarification turns)?
- Both, depending on workflow?

FEAT-0019 says "Repair turns record what was attempted and why" — are repair turns *new turns* in the same run, or new runs? The wording suggests the former. Make it explicit.

**Recommendation:** add to FEAT-0016 §"Run Lifecycle":

> A run is created by one initiating user turn. The run may consume additional turns during its lifecycle — repair turns, validation triage turns, clarification turns. Each turn within the run has its own `turn_id`; the run records the ordered list of `turn_id`s it owns. A turn cannot belong to more than one run.

### F3 — `tool_call_id` is implicit everywhere — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0015 §"Tool Runtime Integration"; FEAT-0016 §"Pipeline Events"

FEAT-0015 §"Tool Runtime Integration" lists 9 fields recorded per tool call (name, normalized input, permission decision, output envelope, …) but does not name an ID for the tool call itself. FEAT-0016 §"Pipeline Events" lists "tool call requested" and "tool result recorded" as separate events that need to correlate — without a stable `tool_call_id`, that correlation is implicit.

Request/result correlation, audit-trail joins, retry semantics, and the FEAT-0021 audit artifact all depend on a stable per-invocation handle. If two parallel tool calls of the same name fire in `tool_loop`, response routing requires a `tool_call_id`. Provider SDKs use `tool_use_id` / `tool_call_id` natively for this reason.

**Recommendation:** add an explicit field to FEAT-0015 §"Tool Runtime Integration":

> Every tool call has a stable `tool_call_id` issued at request time. Tool requests, tool results, permission decisions, and audit records all reference this `tool_call_id`. The `tool_call_id` is unique within a run.

### F4 — FEAT-0021 "request ID" conflates `tool_call_id` with permission decision — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0021 §"Audit Trail", §"Permission Outcomes"

FEAT-0021 §"Audit Trail" records tool decisions with: *"request ID, tool name and input summary, dynamic risk classification, policy source, decision, approver when applicable, timestamp, **execution result reference**"*.

This single "request ID" tries to identify both the tool invocation and the permission decision in one field. They are distinct:

- `tool_call_id` — the model's request to call a tool. Always exists.
- permission_request_id (`decision_id`) — created only when a tool call requires approval. May not exist for auto-allowed calls. May be reused if a single approval covers multiple subsequent calls (FEAT-0021 §"Permission Outcomes" lists "approved for session/run" and "approved for path/domain/tool scope" — those are scope-grants, not per-call approvals).

The "execution result reference" hint deepens the ambiguity — there are at least three distinct IDs (request, decision, result) being squashed into "request ID + result reference." `/permissions` lists pending decisions for the user to act on; the user dispositions a *decision*, not a tool call.

**Recommendation:** rename and split in FEAT-0021 §"Audit Trail":

> Every tool decision record contains: `tool_call_id` (the request being decided), `decision_id` (this approval decision; stable even if later revoked or audited), tool name and input summary, dynamic risk classification, policy source, decision outcome, approver when applicable, timestamp, and `result_id` (reference to the resulting execution record when the decision was allow/auto-allow). Scope-grants produce a `decision_id` that may be referenced by multiple subsequent `tool_call_id`s within the grant scope.

### F5 — `branch_id` absence is implicit, not chosen — significant

**Reviewer:** Architecture Conformance
**Severity:** significant
**Affected sections:** FEAT-0015 §"Foreground and Background Agents", §"Run Queue", §"Workspace Policy"; FEAT-0022 §"Quality-Driven Routing"

None of FEAT-0015–0022 mention branch identity. This is probably fine because:

- `/fork <run-id>` creates an independent run (new `run_id`), not a branch within a run.
- FEAT-0015 §"Workspace Policy" says "parallel candidate implementations should use separate isolated workspaces" — many-runs.
- FEAT-0022 §"Quality-Driven Routing" routes by stage/role within one run, not by branch.

But modeltap historically supports multi-model orchestration. If a future workflow runs 3 models in parallel against the same prompt to compare outputs, the current model is either:

- 3 separate runs with `/fork` semantics (preferred from current FEAT text), or
- 1 run with 3 model branches (would require `branch_id`).

The umbrella never explicitly chooses. Implicit choice via silence is fragile — the choice should survive a future "let's run 3 candidate implementations and pick the best" feature without retrofitting a `branch_id` concept.

**Recommendation:** add to FEAT-0015 §"Foreground and Background Agents" or §"Run Queue":

> Parallel candidate work uses many runs (forks), not branches within a run. The runtime intentionally does not introduce a `branch_id` concept; multi-model comparison and parallel implementation candidates are modeled as sibling runs that may share a parent `run_id` and a synthesis run that aggregates results. If a future workflow requires intra-run branching (for example, parallel `model_call` dispatches inside a single run's `tool_loop`), that is an explicit future ADR, not a silent extension.

### F6 — `/context drop <item>` ID space is undefined — advisory

**Reviewer:** Implementation Readiness
**Severity:** advisory
**Affected sections:** FEAT-0018 §"UI / CLI / API Integration"

`<item>` is almost certainly a context-item ID local to the active context plan, but the spec does not say so.

**Recommendation:** add a clarifying sentence: "*item* refers to the context-item ID shown by `/context`."

### F7 — `/memory <id>` ID space is undefined — advisory

**Reviewer:** Implementation Readiness
**Severity:** advisory
**Affected sections:** FEAT-0022 §"UI / CLI / API Integration"

`<id>` is the memory candidate ID. State explicitly.

**Recommendation:** add: "*id* refers to a memory candidate ID listed by `/memory`."

### F8 — Validation checks lack a per-check ID — advisory

**Reviewer:** Implementation Readiness
**Severity:** advisory
**Affected sections:** FEAT-0019 §"Check Execution", §"Repair Attempts"

FEAT-0019 §"Repair Attempts" says repair turns reference "previously attempted fixes." Without a stable check_id (or repair_attempt_id), repair-loop memory cannot reference prior attempts unambiguously.

**Recommendation:** add to FEAT-0019 §"Check Execution":

> Each check execution has a stable `check_id`. Repair attempts reference the failing `check_id` so repeated failed fixes can be detected and surfaced.

### F9 — `artifact_id` is implicit in FEAT-0020 §"Artifact Persistence" — advisory

**Reviewer:** Implementation Readiness
**Severity:** advisory
**Affected sections:** FEAT-0020 §"Artifact Persistence", §"Artifact Bundle"

FEAT-0020 §"Artifact Bundle" lists 13 artifact types but does not say each artifact has its own ID independent of the bundle. FEAT-0020 §"Artifact Persistence" says "The BFF stores artifact metadata and durable references" — implies but does not state per-artifact identity.

**Recommendation:** add to FEAT-0020 §"Artifact Persistence":

> Every artifact has a stable `artifact_id`. The artifact references its owning `run_id`, never the reverse. Artifacts within a bundle remain independently addressable by `artifact_id`.

## Recommendations (priority order)

1. **F1, F2:** add session/run/turn cardinality contract to FEAT-0016. One paragraph, settles two ambiguities.
2. **F3:** name `tool_call_id` explicitly in FEAT-0015 §"Tool Runtime Integration".
3. **F4:** split "request ID" into `tool_call_id` + `decision_id` + `result_id` in FEAT-0021 §"Audit Trail".
4. **F5:** document the no-branch-id choice in FEAT-0015.
5. **F9:** make `artifact_id` explicit in FEAT-0020 §"Artifact Persistence".
6. **F6, F7:** tighten `/context drop <item>` and `/memory <id>` ID semantics.
7. **F8:** add a per-check / repair-attempt ID in FEAT-0019.

## Companion artifacts

- Per-release plan reviews: `.sdlc/releases/v0.3.0…v0.3.4/.reviews/claude-plan-review.md`
- Prior cross-feature review: `.sdlc/features/.reviews/0015-0022-review-kimi.md`
- Per-feature findings: `.sdlc/features/.reviews/0015-0022-*-claude-findings.{md,json}`

## Disposition

Processed on 2026-04-30.

| Finding | Disposition | Notes |
|---|---|---|
| F1 | accepted | Clarified session/run cardinality in FEAT-0016 and transcript composition plus current-session run listing in FEAT-0017. |
| F2 | accepted | Clarified initiating turn, additional run-owned turns, ordered `turn_id` list, and single-run turn ownership in FEAT-0016. |
| F3 | accepted | Added stable per-run `tool_call_id` language to FEAT-0015 tool runtime integration. |
| F4 | accepted | Split FEAT-0021 audit identity into `tool_call_id`, `decision_id`, and `result_id`, including scope-grant reuse semantics. |
| F5 | accepted | Added explicit no-`branch_id` choice for professional runtime parallel candidate work in FEAT-0015. |
| F6 | accepted | Defined `/context <item>` as the context-item ID shown by `/context` in FEAT-0018. |
| F7 | accepted | Defined `/memory <id>` as a memory candidate ID listed by `/memory` in FEAT-0022. |
| F8 | accepted | Added stable `check_id` and repair-attempt references to failing checks in FEAT-0019. |
| F9 | accepted | Added stable `artifact_id` and run ownership/reference rules in FEAT-0020 artifact persistence. |
