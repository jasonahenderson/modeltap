---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# Storage Format for Request/Response Logs

## Context and Problem Statement

Modeltap captures every request and response flowing between AI/ML clients and API endpoints. This data — including full request bodies, reassembled streamed responses, and usage metadata — must be stored durably and made queryable for analysis, debugging, and cost tracking. The storage format must work within a single-binary deployment model (per ADR-0001) and support a provider interface so users can implement alternative storage backends.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Query capability (5):** Users need to search and filter logs by time range, model, status code, and token count. This is the primary way people interact with stored data daily. A format that cannot be queried efficiently makes the tool frustrating to use.
* **D2 – Zero-dependency deployment (5):** Consistent with ADR-0001's emphasis on single-binary distribution. The storage engine must not require a separate server process or external installation.
* **D3 – Write performance under streaming (4):** Every proxied request involves reassembling an SSE stream and writing the result. Writes must not block the proxy or introduce latency in the forwarded response.
* **D4 – Read performance at scale (4):** Months of heavy LLM usage could produce tens of thousands of records with large response bodies. Listing, filtering, and reading must stay fast as data grows.
* **D5 – Schema evolution and migration (3):** The schema will change as we add multi-provider support (ADR-0006), new metrics fields (ADR-0007), and capture modes (ADR-0005). Adding columns or fields should not require painful migrations.
* **D6 – Export and interoperability (3):** Users will want to pipe data into other tools — `jq`, spreadsheets, custom scripts, LLM cost dashboards. The format should support easy data extraction.
* **D7 – Human readability of raw storage (2):** Nice if you can inspect logs without the CLI (e.g., on a server, in a script), but the CLI will be the primary interface so this is a tiebreaker, not a driver.
* **D8 – Ecosystem tooling (2):** Availability of viewers, editors, and third-party tools that can read the format directly without modeltap.

## Considered Options

* SQLite
* JSON Lines (.jsonl)
* SQLite with JSONL export
* BoltDB (bbolt)

## Decision Outcome

Chosen option: **SQLite with JSONL export capability**, because it achieves the highest weighted score (123) with a clear margin over all alternatives. It combines full SQL query capability with on-demand interoperability through a built-in export command, while maintaining zero-dependency deployment via a pure Go SQLite driver.

The storage layer will be implemented behind a **provider interface** so that users can implement alternative storage backends (e.g., JSONL flat file, PostgreSQL, cloud storage) as needed. SQLite with JSONL export serves as the default provider.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                             | Weight | SQLite | JSONL | SQLite + JSONL export | BoltDB |
|------------------------------------|--------|--------|-------|-----------------------|--------|
| D1: Query capability               | 5      | 5      | 1     | 5                     | 2      |
| D2: Zero-dependency deployment     | 5      | 5      | 5     | 5                     | 5      |
| D3: Write performance under streaming | 4   | 4      | 5     | 4                     | 4      |
| D4: Read performance at scale      | 4      | 5      | 2     | 5                     | 3      |
| D5: Schema evolution               | 3      | 4      | 5     | 4                     | 3      |
| D6: Export / interoperability      | 3      | 3      | 5     | 5                     | 2      |
| D7: Human readability              | 2      | 2      | 5     | 2                     | 1      |
| D8: Ecosystem tooling              | 2      | 5      | 4     | 5                     | 2      |
| **Weighted Total**                 |        | **117**| **101** | **123**             | **82** |

### Scoring Justification

#### SQLite (117)

* **D1 (5):** Full SQL — `SELECT * FROM requests WHERE model LIKE '%opus%' AND timestamp > ? ORDER BY timestamp DESC`. Indexes make filtered queries fast.
* **D2 (5):** `modernc.org/sqlite` is a pure Go translation of SQLite. Compiles into the binary, no CGO, no external dependencies.
* **D3 (4):** WAL mode supports concurrent readers and a single writer without blocking. Writes are fast but slightly more overhead than a raw file append due to journaling and indexing.
* **D4 (5):** B-tree indexes on timestamp, model, and status code keep queries fast at 100k+ rows. SQLite is proven at this scale and well beyond it.
* **D5 (4):** `ALTER TABLE ADD COLUMN` works. Adding nullable columns is trivial. More complex migrations (renaming, restructuring) require migration tooling, but for an additive schema this is straightforward.
* **D6 (3):** Data is accessible via any SQLite client, but users wanting JSONL or CSV need to either use the CLI or know SQL. Not zero-friction for piping to `jq`.
* **D7 (2):** Binary file — cannot be inspected with `cat` or a text editor. Requires `sqlite3` CLI or a viewer.
* **D8 (5):** Massive ecosystem — DB Browser for SQLite, `sqlite3` CLI, Datasette, every programming language has bindings. Most widely deployed database engine in the world.

#### JSON Lines (101)

