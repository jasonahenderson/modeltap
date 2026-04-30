# Peer Review: v0.3.4 Release Plan

**Reviewer:** claude
**Date:** 2026-04-30
**Scope:** FEAT-0022 (Durable Memory, Quality Routing, and Workflow Extensions)
**Source plans reviewed:**
- `docs/releases/v0.3.4/plan.md`
- `docs/features/0022-memory-routing-and-workflow-extensions.md`

---

## Summary

The v0.3.4 plan is intentionally gated and cautious — the right posture for a release that sits at the top of the FEAT-0015 dependency stack. It correctly splits memory/routing (which can proceed once run artifacts exist) from workflow-extension alignment (which must coordinate with FEAT-0011/0012/0013). The plan includes a branch: if those related features are not accepted, the release must split or defer extension WUs. This is a well-managed dependency gate.

---

## Alignment Checklist

### FEAT-0022: Durable Memory, Quality Routing, and Workflow Extensions

| Requirement | v0.3.4 WU | Verdict |
|---|---|---|
| Memory/routing/extension trust ADR | WU-147 | Yes |
| Memory candidate schema + artifact links | WU-148 | Yes |
| Candidate generation + disposition UI | WU-149 | Yes |
| Active memory provenance in run details | WU-150 | Yes |
| Routing role taxonomy + policy config | WU-151 | Yes; all 7 roles from FEAT-0022 listed |
| Routing decision/outcome capture | WU-152 | Yes |
| Workflow profile + extension alignment | WU-153 | Conditional; explicitly gated on FEAT-0012/0013 acceptance |
| Tests and docs | WU-154 | Yes |
| Marketplace/plugin distribution | Not covered | Correctly deferred per plan lines 31 |
| Unbounded autonomous swarms | Not covered | Correctly excluded per plan lines 32 |
| Candidate patch comparison | Not covered | Correctly deferred per plan lines 33 |

---

## Findings

### Blocking: None

### Attention

1. **Dependency gates (R1)**  
   The plan states: *"If FEAT-0011, FEAT-0012, or FEAT-0013 are not accepted when this release is opened, Phase 1 must either split this release or mark workflow-extension WUs as deferred before design closes."* This is the single most important risk in v0.3.x. The TPM should track FEAT-0012 and FEAT-0013 status in every status update. If they are still draft when v0.3.4 Phase 1 opens, the plan should be edited immediately to defer WU-153.

2. **Bad memory (R2)**  
   The plan states: *"Memory candidates must be inspectable and reversible; do not silently promote noisy conclusions."* FEAT-0022 Non-Goal 3 confirms: *"Do not require automatic memory promotion without user or policy control."* WU-149 must define the default promotion policy (e.g., manual approval in solo mode, policy-gated in team mode) so that WU-148 schema design can include disposition fields.

3. **Routing opacity (R3)**  
   The plan states: *"Routing decisions must be explainable and recorded, or quality tuning becomes guesswork."* FEAT-0022 Success Criterion 4 requires this. WU-152 should produce a concrete routing-decision record shape before Phase 1 closes so that v0.3.4 does not design a black-box router.

4. **Extension drift (R4)**  
   The plan states: *"Skills and teams must not bypass durable runs."* FEAT-0022 explicitly says skills and agent teams should be revised to fit the run contract rather than replacing FEAT-0012/0013. WU-153 should produce either (a) a revision to FEAT-0012/0013 or (b) a constraint document that those features must accept before their own implementation proceeds.

### Nit

- FEAT-0022 Open Question 5 asks whether to split into an earlier memory/routing slice and a later workflow-extension slice. The v0.3.4 plan already contains the gate logic for this. Consider making the split explicit in the Work Units table by labeling WU-147–152 as Track B/C and WU-153 as Track D, with a dependency comment: "Track D may be deferred."

---

## Verdict

**Proceed to Phase 1.** The plan is appropriately cautious and gated. Condition the opening of Phase 1 on a TPM go/no-go decision for FEAT-0012/0013 acceptance; if they are not accepted, split or defer WU-153 before any design work begins.

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Attention item | Disposition |
|---|---|
| Dependency gates | Accepted; status now tracks FEAT-0011/0012/0013 acceptance or revision before WU-153 design closes. |
| Bad memory | Accepted; WU-149 now states the default no-silent-promotion policy unless WU-147 accepts a narrower automatic mode. |
| Routing opacity | Accepted; WU-152 now requires a concrete routing-decision record. |
| Extension drift | Accepted; WU-153 now produces revisions or constraint docs before implementation. |
| Track D deferrable nit | Accepted in substance; status keeps the split/defer decision as an open Phase 1 item. |
