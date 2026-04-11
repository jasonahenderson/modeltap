# PATCH-0002: Local Inference Support

**Status:** proposed
**Date:** 2026-04-09
**Related:** ADR-0006, ADR-0007, ADR-0008, EXP-0001, EXP-0007
**Branch:** patch/0002-local-inference-support

## Problem

Modeltap's v1 built-in provider adapters cover Anthropic and OpenAI (ADR-0006), but a growing share of real-world AI workloads runs against local or self-hosted inference backends. On Apple Silicon in particular, **MLX** has become the native high-performance path, and **Ollama** — both the local daemon and **Ollama Cloud** — is the dominant portable runtime across platforms. Users running these backends today get partial, accidental coverage:

- Requests that hit an OpenAI-compatible `/v1/chat/completions` path are captured and parsed as if they were OpenAI traffic, regardless of whether the actual server is MLX or Ollama.
- Requests to Ollama's native `/api/chat` / `/api/generate` endpoints are not detected at all and fall through as untyped traffic.
- Captures that do land are mislabeled as provider `openai`, which breaks per-provider metrics, pollutes the knowledge layer's cross-model identity, and hides the fact that the request was served by a local model at zero cloud cost.

The goal of this feature is to make MLX, Ollama (local), and Ollama Cloud **first-class, correctly-labeled providers** in modeltap — with accurate detection, faithful request/response parsing, stable model identity for the knowledge layer, and zero-cost accounting for local runs.

## Scope

Seven coordinated changes, all additive within the existing adapter pattern from ADR-0006:

1. **Host-based provider resolution** in the registry, so traffic is labeled by its destination host before adapter-level `Detect()` runs. This single change fixes mislabeling for any OpenAI-compatible local server.
2. **Native Ollama adapter** implementing the `Provider` interface for Ollama's `/api/chat`, `/api/generate`, and `/api/embeddings` endpoints, including newline-delimited JSON stream reassembly.
3. **Ollama OpenAI-compat delegation** so requests hitting Ollama's `/v1/chat/completions` endpoint reuse the existing OpenAI parser while still being labeled `ollama`.
4. **MLX adapter** as a thin labeled wrapper around the OpenAI parser — no duplicated parsing logic, just a correct provider label.
5. **NDJSON stream parsing** in the capture middleware, alongside the existing SSE path, to handle Ollama's native streaming format.
6. **Model identity normalization** so the knowledge layer (ADR-0008) can reason about the same underlying model across MLX, Ollama, and cloud providers.
7. **Zero-cost pricing defaults** for local providers, with Ollama Cloud tracked separately against its own cost model.

The proxy itself requires no architectural changes. The `Provider` interface stays intact; the `Registry` gains a host-map pre-resolution step; the capture middleware gains one branch for NDJSON streams.

## Implementation Detail

### Host-Based Provider Resolution

- The registry gains a `hostMap` of `host:port` → provider name, consulted before adapter-level `Detect()`.
- Host-map matches take precedence over path-based detection, so an OpenAI-compatible local endpoint is labeled by its host (`ollama`, `mlx`) rather than falling through to `openai`.
- Built-in defaults cover the common cases without configuration:
  - `localhost:11434`, `127.0.0.1:11434` → `ollama`
  - `ollama.com`, `*.ollama.com` → `ollama-cloud`
  - `api.openai.com` → `openai`
  - `api.anthropic.com` → `anthropic`
- MLX is not in the default host map because it commonly shares ports with other services; users register it explicitly via `providers.hosts` in config.
- Unknown hosts fall through to the existing adapter-based detection flow, preserving current behavior.

### Ollama Native Adapter

- Implements the full `Provider` interface for Ollama's native endpoints: `/api/chat`, `/api/generate`, `/api/embeddings`.
- Parses the native request shape, extracting `model`, message count, `options.temperature`, and `options.num_predict` into the existing `RequestMetadata` struct.
- Parses the native response shape, extracting `prompt_eval_count` as input tokens and `eval_count` as output tokens.
- Reassembles Ollama's newline-delimited JSON streaming format by concatenating `message.content` fragments across chunks and extracting final token counts from the terminal `"done": true` chunk.
- Also handles Ollama's OpenAI-compatibility endpoint (`/v1/chat/completions` on the Ollama host) by delegating request and response parsing to the OpenAI provider's parsers — one adapter covers both surfaces, and the label stays `ollama` either way.

### Ollama Cloud Support

