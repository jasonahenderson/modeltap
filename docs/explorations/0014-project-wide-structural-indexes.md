---
exploration: EXP-0014
title: Project-Wide Structural Indexes Beyond Code
status: exploring
date: 2026-05-05
related:
  - EXP-0001: Knowledge Layer (Cross-Model Brain)
  - EXP-0008: Integrated Harness — Modeltap as Professional AI Environment
  - EXP-0011: Harness Excellence Gap Analysis
  - EXP-0012: Code Graphing via AST for Repository-Aware Context
  - ADR-0008: Knowledge layer architecture
  - ADR-0009: MCP server for knowledge access
  - FEAT-0016: Managed Codegen Run Pipeline
  - FEAT-0020: Patch Evidence and Run Artifacts
---

# EXP-0014: Project-Wide Structural Indexes Beyond Code

## Context

EXP-0012 argues that a structural code graph (CodexGraph-shaped, tree-sitter
extracted, SQLite-resident) replaces a class of LLM calls that today fall back
to grep, file reads, or embedding similarity. EXP-0001 / ADR-0008 frame the
parallel embedding-based knowledge layer for prose recall.

A code graph is one instance of a more general pattern: **anything an agent
currently re-derives by reading files is a candidate for a precomputed
index**. The relationships between ADRs, features, work units, commits, tests,
and run artifacts are exactly the kind of structural lookups that today force
the model to crawl `docs/` and `git log` every session. The same pattern
extends to schemas, dependency graphs, configuration surfaces, and — at the
edges — multimedia transcripts.

This exploration takes the EXP-0012 framing ("graphs replace grep") and asks
which other project content surfaces benefit from the same treatment, what the
shape of those indexes should be, and how they stay current as the project
evolves.

## Problem / Motivating Question

In a typical modeltap session the agent re-derives a lot of context that
already exists somewhere on disk:

- "Which feature does this WU belong to? What ADR drives the feature?"
- "Which tests cover this symbol? Which run artifacts mention this error?"
- "What dependencies does this binary pull in? What licenses?"
- "Which env vars / config keys exist? Which are required?"
- "What endpoints does the server expose? Which MCP tools? Which CLI flags?"
- "What does this ADR/feature say without me re-reading it?"

EXP-0012 covers the code-symbol axis. The motivating question here is:

**What other project surfaces deserve precomputed indexes, what is the
appropriate index shape per surface (symbolic vs. semantic vs. extracted
text), and how do those indexes stay fresh without either becoming a daemon
or silently lying to the model?**

## Design Space

The candidate surfaces fall into three index-shape buckets. The shape matters
more than the content type — it determines cost, freshness needs, and whether
the index *replaces* an LLM call or only *narrows the context window*.

### Symbolic indexes (deterministic; replace LLM calls)

Cheap to build, exact, and answer in one hop. These are the highest-leverage
additions because the LLM disappears from the lookup path entirely.

- **SDLC traceability graph.** Nodes: ADR, EXP, FEAT, WU, PATCH, commit, test,
  doc, release. Edges: `drives` (ADR → FEAT), `decomposes-to` (FEAT → WU),
  `implements` (commit → WU), `covers` (test → symbol/feature), `references`
  (doc → ADR). Modeltap already encodes most of this in front matter and
  commit prefixes; the index is mostly a YAML/regex extraction job.
- **API surface index.** HTTP endpoints, MCP tools, CLI commands and flags,
  exported Go symbols. Sourced from route declarations, Cobra command trees,
  MCP server registrations, and `go doc` output.
- **Schema indexes.** SQLite DDL / sqlite-vec table layout, config schema
  (Viper keys + defaults + env-var mappings), front-matter schemas for
  ADR/FEAT/EXP/PATCH. Answers "what configuration exists" without reading
  every config example.
- **Dependency + license graph.** `go.mod` graph + license map. Directly
  serves the ADR-0010 license-check rule already in user memory; today the
  agent re-greps `go.mod` and the deps' `LICENSE` files every time.
- **Test coverage / failure index.** Test → symbol → feature linkage; recent
  failure history. Drives "which tests should I run for this change" without
  spinning the LLM on coverage reasoning.
- **Run artifact index (FEAT-0020).** Runs → patches → evidence → outcomes.
  This is the harness's own structured output and is already on the roadmap
  to be persisted; indexing it as a graph pays off the moment a second turn
  asks "what did the previous run produce."
