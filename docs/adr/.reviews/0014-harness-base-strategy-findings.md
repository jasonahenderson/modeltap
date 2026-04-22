# ADR-0014 Findings

- ADR: `docs/adr/0014-harness-base-strategy.md`
- Review date: 2026-04-22
- Reviewer: peer review (Claude)
- total_findings: 10
- blocking: 2
- significant: 5
- advisory: 3
- top_line: The decision (O1 — continue modeltap harness) is sound and well-argued, but the ADR has concrete defects that must be fixed before it moves to `accepted`: arithmetic errors in four of six weighted totals, a skipped option number, and a stale ADR index. Several scoring calls also deserve scrutiny because they compress the real gap between "already built" and "must be built," and one of them may actually flip the ranking of O1 vs O7 if corrected.

## Findings

### F1 — Blocking

**Reviewer:** Scoring Integrity

**Affected drivers:** All (D1–D7)

**Affected options:** O2, O3, O4, O5

**Summary:** Four of six weighted totals in the scoring matrix are computed incorrectly.

**Detail:** Using the stated formula `weight × score/10` and the scores listed in the matrix (`docs/adr/0014-harness-base-strategy.md:44-53`), the computed totals diverge from the stated totals:

| Option | Stated | Computed | Δ |
|---|---|---|---|
| O1 | 0.805 | 0.805 | ✓ |
| O2 | 0.42 | 0.445 | +0.025 |
| O3 | 0.39 | 0.430 | +0.040 |
| O4 | 0.53 | 0.505 | −0.025 |
| O5 | 0.43 | 0.405 | −0.025 |
| O7 | 0.755 | 0.755 | ✓ |

The ordering is unchanged (O1 still wins, O7 still runner-up), so this does not invalidate the decision. But the matrix numbers must be correct before the ADR can be `accepted`.

**Scoring impact:** None on ranking. Corrects absolute totals for four options.

**Recommendation:** Recompute the four incorrect totals and consider adding a scratch check (spreadsheet or small script) so future ADRs don't ship with arithmetic drift.

### F2 — Blocking

**Reviewer:** Document Integrity

**Affected drivers:** None

**Affected options:** Numbering scheme as a whole

**Summary:** Option numbering skips O6. Options are labeled O1, O2, O3, O4, O5, **O7**.

**Detail:** `docs/adr/0014-harness-base-strategy.md:29-34` defines options O1 through O5 and then jumps to O7. The scoring matrix, justifications, and the "Why O1 beats O7" section all reference O7. If an option was dropped during drafting, the remaining options should be renumbered O1–O6 for consistency. Alternatively, restore the missing O6 if it was a genuine candidate. Leaving the gap is confusing to future readers who will assume an option was lost.

**Scoring impact:** None.

**Recommendation:** Renumber O7 → O6 throughout, or restore the intended O6 option.

### F3 — Significant

**Reviewer:** Document Integrity

**Affected drivers:** None

**Affected options:** N/A

**Summary:** The ADR index (`docs/adr/README.md`) does not list ADR-0014, and its "Current Architecture" paragraph still stops at ADR-0012.

**Detail:** `docs/adr/README.md:11-25` stops the index at ADR-0013. ADR-0014 must be added as a row with status `Proposed`. Separately, the "Current Architecture (Effective Decisions)" paragraph at `docs/adr/README.md:7` hasn't absorbed ADR-0013 (Bubbletea) or ADR-0014 yet. That drift is broader than this ADR but worth catching here.

**Scoring impact:** None on this ADR.

**Recommendation:** Add the ADR-0014 row to the index table, and plan a separate small edit to refresh the Current Architecture paragraph once the proposed ADRs stabilize.

### F4 — Significant

**Reviewer:** Template Conformance

**Affected drivers:** All

**Affected options:** N/A

**Summary:** The weight-scoring convention departs from the template without amending the template.

