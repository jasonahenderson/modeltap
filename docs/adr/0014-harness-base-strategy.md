---
status: accepted
date: 2026-04-22
decision-makers: Jason Henderson
related:
  - EXP-0010
  - FEAT-0009
  - FEAT-0013
  - ADR-0013
  - docs/adr/.reviews/0014-harness-base-strategy-findings.md
---

# ADR-0014: Harness Base Strategy

## Context and Problem Statement

Modeltap's harness **is already** a thin client: it delegates model routing, cost control, logging, and capture to the BFF over JSON-RPC. **Crucially, the harness must continue to support larger sub-agent teams coordinating across tasks and potentially across different models as a first-class capability.** The question is whether to evolve this existing architecture or abandon it by forking OpenCode or OpenHarness — both of which are self-contained agent loops that would require inverting to BFF delegation.

This decision constrains multi-agent orchestration architecture, language/runtime alignment, maintenance burden, and distribution model.

## Decision Drivers

Weights are expressed as hundredths and sum to **1.00** (see ADR Template Conventions).

* **D1 — Proxy-centric control (0.25):** The harness must delegate model routing, cost control, and logging to the BFF. A self-contained agent loop is a project-threatening anti-pattern for modeltap's core value proposition.
* **D2 — Multi-agent orchestration (0.25):** Running larger sub-agent teams that coordinate across tasks and potentially across models is a critical strategic capability. The base must either already support this or be structured so the BFF can own orchestration and dispatch to the harness.
* **D3 — Language/runtime alignment (0.15):** The BFF is Go. A harness in the same language shares types, build system, and contributor pool. A different runtime (Python, Node.js) creates a permanent two-team tax and complicates deployment.
* **D4 — Upstream maintainability (0.15):** Forking without upstream continuity means owning 100% of future maintenance. Tracking upstream reduces burden but introduces merge tax. An archived upstream is a liability.
* **D5 — Terminal UX quality (0.10):** Streaming token rendering, markdown display, multi-line input, and scrollback are the core daily UX. The base must handle this well without heroic engineering.
* **D6 — Single binary distribution (0.05):** Modeltap ships as one compiled artifact. A second runtime complicates install, CI, and cross-compilation. Important but secondary to orchestration and proxy alignment.
* **D7 — Feature richness today (0.05):** Tools, memory, extensibility, and safety governance matter for user adoption. Starting from a rich base accelerates time-to-value, but only if the richness can be harvested without architectural inversion costs that negate the savings.

## Considered Options

* **O1 — Continue modeltap harness (universal orchestration client).** Evolve the existing in-tree Go/Bubbletea thin client. The harness remains the **primary human interface for orchestration** — it displays subagent team status, allows users to kick off multi-agent tasks, approves team-level tool calls, and observes cross-model coordination — while all execution stays in the BFF. To close capability gaps, selectively port proven subsystems from OpenHarness (e.g., `coordinator/` swarm logic, `memory/` compression, `permissions/` governance) into Go as BFF subsystems. The harness grows orchestration-aware UI; the BFF grows agent richness. No fork.
* **O2 — Fork OpenCode (hard fork).** Fork the archived `opencode-ai/opencode` Go/Bubbletea codebase, then gut its self-contained agent loop and invert it into a thin BFF client. Abandon upstream tracking.
* **O3 — Fork OpenCode (soft fork, track Crush).** Fork OpenCode but maintain a merge strategy with its successor `charmbracelet/crush`. Periodically pull upstream fixes and re-apply the BFF-inversion patches.
* **O4 — Fork OpenHarness (hard fork).** Fork the active Python `HKUDS/OpenHarness` project, then rewrite its TUI in Go/Bubbletea and invert its agent loop to delegate to the BFF. Abandon upstream tracking.
* **O5 — Fork OpenHarness (soft fork, track upstream).** Fork OpenHarness but maintain a merge strategy with upstream. Periodically pull Python/Node.js updates and re-apply Go-port + BFF-inversion patches.
* **O6 — Conversation-only harness.** Draw a hard boundary: all multi-agent orchestration lives in the BFF. The harness remains a **strictly single-model conversation client** — it renders streaming responses, approves tool calls, and manages sessions, but it is **oblivious to subagents, teams, or cross-model coordination**. Orchestration UI (team dashboards, subagent status, model fan-out controls) lives exclusively in the web dashboard, Slack bot, or API. The harness knows nothing more than a single-turn conversation loop.