- **Ownership / authorship.** CODEOWNERS-equivalent + git-blame summary keyed
  by file or subsystem.

### Semantic indexes (embedding-based; narrow context, still need LLM)

Already on the roadmap via sqlite-vec / ADR-0008. The natural extensions:

- All prose surfaces: ADRs, features, explorations, history logs, READMEs,
  user docs, contracts in `.agents/`.
- Run logs and review artifacts (`docs/features/.reviews/`).
- External corpora when present: chat threads, PR comments, issue bodies.

Semantic indexes do not replace LLM calls — they reduce how much text is
fed to the call. They complement, never substitute, the symbolic indexes
above.

### Extracted-text from multimedia (pragmatic floor; full embeddings rare)

The principle: in an SDLC tool the *transcript* layer captures most of the
queryable signal. Raw multimedia embeddings are rarely worth their cost.

- **Images:** OCR (architecture diagrams, screenshots), alt text, EXIF, a
  perceptual hash for dedup. Embed only when visual similarity is the actual
  query.
- **Video:** transcript + scene/timestamp index. The transcript does ~90% of
  the retrieval job. Frame embeddings are a niche addition.
- **Audio:** transcript + speaker diarization. Same logic as video.

For modeltap specifically these are speculative — there is no current
multimedia in-tree — and would only matter once we ingest user-supplied
project corpora.

## Index Freshness

This is the user-flagged open question and the hardest part of the design.
EXP-0012 raises the same concern in code-graph form ("a graph that lags the
working tree silently misleads the model"). The general failure mode is the
same across surfaces: a stale index is worse than no index, because the model
trusts it.

The candidate cadences, in roughly increasing operational cost:

### A. Lazy / on-demand

Build the index the first time it is queried in a session. Cheap setup, no
daemon, but the first query in any cold project is slow and the index is
session-scoped unless persisted.

### B. Build-time (`make` / pre-commit)

The user's hypothesis. Re-index as part of `make build`, `make test`, or a
pre-commit hook. Works well for symbolic indexes that the *developer* queries
(CI checks, ownership lookups, traceability reports) and ties freshness to
existing developer workflow muscle memory. Two failure modes:

1. **Make-time lag during agent turns.** The harness rewrites files between
   makes. If the agent renames a symbol mid-turn and the next prompt looks it
   up, a make-tied index is stale. This breaks exactly when the index is
   most valuable.
2. **Discipline coupling.** It only works if `make` is consistently run.
   First-time clones, IDE-only edits, and out-of-band history changes (rebase,
   reset, branch switch) all skip it.

### C. Git hook driven

`pre-commit`, `post-commit`, `post-merge`, `post-checkout`. Catches most
state-change events without a daemon. Same agent-turn-lag problem as (B), but
better coverage of branch operations.

### D. File-watch daemon

codebase-memory-mcp's approach: long-lived process listens for FS events,
re-indexes incrementally. Gives the freshest possible index but conflicts
with modeltap's "single binary, no daemon" posture (ADR-style expectation,
called out in EXP-0012). Acceptable when the daemon *is* the harness.

### E. Harness-resident incremental

Modeltap itself, while running a session, watches the working tree and
re-indexes incrementally between turns. This is the natural fit for an
integrated harness (EXP-0008): the index is a first-class harness capability
rather than a separate process. Outside an active session, fall back to (B)
or (C).

### F. Hybrid (likely answer)

Layer cadences by who asks the question:

- **Agent-internal lookups during a turn:** harness-resident incremental (E).
  Driven by FS events the harness already sees, since it owns the edits.
- **Cross-session / developer / CI lookups:** make + CI rebuild (B), with a
  hash-based reconcile so a divergence between the persisted index and the
  current tree triggers a rebuild rather than a silent stale read.
- **Branch operations:** git-hook driven (C) for `post-checkout` and
  `post-merge` specifically, because those are the cheapest places to detect
  large-scale tree changes.

The reconciliation primitive matters more than the cadence choice: every
index entry should be keyed by a content hash, and any query path must be
willing to invalidate-and-rebuild on hash mismatch. That converts the
freshness problem from "do we run often enough?" to "do we notice when we
were wrong?" — a strictly safer property.

### Per-surface freshness needs

The cadences above also differ by surface:

