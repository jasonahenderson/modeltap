# EXP-0002 Review

- Exploration: `.sdlc/explorations/0002-multi-user-support.md`
- Review date: 2026-04-14
- Reviewer: peer review
- total_findings: 4
- blocking: 2
- significant: 2
- advisory: 0
- top_line: The exploration is directionally strong, but it is not ready to promote because it leaves the deployment model, auth negotiation, and isolation guarantees underspecified where other docs already create constraints.

## Findings

### F1 — Blocking

**Summary:** The deployment model conflicts with the integrated-harness direction and does not say which document wins.

**Affected sections:** Overview, Architecture, Relationship to Other Explorations, Phasing

**Detail:** `EXP-0002` assumes a centrally operated remote server that multiple developers connect to over the network (`.sdlc/explorations/0002-multi-user-support.md:21-23`, `:40-76`, `:419-487`). But `EXP-0008` currently frames the product as one binary with no proxy-only or harness-only mode, and its service mode is a local socket attachment rather than a remote multi-user deployment. `ADR-0012` is also written around single-user local background execution. That leaves a hard ambiguity: is enterprise multi-user a new remote deployment mode of the integrated product, or does it require changing the integrated-harness exploration itself?

**Recommendation:** Add an explicit cross-doc resolution section before promotion. State whether remote multi-user server mode is a planned extension of `EXP-0008`, a separate enterprise deployment profile, or a superseding direction that requires edits to `EXP-0008` and any affected ADRs.

### F2 — Blocking

**Summary:** The auth negotiation flow is downgrade-prone and lacks a binding between discovery, selected method, and accepted credential.

**Affected sections:** Pluggable Identity Provider Chain, Provider Priority and Discovery, Server Configuration

**Detail:** The exploration says the harness hits an unauthenticated discovery endpoint, chooses the "best available method," and then the server accepts the first configured provider that can verify the presented credential (`.sdlc/explorations/0002-multi-user-support.md:94-105`, `:265-287`, `:425-450`). That is not enough to reason about downgrade resistance. A stolen long-lived token should not be able to satisfy a connection that an enterprise intends to require over mTLS or OIDC, and mixed credential presentation order is currently unspecified. The design also does not define how the chosen auth method is pinned to the session, how issuer/audience are scoped, or how the server rejects weaker fallback methods when multiple methods are configured.

**Recommendation:** Make auth selection server-driven, not best-effort client-driven. Define method pinning, rejection semantics for weaker credentials, and the exact transport requirements for each provider. If method discovery remains unauthenticated, document why it does not create downgrade or spoofing risk.

### F3 — Significant

**Summary:** The document claims "absolute" content isolation while recommending an application-layer tenant filter that can still fail open.

**Affected sections:** Data Isolation, Privacy Model, Deferred: Shared Knowledge, Phase 1

**Detail:** The exploration says isolation is enforced at the query layer and recommends a single database with `user_id` filtering for the initial implementation (`.sdlc/explorations/0002-multi-user-support.md:293-307`, `:398-411`, `:520-529`). That is a reasonable starting point, but it is not the same thing as "Content isolation is absolute." The same document also asks the storage model to prepare for future shared knowledge (`:496-506`), which further weakens the claim that there is no path across tenants. The current wording overstates the guarantee and hides the real design trade-off: operational simplicity first, hard isolation later if required.

**Recommendation:** Either soften the guarantee language or add concrete guardrails that justify it: tenant-scoped storage interfaces, mandatory negative tests for cross-user reads, no raw query escape hatches, and an explicit threshold for when per-user databases become mandatory.

### F4 — Significant

**Summary:** The internal identity and authorization model is too small for the policies the document already wants to express.

**Affected sections:** Pluggable Identity Provider Chain, Roles and Authorization, Policy Enforcement, Open Questions

**Detail:** All providers currently normalize to `{user_id, role}` (`.sdlc/explorations/0002-multi-user-support.md:94-105`), but the rest of the exploration already needs more than one role source and more than one effective role: provider-derived groups, supplemental static admin assignment, future auditor access, and policy examples for `developer` and `senior_developer` (`:172-189`, `:321-332`, `:338-383`, `:447-450`). A single `role` field does not explain precedence, composition, or how conflicting claims are resolved. Without that, the policy model is not promotable into an implementation-scoped artifact.

**Recommendation:** Promote the normalized identity shape to something closer to `{user_id, roles, auth_method, org_id?}` and define role-resolution precedence. If the exploration intentionally wants to stay narrow, then cut back the policy examples to match the smaller identity contract.

## Promotion Recommendation

Do not promote `EXP-0002` yet.

Promote after:

1. The remote enterprise deployment story is reconciled with `EXP-0008` and `ADR-0012`.
2. The auth negotiation model is specified tightly enough to reason about downgrade resistance.
3. The isolation claim is aligned with the actual first-phase enforcement model.
4. The normalized identity contract is expanded or the policy surface is narrowed to fit it.

## Residual Strengths

- The exploration correctly identifies the business driver: enterprise deployment needs centralized credentials, per-user isolation, and aggregate metrics.
- The phased rollout is sensible.
- Deferring shared knowledge is the right default; the document does not overpromise collaborative memory in v1.
