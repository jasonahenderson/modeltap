# WU-019: Cost Estimation with Pricing Table

**Date:** 2026-03-08
**Role:** Test Engineer, Backend Implementer
**Status:** Complete

## Summary

Implemented a configurable pricing table that estimates the cost of each captured API request based on provider, model, and token counts. The pricing table ships with sensible defaults for current Anthropic and OpenAI models and can be overridden via the YAML config file.

## Changes

### New Files
- `internal/config/pricing.go` — `PricingTable` type with `EstimateCost()`, `SetPricing()`, `NewPricingTable()` (defaults), `NewPricingTableFromConfig()` (defaults + user overrides), and `PricingConfig` type for YAML deserialization.
- `internal/config/pricing_test.go` — 11 test cases covering: known model cost calculation, OpenAI models, unknown model/provider returning $0, nil PricingTable safety, zero tokens, default table coverage for current models, config override, custom model addition, SetPricing, and loading pricing from a YAML config file.

### Modified Files
- `internal/config/config.go` — Added `Pricing PricingConfig` field to `Config` struct.
- `internal/proxy/capture.go` — `CaptureMiddleware` now holds a `*config.PricingTable`; `NewCaptureMiddleware` accepts it as a third parameter; cost estimation runs after metadata extraction, before async save.
- `internal/proxy/server.go` — `ServerConfig` gains a `Pricing *config.PricingTable` field; passed through to `NewCaptureMiddleware`.
- `internal/cli/start.go` — Builds a `PricingTable` from config and passes it into `ServerConfig`.

## Default Pricing (USD per million tokens)

| Provider | Model | Input | Output |
|----------|-------|-------|--------|
| anthropic | claude-opus-4 | $15.00 | $75.00 |
| anthropic | claude-sonnet-4 | $3.00 | $15.00 |
| anthropic | claude-3-5-sonnet-20241022 | $3.00 | $15.00 |
| anthropic | claude-3-5-haiku-20241022 | $0.80 | $4.00 |
| openai | gpt-4o | $2.50 | $10.00 |
| openai | gpt-4o-mini | $0.15 | $0.60 |
| openai | gpt-4 | $30.00 | $60.00 |
| openai | o1 | $15.00 | $60.00 |
| openai | o3-mini | $1.10 | $4.40 |

## Config Override Example

```yaml
pricing:
  anthropic:
    claude-opus-4:
      input_per_mtok: 20.00
      output_per_mtok: 100.00
  custom:
    my-model:
      input_per_mtok: 1.00
      output_per_mtok: 5.00
```

## Test Results

- All 19 pricing/config tests pass.
- All proxy tests pass (one pre-existing flaky test for latency_ms=0 is unrelated).
- `go build ./...` succeeds with no errors.

## Design Decisions

- PricingTable is a simple in-memory map, not an interface — YAGNI for now.
- Unknown models return $0 cost silently (no error), per requirements.
- Defaults are baked into code; user config overlays on top (does not replace).
- Nil PricingTable is safe — returns $0, so capture middleware works even without pricing configured.