- **Code graph:** highest churn; needs (E) during turns, (B/C) otherwise.
- **SDLC traceability:** moderate churn; (B) is enough, with a reconcile on
  every release-status update.
- **Schema / config / API surface:** low churn; (B) is fine.
- **Dependency + license graph:** changes only when `go.mod` does;
  `post-checkout` + `post-merge` + on-demand on `go.mod` change.
- **Run artifacts:** append-only by construction (FEAT-0020); index updates
  on write, never invalidate.
- **Multimedia transcripts:** near-zero churn; build once on ingest.

## Tensions and Tradeoffs

- **Symbolic vs. semantic.** Symbolic indexes replace LLM calls but each one
  is a custom extractor that has to be maintained. Semantic indexes are
  general-purpose but only narrow context. Building too many symbolic indexes
  becomes a maintenance burden; relying only on semantic ones leaves a long
  tail of cheap-but-foregone wins.
- **Indexer surface area vs. payoff.** Every additional surface (schemas,
  routes, ownership, multimedia) is its own extractor and its own freshness
  story. The discipline is to add only the surfaces where the agent
  *demonstrably* re-derives the same answer across sessions.
- **Coupling to the harness.** Harness-resident incremental indexing is the
  cleanest freshness story but couples index health to a running modeltap
  process. CI and external tooling (Claude Code, Codex, Cursor) need a
  cadence that does not assume modeltap is running — argues for the hybrid
  in (F).
- **Single-binary purity.** Same tension EXP-0012 raises. Indexers for
  non-code surfaces (front-matter regex, `go list`, `go.mod` parsing) are all
  pure-Go and free of CGO; multimedia transcript extraction would import
  much heavier dependencies (ffmpeg, whisper-style models) and probably
  belongs out-of-process behind MCP.
- **Privacy.** ADR-0008's local-first posture extends to all indexes here.
  No surface should require shipping project content off-box, including
  multimedia transcription.
- **Overlap with EXP-0012.** The code graph and the SDLC traceability graph
  share storage (SQLite), share a freshness story, and share an MCP surface.
  Treating them as separate indexes would duplicate machinery; treating them
  as one risks an unwieldy schema. Likely answer: shared storage and
  shared freshness machinery, distinct logical schemas.

## Reference Corpora and Observed Prior Art

Three local projects already produce SDLC corpora at different maturities and
process shapes. They are useful both as evidence for the inventory step and
as prior art for the freshness story.

### Modeltap (this repo)

- ~60 days of activity (2026-03-06 → 2026-05-05).
- 14 ADRs, 17 features, 12 explorations (this one included), 15 patches,
  144 history session logs.
- Filename grammar embeds role + artifact: `backend-wu005-cobra-cli.md`,
  `2026-04-30-session-feat-0015-0022-id-hygiene-processing.md`. Most
  history logs declare which artifacts they touch in the filename itself.
- Commit prefixes (`WU-NNN`, `PATCH-NNNN`, `FEAT-NNNN`, `ADR-NNNN`,
  `EXP-NNNN`, `ADMIN`) provide a second declarative trace from commit →
  artifact, independent of doc front matter.
- What it demonstrates: the SDLC traceability extractor can be built
  almost entirely from filename and commit-message regex — no LLM in the
  extraction path — for projects that follow this discipline.

### MeetingPlaceAI alpha (`~/Projects/meetingplaceai/alpha/`)

- ~50 days of activity (2026-03-16 → 2026-05-04).
- 19 ADRs (`docs/decisions/`), 39 features, 38 explorations, 298 history
  session logs.
- Same EXP/FEAT/PATCH/ADR/ADMIN artifact taxonomy modeltap uses,
  documented explicitly in `alpha/CLAUDE.md`. Filenames carry the same
  declarative cross-references.
- ADR process is **hook-enforced via `.claude/skills/` and `/adr`
  commands** — the index update fires from Claude Code hooks rather than
  from `make` or git hooks.
- What it demonstrates: at 96 cross-referenced artifacts and 298 logs the
  payoff axis for an index is the traceability graph, not the code graph.
  Also concrete prior art that option (E) — harness-resident incremental
  via hooks — is the cadence people actually reach for once they live
  with the freshness problem at scale.

### VillageData alpha (`~/Projects/villagedata/alpha/`)

- ~2 months of activity (2025-12-11 → 2026-02-14).
- No ADR / feature / exploration tree — `docs/` only carries
  `user-guide`, `problems`, `developer-guide`.
