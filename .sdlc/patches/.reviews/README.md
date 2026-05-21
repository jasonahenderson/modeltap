# Patch Reviews

This directory stores review artifacts for patch docs and patch-adjacent planning work.

## Layout

Canonical per-patch findings stay at the root:

- `.sdlc/patches/.reviews/{patch-stem}-findings.md`
- `.sdlc/patches/.reviews/{patch-stem}-findings.json`

Plan reviews (reviews of implementation plans derived from a patch, broader execution reviews not part of the canonical findings pair) live under:

- `.sdlc/patches/.reviews/plan-reviews/`

When the reviewing model or harness is known, include it in the plan-review filename:

- `.sdlc/patches/.reviews/plan-reviews/codex-0001-openai-responses-api-support-plan-review.md`
- `.sdlc/patches/.reviews/plan-reviews/codex-0001-openai-responses-api-support-plan-review.json`

Cross-cutting syntheses (multi-patch reviews, baseline crosswalks, security sweeps spanning several patches) live under:

- `.sdlc/patches/.reviews/syntheses/`

## Canonical Files

Each formally reviewed patch keeps exactly two canonical files:

- `{stem}-findings.md` — markdown findings for humans
- `{stem}-findings.json` — structured findings for tooling and tracking

The `stem` matches the patch filename without the `.md` extension (e.g. `0001-openai-responses-api-support`).

## Findings Schema

Each finding in the JSON file should include:

- `id` — short identifier (e.g., `F1`, `F2`)
- `summary` — one-line description
- `reviewer` — reviewer role or name (e.g., `Security`, `Implementation Readiness`)
- `severity` — `blocking` | `significant` | `advisory`
- `affected_sections` — sections of the patch doc the finding touches
- `detail` — full explanation
- `recommendation` — concrete next action
- `disposition` — `accepted` | `rejected` | `deferred` | `null` until resolved
- `disposition_rationale` — why the disposition was chosen

The markdown file mirrors this structure for human reading and includes a top-line summary block:

```
total_findings, blocking, significant, advisory, top_line
```

## When to Write a Review

Patches do not require formal review by default — most are small enough that the patch doc itself is the authorization and the PR is the review surface. Write a `.reviews` artifact when:

- A reviewer (security, architecture, integration) raises findings the patch author needs to address before status flips to `done`
- A patch touches a sensitive surface (auth, capture/redaction, storage migrations) and warrants a recorded review
- A patch is being promoted to a feature spec or ADR and the findings inform that promotion

For trivial fixes, the commit message and PR description are sufficient — no `.reviews` artifact needed.
