---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# Capture Mode Strategy

## Context and Problem Statement

Modeltap captures every request and response flowing through the proxy. Full request and response bodies for model API calls can be large (10–100KB per response), and months of usage could consume significant disk space. Users need a strategy that balances complete data capture — essential for debugging, replay, and the knowledge layer — against storage growth. The decision is whether to offer multiple capture modes that discard data at write time, or to always capture everything and manage disk via retention policies.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Disk usage control (5):** Full request/response bodies for model API calls are large. Months of usage at full capture could consume gigabytes. Users must be able to control storage growth.
* **D2 – Usefulness for debugging and replay (5):** A primary use case is understanding what happened in a specific interaction — what was sent, what came back. Modes that discard body content reduce this core value.
* **D3 – Knowledge layer compatibility (4):** The knowledge layer (ADR-0008) needs access to response content to generate embeddings and extract metadata. Capture modes that discard bodies limit what the knowledge layer can do.
* **D4 – Metrics accuracy (4):** Usage metrics (ADR-0007) — token counts, cost, latency — must be captured regardless of mode. Modes should never lose metadata even when they reduce body storage.
* **D5 – Runtime configurability (3):** Users should be able to change capture behavior without restarting the proxy, or at least change it between sessions. Baking it in at compile time or requiring a restart is friction.
* **D6 – Per-provider/per-model granularity (3):** Users may want different capture behavior for expensive Opus calls vs cheap Haiku calls, or for production API calls vs local Ollama experiments.
* **D7 – Implementation simplicity (3):** The capture pipeline (SSE reassembly → storage) is the most complex part of modeltap. Adding multiple modes should not make it significantly harder to maintain or test.

## Considered Options

* Fixed modes (full / summary / metadata-only)
* Fixed modes with per-route rules
* Pipeline with pluggable processors
* Always capture full, with retention-based disk management

## Decision Outcome

Chosen option: **Always capture full, with retention-based disk management**, because it achieves the highest weighted score (116) with a significant margin over all alternatives. The key insight is that capture modes which discard data are irreversible — you can never get back a response body you did not store. This directly undermines the two highest-value features of modeltap: debugging/replay and the knowledge layer. Retention-based pruning achieves disk management without sacrificing data completeness within the retention window.

The mental model is the simplest of all options: modeltap captures everything; old data gets pruned automatically.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                              | Weight | Fixed modes | Fixed + per-route | Pipeline | Always full + retention |
|-------------------------------------|--------|-------------|-------------------|----------|------------------------|
| D1: Disk usage control              | 5      | 4           | 4                 | 4        | 3                      |
| D2: Debugging / replay              | 5      | 3           | 3                 | 3        | 5                      |
| D3: Knowledge layer compat          | 4      | 3           | 4                 | 4        | 5                      |
| D4: Metrics accuracy                | 4      | 5           | 5                 | 5        | 5                      |
| D5: Runtime configurability         | 3      | 4           | 3                 | 2        | 5                      |
| D6: Per-provider granularity        | 3      | 1           | 5                 | 5        | 3                      |
| D7: Implementation simplicity       | 3      | 5           | 3                 | 2        | 4                      |
| **Weighted Total**                  |        | **97**      | **103**           | **99**   | **116**                |

### Scoring Justification

#### Fixed modes (97)

* **D1 (4):** Users choose between full/summary/metadata-only. Summary and metadata modes meaningfully reduce storage. Not a 5 because users cannot tune per-provider — it is all or nothing.
* **D2 (3):** In summary or metadata mode, the original response body is gone. You cannot replay or inspect the full interaction. Users who switch to summary mode for disk savings lose debugging capability retroactively for those requests.
* **D3 (3):** In summary mode, only a truncated response is available for embedding. In metadata mode, there is nothing to embed. The knowledge layer is partially or fully disabled depending on mode.
* **D4 (5):** All three modes capture response headers and usage metadata (token counts, latency, status). Metrics are never lost regardless of capture mode.
* **D5 (4):** Changing a single config value or flag switches modes. Viper supports runtime config reload. Simple.
* **D6 (1):** One mode for everything. Cannot capture full for expensive Opus calls and metadata-only for local Ollama. All-or-nothing.
* **D7 (5):** Simplest to implement. A single switch statement in the storage path that truncates or strips the body before writing. Minimal branching.

#### Fixed modes with per-route rules (103)

* **D1 (4):** Same disk control as fixed modes, but more granular — users can save disk on high-volume low-value routes while keeping full capture where it matters.
* **D2 (3):** Same limitation — routes configured for summary or metadata lose debugging capability for those requests.
* **D3 (4):** Better than flat fixed modes because users can ensure full capture for providers that feed the knowledge layer while reducing capture elsewhere. Still lossy for routes configured as summary or metadata.
* **D4 (5):** Same as fixed modes — metadata always captured.
* **D5 (3):** Config file rules add complexity. Changing per-route rules requires editing YAML and potentially reloading config. Not as simple as flipping a single flag.
* **D6 (5):** Full per-provider, per-model granularity. `anthropic/claude-opus-4: full`, `ollama/*: metadata-only`. This is the core advantage.
* **D7 (3):** Route matching logic, rule priority, wildcard patterns — all need to be implemented and tested. More complex than a single mode flag but straightforward pattern matching.

