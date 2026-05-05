# FEAT-0022 Findings (Architect + SRE pass)

- Feature: `docs/features/0022-memory-routing-and-workflow-extensions.md`
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- total_findings: 12
- blocking: 0
- significant: 8
- advisory: 4
- top_line: The "use run artifacts as the bridge" framing is correct — it stops memory, routing, and extensions from inventing parallel state stores. Gaps are concentrated in three places: memory needs a precedence rule across scopes (and a separation rule for "durable" vs "ephemeral"), routing roles must be unambiguously distinguished from FEAT-0016 stages they overlap with, and the extension trust contract (hooks/skills/teams composing tools, prompts, and routing) is the largest single back-door into the policy model from FEAT-0021.

## Findings

### A1 — Significant

**Reviewer:** Architecture / Memory Scope Precedence

**Affected sections:** Key Capabilities → Active Memory Inspection

**Summary:** Five memory scopes (global, project, package, file, workflow) without conflict resolution.

**Detail:** §Active Memory Inspection:0066 names scopes `global, project, package, file, workflow`. Two memories with overlapping scopes can express conflicting guidance ("project: prefer X" vs "file: prefer Y"). The spec does not define which wins. FEAT-0018 architect+SRE A1 raised the same question for project rules; memory inherits the issue.

**Recommendation:** Pin specificity: file > package > project > workflow > global. Conflicts within a scope produce a recorded warning and present both. Reuse FEAT-0018's rule precedence as the model where applicable.

**Disposition:** accepted

---

### A2 — Significant

**Reviewer:** Architecture / Durable vs Ephemeral Separation

**Affected sections:** Key Capabilities → Memory Candidates

**Summary:** "BFF separates durable knowledge from ephemeral traces" with no separation criteria.

**Detail:** §Memory Candidates:0046-0057 says the BFF separates durable from ephemeral. The criteria are not given. Without criteria, automatic candidate generation is unverifiable: the same successful run could generate 0 or 50 candidates depending on implementation choices. Open Question 1 asks which candidates require approval but does not pin the candidate-generation rule.

**Recommendation:** State that candidate generation is rule-based, not heuristic: candidates are produced only from explicit triggers — accepted ADR/feature/release artifacts, validation commands the user explicitly approved, and user-flagged moments (`/remember`). Heuristic candidate generation is a future capability behind a config flag.

**Disposition:** accepted

---

### A3 — Significant

**Reviewer:** Architecture / Role vs Stage Distinction

**Affected sections:** Key Capabilities → Quality-Driven Routing; Cross-Feature Impact

**Summary:** Routing roles overlap with FEAT-0016 pipeline stages and the orthogonality claim is undersupported.

**Detail:** §Quality-Driven Routing:0070-0078 lists roles `context helper, implementation, validation summarizer, repair, reviewer, documentation, synthesizer`. Several map directly onto FEAT-0016 stages (`validation summarizer` is what `validation` does; `repair` is what repair turns do; `context helper` is what `context_plan` does). §0097-0100 asserts roles are "orthogonal" to workflows and gives the `debug` example, but the role-stage relationship is not addressed. Two systems with similar names that are "orthogonal" tend to drift; pinning the relationship now prevents that.

**Recommendation:** State that roles are *model selections within a stage*: `context helper` is the model selected for the `context_plan` stage; `validation summarizer` is the model selected for the BFF-side summarization step inside `validation`; etc. Roles do not introduce new stages; they specialize existing ones.

**Disposition:** accepted

---

### A4 — Significant

**Reviewer:** Architecture / Extension Trust Boundary

**Affected sections:** Key Capabilities → Workflow Extensions; Cross-Feature Impact; Configuration

**Summary:** Hooks, skills, teams, and slash commands compose tools/prompts/routing without a trust model.

**Detail:** §Workflow Extensions:0084-0095 says extensions "may narrow tools, set model preferences, define artifact requirements, or add validation behavior." That is enough authority to bypass FEAT-0021 policy if extensions are not trust-classified. §Configuration:0142 names "hook enablement and trust policy" without contract. FEAT-0021 architect+SRE A5 raised hooks at the policy level; FEAT-0022 is the natural home for the extension-side trust model.

**Recommendation:** Pin trust tiers: built-in extensions (full trust), workspace-local extensions (medium, can narrow but not widen tool surface), third-party extensions (low, must request capabilities and run inside FEAT-0021 policy). Untrusted extensions cannot widen tool surface, override policy decisions, or override workflow validation requirements. Cross-link to FEAT-0021 §Hooks and Extensions.

**Disposition:** accepted

---

### A5 — Advisory

**Reviewer:** Architecture / Memory Retrieval Ranking

**Affected sections:** Key Capabilities → Active Memory Inspection

**Summary:** Retrieval ranking method is unstated.

**Detail:** §Active Memory Inspection:0066 lists "relevance reason" as inspectable but does not say how relevance is computed: vector similarity, recency, scope match, manual tagging. FEAT-0018 budgets memory and depends on ranking to know what to drop first under overflow.

**Recommendation:** Default to scope-match-then-similarity-then-recency. Vector similarity uses ADR-0008's sqlite-vec. Record the ranking method on the active-memory artifact for debuggability.

**Disposition:** accepted

---

### A6 — Advisory

**Reviewer:** Architecture / Confidence/Age Availability

**Affected sections:** Key Capabilities → Active Memory Inspection

**Summary:** "Age and confidence when available" with no rule for *when*.

**Detail:** §Active Memory Inspection:0067 says age and confidence are shown "when available." Without a rule for when those fields are populated vs missing, the harness cannot consistently render memory items.

