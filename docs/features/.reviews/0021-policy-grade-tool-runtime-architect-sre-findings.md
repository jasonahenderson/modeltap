# FEAT-0021 Findings (Architect + SRE pass)

- Feature: `docs/features/0021-policy-grade-tool-runtime.md`
- Review date: 2026-05-04
- Reviewer: Claude Opus 4.7 (1M context), architect + SRE perspective
- total_findings: 12
- blocking: 0
- significant: 8
- advisory: 4
- top_line: The audit-trail discipline (`tool_call_id` + `decision_id` + `result_id`) and the BFF-policy-version handshake are two of the strongest contracts in the series. Gaps cluster in three places: the policy *language* (layer precedence, scope expressions, ambiguity defaults) is deferred to open questions that downstream features depend on; the policy-mismatch revoke flow assumes side effects can be unwound but does not provide the rollback path; and the operability of the policy engine (evaluation latency, audit tamper-resistance, network-domain enforcement boundary) is not addressed.

## Findings

### A1 — Significant

**Reviewer:** Architecture / Layer Conflict Resolution

**Affected sections:** Key Capabilities → Policy Dimensions; Open Questions

**Summary:** Four policy layers (user, project, team, server) without a precedence rule.

**Detail:** §Policy Dimensions:0055 lists "user, project, team, and server policy layers." Open Question 2 asks which wins on conflict. Without precedence, the "policy mismatch" rejection path (§0108-0110) cannot reliably detect mismatches: the BFF and harness can each compute different effective policy from the same layers and disagree. This is the single biggest deferred contract in the series for permission flow.

**Recommendation:** Pin a default: server > team > project > user, with non-overridable flags allowed at each upper layer. Decisions record the *winning layer*. Defer the rich policy language to the ADR but commit the precedence rule here.

**Disposition:** accepted

---

### A2 — Significant

**Reviewer:** Architecture / Policy Versioning

**Affected sections:** Key Capabilities → Audit Trail

**Summary:** Policy version semantics, increment cadence, and migration are unspecified.

**Detail:** §Audit Trail:0103-0111 says "the harness must acknowledge the BFF policy version before running tool calls in a run." The version concept is named but the contract is not: is it monotonic per project? Per server? What triggers an increment (any policy change? only structural change?)? What does the harness do when the version moves mid-run — drain, restart, or apply-on-next-decision? FEAT-0015 architect+SRE A6 raised schema versioning at the umbrella; policy versioning is its own version concept.

**Recommendation:** Commit to: monotonic integer per (server, project, run_creator) tuple, increments on any policy change that affects evaluation, mid-run version moves block new tool decisions until the harness re-acknowledges. Record the version on every decision artifact.

**Disposition:** accepted

---

### A3 — Significant

**Reviewer:** Architecture / Approval Scope Expressions

**Affected sections:** Key Capabilities → Permission Outcomes

**Summary:** "Approved for path/domain/tool scope" has no defined expression language.

**Detail:** §Permission Outcomes:0066 names "approved for path/domain/tool scope" but does not say whether scope is exact match, prefix, glob, or regex. With ambiguous scope semantics, a single approval (e.g. `~/projects/*`) cannot deterministically match future `tool_call_id`s. The audit trail records `decision_id` referenced by multiple `tool_call_id`s (§0099-0101), so a wrong scope-match rule scopes-out either too broadly (security risk) or too narrowly (UX).

**Recommendation:** Pin scope expressions per dimension: paths use prefix-with-trailing-slash, domains use suffix-match (incl. wildcard subdomain), commands use exact-match or canonicalized argv, tools use exact tool name. Defer richer expressions to a future policy-language slice.

**Disposition:** accepted

---

### A4 — Significant

**Reviewer:** Architecture / Server-Safe Ambiguity

**Affected sections:** Key Capabilities → Server-Safe Tools

**Summary:** "Ambiguous tools default to harness-owned execution" with no classification source-of-truth.