#### Pipeline with pluggable processors (99)

* **D1 (4):** Same disk control as per-route rules, plus finer-grained processing (e.g., truncate to 2000 chars vs 500 chars per route).
* **D2 (3):** Same limitation — processors that truncate or strip bodies lose the original data.
* **D3 (4):** Same as per-route — users can configure full capture for knowledge-relevant routes.
* **D4 (5):** Same — metadata always captured.
* **D5 (2):** Pipeline configuration is significantly more complex. Users must understand processor ordering, chaining behavior, and per-route pipeline assignment. Config file becomes harder to write and validate.
* **D6 (5):** Full granularity, same as per-route rules.
* **D7 (2):** Most complex implementation. Pipeline orchestration, processor interface, configuration parsing, ordering guarantees. A summarize processor would require an LLM call, adding latency, cost, and failure modes to the capture path. Over-engineered for v1.

#### Always full + retention (116)

* **D1 (3):** Disk grows unchecked until retention kicks in. A 30-day retention policy means 30 days of full-body storage. Users with heavy usage may accumulate significant disk before pruning. Not a 2 because retention policies do eventually bound growth and disk is cheap, but there is no way to proactively reduce storage on a per-request basis.
* **D2 (5):** Every request and response is stored in full, always. Any interaction can be inspected, replayed, or debugged regardless of when it happened (within retention window). Maximum value for the core use case.
* **D3 (5):** Full response bodies are always available for the knowledge layer to embed and extract metadata from. No compromise on knowledge layer capability.
* **D4 (5):** All metadata captured, same as all other options.
* **D5 (5):** Nothing to configure for capture — it is always full. Retention policy is a single config value (`retention_days: 30`). Runtime change is trivial. The simplest mental model.
* **D6 (3):** No per-provider granularity for capture. Retention policy is global. Could add per-provider retention rules in a future iteration.
* **D7 (4):** Capture path is the simplest possible — always store everything. Retention pruning is a background goroutine with a periodic `DELETE FROM requests WHERE timestamp < ?`. One additional component (the pruner) but the capture path itself is trivially simple.

### Consequences

* Good, because every interaction is always available for debugging, replay, and inspection within the retention window — the core value proposition of modeltap is never compromised.
* Good, because the knowledge layer always has access to full response bodies for embedding and metadata extraction.
* Good, because the capture pipeline is the simplest possible implementation — no branching, no mode logic, no configuration parsing in the hot path.
* Good, because the mental model is trivially understandable: modeltap captures everything; old data gets pruned.
* Good, because metrics accuracy is guaranteed — all metadata is always captured.
* Neutral, because retention-based pruning adds a background component, but it is a simple periodic SQL DELETE.
* Bad, because disk usage within the retention window is unbounded per-request — heavy users could see 1–2 GB/month at full capture with a 30-day retention.
* Bad, because there is no per-provider granularity in v1 — users cannot keep Opus calls longer than Ollama calls. This can be addressed in a future iteration with per-provider retention rules.

### Confirmation

The decision will be confirmed by:

1. Implementing the retention pruner as a background goroutine that runs on a configurable interval.
2. Verifying that disk usage stays within expected bounds for typical usage patterns (100 calls/day ≈ 5 MB/day ≈ 150 MB/month).
3. Shipping a `modeltap status` command that reports current database size and retention settings.
4. Confirming that the knowledge layer can successfully embed and extract metadata from all captured interactions.

If disk usage becomes a concern for heavy users before per-provider retention rules are implemented, a global `max_db_size` config option could trigger accelerated pruning as a stopgap.

## More Information

The decision aligns with the weighted scoring matrix. No override was necessary — always-full-with-retention leads by 13 points over the next closest option.

### Disk Usage Estimates

| Usage Level | Calls/day | Avg Response Size | Daily Growth | Monthly (30-day retention) |
|-------------|-----------|-------------------|--------------|---------------------------|
| Light       | 50        | 50 KB             | 2.5 MB       | 75 MB                     |
| Moderate    | 200       | 50 KB             | 10 MB        | 300 MB                    |
| Heavy       | 1000      | 50 KB             | 50 MB        | 1.5 GB                    |

These estimates are well within acceptable ranges for modern systems. Users can reduce retention days for tighter disk control.

### Future Enhancement: Per-Provider Retention

A future iteration can add per-provider retention rules without changing the capture strategy:

```yaml
retention:
  default_days: 30
  rules:
    - provider: anthropic
      model: "claude-opus-*"
      days: 90
    - provider: ollama
      days: 7
```

This preserves the "always capture full" principle while giving users granular control over how long data is kept.
