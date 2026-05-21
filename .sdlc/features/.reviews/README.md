# Feature Reviews

This directory stores review artifacts for feature specs and feature-adjacent planning docs.

## Layout

Canonical per-feature findings stay at the root as markdown only:

- `.sdlc/features/.reviews/{feature-stem}-findings.md`

The `stem` matches the feature filename without the `.md` extension (e.g. `0003-web-dashboard`). When a reviewer identity matters (model, role, or harness), include it in the filename — for example `0015-professional-harness-runtime-architect-sre-findings.md`.

Plan reviews — reviews of implementation plans derived from a feature spec, technical design writeups tied to one feature, or narrower execution reviews — live under:

- `.sdlc/features/.reviews/plan-reviews/`

When the reviewing model or harness is known, include it in the plan-review filename:

- `.sdlc/features/.reviews/plan-reviews/codex-0008-bff-server-connectivity-review.md`

Cross-feature syntheses (baseline crosswalks, prereviews spanning a group of features, design-thoughts reviews) live under:

- `.sdlc/features/.reviews/syntheses/`

## Findings Format

Review artifacts are markdown only. Each formally reviewed feature spec keeps a single canonical findings file:

- `{stem}-findings.md`

Each finding documents:

- short identifier (e.g. `F1`)
- one-line summary
- reviewer role or identity (e.g. `Architecture`, `SRE`, `Security`)
- severity — `blocking` | `significant` | `advisory`
- affected sections of the feature spec
- detail
- recommendation
- disposition — `accepted` | `rejected` | `deferred` | `null` until resolved
- disposition rationale

Each markdown file should include a top-line summary block at the top:

```
total_findings, blocking, significant, advisory, top_line
```

Dispositions live in a table at the bottom of the same markdown file. No paired JSON sidecar — the canonical rule is in `.agents/process.md` under "Review Artifact Placement".

## When to Write a Review

Not every feature spec requires a formal `.reviews` artifact. Write one when:

- A feature is being moved from `proposed` to `accepted` and the trade-offs deserve recorded review
- Multiple reviewers (architecture, implementation readiness, security, integration) raise findings the author needs to address
- A feature touches a sensitive surface (auth, capture/redaction, storage migrations) and the review establishes guardrails for the implementation
- The feature is part of a cross-feature synthesis being prepared

For features that pass review without significant findings, the commit history and the feature spec itself are sufficient.
