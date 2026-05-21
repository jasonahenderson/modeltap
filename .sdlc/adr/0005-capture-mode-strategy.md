---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0005: Capture Mode Strategy

## Context and Problem Statement

Modeltap captures every request and response flowing through the proxy. Full request and response bodies for model API calls can be large (10–100KB per response), and months of usage could consume significant disk space. Users need a default that prioritizes debugging value while providing an escape valve for disk pressure.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Usefulness for debugging and replay (5):** A primary use case is understanding what happened in a specific interaction — what was sent, what came back. Discarding body content reduces this core value.
* **D2 – Disk usage control (5):** Full request/response bodies are large. Users must have a way to control storage growth without waiting for retention to kick in.
* **D3 – Metrics accuracy (4):** Usage metrics — token counts, cost, latency — must be captured regardless of mode. Modes should never lose metadata even when they reduce body storage.
* **D4 – Implementation simplicity (4):** The capture pipeline (SSE reassembly → storage) is the most complex part of modeltap. Capture modes should not make it significantly harder to maintain or test.
* **D5 – Runtime configurability (3):** Users should be able to change capture behavior via config or flag without restarting the proxy.

## Decision Outcome

**Capture full by default, with a `summary` mode and retention-based pruning.**

Two capture modes:

- **`full` (default):** Store complete request and response bodies. Every interaction can be inspected, replayed, or debugged within the retention window.
- **`summary`:** Store headers, usage metadata, and a truncated response body (first 500 characters). Significantly reduces disk usage while preserving enough context for log browsing.

Retention-based pruning (`retention_days`, default 30) runs as a background goroutine and periodically deletes old records.

### Why this approach

The key tradeoff is between data completeness and disk pressure. Full capture is the right default because modeltap's core value is showing you exactly what happened. But "always full with no alternative" forces heavy users into 1–2 GB/month with no recourse other than reducing retention days.

Adding a single `summary` mode gives users an escape valve. The implementation is minimal — one conditional in the write path:

```go
if captureMode == "summary" {
    body = body[:min(500, len(body))]
}
```

This avoids the complexity of per-route rules, pluggable pipelines, or multiple granular modes. Two modes, one config value, one `if` statement.

### Consequences

* Good, because full capture is the default — the core debugging and replay value is preserved without configuration.
* Good, because summary mode gives users immediate control over disk pressure without waiting for retention.
* Good, because the capture pipeline stays simple — one conditional, not a mode framework.
* Good, because metrics (token counts, cost, latency, status) are always captured regardless of mode.
* Neutral, because retention pruning adds a background component, but it is a simple periodic `DELETE`.
* Bad, because summary mode discards data irreversibly — users cannot recover full bodies for requests captured in summary mode.
* Bad, because there is no per-provider granularity — it is all-or-nothing. This can be added later if users request it.

### Confirmation

The decision will be confirmed by:

1. Implementing both capture modes with the `--capture-mode` flag and `capture_mode` config key.
2. Implementing the retention pruner as a background goroutine.
3. Verifying that summary mode meaningfully reduces disk usage (expected: ~90% reduction vs full).
4. Shipping a `modeltap status` command that reports current database size and retention settings.

### Disk Usage Estimates

| Usage Level | Calls/day | Avg Response Size | Daily Growth (full) | Daily Growth (summary) | Monthly (30-day, full) |
|-------------|-----------|-------------------|---------------------|------------------------|------------------------|
| Light       | 50        | 50 KB             | 2.5 MB              | ~0.25 MB               | 75 MB                  |
| Moderate    | 200       | 50 KB             | 10 MB               | ~1 MB                  | 300 MB                 |
| Heavy       | 1000      | 50 KB             | 50 MB               | ~5 MB                  | 1.5 GB                 |
