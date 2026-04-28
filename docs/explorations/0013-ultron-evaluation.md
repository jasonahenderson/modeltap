---
exploration: EXP-0013
title: Ultron Evaluation — Tiered Memory and Skill Evolution from ModelScope
status: watching
date: 2026-04-28
related:
  - EXP-0001: Knowledge Layer (Cross-Model Brain)
  - EXP-0008: Integrated Harness
  - EXP-0009: Harness Prompt Architecture
  - EXP-0011: Harness Excellence Gap Analysis
  - EXP-0012: Code Graphing via AST for Repository-Aware Context
  - ADR-0008: Knowledge layer architecture
  - ADR-0009: MCP server for knowledge access
  - ADR-0010: License (Apache 2.0)
  - FEAT-0011: Knowledge Integration
  - FEAT-0012: Skills and Agent Teams
  - FEAT-0013: Agent Teams
  - FEAT-0014: Harness Conversation Shell
---

# EXP-0013: Ultron Evaluation — Tiered Memory and Skill Evolution from ModelScope

## Context

[Ultron][ultron-repo] is a self-evolving collective-intelligence service
released in early April 2026 by ModelScope (Alibaba's open model hub). The
pitch: capture agent session trajectories, crystallize them into tiered
decay-weighted memories, distill those memories into reusable filesystem
skills (`SKILL.md` + metadata + scripts), and let agents share a "harness"
profile so a fleet learns once instead of repeating mistakes.

That problem space overlaps three modeltap artifacts directly:

- EXP-0001 / ADR-0008 frame modeltap as a cross-model knowledge brain.
- EXP-0008 / EXP-0011 frame modeltap as a professional integrated harness.
- FEAT-0011 (Knowledge Integration), FEAT-0012 (Skills and Agent Teams), and
  FEAT-0013 (Agent Teams) define the in-flight scope where this overlap
  lives.

The motivating question for this exploration is **whether Ultron is
something modeltap should integrate, mine for design ideas, or note as
prior art and move on**. Ultron is Apache-2.0 (compatible with ADR-0010),
ships as a Python FastAPI server with no MCP surface and no Go SDK, and is
roughly three weeks old at time of writing — small, opinionated, and
unstable, but pointed at a problem we already have on the roadmap.

[ultron-repo]: https://github.com/modelscope/ultron

## Problem / Motivating Question

Modeltap's current and planned surface already includes:

- session/trajectory capture (existing — proxy core)
- a conversation-centric knowledge layer with sqlite-vec (ADR-0008)
- MCP exposure of that layer (ADR-0009)
- skills and agent teams as features in flight (FEAT-0012, FEAT-0013)
- a conversation shell as the harness front-end (FEAT-0014)

Ultron addresses the same arc — capture → memory → skill → harness — but
in Python, behind HTTP, with its own embedding stack. So:

1. Is there a load-bearing idea in Ultron that we should adopt directly
   in our Go-native design (e.g. tiered L0/L1/Full memory with hotness
   decay; structure-score-gated skill evolution)?
2. Is there an interop contract worth honoring (e.g. the `SKILL.md`
   filesystem layout, so modeltap-authored skills can be consumed by
   Ultron and vice versa)?
3. Does it ever make sense to run Ultron as a sidecar that modeltap
   proxies — or does that violate the single-binary, MCP-native posture
   ADR-0009 commits us to?

## Ultron in One Page

(See research notes 2026-04-28 for the long-form version.)

- **Shape:** Python FastAPI server (`0.0.0.0:9999`) plus an in-process
  Python SDK and a bundled TypeScript dashboard SPA. HTTP-only — no MCP,
  no gRPC, no OpenAPI publication beyond FastAPI's auto-generated
  `/openapi.json`.
- **Storage:** single SQLite database via mixin classes (memory, skill,
  harness, cluster, trajectory, SFT-training, user, ingestion, catalog).
  Skills additionally live on the filesystem as
  `{slug}-{version}/SKILL.md + _meta.json + scripts/`. No dedicated vector
  store — embeddings live in SQLite.
- **Embeddings / LLM:** DashScope `text-embedding-v4` and Qwen via an
  OpenAI-compatible client by default; `sentence-transformers` and
  `transformers` available as a local fallback.
- **Four "Hubs":**
  - **Trajectory Hub** — segmentation and quality scoring of captured
    sessions.
  - **Memory Hub** — tiered memory with `L0 / L1 / Full` summaries, hotness
    decay `exp(-α·days)`, dedup, auto-classification by an LLM (callers
    don't tag memory type), intent-expanded retrieval.
  - **Skill Hub** — re-crystallizes memory clusters into skills, with a
    "structure score" gate so an evolved skill can't regress; integrates
    with ModelScope's external 82K-skill catalog.
  - **Harness Hub** — sync/share profiles, "soul presets" keyed off
    role/MBTI/zodiac (yes, really).
- **Auth:** PyJWT + bcrypt; Bearer required on harness routes; memory and
  skill routes default to no auth. CORS open. PII handled via Microsoft
  Presidio (EN/ZH).
- **License:** Apache-2.0 (compatible with ADR-0010).
- **Maturity:** created 2026-04-09; ~20 commits, 65 stars, 0 published
  releases, 2 open issues at time of writing. Pre-alpha, APIs unstable.
- **Research roots:** SkillClaw (skill evolution methodology), ZClawBench
  (696-trajectory eval set), WildClawLMCache (147-trajectory eval set).
  No formal whitepaper; theory of operation lives only in
  `docs/en/Components/*.md`.

The notable design choices worth carrying forward independently of Ultron
the codebase: **tiered summary memory with explicit hotness decay**;
**LLM-classified memory type**; **structure-score-gated skill evolution
with provenance**; **filesystem-first skill format**.

## Where Ultron Overlaps Modeltap

| Modeltap concept | Modeltap home | Ultron concept |
|---|---|---|
| Capture proxy | core | Trajectory Hub |
| Knowledge layer | ADR-0008, FEAT-0011 | Memory Hub |
| Skills / agent teams | FEAT-0012, FEAT-0013 | Skill Hub |
| Harness profiles | FEAT-0014, EXP-0008 | Harness Hub |
| Cross-model brain | EXP-0001 | Collective intelligence |

Ultron is not aimed at the "professional terminal coding harness" target
that EXP-0008 / EXP-0011 set for modeltap. It is aimed at a fleet of
general-purpose Python agents sharing learned behavior. The overlap is in
substrate, not product.

## Design Space / Options

### Option A — Ignore (note as prior art)

Cite Ultron in EXP-0001 and FEAT-0012 as related work and otherwise let
modeltap's own designs proceed. Cost: zero. Risk: we re-invent two pieces
(tiered decay memory; structure-score skill evolution) that Ultron has
already pressure-tested in a similar SQLite-shaped environment.

### Option B — Mine for design (selective adoption)

Treat Ultron as a reference design for specific subsystems. Carry
forward, in our Go code, the parts that fit modeltap's posture:

1. **L0/L1/Full tiered summaries with hotness decay** as an extension to
   ADR-0008's knowledge schema (FEAT-0011 territory).
2. **Auto-classified memory type** via the LLM rather than caller-supplied
   tags — reduces caller ceremony and matches modeltap's "capture
   everything, classify later" stance (ADR-0005).
3. **Structure-score-gated skill evolution with provenance** as the
   evolution loop in FEAT-0012.
4. **Intent-expanded retrieval** as a query-side augmentation to
   sqlite-vec searches.

This is the highest-leverage option per unit of risk: we adopt the ideas
without taking on a Python runtime dependency or a pre-alpha API surface.

### Option C — Honor the `SKILL.md` filesystem layout

Independent of any code adoption, decide whether modeltap's skill format
should be **wire-compatible** with Ultron's
`{slug}-{version}/SKILL.md + _meta.json + scripts/`. If FEAT-0012 picks the
same on-disk shape, modeltap-authored skills are portable to and from any
Ultron deployment without translation. The cost is a constraint on
FEAT-0012's schema; the benefit is one more interop seam that fits
EXP-0001's "cross-model brain" narrative.

This is decoupled from Option B: we can mine the design *and* match the
filesystem layout, or pick one or neither.

### Option D — Run Ultron as a sidecar; modeltap proxies / wraps it

Stand Ultron up as a daemon on `:9999` and have modeltap call its
HTTP endpoints (`/memory/*`, `/skills/*`, `/harness/*`, `/ingest`) from
Go. Modeltap could even expose Ultron's surface over MCP, closing the
"no MCP" gap upstream punts on.

Costs are real:

- Adds a Python runtime to a project committed to single-binary Go
  distribution.
- Adopts a pre-alpha, unversioned HTTP API as a hard dependency.
- Pulls in DashScope/Qwen embedding defaults that conflict with
  modeltap's MLX/Ollama-first local stance.
- Splits state across two SQLite stores (Ultron's and modeltap's).

This option is unattractive for the modeltap core, but plausible as an
**optional MCP integration** packaged separately — for users who already
run Ultron and want modeltap to read/write its memory hub.

### Option E — Vendor and re-implement in Go

Read Ultron's schema, services, and prompt scaffolding; rewrite in Go
under modeltap's existing storage and config layout. Net effect is
similar to Option B but with stronger fidelity to Ultron's specific
shapes. Cost is the labor of a clean-room port.

The line between B and E is fuzzy in practice; the practical question is
whether we want to carry Ultron's specific schema names and LLM prompts,
or just the conceptual moves.

## Tensions and Tradeoffs

- **Substrate alignment vs. runtime alignment.** Ultron picked SQLite, the
  same store ADR-0002 picked. That makes design ideas portable. It does
  not make the *runtime* portable — Python plus FastAPI plus the
  DashScope embedding chain is a heavier hard dependency than modeltap
  should accept in its core.
- **Maturity vs. timing.** A three-week-old project with no releases is
  too unstable to depend on as an upstream API. It is not too unstable to
  read for design moves we lock in ourselves.
- **Single-binary purity vs. ecosystem reuse.** Same tension as EXP-0012
  with codebase-memory-mcp. Options B/C respect the single-binary stance;
  Option D explicitly violates it and should live outside the core if it
  ships at all.
- **Scope creep into "agent fleet" territory.** Ultron is sized for a
  fleet of long-lived agents sharing memory. EXP-0011 deliberately scopes
  modeltap to a professional *coding* harness first. Borrowing too much of
  Ultron's framing risks pulling FEAT-0012 toward a general-purpose agent
  marketplace before the coding-harness story is solid.
- **Prompt-architecture compatibility.** EXP-0009 took strong positions on
  prompt structure for a coding harness. Ultron's "soul presets"
  (MBTI/zodiac persona injection) are aesthetically opposite to EXP-0009's
  recommendations. If we adopt Ultron's harness format, we should not
  adopt that part of it.
- **Embedding stack drift.** Ultron defaults to DashScope. Modeltap's user
  memory and EXP-0008 commit to MLX / Ollama (incl. Ollama Cloud) as the
  priority local backends. Adopting Ultron's *interfaces* is fine; its
  *defaults* are not.

## Open Questions

1. Are L0/L1/Full tiered summaries strictly better than the
   single-resolution captures ADR-0008 currently implies, *for coding-harness
   workloads*? Tiered memory is plainly useful for general agents; for a
   coding harness, the answer depends on whether typical recalls want
   "the gist of the last debug session" or "the exact diff context."
2. Does FEAT-0012's skill schema benefit from filesystem-portability with
   Ultron, or does that constrain a more modeltap-specific shape (e.g. one
   that integrates with our agent-team contracts)?
3. Is Ultron's "structure score" criterion well-defined enough to adopt,
   or is it a heuristic we'd need to redesign? (Their docs describe it
   only at the README level.)
4. Where does PII handling live in modeltap? Ultron uses Presidio; ADR-0008
   currently leaves PII as an open question. Tiered memory makes the
   question more pressing because L1/Full summaries can re-encode raw PII
   from L0.
5. Is there enough demand for a modeltap-as-MCP-front-for-Ultron bridge
   (Option D) that it justifies a separate optional integration, or is
   that effort better spent on modeltap-native memory work?

## Proposed Next Step

Two-step, low-commitment:

1. **Read pass (~half day):** read `docs/en/Components/{TrajectoryHub,
   MemoryHub,SkillHub,HarnessHub}.md`, the SDK doc, and the schema
   mixins in `ultron/core/db_*.py` directly. Produce a short addendum to
   this exploration capturing:
   - the exact tiered-memory schema and decay parameters,
   - the "structure score" definition,
   - the `SKILL.md` + `_meta.json` field set,
   - the intent-expansion prompt(s).
2. **Decision (single round):** based on that addendum, decide between
   Options A, B, C, and a possible "B + C" combination. Do **not**
   pursue Option D in this round. If we choose B or B+C, fold the
   adopted ideas into FEAT-0011 and FEAT-0012 directly rather than
   creating a new feature — Ultron is design input, not a product.

If the decision is Option A, close this exploration as `closed`. If it
yields concrete additions to FEAT-0011 / FEAT-0012, mark this exploration
`promoted` and link the resulting feature revisions.
