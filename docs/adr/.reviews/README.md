# ADR Reviews

This directory stores review artifacts for architectural decision records.

## Layout

Canonical per-ADR findings stay at the root:

- `docs/adr/.reviews/{adr-stem}-findings.md`
- `docs/adr/.reviews/{adr-stem}-findings.json`

The `stem` matches the ADR filename without the `.md` extension (e.g. `0006-multi-provider-support`).

Plan reviews — reviews of implementation plans derived from an ADR or narrower execution reviews tied to a single ADR — live under:

- `docs/adr/.reviews/plan-reviews/`

When the reviewing model or harness is known, include it in the plan-review filename:

- `docs/adr/.reviews/plan-reviews/codex-0006-provider-formatting-plan-review.md`
- `docs/adr/.reviews/plan-reviews/codex-0006-provider-formatting-plan-review.json`

Cross-cutting syntheses (multi-ADR reviews, baseline crosswalks, supersession analyses) live under:

- `docs/adr/.reviews/syntheses/`

## Canonical Files

Each formally reviewed ADR keeps exactly two canonical files:

- `{stem}-findings.md` — markdown findings for humans
- `{stem}-findings.json` — structured findings for tooling

## Findings Schema

Each finding in the JSON file should include:

- `id` — short identifier (e.g. `F1`)
- `summary` — one-line description
- `reviewer` — reviewer role (e.g. `Architecture Conformance`, `Implementation Readiness`, `Security`)
- `severity` — `blocking` | `significant` | `advisory`
- `affected_drivers` — drivers from the ADR's decision-driver list that the finding touches
- `affected_options` — options the finding affects scoring or evaluation of
- `scoring_impact` — narrative description of how the finding shifts scoring (or `"No scoring impact"`)
- `detail` — full explanation
- `recommendation` — concrete next action
- `disposition` — `accepted` | `rejected` | `deferred` | `null` until resolved
- `disposition_rationale` — why the disposition was chosen

The markdown file mirrors this structure for human reading and includes a top-line summary block:

```
total_findings, blocking, significant, advisory, top_line
```

## When to Write a Review

Not every ADR requires a formal `.reviews` artifact. Write one when:

- Multiple reviewers (architecture, implementation readiness, security) raise findings against a proposed ADR before it can be accepted
- An ADR is being superseded and the supersession analysis is non-trivial
- An ADR's score adjustments deserve a recorded paper trail
- A cross-ADR baseline review is being conducted (use `syntheses/`)

For straightforward ADRs that pass review without findings, the commit history and the ADR itself are sufficient — no `.reviews` artifact needed.
