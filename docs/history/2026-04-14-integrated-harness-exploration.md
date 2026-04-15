# 2026-04-14: Integrated Harness Exploration and Feature Specs

## Session Summary

Deep product-architecture exploration session that reframed modeltap from a passive reverse proxy into an integrated professional AI environment. Started with a conversation about pairing a terminal harness with the proxy (BFF pattern), evolved through enterprise auth, deployment profiles, terminal UI framework evaluation, harness capabilities, session quality analysis, domain extensibility, and agent teams — then produced six feature specs with peer review processing.

## Key Decisions

1. **One product, not two**: the harness and server are inseparable. No standalone proxy mode. If someone wants just a proxy, they fork.
2. **Server always required**: the harness never talks to providers directly. Even solo developers run a local server process (auto-started).
3. **Proxy owns API keys**: developers never hold provider credentials. Server is the credential boundary.
4. **Per-user isolation, no shared knowledge (for now)**: each user's data is fully isolated. Shared knowledge deferred.
5. **Bubbletea from day one**: ADR-0013 revised 2026-04-15 — feature requirements make phased approach impractical.
6. **Domain-neutral architecture**: coding is the first vertical. Legal, finance, healthcare are future verticals with the same server, different tool packages and system prompts.
7. **Skills and agent teams are separate features**: skills are harness-side prompt templates; agent teams are BFF-orchestrated multi-model coordination.

## Artifacts Created

### Explorations
- **EXP-0008**: Integrated Harness — Modeltap as Professional AI Environment (new)
- **EXP-0002**: Multi-User Support (rewritten for enterprise auth architecture)
- Both processed through peer review with all findings addressed

### ADRs
- **ADR-0013**: Terminal UI Framework — phased minimal → Bubbletea (proposed)

### Feature Specs (all proposed)
- **FEAT-0008**: BFF Server — harness protocol, conversation state, routing, session persistence
- **FEAT-0009**: Terminal Harness — tool execution, permissions, plan/build modes, file context
- **FEAT-0010**: Enterprise Auth — pluggable identity (token, OIDC, SPIFFE), isolation, credentials
- **FEAT-0011**: Knowledge Integration — embedding, context enrichment, lossless compaction
- **FEAT-0012**: Skills — reusable prompt+tool configurations for common tasks
- **FEAT-0013**: Agent Teams — BFF-coordinated multi-model orchestration with safe execution rules
- All six processed through peer review with all findings addressed (15 findings total: 8 blocking, 7 significant)

### Branches
- `pre-harness-exploration` — project state before this session (at d2627b7)
- `exploration/integrated-harness` — all new work (4 commits)

## Dependency Chain

```
FEAT-0008 (BFF Server)
    └── FEAT-0009 (Terminal Harness)
            ├── FEAT-0010 (Enterprise Auth)
            ├── FEAT-0011 (Knowledge Integration)
            ├── FEAT-0012 (Skills)
            └── FEAT-0013 (Agent Teams)
```

Minimum viable product: FEAT-0008 + FEAT-0009
First enterprise engagement: + FEAT-0010
Full differentiated product: + FEAT-0011 + FEAT-0012 + FEAT-0013

## ADR Amendments Identified (not yet written)
- ADR-0006 amendment: add outbound message formatting to provider adapter interface
- ADR-0008 amendment: knowledge layer on by default (proposed, not required for feature acceptance)
- ADR-0009 scope clarification: external MCP surface only, harness protocol is separate

## What's Next
- See 2026-04-15 session log for continued refinement
- Implementation planning for FEAT-0008 and FEAT-0009

## Memory Notes
- User wants enterprise support as near-term priority for contract work and LinkedIn positioning
- User's M5 Max hardware makes local inference (MLX, Ollama) relevant for the security-agent-runs-locally story
- Product positioning: "enterprise AI environment for professional workflows" — coding is first vertical, not the only one