**Detail:** `docs/adr/README.md:66` specifies `Weighted criteria (1-5, 5 = critical)`, and ADR-0013 follows that convention. ADR-0014 uses hundredths summing to 1.00 with scores on a 1–10 scale. The new convention is arguably cleaner (forces zero-sum trade-offs and produces a 0–1 utility number), but if it becomes the house style it should replace the template; if it doesn't, ADR-0014 should use the existing convention. Two incompatible scoring idioms in the same ADR set will fragment the review process.

**Scoring impact:** None directly — scheme choice is orthogonal to the decision. But matters for long-term consistency.

**Recommendation:** Decide which convention is canonical; update `docs/adr/README.md` template section to match; and either keep or rewrite ADR-0014's drivers to match.

### F5 — Significant

**Reviewer:** Scoring Integrity

**Affected drivers:** D2 (Multi-agent orchestration), D7 (Feature richness today)

**Affected options:** O1

**Summary:** O1's scores on "must be built from scratch" drivers may be too high, and correcting them could flip O1 behind O7.

**Detail:** O1 scores D2=7 and D7=4 (`docs/adr/0014-harness-base-strategy.md:60,65`) despite the justification text explicitly saying "No multi-agent orchestration today" and "MCP only today. Memory, plugins, slash commands, and 43+ tools are missing." Meanwhile O4 (OpenHarness, best-in-class on both) scores D2=8 and D7=9. A one-point gap between "best-in-class already exists" and "must be built from scratch" on D2 compresses the matrix.

Quick sensitivity check: if O1's D2 drops to 4 (closer to what "must be built" typically scores), O1's total falls to 0.730, below O7's 0.755. That *would* flip the decision. The prose argues the D2 score is a bet on future orchestration-aware UI; if so, the ADR should say that explicitly rather than encoding the bet as a current-capability score.

**Scoring impact:** Potentially decision-flipping between O1 and O7 depending on how the future-capability bet is scored.

**Recommendation:** Either (a) lower O1's D2 to reflect today's capability and acknowledge that the decision depends on a forward commitment to build orchestration-aware UI, or (b) leave the score and add an explicit note that the D2 score reflects both current capability *and* the intention to close the gap as part of accepting this ADR.

### F6 — Significant

**Reviewer:** Scoring Integrity

**Affected drivers:** D2 (Multi-agent orchestration)

**Affected options:** O7

**Summary:** O7's D2 score of 5 looks high given the justification explicitly calls the harness "orchestration-oblivious."

**Detail:** `docs/adr/0014-harness-base-strategy.md:110` argues that under O7 "users must leave the terminal to a web dashboard or Slack bot to manage teams" and calls this a "UX regression." For a terminal-first product on a multi-agent orchestration driver, that reads closer to a 3 than a 5. Lowering this score widens O1's lead on the decisive driver and makes the "Why O1 beats O7" argument land on numbers that match the prose.

**Scoring impact:** Widens O1's lead over O7 (O1 stays 0.805, O7 drops from 0.755 to ~0.705).

**Recommendation:** Reduce O7's D2 score to 3 and update the justification accordingly.

### F7 — Significant

**Reviewer:** Scope Integrity

**Affected drivers:** None

**Affected options:** N/A (scope of the ADR itself)

**Summary:** The Confirmation criteria require observing orchestration work that the ADR itself says is out of scope.

**Detail:** The Confirmation section (`docs/adr/0014-harness-base-strategy.md:138`) requires "observing a server-orchestrated subagent team's progress and results" — but the same section notes that "Multi-agent orchestration itself is a separate BFF feature (to be planned under FEAT-0013 or a successor) and is not gated by this ADR." That's internally contradictory: the ADR can't confirm until a feature it doesn't gate is shipped.

**Scoring impact:** None on options.

**Recommendation:** Split the confirmation into two tiers: (a) this-ADR confirmation — harness connects to BFF, renders multi-session explorer, streams markdown, executes tools with permission prompts — and (b) future-feature confirmation — orchestration observation, tracked under FEAT-0013 or its successor.

### F8 — Advisory

**Reviewer:** Traceability

**Affected drivers:** None

**Affected options:** N/A

**Summary:** Frontmatter lacks a `related:` block that links the inputs to this decision.

