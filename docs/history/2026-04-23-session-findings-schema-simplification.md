# Session Log: 2026-04-23

## Agent: assistant (Claude Code)

## Planned
- Peer-review ADR-0014 on harness base strategy
- Land the review as a `.reviews` findings file
- Evaluate whether the existing `{stem}-findings.json` sidecar convention was pulling its weight, and simplify it if not

## What Was Done

### 1. ADR-0014 peer review
- Reviewed `docs/adr/0014-harness-base-strategy.md` against the ADR template, scoring rigor, and internal consistency.
- Produced 10 findings: 2 blocking, 5 significant, 3 advisory. Key items:
  - **F1** — arithmetic errors in four of six weighted totals (O2, O3, O4, O5)
  - **F2** — option numbering skipped from O5 to O7
  - **F3** — ADR-0014 missing from the ADR index in `docs/adr/README.md`
  - **F4** — scoring convention (hundredths / 1–10) diverged from the README template (1–5 weight)
  - **F5** — O1's D2 score of 7 compressed the gap between "must build" and "best-in-class exists," and the sensitivity check showed the decision could flip to O7 if O1 D2 dropped to 4
  - **F6** — O7's D2=5 inconsistent with the "orchestration-oblivious" prose
  - **F7** — Confirmation criteria required observing orchestration work the ADR itself said was out of scope
  - **F8–F10** — missing `related:` frontmatter, open questions duplicated from EXP-0010, "Why O1 beats O7" leaning on numerical margin rather than strategic claim
- Saved to `docs/adr/.reviews/0014-harness-base-strategy-findings.md`.
- Findings were subsequently processed and all 10 accepted by a parallel OpenCode session (see `2026-04-22-session-analysis-adr-0014.md`). Final ADR-0014 accepted with the revised scoring.

### 2. Evaluation of the findings.json sidecar
- Investigated whether `{stem}-findings.json` sidecars were actually being used:
  - Six JSON files existed (FEAT-0008/9/10/11/12, PATCH-0008).
  - Every `disposition` field across all six was `null` — never filled in.
  - No Go code, script, Makefile target, or CI job consumed them. The "structured findings for tooling" the schema anticipated did not exist.
- Conclusion: the JSON sidecar duplicated the `.md` content, added a second file to keep in sync, and carried nothing the `.md` couldn't. A disposition table at the bottom of the `.md` is strictly simpler and keeps findings + resolutions in one place.

### 3. Schema change: drop JSON sidecar, adopt disposition table
Updated six docs to replace the sidecar convention with a single-file findings convention (disposition table at bottom):
- `docs/adr/.reviews/README.md`
- `docs/features/.reviews/README.md`
- `docs/patches/.reviews/README.md`
- `docs/explorations/README.md`
- `.agents/process.md` (line 77)
- `docs/agents.md` (line 22)

### 4. Migration of existing findings files
- Confirmed all dispositions across the 6 existing JSON files were `null`; nothing lost by deletion.
- Appended a Dispositions table (one row per finding, all marked `—`) to each of:
  - `docs/features/.reviews/0008-bff-server-findings.md`
  - `docs/features/.reviews/0009-terminal-harness-findings.md`
  - `docs/features/.reviews/0010-enterprise-auth-findings.md`
  - `docs/features/.reviews/0011-knowledge-integration-findings.md`
  - `docs/features/.reviews/0012-skills-and-agent-teams-findings.md`
  - `docs/patches/.reviews/0008-moonshot-provider-adapter-findings.md`
- For PATCH-0008, also stripped the six inline `**Disposition:** null` lines, since the table now owns dispositions canonically.
- Deleted the six `-findings.json` files.

### 5. User-authored addition to the ADR review README
- During the session, the user added a "Processing Review Findings" section to `docs/adr/.reviews/README.md` (8-step workflow: read → triage → edit ADR → update dispositions → mirror in ADR → recalc scoring → revisit decision → commit). That section formalizes the author-side workflow for handling findings and pairs naturally with the new single-file disposition-table convention.

## Files Created
- `docs/adr/.reviews/0014-harness-base-strategy-findings.md`
- `docs/history/2026-04-23-session-findings-schema-simplification.md` (this file)

## Files Modified
- `docs/adr/.reviews/README.md` — dropped JSON sidecar; added Dispositions section; user added Processing Review Findings workflow
- `docs/features/.reviews/README.md` — dropped JSON sidecar; added Dispositions section
- `docs/patches/.reviews/README.md` — dropped JSON sidecar; added Dispositions section
- `docs/explorations/README.md` — dropped `.json` review artifact reference
- `.agents/process.md` — updated "Review Artifact Placement" to single-file convention
- `docs/agents.md` — updated "Review Artifact Naming" to single-file convention
- `docs/features/.reviews/0008-bff-server-findings.md` (+ 4 siblings: 0009/0010/0011/0012) — appended Dispositions table
- `docs/patches/.reviews/0008-moonshot-provider-adapter-findings.md` — stripped inline `**Disposition:** null` lines; appended Dispositions table

## Files Deleted
- `docs/features/.reviews/0008-bff-server-findings.json`
- `docs/features/.reviews/0009-terminal-harness-findings.json`
- `docs/features/.reviews/0010-enterprise-auth-findings.json`
- `docs/features/.reviews/0011-knowledge-integration-findings.json`
- `docs/features/.reviews/0012-skills-and-agent-teams-findings.json`
- `docs/patches/.reviews/0008-moonshot-provider-adapter-findings.json`

## Decisions
- Review findings files are canonical in `.md` only. Dispositions live in a table at the bottom of the findings file, not in a sidecar JSON.
- Existing findings.json files with all-null dispositions were safe to delete; none had tracking state worth preserving.

## Intentionally Untouched
- `docs/history/*.md` files that reference the old `-findings.json` paths. Those are historical session logs describing what existed at the time; rewriting them would be revisionist.

## Issues / Open Questions
- The ADR review schema now mentions `affected_drivers`, `affected_options`, and `scoring_impact`, which the older feature/patch findings files don't use. Minor convention drift is acceptable — the ADR fields fit ADRs specifically.
- Commit strategy: this work is a mix of (a) ADR review output and (b) schema/process changes. Candidate split:
  - `ADR-0014: add peer review findings` for the findings file
  - `ADMIN: replace findings.json sidecar with disposition table` for the schema change + migration
  Up to the user to decide whether to split or bundle.

## What's Next
- User to commit the work (see split suggestion above).
- ADR-0014 is now `accepted` per the OpenCode session; downstream feature/WU work that depends on the harness-base decision can proceed.
