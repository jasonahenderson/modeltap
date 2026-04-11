---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0008: Knowledge Layer Architecture

## Context and Problem Statement

Modeltap captures every model API interaction across all providers (ADR-0006). This captured data — full request/response bodies (ADR-0005) with usage metrics (ADR-0007) — creates the foundation for a cross-model knowledge layer: a persistent, searchable memory system that works across every AI tool a user interacts with. The decision is how to architect this knowledge layer — how to generate embeddings, store vectors, extract metadata, and make all of it queryable — as an optional module on top of the proxy core.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Separation from proxy core (5):** The knowledge layer must be fully optional. Users who want a simple logging proxy should not pay any performance, disk, or dependency cost for knowledge features. A tight coupling would undermine modeltap's value as a lightweight tool.
* **D2 – Embedding quality and flexibility (5):** Semantic search quality depends directly on embedding quality. Users must be able to choose their embedding model — cloud (OpenAI, Anthropic) or local (Ollama) — depending on their privacy, cost, and quality preferences.
* **D3 – Query performance for semantic search (4):** Users will search their knowledge base frequently. Vector similarity search must return results in under 500ms for databases with 100k+ embedded interactions.
* **D4 – Privacy and local-only operation (4):** Some users will refuse to send captured data to cloud embedding services. The architecture must support fully local operation using local embedding models (Ollama, sentence-transformers).
* **D5 – Incremental embedding (4):** Embedding must happen asynchronously after capture, not in the request hot path. New interactions should be embedded within seconds of capture, not batched for hours.
* **D6 – Metadata extraction capability (3):** Beyond raw embedding, the knowledge layer should extract structured metadata — decisions, action items, topics, people — to enable filtered queries like "show me all decisions from last week."
* **D7 – Storage consistency with proxy core (3):** The knowledge layer should integrate cleanly with SQLite (ADR-0002) rather than requiring a separate database engine, keeping operational complexity low.

## Considered Options

* SQLite with vector extension (sqlite-vec)
* Separate vector database (Qdrant, ChromaDB)
* Hybrid: SQLite metadata + embedded vector index (FAISS/hnswlib)
* Full-text search only (no vectors)

## Decision Outcome

Chosen option: **SQLite with vector extension (sqlite-vec)**, because it achieves the highest weighted score (119) and maintains the single-database architecture established in ADR-0002. sqlite-vec is a loadable SQLite extension that adds vector similarity search directly into the SQLite query engine, meaning embeddings are stored alongside request logs in the same database file, queried with SQL, and backed up with a single file copy.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                              | Weight | SQLite + sqlite-vec | Separate vector DB | Hybrid SQLite + FAISS | Full-text only |
|-------------------------------------|--------|--------------------|--------------------|----------------------|----------------|
| D1: Separation from core            | 5      | 5                  | 4                  | 4                    | 5              |
| D2: Embedding flexibility           | 5      | 4                  | 5                  | 5                    | 1              |
| D3: Query performance               | 4      | 4                  | 5                  | 5                    | 3              |
| D4: Privacy / local-only            | 4      | 5                  | 3                  | 4                    | 5              |
| D5: Incremental embedding           | 4      | 4                  | 5                  | 4                    | 5              |
| D6: Metadata extraction             | 3      | 4                  | 3                  | 4                    | 2              |
| D7: Storage consistency             | 3      | 5                  | 1                  | 3                    | 5              |
| **Weighted Total**                  |        | **119**            | **109**            | **116**              | **96**         |

### Scoring Justification

#### SQLite + sqlite-vec (119)

