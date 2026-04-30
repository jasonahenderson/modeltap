# Peer Review: v0.3.0 Release Plan

**Reviewer:** claude
**Date:** 2026-04-30
**Scope:** FEAT-0015 (umbrella), FEAT-0016 (Managed Codegen Run Pipeline), FEAT-0017 (Durable Runs and Background Agents) — foundation slice
**Source plans reviewed:**
- `docs/releases/v0.3.0/plan.md`
- `docs/features/0015-professional-harness-runtime.md`
- `docs/features/0016-managed-codegen-run-pipeline.md`
- `docs/features/0017-durable-runs-and-background-agents.md`

---

## Summary

The v0.3.0 release plan is a faithful, stack-ranked translation of the FEAT-0015 → FEAT-0017 feature contracts into ordered work units with clear explicit exclusions. It correctly declares the phased-release rules (Phase 1 designs all WUs; Phase 2 review; Phase 3 implement in dependency-legal order) and surfaces the key cross-feature constraints from the feature specs. The plan can proceed to Phase 1 once opened by an ADMIN commit.

---

## Alignment Checklist

### FEAT-0015: Professional Harness Runtime (umbrella constraints)

| Capability | v0.3.0 Coverage | Verdict |
|---|---|---|
| Run ID + lifecycle status | WU-111, 112, 113 | Yes |
| Pipeline stage vocabulary (preflight → completion) | WU-113, 114 | Yes; all 9 stages scoped in v0.3.0 |
| Attachment state (attached/detached) | WU-115 | Yes; foreground-run slice only |
| Checkpoint metadata | WU-113 | Yes; metadata only, not full continuation logic |
| Foreground/background shared runtime | WU-112, 115, 116 | Yes; background queue deferred to FEAT-0017 full slice |
| Tool calls as first-class run events | WU-112, 113 | Partial; policy-grade audit trail deferred to v0.3.3 |
| Workspace policy modes | WU-110 ADR decision needed; modes not enforced until v0.3.3 | Correctly deferred |
| Context planning | Not covered | Correctly deferred to v0.3.1 |
| Validation / repair | Not covered | Correctly deferred to v0.3.2 |
| Patch evidence | Not covered | Correctly deferred to v0.3.2 |
| Policy-grade tool runtime | Not covered | Correctly deferred to v0.3.3 |
| Memory / routing | Not covered | Correctly deferred to v0.3.4 |

### FEAT-0016: Managed Codegen Run Pipeline

| Requirement | v0.3.0 WU | Verdict |
|---|---|---|
| Run ID before model dispatch | WU-112 | Yes |
| Stage/status emission | WU-113 | Yes |
| Tool/cost/outcome correlated with run ID | WU-112, 113 | Yes |
| Harness `/run` inspection | WU-114 | Yes |
| Interrupt/retry/continue/fork on run ID | WU-115 | Yes; early retry may be shallow per plan |
| Simple chat compatible as foreground run | WU-112 | Yes |
| Background queue UX | Not covered | Correctly deferred to FEAT-0017 |
| Repo-aware context selection | Not covered | Correctly deferred to v0.3.1 |
| Validation planning | Not covered | Correctly deferred to v0.3.2 |

### FEAT-0017: Durable Runs and Background Agents (foundation slice only)

| Requirement | v0.3.0 WU | Verdict |
|---|---|---|
| Attach/detach semantics | WU-115 | Yes |
| Run list (`/runs` / `/jobs`) | WU-115 | Yes |
| Cancel/retry/continue/fork | WU-115 | Yes |
| Reconnect/resume behavior | WU-116 | Yes; replay/summarize based on checkpoint availability |
| Separate background transcript | WU-115 | Partial; plan says "separate" but does not detail merging/summarizing |
| Background permission inbox | Not covered | Correctly deferred to v0.3.3 policy work |
| Full background local-tool execution while disconnected | Explicitly excluded per plan line 35 | Correct |

---

## Findings

### Blocking: None

### Attention

1. **FEAT-0015 Open Question 7 (local executor availability)**  
   The plan states: *"The ADR must settle the open executor-availability question before implementation."* This maps to FEAT-0017 Open Question 1 and FEAT-0015 Open Question 7. WU-108 must resolve it explicitly. If the ADR punts, WU-116 (reconnect/resume) and v0.3.3 background policy will inherit an ambiguity.

2. **Checkpoint data contract drift risk**  
   FEAT-0016 Open Question 2 asks: *"What minimum checkpoint data is required to safely continue a failed run?"* v0.3.0 scope says checkpoint metadata must be *"sufficient for inspect/retry/continue/fork designs"* but the actual retry/continue/fork logic may be shallow. The design docs (WU-108, WU-109, WU-113) should define the exact checkpoint payload so v0.3.2 does not have to revise the storage contract.

3. **Run protocol surface naming**  
   FEAT-0016 Open Question 1 asks whether to extend `turn.submit` or add `run.*` methods. WU-110 must settle this and document it in the protocol design. If naming is deferred, v0.3.1 context planner protocol will have an unstable surface to build on.

### Nit

- Phase 1 checklist bundles WU-111–113 and WU-114–116. Given FEAT-0015’s emphasis on a shared run/artifact model, consider whether WU-114–116 should be bundled with WU-111–113 into a single cross-track design doc if they share the same protocol/event surface.

---

## Cross-Release Traceability

| Downstream dependency | v0.3.0 prerequisite | Risk if v0.3.0 is incomplete |
|---|---|---|
| v0.3.1 FEAT-0018 context plan | Run ID + checkpoint metadata + context plan schema | Delay or redesign |
| v0.3.2 FEAT-0019 validation + FEAT-0020 artifacts | Run artifact bundle schema + validation evidence envelope | Delay or redesign |
| v0.3.3 FEAT-0021 policy | Tool event schema + workspace mode metadata | Delay or redesign |
| v0.3.4 FEAT-0022 memory/routing | Artifact store + run outcome field | Delay or redesign |

All four downstream releases are correctly gated. v0.3.0 is the critical-path foundation.

---

## Verdict

**Proceed to Phase 1.** The plan is internally consistent and correctly defers out-of-scope work. Three attention items (executor availability, checkpoint contract, protocol naming) should be resolved in WU-108/WU-110 before Phase 1 closes.

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Attention item | Disposition |
|---|---|
| Executor availability | Accepted; WU-108 now explicitly resolves connected-harness requirements for foreground and detached runs. |
| Checkpoint data contract drift | Accepted; WU-109/WU-113 retain checkpoint contract ownership for future retry/continue/fork. |
| Run protocol naming | Accepted; WU-110 now explicitly owns `run.*` versus `turn.*` compatibility. |
| Cross-track design bundle nit | Deferred to Phase 1 design packaging; no release-plan change needed. |
