# 2026-04-28 — Code Graph Exploration (EXP-0012)

## Context

User asked for an exploration on code graphing via AST, prompted by the
CodexGraph paper ([arXiv 2408.03910v2](https://arxiv.org/html/2408.03910v2)),
and wanted to know whether open-source code graph tools fit modeltap's
Go / SQLite / single-binary posture.

## Discussion

- Summarized CodexGraph: property-graph schema (MODULE / CLASS / FUNCTION /
  FIELD / METHOD / GLOBAL_VARIABLE nodes, CONTAINS / HAS_METHOD / HAS_FIELD /
  INHERITS / USES edges), Cypher queries via "write then translate" LLM
  pattern, Python-only evaluation, ~23% on SWE-bench Lite.
- Surveyed open-source candidates and grouped by fit:
  - Strong fits: smacker/go-tree-sitter, tree-sitter/go-tree-sitter,
    tree-sitter-graph, DeusData/codebase-memory-mcp, Sourcegraph SCIP +
    scip-go.
  - Adjacent / informative: er77/code-graph-rag-mcp, tirth8205/code-review-graph,
    malivvan/tree-sitter (pure-Go via wazero), Graphify.
  - Probably out for embedding (JVM/Python heavy): Joern,
    Fraunhofer-AISEC/cpg, AppThreat/cpggen.
- Verified license compatibility against ADR-0010 for the strong-fit
  candidates (smacker MIT, codebase-memory-mcp MIT, scip-go Apache-2.0,
  tree-sitter MIT, tree-sitter-graph MIT/Apache-2.0).
- User signaled that the code graph should be MCP-first so any harness
  (Claude Code, Codex, Cursor, modeltap's own) can use it — consistent
  with ADR-0009 and EXP-0001's cross-model brain framing. Resolved that
  question in the exploration.
- Confirmed in current code that `modeltap start` (proxy) is independent
  of the harness via `internal/cli/root.go` — peer Cobra subcommands, not
  nested. External harnesses can use the proxy today regardless of
  whether modeltap's own harness is running.
- Discussed when single-binary distribution stops paying for itself.
  Conclusion: today's shared core (storage, providers, capture, config)
  keeps one binary correct; the realistic first fork point is the
  code-graph indexer (CGO + tree-sitter grammars + 66 languages) if it
  starts hurting proxy-only users — defer until the spike measures
  actual binary size and CGO impact.
- Continued follow-up inspected codebase-memory-mcp's README and source tree.
  EXP-0012 now records its generic SQLite node/edge schema, JSON properties,
  FTS5 identifier index, multi-pass indexing pipeline, and integrate-first
  recommendation.

## Files Created

- `.sdlc/explorations/0012-code-graphing-via-ast.md` — EXP-0012 exploration.

## Files Modified

- `.sdlc/explorations/README.md` — added EXP-0012 to the index.

## Open Items / Next Steps

- External baseline (~0.5 day): install or build codebase-memory-mcp in a
  throwaway cache, index modeltap, and capture concrete query/schema results.
- Native comparison spike (~1 day): smacker/go-tree-sitter against modeltap
  itself, emit a minimal sidecar SQLite graph for definers/callers, and measure
  index time and on-disk size.
- Open questions still parked in EXP-0012: schema fusion with existing
  knowledge layer, CodexGraph taxonomy vs. richer codebase-memory-mcp-style
  edges, placement in FEAT-0008..0014 roadmap, indexing budget on M-series Mac.

## Notes

- No code changes. Exploration only — does not authorize implementation.
- The session followed the established pattern of using public papers
  and open-source repos as design input; no leaked proprietary source
  was referenced.
