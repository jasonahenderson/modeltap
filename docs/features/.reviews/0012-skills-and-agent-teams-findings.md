# FEAT-0012 Findings

- Feature: `docs/features/0012-skills-and-agent-teams.md`
- Review date: 2026-04-14
- Reviewer: peer review
- total_findings: 3
- blocking: 2
- significant: 1
- advisory: 0
- top_line: The document contains valuable ideas, but it is not ready to accept as one feature because it combines a small harness-side skill system with a much larger multi-agent orchestration system that still has unresolved safety and flow semantics.

## Findings

### F1 — Blocking

**Reviewer:** Feature Scoping

**Affected sections:** Solution, Skills, Agent Teams, Success Criteria

**Summary:** The spec combines two materially different features that should not share one acceptance gate.

**Detail:** FEAT-0012 says skills are harness-side and require no new server capabilities, while agent teams require BFF-coordinated multi-agent orchestration, shared context, concurrency, cost controls, and policy enforcement (`docs/features/0012-skills-and-agent-teams.md:29-35`, `:38-86`, `:88-186`, `:423-445`). Those are not one unit of behavior or risk. As written, a lightweight skill implementation and a safe agent-team implementation either block each other or get accepted together without equivalent maturity.

**Recommendation:** Split FEAT-0012 into at least two features: one for skills and one for agent teams, or phase the document explicitly so only one capability is acceptance-critical at a time.

### F2 — Blocking

**Reviewer:** Safety and Execution Model

**Affected sections:** Team Execution Flow, Tool Execution and Permissions, Agent Context Sharing, Open Questions, Success Criteria

**Summary:** The feature assumes safe multi-agent execution before defining failure, conflict, and write-serialization behavior.

**Detail:** The success criteria require concurrent agent execution, scoped approvals, shared file-system effects, and policy-respecting total cost caps (`docs/features/0012-skills-and-agent-teams.md:230-286`, `:433-445`). But the open questions still leave unresolved what happens when a writing agent fails, when two writers conflict, and whether the BFF must serialize writes (`:454-460`). Those are core safety semantics, not follow-up polish. Without them, the feature cannot serve as implementation authorization for an agent-team system that mutates local files.

**Recommendation:** Define the minimal safe execution rules before acceptance: write serialization policy, retry/fail behavior, plan-scope enforcement, and what exactly pauses or aborts the team when a sub-agent goes off-plan or fails.

### F3 — Significant

**Reviewer:** Dependency Clarity

**Affected sections:** Solution, Skills, CLI Integration, Success Criteria

**Summary:** The feature understates how much skills depend on BFF and harness protocol changes.

**Detail:** The spec says skills are harness-side and need no new server capabilities (`docs/features/0012-skills-and-agent-teams.md:31-35`), but even the skill behavior described here requires per-turn model overrides, narrowed tool availability, skill discovery, and capture semantics that FEAT-0008 and FEAT-0009 do not yet formalize (`:76-86`, `:288-305`, `:427-432`). That does not mean skills are a bad idea; it means their implementation dependency surface is wider than the feature currently claims.

**Recommendation:** Either narrow skills to pure prompt aliases in the first phase, or explicitly add the required protocol and session-state changes to the dependency story so the feature remains honest about scope.