**Detail:** §Server-Safe Tools:0119-0121 says ambiguous tools default to harness-owned but does not define ambiguity. If the BFF claims a tool is server-safe and the harness disagrees, who wins? FEAT-0015 architect+SRE noted the disconnected-executor rule; FEAT-0017 names the open question. The actual classification authority needs to live somewhere.

**Recommendation:** State that the run-runtime ADR enumerates the authoritative server-safe set; tools not on that list are harness-owned regardless of BFF classification; the harness rejects BFF-side execution of any tool not on the enumerated list.

**Disposition:** accepted

---

### A5 — Advisory

**Reviewer:** Architecture / Hook Execution Surface

**Affected sections:** Key Capabilities → Hooks and Extensions

**Summary:** Hooks "should not be impossible" with no execution contract.

**Detail:** §Hooks and Extensions:0125-0126 declines to specify hook packaging, execution order, isolation, or error semantics. FEAT-0022 will load extensions onto runs. Without a contract for hook order and error handling, two extensions registering tool-call hooks have undefined interaction.

**Recommendation:** Pin a minimal contract: hooks execute in registration order; a hook returning an error blocks the tool call by default; hook error handling is configurable; hooks see the same `tool_call_id` and policy context as the decision. Defer packaging to FEAT-0022.

**Disposition:** accepted

---

### A6 — Advisory

**Reviewer:** Architecture / Risk Classification Source

**Affected sections:** Key Capabilities → Audit Trail; Open Questions

**Summary:** "Dynamic risk classification" has no source-of-truth.

**Detail:** §Audit Trail:0091 names "dynamic risk classification" as a recorded field. Open Question 3 asks parser-vs-pattern. FEAT-0019 architect+SRE A2 cross-references the same gap for validation checks. Without a single classifier, validation, tool runtime, and any future risk-aware feature each invent their own.

**Recommendation:** Declare the policy runtime as the single risk classifier and have FEAT-0019 inherit it. Defer the classifier implementation (parser, pattern, hybrid) to the ADR.

**Disposition:** accepted

---

### S1 — Significant

**Reviewer:** SRE / Mismatch-Revoke Rollback

**Affected sections:** Key Capabilities → Audit Trail

**Summary:** When the BFF revokes a decision, side effects already produced by the tool cannot be undone.

**Detail:** §Audit Trail:0107-0111 says "on rejection, the BFF revokes the decision and the harness must not deliver the tool result to the model." But if the tool was a `git push` or `rm`, the side effect is irreversible. The contract prevents the *result* from reaching the model; it does not prevent the *world* from being changed. The rollback gap silently undermines the policy-version handshake.

**Recommendation:** Two-phase tool execution for mutating tools: harness asks the BFF for a *commitment* before running mutating tools; BFF returns the committed policy version; harness runs the tool only if the commitment is fresh; revoke applies only to *uncommitted* tools. Read-only tools can stay single-phase. Cross-link to FEAT-0016 architect+SRE A2 (concurrency).

**Disposition:** accepted

---

### S2 — Significant

**Reviewer:** SRE / Audit Tamper Resistance

**Affected sections:** Key Capabilities → Audit Trail

**Summary:** Audit records live inside mutable run records and can be lost on cancel/fork.

**Detail:** §Audit Trail:0103-0107 says decisions are recorded as "durable run evidence." If a run is cancelled, forked, or its artifacts are GC'd (FEAT-0017/0020 retention), the audit record can disappear with it. For team/enterprise usage with compliance requirements, the audit log should be append-only and outlive the run. FEAT-0015 architect+SRE noted this; FEAT-0021 is the home.

**Recommendation:** Commit decisions to a separate append-only audit log, indexed by `decision_id`, that survives run mutations and retention. Run records hold a reference; the audit log is the source of truth. Defer the storage shape to the artifact-storage ADR.

**Disposition:** accepted

---

### S3 — Significant

**Reviewer:** SRE / Evaluation Latency

**Affected sections:** Key Capabilities → Policy Dimensions; Audit Trail

**Summary:** Per-tool-call policy evaluation has no caching or fast-path contract.

