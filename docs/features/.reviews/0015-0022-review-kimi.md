# FEAT-0015 through FEAT-0022 — Review

**Reviewer:** Kimi  
**Date:** 2026-04-28  
**Scope:** Professional Harness Runtime umbrella (FEAT-0015) and its seven member features (FEAT-0016–0022).  
**Status:** All specs are `draft`.

---

## Verdict

These eight specs form a coherent, well-scoped feature family. The umbrella pattern works, the dependency graph is acyclic, and each child feature has testable success criteria. The specs are **nearly ready to move to `proposed`** after a small set of consistency fixes.

**Overall grade:** B+ (strong, needs polish before review).

---

## Completeness

### What is complete

- **Required sections present.** Every spec has Problem, Solution, Key Capabilities, UI/CLI/API Integration, Configuration, Non-Goals, Success Criteria, Relationship to ADRs, and Open Questions.
- **Umbrella coverage.** FEAT-0015 provides a clear relationship map, document placement guidance, and explicit non-goals that bound the series.
- **Cross-references.** Children point to FEAT-0015 as `parent`. FEAT-0015 lists all children in `related`. Non-goal cross-references are precise (e.g., FEAT-0016 → FEAT-0017 for background queue).
- **Success criteria are testable.** Numbered, concrete, and bounded.

### What is incomplete or missing

1. **No explicit `series-order` dependency enforcement.** The `series-order` field (1–7) implies a sequence, but `depends-on` frontmatter should also express the chain. Currently:
   - FEAT-0016 (order 1) has no `depends-on` FEAT-0015.
   - FEAT-0017 (order 2) depends on FEAT-0016.
   - FEAT-0018 (order 3) depends on FEAT-0016.
   - FEAT-0019 (order 4) depends on FEAT-0016 and FEAT-0018.
   - FEAT-0020 (order 5) depends on FEAT-0016 and FEAT-0019.
   - FEAT-0021 (order 6) depends on FEAT-0009, FEAT-0016, and FEAT-0017.
   - FEAT-0022 (order 7) depends on FEAT-0011, FEAT-0012, FEAT-0013, FEAT-0016, and FEAT-0020.

   This is mostly correct, but FEAT-0016 should probably declare `depends-on: [FEAT-0015]` to make the umbrella relationship explicit in the DAG, even if it is structurally implied by `parent`.

2. **Future ADRs are scattered.** FEAT-0015 through FEAT-0021 each call out one or more “Future ADRs” that should be written. The umbrella should consolidate these into a single list so the ADR writing work can be scheduled. At least three distinct future ADRs are implied:
   - Run ownership, lifecycle, attachment, checkpoint semantics (FEAT-0015, 0016, 0017)
   - Policy inheritance, sandbox/workspace boundaries, non-overridable policy (FEAT-0021)
   - Artifact storage, redaction, retention (FEAT-0020)
   - Memory promotion defaults, routing taxonomy, extension trust (FEAT-0022)
   - Validation artifact schema, repair-loop limits (FEAT-0019)
   - Project-rule precedence, prompt-layer ownership (FEAT-0018)

3. **PATCH listed in Feature Relationship Map.** FEAT-0015’s map includes “PATCH: Codegen Evaluation Harness” as rank 9. Per `docs/features/README.md`, patches are implementation-scoped and do not belong in a behavior-contract feature map. This item should be an exploration, a feature spec, or removed from the map.

---

## Consistency

### Frontmatter and taxonomy

4. **`promoted-from` semantics misused on child specs.** The `promoted-from` field on FEAT-0016–0022 says `FEAT-0015`. The README promotion path says “an exploration that firms up into a behavior-scoped capability should promote to a feature spec.” FEAT-0015 is already a feature spec, not an exploration. These children were *decomposed* from an umbrella, not promoted. Consider using `decomposed-from: FEAT-0015` or removing the field, since `parent` and `series-role: member` already capture the relationship.

5. **`depends-on`, `related`, `promoted-from` are undocumented in README frontmatter.** The README template lists only `feature`, `title`, `status`, `date`, `parent`, `series`, `series-role`, `series-order`, and `adr-constraints`. The extra fields are fine as conventions, but they should be documented in `docs/features/README.md` so future specs follow the same shape.

6. **Future dates.** Every spec is dated `2026-04-29`. If these were drafted today (2026-04-28), the date should probably be today or the date of final acceptance. This is minor but noticeable.

### Terminology

