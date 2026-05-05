# FEAT-0015 Findings (Claude)

- Feature: `docs/features/0015-professional-harness-runtime.md`
- Review date: 2026-04-29
- Reviewer: Claude Opus 4.7 (1M context)
- total_findings: 6
- blocking: 0
- significant: 3
- advisory: 3
- top_line: The umbrella spec is structurally sound and the dependency DAG across the 7 members is acyclic. Three issues need resolution before acceptance: lifecycle vocabulary diverges from FEAT-0016, the `release` workflow type collides with the existing release-plan structure in `docs/releases/`, and FEAT-0017's open question about local-executor availability for background runs is foundational and should be raised here. A `promoted-from: FEAT-0015` field used by all 7 members is semantically redundant with `parent: FEAT-0015` and should be dropped.

## Findings

### F1 — Significant

**Reviewer:** Vocabulary Consistency

**Affected sections:** Key Capabilities → Durable Runs

**Summary:** Run lifecycle states in FEAT-0015 do not match the stage vocabulary used by FEAT-0016.

**Detail:** FEAT-0015:0077-0090 lists 12 lifecycle states (`queued`, `preflight`, `context_planning`, `prompt_planning`, `running`, `waiting_permission`, `waiting_user`, `validating`, `checkpointed`, `completed`, `failed`, `cancelled`). FEAT-0016:0040 defines the pipeline as 8 stages (`preflight`, `context_plan`, `prompt_plan`, `model_call`, `tool_loop`, `artifact_capture`, `checkpoint`, `completion`). Same concepts, different identifiers (e.g. `context_plan` vs `context_planning`, `checkpoint` vs `checkpointed`). Both specs flag the future "Run Runtime" ADR as the place to harmonize this, but readers comparing the two specs today have no stable contract.

**Recommendation:** Add a one-line note in both FEAT-0015 §Durable Runs and FEAT-0016 §Run Lifecycle stating the vocabulary is provisional pending the lifecycle ADR. Better: pre-resolve the vocabulary in this spec and have FEAT-0016 reference it.

### F2 — Significant

**Reviewer:** Process Coherence

**Affected sections:** Workflow Contracts

**Summary:** The `release` workflow type overlaps the existing release-plan structure without explaining the relationship.

**Detail:** FEAT-0015:0136-0146 introduces a `release` workflow type. The repo already has a structured release process (`docs/releases/<version>/plan.md`, `status.md`, `track-*.md`, `changelog.md`) plus the three-phase Phase 1/2/3 model in CLAUDE.md. There are now potentially two "release workflow" notions: the existing process model and a runtime workflow contract. The spec does not say whether the `release` workflow drives the existing artifact set, replaces it, or is a separate concept.

**Recommendation:** Add a sentence to §Workflow Contracts explaining whether the `release` workflow type produces or coexists with the existing `docs/releases/<version>/` artifacts. Same clarification likely needed for `feature`, `adr`, and `exploration` workflow types and their relationship to `docs/features/`, `docs/adr/`, `docs/explorations/`.

### F3 — Significant

**Reviewer:** Open Question Surfacing

**Affected sections:** Open Questions

**Summary:** A foundational question about background-run executor availability lives in FEAT-0017 but should be raised at the umbrella level too.

**Detail:** FEAT-0017 Open Question 1 asks whether background runs can continue local tool execution if no harness/local executor is connected. The answer determines whether the BFF needs server-side tools at all, which changes the BFF/harness boundary for the whole runtime. That is an umbrella-level concern, not a member-level concern.

**Recommendation:** Add this as an Open Question in FEAT-0015 so the future Run Runtime ADR is forced to address it.

### F4 — Advisory

**Reviewer:** Frontmatter Convention (cross-cutting)

**Affected sections:** Frontmatter (applies to FEAT-0016 through FEAT-0022)

**Summary:** Member specs use `promoted-from: FEAT-0015` redundantly with `parent: FEAT-0015`.

**Detail:** Per `docs/features/README.md` §Promotion Path, `promoted-from` is for explorations or patches that firm up into a feature spec ("keep the exploration and add a `promoted-to:` reference"). The 7 member specs are children of an umbrella, not promotions of one. The relationship is already encoded by `parent: FEAT-0015` and `series: Professional Harness Runtime`. FEAT-0015's own `promoted-from: EXP-0011` is correct usage; the members' usage stretches the semantic.

**Recommendation:** Drop `promoted-from: FEAT-0015` from FEAT-0016 through FEAT-0022. Keep `parent` and `series` metadata. Optionally, document an "umbrella-spawned" semantic in `docs/features/README.md` if you want the relationship explicit in metadata.

### F5 — Advisory

**Reviewer:** Workspace Identifier Consistency (cross-cutting)

**Affected sections:** Key Capabilities → Workspace Policy (also affects FEAT-0021)

**Summary:** Workspace mode identifiers are canonical here but drift in FEAT-0021.

**Detail:** FEAT-0015:0177-0182 defines snake_case identifiers (`current`, `current_readonly`, `worktree`, `temp_copy`, `remote`). FEAT-0021:0076-0082 uses prose names ("current workspace", "read-only current workspace", "Git worktree", "temp copy", "remote sandbox"). Same set, different surface form.

**Recommendation:** Pin the snake_case set in FEAT-0015 as canonical and have FEAT-0021 reference rather than re-name them.

### F6 — Advisory

**Reviewer:** Document Hygiene

**Affected sections:** Feature Relationship Map

**Summary:** "PATCH: Codegen Evaluation Harness" appears as rank 9 in the relationship map but has no reserved `PATCH-NNNN` identifier.

**Detail:** FEAT-0015:0311 lists this as a planned artifact at the end of the family but does not assign or reserve an ID, leaving readers unsure whether it is in flight, deferred, or speculative.

**Recommendation:** Either reserve a `PATCH-NNNN` (e.g. `PATCH-0008`) when the patch series is checked, or annotate the entry as "future, not yet drafted" so the relationship map is unambiguous.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| F1 | accepted | Aligned FEAT-0015 and FEAT-0016 around explicit status/stage/attachment axes and canonical stage names. |
| F2 | accepted | Clarified that artifact workflows produce or revise existing docs/* artifact families and must honor canonical process, including release phases. |
| F3 | accepted | Added the disconnected local-executor/background-run question to FEAT-0015 Open Questions and Future ADR coverage. |
| F4 | accepted | Removed promoted-from: FEAT-0015 from FEAT-0016 through FEAT-0022. |
| F5 | accepted | Pinned FEAT-0015 workspace mode identifiers as canonical and changed FEAT-0021 to use them. |
| F6 | accepted | Removed the undrafted patch from the feature relationship map and noted it separately as a future supporting patch. |
