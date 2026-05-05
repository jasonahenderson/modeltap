# Peer Review: v0.3.2 Release Plan

**Reviewer:** claude
**Date:** 2026-04-30
**Scope:** FEAT-0019 (Validation and Repair Loop), FEAT-0020 (Patch Evidence and Run Artifacts)
**Source plans reviewed:**
- `docs/releases/v0.3.2/plan.md`
- `docs/features/0019-validation-and-repair-loop.md`
- `docs/features/0020-patch-evidence-and-run-artifacts.md`

---

## Summary

The v0.3.2 plan is the first release to close the implementation loop: it turns generated changes into validated, reviewable work. FEAT-0019 and FEAT-0020 are correctly treated as parallel tracks that share the run artifact schema from v0.3.0. The plan explicitly limits artifact storage scope (no deep redaction engine yet) and acknowledges command-safety risk until v0.3.3 policy work lands. The plan can proceed to Phase 1 once v0.3.0 artifact schema and v0.3.1 context planning surfaces are stable.

---

## Alignment Checklist

### FEAT-0019: Validation and Repair Loop

| Requirement | v0.3.2 WU | Verdict |
|---|---|---|
| Validation plan from changed files + structure | WU-129 | Yes |
| Structured check evidence | WU-130 | Yes |
| Failure summarization + repair context | WU-131 | Yes |
| Repair-attempt memory + stop/ask limits | WU-132 | Yes |
| Command safety under existing permissions | WU-129–132 | Partial; plan acknowledges this is R1 and defers policy-grade enforcement to v0.3.3 — correct |
| Full context planner integration | Not covered | Correctly deferred; v0.3.1 context enriches repair but is not a hard prerequisite per FEAT-0019 |

### FEAT-0020: Patch Evidence and Run Artifacts

| Requirement | v0.3.2 WU | Verdict |
|---|---|---|
| Artifact bundle store + API | WU-133 | Yes |
| Patch/diff evidence collector | WU-134 | Yes |
| `/artifacts`, `/diff`, `/evidence` inspection | WU-135 | Yes |
| Compact transcript tokens + preview | WU-135 | Yes |
| Artifact storage boundaries + retention | WU-128 (ADR) | Yes; redaction/encryption deferred per plan lines 27–28 |
| Validation artifact linkage to run | WU-133 | Yes; depends on FEAT-0019 evidence envelopes |

---

## Findings

### Blocking: None

### Attention

1. **Repair-loop false confidence (R3)**  
   The plan states: *"Final summaries must distinguish passing, skipped, failed, and inconclusive validation."* FEAT-0019 Success Criterion 6 requires this too. WU-131 (failure summarization) and WU-137 (integration tests) should include a matrix of possible validation outcomes and ensure the repair-turn context injection does not conflate inconclusive with passed.

2. **Artifact bloat (R2)**  
   The plan acknowledges: *"Large logs and diffs need retention/redaction boundaries before implementation."* WU-128 must design these boundaries because WU-133 (artifact store) cannot choose inline vs reference storage without knowing the maximum captured log size and redaction policy. If WU-128 is thin, WU-133 risks building a schema that cannot accommodate later team/enterprise profiles.

3. **Codegen evaluation harness patch (WU-136)**  
   This is the first PATCH-authorized work unit in the 0.3.x series. The plan says it depends on WU-129–135. The design doc should explicitly state whether the evaluation harness patch is a standalone binary, a Go test package, or a CI script — this affects the Infrastructure Engineer’s CI/build commitments in later releases.

### Nit

- FEAT-0020 Open Question 3 asks: *"Should suspicious patch warnings block autonomous mode?"* The plan does not mention autonomous mode in v0.3.2 scope. Either confirm that autonomous mode is out of scope for this release, or add a note to WU-135 that warnings are advisory only.

---

## Verdict

**Proceed to Phase 1.** Scope is correctly bounded. WU-128 (artifact storage ADR) and WU-131 (failure summarization) need the most design depth to avoid downstream schema churn.

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Attention item | Disposition |
|---|---|
| Repair-loop false confidence | Accepted; WU-131 and WU-137 now require a passed/failed/skipped/inconclusive outcome matrix. |
| Artifact bloat | Accepted; WU-128 now explicitly owns artifact size limits along with retention/redaction. |
| Codegen evaluation harness patch form | Accepted; WU-136 must decide package/binary/CI shape in the patch. |
| Autonomous warning scope nit | Accepted; autonomous-mode blocking policy remains out of scope, with advisory artifact notes only. |