## Decision Outcome

Chosen option: **O1 — Continue modeltap harness (universal orchestration client)**, because it scores highest (0.730) and wins on the two highest-weighted drivers (proxy-centric control and multi-agent orchestration). It is the only option that preserves the existing thin-client architecture while also evolving the harness into an **orchestration-aware human interface** — not a passive observer, but the primary terminal from which users interact with subagent teams.

**Note on scoring:** O1's D2 score (4) reflects today's gap — there is no multi-agent orchestration in the harness yet — combined with the forward commitment to close it. This ADR accepts that commitment. If that commitment were not accepted, O6 (0.705) would score higher. The strategic claim, not the arithmetic margin, is the reason for choosing O1 (see "Why O1 beats O6" below).

### Scoring Matrix

Scale: 1–10 (poor → excellent). Weighted total = sum of (weight × score/10).

| Driver | Weight | O1 Universal orchestration client | O2 Hard fork OpenCode | O3 Soft fork OpenCode | O4 Hard fork OpenHarness | O5 Soft fork OpenHarness | O6 Conversation-only harness |
|--------|--------|-------------------------------------|-----------------------|-----------------------|--------------------------|--------------------------|------------------------------|
| D1: Proxy-centric control | 0.25 | 9 | 3 | 3 | 4 | 3 | 9 |
| D2: Multi-agent orchestration | 0.25 | 4 | 2 | 2 | 8 | 7 | 3 |
| D3: Language/runtime alignment | 0.15 | 9 | 9 | 9 | 2 | 2 | 9 |
| D4: Upstream maintainability | 0.15 | 9 | 2 | 1 | 5 | 2 | 9 |
| D5: Terminal UX quality | 0.10 | 7 | 8 | 8 | 5 | 5 | 7 |
| D6: Single binary distribution | 0.05 | 9 | 9 | 9 | 1 | 1 | 9 |
| D7: Feature richness today | 0.05 | 4 | 6 | 6 | 9 | 8 | 4 |
| **Weighted Total** | **1.00** | **0.730** | **0.445** | **0.430** | **0.505** | **0.405** | **0.705** |

### Scoring Justification

#### O1 — Continue modeltap harness (0.730)

* **D1 (9):** Native thin-client architecture. The BFF owns routing, costing, logging, and capture. Zero architectural inversion required.
* **D2 (4):** No multi-agent orchestration today, which is a genuine gap (FEAT-0013 unimplemented). The score reflects current capability, not future aspiration. The justification for accepting this gap is strategic: the thin-client model gives the BFF a clean place to insert orchestration, and the harness can grow orchestration-aware UI incrementally as the BFF matures. Selective porting of OpenHarness `coordinator/` logic into Go BFF subsystems is a credible path.
* **D3 (9):** Pure Go. Shared types, one build, one test suite, no serialization boundary.
* **D4 (9):** No upstream dependency other than Go stdlib, Bubbletea, and Charm ecosystem — all actively maintained. No merge tax.
* **D5 (7):** Bubbletea + Glamour is proven (OpenCode precedent). Streaming requires debouncing but is solvable. Slightly behind OpenCode which already solved the same pattern.
* **D6 (9):** Single compiled binary with the BFF. No runtime dependencies.
* **D7 (4):** MCP only today. Memory, plugins, slash commands, and 43+ tools are missing. These must be built or ported.

#### O2 — Hard fork OpenCode (0.445)

* **D1 (3):** Requires gutting the self-contained agent loop and inverting it to BFF delegation. OpenCode was never designed for proxy-centric control.
* **D2 (2):** Recursive agent tool exists but is single-process. No multi-model orchestration. Converting this to BFF-mediated swarm is substantial redesign, not adaptation.
* **D3 (9):** Go. Same as modeltap.
* **D4 (2):** Archived upstream. No upstream fixes. All maintenance is owned. Effectively a one-time snapshot.
* **D5 (8):** Bubbletea implementation is mature and proven at scale. Direct reference for streaming markdown, plan/build modes, session explorer.
* **D6 (9):** Single compiled binary.
* **D7 (6):** SQLite sessions, LSP, auto-compact, custom commands, MCP. Richer than modeltap today but not as rich as OpenHarness.

