# Feature Specs

This directory holds **behavior-scoped** product definitions for modeltap that are concrete enough to drive implementation. Feature specs capture user-visible capabilities, the personas they serve, and the success criteria that determine "done." They are the unit of work that ADRs constrain and that work units (`WU-NNN`) implement.

## Feature Index

| Feature | Title | Status |
|---------|-------|--------|
| [FEAT-0003](0003-web-dashboard.md) | Web Dashboard | accepted |
| [FEAT-0004](0004-service-management.md) | Service Management | accepted |
| [FEAT-0008](0008-bff-server.md) | BFF Server | proposed |
| [FEAT-0009](0009-terminal-harness.md) | Terminal Harness | proposed |
| [FEAT-0010](0010-enterprise-auth.md) | Enterprise Auth and Multi-User | proposed |
| [FEAT-0011](0011-knowledge-integration.md) | Knowledge Integration | proposed |
| [FEAT-0012](0012-skills-and-agent-teams.md) | Skills | proposed |
| [FEAT-0013](0013-agent-teams.md) | Agent Teams | proposed |
| [FEAT-0014](0014-harness-conversation-shell.md) | Harness Conversation Shell | accepted |
| [FEAT-0015](0015-professional-harness-runtime.md) | Professional Harness Runtime | accepted |
| [FEAT-0016](0016-managed-codegen-run-pipeline.md) | Managed Codegen Run Pipeline | accepted |
| [FEAT-0017](0017-durable-runs-and-background-agents.md) | Durable Runs and Background Agents | accepted |
| [FEAT-0018](0018-context-planner-and-project-rules.md) | Context Planner and Project Rules | draft |
| [FEAT-0019](0019-validation-and-repair-loop.md) | Validation and Repair Loop | draft |
| [FEAT-0020](0020-patch-evidence-and-run-artifacts.md) | Patch Evidence and Run Artifacts | draft |
| [FEAT-0021](0021-policy-grade-tool-runtime.md) | Policy-Grade Tool Runtime | draft |
| [FEAT-0022](0022-memory-routing-and-workflow-extensions.md) | Durable Memory, Quality Routing, and Workflow Extensions | draft |
| [FEAT-0024](0024-shell-ux-chrome.md) | Shell UX Chrome — sidebar, command palette, slash autocomplete, agents view | draft |

Other previously feature-shaped docs were reclassified when they proved to be upstream explorations or implementation-scoped patches rather than active behavior contracts. See `docs/explorations/` and `docs/patches/`.

## When to Write a Feature Spec

- The work introduces a new user-visible capability or substantially changes an existing one
- Multiple personas, user stories, or acceptance criteria need to be recorded
- The work spans multiple work units and needs a coherent target
- Stakeholders need a shared understanding of the problem and the success criteria before implementation begins

## When NOT to Write a Feature Spec

- The work is still problem framing or design-space exploration → use an **exploration** in `docs/explorations/`
- The work is implementation-scoped (bug fix, missing endpoint coverage, internal plumbing) → use a **patch** in `docs/patches/`
- The work is an architectural decision with future constraint value → use an **ADR** in `docs/adr/`
- The change is repo process / workflow / instruction-file only → commit with `ADMIN:` prefix, no doc needed

## Naming

Files: `NNNN-short-title.md` — four-digit zero-padded sequence (e.g. `0003-web-dashboard.md`).

Identifiers: `FEAT-NNNN` in the document title heading and YAML frontmatter (e.g. `# FEAT-0003: Web Dashboard`).

Numbering is monotonic; do not reuse a number even if a feature is abandoned. Numbers are assigned in chronological order of when the spec was first drafted, not when it was accepted.

## Format

Feature specs use YAML frontmatter for machine-readable metadata followed by a structured markdown body. The frontmatter shape:

