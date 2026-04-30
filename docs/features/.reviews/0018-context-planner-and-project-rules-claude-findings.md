# FEAT-0018 Findings (Claude)

- Feature: `docs/features/0018-context-planner-and-project-rules.md`
- Review date: 2026-04-29
- Reviewer: Claude Opus 4.7 (1M context)
- total_findings: 2
- blocking: 0
- significant: 0
- advisory: 2
- top_line: Spec is clean and the BFF/harness split is consistent with ADR-0014. No structural issues. The two notes are minor: a redundant `promoted-from` field, and a tighter-than-necessary downstream coupling with FEAT-0019 (validation depends on this spec, but does not strictly require the full context planner to ship first).

## Findings

### F1 — Advisory

**Reviewer:** Sequencing

**Affected sections:** Relationship to other features (downstream — FEAT-0019)

**Summary:** FEAT-0019 declares a hard dependency on FEAT-0018, but a minimum-viable validation slice may not need the full context planner.

**Detail:** FEAT-0019:0010-0012 lists `depends-on: FEAT-0018`. FEAT-0019's validation logic primarily needs a changed-file list, language detection, and a way to surface failures back into a repair turn — none of which strictly require the full context planner from FEAT-0018. Provenance and repo-aware selection from FEAT-0018 strengthen FEAT-0019, but they are not required for a first slice. This is a downstream coupling note rather than a defect of FEAT-0018.

**Recommendation:** Note in FEAT-0018 (or in FEAT-0019) that FEAT-0019 can be implemented with a minimal context plan that only carries changed-file metadata, and that full FEAT-0018 capability strengthens but does not gate FEAT-0019.

### F2 — Advisory

**Reviewer:** Frontmatter Convention

**Affected sections:** Frontmatter

**Summary:** `promoted-from: FEAT-0015` is redundant with `parent: FEAT-0015`.

**Detail:** See FEAT-0015 review F4.

**Recommendation:** Drop `promoted-from: FEAT-0015` from frontmatter.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| F1 | accepted | Added the sequencing clarification in FEAT-0019: basic validation can start with minimal context; FEAT-0018 enriches repair quality. |
| F2 | accepted | Removed promoted-from: FEAT-0015 from frontmatter. |
