---
feature: FEAT-0011
title: Knowledge Integration
status: proposed
date: 2026-04-14
depends-on:
  - FEAT-0008: BFF Server
  - FEAT-0009: Terminal Harness
adr-constraints:
  - ADR-0008: sqlite-vec for vector embeddings (amendment proposed — on by default)
  - ADR-0009: MCP stdio for external knowledge access (unchanged)
promoted-from:
  - EXP-0001: Knowledge Layer
  - EXP-0008: Integrated Harness
---

# FEAT-0011: Knowledge Integration

## Problem

FEAT-0008 and FEAT-0009 create a multi-model terminal AI environment with conversation state, tool execution, and cost tracking. But without the knowledge layer, every session starts from zero. The model has no memory of prior sessions, no awareness of past decisions, and no institutional context. This is the same limitation every other AI tool has — and the gap modeltap is uniquely positioned to fill.

The knowledge layer (EXP-0001, ADR-0008) was designed as an optional module for the v1 proxy. In the integrated harness product, it becomes the core differentiator: the system that makes the 50th conversation smarter than the 1st.

## Solution

Wire the knowledge layer into the BFF's conversation loop so that:

1. Every conversation turn is automatically captured and embedded for future retrieval
2. Before each turn, the BFF searches the knowledge base and injects relevant prior context
3. Compaction is backed by the knowledge layer — compressed context is searchable, not lost
4. Users can query, pin, forget, and manage their knowledge explicitly

This feature implements the integration between the existing knowledge layer architecture (ADR-0008) and the BFF conversation loop (FEAT-0008). It does not re-implement embedding or vector search — it wires them together.

## Key Capabilities

### Automatic Capture and Embedding

Every conversation turn that flows through the BFF is:

1. Stored in the request log (existing capture, ADR-0005)
2. Queued for asynchronous embedding
3. Processed by the embedding pipeline:
   - User messages are embedded (questions reveal intent)
   - Model responses are embedded (answers contain knowledge)
   - Structured metadata is extracted (decisions, action items, topics, files referenced)
   - Tool results containing significant content are embedded separately

**Embedding model**: local by default (`nomic-embed-text` via Ollama). Cloud embeddings (`text-embedding-3-small` via OpenAI) as opt-in upgrade. The default experience is free and private.

**Chunking**: each conversation turn is one embedding unit. Long turns are split at paragraph boundaries. Metadata is embedded as structured text.

**Storage**: sqlite-vec (ADR-0008). Vectors stored alongside all other data in the same SQLite database.

### Transparent Context Enrichment

Before each turn, the BFF performs relevance-gated knowledge injection:

1. Embed the user's current message
2. Search the knowledge base for semantically similar prior interactions
3. Score results by: relevance x recency x importance (pinned items score highest)
4. Inject top results into the system prompt as a "prior context" block
5. Respect a configurable token budget (default: max 20% of context window for injections)

The user does not see the injection unless they ask (`/context` shows what was injected). The model sees prior decisions, approaches, and context as if the user had explicitly provided them.

**Project scoping**: knowledge queries prefer results from the current project. Cross-project results are included at lower relevance scores, enabling "what did I decide about auth in any project?" while prioritizing "what did I decide about auth in this project?"

### Lossless Compaction

When the BFF compacts conversation context (FEAT-0008 context window management):

1. Identify low-value segments (old turns, resolved tangents, repeated instructions)
2. Summarize them for the live context window
3. The full original remains in storage and the knowledge layer
4. If compacted content becomes relevant in a later turn, the BFF can re-retrieve it from the knowledge base and inject it

This makes compaction reversible from the user's perspective. Nothing is lost — it moves from working memory to searchable long-term memory.

### Knowledge Commands

| Command | Description |
|---------|-------------|
| `/context` | Show files in context, knowledge injections, and token budget |
| `/pin <text>` | Mark a decision or constraint as always-carry-forward |
| `/unpin <text>` | Remove a pinned item |
| `/forget <query>` | Remove specific knowledge entries matching the query |
| `/search <query>` | Explicit semantic search across knowledge base |
| `/knowledge stats` | Show knowledge base size, embedding coverage, recent extractions |

### Metadata Extraction

