---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0007: Usage Metrics Tracking and Reporting

## Context and Problem Statement

Modeltap captures every model API interaction. Users need to understand their usage patterns — how many tokens they consume, what it costs, which models they use most, and how performance varies. The decision is what metrics to track, how to aggregate them, and how to expose them to users. This spans storage schema design, aggregation strategy, and reporting interface.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Cost visibility (5):** Token-based pricing makes model API costs opaque. Users need clear, accurate cost reporting broken down by provider, model, and time period. This is often the primary reason someone installs a tool like modeltap.
* **D2 – Accuracy of token and cost tracking (5):** Metrics must come from actual API responses, not estimates. Incorrect token counts or cost calculations undermine trust in the tool.
* **D3 – Query performance for aggregated metrics (4):** Users will query metrics frequently — daily cost, weekly model breakdown, monthly totals. Aggregation queries must be fast even over large datasets.
* **D4 – Multi-provider normalization (4):** Different providers report usage differently. Anthropic reports `input_tokens`/`output_tokens`. OpenAI reports `prompt_tokens`/`completion_tokens`. Metrics must normalize these into a consistent schema.
* **D5 – Extensibility for new metrics (3):** As modeltap evolves, new metrics will be needed — latency percentiles, error rates, cache hit rates. The schema and reporting system should accommodate new metrics without breaking changes.
* **D6 – Export and integration (3):** Users will want to send metrics to external systems — dashboards, spreadsheets, cost management tools. Standard export formats matter.
* **D7 – Real-time vs batch reporting (2):** Some users want live dashboards; others are fine with periodic reports. The architecture should not preclude either, but real-time is not required for v1.

## Considered Options

* Derive metrics from raw request logs at query time
* Pre-computed aggregation tables alongside raw logs
* Metrics stored in raw logs with materialized views
* Separate time-series store (e.g., Prometheus-style)

## Decision Outcome

Chosen option: **Pre-computed aggregation tables alongside raw logs**, because it achieves the highest weighted score (113) and provides the best balance between query performance and accuracy. Raw request logs contain the source-of-truth data (per ADR-0005, always full capture). Aggregation tables are updated on each request write and provide fast, indexed access to rollups by hour, day, provider, and model without scanning the full request table.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                              | Weight | Query-time derivation | Pre-computed agg | Materialized views | Separate time-series |
|-------------------------------------|--------|----------------------|------------------|--------------------|---------------------|
| D1: Cost visibility                 | 5      | 4                    | 5                | 5                  | 5                   |
| D2: Accuracy                        | 5      | 5                    | 5                | 5                  | 4                   |
| D3: Query performance               | 4      | 2                    | 5                | 4                  | 5                   |
| D4: Multi-provider normalization    | 4      | 4                    | 4                | 4                  | 4                   |
| D5: Extensibility                   | 3      | 5                    | 3                | 4                  | 4                   |
| D6: Export and integration          | 3      | 3                    | 4                | 4                  | 5                   |
| D7: Real-time vs batch              | 2      | 3                    | 4                | 3                  | 5                   |
| **Weighted Total**                  |        | **101**              | **113**          | **111**            | **112**             |

### Scoring Justification

#### Query-time derivation (101)

* **D1 (4):** Can compute any cost breakdown by querying raw logs. But complex aggregations (monthly cost by model) require scanning many rows each time.
* **D2 (5):** Maximum accuracy — always computed from source-of-truth data. No risk of stale or inconsistent aggregates.
* **D3 (2):** Full table scans for aggregation queries. At 50k+ records, `SELECT SUM(cost) FROM requests WHERE timestamp > ? GROUP BY model` becomes slow. No pre-computed indexes for common rollups.
* **D4 (4):** Normalization happens at query time. Can be done in SQL or in Go. Works but repeated across every query.
* **D5 (5):** Adding a new metric means adding a new query. No schema changes needed. Maximum flexibility.
* **D6 (3):** Export requires running aggregation queries, which are slow on large datasets.
* **D7 (3):** Real-time possible but each dashboard refresh runs expensive queries.

#### Pre-computed aggregation tables (113)

* **D1 (5):** Aggregation tables provide instant access to cost breakdowns. `SELECT * FROM daily_usage WHERE date = '2026-03-03'` is a single indexed lookup.
* **D2 (5):** Aggregates are updated atomically with each request write (same transaction). Source-of-truth remains in raw logs; aggregates are derived but always consistent.
* **D3 (5):** Aggregation queries hit small, indexed tables. Hourly and daily rollups are pre-computed. Even complex multi-dimension queries (cost by provider by model by day) are fast.
* **D4 (4):** Normalization happens once at write time when populating the aggregation table. Consistent schema across providers.
* **D5 (3):** Adding a new metric requires a schema migration to the aggregation table and updating the write-time aggregation logic. More work than query-time derivation but well-defined.
* **D6 (4):** Pre-computed tables are trivially exportable. `modeltap metrics export --format csv --since 30d` reads from the aggregation table directly.
* **D7 (4):** Near-real-time — aggregates are updated on each request. Dashboard can poll the aggregation table cheaply.

