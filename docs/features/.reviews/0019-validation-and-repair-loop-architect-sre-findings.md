# FEAT-0019 Findings (Architect + SRE pass)

- Feature: `docs/features/0019-validation-and-repair-loop.md`
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- total_findings: 11
- blocking: 0
- significant: 7
- advisory: 4
- top_line: Validation as structured run evidence and repair turns indexed by `check_id` are the right primitives. The spec underspecifies who proposes the plan, how risky checks are classified, what counts as a "different" repair attempt vs the same fix re-tried, and what happens when checks themselves error rather than fail. Repair-loop cost (multiple `model_call`s × multiple validations) also has no defined budget — the run can spiral against FEAT-0015's unenforced cost envelope.

## Findings

### A1 — Significant

**Reviewer:** Architecture / Plan Authority

**Affected sections:** Key Capabilities → Validation Plan; Check Execution

**Summary:** Plan authorship is split between BFF and harness without a defined boundary.

**Detail:** §Validation Plan:0037-0049 says "the system proposes a validation plan." Plan inputs include language/toolchain (harness-discovered), workflow type (BFF-known), prior failures (BFF-stored), and changed files (harness-or-BFF, depending on FEAT-0020 patch-evidence ownership). §Check Execution:0050-0064 then says "the harness executes checks through the normal tool/runtime policy." Plan composition is BFF logic; execution is harness; discovery is harness; persistence is BFF. The handoffs are not pinned.

**Recommendation:** State that the BFF composes the validation plan from facts the harness reports (toolchain detection, changed-file list, language detection). The plan is recorded as an artifact before any check runs. The harness executes the plan and reports per-check results. Cross-reference FEAT-0016 §BFF Responsibilities for the same authority pattern.

**Disposition:** accepted

---

### A2 — Significant

**Reviewer:** Architecture / Risk Classification

**Affected sections:** Key Capabilities → Check Execution

**Summary:** "Expensive or risky checks may require approval" with no classification rule.

**Detail:** §Check Execution:0064 introduces approval gating for risky checks but does not define risky. FEAT-0021 §Audit Trail:0091 names "dynamic risk classification" with the same gap (FEAT-0021 architect+SRE A6). Without a classification source-of-truth, the same `go test ./...` may auto-run on one project and require approval on another based on undefined heuristics.

**Recommendation:** Inherit the classification mechanism from FEAT-0021 explicitly: validation checks reuse the policy-grade tool runtime's risk classification rather than defining its own. Pin the default: shell commands matching the project's known toolchain auto-run; novel commands require approval.

**Disposition:** accepted

---

### A3 — Significant

**Reviewer:** Architecture / Repair-Attempt Identity

**Affected sections:** Key Capabilities → Repair Attempts

**Summary:** "Repeated failed fixes" detection requires defining when two attempts are "the same."

**Detail:** §Repair Attempts:0080-0082 says repair attempts reference the failing `check_id` so repeated failed fixes can be detected. But the model can attempt N different fixes against the same `check_id`; each is a distinct attempt. The detection that *matters* is "this fix is structurally similar to one that already failed" — diff-similarity, edit-set overlap, or identical file targets. The current contract only catches "same `check_id` failed again," not loop detection across distinct attempted fixes.

**Recommendation:** Track per-attempt edit-set fingerprints (set of (file, line-range) tuples) and flag repeated fixes when fingerprints overlap above a threshold. Failed-fingerprint history is fed into the next repair turn explicitly.

**Disposition:** accepted

---

### A4 — Significant

**Reviewer:** Architecture / Repair-Loop Termination

**Affected sections:** Key Capabilities → Repair Attempts; Open Questions

**Summary:** No default repair-attempt limit; loop termination is ambiguous.

**Detail:** §Repair Attempts:0082 says the run "stops or asks the user when it hits configured repair limits." Open Question 4 asks for a default. Without a default, loops fall back to FEAT-0015's umbrella budgets, which themselves have no stop-behavior committed (FEAT-0015 architect+SRE S5). The contract for "stops or asks the user" is also ambiguous: which workflows ask, which workflows hard-stop?

**Recommendation:** Commit to a default of 3 repair attempts for `implementation`, 5 for `debug`, 1 for `docs` and `release`. On limit, transition to `waiting_user` with a structured "repair limit reached" reason and the failed-fingerprint history attached.

**Disposition:** accepted

---

### A5 — Advisory

**Reviewer:** Architecture / Pre-existing Failure Marking

**Affected sections:** Key Capabilities → Failure Summarization; Open Questions

**Summary:** Pre-existing failure classification has no contract.

**Detail:** §Failure Summarization:0075 says the BFF reports whether the failure "appears introduced, pre-existing, or inconclusive." Open Question 3 asks how known pre-existing failures should be marked. Without a contract, the model can spend repair attempts on failures that pre-date the run.

**Recommendation:** Run a baseline pass during `preflight` for mutating workflows (lightest-cost subset of the validation plan). Failures present in the baseline are recorded as `pre_existing` and excluded from repair-loop drive. Make the baseline pass configurable.

**Disposition:** accepted

---

### A6 — Advisory

**Reviewer:** Architecture / Plan Preview vs Execution Race

**Affected sections:** UI / CLI / API Integration

**Summary:** `/validate plan` previews can drift before `/validate` runs.

