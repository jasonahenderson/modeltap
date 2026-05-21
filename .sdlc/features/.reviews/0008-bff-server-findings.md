# FEAT-0008 Findings

- Feature: `.sdlc/features/0008-bff-server.md`
- Review date: 2026-04-14
- Reviewer: peer review
- total_findings: 3
- blocking: 1
- significant: 2
- advisory: 0
- top_line: The feature has the right architectural direction, but it is not ready to accept because the core harness protocol is still too underspecified for the dependent features to implement against safely.

## Findings

### F1 — Blocking

**Reviewer:** Architecture Conformance

**Affected sections:** Harness Protocol Endpoint, Protocol Messages, Success Criteria, Open Questions

**Summary:** The harness/server protocol is the load-bearing contract for multiple downstream features, but the feature does not define enough of it to implement or version safely.

**Detail:** FEAT-0008 says the server speaks "JSON-RPC with bidirectional streaming" and enumerates message names, but it does not define request/stream correlation, cancellation, stream lifecycle, capability negotiation, protocol versioning, or how tool schemas become visible to the server and model (`.sdlc/features/0008-bff-server.md:40-75`, `:240-250`, `:264-268`). FEAT-0009, FEAT-0010, and FEAT-0012 all depend on this contract. Without a concrete protocol spec, the feature is still describing intent rather than a stable behavior target.

**Recommendation:** Add a protocol appendix or linked ADR that specifies framing, correlation IDs, stream ownership, cancellation, capability/tool registration, auth handshake boundaries, and version negotiation before this feature is accepted.

### F2 — Significant

**Reviewer:** ADR Conformance

**Affected sections:** Provider Message Format Translation, Relationship to ADRs

**Summary:** The feature changes the provider adapter contract in a way that exceeds the accepted ADR without an explicit ADR update.

**Detail:** The accepted provider strategy in ADR-0006 is framed around detection, parsing, usage extraction, and stream reassembly. FEAT-0008 extends that contract to outbound formatting and full-history translation across multiple provider message shapes (`.sdlc/features/0008-bff-server.md:86-102`, `:252-260`). That is a sensible direction, but it materially changes a constraining interface. Treating it as an implied extension inside the feature spec weakens the ADR/feature boundary.

**Recommendation:** Amend or supersede ADR-0006 to define the expanded provider interface explicitly, then keep FEAT-0008 focused on the user-visible server behavior that depends on that interface.

### F3 — Significant

**Reviewer:** Implementation Readiness

**Affected sections:** Cost Tracking, Session Persistence, Relationship to ADRs, Success Criteria

**Summary:** The spec promises new session- and turn-level reporting behavior without defining the storage and aggregation shape that supports it.

**Detail:** FEAT-0008 promises per-turn cost updates, per-session totals, model-switch history, session inspection commands, and future concurrent harness support (`.sdlc/features/0008-bff-server.md:143-175`, `:185-191`, `:247-250`). ADR-0007 only establishes hourly and daily provider/model aggregates, and the feature does not define the new tables or retention semantics for session/turn data. That leaves a gap between the proposed CLI behavior and the storage contract needed to support it.

**Recommendation:** Add the minimal session/turn persistence model to this feature, or split the storage/reporting portion into a smaller supporting patch/ADR so the acceptance target is concrete.
