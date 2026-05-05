# FEAT-0017 Findings (Claude)

- Feature: `docs/features/0017-durable-runs-and-background-agents.md`
- Review date: 2026-04-29
- Reviewer: Claude Opus 4.7 (1M context)
- total_findings: 3
- blocking: 0
- significant: 2
- advisory: 1
- top_line: The attached/detached/observable/blocked attachment-state model is useful, but it overlaps FEAT-0015's `waiting_permission`/`waiting_user` lifecycle states without explaining whether attachment and lifecycle are orthogonal axes or one merged dimension. Open Question 1 (whether background runs can continue local tool execution with no harness/local executor connected) is foundational enough to belong in FEAT-0015 too — the answer determines whether the BFF needs server-side tools.

## Findings

### F1 — Significant

**Reviewer:** Conceptual Modeling

**Affected sections:** Attachment Semantics, Background Permission Behavior

**Summary:** Attachment state and FEAT-0015 lifecycle state overlap without an explicit relationship.

**Detail:** FEAT-0017:0046-0052 defines attachment states `attached`, `detached`, `observable`, `blocked`. FEAT-0015:0077-0090 lists lifecycle states including `waiting_permission` and `waiting_user`. A "blocked" run in FEAT-0017 corresponds to a `waiting_permission` or `waiting_user` run in FEAT-0015 — but it is not clear whether these are two orthogonal axes (lifecycle × attachment, so a run is e.g. `running × detached` or `waiting_permission × attached`) or a single axis where blocked subsumes the lifecycle state. Downstream design has to guess.

**Recommendation:** Add a short subsection clarifying that lifecycle state and attachment state are orthogonal (preferred) or that one collapses into the other, and show an example or two of valid combinations. The future Run Runtime ADR can then ratify the model.

### F2 — Significant

**Reviewer:** Open Question Surfacing

**Affected sections:** Open Questions

**Summary:** Open Question 1 about local-executor availability is umbrella-level, not member-level.

**Detail:** "Can background runs continue local tool execution if no harness/local executor is connected?" determines whether the BFF needs to grow server-side tools, which changes the BFF/harness boundary for the entire runtime — not just background runs. The question also influences FEAT-0021 (workspace-aware execution), FEAT-0019 (validation under what runtime), and FEAT-0020 (artifact persistence when files are not BFF-readable).

**Recommendation:** Echo this question in FEAT-0015's Open Questions so the future Run Runtime ADR is forced to address it explicitly.

### F3 — Advisory

**Reviewer:** Frontmatter Convention

**Affected sections:** Frontmatter

**Summary:** `promoted-from: FEAT-0015` is redundant with `parent: FEAT-0015`.

**Detail:** See FEAT-0015 review F4.

**Recommendation:** Drop `promoted-from: FEAT-0015` from frontmatter.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| F1 | accepted | Clarified attachment state as orthogonal to run status and pipeline stage. |
| F2 | accepted | Raised the local-executor availability question to FEAT-0015. |
| F3 | accepted | Removed promoted-from: FEAT-0015 from frontmatter. |