```yaml
---
feature: FEAT-NNNN
title: Human-readable title
status: draft | proposed | accepted | superseded
date: YYYY-MM-DD
parent: FEAT-NNNN
series: Human-readable grouping name
series-role: umbrella | member
series-order: 1
adr-constraints:
  - ADR-NNNN: Short reason this ADR constrains the feature
  - ADR-NNNN: ...
depends-on:
  - FEAT-NNNN: Required predecessor feature
related:
  - EXP-NNNN: Related upstream exploration
promoted-from:
  - EXP-NNNN: Upstream exploration or PATCH-NNNN: Upstream patch
decomposed-from:
  - FEAT-NNNN: Umbrella feature, when parent/series metadata is insufficient
---
```

Required fields:

- `feature`
- `title`
- `status`
- `date`

Optional grouping fields:

- `parent`: canonical parent artifact ID
- `series`: human-readable grouping name
- `series-role`: `umbrella` or `member`
- `series-order`: optional integer for planned order within the series

Other optional relationship fields:

- `depends-on`: features that must land before this feature can be implemented
  or accepted as a complete behavior contract
- `related`: non-blocking related features, explorations, ADRs, patches, or
  release artifacts
- `promoted-from`: upstream exploration or patch that became this feature
- `decomposed-from`: umbrella feature that produced this child feature, if a
  relationship beyond `parent` / `series` is needed

Use grouping metadata for umbrella features and related feature families rather
than changing the ID format. For example, keep child features as `FEAT-0016`,
`FEAT-0017`, etc., and point them at `parent: FEAT-0015`.

The body follows this rough shape — sections may vary, but the load-bearing parts are the problem statement, the proposed solution, the key capabilities, and the success criteria:

```markdown
# FEAT-NNNN: Title

## Problem
What's missing or broken from a user perspective. Why does this need to exist?

## Solution
The high-level approach. One or two paragraphs.

## Key Capabilities
The concrete capabilities the feature delivers, grouped by area.

## CLI / UI / API Integration
How the feature surfaces to the user.

## Configuration
New config keys, env vars, or CLI flags introduced.

## Non-Goals
What this feature explicitly does NOT do.

## Success Criteria
Numbered, testable criteria that determine when the feature is "done."

## Relationship to ADRs
Which ADRs constrain this feature and how.

## Open Questions
Unresolved trade-offs the spec still needs to settle.
```

Existing features in this directory follow this shape. New features should match.

## Lifecycle

1. **Draft** — Status `draft`. Spec is a sketch; problem and solution are still being shaped.
2. **Propose** — Status `proposed`. Spec is complete enough to review. Open questions are explicit. Decision-makers can evaluate trade-offs.
3. **Accept** — Status `accepted`. The feature is selected for implementation. Work units are planned in `docs/releases/<version>/` and tracked through completion. Only `accepted` features drive work per `CLAUDE.md`.
4. **Supersede** — If a later feature replaces this one, set status to `superseded` and add a forward reference. Do not delete the original.

## Commit Convention

- Commits implementing an accepted feature use the work-unit prefix: `WU-NNN: short description`
- Commits drafting or revising the feature spec itself can use a `FEAT-NNNN:` prefix (e.g. `FEAT-0003: tighten dashboard access-control wording`)
- Use `git commit -s` for DCO sign-off

## Promotion Path

- An exploration that firms up into a behavior-scoped capability should promote to a feature spec — keep the exploration and add a `promoted-to:` reference
- A patch that grows past ~8 checklist items or sprouts user stories should promote to a feature spec — keep the patch doc and add a `Promoted to:` reference
- A feature whose central question is really an architectural choice should spawn or wait on an ADR before acceptance

## Reviews

Review artifacts (canonical findings, plan reviews, syntheses) live under `docs/features/.reviews/`. See `.reviews/README.md` for the layout.

## Relationship to Other Docs

| Doc Type | Scope | Lives In |
|----------|-------|----------|
| Exploration | Upstream problem framing and design-space exploration | `docs/explorations/` |
| ADR | Architectural decisions with future constraint value | `docs/adr/` |
| **Feature spec** | **Behavior — user-visible capabilities** | **`docs/features/`** |
| Patch | Implementation — fixes, missing endpoints, internal work | `docs/patches/` |
| Work unit (`WU-NNN`) | Planned increments inside a feature, tracked in status.md | `docs/history/` |
