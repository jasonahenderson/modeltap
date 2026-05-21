---
status: deferred
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0008: Knowledge Layer Architecture

## Status

**Deferred** — this ADR originally chose sqlite-vec for a vector-based knowledge layer. The decision has been deferred until the core proxy is built and validated with real users.

## Rationale for Deferral

The knowledge layer (semantic search, embeddings, metadata extraction, vector storage) is a separate product built on top of the proxy. Designing it before the proxy exists creates several risks:

1. **Circular justification:** ADR-0005 (capture strategy) was partially justified by knowledge layer needs, and the knowledge layer was justified by ADR-0005 guaranteeing full capture. Neither has been validated with real usage.
2. **Premature commitment to sqlite-vec:** sqlite-vec is a newer extension with brute-force KNN. Committing to it before knowing actual query patterns and scale requirements is premature.
3. **Unaddressed metadata extraction costs:** Extracting decisions, topics, and action items from conversations requires LLM calls — the cost, latency, and failure modes were not addressed.
4. **Scope creep:** The embedding pipeline, queue table, background worker, and sqlite-vec extension bundling add significant complexity to a tool that should start as a simple proxy.

## What to do instead

1. Build the proxy core (ADR-0001 through ADR-0006).
2. Ship `modeltap logs` and `modeltap show <id>` for basic data access.
3. Full response bodies are stored in SQLite (ADR-0005 default: full capture) — nothing prevents adding embeddings later.
4. When users request semantic search, revisit this ADR with real data about usage patterns, scale, and embedding model preferences.

## Original proposal (preserved for reference)

The original ADR evaluated sqlite-vec, separate vector databases (Qdrant/ChromaDB), hybrid SQLite + FAISS, and full-text search only. sqlite-vec scored highest (119) due to single-file database consistency with ADR-0002. This analysis remains valid as a starting point when the decision is revisited.
