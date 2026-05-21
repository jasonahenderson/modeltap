# FEAT-0018 Findings (Architect + SRE pass)

- Feature: `.sdlc/features/0018-context-planner-and-project-rules.md`
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- total_findings: 10
- blocking: 0
- significant: 6
- advisory: 4
- top_line: The planner's input categories and provenance model are right-sized, but the spec leaves the determinism-critical contracts open: rule precedence, plan immutability across pipeline stages, repo-snapshot point, budget overflow choice, and selection latency at scale. Context plans are reused across `prompt_plan -> model_call -> tool_loop` and into repair turns, so any of these gaps becomes a non-deterministic-prompt or stale-context bug under load.

## Findings

### A1 — Significant

**Reviewer:** Architecture / Rule Precedence

**Affected sections:** Key Capabilities → Project Rule Discovery; Open Questions

**Summary:** "Deterministic precedence and conflict behavior" is required but not specified.

**Detail:** §Project Rule Discovery:0042-0053 lists 7 rule sources (`MODELTAP.md`, `.modeltap/`, `AGENTS.md`, `CLAUDE.md`, `.modeltap.yaml`, user/global config, team/server policy) and asserts the planner must define deterministic precedence — without giving one. Open Question 1 explicitly defers it. Without precedence, two valid sources can carry conflicting rules (e.g. ignore-paths in `.modeltap.yaml` vs `AGENTS.md`) and the planner has no defined resolution.

**Recommendation:** Pin a default precedence in this spec (e.g. server > team > project (`.modeltap.yaml`) > project (`MODELTAP.md`/`.modeltap/`) > project (`CLAUDE.md`/`AGENTS.md`) > user > global). Note that conflicts produce a recorded warning in the context plan. Defer the policy-language ADR for richer cases.

**Disposition:** accepted

---

### A2 — Significant

**Reviewer:** Architecture / Plan Immutability

**Affected sections:** Key Capabilities → Repository-Aware Selection; Budgeting

**Summary:** Whether the context plan is frozen across pipeline stages or recomputed is unspecified.

**Detail:** A run produces a context plan during `context_plan`. That plan feeds `prompt_plan`, then `model_call`, then `tool_loop` (which may read more files and trigger a re-plan), then `validation`, then potentially repair turns that re-enter `model_call`. If files change between stages, does the prompt see the *frozen* selected text, or *recomputed* current text? FEAT-0019 repair turns explicitly need new context but the freeze rule is not stated. Affects determinism, replay, and FEAT-0020 patch-evidence accuracy.

**Recommendation:** State that the context plan is frozen for a single `model_call`, with an explicit re-plan event before any subsequent `model_call` in the same run. The frozen plan and its content snapshot are persisted as a context artifact (cross-link to FEAT-0020).

**Disposition:** accepted

---

### A3 — Significant

**Reviewer:** Architecture / Budget Overflow Behavior

**Affected sections:** Key Capabilities → Budgeting; Open Questions

**Summary:** Budget overflow chooses between summarize/trim/reject without per-category rules.

**Detail:** §Budgeting:0094-0095 says the planner "may summarize, trim, or reject oversized context before model dispatch." Three different behaviors with very different downstream effects: summarization changes content fidelity, trimming drops material, rejection halts the run. Open Question 4 ("what context categories are pinned and what categories may be trimmed") confirms the gap. Without the contract, identical inputs can produce very different prompts.

**Recommendation:** Pin per-category overflow defaults: project rules and user attachments are *pinned* (overflow rejects the run with a budget-exceeded reason); transcript/history is *summarized*; selected files are *trimmed* (least-recently-touched first); memory is *trimmed* (lowest-relevance first). Make these configurable and recorded in the plan artifact.

**Disposition:** accepted

---

### A4 — Significant

**Reviewer:** Architecture / Repo Snapshot Point

**Affected sections:** Key Capabilities → Repository-Aware Selection

**Summary:** Snapshot semantics for dirty working tree and concurrent edits are unspecified.

**Detail:** §Repository-Aware Selection:0058-0066 reads files, imports, "recent git changes", and "files already read in the run." With a dirty working tree (unstaged edits), `git log` and the filesystem disagree. With concurrent agent runs, a sibling run can mutate files between `context_plan` and `prompt_plan`. The plan does not commit to a snapshot point (HEAD, working-tree, "as-of plan time"). FEAT-0020 §Patch Evidence depends on knowing what the run "saw" at start.

**Recommendation:** Commit to working-tree snapshot at the moment of `context_plan`, including dirty changes. Record the snapshot fingerprint (commit + dirty-file digest) on the context-plan artifact. Re-plan before any subsequent `model_call` re-checks the snapshot.

**Disposition:** accepted

---

### A5 — Advisory

**Reviewer:** Architecture / Disclosure Flag Scope

**Affected sections:** Key Capabilities → Budgeting

**Summary:** "User-controlled debug flag" for prompt-content disclosure has no scope.

**Detail:** §Budgeting:0100-0102 says a user-controlled debug flag may permit prompt disclosure to the harness, off by default and recorded as an artifact. Scope is unspecified: is it per-run, per-session, per-user, or per-project? Multi-user team contexts make this material — one user's debug flag should not expose another user's prompt.

