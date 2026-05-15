---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0006: Multi-Provider Support Strategy

## Context and Problem Statement

Modeltap's value increases with the number of model providers it can proxy. Users interact with multiple providers — Anthropic, OpenAI, Google, local models via Ollama, and others. The proxy must handle their different API formats, authentication schemes, and streaming implementations. The decision is how to architect multi-provider support: whether to build a provider abstraction layer from day one, start with one provider and grow, or treat all HTTP traffic generically.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Breadth of provider coverage (5):** The knowledge layer and cross-model brain features only become valuable when modeltap captures traffic from all providers a user interacts with. Supporting only one provider undermines the core vision.
* **D2 – Accuracy of metadata extraction (5):** Each provider returns token counts, model names, and usage data in different response formats. Modeltap must parse these correctly for metrics and usage tracking to work.
* **D3 – Ease of adding new providers (4):** The AI model ecosystem changes rapidly. New providers and API formats emerge frequently. Adding support for a new provider should be a contained, well-defined task.
* **D4 – Time to first working provider (4):** Getting Anthropic (Claude Code's provider) working quickly validates the core proxy architecture. The first provider should not be blocked by abstraction layer design.
* **D5 – Streaming format handling (4):** Providers use different streaming approaches — Anthropic uses SSE with `message_start`/`content_block_delta` events, OpenAI uses SSE with `data: [DONE]` terminators, Ollama uses newline-delimited JSON. The architecture must handle all of these.
* **D6 – Local/private model support (3):** Users running Ollama, vLLM, or other local inference servers need the same capture and logging as cloud providers. The architecture should not assume HTTPS or cloud-hosted endpoints.
* **D7 – Minimal code duplication (3):** Common proxy logic (SSE capture, request logging, metrics extraction) should be shared across providers. Provider-specific code should only handle the differences.

## Considered Options

* Generic HTTP proxy with provider-specific parsers
* Provider adapter interface from day one
* Single provider (Anthropic) first, refactor later
* LiteLLM-style unified API translation

## Decision Outcome

Chosen option: **Provider adapter interface from day one**, because it achieves the highest weighted score (118) and establishes the architectural pattern that every subsequent ADR depends on. The adapter interface is a small Go interface — not a heavyweight framework — that cleanly separates "how to proxy HTTP" from "how to parse this provider's response format." Starting with the interface means the first provider (Anthropic) serves as the reference implementation, and every subsequent provider follows the same pattern.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                              | Weight | Generic HTTP | Provider adapter | Single first | LiteLLM-style |
|-------------------------------------|--------|-------------|-----------------|-------------|---------------|
| D1: Provider coverage breadth       | 5      | 4           | 5               | 2           | 5             |
| D2: Metadata extraction accuracy    | 5      | 2           | 5               | 5           | 4             |
| D3: Ease of adding providers        | 4      | 3           | 5               | 2           | 3             |
| D4: Time to first provider          | 4      | 5           | 4               | 5           | 2             |
| D5: Streaming format handling       | 4      | 2           | 5               | 4           | 4             |
| D6: Local/private model support     | 3      | 4           | 4               | 2           | 3             |
| D7: Minimal code duplication        | 3      | 2           | 4               | 3           | 4             |
| **Weighted Total**                  |        | **93**      | **130**         | **93**      | **103**       |

### Scoring Justification

#### Generic HTTP proxy (93)

* **D1 (4):** Works with any HTTP endpoint by default — no provider-specific code needed to proxy traffic. But cannot intelligently parse responses it does not understand.
* **D2 (2):** Without provider-specific parsing, token counts, model names, and usage metadata must be extracted via generic heuristics. Different providers put this data in different places — Anthropic uses `usage` in the response body, OpenAI uses a similar but differently structured field. Generic extraction is unreliable.
* **D3 (3):** No provider code to write, but adding accurate metadata extraction for a new provider means adding special cases to the generic parser — which gradually becomes a provider adapter without the clean interface.
* **D4 (5):** Fastest to ship — the proxy works immediately for any endpoint. No provider code needed.
* **D5 (2):** Generic SSE handling misses provider-specific stream formats. Anthropic's event types differ from OpenAI's. Reassembling a complete response from the stream requires understanding the provider's event structure.
* **D6 (4):** Works with any HTTP endpoint including local servers. But cannot extract Ollama-specific metadata without special casing.
* **D7 (2):** Starts simple but accumulates special cases. Without a clean interface, provider-specific logic spreads through the codebase.

#### Provider adapter interface (130)

* **D1 (5):** Designed for multi-provider from the start. Each provider is a self-contained adapter that implements a common interface. Adding a provider does not affect existing ones.
* **D2 (5):** Each adapter knows exactly how to parse its provider's response format. Anthropic adapter parses `usage.input_tokens`, OpenAI adapter parses `usage.prompt_tokens`. Maximum accuracy.
* **D3 (5):** Adding a new provider means implementing a Go interface — `DetectProvider()`, `ParseResponse()`, `ExtractUsage()`, `ReassembleStream()`. Well-defined contract, contained scope, easy to test.
* **D4 (4):** Slightly more upfront work than the generic approach — must define the interface and implement the first adapter. But the interface is small (5-6 methods) and the Anthropic adapter validates the design.
* **D5 (5):** Each adapter handles its provider's streaming format. Anthropic adapter understands `message_start`/`content_block_delta`. OpenAI adapter understands `data: [DONE]`. No generic guessing.
* **D6 (4):** Local providers like Ollama get their own adapter. The interface does not assume cloud endpoints — adapters can handle any transport.
* **D7 (4):** Common logic (HTTP proxying, logging, storage) lives in the core. Provider-specific logic is confined to adapters. Clean separation.

#### Single provider first (93)

* **D1 (2):** Only Anthropic works initially. Users of OpenAI, Google, or Ollama cannot use modeltap until their provider is added. Limits adoption and undermines the knowledge layer vision.
* **D2 (5):** Anthropic parsing can be deeply optimized without abstraction overhead. Maximum accuracy for the one supported provider.
* **D3 (2):** No interface means refactoring when the second provider is added. The Anthropic-specific code must be extracted into a pattern that generalizes. This refactor touches the core proxy path.
* **D4 (5):** Fastest path to a working Anthropic proxy. No abstraction overhead.
* **D5 (4):** Anthropic's SSE format is handled directly. Works well for one provider but the approach does not generalize without refactoring.
* **D6 (2):** Local models are not supported until the architecture is refactored for multi-provider.
* **D7 (3):** No duplication initially (one provider), but the refactor to add a second provider will require significant restructuring.

#### LiteLLM-style unified API translation (103)

* **D1 (5):** Translates all provider APIs into a single unified format. Maximum coverage.
* **D2 (4):** Translation layer normalizes metadata, but translation is lossy — provider-specific fields that do not map to the unified format are lost.
* **D3 (3):** Adding a provider means writing a bidirectional translation layer (client format → unified, unified → provider). More work per provider than a simple parser.
* **D4 (2):** Designing a unified API format is significant upfront work. Must account for differences across all providers before shipping the first one.
* **D5 (4):** Streaming can be normalized, but each provider's stream format must still be individually handled in the translation layer.
* **D6 (3):** Works but adds unnecessary translation overhead for local models that could be proxied directly.
* **D7 (4):** Good code sharing through the unified format. But the translation layers themselves can be complex.

### Consequences

* Good, because the adapter interface establishes a clean, testable pattern for multi-provider support from the first line of code.
* Good, because each provider adapter is self-contained — adding OpenAI support does not risk breaking Anthropic support.
* Good, because provider-specific metadata extraction is maximally accurate, which is critical for metrics and the knowledge layer.
* Good, because the interface is small enough (5-6 methods) that it does not add significant abstraction overhead.
* Neutral, because the first provider (Anthropic) takes slightly longer to ship than a hard-coded approach, but the difference is marginal.
* Bad, because maintaining adapters for many providers is an ongoing community effort — each API change requires an adapter update.
* Bad, because the adapter interface must be designed well upfront. A poor interface will require breaking changes when edge cases emerge in later providers.

### Confirmation

The decision will be confirmed by:

1. Defining the `Provider` interface and implementing the Anthropic adapter as the reference implementation.
2. Implementing a second adapter (OpenAI) and confirming that it slots in without changes to the core proxy or the Anthropic adapter.
3. Verifying that metadata extraction accuracy matches direct API parsing for both providers.
4. Confirming that a community contributor could implement a third adapter (e.g., Ollama) using only the interface documentation and existing adapters as examples.

## More Information

The decision aligns with the weighted scoring matrix. No override was necessary — provider adapter interface leads by 27 points.

The provider interface will follow this approximate shape:

```go
type Provider interface {
    // Name returns the provider identifier (e.g., "anthropic", "openai", "ollama")
    Name() string

    // Detect returns true if this provider handles the given request
    Detect(r *http.Request) bool

    // ParseRequest extracts metadata from the outgoing request
    ParseRequest(r *http.Request, body []byte) (*RequestMetadata, error)

    // ParseResponse extracts metadata from the complete response
    ParseResponse(statusCode int, headers http.Header, body []byte) (*ResponseMetadata, error)

    // ReassembleStream reconstructs the full response from SSE chunks
    ReassembleStream(chunks []StreamChunk) ([]byte, error)
}
```

Built-in adapters for v1: Anthropic, OpenAI. Community-contributed adapters: Google, Ollama, Mistral, etc.
