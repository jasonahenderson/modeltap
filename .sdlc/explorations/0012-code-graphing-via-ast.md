---
exploration: EXP-0012
title: Code Graphing via AST for Repository-Aware Context
status: exploring
date: 2026-04-28
related:
  - EXP-0001: Knowledge Layer (Cross-Model Brain)
  - EXP-0008: Integrated Harness
  - EXP-0011: Harness Excellence Gap Analysis
  - ADR-0008: Knowledge layer architecture
  - ADR-0009: MCP server for knowledge access
---

# EXP-0012: Code Graphing via AST for Repository-Aware Context

## Context

Modeltap's existing knowledge layer (EXP-0001 / ADR-0008 / ADR-0009) is
conversation-centric: it captures requests and responses, embeds them with
sqlite-vec, and exposes them via MCP. That works well for "what did I decide
about X" recall, but it does not give the model a structural map of the
*current repository* it is editing.

EXP-0011 frames excellence in a coding harness as control over context,
repository understanding, edit mechanics, and validation. Repository
understanding is the weakest leg in modeltap today: when a turn needs
"every caller of `Foo`," "all classes that inherit `Bar`," or "the file that
defines this symbol," the harness has no first-class structural index — it
falls back to grep, file reads, or embedding similarity.

The motivating prompt for this exploration is the CodexGraph paper
([Liu et al., 2024 — arXiv 2408.03910v2][codexgraph]), which argues that
graph-database-shaped repository indices outperform similarity retrieval
and task-specific tools on repo-level tasks. The question for modeltap is
**whether to add a structural code-graph layer alongside the existing
embedding-based knowledge layer, and what open-source building blocks would
let us do that without inventing parsers from scratch**.

[codexgraph]: https://arxiv.org/html/2408.03910v2

## Problem / Motivating Question

For repo-level coding work the harness routinely needs answers like:

- What functions call this one? What does this function call?
- Where is this symbol defined? What are its concrete implementations?
- What modules import this one? What inherits from this class?
- Which tests exercise this path?

Embedding similarity can approximate some of these, but:

- Embeddings retrieve *passages that look related* to a query, not
  *symbols that are structurally related* to a target.
- Cross-file relationships (inheritance, call edges, import graphs) are
  exactly the relationships embeddings encode least reliably.
- Token cost grows roughly linearly with file-by-file exploration, while a
  pre-materialized graph answers structural queries in one hop.

So: should modeltap build a code-graph index of the active repository,
shaped roughly like CodexGraph's property graph, and expose it via MCP
alongside the current knowledge-layer surface?

## CodexGraph in One Page

The paper proposes a property graph rather than raw ASTs:

- **Nodes:** `MODULE`, `CLASS`, `FUNCTION`, `FIELD`, `METHOD`,
  `GLOBAL_VARIABLE`.
- **Edges:** `CONTAINS`, `HAS_METHOD`, `HAS_FIELD`, `INHERITS`, `USES`.
- **Construction:** a shallow pass extracts symbols and local edges, then a
  DFS pass resolves cross-file references (especially inheritance).
- **Query path:** an LLM agent emits a natural-language query; a second
  agent translates it into Cypher against the graph database; the result is
  returned to the planning model.
- **Evaluation:** Python only — competitive on CrossCodeEval Lite, SWE-bench
  Lite (~23%), and EvoCodeBench versus BM25 and AutoCodeRover. The authors
  flag Python-only support as a key limitation.

The takeaway for modeltap is the *shape*, not the implementation. The
node/edge taxonomy and the "extract once, query many times" pattern
generalize cleanly. The Cypher-translation agent and Neo4j dependency do
not — modeltap has SQLite (ADR-0002) and sqlite-vec (ADR-0008) already, and
adding Neo4j would be heavy.

## Open-Source Building Blocks

Below are candidate tools, grouped by what they bring to a Go-native,
SQLite-backed harness. Licenses checked against ADR-0010 (Apache-2.0
required-compatible) — flagged where it matters.