**Recommendation:** Age is always available (creation timestamp). Confidence is available when the source is one with a quantitative confidence (validation pass count, vector similarity score) and absent otherwise; absence is rendered as "—" rather than hidden.

**Disposition:** accepted

---

### S1 — Significant

**Reviewer:** SRE / Memory Storage Growth

**Affected sections:** Key Capabilities → Memory Candidates; Configuration

**Summary:** No retention or expiry default for memory items.

**Detail:** §Configuration:0138 names "memory scopes and retention" but the default is unstated. Long-running projects accumulate memory faster than they prune. Stale memory (pointing to renamed functions, removed files) is then injected into prompts and degrades context quality. There is no committed mechanism for memory staleness detection.

**Recommendation:** Default retention: durable memory has no automatic expiry but is subject to a staleness check (when a referenced file/symbol no longer exists, flag the memory as `stale`); ephemeral memory ages out after 30 days. The user can pin memories against expiry.

**Disposition:** accepted

---

### S2 — Significant

**Reviewer:** SRE / Routing Decision Latency

**Affected sections:** Key Capabilities → Quality-Driven Routing

**Summary:** Per-stage routing has no latency or fallback contract.

**Detail:** §Quality-Driven Routing:0070-0082 specifies routing decisions per role. If routing requires an inference step (BFF-side helper model) or a rules evaluation, the cost is added to every stage transition. No commitment to deadline, fallback (when routing helper is unavailable), or caching. A degraded routing service can stall every run.

**Recommendation:** State that routing decisions are computed from a fast deterministic policy by default (workflow + stage + run history → preferred model), with optional inference-driven routing as an opt-in feature; routing failures fall back to the configured default model and record `routing_fallback` on the decision.

**Disposition:** accepted

---

### S3 — Significant

**Reviewer:** SRE / Outcome Dataset Boundary

**Affected sections:** Key Capabilities → Quality-Driven Routing; Success Criteria

**Summary:** Outcome-based learning has no privacy or tenancy boundary.

**Detail:** §Success Criteria:0162 says "future routing improvements can be evaluated against stored run outcomes." With team/enterprise modeltap profiles, run outcomes from one tenant cannot inform routing for another tenant without consent. The spec is silent on dataset scope.

**Recommendation:** State that routing improvement is per-deployment by default. Cross-tenant aggregation is an opt-in feature with explicit consent and is out of scope for v1. Record dataset provenance on each routing decision.

**Disposition:** accepted

---

### S4 — Significant

**Reviewer:** SRE / Extension Resource Limits

**Affected sections:** Key Capabilities → Workflow Extensions

**Summary:** Extensions can run code (hooks) without resource bounds.

**Detail:** §Workflow Extensions:0086-0089 says hooks can warn, block, or enrich. Without CPU/memory/time bounds per hook, a buggy or adversarial hook can stall a run or exhaust the host. FEAT-0021 architect+SRE A5 raised the order/error contract; the resource contract is the SRE half.

**Recommendation:** Default per-hook deadline (e.g. 5s) and memory bound (e.g. 256MB); over-limit hook is reported as a hook error and the tool call proceeds under default policy. Workspace-local hooks can raise limits via config; third-party hooks cannot.

**Disposition:** accepted

---

### S5 — Advisory

**Reviewer:** SRE / Candidate Inbox Overload

**Affected sections:** Key Capabilities → Memory Candidates

**Summary:** A chatty run could produce hundreds of memory candidates.

**Detail:** Without a candidate cap or coalescing rule, the inbox overflows and the user disengages from approving any of them. A2 covers the generation side; this is the inbox-shape side.

**Recommendation:** Soft-cap candidates per run (default 10) and coalesce duplicates; over-cap candidates are summarized into a "deferred candidates" bucket the user can expand on demand.

**Disposition:** accepted

---

### S6 — Advisory

**Reviewer:** SRE / Routing Audit Coordination

**Affected sections:** Key Capabilities → Quality-Driven Routing

**Summary:** Routing decisions form a parallel audit trail uncoordinated with FEAT-0020/0021.

**Detail:** §Quality-Driven Routing:0080-0082 says routing decisions record reason, cost, model capability, and outcome. FEAT-0020 stores artifacts; FEAT-0021 stores audit decisions. Routing decisions need to share schema with the artifact bundle (FEAT-0020 architect+SRE A1) rather than inventing their own surface.

**Recommendation:** Persist routing decisions as a `routing_decision` artifact type under the FEAT-0020 envelope; they carry the same `schema_version` discipline.

**Disposition:** accepted

---

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| A1 | accepted | Added memory specificity precedence and same-scope conflict warning behavior. |
| A2 | accepted | Added rule-based durable/ephemeral candidate triggers and future opt-in heuristic generation. |
| A3 | accepted | Clarified routing roles as model selections within existing FEAT-0016 stages, not new stages. |
| A4 | accepted | Added extension trust tiers and forbade untrusted widening or policy/validation bypass. |
| A5 | accepted | Added scope-match, vector-similarity, recency ranking and active-memory artifact recording. |
| A6 | accepted | Made age always available and defined when confidence is present or absent. |
| S1 | accepted | Added durable-memory staleness checks, 30-day ephemeral expiry, and pinning. |
| S2 | accepted | Added deterministic default routing, opt-in inference routing, fallback model behavior, and `routing_fallback`. |
| S3 | accepted | Scoped outcome learning per deployment by default with opt-in cross-tenant aggregation and provenance. |
| S4 | accepted | Added default hook deadline/memory bounds and trust-tier limit behavior. |
| S5 | accepted | Added per-run candidate soft cap, duplicate coalescing, and deferred-candidate bucket. |
| S6 | accepted | Persisted routing decisions as FEAT-0020 `routing_decision` artifacts. |