**Detail:** Multi-layer policy + dynamic risk classification per tool call adds latency to every `tool_call_id`. A run that issues 100 file reads pays this cost 100×. No commitment to caching (e.g. cache compiled policy per run, cache risk classification per command), fast-path for previously-approved scopes, or batch evaluation.

**Recommendation:** State that policy is compiled per run at `preflight` and reused; risk classification results are cached by canonical input; scope grants short-circuit re-evaluation for matching `tool_call_id`s. Latency budgets default to ≤10ms per evaluation after cache warm.

**Disposition:** accepted

---

### S4 — Significant

**Reviewer:** SRE / Network-Domain Enforcement Boundary

**Affected sections:** Key Capabilities → Policy Dimensions

**Summary:** Domain rules are listed but not bound to a specific network surface.

**Detail:** §Policy Dimensions:0050 names "network/domain rules." Domain rules apply to: outbound HTTP from tools, MCP server connections, model-provider calls, image fetches in attached files. The spec does not enumerate which surface(s) enforce the rules. Without enumeration, domain policy is half-applied at best.

**Recommendation:** Apply domain rules to: outbound network tools (curl-like), MCP server endpoints, and any tool that loads remote URLs. Model-provider calls are *not* subject to domain rules (they follow provider routing) but are recorded with provider identity. State this explicitly.

**Disposition:** accepted

---

### S5 — Advisory

**Reviewer:** SRE / MCP Provenance and Trust

**Affected sections:** Key Capabilities → Policy Dimensions; Open Questions

**Summary:** MCP tool provenance is named but trust model is open.

**Detail:** §Policy Dimensions:0054 names "MCP server/tool provenance" as a policy dimension. Open Question 4 asks how MCP provenance and trust should be represented. With MCP being upstream protocol, the deferral is reasonable, but the trust model is essential to back-fill — a malicious MCP server should not be able to impersonate a trusted tool name.

**Recommendation:** Track MCP servers by stable identity (URL + public key fingerprint); tool names are scoped by server. Untrusted servers default to per-call approval. Defer richer trust to a follow-on.

**Disposition:** accepted

---

### S6 — Advisory

**Reviewer:** SRE / Policy Evaluation DOS

**Affected sections:** Key Capabilities → Policy Dimensions

**Summary:** A run can issue many tool calls each driving expensive evaluation.

**Detail:** Without rate limits or batch evaluation, an adversarial or buggy run can issue thousands of tool calls per minute. Each call drives policy evaluation, audit-record write, and BFF round-trip. SRE-side this is a DOS vector against the BFF.

**Recommendation:** Rate-limit tool-call evaluation per run (default 100/s burst, configurable) and per harness; over-limit calls pause the run with a structured "tool_call_rate_exceeded" reason. Audit writes can batch with bounded delay.

**Disposition:** accepted

---

## Dispositions

| ID | Disposition | Rationale |
|---|---|---|
| A1 | accepted | Added server > team > project > user precedence, non-overridable upper layers, and winning-layer recording. |
| A2 | accepted | Added monotonic policy-version scope, increment cadence, mid-run re-acknowledgement, and decision recording. |
| A3 | accepted | Added conservative matching rules for path, domain, command, and tool scopes. |
| A4 | accepted | Made the ADR-enumerated server-safe list authoritative and non-listed tools harness-owned. |
| A5 | accepted | Added minimal hook order, error, and context contract. |
| A6 | accepted | Declared the policy runtime the single dynamic risk classifier inherited by validation. |
| S1 | accepted | Added two-phase mutating-tool commitment flow and non-reversibility audit language. |
| S2 | accepted | Added append-only audit log as the source of truth outliving run mutation/normal retention. |
| S3 | accepted | Added preflight policy compilation, classification cache, scope-grant fast path, and latency budget. |
| S4 | accepted | Enumerated domain-rule enforcement surfaces and excluded provider routing from domain policy. |
| S5 | accepted | Added MCP server identity, server-scoped tool names, and untrusted-server per-call approval. |
| S6 | accepted | Added per-run/per-harness tool-call evaluation rate limits and structured over-limit pause reason. |