#### Materialized views (111)

* **D1 (5):** Same query performance benefits as pre-computed tables when views are fresh.
* **D2 (5):** Views are derived from raw logs. Accuracy matches the source data when refreshed.
* **D3 (4):** Fast when views are fresh. But SQLite does not natively support materialized views — they must be simulated with tables and triggers, or refreshed periodically. Stale views return stale data.
* **D4 (4):** Same normalization as pre-computed tables.
* **D5 (4):** Adding metrics means updating the view definition. Slightly easier than manual aggregation tables.
* **D6 (4):** Same export capability as pre-computed tables.
* **D7 (3):** Depends on refresh frequency. Periodic refresh means data can be minutes stale.

#### Separate time-series store (112)

* **D1 (5):** Purpose-built for metrics queries. Excellent for time-range aggregations and dashboards.
* **D2 (4):** Metrics are written at capture time, same as pre-computed tables. But a separate store introduces the risk of the metrics store and request log diverging (e.g., if one write succeeds and the other fails).
* **D3 (5):** Time-series databases are optimized for exactly these queries. Best raw performance.
* **D4 (4):** Same normalization at write time.
* **D5 (4):** Time-series stores handle new metrics naturally — just add a new metric name.
* **D6 (5):** Native Prometheus endpoint or similar makes integration with Grafana, Datadog, etc. trivial.
* **D7 (5):** Purpose-built for real-time dashboards and alerting.

### Consequences

* Good, because common metrics queries (daily cost, model breakdown, monthly totals) are instant indexed lookups rather than full table scans.
* Good, because aggregation tables are updated atomically with request writes, ensuring consistency between raw logs and metrics.
* Good, because the CLI can offer fast `modeltap metrics` commands that read from pre-computed data.
* Good, because the approach stays within SQLite (per ADR-0002) — no additional storage engine needed.
* Neutral, because adding new metrics requires schema migration and aggregation logic updates, but this is a well-defined, contained task.
* Bad, because the aggregation logic adds complexity to the write path — each request write must also update rollup tables.
* Bad, because if aggregation tables become corrupted or out of sync, they must be rebuilt from raw logs (a supported but slow operation).

### Confirmation

The decision will be confirmed by:

1. Implementing hourly and daily aggregation tables with columns for provider, model, token counts (input/output), estimated cost, request count, and average latency.
2. Verifying that `modeltap metrics --since 30d --group-by model` returns results in under 100ms on a database with 50k+ records.
3. Shipping cost estimation based on published provider pricing, with a clear disclaimer that estimates may not match actual bills exactly.
4. Confirming that aggregation tables can be rebuilt from raw logs via `modeltap metrics rebuild`.

## More Information

The decision aligns with the weighted scoring matrix. The margin is narrow (113 vs 112 for separate time-series), but pre-computed aggregation tables win because they stay within the SQLite ecosystem established in ADR-0002, avoiding a second storage engine.

### Metrics Schema (approximate)

```sql
CREATE TABLE hourly_usage (
    hour          TEXT NOT NULL,    -- '2026-03-03T14:00:00Z'
    provider      TEXT NOT NULL,    -- 'anthropic', 'openai'
    model         TEXT NOT NULL,    -- 'claude-opus-4-20250514'
    request_count INTEGER DEFAULT 0,
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    estimated_cost_usd REAL DEFAULT 0.0,
    avg_latency_ms REAL DEFAULT 0.0,
    error_count   INTEGER DEFAULT 0,
    PRIMARY KEY (hour, provider, model)
);

CREATE TABLE daily_usage (
    date          TEXT NOT NULL,
    provider      TEXT NOT NULL,
    model         TEXT NOT NULL,
    request_count INTEGER DEFAULT 0,
    input_tokens  INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    estimated_cost_usd REAL DEFAULT 0.0,
    avg_latency_ms REAL DEFAULT 0.0,
    error_count   INTEGER DEFAULT 0,
    PRIMARY KEY (date, provider, model)
);
```

Cost estimation will use a configurable pricing table that can be updated without code changes:

```yaml
pricing:
  anthropic:
    claude-opus-4:
      input_per_mtok: 15.00
      output_per_mtok: 75.00
    claude-sonnet-4:
      input_per_mtok: 3.00
      output_per_mtok: 15.00
  openai:
    gpt-4o:
      input_per_mtok: 2.50
      output_per_mtok: 10.00
```
