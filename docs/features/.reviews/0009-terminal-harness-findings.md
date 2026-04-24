# FEAT-0009 Findings

- Feature: `docs/features/0009-terminal-harness.md`
- Review date: 2026-04-14
- Reviewer: peer review
- total_findings: 3
- blocking: 2
- significant: 1
- advisory: 0
- top_line: The feature is directionally sound, but it mixes prototype and production acceptance targets and assumes a tool-protocol contract that FEAT-0008 has not yet defined.

## Findings

### F1 — Blocking

**Reviewer:** Scope and Phasing

**Affected sections:** Solution, Key Capabilities, Non-Goals, Success Criteria, Relationship to ADRs

**Summary:** The feature collapses the minimal prototype and the production Bubbletea harness into one acceptance target.

**Detail:** The solution and ADR reference explicitly adopt a phased UI strategy: minimal prototype first, Bubbletea later (`docs/features/0009-terminal-harness.md:24-27`, `:197-203`, `:218-225`). But the success criteria require production-grade behavior such as styled markdown rendering, a scrollable viewport, a status bar, session commands, and persistent UX polish (`:204-217`). That makes it unclear what "accepted" means and undermines the value of ADR-0013's phased decision.

**Recommendation:** Split FEAT-0009 into Phase 1 and Phase 2 acceptance criteria, or create a follow-on feature/work unit for the Bubbletea production harness.

### F2 — Blocking

**Reviewer:** Integration Readiness

**Affected sections:** Tool Execution, Permission Model, Execution Modes, Success Criteria

**Summary:** The harness feature assumes a dynamic tool catalog and permission boundary that the BFF protocol does not yet expose.

**Detail:** FEAT-0009 says the harness executes core tools and MCP-discovered tools locally, narrows tool permissions by mode, and exposes those tools to the model (`docs/features/0009-terminal-harness.md:37-82`, `:216`). But FEAT-0008 currently defines only `tool.call` and `tool.result`; it does not define how available tools, input schemas, descriptions, or permission-narrowed subsets are registered with the server or included in model-facing prompt assembly. Without that handshake, the server cannot reliably instruct the model about the tools it may call.

**Recommendation:** Add an explicit tool-registration/capabilities exchange to FEAT-0008 and reference it here, or narrow FEAT-0009 to a fixed built-in tool set until dynamic discovery is specified.

### F3 — Significant

**Reviewer:** Boundary Clarity

**Affected sections:** File Context Management, Large Paste Handling, Configuration

**Summary:** The harness/server boundary for attachment parsing and large-paste summarization is still ambiguous.

**Detail:** The feature says the harness parses files, extracts PDFs/DOCX/images/spreadsheets, detects large pastes, and can offer summarization via a cheap model while the BFF still captures the full content (`docs/features/0009-terminal-harness.md:84-106`). That leaves two unresolved questions: which side owns content transformation before model submission, and how the full raw payload versus summarized payload is represented on the wire. Those details matter for capture correctness, privacy, and reproducibility.

**Recommendation:** Specify whether the harness always sends raw content plus user intent metadata, or whether it may send preprocessed artifacts. The protocol should distinguish raw capture payloads from transformed context payloads explicitly.

## Dispositions

| ID | Severity | Disposition | Rationale |
|----|----------|-------------|-----------|
| F1 | blocking | — | |
| F2 | blocking | — | |
| F3 | significant | — | |

Dispositions: one of `accepted`, `rejected`, `deferred`. Leave as `—` until resolved. Add a rationale cell whenever a disposition is set (especially for `rejected` / `deferred`).