### 1. AST extraction (lowest level)

- **tree-sitter** (MIT) — incremental, language-agnostic parser generator.
  De-facto standard for ASTs in coding harnesses.
  ([repo](https://github.com/tree-sitter/tree-sitter))
- **smacker/go-tree-sitter** (MIT) — Go bindings for tree-sitter via CGO,
  with grammars for most common languages vendored in. The most direct fit
  for a Go-only modeltap binary, at the cost of pulling CGO into the build.
  ([pkg.go.dev](https://pkg.go.dev/github.com/smacker/go-tree-sitter))
- **tree-sitter/go-tree-sitter** (MIT) — newer official Go bindings. Less
  mature than smacker's but actively maintained upstream.
  ([repo](https://github.com/tree-sitter/go-tree-sitter))
- **malivvan/tree-sitter** (license TBD) — pure-Go via wazero, no CGO. Worth
  evaluating if avoiding CGO matters for goreleaser/cross-compile.
  ([pkg.go.dev](https://pkg.go.dev/github.com/malivvan/tree-sitter))

### 2. AST → graph DSL

- **tree-sitter-graph** (MIT/Apache-2.0) — tree-sitter's official DSL for
  emitting graph structures from parse trees. Originally built for
  stack-graphs; reusable for CodexGraph-shaped extraction.
  ([repo](https://github.com/tree-sitter/tree-sitter-graph))

### 3. Pre-built code-graph indexers

- **DeusData/codebase-memory-mcp** (MIT) — closest spiritual sibling.
  Tree-sitter + SQLite, 66 languages, MCP-native, ships as a single static
  binary, claims sub-ms structural queries. Core is C with Go packaging.
  Could be a reference implementation, an MCP tool we shell out to, or a
  vendor candidate (license is compatible).
  ([repo](https://github.com/DeusData/codebase-memory-mcp))
- **er77/code-graph-rag-mcp** (license TBD) — tree-sitter + sqlite-vec, MCP
  surface. Distributed via npm — adopting wholesale would conflict with
  modeltap's single-binary posture, but the schema is informative.
  ([repo](https://github.com/er77/code-graph-rag-mcp))
- **tirth8205/code-review-graph** (license TBD) — local code KG aimed at
  Claude Code reviews. Useful prior art for the review-loop integration
  even if we don't adopt the code.

### 4. Heavier code-property-graph stacks

- **Sourcegraph SCIP + scip-go** (Apache-2.0) — protobuf-based code
  intelligence format with a Go-native indexer for Go repos. Strong fit
  *for indexing modeltap itself*; ecosystem of indexers for other
  languages (`scip-typescript`, `scip-java`, `scip-python`,
  `scip-clang`, `rust-analyzer`, etc.). Tradeoff: SCIP is symbol-table /
  xref-shaped, not call-graph-shaped.
  ([scip](https://github.com/sourcegraph/scip),
   [scip-go](https://github.com/sourcegraph/scip-go))
- **Joern** (Apache-2.0) — full code property graph (AST + CFG + DDG) with
  a query language. JVM-based, heavy operational footprint, awkward to
  embed in a Go binary. Probably out for modeltap core, viable as an
  optional power-user MCP tool.
  ([repo](https://github.com/joernio/joern))
- **Fraunhofer-AISEC/cpg** (Apache-2.0) — JVM CPG library with native
  C/C++/Java support and experimental Go/Python/TypeScript. Same embedding
  problem as Joern.
  ([repo](https://github.com/Fraunhofer-AISEC/cpg))
- **AppThreat/cpggen** (Apache-2.0) — Python wrapper that emits CPGs into
  Joern. Useful as a one-shot ingestion tool, not a runtime dep.

### 5. Adjacent / mixed-modal

- **Graphify** (license TBD) — tree-sitter + NetworkX + Leiden clustering
  over code, docs, PDFs, and diagrams. Python. Interesting for the
  multi-modal angle but not a Go embedding candidate.

## Best Fit for Modeltap

Filtering by Go-native, single-binary, Apache-2.0-compatible,
SQLite-friendly, ADR-0008-aligned:

1. **smacker/go-tree-sitter (or the new official Go bindings) +
   tree-sitter-graph DSL + a CodexGraph-shaped schema written into our
   existing SQLite database.** This is the lowest-coupling option —
   modeltap owns the indexer end-to-end, the graph lives next to the
   conversation index in `~/.config/modeltap`, and there is no extra
   process to manage. CGO is the main wart.
2. **DeusData/codebase-memory-mcp as a reference / drop-in MCP backend.**
   It already covers 66 languages, ships as a single binary, and exposes
   the right shape over MCP. We could either (a) call it as an MCP server
   and skip building our own, (b) vendor and re-export it under the
   modeltap binary, or (c) treat it as the spec we re-implement in Go.
3. **scip-go as a complement, not a replacement.** SCIP gives us a
   battle-tested Go indexer for Go repos and an ecosystem of indexers for
   other languages without us writing parsers — but the symbol/xref model
   doesn't cover call-graph queries on its own. A hybrid (SCIP for
   def/ref, our own tree-sitter pass for call edges and inheritance) is
   plausible.

The Joern/Fraunhofer-CPG family is the most semantically rich, but the
JVM dependency conflicts with modeltap's distribution model. Worth keeping
in mind as an optional "power-user" MCP integration rather than a core
component.

## Tensions and Tradeoffs

- **Structural vs. embedding retrieval.** Code graphs answer "what calls
  this" cheaply and exactly; embeddings answer "what looks similar" with
  recall but no guarantees. Both have legitimate roles — the hard part is
  designing a query interface that picks the right one (or fuses them)
  without burning the model's tokens.
- **Single-binary purity vs. ecosystem reuse.** Vendoring a tree-sitter +
  CGO + 66 grammars stack is heavy; shelling out to an external indexer
  binary breaks ADR-style "one binary, no daemons" expectations. Picking
  one of these two costs is unavoidable.
- **Index freshness.** A graph that lags the working tree silently misleads
  the model. File-watch + content-hash incremental re-indexing
  (codebase-memory-mcp's approach) is the right shape, but adds
  operational complexity not present in the conversation-only knowledge
  layer.
- **Scope of languages.** CodexGraph evaluated only Python. Modeltap users
  span Go, TS, Python, Rust, Java. tree-sitter solves the parser problem;
  *semantic resolution* (inheritance, type inference, cross-module
  references) is much harder per-language and is exactly where Joern/CPG
  win.
- **MCP surface vs. internal tool.** EXP-0011 argues the harness should
  treat repo understanding as a first-class internal capability. The
  resolution here is *both, with MCP as the primary surface*: ADR-0009
  already exposes the knowledge layer over MCP, EXP-0001 frames modeltap
  as a cross-model brain, and the value of a structural index multiplies
  if Claude Code, Codex, Cursor, and modeltap's own harness can all hit
  the same graph for the same repo. Modeltap's harness consumes the same
  MCP surface internally rather than reaching past it.
- **Privacy.** ADR-0008's privacy posture (local SQLite, no cloud
  embeddings by default) extends naturally to a local code graph. Any
  candidate that wants to phone home is out.

## Focused Read: codebase-memory-mcp

A focused read of codebase-memory-mcp makes it stronger prior art than the
initial scan suggested:

- **Shape:** it is already MCP-first, not an LLM wrapper. The MCP client does
  the natural-language-to-tool planning; the server owns indexing and
  structural query execution.
- **Runtime:** it is primarily C/C++, not Go, with vendored tree-sitter
  grammars and SQLite storage. That makes it a clean external MCP baseline but
  a poor direct in-process dependency for modeltap's Go binary.
- **Data model:** its graph is richer than CodexGraph's paper schema:
  `Project`, `Package`, `Folder`, `File`, `Module`, `Class`, `Function`,
  `Method`, `Interface`, `Enum`, `Type`, `Route`, and `Resource` nodes, with
  edges for containment, definitions, imports, calls, HTTP calls,
  implementations, tests, type usage, config writes, and file co-change.
- **Storage schema:** the source-level schema is intentionally generic:
  `projects`, `file_hashes`, `nodes`, `edges`, and `project_summaries`.
  Nodes and edges carry JSON `properties`; edges are unique by source,
  target, and type; indexes cover label/name/file and source/target/type
  traversal. It also creates an FTS5 identifier index that splits camelCase.
- **Tool surface:** it exposes indexing, project listing, graph search, call
  tracing, git-diff impact analysis, Cypher-like read-only graph queries,
  schema inspection, source snippets, architecture summaries, text search, ADR
  management, and runtime trace ingestion.
- **Pipeline depth:** the source tree shows dedicated passes for definitions,
  calls, packages, routes, semantic edges, usages, tests, git diff/history,
  infrastructure/Kubernetes, configuration, cross-repo links, and similarity
  edges. This is closer to a productized code-intelligence engine than a thin
  AST demo.
- **Operational posture:** indexes persist under a local SQLite cache, optional
  auto-indexing keeps projects fresh, `.gitignore` / `.cbmignore` are honored,
  and the published binaries are advertised as static with no runtime
  dependencies.
- **License:** MIT, compatible with ADR-0010's Apache-2.0-compatible
  requirement.

That shifts the near-term recommendation: **evaluate codebase-memory-mcp as an
external MCP baseline before building a modeltap-native indexer**. The
modeltap-native spike is still useful, but its job changes from "prove whether
code graphs are feasible" to "measure what modeltap gains by owning the
schema, packaging, and storage instead of wrapping an external MCP server."

## Open Questions

1. Do we want a **separate** code-graph index or do we extend the existing
   knowledge-layer schema with structural tables? sqlite-vec coexists fine
   with relational graph tables; the question is whether queries fuse.
2. Is the **CodexGraph node/edge taxonomy** sufficient, or do we want
   call-graph, route, test, config, co-change, and runtime-trace edges from
   day one? codebase-memory-mcp is evidence that the richer surface is useful,
   but it also raises the implementation bar.
3. **Build vs. integrate vs. vendor** for codebase-memory-mcp. The focused
   read favors integrate-first. Vendoring would import a C/C++ subsystem into
   the Go release lifecycle; rebuilding everything in Go should wait for
   evidence that external MCP integration cannot meet modeltap's UX or
   storage needs.
4. Where does this sit in the FEAT-0008..0014 harness roadmap? It is
   probably a peer to the conversation knowledge layer, not a sub-feature
   of any single track.
5. Indexing budget — how big a repo do we support before incremental
   indexing stops being instant on the user's M-series Mac?

## Proposed Next Step

Three-step, low-commitment:

1. **External baseline (~0.5 day):** install or build codebase-memory-mcp in a
   throwaway cache, index modeltap, and capture concrete results for
   `get_graph_schema`, `search_graph`, `trace_call_path`, `detect_changes`,
   and `get_architecture`.
2. **Native spike (~1 day):** stand up smacker/go-tree-sitter against this repo,
   emit a minimal graph into a sidecar SQLite schema, and measure index time +
   on-disk size for the modeltap tree itself. Keep the scope to definers and
   callers so it can be compared honestly against the external baseline.
3. **Decision note:** promote the outcome into either a feature spec
   ("Repository Code Graph") plus ADR, or a smaller patch to integrate an
   external MCP code-graph server as a supported knowledge backend.

If the spike's indexing cost is sustainable and the query results are
useful in real harness turns, promote this exploration into a feature
spec (roughly: "Repository Code Graph", peer to the knowledge layer) and
an ADR for the schema choice. Otherwise document the negative result and
keep the harness on embedding-only retrieval.
