# FEAT-0021 Findings (Claude)

- Feature: `.sdlc/features/0021-policy-grade-tool-runtime.md`
- Review date: 2026-04-29
- Reviewer: Claude Opus 4.7 (1M context)
- total_findings: 3
- blocking: 0
- significant: 1
- advisory: 2
- top_line: Spec captures the right policy dimensions and audit shape. Two concerns: the `depends-on: FEAT-0017` is tighter than necessary (foreground-only policy could ship without durable backgrounding), and workspace mode names diverge from FEAT-0015's canonical snake_case identifiers.

## Findings

### F1 — Significant

**Reviewer:** Sequencing

**Affected sections:** Frontmatter (depends-on), Workspace-Aware Execution

**Summary:** Hard `depends-on: FEAT-0017` may force a later landing than necessary.

**Detail:** FEAT-0021:0010-0014 lists `depends-on: FEAT-0009, FEAT-0016, FEAT-0017`. Path/command/Git/domain policy and the audit-trail capability are useful for foreground runs alone. Background-specific policy (auto-deny, pre-approved scopes, blocked-run inbox) does need FEAT-0017, but those are a subset of what FEAT-0021 specifies. Forcing the whole feature behind FEAT-0017 means policy-grade tools cannot land for foreground use until durable runs ship — and policy-grade tools are arguably more valuable to land first because they protect the foreground experience that already exists today.

**Recommendation:** Either split FEAT-0021 into a foreground-policy slice (no FEAT-0017 dependency) and a background-policy slice (depends on FEAT-0017), or keep one feature but document explicit phasing in §Success Criteria so the foreground slice can start earlier.

### F2 — Advisory

**Reviewer:** Vocabulary Consistency

**Affected sections:** Workspace-Aware Execution

**Summary:** Workspace mode names diverge from FEAT-0015's canonical identifiers.

**Detail:** FEAT-0015:0177-0182 defines snake_case identifiers (`current`, `current_readonly`, `worktree`, `temp_copy`, `remote`). FEAT-0021:0076-0082 uses prose forms ("current workspace", "read-only current workspace", "Git worktree", "temp copy", "remote sandbox"). Same set, different surface form.

**Recommendation:** Use the snake_case identifiers from FEAT-0015 verbatim in FEAT-0021 §Workspace-Aware Execution.

### F3 — Advisory

**Reviewer:** Frontmatter Convention

**Affected sections:** Frontmatter

**Summary:** `promoted-from: FEAT-0015` is redundant with `parent: FEAT-0015`.

**Detail:** See FEAT-0015 review F4.

**Recommendation:** Drop `promoted-from: FEAT-0015` from frontmatter.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| F1 | accepted | Moved FEAT-0017 from depends-on to related and added a foreground-policy-first success criterion. |
| F2 | accepted | Changed workspace mode names to FEAT-0015 canonical snake_case identifiers. |
| F3 | accepted | Removed promoted-from: FEAT-0015 from frontmatter. |