- Detected by host (`ollama.com`, `*.ollama.com`) and labeled as provider `ollama-cloud`, distinct from local `ollama`, so users can separate local vs. hosted usage in metrics and billing.
- Supports both Ollama Cloud's native API and its OpenAI-compatible surface via the same adapter code as local Ollama.
- Passes through `Authorization: Bearer <key>` headers unchanged; the proxy does not manage Ollama Cloud credentials.

### MLX Adapter

- Thin wrapper around the OpenAI parser, since MLX servers (`mlx_lm.server`, `mlx-omni-server`, and compatible forks) conform to the OpenAI Chat Completions shape.
- Only value-add is the label `mlx` and the correct host-map resolution — all parsing delegates to the existing OpenAI implementation.
- Records the MLX model identifier (e.g. `mlx-community/Llama-3.1-8B-Instruct-4bit`) verbatim in captures, with the normalized name exposed for metrics and knowledge-layer use.

### NDJSON Stream Reassembly

- The capture middleware gains a branch alongside the existing SSE path: when `Content-Type` is `application/x-ndjson` (or the provider signals NDJSON streaming), chunks are split on single newlines instead of double.
- The resulting `[]StreamChunk` is passed to `provider.ReassembleStream()` exactly as SSE chunks are today — the interface does not change.
- Ollama's native streaming is the only format needing this path in v1; future providers can opt in by declaring their stream format.

### Model Identity Normalization

- For each captured request, modeltap records both the raw provider-reported model string and a normalized identifier suitable for cross-provider aggregation.
- Example: `llama-3.1-8b-instruct` is the normalized form of `llama3.1:8b` (Ollama) and `mlx-community/Llama-3.1-8B-Instruct-4bit` (MLX).
- Normalization rules live in a small, user-extensible mapping file (`providers.model_aliases` in config) so new models can be registered without a modeltap release.
- Built-in aliases cover the most common local models: Llama 3.1 family, Qwen 2.5 Coder family, Mistral family, Phi family.
- Unknown models fall through with the raw string used as their own normalized identifier — no data is dropped, and the normalization map is strictly additive.
- A new `normalized_model` column on the `requests` table stores the result; aggregation queries can group by either raw or normalized model.

### Pricing and Cost Accounting

- Local providers (`ollama`, `mlx`) default to zero cost for all models — captured token counts are recorded but contribute $0 to billing views.
- Ollama Cloud is tracked against its own pricing table, separate from local Ollama, so users see distinct rows for local vs. cloud spend.
- Users can override any default by adding entries under `pricing.<provider>.<model>` in config, matching the existing pricing override mechanism.

### Metrics and Dashboard

- Per-provider metrics (ADR-0007) cleanly separate `mlx`, `ollama`, `ollama-cloud`, `openai`, and `anthropic`.
- The web dashboard's provider filter includes the new providers automatically — no hardcoded provider list.
- Daily and hourly aggregation tables gain rows for each new provider as traffic arrives; `modeltap metrics rebuild` can backfill historical captures once host-based relabeling is applied.
- A new `--group-by normalized_model` option on `modeltap metrics` lets users see total usage of a given model across providers.

## CLI / Config Impact

No new top-level subcommands. Existing commands gain awareness of the new providers:

```
modeltap config set providers.hosts.ollama "localhost:11434,my-box.local:11434"
modeltap config set providers.hosts.mlx    "localhost:8080"
modeltap logs --provider ollama
modeltap logs --provider mlx
modeltap metrics --group-by provider
modeltap metrics --group-by normalized_model
modeltap status   # shows per-provider request counts including the new providers
```

## Configuration

New config keys under `providers`:

```yaml
providers:
  hosts:
    ollama:       ["localhost:11434", "127.0.0.1:11434"]
    ollama-cloud: ["ollama.com"]
    mlx:          ["localhost:8080"]
  model_aliases:
    llama-3.1-8b-instruct:
      - "llama3.1:8b"
      - "mlx-community/Llama-3.1-8B-Instruct-4bit"
      - "mlx-community/Meta-Llama-3.1-8B-Instruct-4bit"
      - "meta-llama/Llama-3.1-8B-Instruct"
    qwen-2.5-coder-7b:
      - "qwen2.5-coder:7b"
      - "mlx-community/Qwen2.5-Coder-7B-Instruct-4bit"

pricing:
  ollama-cloud:
    # Populated once Ollama publishes stable pricing
```

All keys are optional; built-in defaults cover the common cases.

## Checklist