The BFF extracts structured metadata from conversations:

- **Decisions**: "We decided to use JWT instead of sessions"
- **Action items**: "TODO: add rate limiting to the auth endpoint"
- **Topics**: classification of conversation subjects
- **Files referenced**: which files were discussed or modified
- **People mentioned**: names and roles referenced in conversation

Extraction is model-driven: the BFF prompts a cheap/fast model to extract structured fields from completed turns. Extraction happens asynchronously — it does not block the conversation.

### Graceful Degradation

The knowledge layer degrades gracefully when components are unavailable:

- **No embedding model available**: knowledge layer operates in keyword-search-only mode. Capture continues. Embeddings are generated when a model becomes available (backfill queue).
- **Knowledge disabled** (`knowledge.enabled: false`): capture continues. No embedding, no injection, no search. The harness works as a standard multi-model terminal tool.
- **Knowledge database corrupted**: the BFF falls back to no-knowledge mode. The capture layer can rebuild the knowledge base from the raw request log.

## CLI Integration

```
modeltap search "authentication approaches"    # CLI semantic search (existing, enhanced)
modeltap knowledge stats                       # Knowledge base statistics
modeltap knowledge rebuild                     # Rebuild embeddings from capture log
```

## Configuration

```yaml
knowledge:
  enabled: true                          # on by default (ADR-0008 amendment)
  embedding_model: nomic-embed-text      # local via Ollama
  auto_enrich: true                      # inject knowledge into conversations
  injection_budget: 0.20                 # max 20% of context window
  retention: 90d                         # knowledge retention period
  extraction:
    enabled: true                        # extract structured metadata
    model: llama-3.1-8b                  # cheap model for extraction
```

## Non-Goals

- **Shared knowledge across users**: per-user isolation is the baseline (FEAT-0010). Shared knowledge is deferred per EXP-0002.
- **Knowledge graph**: relationships between decisions, people, and projects. Future work.
- **Context injection for external MCP clients**: the MCP stdio interface (ADR-0009) exposes search tools, but does not perform automatic injection. Injection is a BFF-to-model feature.
- **Custom embedding models**: the feature supports configurable embedding models, but does not build a model management UI. Users configure the model name in YAML.

## Success Criteria

1. Conversations are automatically embedded within 30 seconds of turn completion when an embedding model is available.
2. Semantic search returns relevant results: a query for "authentication decisions" finds conversations where auth was discussed, even if the word "authentication" was not used.
3. Context enrichment injects relevant prior context into the system prompt before each turn. The model demonstrates awareness of prior decisions without being explicitly told.
4. Compacted content is retrievable: after compaction, a query referencing compacted content re-surfaces it from the knowledge layer.
5. Pinned items appear in every turn's context regardless of relevance scoring.
6. `/forget` removes entries from the knowledge base and they no longer appear in search or injection results.
7. The knowledge layer degrades gracefully when no embedding model is available — conversations work, just without semantic features.
8. Knowledge features respect per-user isolation (FEAT-0010): user A's knowledge is not visible to user B.
9. Injection budget is respected: knowledge injections never exceed the configured percentage of the context window.
10. The embedding backfill queue processes historical captures when an embedding model becomes available for the first time.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0005 (Capture) | Knowledge layer builds on full-fidelity capture — embeddings are generated from captured content |
| ADR-0008 (sqlite-vec) | Vector storage and search use sqlite-vec. Amendment proposed: on by default. |
| ADR-0009 (MCP stdio) | MCP interface exposes search_knowledge and related tools for external clients. Injection is BFF-internal. |

## Open Questions

1. **Injection quality measurement**: how do we measure whether knowledge injection actually improves response quality? A/B comparison with injection on/off? User feedback signals?
2. **Negative knowledge**: should the knowledge layer distinguish "facts learned" from "approaches that failed"? Failed approaches are valuable context but should be framed differently.
3. **Scale**: sqlite-vec brute-force KNN is 50-200ms at 100k vectors. At 200 interactions/day, 100k is ~500 days. When should HNSW indexing be evaluated?
4. **Extraction accuracy**: model-driven metadata extraction will have errors. Should extracted metadata be user-reviewable? Or is best-effort acceptable for search ranking?