* **D1 (1):** No query engine. Filtering requires reading every line and parsing JSON. Time-range queries mean scanning the entire file. Untenable at scale.
* **D2 (5):** It is a flat file. No dependencies whatsoever.
* **D3 (5):** Append is the fastest possible write operation — open file, write line, done. No journaling, no indexing, no overhead.
* **D4 (2):** Every query is a full file scan. At 50k records with large response bodies, listing recent requests becomes painfully slow.
* **D5 (5):** Schema-less — just add new fields to new records. Old records without the field still parse fine. Zero migration effort.
* **D6 (5):** Native format for `jq`, `grep`, streaming pipelines. Every tool in the Unix ecosystem handles newline-delimited JSON.
* **D7 (5):** Open the file in any text editor. Pipe to `jq . | less`. Maximum transparency.
* **D8 (4):** Universally supported, though no specialized JSONL viewer — users rely on general-purpose text and JSON tools.

#### SQLite with JSONL export (123)

* **D1 (5):** Same as plain SQLite — full SQL query capability.
* **D2 (5):** Same as plain SQLite — pure Go, no external dependencies.
* **D3 (4):** Same as plain SQLite — WAL mode, slight overhead vs raw append.
* **D4 (5):** Same as plain SQLite — indexed queries stay fast at scale.
* **D5 (4):** Same as plain SQLite — additive schema changes are straightforward.
* **D6 (5):** This is where it pulls ahead of plain SQLite. `modeltap export --format jsonl` gives users native pipeline-friendly output. Can also support `--format csv` trivially. Users get SQL power for daily use and flat-file interop when they need it.
* **D7 (2):** Same as plain SQLite — primary storage is still a binary file.
* **D8 (5):** Same as plain SQLite — massive ecosystem, plus the export command adds CLI-native interoperability.

#### BoltDB / bbolt (82)

* **D1 (2):** Key-value only. Can look up by primary key (request ID) efficiently, but filtering by model, time range, or status requires iterating all records in a bucket and deserializing each one.
* **D2 (5):** Pure Go, no CGO, single file. Designed for embedding in Go applications.
* **D3 (4):** Transactional writes are fast. Similar overhead profile to SQLite for single-record inserts.
* **D4 (3):** Sequential scans through a bucket are faster than JSONL (binary format, memory-mapped) but far slower than indexed SQL queries. No secondary indexes.
* **D5 (3):** Schema lives in Go structs, serialized as bytes. Adding fields means updating serialization logic. No migration tooling — handled manually.
* **D6 (2):** Data is locked in a binary format readable only by bbolt-aware code. No standard export without writing custom tooling.
* **D7 (1):** Opaque binary format. Cannot be inspected without specialized tooling. Even less accessible than SQLite, which at least has a universal CLI.
* **D8 (2):** Small ecosystem. `bbolt` CLI exists for basic inspection but nothing comparable to SQLite's tooling breadth.

### Consequences

* Good, because full SQL querying enables powerful filtering, aggregation, and analysis of LLM usage patterns without custom code.
* Good, because the JSONL export command bridges the interoperability gap, letting users pipe data into `jq`, spreadsheets, and external dashboards.
* Good, because the provider interface allows users to implement alternative storage backends without forking the project.
* Good, because SQLite's WAL mode handles concurrent reads (CLI queries) and writes (proxy logging) without contention.
* Good, because `modernc.org/sqlite` maintains the single-binary deployment model established in ADR-0001.
* Neutral, because the provider interface adds a layer of abstraction, but this is minimal in Go (a small interface) and pays for itself in extensibility.
* Bad, because the raw storage file is a binary format that cannot be inspected with a text editor — users must use the CLI, the export command, or a SQLite client.
* Bad, because `modernc.org/sqlite` (pure Go) is slower than CGO-based SQLite bindings, though the difference is negligible for this workload.

### Confirmation

The decision will be confirmed by:

1. Successfully implementing the `Store` interface with SQLite as the default provider.
2. Demonstrating that write latency does not measurably impact proxy response times under normal usage.
3. Verifying that query performance remains acceptable at 50k+ records with indexed columns.
4. Shipping a working `modeltap export --format jsonl` command.

If write latency becomes a bottleneck under high-concurrency scenarios, the provider interface allows introducing an alternative backend without architectural changes.

## More Information

The decision aligns with the weighted scoring matrix. No override was necessary — SQLite with JSONL export leads on weighted total with no disqualifying weaknesses.

The storage provider interface will be defined as a Go interface, enabling alternative implementations:

```go
type Store interface {
    SaveRequest(ctx context.Context, record *RequestRecord) error
    GetRequest(ctx context.Context, id string) (*RequestRecord, error)
    ListRequests(ctx context.Context, filter *Filter) ([]*RequestRecord, error)
    GetUsageMetrics(ctx context.Context, filter *MetricsFilter) (*UsageMetrics, error)
    Export(ctx context.Context, format ExportFormat, w io.Writer) error
    Close() error
}
```

Community-contributed providers (e.g., PostgreSQL, JSONL flat file, S3) can be developed independently and registered at startup.