**Detail:** §UI / CLI / API Integration:0086-0094 names `/validate plan` and `/validate retry`. If files change between the preview and the execution (user edits, sibling run writes, or repair turn applies a fix), the executed plan differs from the preview. FEAT-0018 architect+SRE A4 raised the snapshot-point issue at the planning level; the same issue applies to validation.

**Recommendation:** Pin the validation plan to the snapshot fingerprint from FEAT-0018; if the working tree changes between preview and execution, the run re-plans and the preview is invalidated with a visible reason.

**Disposition:** accepted

---

### S1 — Significant

**Reviewer:** SRE / Per-class Timeouts

**Affected sections:** Configuration

**Summary:** A single `validation timeout` covers very different check classes.

**Detail:** §Configuration:0109 names "validation timeout" as a single value. Lints complete in seconds; integration tests run for minutes; e2e suites for tens of minutes. A single timeout either kills slow tests or wastes budget on hung lints. FEAT-0016 architect+SRE S1 raised stage-timeouts at the pipeline level; validation is the most acute case.

**Recommendation:** Tier the timeout: per-check default (30s), per-class default (lint 60s, unit 5m, integration 15m, e2e 30m), per-run cap on total validation wall-clock. Make all configurable.

**Disposition:** accepted

---

### S2 — Significant

**Reviewer:** SRE / Check Error vs Check Failure

**Affected sections:** Key Capabilities → Check Execution; Failure Summarization

**Summary:** A check that errors (couldn't run) is not distinguished from a check that fails (assertion failed).

**Detail:** §Check Execution:0050-0064 records exit status per check, but failure analysis (§Failure Summarization:0066-0075) does not separate "test runner crashed / dependency missing / compile error in test harness" from "test assertion failed." Repair logic for the two is very different: a run-error needs environmental fix or human intervention; a fail-result feeds normal repair.

**Recommendation:** Classify check outcomes as `pass | fail | error | timeout | skipped`. Only `fail` feeds the repair loop by default; `error` transitions the run to `waiting_user` with the error context.

**Disposition:** accepted

---

### S3 — Significant

**Reviewer:** SRE / Repair-Loop Cost Envelope

**Affected sections:** Key Capabilities → Repair Attempts; Configuration

**Summary:** Repair loops can amplify run cost without a defined envelope.

**Detail:** A 5-attempt repair loop with re-validation per attempt issues 5+ `model_call`s and 5+ validation passes. Each `model_call` may be expensive; each validation pass burns local time. §Configuration:0107 mentions `maximum repair attempts` but no cost-side ceiling. FEAT-0015 architect+SRE A5/S5 raised cost envelope at the umbrella; repair loops are the primary amplifier.

**Recommendation:** Add a per-loop cost ceiling (cost-of-repair-loop ≤ N × cost-of-initial-attempt, default 3×) and a wall-clock ceiling. On either, transition to `waiting_user` with a "repair loop budget exhausted" reason.

**Disposition:** accepted

---

### S4 — Advisory

**Reviewer:** SRE / Output Capture Bounds

**Affected sections:** Key Capabilities → Check Execution

**Summary:** stdout/stderr "references" have no defined size cap or truncation.

**Detail:** §Check Execution:0061-0062 says checks record "stdout/stderr references" but does not say what happens with verbose tests producing 100MB+ logs. FEAT-0020 §Configuration:0116 names a "maximum captured log size" but no truncation contract.

**Recommendation:** Default to keeping the last N MB of stdout/stderr (e.g. 4 MB each), prefer the tail (failures usually surface at end), and store a checksum of the full output. Cite FEAT-0020 for the artifact-side cap.

**Disposition:** accepted

---

### S5 — Advisory

**Reviewer:** SRE / Check Concurrency

**Affected sections:** Key Capabilities → Check Execution

**Summary:** Whether checks run serially or in parallel is unstated.

**Detail:** §Check Execution:0050-0064 does not commit to execution model. Some checks are independent (lint + format); others have ordering constraints (build then test). Naive parallelism causes flaky failures; naive serialism wastes wall-clock.

**Recommendation:** Default to serial execution within a class and parallel across classes (lint/format/typecheck in parallel, then build, then test). Document the dependency model and let projects override.

**Disposition:** accepted

---

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| A1 | accepted | Clarified BFF plan composition from harness facts, pre-check plan artifact recording, harness execution, and per-check reports. |
| A2 | accepted | Reused FEAT-0021 risk classification and pinned known-toolchain vs novel-command approval defaults. |
| A3 | accepted | Added edit-set fingerprints, overlap-based repeated-fix detection, and failed-fingerprint repair context. |
| A4 | accepted | Added default repair limits by workflow and `waiting_user` behavior on limit. |
| A5 | accepted | Added configurable baseline validation pass and `pre_existing` classification excluded from repair by default. |
| A6 | accepted | Tied `/validate plan` to FEAT-0018 snapshot fingerprint and invalidated previews on working-tree drift. |
| S1 | accepted | Added per-check, per-class, and total validation timeout controls with defaults. |
| S2 | accepted | Added `pass/fail/error/timeout/skipped` outcomes and made only `fail` feed repair by default. |
| S3 | accepted | Added repair-loop cost and wall-clock ceilings with `waiting_user` exhaustion behavior. |
| S4 | accepted | Added tail-first stdout/stderr cap behavior, checksum, and FEAT-0020 artifact-cap reference. |
| S5 | accepted | Added default check concurrency and dependency model with project override. |
