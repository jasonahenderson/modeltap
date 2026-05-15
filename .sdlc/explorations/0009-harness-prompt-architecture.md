---
exploration: EXP-0009
title: Harness Prompt Architecture — Lessons from the Claude Code Leak
status: exploring
date: 2026-04-17
related:
  - EXP-0008: Integrated Harness — Modeltap as Professional AI Environment
  - EXP-0007: Multi-Model Orchestration
  - FEAT-0009: Terminal Harness
  - FEAT-0012: Skills
  - FEAT-0013: Agent Teams
---

# EXP-0009: Harness Prompt Architecture — Lessons from the Claude Code Leak

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Context

In March–April 2026, the Claude Code npm package (`@anthropic-ai/claude-code` 2.1.88) shipped source maps that were quickly reverse-engineered and published. Several independent analyses (ghuntley, Kir Shatrov, Piebald-AI, dbreunig, alex000kim) describe the internal architecture in enough detail to be used as a reference. EXP-0008 already cites Claude Code as architectural inspiration at a high level, but the leak exposes specific composition and hardening patterns that are not yet captured anywhere in Modeltap's design corpus.

This exploration catalogues the two patterns that are most relevant to Modeltap and not obviously covered by existing accepted artifacts: **prompt layering** (how a harness assembles the prompt each turn from multiple overlay sources) and **traffic hardening** (how a harness defends against the captured-traffic threat model — a model that applies to Modeltap a fortiori, since it *is* the capture surface).

FEAT-0009 specifies the terminal harness, FEAT-0012 specifies skills, FEAT-0013 specifies agent teams. Each touches pieces of these patterns, but none of them names the composition contract or the hardening posture explicitly. That gap is what this exploration exists to name.

## Problem

### Prompt Composition Is Currently Implicit

FEAT-0012 says skills "shape one turn." FEAT-0013 says agent teams spawn subagents with fresh context. FEAT-0009 says the harness streams conversation turns to the server. None of these say:

- What the **base system prompt** contains, and who owns it (harness, server, or per-skill).
- How **project context** (a CLAUDE.md-equivalent) is loaded, scoped, and versioned.
- How **skill overlays** compose with the base prompt — append, replace, or structured merge.
- How **subagent prompts** inherit from or override the parent's prompt layers.
- How **memory** (short-term session memory, long-term consolidated memory) is injected into the prompt without context explosion.
- How **tool descriptions** are assembled and scoped per-turn (all tools? skill-filtered? subagent-restricted?).

Claude Code's leaked implementation has concrete answers for each of these. Modeltap does not, and without an explicit composition contract the three features above will drift — each implementation will invent its own assembly order, scoping rules, and precedence semantics.

### The Capture-Traffic Threat Model Is Unaddressed

Modeltap is a reverse proxy that captures full request/response bodies (ADR-0005). This is the product's core value and its core risk. Anyone who can read a Modeltap capture store — an insider, a backup leak, a compromised team deployment — can reconstruct prompts, tool definitions, conversation histories, and model outputs in enough detail to:

- Distill a smaller model against the captured responses.
- Extract proprietary prompts and agent designs from captured system prompts.
- Harvest secrets and PII that leaked into prompts despite redaction.
- Fingerprint a user's workflow and replay it.

The Claude Code leak reveals that Anthropic's client actively defends against this: the `anti_distillation: ['fake_tools']` flag asks the API server to inject decoy tool definitions into the prompt, poisoning any distillation attempt that consumes captured traffic.

Modeltap's current design corpus assumes capture is safe because it is local (solo profile) or authenticated (team/enterprise). That assumption is necessary but not sufficient. Capture is a long-lived artifact; local today is backed-up-to-cloud tomorrow. The harness and server need a defensible story about what captured traffic can and cannot be used for, and ideally a first-class feature that lets users *opt in* to traffic hardening the way Claude Code does by default.

## Design Space

### Layered Prompt Composition

Four overlay sources, each with a clear owner and scope:

1. **Base system prompt** — owned by the server, versioned alongside the BFF. Contains the harness's identity, tool-use conventions, safety rules, output format. One per harness version; not user-editable.
2. **Project context** — a per-project file (working name: `MODELTAP.md` or `.modeltap/context.md`) loaded when the harness starts in a given directory. Scoped to the current workspace. User-editable. Analogous to `CLAUDE.md`.
3. **Skill overlay** — per FEAT-0012, a prompt fragment + narrowed tool set activated for one turn. Composes *above* the base prompt and project context.
4. **Subagent prompt** — per FEAT-0013, a fully-specified prompt used to launch an isolated agent loop. Inherits base + project by default; can override skill overlays.

