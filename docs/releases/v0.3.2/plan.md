# Implementation Plan: v0.3.2 — Validation, Repair, and Run Artifacts

## Context

`v0.3.0` gives work a durable run identity. `v0.3.1` improves model inputs with
context planning. `v0.3.2` closes the loop by turning generated changes into
validated, reviewable work with structured evidence.

This release implements FEAT-0019 and FEAT-0020, plus an implementation-scoped
codegen evaluation harness patch.

## Scope

This release covers:

- validation and repair ADR/design decisions
- artifact storage/redaction/retention ADR/design decisions
- validation plan generation from changed files and project structure
- structured command/check evidence
- failure summarization and repair-attempt memory within a run
- run artifact bundles
- patch evidence: diff summary, changed files, churn/unrelated-change warnings
- artifact inspection through `/artifacts`, `/diff`, and `/evidence`
- codegen evaluation harness patch

This release does not cover:

- policy-grade tool rules beyond existing permission behavior
- durable memory promotion
- quality-driven routing beyond recording evidence for future routing
- background writers beyond v0.3.0 attach/detach behavior
- autonomous-mode policy warnings beyond advisory artifact notes; policy-grade
  enforcement belongs to v0.3.3

## Feature Scope

- FEAT-0019: Validation and Repair Loop
- FEAT-0020: Patch Evidence and Run Artifacts
- Future PATCH: Codegen Evaluation Harness

## Approach

Current phase: **Planning draft — Phase 1 not opened.**

## Work Units

| WU | Title | Dependencies | Size | Feature |
|---|---|---|---|---|
| 127 | Validation artifact and repair-loop ADR/design | v0.3.0 | M | FEAT-0019 |
| 128 | Artifact storage, retention, and redaction ADR/design | v0.3.0 | M | FEAT-0020 |
| 129 | Validation plan generator | 127 | L | FEAT-0019 |
| 130 | Structured command/check evidence envelopes | 127 | M | FEAT-0019 |
| 131 | Failure summarization and repair context injection | 129, 130 | L | FEAT-0019 |
| 132 | Repair-attempt memory and stop/ask behavior | 131 | M | FEAT-0019 |
| 133 | Run artifact bundle store and API | 128 | L | FEAT-0020 |
| 134 | Patch/diff evidence collector | 133 | M | FEAT-0020 |
| 135 | Harness artifact inspection surfaces | 133, 134 | M | FEAT-0020 |
| 136 | Codegen evaluation harness patch | 129-135 | M | PATCH |
| 137 | Validation/artifact integration tests and docs | 129-136 | M | FEAT-0019/0020 |

## Detailed WU Plan

### Track A — Decisions

**WU-127: Validation artifact and repair-loop ADR/design**

Define validation evidence schema, repair-loop limits, failure classification,
and when the system stops versus asks the user.

**WU-128: Artifact storage, retention, and redaction ADR/design**

Define artifact storage boundaries, large-log handling, redaction/encryption by
deployment profile, artifact size limits, and artifact references for isolated
workspaces.

### Track B — Validation and Repair

**WU-129: Validation plan generator**

Infer targeted checks from changed files, project structure, known commands, and
workflow type. Prefer targeted checks before broad checks.

**WU-130: Structured command/check evidence envelopes**

Capture command, workspace, timing, exit status, stdout/stderr references, and
summary fields as run artifacts. This WU activates the `validation` pipeline
stage first defined by v0.3.0.

**WU-131: Failure summarization and repair context injection**

Summarize compiler/test/lint/runtime failures with file/line references and feed
the summary into repair turns. The design must use a validation outcome matrix
that distinguishes passed, failed, skipped, and inconclusive checks.

**WU-132: Repair-attempt memory and stop/ask behavior**

Record attempted repairs and prevent repeated loops. Enforce configured repair
attempt limits.

### Track C — Artifact Evidence

**WU-133: Run artifact bundle store and API**

Persist artifact metadata and content/reference handles. Expose artifact list
and detail APIs. Include an approval-decision artifact schema stub so FEAT-0020
SC#4 has a stable run-level home until v0.3.3 fills policy audit details. This
WU activates the `artifact_capture` pipeline stage first defined by v0.3.0.

**WU-134: Patch/diff evidence collector**

Collect changed-file lists, diff summaries, suspicious churn warnings,
unrelated-change warnings, and read-before-write warnings where available.

**WU-135: Harness artifact inspection surfaces**

Add `/artifacts`, `/diff`, and `/evidence` surfaces with compact transcript
tokens and preview behavior.

### Track D — Evaluation and Verification

**WU-136: Codegen evaluation harness patch**

Draft and implement the patch for benchmark scenarios, diff-quality scoring,
validation success metrics, and regression fixtures. WU-136 is gated on a
separately drafted and accepted `PATCH-NNNN` before any implementation commit;
the patch must decide whether the harness is a Go test package, standalone
binary, CI script, or combination.

**WU-137: Validation/artifact integration tests and docs**

Add E2E tests for validation planning, repair summaries, artifact inspection,
diff evidence, and final-answer evidence citation. Tests must assert mutating
run summaries reference validation artifact IDs or explain why validation was
skipped, failed to run, or remained inconclusive.

## Phase 1 Design Checklist

- [ ] WU-127 validation/repair ADR design
- [ ] WU-128 artifact storage/redaction ADR design
- [ ] WU-129 to WU-132 validation/repair design bundle
- [ ] WU-133 to WU-135 artifact evidence design bundle
- [ ] WU-136 patch doc for codegen evaluation harness
- [ ] WU-137 verification/docs design

## Risk Register

- **R1 — command safety.** Validation commands still run through existing tool
  permissions until v0.3.3 policy work lands.
- **R2 — artifact bloat.** Large logs and diffs need retention/redaction
  boundaries before implementation.
- **R3 — false confidence.** Final summaries must distinguish passing,
  skipped, failed, and inconclusive validation.
- **R4 — repair loops.** Repair attempt memory and hard limits are required.

## Definition of Done

1. Mutating runs produce validation plans or explicit skip reasons.
2. Validation output is stored as structured evidence.
3. Failed validation can feed repair turns without raw-log dumping.
4. Mutating runs produce patch evidence artifacts.
5. The harness can inspect artifacts, diffs, and evidence.
6. Approval decisions are represented by run artifact metadata, even before
   v0.3.3 policy audit enrichment.
7. Final run summaries cite validation evidence or explicit skip/inconclusive
   reasons.
8. Codegen evaluation harness patch exists and runs against fixture scenarios.