7. **Lifecycle state names are inconsistent between umbrella and FEAT-0016.**

   | FEAT-0015 (umbrella states) | FEAT-0016 (pipeline stages) |
   |------------------------------|-----------------------------|
   | `preflight`                  | `preflight`                 |
   | `context_planning`           | `context_plan`              |
   | `prompt_planning`            | `prompt_plan`               |
   | `running`                    | `model_call`, `tool_loop`   |
   | `validating`                 | *(not in pipeline)*         |
   | `checkpointed`               | `checkpoint`                |
   | `completed`                  | `completion`                |

   **Fix:** Align terminology. The umbrella should either:
   - Adopt FEAT-0016’s stage names and add `blocked`/`failed`/`cancelled` as terminal/meta states, or
   - Explicitly state that umbrella states are runtime states and FEAT-0016 stages are pipeline stages, and provide a mapping.

8. **`waiting_user` is undefined outside FEAT-0015.** FEAT-0015 splits blocked states into `waiting_permission` and `waiting_user`. FEAT-0017 defines `blocked` as “needs permission or user input” but does not distinguish the two. FEAT-0021 (policy) also does not mention `waiting_user`. Either define `waiting_user` in a child feature or merge it into `waiting_permission`.

9. **Workflow commands vs workflow types vs routing roles.** FEAT-0015 defines 8 workflow *types* (`exploration`, `feature`, `adr`, `release`, `implementation`, `debug`, `docs`, `devops`). FEAT-0022 defines routing *roles* (`context helper`, `implementation`, `validation summarizer`, `repair`, `reviewer`, `documentation`, `synthesizer`). The terms are distinct (workflows vs roles), but the spec should explicitly state that roles are orthogonal to workflows to avoid reader confusion.

### CLI surface

10. **`/run` vs `/runs` ambiguity.** FEAT-0016 introduces `/run` (singular) for the active run summary. FEAT-0015 and FEAT-0017 introduce `/runs` or `/jobs` for the run list. The difference is logically sound (singular = active, plural = list), but this should be stated explicitly in FEAT-0015 or FEAT-0016 so implementers do not collide the commands.

---

## Logic

11. **FEAT-0016 Open Question 3 contradicts FEAT-0015 Success Criterion 9.**
   - FEAT-0016 asks: “Should every chat turn be a run, or only workflow/codegen turns?”
   - FEAT-0015 SC 9 states: “Existing FEAT-0008/0009/0014 behavior remains compatible: normal attached chat still works as a foreground run.”

   The umbrella has already decided that normal chat *is* a foreground run. FEAT-0016 should not re-open this; it should state the assumption and focus on how to make simple chat a lightweight run.

12. **Background permission default is stated inconsistently.**
   - FEAT-0015 says: “The default for mutating operations should be pause, not silent mutation.” (under Background Permission Behavior)
   - FEAT-0017 says: “The default for mutating operations should be pause, not silent mutation.”
   - FEAT-0021 says: “Do not silently approve background writes by default.”

   This is consistent in intent, but the phrasing is slightly different. Not a blocker.

13. **FEAT-0017 Open Question 1 is a hard architectural question.** “Can background runs continue local tool execution if no harness/local executor is connected?” This question is fundamental to whether background runs are purely BFF-server-side or require a persistent local agent. The umbrella should at least state an assumption (e.g., “background runs require a connected harness for local tool execution” or “the BFF may provide a subset of server-safe tools for disconnected execution”). Leaving this entirely open in `draft` is risky for scheduling.

14. **FEAT-0022 depends on FEAT-0011, FEAT-0012, FEAT-0013, which are all `proposed`.** This is acceptable, but it means FEAT-0022 cannot be accepted until those three features are accepted or its `depends-on` is relaxed to “builds on knowledge layer and skills concepts” rather than specific feature IDs.

15. **FEAT-0015 Feature Relationship Map rank 1 is “ADR: Run Runtime Ownership and Semantics.”** This is an ADR that does not yet exist. Because ADRs are the highest-tier decision documents, the umbrella feature cannot be `accepted` until this ADR is at least `proposed` and ideally `accepted`. The review should note that the feature family is currently blocked on ADR drafting.

---

## Recommendations

### Must-fix before `proposed`
1. **Align lifecycle terminology** between FEAT-0015 and FEAT-0016 (item 7).
2. **Resolve the FEAT-0016 Open Question 3 contradiction** with FEAT-0015 SC 9 (item 11).
3. **Define or remove `waiting_user`** state (item 8).
4. **Remove the PATCH from FEAT-0015’s Feature Relationship Map** or reclassify it (item 3).
5. **Fix `promoted-from` on child specs** to reflect decomposition, not promotion (item 4).