Composition order (top to bottom of the final prompt):

```
[base system prompt]                 ← server-owned, versioned
[project context]                    ← workspace-scoped, user-owned
[memory digest]                      ← short-term + relevant long-term, bounded size
[skill overlay, if active]           ← one-turn, harness-injected
[tool descriptions]                  ← filtered by skill/subagent scope
[conversation history]
[current user turn]
```

Subagents start a fresh loop with the same structure but typically with a narrower tool set and a replaced skill overlay (the subagent's own prompt becomes the overlay).

### Memory Without Context Explosion

Three tiers, drawn from the Claude Code three-layer memory pattern and EXP-0001's knowledge layer:

1. **Session memory** — in-process, scoped to one harness session. Cleared on exit.
2. **Project memory** — durable, scoped to the workspace (file-backed, like `.modeltap/memory/`).
3. **Global/cross-model memory** — the knowledge layer from EXP-0001, retrieved by relevance.

Only a digest is injected into the prompt each turn. Retrieval ranking and consolidation run server-side during idle time (analogous to autoDream). Consolidation is where EXP-0001's knowledge layer meets the harness: captured conversations feed memory, memory feeds future prompts.

### Traffic Hardening Options

Three hardening levels the user could select per workspace:

1. **Standard** (default) — full capture, standard redaction (existing behaviour).
2. **Fake-tools** — analogous to Claude Code's anti-distillation flag. The server injects decoy tool definitions into the outbound prompt and strips them from what it stores, so captured traffic cannot be trivially replayed or distilled against real tools.
3. **Opaque-capture** — captures are stored encrypted-at-rest with a key not available to the capture store itself (held in a separate secrets service), so a capture-store compromise alone does not yield readable prompts.

These are not mutually exclusive. Standard + Opaque-capture is a reasonable enterprise default. Fake-tools is a defense against distillation specifically and matters most when prompts contain proprietary tool schemas.

### Alternative: Do Nothing And Let Features Converge

The cheaper option: accept that FEAT-0009, FEAT-0012, FEAT-0013 will each invent their own answers and refactor later when the inconsistencies become expensive. This is what usually happens in practice and is occasionally the right call, but it means the peer review in Phase 2 will have no canonical reference to check composition rules against, and any third-party skill/subagent contribution will hit the same ambiguity.

## Tensions and Tradeoffs

- **Explicit composition contract vs. feature independence**: naming the composition order now constrains all three features (0009, 0012, 0013). That constraint is the point — but it means this exploration needs to be promoted before or alongside those features' Phase 1 designs, not after.
- **Traffic hardening vs. observability**: the whole point of Modeltap is to make prompts and responses inspectable. Fake-tools and opaque-capture erode that by design. The hardening features are opt-in for a reason — but they conflict with the metrics/aggregation features (ADR-0007) if enabled naively. Someone needs to decide what aggregations are still computable over opaque captures.
- **Anti-distillation as feature vs. as differentiator**: offering anti-distillation to users reframes Modeltap from "captures everything" to "captures everything *safely*." That is a genuine enterprise differentiator, but it complicates the product story.
- **CLAUDE.md-analogue name collision**: calling it `MODELTAP.md` invites users to commit it, which is usually what they want, but sometimes it contains secrets. The naming + default-gitignore story needs thought.

## Open Questions

1. Should the prompt-composition contract live in a new ADR, or be folded into FEAT-0009's design?
2. Should traffic hardening be a single feature or split (opaque-capture is arguably an enterprise auth / storage concern, FEAT-0010; fake-tools is arguably a provider-adapter concern, ADR-0006)?
3. Does the fake-tools approach require provider-adapter cooperation, or can it be done purely at the BFF layer?
4. Is there a version of memory injection that does not require a custom retrieval step per turn (e.g., pre-computed per-session digest refreshed on idle)?
5. How does skill/subagent prompt layering interact with orchestration (EXP-0007) when a single user turn fans out to multiple providers with different prompt formats?

## Proposed Next Step

1. Land this exploration as upstream rationale.
2. In Phase 2 peer review of v0.2.0, flag FEAT-0009 / FEAT-0012 / FEAT-0013 for a cross-cutting consistency check against this exploration's composition contract.
3. After Phase 2, decide whether to:
   - Promote the composition contract to an ADR (if the decision becomes constraining and hard to reverse), and/or
   - Promote traffic hardening to a future feature spec, separately from the composition work.
4. Do **not** treat this exploration as implementation authorization. No code changes until the downstream artifact exists.
