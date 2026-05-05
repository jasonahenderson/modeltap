# FEAT-0010 Findings

- Feature: `docs/features/0010-enterprise-auth.md`
- Review date: 2026-04-14
- Reviewer: peer review
- total_findings: 3
- blocking: 1
- significant: 2
- advisory: 0
- top_line: The feature captures the right enterprise requirements, but the auth negotiation behavior is still too delegated to upstream exploration material for this to be an accept-ready feature on a sensitive surface.

## Findings

### F1 — Blocking

**Reviewer:** Security

**Affected sections:** Pluggable Identity Provider Chain, Harness Auth Integration, Configuration, Success Criteria

**Summary:** The feature leaves critical auth negotiation and downgrade-resistance behavior outside the feature contract.

**Detail:** FEAT-0010 says auth negotiation is server-driven and points to EXP-0002 for the full protocol, including downgrade resistance (`docs/features/0010-enterprise-auth.md:37-40`, `:173-185`, `:232-242`). For a proposed feature on authentication and authorization, that is not enough. The feature itself still needs to specify method pinning, challenge binding, session identity caching rules, and the exact failure semantics when multiple methods are configured. Otherwise the highest-risk part of the behavior remains advisory rather than contractual.

**Recommendation:** Pull the auth handshake and downgrade-resistance rules into FEAT-0010 directly, or add a constraining ADR that the feature can normatively depend on before acceptance.

### F2 — Significant

**Reviewer:** Operational Readiness

**Affected sections:** Roles and Authorization, CLI Integration, Configuration, Success Criteria

**Summary:** The spec does not define a clear bootstrap and recovery path for the first admin.

**Detail:** The feature says admin operations require OIDC or SPIFFE and explicitly blocks token-authenticated users from admin actions (`docs/features/0010-enterprise-auth.md:118-125`, `:188-223`, `:239-241`). That improves security, but it leaves an operational gap: how does the very first admin get established in a new deployment, and what is the break-glass path if OIDC is unavailable? The listed admin commands assume an admin already exists.

**Recommendation:** Define an initial bootstrap path explicitly, such as local-only bootstrap on first start, a signed config-time admin seed, or a one-time installation command with tightly scoped semantics.

### F3 — Significant

**Reviewer:** Isolation Assurance

**Affected sections:** Per-User Data Isolation, Admin Aggregate Metrics, Success Criteria, Open Questions

**Summary:** The isolation test contract is narrower than the feature surface that can cross user boundaries.

**Detail:** The feature requires negative isolation tests for query paths and defines a `UserScopedStore` boundary (`docs/features/0010-enterprise-auth.md:96-107`, `:234-242`). That is good, but the spec also exposes admin metrics, background aggregation, token provisioning, and future knowledge integration. It does not explicitly require negative tests for rebuild jobs, export paths, MCP/search access, or any other non-request-path data access. On a multi-user feature, those are the paths that usually leak first.

**Recommendation:** Expand the success criteria to require isolation tests for every data-access surface: request queries, session resume/list, metrics, export/search, background rebuild jobs, and any MCP-facing knowledge queries introduced by dependent features.
