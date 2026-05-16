---
feature: FEAT-0011
title: Knowledge Integration
status: proposed
date: 2026-04-14
depends-on:
  - FEAT-0008: Runtime Server
  - FEAT-0009: Terminal Harness
adr-constraints:
  - ADR-0008: sqlite-vec for vector embeddings (optional per current ADR; amendment proposed but not required for acceptance)
  - ADR-0009: MCP stdio for external knowledge access (unchanged)
  - ADR-0005: Always full capture (constrains /forget semantics)
promoted-from:
  - EXP-0001: Knowledge Layer
  - EXP-0008: Integrated Harness
---

# FEAT-0011: Knowledge Integration

## Problem

FEAT-0008 and FEAT-0009 create a multi-model terminal AI environment with conversation state, tool execution, and cost tracking. But without the knowledge layer, every session starts from zero. The model has no memory of prior sessions, no awareness of past decisions, and no institutional context. This is the same limitation every other AI tool has — and the gap modeltap is uniquely positioned to fill.

The knowledge layer (EXP-0001, ADR-0008) was designed as an optional module for the v1 proxy. In the integrated harness product, it becomes the core differentiator: the system that makes the 50th conversation smarter than the 1st.

## Solution

Wire the knowledge layer into the runtime server's conversation loop so that:

1. Every conversation turn is automatically captured and embedded for future retrieval
2. Before each turn, the runtime server searches the knowledge base and injects relevant prior context
3. Compaction is backed by the knowledge layer — compressed context is searchable, not lost
4. Users can query, pin, forget, and manage their knowledge explicitly

This feature implements the integration between the existing knowledge layer architecture (ADR-0008) and the runtime server conversation loop (FEAT-0008). It does not re-implement embedding or vector search — it wires them together.

**Relationship to ADR-0008 amendment**: EXP-0008 proposes changing the knowledge layer default from off to on. This feature does not depend on that amendment. It works under either default:
- If the amendment is accepted (on by default): this feature is active for all harness users out of the box.
- If ADR-0008 is unchanged (off by default): this feature activates when the user sets `knowledge.enabled: true`.

The feature's acceptance criteria are testable regardless of the default. The recommended default is a product decision, not a behavior contract.

## Key Capabilities

### Automatic Capture and Embedding

Every conversation turn that flows through the runtime server is:

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

Before each turn, the runtime server performs relevance-gated knowledge injection:

1. Embed the user's current message
2. Search the knowledge base for semantically similar prior interactions
3. Score results by: relevance x recency x importance (pinned items score highest)
4. Inject top results into the system prompt as a "prior context" block
5. Respect a configurable token budget (default: max 20% of context window for injections)

The user does not see the injection unless they ask (`/context` shows what was injected). The model sees prior decisions, approaches, and context as if the user had explicitly provided them.

**Project scoping**: knowledge queries prefer results from the current project. Cross-project results are included at lower relevance scores, enabling "what did I decide about auth in any project?" while prioritizing "what did I decide about auth in this project?"

### Lossless Compaction

When the runtime server compacts conversation context (FEAT-0008 context window management):

1. Identify low-value segments (old turns, resolved tangents, repeated instructions)
2. Summarize them for the live context window
3. The full original remains in storage and the knowledge layer
4. If compacted content becomes relevant in a later turn, the runtime server can re-retrieve it from the knowledge base and inject it

This makes compaction reversible from the user's perspective. Nothing is lost — it moves from working memory to searchable long-term memory.

### Knowledge Commands

| Command | Description |
|---------|-------------|
| `/context` | Show files in context, knowledge injections, and token budget |
| `/pin <text>` | Mark a decision or constraint as always-carry-forward |
| `/unpin <text>` | Remove a pinned item |
| `/forget <query>` | Suppress knowledge entries from retrieval (see Forget Semantics) |
| `/search <query>` | Explicit semantic search across knowledge base |
| `/knowledge stats` | Show knowledge base size, embedding coverage, recent extractions |

### Metadata Extraction

The runtime server extracts structured metadata from conversations:

- **Decisions**: "We decided to use JWT instead of sessions"
- **Action items**: "TODO: add rate limiting to the auth endpoint"
- **Topics**: classification of conversation subjects
- **Files referenced**: which files were discussed or modified
- **People mentioned**: names and roles referenced in conversation

Extraction is model-driven: the runtime server prompts a cheap/fast model to extract structured fields from completed turns. Extraction happens asynchronously — it does not block the conversation.

### Forget Semantics

`/forget` is **suppression from retrieval**, not deletion of captured data. This respects ADR-0005 (always full capture, retention-based pruning):

