# Explorations

Explorations are upstream product and design artifacts. They capture problem framing, design space, tensions, and open questions before a topic is ready to become a feature spec, ADR, or implementation patch.

## Exploration Index

| Exploration | Title | Status |
|-------------|-------|--------|
| [EXP-0001](0001-knowledge-layer.md) | Knowledge Layer (Cross-Model Brain) | promoted |
| [EXP-0002](0002-multi-user-support.md) | Multi-User Support | exploring |
| [EXP-0005](0005-apprenticeship.md) | Apprenticeship Program | exploring |
| [EXP-0007](0007-multi-model-orchestration.md) | Multi-Model Orchestration | exploring |
| [EXP-0008](0008-integrated-harness.md) | Integrated Harness — Modeltap as Professional AI Environment | exploring |
| [EXP-0009](0009-harness-prompt-architecture.md) | Harness Prompt Architecture — Lessons from the Claude Code Leak | exploring |
| [EXP-0010](0010-harness-comparative-analysis.md) | Harness Comparative Analysis — modeltap, OpenCode, and OpenHarness | exploring |
| [EXP-0011](0011-harness-excellence-gap-analysis.md) | Harness Excellence Gap Analysis | exploring |
| [EXP-0012](0012-code-graphing-via-ast.md) | Code Graphing via AST for Repository-Aware Context | exploring |
| [EXP-0013](0013-ultron-evaluation.md) | Ultron Evaluation — Tiered Memory and Skill Evolution from ModelScope | watching |

## Lifecycle

The expected flow is:

`exploration -> feature spec and/or ADR and/or patch -> implementation plan -> code`

Not every exploration becomes a downstream artifact. Some are deferred, superseded, closed, or split across multiple later docs.

## When to Use an Exploration

Use an exploration when:

- the problem is still being framed
- multiple solution shapes are plausible
- the topic may promote into a feature spec, ADR, or patch
- you want durable upstream rationale before committing to a formal downstream contract

Do not use an exploration as implementation authorization. If code should land, promote the topic into an accepted feature/work unit or an approved patch first.

## File Naming

Explorations use the same numbered naming pattern as ADRs, features, and patches:

- `docs/explorations/NNNN-title-with-dashes.md`

The document identifier inside the file uses the `EXP-` prefix:

- `EXP-0001`
- `EXP-0002`

The file number and exploration number must match.

## Front Matter

Every exploration should start with YAML front matter:

```yaml
---
exploration: EXP-0001
title: Short Title
status: exploring
date: 2026-04-10
related:
  - EXP-0002
  - ADR-0008
promoted-to:
  - FEAT-0003
---
```

Required fields:

- `exploration`
- `title`
- `status`
- `date`

Optional fields:

- `related`
- `promoted-to`
- `supersedes`
- `superseded-by`
- `parent`
- `series`
- `series-role`
- `series-order`

Use optional grouping metadata when an exploration belongs to an umbrella,
roadmap, or other cross-artifact work stream:

```yaml
parent: EXP-0011
series: Harness Excellence
series-role: member
series-order: 2
```

Do not encode hierarchy in the exploration identifier. Keep IDs monotonic and
put grouping relationships in metadata.

## Status Values

Use one of:

- `exploring` — active problem-space exploration
- `watching` — not actively pursuing, but tracking an external project or
  development for changes that might shift our decision
- `deferred` — intentionally parked for later
- `promoted` — exploration produced downstream canonical artifact(s)
- `superseded` — replaced by another exploration or a more canonical document
- `closed` — decided not to pursue

If an exploration is promoted, keep the exploration as the upstream rationale record and add `promoted-to`.

## Content Shape

Explorations are lighter than feature specs and ADRs, but should still be structured. Prefer this shape:

1. Context
2. Problem or motivating question
3. Design space / options
4. Tensions and tradeoffs
5. Open questions
6. Proposed next step

## Review Artifacts

If formal review artifacts are created for explorations, store them in:

- `docs/explorations/.reviews/`

Use the exploration file stem for review artifacts:

- `docs/explorations/.reviews/0001-some-topic-review.md`
- `docs/explorations/.reviews/0001-some-topic-review.json`

Exploration reviews are advisory. They should capture challenge, unresolved questions, and promotion recommendations, but they do not use the ADR acceptance process.

## Promotion Rules

Promote an exploration to a feature spec when:

- user-facing behavior needs to be specified
- capabilities and success criteria need a durable contract
- implementation work needs a scoped behavior target

Promote an exploration to a patch when:

- the work is implementation-scoped
- a checklist is sufficient to define done
- no personas or behavior-level acceptance criteria are needed

Promote an exploration to an ADR when:

- an architectural decision becomes necessary
- the choice is constraining, hard to reverse, or cross-cutting

When promotion happens:

1. Keep the exploration file.
2. Add `promoted-to` in front matter.
3. Cross-link the resulting feature, patch, or ADR back to the exploration when useful.
