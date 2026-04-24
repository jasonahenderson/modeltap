---
exploration: EXP-0011
title: Upstream Feature Porting — Crush and OpenHarness Subsystem Analysis
status: exploring
date: 2026-04-22
related:
  - ADR-0014: Harness Base Strategy
  - EXP-0010: Harness Comparative Analysis
  - FEAT-0009: Terminal Harness
  - FEAT-0012: Skills
  - FEAT-0013: Agent Teams
  - FEAT-0010: Enterprise Policy & Multi-User
---

# EXP-0011: Upstream Feature Porting — Crush and OpenHarness Subsystem Analysis

## Context

ADR-0014 chose to **continue the in-tree modeltap harness** rather than fork OpenCode or OpenHarness. The winning option (O1) relies on selective porting of proven upstream subsystems into Go-based BFF/harness components. At the time ADR-0014 was written, OpenCode was archived (Sep 2025) and its successor **Crush** (`charmbracelet/crush`) was named but not deeply analyzed.

**Crush is now the primary upstream reference.** It is:
- **Active:** 23.3k stars, latest release v0.61.1 (Apr 21 2026), maintained by the Charm team
- **Go-native:** Same language and Bubbletea ecosystem as modeltap
- **Rich:** Multi-model, session-based, LSP-enhanced, MCP-extensible, skills-aware, auto-compacting
- **FSL-1.1-MIT licensed:** Functional Source License, not pure MIT

This exploration reframes the porting question around **Crush** (active Go) and **OpenHarness** (active Python). OpenCode is treated as Crush's historical predecessor — relevant only where Crush has not yet diverged.

## Why Crush Changes the Calculus

ADR-0014 scored O3 (soft fork OpenCode, track Crush) at **0.430**, the lowest of all options. That score was driven by two assumptions that are now questionable:

1. **"Crush may diverge architecturally from OpenCode"** — it did, but in ways that matter less than feared. Crush is still Go + Bubbletea + Charm. The architectural divergence is feature richness, not runtime mismatch.
2. **"Merge tax may exceed upstream benefit"** — with an active upstream in the same language, the merge tax is real but bounded. The tax is proportional to how much of Crush's self-contained agent loop must be gutted.

The decisive question is unchanged: **can Crush be inverted into a thin BFF client without gutting its value?** But because Crush is active and Go-native, the cost/benefit of forking vs. selective porting vs. design-reference shifts materially.

---

## Methodology

"Impactful" is scored against four criteria derived from ADR-0014 drivers:

| Criterion | Weight | Description |
|---|---|---|
| **Strategic fit (SF)** | 0.35 | Advances modeltap's core differentiators: proxy-centric control, multi-agent orchestration, cross-model memory |
| **Gap severity (GS)** | 0.30 | How badly the feature is missing today |
| **Porting feasibility (PF)** | 0.25 | Effort to extract the concept and reimplement in Go/BFF architecture (1 = years, 10 = weeks) |
| **Maintenance tax (MT)** | 0.10 | Ongoing cost if ported poorly or into the wrong layer (higher = worse) |

**Impact score** = weighted sum of SF, GS, PF (inverted), and MT (inverted).

---

## Crush Subsystems

### 1. Agent Loop & Session Engine (`internal/session`, `internal/app`)

**What it does:** Self-contained query → stream → tool-call → loop. SQLite-backed sessions with auto-compact at 95% context. Multi-model switching mid-session.

**Strategic fit (SF: 3):** The self-contained loop is the **anti-pattern** for modeltap. The BFF must own the agent loop. Porting this subsystem would be a mistake.

**Gap severity (GS: 10):** Modeltap has no working agent loop today (FEAT-0013 unimplemented, FEAT-0009 in design). This is the biggest gap.

**Porting feasibility (PF: 2):** Inverting the loop is hard. Crush's `internal/app` is deeply wired around direct LLM client calls, local SQLite, and in-process tool execution. Gutting it to delegate to a BFF is substantial architectural surgery.

**Maintenance tax (MT: 2):** If inverted, every upstream change to `internal/app` or the stream loop is a merge conflict.

**Impact score:** 0.35×3 + 0.30×10 + 0.25×(10−2) + 0.10×(10−2) = **7.25** — but the *direction* is negative. This subsystem is high-impact to **avoid**, not to port.

**Port scope:** None. Treat as **design reference** for what a good loop looks like, then build the BFF-native equivalent.

