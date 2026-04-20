# Patches

Implementation-scoped work authorization documents. Patches are the lightweight counterpart to feature specs (`docs/features/`) and ADRs (`docs/adr/`).

## Patch Index

| Patch | Title | Status |
|-------|-------|--------|
| [PATCH-0001](0001-openai-responses-api-support.md) | OpenAI Responses API Support | proposed |
| [PATCH-0002](0002-local-inference-support.md) | Local Inference Support | proposed |
| [PATCH-0003](0003-harness-app-conn-mgr-wiring.md) | Harness App ↔ ConnectionManager Wiring | approved |
| [PATCH-0004](0004-secret-prefix-resolver.md) | Secret Prefix Resolver for Provider API Keys | done |
| [PATCH-0005](0005-bff-route-via-proxy-default.md) | Route BFF provider traffic through the v0.1 proxy by default | approved |

## When to Use a Patch

- The work is implementation-scoped: bug fixes, missing endpoint coverage, internal plumbing, tooling, infrastructure, DX, test-harness improvements
- The work affects the product codebase or its delivery system
- The problem and scope are well-defined
- A checklist is sufficient to define "done"
- No personas, user stories, or acceptance criteria are needed

## When NOT to Use a Patch

- The work is still problem framing or design-space exploration → use an **exploration** in `docs/explorations/`
- The work is behavior-scoped (new capability surfaced to users, multiple personas, success criteria) → use a **feature spec** in `docs/features/`
- The work requires an architectural choice with future constraint value → use an **ADR** in `docs/adr/`
- The change is repo process / workflow / instruction-file only (CLAUDE.md, agents.md, skills, hooks) → commit directly with an `ADMIN:` prefix, no doc needed

Patch classification is about scope, not release semantics:

- `PATCH` here does **not** mean semver patch
- Use a patch for implementation-scoped work whether it's a bug fix, a missing-endpoint adapter extension, a CI/tooling addition, or an internal plumbing improvement
- Do **not** use a patch for workflow or process artifacts (those are `ADMIN:` commits)

## Naming

Files: `NNNN-title-with-dashes.md` — four-digit zero-padded sequence (e.g., `0001-openai-responses-api-support.md`).

Identifiers: `PATCH-NNNN` in the document title (e.g., `PATCH-0001`).

Numbering is monotonic across the patches directory; do not reuse a number even if a patch is abandoned.

## Format

```markdown
# PATCH-NNNN: Title

**Status:** proposed | approved | done
**Date:** YYYY-MM-DD
**Related:** FEAT-name, ADR-NNNN, PATCH-NNNN (optional — associated features, ADRs, or patches)
**Branch:** patch/NNNN-short-name
**PR:** (added when PR is created)

## Problem
What's broken or missing, in 2-3 sentences. Include severity and how it was discovered if relevant.

## Scope
Bulleted list of what this patch does. Reference concrete file paths and line numbers where possible.

## Out of Scope
What it explicitly does NOT do. Adjacent bugs, broader refactors, future work.

## Checklist
- [ ] Concrete deliverables, testable items
- [ ] Tests added or updated
- [ ] Build and vet pass

## Fix Detail (optional)
Code snippets, design notes, or commentary that don't fit in the checklist but help a reviewer understand the change.
```

The status, date, related, branch, and PR fields use inline bold-key formatting (not YAML frontmatter) to match this project's existing feature-doc style of light-touch metadata.

## Lifecycle

1. **Propose** — Write the patch doc with status `proposed`. Number it next in sequence.
2. **Approve** — Human review. Status flips to `approved` once the scope is agreed.
3. **Implement** — Commits land with `PATCH-NNNN:` prefix on a `patch/NNNN-short-name` branch (or directly to main for trivial fixes, noted in the patch doc). Multiple commits may iterate under one patch.
4. **Done** — All checklist items complete, status flips to `done`, PR link recorded in the doc.

## Commit Convention

- Branch: `patch/NNNN-short-name`
- Commit subject: `PATCH-NNNN: short description`
- Commit body references the patch doc path so reviewers can jump to the authorization
- Use `git commit -s` for DCO sign-off, matching the project's existing commit policy in `CLAUDE.md`

## Relationship to Other Docs

| Doc Type | Scope | Lives In |
|----------|-------|----------|
| Exploration | Upstream problem framing and design-space exploration | `docs/explorations/` |
| Feature spec | Behavior — new capabilities, user-visible work | `docs/features/` |
| **Patch** | **Implementation — fixes, missing endpoints, internal work** | **`docs/patches/`** |
| ADR | Architectural decisions with future constraint value | `docs/adr/` |
| Work unit (`WU-NNN`) | Planned increments inside a feature, tracked in status.md | `docs/history/` |

Work units (`WU-NNN`) are the existing planning unit for advancing accepted features through `tpm`. Patches are a separate axis for fixes and small implementation-scoped work that doesn't fit a feature's work-unit plan.

## Promotion

An exploration may promote directly to a patch when the topic becomes implementation-scoped and a checklist is enough.

If a patch grows past ~8 checklist items, sprouts user stories, or starts requiring an architectural choice, promote it:

- To a **feature spec** if it has become behavior-scoped — keep the patch doc and add a `Promoted to:` reference at the top.
- To an **ADR** if a real architectural decision emerged — same pattern.

## Reviews

Review artifacts (findings, plan reviews, syntheses) live under `docs/patches/.reviews/`. See `.reviews/README.md` for layout.