* **D1 (5):** The knowledge layer is a set of additional tables and a background goroutine. When disabled, no vector tables are created, no embedding goroutine runs, and the proxy core is unchanged. The sqlite-vec extension is only loaded when the knowledge layer is enabled.
* **D2 (4):** Embedding model is configurable — any model that produces float32 vectors works. sqlite-vec stores vectors as BLOBs and performs cosine similarity. The only constraint is that all embeddings must use the same dimensionality within a table, which is standard. Scores 4 instead of 5 because sqlite-vec is newer and has fewer community examples for advanced embedding workflows.
* **D3 (4):** sqlite-vec uses brute-force KNN for exact nearest neighbor search. At 100k vectors with 1536 dimensions, queries complete in ~50-200ms depending on hardware. Not as fast as HNSW-based indexes in purpose-built vector databases, but well within the 500ms target. Approximate nearest neighbor (ANN) support is on the sqlite-vec roadmap.
* **D4 (5):** Everything stays in a single local SQLite file. No network calls, no external processes, no cloud dependencies. Embedding can use local Ollama models for fully air-gapped operation.
* **D5 (4):** A background goroutine watches for new unembedded interactions and processes them. Embedding is async and does not block the proxy. Slightly less sophisticated than purpose-built vector databases that have native change-data-capture, but functionally equivalent for modeltap's throughput.
* **D6 (4):** Metadata extraction results are stored in SQLite tables alongside vectors. SQL joins between metadata tables and vector search results are trivial. Strong integration.
* **D7 (5):** Same database, same backup, same `modeltap.db` file. Maximum consistency with ADR-0002.

#### Separate vector database (109)

* **D1 (4):** A separate database process (Qdrant, ChromaDB) must be installed and running alongside modeltap. Even if optional, this is a significant operational burden compared to a single binary. Users must manage two processes.
* **D2 (5):** Purpose-built vector databases support multiple embedding models, dynamic dimensionality, and advanced features like multi-vector search and filtering. Maximum flexibility.
* **D3 (5):** HNSW indexes provide sub-millisecond approximate nearest neighbor search at any scale. Best raw query performance.
* **D4 (3):** Qdrant and ChromaDB can run locally, but they are separate processes with their own storage. Not as simple as a single file. Some vector databases have cloud-first architectures that make local operation a second-class citizen.
* **D5 (5):** Purpose-built for streaming inserts and real-time indexing. Best incremental embedding support.
* **D6 (3):** Vector databases are optimized for vector operations, not relational queries. Metadata filtering is supported but less expressive than SQL. Joining vector results with request log metadata requires cross-database queries.
* **D7 (1):** Completely separate storage engine. Two databases to back up, two to migrate, two failure modes. Directly contradicts ADR-0002's single-database architecture.

#### Hybrid SQLite + FAISS (116)

* **D1 (4):** FAISS is an in-memory library, not a database. The index must be built in memory at startup and persisted to disk. This adds startup latency and memory usage proportional to the number of vectors.
* **D2 (5):** FAISS supports multiple index types and embedding dimensions. Maximum flexibility for embedding model choice.
* **D3 (5):** FAISS provides state-of-the-art ANN search performance. Sub-millisecond queries at scale.
* **D4 (4):** Everything stays local. FAISS index is a file on disk alongside SQLite. But two storage formats means two backup targets.
* **D5 (4):** New vectors can be added to a FAISS index incrementally, but some index types require periodic rebuilding for optimal performance.
* **D6 (4):** Metadata stays in SQLite with full SQL expressiveness. Vector search in FAISS, metadata filtering in SQLite, results joined in Go code.
* **D7 (3):** SQLite handles metadata but vectors live in a separate FAISS index file. Partial consistency — not a separate database engine, but not a single file either.

#### Full-text search only (96)

* **D1 (5):** FTS5 is built into SQLite. No additional dependencies, no extension loading. Simplest possible implementation.
* **D2 (1):** No vector embeddings means no semantic search. Users can only find interactions by keyword match. "What did I decide about authentication?" fails if the word "authentication" was never used — the interaction might have discussed "login flow" or "OAuth" instead.
* **D3 (3):** FTS5 is fast for keyword queries but cannot rank by semantic similarity. Results are keyword-match ranked, which is less useful for knowledge retrieval.
* **D4 (5):** Fully local, no external dependencies.
* **D5 (5):** FTS5 indexes are updated on insert. No async processing needed.
* **D6 (2):** Full-text search cannot extract structured metadata. It finds text matches, not meaning. Limited utility for "show me decisions" type queries.
* **D7 (5):** Native SQLite feature. Maximum consistency.