---

### 2. Streaming Markdown & TUX (`internal/tui`)

**What it does:** High-frequency token streaming in Bubbletea with debounced redraws, Glamour rendering, scrollable viewport, multi-line input, plan/build mode UI, spinner states, permission dialogs.

**Strategic fit (SF: 9):** Terminal UX quality is a daily-use differentiator (D5 in ADR-0014). Crush solves this at scale.

**Gap severity (GS: 9):** FEAT-0009 specifies this but it is unimplemented. The harness has basic Bubbletea scaffolding.

**Porting feasibility (PF: 7):** The TUI components are Bubbletea models. They can be extracted and rewired to consume BFF events instead of in-process LLM streams. The `tea.Msg` types need to change (Crush's `StreamMsg` → modeltap BFF event types), but the rendering logic is reusable.

**Maintenance tax (MT: 6):** If forked and inverted, upstream TUI improvements (new components, bug fixes) must be manually merged. Crush's TUI is actively evolving.

**Impact score:** 0.35×9 + 0.30×9 + 0.25×(10−7) + 0.10×(10−6) = **8.05**

**Port scope:** Primarily **harness**. The harness already uses Bubbletea; Crush's components can be adapted.

**Open question:** How much of Crush's TUI depends on in-process state (e.g., `internal/session` types) vs. pure rendering? If the TUI models are tightly coupled to the session engine, extraction is harder.

---

### 3. Auto-Compact & Context Management

**What it does:** Monitors token usage, triggers summarization at 95%, creates a compacted session continuation.

**Strategic fit (SF: 8):** Cross-session memory and context health are core differentiators (EXP-0001, ADR-0008).

**Gap severity (GS: 8):** Not implemented. FEAT-0009's `/compact` is specified but not built.

**Porting feasibility (PF: 7):** The threshold heuristic (95%) and summarization trigger are standalone logic. The summarization itself must route through the BFF (harness never calls LLMs directly), but the state machine (monitor → trigger → summarize → resume) is portable.

**Maintenance tax (MT: 7):** Low if reimplemented. Crush's auto-compact is ~100 lines of state logic.

**Impact score:** 0.35×8 + 0.30×8 + 0.25×(10−7) + 0.10×(10−7) = **7.70**

**Port scope:** BFF owns summarization (model call); harness owns threshold monitoring and UI.

---

### 4. Skills Framework (`.agents/skills`, `SKILL.md`)

**What it does:** Discovers `SKILL.md` files from disk (`anthropics/skills` compatible), loads them on demand, and injects them as prompt overlays. Also auto-discovers custom paths.

**Strategic fit (SF: 8):** Directly enables FEAT-0012. Lowers barrier to adoption by leveraging the emerging `anthropics/skills` ecosystem.

**Gap severity (GS: 8):** Skills are entirely unimplemented.

**Porting feasibility (PF: 8):** Straightforward: a file walker, markdown parser, skill registry, and prompt composition hook. Crush's implementation is ~300 lines of file I/O + a map overlay.

**Maintenance tax (MT: 8):** Go-native. No upstream tax if reimplemented.

**Impact score:** 0.35×8 + 0.30×8 + 0.25×(10−8) + 0.10×(10−8) = **7.70**

**Port scope:** BFF owns skill resolution and prompt composition. Harness owns file watching and `/skill` command UI.

**Notable:** Crush searches for skills in `.agents/skills`, `.crush/skills`, `.claude/skills`, `.cursor/skills`. Modeltap should add `.modeltap/skills` to this search path for ecosystem compatibility.

---

### 5. Multi-Model Provider System (`internal/llm`, Catwalk database)

**What it does:** Pluggable provider architecture with a community-maintained model database (Catwalk). Supports OpenAI-compatible, Anthropic-compatible, and custom APIs. Auto-updates provider definitions from remote.

**Strategic fit (SF: 4):** Modeltap's BFF already has its own provider layer (ADR-0006). Crush's provider system is client-side, which conflicts with proxy-centric control.

**Gap severity (GS: 5):** The BFF has provider adapters; the harness does not need its own.

**Porting feasibility (PF: 3):** The provider abstractions are deeply tied to direct HTTP calls. Inverting them to BFF delegation is not a port — it's a rewrite.

**Maintenance tax (MT: 3):** High if attempted.

**Impact score:** 0.35×4 + 0.30×5 + 0.25×(10−3) + 0.10×(10−3) = **5.35**

**Recommendation:** Do not port. Use Catwalk's **model metadata schema** as a reference for the BFF's provider registry, not the provider implementation itself.

---

### 6. LSP Integration

**What it does:** Multi-language LSP client (diagnostics, completions, hover, definitions). Currently exposes diagnostics to the AI; full LSP protocol supported internally.

**Strategic fit (SF: 5):** Code intelligence improves tool quality but is not a proxy/orchestration differentiator.

**Gap severity (GS: 6):** No LSP integration today. Tools use `Read`, `Grep`, `Glob`.

**Porting feasibility (PF: 4):** LSP is a large surface. Crush's `internal/lsp` is a full Go LSP client. Extracting it is possible but it assumes local file access and in-process tool calls.

**Maintenance tax (MT: 4):** LSP client libraries evolve. If reimplemented, the tax is maintaining LSP state machines per language.

**Impact score:** 0.35×5 + 0.30×6 + 0.25×(10−4) + 0.10×(10−4) = **5.95**

**Recommendation:** **Defer.** Not a top gap. The BFF's tool framework can add an LSP tool later without architectural changes.

---

### 7. Session Explorer & SQLite Management

**What it does:** SQLite-backed session search, filtering, and navigation. Sessions are scoped to projects, support metadata tags, and are queryable.

**Strategic fit (SF: 6):** Session management is a BFF concern in modeltap. Crush's implementation is client-local.

**Gap severity (GS: 7):** The harness's session explorer (FEAT-0009) is specified but unbuilt.

**Porting feasibility (PF: 6):** The UI pattern (list, details, date grouping) is portable. The storage layer must be replaced with BFF queries. The SQLite schema is informative for designing the BFF's session tables.

**Maintenance tax (MT: 6):** Moderate. Schema drift if Crush changes its storage model.

**Impact score:** 0.35×6 + 0.30×7 + 0.25×(10−6) + 0.10×(10−6) = **6.60**

**Recommendation:** Treat as **schema reference** for the BFF's session tables and **UI reference** for the harness's session explorer. Do not port the storage layer.

---

### 8. Built-in Tools & Permission Model

**What it does:** ~12 built-in tools (glob, grep, ls, view, write, edit, patch, bash, fetch, diagnostics, agent, custom commands). Permission dialog UI with allow/deny/session-allow modes.

**Strategic fit (SF: 6):** Tool breadth improves UX but is not proxy-centric. MCP is the preferred extensibility model.

**Gap severity (GS: 7):** 13 built-in tools vs. Crush's ~12. The gap is narrow — the tooling is roughly equivalent.

**Porting feasibility (PF: 5):** Individual tools are extractable. The permission UI (`PermissionDialog` Bubbletea model) is a reusable component.

**Maintenance tax (MT: 5):** If reimplemented, low. If forked, merge tax on permission flow changes.

**Impact score:** 0.35×6 + 0.30×7 + 0.25×(10−5) + 0.10×(10−5) = **6.55**

**Recommendation:** Do not port tool implementations — modeltap's tool set is already specified in FEAT-0009. Port the **permission dialog UI pattern** as a Bubbletea component reference.

---

## OpenHarness Subsystems (Revised Position)

Because Crush is active and Go-native, OpenHarness's role shrinks. It is still the richer agent runtime overall (43+ tools, `coordinator/`, chat gateways), but its Python/Node.js runtime makes it a weaker porting candidate than Crush for most features.

| Subsystem | Crush Equivalent | OpenHarness Advantage | Recommendation |
|---|---|---|---|
| Agent loop / sessions | Full (but self-contained) | None (Python) | Use neither; build BFF-native |
| TUI / streaming | Full (Bubbletea) | None (React/Ink) | Crush is primary reference |
| Auto-compact | Full | None | Crush is primary reference |
| Skills | Full (`anthropics/skills` compat) | None | Crush is primary reference |
| Multi-model providers | Full (Catwalk) | Profiles | Use BFF-native; reference Catwalk schema |
| LSP | Partial (diagnostics) | None | Defer both |
| `coordinator/` / swarm | None | Best-in-class | **OpenHarness is the reference** for FEAT-0013 |
| `memory/` / context compression | Auto-compact only | Richer (MEMORY.md) | **OpenHarness is reference** for memory architecture |
| `permissions/` / governance | Basic (allow/deny/session) | Multi-level + path rules | **OpenHarness is reference** for advanced governance |
| Chat gateways (`ohmo`) | None | Slack, Telegram, Discord | Defer; OpenHarness is reference |
| Notebook / task scheduling | None | Yes | Defer; OpenHarness is reference |

**Crush covers TUI, session UX, skills, and auto-compact. OpenHarness covers orchestration, advanced memory, and governance.**

---

## The Fork Question Revisited

ADR-0014 ruled out forking because:
- OpenCode was archived (O2, O3)
- OpenHarness was Python/Node.js (O4, O5)

**Crush introduces a new option not scored in ADR-0014:**

### O7 — Fork Crush (hard fork, active upstream)

Fork `charmbracelet/crush`, then invert its self-contained loop into a thin BFF client. Because Crush is active, a hard fork means diverging from upstream immediately and cherry-picking TUI improvements manually.

**Hypothetical ADR-0014 scores:**

| Driver | O1 (modeltap) | O2 (hard OpenCode) | O7 (hard Crush) |
|---|---|---|---|
| D1 Proxy-centric | 9 | 3 | **3** |
| D2 Multi-agent | 4 | 2 | **3** (Crush has no swarm) |
| D3 Language | 9 | 9 | **9** |
| D4 Maintainability | 9 | 2 | **4** (active upstream, but diverging) |
| D5 Terminal UX | 7 | 8 | **9** (best Bubbletea UX in class) |
| D6 Single binary | 9 | 9 | **9** |
| D7 Feature richness | 4 | 6 | **7** (skills, auto-compact, LSP) |

**Weighted total:** ~0.600. Higher than O2/O3/O4/O5, but still below O1 (0.730) and O6 (0.705).

**Why O7 still loses to O1:** The architectural inversion cost is identical to O2/O3. Crush may be active, but it is still a self-contained agent loop. Gutting `internal/app`, `internal/llm`, and `internal/session` to delegate to a BFF is the same fundamental surgery. The active upstream only helps on TUI bug fixes and component patterns — not on the core inversion.

**Conclusion from this exploration:** ADR-0014's O1 decision remains valid. Crush does not change the outcome. It changes the **quality of the design references** and the **credibility of selective porting.**

---

## License Consideration

Crush is licensed under **FSL-1.1-MIT** (Functional Source License), not MIT.

- **Before 2 years:** FSL applies. Commercial use, modification, and distribution are allowed. Competition with the licensor's commercial offering is restricted.
- **After 2 years:** Automatically converts to MIT.

Modeltap is not a competing product to Crush (Crush is a self-contained coding agent; modeltap is a proxy/orchestration infrastructure). The FSL restriction likely does not apply. However, legal review is warranted before incorporating Crush-derived code or forking.

**Recommendation:** Use Crush as a **design reference** and **concept source**, not a code fork base. If code-level borrowing occurs (e.g., a Bubbletea debounce heuristic), treat it as a clean-room reimplementation inspired by public documentation, not a code copy.

---

## Prioritization Summary (Reframed)

| Rank | Upstream Feature | Source | Impact Score | Port Scope | Complexity | Recommendation |
|---|---|---|---|---|---|---|
| 1 | `coordinator/` swarm logic | OpenHarness | 8.70 | BFF | High | **Port first.** Unlocks FEAT-0013. |
| 2 | Streaming TUX (debounce, viewport, dialogs) | Crush | 8.05 | Harness | Medium | **Design reference + component patterns.** Implement during FEAT-0009. |
| 3 | Auto-compact / context health | Crush | 7.70 | BFF + Harness | Medium | **Port third.** Reimplement threshold logic in BFF. |
| 4 | Skills framework (`SKILL.md`) | Crush | 7.70 | BFF | Low | **Port fourth.** Add `.modeltap/skills` to search path. |
| 5 | `permissions/` governance | OpenHarness | 7.15 | BFF + Harness | Medium | **Port fifth.** Path rules, deny lists, hooks. |
| 6 | Session explorer UI pattern | Crush | 6.60 | Harness | Medium | **UI reference only.** Storage is BFF-side. |
| 7 | Built-in tools | Both | 6.55 | Harness | Low | Do not port. FEAT-0009 already specified. |
| 8 | Chat gateways | OpenHarness | 5.85 | BFF API | Medium | **Defer.** Design BFF for async clients. |
| 9 | LSP integration | Crush | 5.95 | Harness | High | **Defer.** Non-critical gap. |
| 10 | Provider system (Catwalk) | Crush | 5.35 | — | High | Do not port. BFF owns providers. Reference schema only. |
| — | Agent loop inversion | Crush | 7.25 (negative) | — | Very High | **Avoid.** Build BFF-native instead. |

---

## What "Porting" Means Here

Because neither upstream project shares modeltap's thin-client architecture, "porting" is **concept extraction + Go reimplementation**, not code translation.

Guidelines:
- **No code copy from FSL-licensed Crush.** Clean-room reimplementation from public docs.
- **No agent loop in the harness.** LLM client lives in BFF.
- **No Python runtime in the binary.** OpenHarness concepts become Go BFF subsystems.
- **MCP over built-in where possible.** Extensibility belongs in the BFF or MCP layer.
- **Attribute design debt.** Comment design lineage: `// Design derived from OpenHarness coordinator/`.

---

## Risk: Over-Indexing on Upstream

The biggest risk is treating Crush's feature set as the target. Crush is a self-contained coding agent. Modeltap is a proxy/orchestration layer. Every feature must be evaluated against proxy-centric control, not feature parity.

**Anti-patterns to avoid:**
- Building a self-contained agent loop in the harness because Crush has one.
- Adding client-side provider routing because Crush has Catwalk.
- Storing sessions in local SQLite because Crush does.
- Implementing chat gateways in the harness because OpenHarness has `ohmo`.

The BFF is the agent. The harness is the viewport. Crush and OpenHarness are design references for how to make that viewport and agent rich — not blueprints for their architecture.

---

## Relationship to Release Planning

| Upstream Subsystem | Source | Maps To | Phase |
|---|---|---|---|
| `coordinator/` | OpenHarness | FEAT-0013 (Agent Teams) | Phase 1 design, Phase 3 |
| Streaming TUX | Crush | FEAT-0009 (Terminal Harness) | Phase 3 (pattern reference) |
| Auto-compact | Crush | FEAT-0009 (Compaction) | Phase 1 design, Phase 3 |
| Skills | Crush | FEAT-0012 (Skills) | Phase 1 design, Phase 3 |
| Permissions | OpenHarness | FEAT-0010 (Enterprise) + FEAT-0009 | Phase 1 design, Phase 3 |
| Session explorer | Crush | FEAT-0009 (Session Explorer) | Phase 3 (UI reference) |
| Notebook / search | OpenHarness | New feature (FEAT-0014+) | Future release |
| Chat gateways | OpenHarness | New feature (FEAT-0015+) | Future release |

---

## Open Questions

1. **Crush FSL license:** Does modeltap's proxy/orchestration model constitute "competition" under FSL-1.1? If so, a fork is legally prohibited before the 2-year MIT conversion.
2. **TUI extraction feasibility:** How tightly coupled are Crush's `internal/tui` components to `internal/session` types? Can the viewport and permission dialog be extracted as pure rendering components?
3. **Catwalk schema reuse:** Can modeltap's BFF provider registry import or mirror Catwalk's model metadata format without importing Crush code?
4. **Auto-compact BFF contract:** Should the harness monitor token usage and request compaction, or should the BFF push compaction suggestions? Crush does it client-side; modeltap's BFF owns the model.
5. **Crush skills vs. FEAT-0012:** Crush supports `anthropics/skills` and custom paths. FEAT-0012 specifies a skill overlay system. Are these compatible, or does modeltap need a superset format?
6. **Does Crush's active status justify a new ADR amendment?** ADR-0014 was decided with incomplete Crush data. The outcome (O1) is unchanged, but the upstream landscape has shifted. Should the ADR be updated to name Crush as the active upstream reference, or is this exploration sufficient?

---

## Proposed Next Steps

1. **Land this exploration** as the Crush-reframed upstream rationale.
2. **Legal check on FSL:** Verify that design-reference and clean-room reimplementation are permissible under FSL-1.1-MIT before any code-level borrowing.
3. **Before FEAT-0013 Phase 1:** Read OpenHarness `coordinator/` source. Study Crush's session/model-switching UI for harness rendering patterns.
4. **Before FEAT-0009 Phase 3:** Study Crush `internal/tui` for the streaming debounce heuristic and permission dialog pattern. Document as references in the implementation history.
5. **Do not treat any upstream as a fork base.** Continue with O1: in-tree harness, selective porting of concepts, BFF-native agent loop.

(End of document)
