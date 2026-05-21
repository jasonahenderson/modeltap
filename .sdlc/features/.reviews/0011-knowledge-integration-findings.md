# FEAT-0011 Findings

- Feature: `.sdlc/features/0011-knowledge-integration.md`
- Review date: 2026-04-14
- Reviewer: peer review
- total_findings: 3
- blocking: 2
- significant: 1
- advisory: 0
- top_line: The feature captures the intended differentiator well, but it is not ready to accept because it changes accepted knowledge-layer constraints and promises user-visible forgetting semantics that conflict with always-full capture.

## Findings

### F1 — Blocking

**Reviewer:** ADR Conformance

**Affected sections:** Problem, Configuration, Relationship to ADRs, Success Criteria

**Summary:** The feature depends on an unaccepted change to ADR-0008 but treats that change as if it were already settled.

**Detail:** FEAT-0011 repeatedly states that knowledge is on by default and labels this as an ADR-0008 amendment (`.sdlc/features/0011-knowledge-integration.md:21-34`, `:122-132`, `:154-160`). At the same time it also promises graceful no-knowledge operation when `knowledge.enabled: false` (`:104-110`). That means the feature is straddling two different product postures: optional knowledge layer versus default-on integrated memory. Until the ADR change is accepted, this feature is not anchored to a stable architectural constraint.

**Recommendation:** Resolve the ADR-0008 amendment first, or rewrite the feature so acceptance does not depend on the amendment. The feature can still specify a recommended default without treating the architectural change as already decided.

### F2 — Blocking

**Reviewer:** Product Semantics

**Affected sections:** Knowledge Commands, Graceful Degradation, Success Criteria, Relationship to ADRs

**Summary:** `/forget` is specified in a way that conflicts with always-full capture and rebuild-from-capture behavior.

**Detail:** The feature promises that `/forget` removes entries from the knowledge base and that forgotten content no longer appears in search or injection results (`.sdlc/features/0011-knowledge-integration.md:81-90`, `:146-152`). But the same feature says the knowledge layer can be rebuilt from raw capture, while ADR-0005 requires always-full capture with retention-based pruning. Without tombstones, redaction markers, or a second-layer suppression list, forgotten content can reappear after rebuild and is not actually forgotten in any durable sense.

**Recommendation:** Define `/forget` precisely as either suppression-from-retrieval, durable redaction with rebuild support, or deletion subject to capture-retention policy. The current wording overpromises behavior the underlying capture model does not provide.

### F3 — Significant

**Reviewer:** Acceptance Quality

**Affected sections:** Transparent Context Enrichment, Success Criteria, Open Questions

**Summary:** Several success criteria are valuable product goals but are not yet testable enough to act as acceptance gates.

**Detail:** Criteria such as "semantic search returns relevant results" and "the model demonstrates awareness of prior decisions" are directionally right (`.sdlc/features/0011-knowledge-integration.md:56-69`, `:141-152`), but they do not specify an evaluation dataset, ranking threshold, or pass/fail method. Without that, the feature can neither fail nor pass objectively, which makes review and implementation tracking weak.

**Recommendation:** Add measurable evaluation rules such as a fixed benchmark set, top-k retrieval thresholds, latency limits, and a concrete before/after prompt-evaluation procedure for injection quality.