#### O3 — Soft fork OpenCode, track Crush (0.430)

* **D1 (3):** Same inversion burden as O2.
* **D2 (2):** Same as O2.
* **D3 (9):** Same as O2.
* **D4 (1):** Crush may diverge architecturally from OpenCode. The thin-client inversion touches core packages, creating constant merge conflicts. The merge tax may exceed the upstream benefit.
* **D5 (8):** Same as O2.
* **D6 (9):** Same as O2.
* **D7 (6):** Same as O2.

#### O4 — Hard fork OpenHarness (0.505)

* **D1 (4):** Self-contained agent loop, but Python's dynamic nature makes inversion *slightly* easier than Go (monkey-patching, hook points). Still requires a significant architectural rewrite.
* **D2 (8):** `coordinator/` provides mature swarm/subagent logic with team registries and background tasks. Best-in-class for this driver.
* **D3 (2):** Python for engine + Node.js for TUI. Cannot compile into Go binary. Requires either a Python runtime alongside the BFF or a complete Go rewrite of the TUI layer.
* **D4 (5):** Active upstream, but hard fork means absorbing the entire codebase and owning it. No merge tax, but also no upstream fixes.
* **D5 (5):** React/Ink is proven (Claude Code precedent), but requires Node.js. Porting to Bubbletea is a full rewrite.
* **D6 (1):** `pip install` + Node.js runtime. Not a single binary.
* **D7 (9):** 43+ tools, memory, plugins, hooks, slash commands, safety governance, chat gateway. Richest feature set of any option.

#### O5 — Soft fork OpenHarness, track upstream (0.405)

* **D1 (3):** Same inversion burden as O4, plus the added complexity of maintaining BFF-inversion patches across upstream releases.
* **D2 (7):** Same richness as O4, but patches may conflict when `coordinator/` changes upstream. Merge tax erodes the orchestration advantage.
* **D3 (2):** Same as O4.
* **D4 (2):** Unsustainable. Active upstream moves fast. Re-applying architectural inversion patches (Go-port + BFF-delegation) on every upstream sync is a high ongoing tax.
* **D5 (5):** Same as O4.
* **D6 (1):** Same as O4.
* **D7 (8):** Same richness as O4, but with the merge-tax risk that new upstream features may not integrate cleanly.

#### O6 — Conversation-only harness (0.705)

* **D1 (9):** Native thin-client architecture. Same as O1.
* **D2 (3):** Orchestration is strictly server-side, which is architecturally clean. But the harness is **oblivious to subagents** — no team status, no subagent output, no orchestration UI. Users must leave the terminal to a web dashboard or Slack bot to manage teams. For a tool where the terminal is the primary interface, making orchestration invisible to the harness undermines the user experience.
* **D3 (9):** Same as O1.
* **D4 (9):** Same as O1.
* **D5 (7):** Same as O1.
* **D6 (9):** Same as O1.
* **D7 (4):** Same gap as O1 — missing rich tools, memory, plugins today. Additionally, the harness deliberately excludes orchestration awareness, so it cannot grow into a richer agent UI even as the BFF gains capability.

### Why O1 beats O6

Both are thin-client architectures with identical scores on proxy-centric control, language alignment, maintainability, terminal UX, and single-binary distribution. The decisive difference is whether the harness should be the **universal orchestration client** or a **conversation-only client**.

**The strategic claim:** modeltap's terminal is meant to be the primary interface for all agent interaction — not just single-model chat, but multi-agent team management. If orchestration UI lives only in the dashboard, the product fragments into "terminal for chat, web for teams." Users who spend their day in the terminal will abandon multi-agent workflows because the friction of context switching is too high. O6 compounds this fragmentation by design; O1 avoids it by evolving the harness.

The D2 score gap (O1=4, O6=3) reflects this: neither option delivers orchestration today, but O1's path preserves the terminal as the primary surface, while O6's path deliberately surrenders it. The numeric margin (0.730 vs 0.705) is real but secondary to the architectural argument.

Both options assume the BFF grows orchestration capability; neither avoids that work. O1 is preferred because it keeps the harness as the **universal orchestration client**.

## Consequences

