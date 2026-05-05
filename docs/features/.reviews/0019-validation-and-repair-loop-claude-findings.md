# FEAT-0019 Findings (Claude)

- Feature: `docs/features/0019-validation-and-repair-loop.md`
- Review date: 2026-04-29
- Reviewer: Claude Opus 4.7 (1M context)
- total_findings: 2
- blocking: 0
- significant: 0
- advisory: 2
- top_line: Spec is well scoped and the validation-then-repair flow is coherent. Two minor notes: the `depends-on: FEAT-0018` is tighter than the actual dependency requires, and the `promoted-from: FEAT-0015` field is redundant.

## Findings

### F1 — Advisory

**Reviewer:** Sequencing

**Affected sections:** Frontmatter (depends-on)

**Summary:** Hard `depends-on: FEAT-0018` may be tighter than required.

**Detail:** FEAT-0019:0010-0012 lists `depends-on: FEAT-0016, FEAT-0018`. The validation pipeline primarily needs a changed-file list, language/toolchain detection, and a way to feed failure summaries back to the model. The full FEAT-0018 context planner (provenance, project-rule precedence, repo-aware selection, budgeting) is a strengthener for repair-turn quality, not a hard prerequisite for executing checks.

**Recommendation:** Either weaken to `depends-on: FEAT-0016` and document that FEAT-0018 enriches FEAT-0019, or split FEAT-0019's success criteria into a slice that lands without FEAT-0018 plus a slice that requires it.

### F2 — Advisory

**Reviewer:** Frontmatter Convention

**Affected sections:** Frontmatter

**Summary:** `promoted-from: FEAT-0015` is redundant with `parent: FEAT-0015`.

**Detail:** See FEAT-0015 review F4.

**Recommendation:** Drop `promoted-from: FEAT-0015` from frontmatter.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| F1 | accepted | Moved FEAT-0018 from depends-on to related and documented the minimal-context validation slice. |
| F2 | accepted | Removed promoted-from: FEAT-0015 from frontmatter. |
