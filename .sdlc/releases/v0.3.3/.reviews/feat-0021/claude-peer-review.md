# Peer Review: v0.3.3 Release Plan

**Reviewer:** claude
**Date:** 2026-04-30
**Scope:** FEAT-0021 (Policy-Grade Tool Runtime)
**Source plans reviewed:**
- `.sdlc/releases/v0.3.3/plan.md`
- `.sdlc/features/0021-policy-grade-tool-runtime.md`

---

## Summary

The v0.3.3 plan correctly treats FEAT-0021 as a policy-elevation release: it replaces first-use permission prompts with structured policy dimensions, workspace modes, and audit trails. The plan scopes foreground policy first, then background-aware policy integration using the durable run queue from v0.3.0. It explicitly excludes full enterprise RBAC and hook/plugin packaging, both of which belong later. The plan can proceed to Phase 1 once v0.3.0 run IDs, attachment state, and event schemas are stable.

---

## Alignment Checklist

### FEAT-0021: Policy-Grade Tool Runtime

| Requirement | v0.3.3 WU | Verdict |
|---|---|---|
| Policy/workspace ADR (inheritance, non-overridable sources) | WU-138 | Yes |
| Policy schema + inheritance model | WU-139 | Yes |
| Workspace mode resolver (`current`, `current_readonly`, `worktree`, `temp_copy`, `remote`) | WU-140 | Yes; all 5 modes from FEAT-0015 §Workspace Policy included |
| Foreground tool policy enforcement | WU-141 | Yes |
| Command / path / Git / domain classifiers | WU-142 | Yes |
| Permission explanation + `/policy` harness surface | WU-143 | Yes |
| Background blocked-operation behavior | WU-144 | Yes; pauses auto-denies per policy; uses FEAT-0017 attach/detach semantics |
| Tool audit artifacts by run | WU-145 | Yes |
| Full enterprise RBAC | Not covered | Correctly deferred per plan lines 25 |
| Hook/plugin packaging | Not covered | Correctly deferred per plan lines 26 |
| Durable memory promotion | Not covered | Correctly deferred to v0.3.4 |

---

## Findings

### Blocking: None

### Attention

1. **Rules-engine creep (R1)**  
   The plan warns: *"Keep policy expressive enough for v1 without becoming a general-purpose policy language."* FEAT-0021 Open Question 1 asks: *"What policy language is sufficient for v1 without becoming a general-purpose rules engine?"* WU-138 and WU-139 must choose a concrete schema (e.g., layered YAML with deny/allow lists and risk classifiers) and reject DSL ambitions in this release. If the ADR leaves the door open to a DSL, v0.3.4 workflow extension alignment will inherit an unstable policy surface.

2. **Unsafe defaults (R2)**  
   The plan states: *"Background writes must not silently proceed by default."* This aligns with FEAT-0015 §Background Permission Behavior and FEAT-0017 §Background Permission Behavior. WU-144 must define the exact default (pause vs auto-deny) per operation class so that FEAT-0017 Open Question 3 (default behavior for background write requests in solo mode) is resolved. If WU-144 leaves this configurable without a safe default, solo users may accidentally enable silent background writes.

3. **False blocks (R3)**  
   The plan notes: *"Classifiers need clear override/approval behavior for solo workflows."* WU-142 (classifiers) and WU-143 (`/policy`) should include a solo-mode fast-path design so that single-user workflows are not crippled by multi-user guardrails.

4. **Background policy integration with FEAT-0017**  
   WU-144 depends on FEAT-0017 background run semantics. The v0.3.0 plan says v0.3.0 implements only the FEAT-0017 foundation slice. The v0.3.3 plan must confirm whether the required FEAT-0017 surface (blocked-run inbox, detach state machine) is complete enough to support WU-144. If not, the dependency graph between v0.3.0 and v0.3.3 needs an explicit interface contract.

### Nit

- FEAT-0021 Success Criterion 7 says: *"A foreground-policy slice can ship before FEAT-0017; background-specific behavior lands after durable background runs exist."* The v0.3.3 plan covers both foreground and background policy. This is fine, but the design doc should mark which WUs are foreground-only-safe if v0.3.3 ever needs to ship before FEAT-0017 background semantics are fully available.

---

## Verdict

**Proceed to Phase 1.** Scope is disciplined. WU-138 and WU-139 must pin the policy schema language to avoid DSL creep. WU-144 must resolve the default background write behavior explicitly.

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Attention item | Disposition |
|---|---|
| Rules-engine creep | Accepted; WU-138 now rejects a general-purpose policy DSL for this release. |
| Unsafe defaults | Accepted; WU-144 now states the safe default that background writes do not proceed silently. |
| False blocks | Accepted; WU-142/WU-143 now include solo-mode fast-path design. |
| FEAT-0017 background integration surface | Accepted; WU-144 now identifies itself as the implementation home for deferred FEAT-0017 background criteria. |
| Foreground-only-safe nit | Deferred to WU-146/Phase 1 design if release slicing changes. |