* Good, because the thin-client architecture is preserved — central model control, logging, and costing remain native to the BFF.
* Good, because the Go single-binary story is preserved — no Python or Node.js runtime in the client.
* Good, because there is no upstream merge tax — modeltap owns its own roadmap without re-applying patches on every external release.
* Good, because selective porting of OpenHarness subsystems (swarm logic, memory compression, governance) is a viable path to feature parity without architectural inversion.
* Bad, because feature richness (tools, memory, plugins, slash commands) must be built or ported — there is no drop-in inheritance from a richer codebase.
* Bad, because streaming token UX in Bubbletea requires deliberate engineering (debounced redraws) that OpenCode has already solved — modeltap must replicate or surpass this work.
* Neutral, because multi-agent orchestration is still unimplemented. The decision does not magically produce subagent teams. It merely names the base from which they will be built or ported.

## Confirmation

This-ADR confirmation (gates `accepted` status):
- The harness connects to the BFF, renders a multi-session explorer, streams a model response with styled markdown, executes tools with permission prompts, and displays orchestration-aware UI elements (e.g., session list with team indicators).

Future-feature confirmation (tracked under FEAT-0013 or successor):
- The harness can observe and meaningfully interact with a server-orchestrated subagent team's progress and results (view team status, approve team-level tool calls, initiate coordinated tasks).

## More Information

**Comparable tools and their relevance:**

* **OpenCode** (`opencode-ai/opencode`, archived, succeeded by `charmbracelet/crush`) is the closest Bubbletea precedent. It proves that a Go terminal AI agent can achieve high UX quality with streaming markdown, plan/build modes, and session management. It is best treated as a **design reference for TUI patterns**, not a fork base, because retrofitting BFF delegation would require gutting its core architecture.
* **OpenHarness** (`HKUDS/OpenHarness`, active Python) is the richest agent runtime. Its `coordinator/`, `memory/`, and `permissions/` subsystems are proven and well-designed. It is best treated as a **design reference for agent subsystems** and a **selective porting source** for BFF-side features, not a fork base, because its Python/Node.js runtime is fundamentally incompatible with modeltap's Go single-binary model.

**Open questions:**

1. Which OpenHarness subsystems should be ported first — `coordinator/`, `memory/`, or `permissions/`?
2. How much of the forward bet on orchestration-aware UI (the D2=4 score in O1) must be materialized before this ADR flips to `accepted`?
3. When does orchestration-aware UI land — v0.2.0 or a later release?
4. Should the selective porting of OpenHarness subsystems be treated as a feature spec (FEAT-0014+) or as an engineering strategy in the harness track?

## Review Findings

Reviewed per `docs/adr/.reviews/0014-harness-base-strategy-findings.md`. Dispositions below.

| ID | Severity | Disposition | Rationale |
|----|----------|-------------|-----------|
| F1 | blocking | accepted | Recomputed all weighted totals using `weight × score/10`. Four totals corrected (O2: 0.445, O3: 0.430, O4: 0.505, O5: 0.405). |
| F2 | blocking | accepted | Renumbered O7 → O6 throughout. |
| F3 | significant | accepted | Added ADR-0014 row to `docs/adr/README.md` index (see separate edit). |
| F4 | significant | accepted | ADR-0014 uses hundredths-weighted scoring (user requirement). Template convention updated in `docs/adr/README.md`. |
| F5 | significant | accepted | Lowered O1 D2 from 7 to 4 to reflect current gap. Added explicit note that the score combines current capability with a forward commitment, and that the strategic claim (not the margin) justifies O1 over O6. |
| F6 | significant | accepted | Lowered O6 D2 from 5 to 3 to match the "orchestration-oblivious" description. |
| F7 | significant | accepted | Split Confirmation into two tiers: this-ADR confirmation and future-feature confirmation tracked under FEAT-0013. |
| F8 | advisory | accepted | Added `related:` frontmatter linking EXP-0010, FEAT-0009, FEAT-0013, ADR-0013, and the findings file. |
| F9 | advisory | accepted | Replaced exploration-duplicate open questions with decision-specific ones (port order, acceptance criteria, release timing, spec vs strategy). |
| F10 | advisory | accepted | Rewrote "Why O1 beats O6" to lead with the strategic claim (terminal as universal orchestration surface) rather than the numerical margin. |