- [ ] Add host-based provider resolution for local and hosted inference endpoints
- [ ] Implement Ollama native adapter and OpenAI-compat delegation
- [ ] Implement MLX provider labeling via OpenAI parser reuse
- [ ] Add NDJSON stream capture and reassembly support
- [ ] Add normalized model support for cross-provider metrics and knowledge
- [ ] Add pricing defaults and overrides for local providers and Ollama Cloud
- [ ] Extend metrics and dashboard views for the new provider identities
- [ ] Add or update unit and integration tests for all new provider paths

## Fix Detail

This feature breaks down into small, independently-testable work units following the existing adapter pattern. Rough shape:

| Component | New code | Reuses |
|---|---|---|
| Host-based resolution in `Registry.Detect` | ~30 lines | — |
| Config binding for `providers.hosts` | ~20 lines | Existing Viper setup |
| Ollama native adapter (`internal/provider/ollama.go`) | ~200 lines | — |
| Ollama OpenAI-compat delegation | ~10 lines | OpenAI parser |
| MLX adapter (`internal/provider/mlx.go`) | ~30 lines | OpenAI parser |
| NDJSON stream parsing in `capture.go` | ~40 lines | SSE chunk structure |
| Model normalization (`internal/provider/models.go`) | ~60 lines | — |
| Pricing defaults for local providers | ~5 lines | Existing pricing table |
| `normalized_model` column migration | ~20 lines | Existing schema |

The only genuinely new parsing work is Ollama's native `/api/chat` JSON format and its NDJSON streaming. Everything else is labeling, delegation, and a small amount of schema plumbing.

### Registration Order

Providers must be registered in an order that keeps the OpenAI catch-all last:

```go
registry.Register(provider.NewAnthropicProvider())
registry.Register(provider.NewOllamaProvider())      // before OpenAI: owns /api/chat
registry.Register(provider.NewOllamaCloudProvider())
registry.Register(provider.NewMLXProvider())
registry.Register(provider.NewOpenAIProvider())       // last: catch-all for /v1/chat/completions
registry.SetHostMap(cfg.Providers.Hosts)              // host map runs first during Detect
```

The host map is consulted before the ordered `Detect()` loop, so Ollama and MLX traffic on `/v1/chat/completions` never reaches the OpenAI adapter.

## Out of Scope

- **llama.cpp, LM Studio, vLLM, TGI, and other OpenAI-compatible runtimes** — these will be captured and correctly labeled once users register their hosts under `providers.hosts`, but first-class adapters with custom labels are out of scope for this feature. The host-map pattern proven here with MLX is the extension point for adding them later.
- **Running MLX or Ollama on behalf of the user** — modeltap is a proxy, not a model runner. Users start and manage their own inference servers.
- **Automatic model downloads or model management** — Ollama and MLX have their own tooling for this.
- **Embeddings-specific analytics** — Ollama's `/api/embeddings` and compatible endpoints will be captured, but embedding-specific metrics (dimensions, index stats, similarity distributions) are out of scope and can layer onto the knowledge-layer work.
- **Per-GPU or per-host resource metrics** — modeltap stays at the request layer; hardware telemetry is a separate concern.

## Relationship to ADRs

- Extends ADR-0006 (Multi-Provider Support) by adding Ollama, MLX, and Ollama Cloud as built-in adapters and introducing host-based provider resolution for OpenAI-compatible endpoints.
- Feeds ADR-0007 (Metrics) with new provider dimensions and the `normalized_model` grouping axis.
- Feeds ADR-0008 (Knowledge Layer) with the model identity normalization needed for cross-model reasoning.
- Uses ADR-0004 (Viper) for the new `providers.hosts` and `providers.model_aliases` config keys.
- Compatible with ADR-0002 (Storage) — the `normalized_model` column is an additive `ALTER TABLE ADD COLUMN` migration.

## Open Questions

- Should host-based resolution live as a pre-adapter step in the `Registry`, or as a shared helper invoked from each adapter's `Detect()` method? The former keeps adapters clean and makes the host map the single source of truth; the latter keeps all provider logic inside its adapter. Current design proposes the former.
- Should Ollama Cloud be a distinct adapter from local Ollama, or the same adapter with the label chosen by host? Same adapter is simpler but couples their evolution; separate adapters are cleaner if Ollama Cloud diverges from the local API. Current design proposes same adapter with host-based labeling.
- Do we need per-model cost tracking for Ollama Cloud in v1, or can that wait until Ollama publishes stable pricing? Current design leaves the pricing table empty and defers until pricing is published.
- Should the normalization map ship as a standalone YAML file that users can drop into their config directory, or only as inline `providers.model_aliases` entries? Standalone file is friendlier for sharing; inline is simpler. Current design proposes inline with a future option for a standalone file.
