# v0.3.2 — Plan Review (Claude Opus 4.7)

**Reviewer:** Claude Opus 4.7 (1M context), in-conversation peer review
**Date:** 2026-04-30
**Phase reviewed:** Planning draft (Phase 1 not yet opened)
**Scope:** v0.3.2 Validation, Repair, and Run Artifacts plan vs FEAT-0019, FEAT-0020
**Companion reviews:** v0.3.0, v0.3.1, v0.3.3, v0.3.4 — `claude-plan-review.md` in each

## Verdict

The plan correctly bundles FEAT-0019 and FEAT-0020 with parallel tracks and
respects their internal `depends-on` chain. **One real adherence gap and
one process gap** require resolution before Phase 1 opens.

## Success Criteria Trace

### FEAT-0019 (Validation and Repair Loop)

| SC | Trace | Status |
|---|---|---|
| #1 mutating runs produce/skip validation plan | WU-129 | covered |
| #2 targeted before broad checks | WU-129 (explicit) | covered |
| #3 validation output as structured evidence | WU-130 | covered |
| #4 failed validation summaries usable as repair-turn context | WU-131 | covered |
| #5 repair attempts recorded | WU-132 | covered |
| #6 final summaries cite evidence or explain skip | WU-135, WU-137 | covered (implicit; see Note A) |

### FEAT-0020 (Patch Evidence and Run Artifacts)

| SC | Trace | Status |
|---|---|---|
| #1 every run can expose artifact list | WU-133 | covered |
| #2 mutating runs produce patch evidence | WU-134 | covered |
| #3 validation/command logs linked to run | WU-130, WU-133 | covered |
| #4 **approval decisions inspectable by run** | none in v0.3.2; lands in v0.3.3 WU-145 | gap — see Finding 1 |
| #5 compact transcript tokens / preview | WU-135 | covered |
| #6 final summaries reference artifact IDs | WU-135, WU-137 | covered (implicit) |
| #7 suspicious patch shape surfaced | WU-134 (churn warnings) | covered |

## Findings (release-local)

### Finding 1 — FEAT-0020 SC#4 is not satisfied within v0.3.2

FEAT-0020 SC#4 requires *"approval decisions are inspectable by run."*
v0.3.2 WUs do not include approval-artifact storage; that lands in v0.3.3
WU-145 (tool audit artifacts by run). FEAT-0020 itself was promoted as a
release-shippable feature for v0.3.2.

**Recommendation:** choose one:
- (a) ship a stub approval-artifact schema in WU-133 so SC#4 holds at v0.3.2
  release time, with WU-145 enriching it later, or
- (b) amend FEAT-0020 to mark SC#4 as gated on FEAT-0021 / v0.3.3.

Option (a) is preferred because FEAT-0020 SC#4 is not phrased as
"populated"; "inspectable" only requires the artifact list endpoint to
expose approvals as a category.

### Finding 2 — WU-136 is unnumbered PATCH

WU-136 ("Codegen evaluation harness patch") is labeled `PATCH` in the
Feature column with no `PATCH-NNNN` allocation. Per CLAUDE.md commit
policy, *"Code and product-affecting changes must trace to one of: an
accepted feature with an active or planned WU-NNN, an approved patch,
[…]."* Without a numbered patch doc, WU-136 cannot produce traceable
commits.

The status Open Items correctly flags drafting the patch at Phase 1 entry,
but the plan does not state that WU-136 is gated on patch acceptance.

**Recommendation:**
- allocate `PATCH-NNNN: Codegen Evaluation Harness` and link it from WU-136
  before v0.3.2 opens Phase 1, or
- explicitly mark WU-136 as gated on patch drafting + approval before any
  WU-136 implementation commit.

### Note A — final-summary evidence citation is implicit

FEAT-0019 SC#6 ("Final run summaries cite validation evidence or explain
why validation was not run") is not the subject of a dedicated WU. WU-135
(harness artifact inspection) and WU-137 (integration tests) cover it
indirectly. The DoD #1 ("validation plans or explicit skip reasons") is
weaker than SC#6 wording.

**Recommendation:** strengthen WU-137 or DoD to require a test that asserts
final-answer messages reference at least one validation artifact ID for
mutating runs.

## Cross-cutting concerns affecting v0.3.2

### `workflow_type` referenced by WU-129

v0.3.2 WU-129 (validation plan generator) selects checks "by workflow
type" but no WU across v0.3.0–0.3.4 introduces the workflow_type field
explicitly. See v0.3.0 review, cross-cutting Finding.

**Recommendation:** none v0.3.2-local; ensure v0.3.0 introduces
workflow_type before v0.3.2 reaches Phase 3.

### Pipeline stages `validation` and `artifact_capture` activation

These stages are inactive in v0.3.0 and become real in v0.3.2. Not flagged
in any v0.3.2 WU description.

**Recommendation:** add a one-line note to WU-130 or WU-133 stating the
two stages emit real events for the first time in v0.3.2.

### Soft dependency on v0.3.1

v0.3.2 status correctly notes *"benefits from v0.3.1 context planning but
can run with minimal changed-file metadata if v0.3.1 is delayed,"* matching
FEAT-0019 §"UI/CLI/API" wording.

## Process notes

- Track structure (Decisions / Validation / Artifacts / Verification) is
  appropriate for parallel design work.
- Phase 1 design checklist correctly bundles related WUs.
- Risk register R3 ("false confidence — final summaries must distinguish
  passing/skipped/failed/inconclusive") is exactly right.

## Recommended pre-Phase-1 edits (priority order)

1. Resolve FEAT-0020 SC#4 — either WU-133 schema stub or FEAT-0020
   amendment (Finding 1).
2. Allocate `PATCH-NNNN` for WU-136 or explicit gating note (Finding 2).
3. Strengthen WU-137 or DoD to test final-answer evidence citation
   (Note A).
4. Note `validation` and `artifact_capture` stage activation in WU-130 or
   WU-133.

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Finding | Disposition |
|---|---|
| FEAT-0020 SC#4 approval decisions not satisfied | Accepted; WU-133 now includes an approval-decision artifact schema stub and DoD requires run-level metadata. |
| WU-136 lacks patch allocation gate | Accepted; WU-136 and status now require a separately drafted/accepted `PATCH-NNNN` before implementation commits. |
| Final-summary evidence citation implicit | Accepted; WU-137 and DoD now require citation tests or explicit skip/inconclusive reasons. |
| `validation` and `artifact_capture` activation implicit | Accepted; WU-130/WU-133 now identify first activation. |
| Inherited `workflow_type` dependency | Accepted; status now requires v0.3.0 `workflow_type` introduction before Phase 3. |
