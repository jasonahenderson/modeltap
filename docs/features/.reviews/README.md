# Feature Reviews

This directory stores review artifacts for feature specs and feature-adjacent planning docs.

## Layout

Canonical per-feature findings stay at the root:

- `docs/features/.reviews/{feature-stem}-findings.md`
- `docs/features/.reviews/{feature-stem}-findings.json`

The `stem` matches the feature filename without the `.md` extension (e.g. `0003-web-dashboard`).

Plan reviews — reviews of implementation plans derived from a feature spec, technical design writeups tied to one feature, or narrower execution reviews — live under:

- `docs/features/.reviews/plan-reviews/`

When the reviewing model or harness is known, include it in the plan-review filename:

- `docs/features/.reviews/plan-reviews/codex-0008-bff-server-connectivity-review.md`
- `docs/features/.reviews/plan-reviews/codex-0008-bff-server-connectivity-review.json`

Cross-feature syntheses (baseline crosswalks, prereviews spanning a group of features, design-thoughts reviews) live under:

- `docs/features/.reviews/syntheses/`

## Canonical Files

Each formally reviewed feature spec keeps exactly two canonical files:

- `{stem}-findings.md` — markdown findings for humans
- `{stem}-findings.json` — structured findings for tooling

## Findings Schema

Each finding in the JSON file should include:

- `id` — short identifier (e.g. `F1`)
- `summary` — one-line description
- `reviewer` — reviewer role (e.g. `Architecture Conformance`, `Implementation Readiness`, `Security`)
- `severity` — `blocking` | `significant` | `advisory`
- `affected_sections` — sections of the feature spec the finding touches
- `detail` — full explanation
- `recommendation` — concrete next action
- `disposition` — `accepted` | `rejected` | `deferred` | `null` until resolved
- `disposition_rationale` — why the disposition was chosen

The markdown file mirrors this structure for human reading and includes a top-line summary block:

```
total_findings, blocking, significant, advisory, top_line
```

## When to Write a Review

Not every feature spec requires a formal `.reviews` artifact. Write one when:

- A feature is being moved from `proposed` to `accepted` and the trade-offs deserve recorded review
- Multiple reviewers (architecture, implementation readiness, security, integration) raise findings the author needs to address
- A feature touches a sensitive surface (auth, capture/redaction, storage migrations) and the review establishes guardrails for the implementation
- The feature is part of a cross-feature synthesis being prepared

For features that pass review without significant findings, the commit history and the feature spec itself are sufficient.
