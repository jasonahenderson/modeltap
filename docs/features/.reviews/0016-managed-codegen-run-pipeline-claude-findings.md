# FEAT-0016 Findings (Claude)

- Feature: `docs/features/0016-managed-codegen-run-pipeline.md`
- Review date: 2026-04-29
- Reviewer: Claude Opus 4.7 (1M context)
- total_findings: 2
- blocking: 0
- significant: 1
- advisory: 1
- top_line: Spec is well structured and the BFF/harness responsibility split is clean. The main concern is that FEAT-0016's stage vocabulary (`context_plan`, `prompt_plan`, `model_call`, etc.) does not match FEAT-0015's lifecycle state vocabulary (`context_planning`, `prompt_planning`, `running`, `validating`, etc.). Both specs defer the harmonization to a future "Run Runtime" ADR, but the divergence creates near-term ambiguity for downstream design.

## Findings

### F1 — Significant

**Reviewer:** Vocabulary Consistency

**Affected sections:** Run Lifecycle, Pipeline Events

**Summary:** Pipeline stage names diverge from FEAT-0015's run lifecycle states.

**Detail:** FEAT-0016:0040 defines stages `preflight → context_plan → prompt_plan → model_call → tool_loop → artifact_capture → checkpoint → completion`. FEAT-0015:0077-0090 lists lifecycle states `queued, preflight, context_planning, prompt_planning, running, waiting_permission, waiting_user, validating, checkpointed, completed, failed, cancelled`. There is no explicit mapping between the two — `model_call` plus `tool_loop` here roughly corresponds to `running` plus `validating` in FEAT-0015, and the noun/verb forms differ (`context_plan` vs `context_planning`). Both specs flag the future Run Runtime ADR as the harmonization point, but until that ADR lands, the vocabularies are incompatible.

**Recommendation:** Either (a) note explicitly in §Run Lifecycle that vocabulary is provisional pending the lifecycle ADR and that FEAT-0015 uses a parallel state set, or (b) collapse to a single canonical vocabulary in FEAT-0015 and have this spec reference it.

### F2 — Advisory

**Reviewer:** Frontmatter Convention

**Affected sections:** Frontmatter

**Summary:** `promoted-from: FEAT-0015` is redundant with `parent: FEAT-0015`.

**Detail:** The README's `promoted-from` semantic is reserved for an exploration or patch that firms up into a feature spec. FEAT-0016 is a member of FEAT-0015's umbrella, already encoded by `parent` and `series`. See FEAT-0015 review F4 for the full cross-cutting note.

**Recommendation:** Drop `promoted-from: FEAT-0015` from frontmatter.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| F1 | accepted | Updated FEAT-0016 to use FEAT-0015 stage vocabulary and include the validation stage. |
| F2 | accepted | Removed promoted-from: FEAT-0015 from frontmatter. |
