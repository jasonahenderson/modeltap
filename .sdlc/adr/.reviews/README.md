# ADR Reviews

This directory stores review artifacts for architectural decision records.

## Layout

Canonical per-ADR findings stay at the root:

- `.sdlc/adr/.reviews/{adr-stem}-findings.md`

The `stem` matches the ADR filename without the `.md` extension (e.g. `0006-multi-provider-support`).

Plan reviews — reviews of implementation plans derived from an ADR or narrower execution reviews tied to a single ADR — live under:

- `.sdlc/adr/.reviews/plan-reviews/`

When the reviewing model or harness is known, include it in the plan-review filename:

- `.sdlc/adr/.reviews/plan-reviews/codex-0006-provider-formatting-plan-review.md`

Cross-cutting syntheses (multi-ADR reviews, baseline crosswalks, supersession analyses) live under:

- `.sdlc/adr/.reviews/syntheses/`

## Canonical File

Each formally reviewed ADR keeps a single canonical file: `{stem}-findings.md`. Findings and their dispositions live in one document so they cannot drift.

## Findings Schema

Each finding in the markdown file should include:

- `id` — short identifier (e.g. `F1`)
- `summary` — one-line description
- `reviewer` — reviewer role (e.g. `Architecture Conformance`, `Implementation Readiness`, `Security`)
- `severity` — `blocking` | `significant` | `advisory`
- `affected_drivers` — drivers from the ADR's decision-driver list that the finding touches
- `affected_options` — options the finding affects scoring or evaluation of
- `scoring_impact` — narrative description of how the finding shifts scoring (or `"No scoring impact"`)
- `detail` — full explanation
- `recommendation` — concrete next action

A top-line summary block appears near the top of the file:

```
total_findings, blocking, significant, advisory, top_line
```

## Dispositions

Each findings file ends with a Dispositions table. One row per finding, tracking how the author or decision-maker resolved it:

```markdown
## Dispositions

| ID | Severity | Disposition | Rationale |
|----|----------|-------------|-----------|
| F1 | blocking | accepted | Math corrected in commit abc123 |
| F2 | significant | rejected | Scoring is intentional — see §"Why O1 beats O7" |
| F3 | advisory | deferred | Tracked under FEAT-NNNN |
```

Disposition values are `accepted`, `rejected`, or `deferred`. Use `—` until resolved. Always include a rationale when the disposition is `rejected` or `deferred`; `accepted` rationales are optional but helpful.

## When to Write a Review

Not every ADR requires a formal `.reviews` artifact. Write one when:

- Multiple reviewers (architecture, implementation readiness, security) raise findings against a proposed ADR before it can be accepted
- An ADR is being superseded and the supersession analysis is non-trivial
- An ADR's score adjustments deserve a recorded paper trail
- A cross-ADR baseline review is being conducted (use `syntheses/`)

For straightforward ADRs that pass review without findings, the commit history and the ADR itself are sufficient — no `.reviews` artifact needed.

## Processing Review Findings

When findings are received for a proposed ADR, the author processes them in this order:

1. **Read the findings file** (`.sdlc/adr/.reviews/{stem}-findings.md`) and understand every finding's severity, detail, and recommendation.
2. **Triage each finding** into one of three dispositions:
   - **`accepted`** — the finding is valid and the ADR is revised to address it
   - **`rejected`** — the finding is invalid or the recommendation is declined; rationale must be recorded
   - **`deferred`** — the finding is valid but out of scope for this ADR; tracked under a feature, patch, or future ADR
3. **Edit the ADR** to incorporate all `accepted` findings. This may involve:
   - Correcting arithmetic, renumbering, or factual errors
   - Adjusting scores and rewriting justifications
   - Adding or removing sections
   - Updating frontmatter (e.g., `related:`)
4. **Update the Dispositions table** in the findings file with a rationale for each finding. For `accepted`, link to the commit that fixed it. For `rejected`, explain why. For `deferred`, name the tracking artifact.
5. **Update the ADR's own Review Findings section** (if present) to mirror the dispositions table, so readers of the ADR can see the resolution without opening the findings file.
6. **Recalculate the scoring matrix** if any scores changed. Use `weight × score/10` and verify totals with a scratch check (spreadsheet or small script).
7. **Review the decision outcome** — if scoring changes flipped the ranking or materially narrowed the margin, revisit the "Why X beats Y" argument to ensure it still holds on strategic grounds, not just arithmetic.
8. **Commit** the ADR revision, the updated findings file, and any cross-references (e.g., README index updates) together with a clear commit message naming the ADR.

**Guiding principle:** `accepted` findings are incorporated into the ADR; `rejected` findings are explained in the findings file; `deferred` findings are handed off to the appropriate downstream artifact. The findings file is the canonical record of the review; the ADR is the canonical record of the decision.