- Forgotten entries are marked with a `suppressed` flag in the knowledge tables.
- Suppressed entries are excluded from semantic search results and context injection.
- The raw capture in the request log (ADR-0005) is **not modified** — it remains for audit and compliance purposes.
- Knowledge rebuild (`modeltap knowledge rebuild`) respects suppression markers — forgotten entries are re-embedded but remain suppressed from retrieval.
- Suppression is durable across rebuilds via a `suppressed_entries` table that maps capture IDs to suppression timestamps and user-provided reasons.
- The user can un-suppress via `/remember <query>` if they change their mind.

This design ensures `/forget` is honest: the content is not surfaced in future interactions, but the raw capture record is preserved per the capture policy. True deletion occurs only through retention-based pruning (ADR-0005) when content ages past the configured retention period.

### Graceful Degradation

The knowledge layer degrades gracefully when components are unavailable:

- **No embedding model available**: knowledge layer operates in keyword-search-only mode. Capture continues. Embeddings are generated when a model becomes available (backfill queue).
- **Knowledge disabled** (`knowledge.enabled: false`): capture continues. No embedding, no injection, no search. The harness works as a standard multi-model terminal tool.
- **Knowledge database corrupted**: the runtime server falls back to no-knowledge mode. The capture layer can rebuild the knowledge base from the raw request log.

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
- **Context injection for external MCP clients**: the MCP stdio interface (ADR-0009) exposes search tools, but does not perform automatic injection. Injection is a runtime server-to-model feature.
- **Custom embedding models**: the feature supports configurable embedding models, but does not build a model management UI. Users configure the model name in YAML.

## Success Criteria

1. Conversations are automatically embedded within 30 seconds of turn completion when an embedding model is available. **Test**: submit a turn, wait 30s, query the embedding table — the turn's embedding exists.
2. Semantic search returns relevant results using a fixed benchmark. **Test**: seed the knowledge base with 20 known interactions covering 5 topics. For each topic, run a semantic query using synonyms (not exact keywords). The correct topic's interactions must appear in the top 5 results at least 80% of the time.
3. Context enrichment injects relevant prior context. **Test**: seed knowledge with a decision ("use JWT for auth"). In a new session, ask about authentication. Verify the system prompt sent to the provider contains the JWT decision in the knowledge injection block.
4. Compacted content is retrievable. **Test**: create a session with 10 turns, compact it, then query for content from a compacted turn. The knowledge search returns the original content.
5. Pinned items appear in every turn's injected context. **Test**: pin an item, submit 3 unrelated turns. Verify the pinned item is present in the system prompt for all 3 turns.
6. `/forget` suppresses entries from retrieval. **Test**: embed a turn, verify it appears in search, run `/forget`, verify it no longer appears in search or injection. Verify the raw capture record is unchanged. Run `knowledge rebuild`, verify the entry remains suppressed.
7. The knowledge layer degrades gracefully when no embedding model is available. **Test**: start with no embedding model configured. Verify conversations work, embedding queue grows, keyword search returns results. Add an embedding model. Verify the backfill queue processes pending items within 60 seconds.
8. Knowledge features respect per-user isolation (FEAT-0010). **Test**: user A embeds conversations. User B's search returns zero results from user A's knowledge. User A's injection does not include user B's content.
9. Injection budget is respected. **Test**: configure a 20% budget on a model with 100K context. Seed knowledge with 50K tokens of relevant content. Verify injected knowledge does not exceed 20K tokens.
10. The embedding backfill queue processes historical captures when an embedding model becomes available for the first time. **Test**: capture 50 interactions with knowledge disabled. Enable knowledge with an embedding model. Verify all 50 interactions are embedded within 5 minutes.

## Relationship to ADRs

| ADR | Relationship |
|-----|-------------|
| ADR-0005 (Capture) | Knowledge layer builds on full-fidelity capture — embeddings are generated from captured content |
| ADR-0008 (sqlite-vec) | Vector storage and search use sqlite-vec. Amendment proposed: on by default. |
| ADR-0009 (MCP stdio) | MCP interface exposes search_knowledge and related tools for external clients. Injection is runtime-internal. |

## Open Questions

1. **Injection quality measurement**: how do we measure whether knowledge injection actually improves response quality? A/B comparison with injection on/off? User feedback signals?
2. **Negative knowledge**: should the knowledge layer distinguish "facts learned" from "approaches that failed"? Failed approaches are valuable context but should be framed differently.
3. **Scale**: sqlite-vec brute-force KNN is 50-200ms at 100k vectors. At 200 interactions/day, 100k is ~500 days. When should HNSW indexing be evaluated?
4. **Extraction accuracy**: model-driven metadata extraction will have errors. Should extracted metadata be user-reviewable? Or is best-effort acceptable for search ranking?
