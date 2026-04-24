# Patch Reviews

This directory stores review artifacts for patch docs and patch-adjacent planning work.

## Layout

Canonical per-patch findings stay at the root:

- `docs/patches/.reviews/{patch-stem}-findings.md`

Plan reviews (reviews of implementation plans derived from a patch, broader execution reviews not part of the canonical findings file) live under:

- `docs/patches/.reviews/plan-reviews/`

When the reviewing model or harness is known, include it in the plan-review filename:

- `docs/patches/.reviews/plan-reviews/codex-0001-openai-responses-api-support-plan-review.md`

Cross-cutting syntheses (multi-patch reviews, baseline crosswalks, security sweeps spanning several patches) live under:

- `docs/patches/.reviews/syntheses/`

## Canonical File

Each formally reviewed patch keeps a single canonical file: `{stem}-findings.md`. Findings and their dispositions live in one document so they cannot drift. The `stem` matches the patch filename without the `.md` extension (e.g. `0001-openai-responses-api-support`).

## Findings Schema

Each finding in the markdown file should include:

- `id` — short identifier (e.g., `F1`, `F2`)
- `summary` — one-line description
- `reviewer` — reviewer role or name (e.g., `Security`, `Implementation Readiness`)
- `severity` — `blocking` | `significant` | `advisory`
- `affected_sections` — sections of the patch doc the finding touches
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
| F1 | blocking | accepted | Fixed in commit abc123 |
| F2 | advisory | rejected | Out of scope for this patch |
```

Disposition values are `accepted`, `rejected`, or `deferred`. Use `—` until resolved. Always include a rationale when the disposition is `rejected` or `deferred`; `accepted` rationales are optional but helpful.

## When to Write a Review

Patches do not require formal review by default — most are small enough that the patch doc itself is the authorization and the PR is the review surface. Write a `.reviews` artifact when:

- A reviewer (security, architecture, integration) raises findings the patch author needs to address before status flips to `done`
- A patch touches a sensitive surface (auth, capture/redaction, storage migrations) and warrants a recorded review
- A patch is being promoted to a feature spec or ADR and the findings inform that promotion

For trivial fixes, the commit message and PR description are sufficient — no `.reviews` artifact needed.
