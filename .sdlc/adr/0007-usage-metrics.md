---
status: superseded
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0007: Usage Metrics Tracking and Reporting

## Status

**Superseded** — this ADR originally chose pre-computed aggregation tables for metrics. After review, the decision was reversed: modeltap will derive metrics from raw request logs at query time.

## Decision

Derive all usage metrics (token counts, cost, model breakdown, latency) from the raw `requests` table at query time using SQL aggregation queries.

### Why not pre-computed aggregation tables

- SQLite can aggregate 50k+ rows with indexed columns in milliseconds. For a single-user dev tool, query-time derivation is fast enough.
- Pre-computed tables add write-path complexity (every insert must atomically update rollup tables) for a performance problem that does not exist at this scale.
- Eliminating the `hourly_usage` and `daily_usage` tables removes schema to maintain, migration logic, and a `modeltap metrics rebuild` command.

### Implementation

Add indexes on `timestamp`, `provider`, and `model` to the `requests` table. Metrics queries use standard SQL aggregation:

```sql
-- Daily cost by model
SELECT
    date(timestamp) as date,
    provider,
    model,
    COUNT(*) as request_count,
    SUM(input_tokens) as input_tokens,
    SUM(output_tokens) as output_tokens,
    SUM(estimated_cost_usd) as cost
FROM requests
WHERE timestamp > ?
GROUP BY date, provider, model
ORDER BY date DESC;
```

If aggregation queries become slow at scale (unlikely for a single-user tool), pre-computed tables can be added at that point with real query patterns to optimize for.

### Cost estimation

Cost estimation will use a configurable pricing table in the config file:

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

Cost estimates are clearly labeled as estimates — they may not match actual provider bills exactly.
