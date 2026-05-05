# v0.3.4 — Plan Review (Claude Opus 4.7)

**Reviewer:** Claude Opus 4.7 (1M context), in-conversation peer review
**Date:** 2026-04-30
**Phase reviewed:** Planning draft (Phase 1 not yet opened)
**Scope:** v0.3.4 Memory, Routing, and Workflow Extensions plan vs FEAT-0022
**Companion reviews:** v0.3.0, v0.3.1, v0.3.2, v0.3.3 — `claude-plan-review.md` in each

## Verdict

All seven FEAT-0022 success criteria trace, and the gating language
correctly mirrors FEAT-0022 §"Cross-Feature Impact." The Open Items already
flag the split decision (memory/routing vs workflow-extension alignment)
that FEAT-0022 OQ#5 raises. **No release-local adherence gaps.**

The most important pre-Phase-1 work is the FEAT-0011/0012/0013
coordination called out in the plan §"Approach."

## Success Criteria Trace (FEAT-0022)

| SC | Trace | Status |
|---|---|---|
| #1 successful runs produce memory candidates linked to source artifacts | WU-148, WU-149 | covered |
| #2 users can inspect/disposition candidates | WU-149 | covered |
| #3 active memory injected into a run is visible with provenance | WU-150 | covered |
| #4 routing decisions recorded with stage/reason/model/outcome | WU-152 | covered |
| #5 workflow extensions inside durable run contract | WU-153 (gated) | covered with explicit gate |
| #6 skills/teams reference workflow profiles or stage behavior | WU-153 (gated) | covered with explicit gate |
| #7 routing improvements evaluatable against stored outcomes | WU-152 | covered |

The plan §"Definition of Done" #1 ("Memory/routing/extension trust ADR is
accepted **or the release is split** to isolate memory/routing") matches
FEAT-0022 OQ#5.

## Findings (release-local)

None. The plan is internally consistent and traces 1:1 to FEAT-0022.

## Cross-cutting concerns affecting v0.3.4

### Workflow slash commands `/explore`, `/feature`, `/adr`, `/release`, `/implement`, `/debug`, `/docs`, `/devops`

FEAT-0015 §"Terminal UI" lists these as part of the umbrella behavior
contract. They are not assigned to any v0.3.x release. WU-153 ("Workflow
profile and extension alignment design") is the most natural home, since
it aligns skills, hooks, slash commands, and agent teams with workflow
contracts.

**Recommendation:** explicitly include workflow slash commands in WU-153's
scope, or add a separate WU under v0.3.4 Track D that delivers them once
WU-153 produces the workflow profile registry.

### `workflow_type` is foundational to this release

WU-153 ("workflow profile and extension alignment") and WU-151 ("routing
role taxonomy") both depend on the workflow_type concept established
upstream. v0.3.0 plan does not introduce workflow_type as a runtime field
(see v0.3.0 review, cross-cutting Finding).

**Recommendation:** Phase 1 of v0.3.4 should not begin until v0.3.0 has
introduced workflow_type, or v0.3.4 should include a workflow_type
introduction WU as a prerequisite for WU-151 and WU-153.

### FEAT-0011/0012/0013 acceptance gate

Plan §"Approach" correctly states: *"If FEAT-0011, FEAT-0012, or FEAT-0013
are not accepted when this release is opened, Phase 1 must either split
this release or mark workflow-extension WUs as deferred before design
closes."* This is exactly right.

The status open item ("Decide during Phase 1 whether to split memory/routing
from workflow-extension alignment") complements this and provides a
deterministic decision point.

### Memory/routing slice is independently shippable

Per FEAT-0022 OQ#5, the memory/routing slice (WU-147–152) does not require
FEAT-0011/0012/0013. WU-153 is the only gated WU. If the split is taken,
v0.3.4 ships WU-147–152 + WU-154 cleanly and a hypothetical v0.3.5 picks
up WU-153.

**Recommendation:** none. The plan already permits this. Worth confirming
the split path during Phase 1 if the FEAT-0011/0012/0013 reviews lag.

## Process notes

- Phase 1 design checklist groups WUs sensibly across four tracks.
- Risk register R1 ("dependency gates") and R4 ("extension drift") cover
  the unique risks of this release.
- DoD #1 explicitly names the split option, which is rare and correct.

## Recommended pre-Phase-1 edits

None required. Optional clarifications:

1. Decide where workflow slash commands land (WU-153 scope expansion or
   separate WU).
2. Confirm workflow_type prerequisite is satisfied by v0.3.0 before v0.3.4
   Phase 1 opens.
3. If FEAT-0011/0012/0013 reviews are still pending, prepare the split
   plan in advance so Phase 1 can act on it without blocking.

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Finding | Disposition |
|---|---|
| Workflow slash commands unassigned | Accepted; WU-153 now includes slash command alignment and may defer implementation. |
| `workflow_type` prerequisite | Accepted; status now requires v0.3.0 `workflow_type` introduction before Phase 1 opens. |
| FEAT-0011/0012/0013 gate | Accepted; status now tracks acceptance/revision before WU-153 design closes. |