### Should-fix before `proposed`
6. **Add a consolidated “Future ADRs” section to FEAT-0015** listing all ADRs implied by the child specs (item 2).
7. **Document `depends-on`, `related`, and `promoted-from` in `docs/features/README.md`** (item 5).
8. **Clarify `/run` vs `/runs` semantics** in CLI sections (item 10).
9. **Add an architectural assumption to FEAT-0015 or FEAT-0017** about disconnected harness execution (item 13).
10. **Update dates to current date** (item 6).

### Nice-to-have
11. **Add a dependency diagram** (Mermaid or ASCII) to FEAT-0015 showing the DAG across FEAT-0016–0022 and their external dependencies.
12. **Add a lightweight “acceptance test sketch”** to each feature — one or two sentences describing the simplest end-to-end demonstration of “done.”

---

## Dependency Graph (verified acyclic)

```
FEAT-0008 ──┐
FEAT-0009 ──┼──► FEAT-0016 ──► FEAT-0017 ──► FEAT-0021
FEAT-0014 ──┘        │             ▲
                     │             │
FEAT-0015 (umbrella) │             │
                     ▼             │
              FEAT-0018 ──► FEAT-0019 ──► FEAT-0020 ──► FEAT-0022
                     ▲                                    ▲
                     │                                    │
FEAT-0011 ──────────┘                                    │
FEAT-0012 ─────────────────────────────────────────────────┤
FEAT-0013 ─────────────────────────────────────────────────┘
```

---

## Bottom Line

The Professional Harness Runtime series is a solid, well-thought-out expansion of modeltap’s orchestration layer. The umbrella structure is the right call. Fix the lifecycle naming, the promotion-path taxonomy, and the open contradiction about chat-as-run, then move the family to `proposed`.

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| Completeness 1 | rejected | `parent` and `series` encode the umbrella relationship; adding `depends-on: FEAT-0015` to member features would make the umbrella look like an implementation prerequisite rather than a grouping contract. |
| Completeness 2 | accepted | Added a consolidated Future ADRs section to FEAT-0015 covering run ownership, prompt/rules, validation, artifacts, policy/workspace, and memory/routing/extension trust. |
| Completeness 3 | accepted | Removed the undrafted codegen evaluation patch from the behavior relationship map and noted it separately as a future supporting patch. |
| Consistency 4 | accepted | Removed `promoted-from: FEAT-0015` from FEAT-0016 through FEAT-0022. |
| Consistency 5 | accepted | Documented `depends-on`, `related`, `promoted-from`, and `decomposed-from` in `docs/features/README.md`. |
| Consistency 6 | rejected | The specs were drafted on 2026-04-29; no date change was needed during 2026-04-30 review processing. |
| Consistency 7 | accepted | Aligned lifecycle terminology by defining run status, pipeline stage, and attachment state axes in FEAT-0015 and using matching stage names in FEAT-0016. |
| Consistency 8 | accepted | Defined `waiting_user` in FEAT-0015 and clarified `blocked` as a UI grouping in FEAT-0017. |
| Consistency 9 | accepted | Clarified in FEAT-0022 that routing roles are orthogonal to workflow types. |
| Consistency 10 | accepted | Clarified `/run` as the currently attached run and `/runs`/`/jobs` as the list surface. |
| Logic 11 | accepted | Replaced FEAT-0016's chat-as-run open question with a lightweight-run representation question, preserving FEAT-0015's decision that normal chat can be represented as a foreground run. |
| Logic 12 | accepted | Existing background-write default language was already aligned; related policy text now uses the same foreground/background framing. |
| Logic 13 | accepted | Added the local-executor availability question to FEAT-0015 so the future ADR must address it. |
| Logic 14 | accepted | Moved FEAT-0011/0012/0013 from FEAT-0022 hard dependencies to `related` and added acceptance-gating/phasing language. |
| Logic 15 | accepted | Added the Future ADRs section to FEAT-0015 and kept the run-runtime ADR as the first relationship-map item. |
| Nice-to-have 11 | deferred | A dependency diagram is useful but not required for this review-processing pass. |
| Nice-to-have 12 | deferred | Acceptance-test sketches can be added when the draft features are revised toward `proposed`. |