**Recommendation:** Scope the flag per-run, requested at run start; persist it as a setting on the run and the disclosure-event artifact. Team policy may forbid the flag.

**Disposition:** accepted

---

### A6 — Advisory

**Reviewer:** Architecture / Provenance Vocabulary

**Affected sections:** Key Capabilities → Context Provenance

**Summary:** Provenance enumeration is closed-set; extensions and synthesis sources have no slot.

**Detail:** §Context Provenance:0070-0080 lists 8 provenance categories. FEAT-0022 introduces workflow extensions and routing roles that may inject context (e.g. a `reviewer` role pulling extra files); a sibling/synthesis run (FEAT-0015) may also feed context to a parent. None of those map onto the 8 categories.

**Recommendation:** Add `extension` and `parent_run` provenance categories, or make the list explicitly extensible with a structured `source` field that subsumes the 8 names.

**Disposition:** accepted

---

### S1 — Significant

**Reviewer:** SRE / Selection Latency at Scale

**Affected sections:** Key Capabilities → Repository-Aware Selection; Configuration

**Summary:** No deadline or caching contract for context-plan computation.

**Detail:** §Repository-Aware Selection:0058-0066 lists scans (imports, sibling files, recent git changes, test associations) that scale with repo size. A medium monorepo (100k+ files) makes a naive scan multi-second; a large monorepo unworkable. §Configuration:0119-0127 names ignored paths but does not address: maximum context-plan duration, caching of repo-map facts, incremental updates on file change, or warm-vs-cold first-run cost.

**Recommendation:** Commit to: a default `context_plan` deadline (e.g. 10s) that on overflow proceeds with the partial plan and records `partial_plan` on the artifact; a harness-side repo-map cache invalidated by file mtime; defer rich incremental indexing to EXP-0012's followup feature.

**Disposition:** accepted

---

### S2 — Significant

**Reviewer:** SRE / Memory Subsystem Degradation

**Affected sections:** Key Capabilities → Budgeting; Non-Goals

**Summary:** No fallback when the memory subsystem (FEAT-0022) is unavailable or degraded.

**Detail:** §Budgeting:0086-0092 lists memory as a budget category. FEAT-0022 owns memory retrieval. If the memory layer is degraded (slow, partial, errored), the planner has no defined fallback: skip the category silently, fail-open with a warning, or fail-closed with a planning error. For implementation runs, memory may carry compliance-relevant constraints whose silent omission is a correctness risk.

**Recommendation:** Default to "skip with warning" and mark the context plan with `memory_unavailable`; for implementation/devops workflows, escalate to "pause with `waiting_user`" so the user can choose to proceed without memory or wait for the subsystem.

**Disposition:** accepted

---

### S3 — Significant

**Reviewer:** SRE / Tokenizer Mismatch

**Affected sections:** Key Capabilities → Budgeting

**Summary:** Token budgets are model-specific; the planner's tokenizer choice and mismatch behavior are unstated.

**Detail:** §Budgeting:0085 names token budgets by category. Token counts depend on the model's tokenizer; modeltap supports multiple providers. If the planner uses a generic estimator, budgets are conservatively wrong; if it uses the target model's tokenizer, routing changes (FEAT-0022) that swap models mid-plan invalidate budgets.

**Recommendation:** Commit to per-model tokenizer estimation when available; fall back to a conservative provider-class estimator otherwise. Re-budget on any routing-driven model change. Record the tokenizer used on the plan artifact.

**Disposition:** accepted

---

### S4 — Advisory

**Reviewer:** SRE / Rule File Size

**Affected sections:** Key Capabilities → Project Rule Discovery; Configuration

**Summary:** No bound on project-rule file size.

**Detail:** Real `CLAUDE.md` and `AGENTS.md` files grow unboundedly over time. With 7 source types ingested in full, the rules section can dominate the prompt for a small implementation task. §Configuration:0119-0127 names "additional project rule filenames" but no max size or summarization policy.

**Recommendation:** Cap each rule source at a configurable byte budget (default ~32KB); over-cap rules are summarized with a recorded warning. Pinned status from A3 still applies — overflow of pinned categories rejects the run.

**Disposition:** accepted

---

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| A1 | accepted | Added default rule precedence, non-overridable upper-layer handling, and conflict warnings. |
| A2 | accepted | Froze context plans for one `model_call`, required explicit re-plan/reuse events, and persisted snapshot metadata. |
| A3 | accepted | Added per-category overflow defaults for pinned, summarized, and trimmed context categories. |
| A4 | accepted | Defined working-tree snapshot at `context_plan` with commit plus dirty-file digest fingerprint. |
| A5 | accepted | Scoped prompt disclosure debug flag per run, persisted it, and allowed team/server policy to forbid it. |
| A6 | accepted | Added `extension` and `parent_run`/synthesis provenance plus extensible source-field language. |
| S1 | accepted | Added 10s default context-plan deadline, partial-plan behavior, and repo-map cache guidance. |
| S2 | accepted | Added memory-degraded fallback, `memory_unavailable`, and stricter implementation/devops waiting behavior. |
| S3 | accepted | Added per-model tokenizer estimation, conservative fallback estimator, re-budgeting on model change, and tokenizer recording. |
| S4 | accepted | Added configurable per-rule-source byte cap with summarization warning and pinned overflow behavior. |
