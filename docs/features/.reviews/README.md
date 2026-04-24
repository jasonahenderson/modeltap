# Feature Reviews

This directory stores review artifacts for feature specs and feature-adjacent planning docs.

## Layout

Canonical per-feature findings stay at the root:

- `docs/features/.reviews/{feature-stem}-findings.md`

The `stem` matches the feature filename without the `.md` extension (e.g. `0003-web-dashboard`).

Plan reviews — reviews of implementation plans derived from a feature spec, technical design writeups tied to one feature, or narrower execution reviews — live under:

- `docs/features/.reviews/plan-reviews/`

When the reviewing model or harness is known, include it in the plan-review filename:

- `docs/features/.reviews/plan-reviews/codex-0008-bff-server-connectivity-review.md`

Cross-feature syntheses (baseline crosswalks, prereviews spanning a group of features, design-thoughts reviews) live under:

- `docs/features/.reviews/syntheses/`

## Canonical File

Each formally reviewed feature spec keeps a single canonical file: `{stem}-findings.md`. Findings and their dispositions live in one document so they cannot drift.

## Findings Schema

Each finding in the markdown file should include:

- `id` — short identifier (e.g. `F1`)
- `summary` — one-line description
- `reviewer` — reviewer role (e.g. `Architecture Conformance`, `Implementation Readiness`, `Security`)
- `severity` — `blocking` | `significant` | `advisory`
- `affected_sections` — sections of the feature spec the finding touches
- `detail` — full explanation
- `recommendation` — concrete next action

A top-line summary block appears near the top of the file:

```
total_findings, blocking, significant, advisory, top_line
```

## Dispositions

Each findings file ends with a Dispositions table. One row per finding, tracking how the author resolved it:

```markdown
## Dispositions

| ID | Severity | Disposition | Rationale |
|----|----------|-------------|-----------|
| F1 | blocking | accepted | Protocol appendix added in commit abc123 |
| F2 | significant | deferred | Tracked under FEAT-0014 |
```

Disposition values are `accepted`, `rejected`, or `deferred`. Use `—` until resolved. Always include a rationale when the disposition is `rejected` or `deferred`; `accepted` rationales are optional but helpful.

## When to Write a Review

Not every feature spec requires a formal `.reviews` artifact. Write one when:

- A feature is being moved from `proposed` to `accepted` and the trade-offs deserve recorded review
- Multiple reviewers (architecture, implementation readiness, security, integration) raise findings the author needs to address
- A feature touches a sensitive surface (auth, capture/redaction, storage migrations) and the review establishes guardrails for the implementation
- The feature is part of a cross-feature synthesis being prepared

For features that pass review without significant findings, the commit history and the feature spec itself are sufficient.
