# Peer Review: v0.3.1 Release Plan

**Reviewer:** claude
**Date:** 2026-04-30
**Scope:** FEAT-0018 (Context Planner and Project Rules)
**Source plans reviewed:**
- `docs/releases/v0.3.1/plan.md`
- `docs/features/0018-context-planner-and-project-rules.md`

---

## Summary

The v0.3.1 plan correctly scopes FEAT-0018 as a v0.3.0-follow-up that adds repo-aware context planning without requiring AST/symbol indexing in the first slice. The WU dependency chain respects the v0.3.0 foundation (ADR first, then discovery, then planner, then UI). Exclusions are explicit. The plan can proceed to Phase 1 once the parent release (v0.3.0) has accepted the run-runtime ADR and storage schemas.

---

## Alignment Checklist

### FEAT-0018: Context Planner and Project Rules

| Requirement | v0.3.1 WU | Verdict |
|---|---|---|
| Project rule discovery (MODELTAP.md, AGENTS.md, CLAUDE.md, etc.) | WU-118 (ADR), WU-120 | Yes |
| Deterministic precedence / conflict behavior | WU-118 (ADR) | Yes; ADR gates all downstream design |
| Context plan schema + run correlation | WU-119 | Yes |
| Lightweight repo map | WU-121 | Yes; explicitly excludes full AST indexing |
| Style/test context discovery | WU-122 | Yes; no AST required |
| Context provenance + budget accounting | WU-123, WU-124 | Yes |
| `/context`, `/context rules`, `/context why` | WU-125 | Yes |
| Validation execution | Not covered | Correctly deferred to v0.3.2 |
| Patch evidence | Not covered | Correctly deferred to v0.3.2 |
| Memory promotion | Not covered | Correctly deferred to v0.3.4 |
| Full AST/symbol indexing | Not covered | Correctly deferred per plan lines 29 |

---

## Findings

### Blocking: None

### Attention

1. **Rule precedence conflict (R1)**  
   The plan acknowledges: *"The ADR must define deterministic ordering and visibility."* FEAT-0015 Open Question 6 (workflow commands as skills vs run profiles) and FEAT-0018 Open Question 1 (`MODELTAP.md` vs `AGENTS.md` precedence) are tightly coupled. WU-118 should resolve both, or else v0.3.4 workflow-extension work will revisit the same territory.

2. **Prompt leakage (R3)**  
   Plan line 130: *"Metadata inspection must not expose protected or secret-bearing prompt content by default."* FEAT-0016 Open Question 4 asks: *"How much prompt metadata can be exposed without leaking protected prompt content?"* The v0.3.0 and v0.3.1 ADRs should agree on a single metadata taxonomy so that prompt-layer inspection does not accidentally expose secrets in `/run prompt` or `/context`.

3. **Repo-map cost (R2)**  
   WU-121 is marked Large. The design doc should set a ceiling on repo-map generation cost (e.g., max files scanned, max depth) so that large monorepos do not block the planner.

### Nit

- The plan cites EXP-0012 as advisory input. If the design doc bundles WU-121 and WU-122, it should explicitly mark which fields are placeholders for future AST enrichment so the v0.3.4 memory/routing work does not overfit on v1 repo-map shapes.

---

## Verdict

**Proceed to Phase 1.** Scope is tight and exclusions are correct. WU-118 must resolve rule precedence and prompt-metadata boundaries before Phase 1 closes.

## Disposition

Processed in `ADMIN: process v0.3.x release plan reviews`.

| Attention item | Disposition |
|---|---|
| Rule precedence conflict | Accepted; WU-118 remains the ADR gate for rule precedence and workflow-related prompt layers. |
| Prompt leakage | Accepted; WU-118 now aligns prompt metadata with v0.3.0 run prompt decisions. |
| Repo-map cost | Accepted; WU-121 now requires explicit generation cost ceilings. |
| Future AST placeholders nit | Deferred to WU-121/WU-122 Phase 1 design artifacts. |
