# EXP-0008 Review

- Exploration: `.sdlc/explorations/0008-integrated-harness.md`
- Review date: 2026-04-14
- Reviewer: peer review
- total_findings: 4
- blocking: 2
- significant: 2
- advisory: 0
- top_line: The exploration has a strong product thesis, but it is not ready to promote because it changes accepted knowledge-layer and MCP assumptions without explicitly superseding them, and it bundles several different product directions into one artifact.

## Findings

### F1 — Blocking

**Summary:** The exploration quietly overturns the accepted "knowledge layer is optional" architecture without an explicit supersession path.

**Affected sections:** Context, The Knowledge Layer as Core Differentiator, Implications for Existing Explorations, Proposed Next Steps

**Detail:** `EXP-0008` makes the knowledge layer core to the product and says it is effectively always on: the BFF enriches every turn with knowledge context, every turn enriches the knowledge base, and `EXP-0001` is now "no longer opt-in" (`.sdlc/explorations/0008-integrated-harness.md:153-164`, `:215`, `:507-523`, `:694-697`). But accepted `ADR-0008` explicitly chose a fully optional knowledge layer so users could run modeltap as a lightweight proxy without performance, disk, or dependency cost. This is not a small interpretation change; it is a direct architectural conflict. The exploration can argue for changing that decision, but it needs to say so explicitly and carry the consequences through the ADR layer.

**Recommendation:** Add an explicit supersession or amendment path for `ADR-0008`, or revise the exploration so the integrated harness still works when the knowledge layer is disabled. Until that is resolved, this should not promote into downstream implementation artifacts.

### F2 — Blocking

**Summary:** The protocol and MCP story conflicts with the accepted MCP stdio decision and leaves the canonical interface boundary unclear.

**Affected sections:** MCP, The Frontend Question, Technical Threads, Open Questions

**Detail:** The exploration says the BFF exposes knowledge and orchestration via MCP as an internal standards-compliant interface (`.sdlc/explorations/0008-integrated-harness.md:570-584`), then says the BFF protocol is JSON-RPC over TLS and may back multiple frontends including web (`:667-676`, `:716-724`, `:871-875`). Accepted `ADR-0009` chose MCP over stdio specifically to avoid a network listener and to keep knowledge access local and subprocess-scoped. `EXP-0008` may want both an internal product protocol and a separate MCP interface, but it does not distinguish them clearly enough. Right now it reads as though MCP, the internal harness protocol, and future web-facing transport are partially the same thing.

**Recommendation:** Split the interfaces explicitly: one canonical harness/server protocol, one external MCP surface if still desired, and any separate HTTP/Web API if needed for dashboards or future web frontends. If `ADR-0009` is being superseded or narrowed to "external local knowledge access only," say that directly.

### F3 — Significant

**Summary:** The artifact scope is too broad to promote cleanly because it mixes at least three product bets with different downstream contracts.

**Affected sections:** Problem, Design, Domain Extensibility, Business Positioning, Proposed Next Steps

**Detail:** The document starts as an exploration for an integrated terminal harness, but it also defines a remote enterprise deployment model (`.sdlc/explorations/0008-integrated-harness.md:60-96`), a domain-neutral vertical platform spanning legal/finance/healthcare (`:586-690`), and a future multi-frontend strategy including web harnesses (`:667-676`). Those are related, but they are not one promotable behavior contract. The proposed next steps only implement a narrow subset (`:915-931`), which is a signal that the exploration currently carries more scope than the near-term product decision can support.

**Recommendation:** Keep this exploration as the product-thesis umbrella, but split promotion targets: one feature/patch line for the terminal harness MVP, one separate exploration or ADR for multi-frontend protocol/API strategy, and one separate exploration for domain-vertical expansion if that remains active.

### F4 — Significant

**Summary:** The execution-boundary model is internally inconsistent about who controls local actions and permission gating.

**Affected sections:** The BFF Contract, Plan and Build Modes, Thread 7: Security Model

**Detail:** The harness is described as owning local tool execution and permission enforcement (`.sdlc/explorations/0008-integrated-harness.md:115-123`), and the security thread later says the BFF never executes local operations (`:751-756`). But the plan/build section says "the BFF intercepts tool calls," and "the BFF decides whether to execute or present based on the current mode" (`:320-352`). Those are materially different control models. If the harness is the trust boundary for local actions, mode gating and final execution authority need to live there. Otherwise the server starts to become a policy decision point for machine-local actions it cannot safely enforce.

**Recommendation:** Tighten the contract so the server proposes tool actions and the harness remains the sole authority for local execution mode, permission prompts, and enforcement. If the server retains plan/build policy, define exactly how the harness verifies and enforces that policy without trusting server intent.

## Promotion Recommendation

Do not promote `EXP-0008` yet.

Promote after:

1. The knowledge-layer requirement is reconciled with `ADR-0008`.
2. The internal protocol, MCP surface, and any future web/API surface are separated cleanly.
3. The near-term harness MVP scope is separated from the broader domain-platform thesis.
4. The local-execution trust boundary is made internally consistent.

## Residual Strengths

- The core product thesis is strong: modeltap is more differentiated as an active AI environment than as a passive proxy.
- The harness/server split is directionally sound and makes room for enterprise features without putting provider logic on the client.
- The proposed next steps identify a practical first slice even though the exploration as a whole is broader than that slice.