- 41 daily activity logs in `logs/YYYY-MM-DD.md`, structured per
  `CLAUDE.md` as `[HH:MM] Question / [HH:MM] Answer` pairs from Claude
  sessions.
- 2,648 commits captured in `alpha/commit-history-full.txt`.
- What it demonstrates: in projects without an SDLC artifact tree, the
  agent-re-derivation signal lives in the chat logs and commit history,
  not in cross-references between docs. The first-tier index is a
  commit-message → file/feature map and a Q&A → topic map, not a
  traceability graph. Same exploration, different first index — strong
  evidence that the inventory step is project-shaped, not universal.

### Cross-corpus implications

- **Inventory has a real corpus.** Running the proposed 0.25-day
  inventory pass against modeltap (144 logs) and mp/alpha (298 logs)
  would produce an evidence-based ranking instead of a hypothesized one,
  and vd/alpha's Q&A logs add a complementary signal that captures what
  the *user* had to re-explain rather than what the agent re-derived.
- **Freshness story already has prior art.** mp/alpha's hook-driven ADR
  process is a working instance of option (E) harness-resident
  incremental indexing, on the same Claude Code hook surface modeltap
  would use. Worth borrowing the pattern verbatim before designing
  something new.
- **Index shape varies by SDLC maturity.** mp and modeltap reward a
  traceability graph; vd rewards commit/log indexing. The unified
  storage proposed in the open questions has to accommodate both
  without forcing vd-shaped projects through a traceability schema
  they will not populate.

## Open Questions

1. **Scope discipline.** Which non-code surfaces are worth indexing in v1?
   The argument above favors SDLC traceability + API surface + dependency/
   license + run artifacts as the first tier; schemas, config, ownership,
   tests as a second tier; multimedia deferred.
2. **Storage layout.** One SQLite database with logical schemas per surface,
   or sibling databases? sqlite-vec already lives in one; adding tables there
   keeps everything queryable in one process and avoids cross-DB joins.
3. **Freshness contract.** Do we standardize on content-hash keying across
   all indexes, with a single reconcile primitive, or per-surface freshness
   policies? The simpler invariant ("every entry hashable, every query
   willing to invalidate") seems strictly better.
4. **MCP surface granularity.** One MCP tool per index (`search_code_graph`,
   `search_traceability`, `lookup_dependency`) or a unified
   `search_project_index` with a typed query? EXP-0012's recommendation to
   route harness lookups through the MCP surface argues for unification, but
   typed tools are easier for external clients to discover.
5. **Where does this live in the FEAT-0015..0022 roadmap?** Run-artifact
   indexing is already implied by FEAT-0020. Traceability indexing is a
   peer to the conversation knowledge layer rather than a sub-feature of
   any single track. The shape of the answer probably mirrors EXP-0012's:
   a peer feature spec, not a sub-feature.
6. **Make integration vs. harness integration.** If we adopt the hybrid
   freshness story (F), what is the minimal `make index` target that an
   external user (or CI) can rely on, and how does it relate to the
   harness's own incremental index? They must converge on the same on-disk
   schema, or they will diverge in ways the model cannot detect.

## Proposed Next Step

Two-step, low-commitment:

1. **Inventory pass (~0.25 day):** enumerate, for the modeltap repo as it
   stands today, every "structural lookup" the agent performed in the last
   N sessions (history logs are the source). Cluster by surface (code,
   traceability, dep, schema, run, etc.) and count. The result is an
   evidence-based ranking of which non-code surfaces are worth indexing
   first.
2. **Traceability spike (~0.5 day):** build the SDLC traceability graph as
   the first non-code symbolic index, since it is the lowest-cost extractor
   (front-matter + commit-message regex), the highest-frequency lookup, and
   the cleanest test of the freshness story. Persist it in the same SQLite
   database the code graph would use, key every entry by content hash, and
   wire a `make index` target plus a harness-resident reconcile.

If the inventory shows the agent rarely re-derives non-code structure, this
exploration closes with a negative result and modeltap stays on EXP-0012's
code-graph-only path. If the inventory confirms the pattern and the spike's
freshness primitive holds, promote this exploration into a feature spec
("Project Structural Indexes", peer to the code-graph and knowledge-layer
features) and an ADR for the unified storage + freshness contract.