**Detail:** The ADR references EXP-0010, FEAT-0009, and ADR-0013 throughout the prose but has no machine-readable `related:` field in frontmatter. EXP-0010's frontmatter (`docs/explorations/0010-harness-comparative-analysis.md:6-14`) demonstrates the shape. ADR-0013 also lacks this, so the omission is not unique — but ADR-0014 is a chance to start the convention in the ADR tier.

**Scoring impact:** None.

**Recommendation:** Add `related: [EXP-0010, FEAT-0009, ADR-0013]` (or the repo's preferred shape) to the frontmatter, and adopt this as the convention going forward.

### F9 — Advisory

**Reviewer:** Editorial

**Affected drivers:** None

**Affected options:** N/A

**Summary:** The Open Questions section largely duplicates the exploration's open questions.

**Detail:** Q1, Q2, Q4, Q5 in the ADR (`docs/adr/0014-harness-base-strategy.md:149-153`) are near-verbatim from EXP-0010's Open Questions (`docs/explorations/0010-harness-comparative-analysis.md:263-267`). ADR-tier open questions should be the ones the *decision itself* raises — e.g., "which OpenHarness subsystems port first?", "when does orchestration-aware UI land — v0.2.0 or later?", "how much of the forward bet in F5 must be materialized before this ADR can flip to `accepted`?". The exploration is the proper home for the upstream design-space questions.

**Scoring impact:** None.

**Recommendation:** Trim the duplicated questions and add decision-specific ones.

### F10 — Advisory

**Reviewer:** Editorial

**Affected drivers:** D2

**Affected options:** O1, O7

**Summary:** The "Why O1 beats O7" section leans on the numerical margin rather than the argument.

**Detail:** The prose (`docs/adr/0014-harness-base-strategy.md:117-124`) frames D2 as "the decisive difference" and cites the 0.805 vs 0.755 margin. That is only true at current scores; if F5 and F6 recommendations are applied, the margin changes materially. A stronger framing would be "O1 is the only option that keeps the terminal as the primary orchestration surface — everything else either forgoes orchestration altogether (O7) or requires architectural inversion to achieve it (O2–O5)." That argument holds independent of exact scores.

**Scoring impact:** None.

**Recommendation:** Rewrite the "Why O1 beats O7" argument to lean on the strategic claim rather than the numerical margin.

## Dispositions

| ID | Severity | Disposition | Rationale |
|----|----------|-------------|-----------|
| F1 | blocking | accepted | Recomputed all weighted totals using `weight × score/10`. Four totals corrected (O2: 0.445, O3: 0.430, O4: 0.505, O5: 0.405). |
| F2 | blocking | accepted | Renumbered O7 → O6 throughout ADR. |
| F3 | significant | accepted | Added ADR-0014 row to `docs/adr/README.md` index. |
| F4 | significant | accepted | ADR-0014 uses hundredths-weighted scoring (user requirement). Template convention updated in `docs/adr/README.md`. |
| F5 | significant | accepted | Lowered O1 D2 from 7 to 4 to reflect current gap. Added explicit note that the score combines current capability with a forward commitment, and that the strategic claim (not the margin) justifies O1 over O6. |
| F6 | significant | accepted | Lowered O6 D2 from 5 to 3 to match the "orchestration-oblivious" description. |
| F7 | significant | accepted | Split Confirmation into two tiers: this-ADR confirmation and future-feature confirmation tracked under FEAT-0013. |
| F8 | advisory | accepted | Added `related:` frontmatter linking EXP-0010, FEAT-0009, FEAT-0013, ADR-0013, and the findings file. |
| F9 | advisory | accepted | Replaced exploration-duplicate open questions with decision-specific ones (port order, acceptance criteria, release timing, spec vs strategy). |
| F10 | advisory | accepted | Rewrote "Why O1 beats O6" to lead with the strategic claim (terminal as universal orchestration surface) rather than the numerical margin. |

Dispositions: one of `accepted`, `rejected`, `deferred`. Leave as `—` until resolved. Add a rationale cell whenever a disposition is set (especially for `rejected` / `deferred`).