### Consequences

* Good, because the entire knowledge base (request logs, embeddings, metadata) lives in a single SQLite file, maintaining the operational simplicity established in ADR-0002.
* Good, because the knowledge layer is fully optional — disabled by default, enabled via configuration, with zero cost when disabled.
* Good, because sqlite-vec uses standard SQL for vector queries, meaning existing SQLite tooling (backup, migration, debugging) works unchanged.
* Good, because embedding model choice is fully configurable, supporting both cloud APIs and local models for privacy-sensitive deployments.
* Neutral, because sqlite-vec is a newer extension with a smaller community than purpose-built vector databases, but it is actively maintained and production-ready.
* Bad, because sqlite-vec's brute-force KNN becomes slower at very large scale (500k+ vectors). If modeltap users hit this limit, an ANN index or migration to a hybrid approach may be needed.
* Bad, because the sqlite-vec extension must be distributed alongside the modeltap binary, adding build complexity for cross-platform releases.

### Confirmation

The decision will be confirmed by:

1. Successfully integrating sqlite-vec into modeltap's build and loading it conditionally when the knowledge layer is enabled.
2. Demonstrating semantic search across 10k+ embedded interactions with query latency under 500ms.
3. Verifying that the knowledge layer can be fully disabled with zero impact on proxy core performance and zero additional disk usage.
4. Confirming that both cloud (OpenAI) and local (Ollama) embedding models work interchangeably.

## More Information

The decision aligns with the weighted scoring matrix. The margin over the hybrid approach (119 vs 116) is narrow, but sqlite-vec wins because it maintains the single-file database architecture that is central to modeltap's operational simplicity.

### Knowledge Layer Schema (approximate)

```sql
-- Vector embeddings for captured interactions
CREATE VIRTUAL TABLE IF NOT EXISTS interaction_embeddings USING vec0(
    request_id TEXT PRIMARY KEY,
    embedding FLOAT[1536]  -- dimensionality matches embedding model
);

-- Extracted metadata from interactions
CREATE TABLE IF NOT EXISTS interaction_metadata (
    request_id    TEXT PRIMARY KEY,
    summary       TEXT,          -- one-line summary of the interaction
    topics        TEXT,          -- JSON array of extracted topics
    decisions     TEXT,          -- JSON array of decisions made
    action_items  TEXT,          -- JSON array of action items
    people        TEXT,          -- JSON array of people mentioned
    project       TEXT,          -- inferred project context
    extracted_at  TEXT NOT NULL, -- ISO 8601 timestamp
    FOREIGN KEY (request_id) REFERENCES requests(id)
);

-- Embedding processing queue
CREATE TABLE IF NOT EXISTS embedding_queue (
    request_id    TEXT PRIMARY KEY,
    status        TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    attempts      INTEGER DEFAULT 0,
    last_error    TEXT,
    created_at    TEXT NOT NULL,
    completed_at  TEXT,
    FOREIGN KEY (request_id) REFERENCES requests(id)
);
```

### Embedding Pipeline

```
New Request Captured
        │
        ▼
  ┌─────────────┐
  │  Write to    │
  │  SQLite log  │
  │  (sync)      │
  └──────┬───────┘
         │
         ▼
  ┌─────────────┐
  │  Enqueue for │
  │  embedding   │
  │  (async)     │
  └──────┬───────┘
         │
         ▼
  ┌─────────────────────┐
  │  Background Worker   │
  │  1. Dequeue          │
  │  2. Call embed model │
  │  3. Store vector     │
  │  4. Extract metadata │
  │  5. Mark complete    │
  └─────────────────────┘
```

If sqlite-vec's performance becomes insufficient at scale, the migration path is to swap in FAISS or hnswlib for the vector index while keeping metadata in SQLite — the hybrid approach scored 116 and serves as the natural fallback.
