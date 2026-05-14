---
patch: "PATCH-0008"
title: "Moonshot Provider Adapter"
status: "proposed"
date: "2026-04-20"
related:
  - "PATCH-0004 (secret prefix resolver)"
  - "PATCH-0007 (dotenv loader)"
  - "FEAT-0009 (terminal harness)"
branch: "patch/0008-moonshot-provider-adapter"
---

# PATCH-0008: Moonshot Provider Adapter

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`docs/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

modeltap cannot route to Moonshot AI (Kimi K2.6 / K2.5) because no provider adapter exists in `internal/provider/`. The provider registry knows Anthropic, OpenAI, and Ollama only. Users who set `type: moonshot` in `config.yaml` get a startup error from the registry. Kimi's API is OpenAI-compatible, so the adapter is small — mostly defaults and response-field handling — but without it the harness cannot leverage Kimi's 256k context window or K2.6 reasoning capabilities.

## Scope

### Provider adapter: `internal/provider/moonshot.go`
- Implements the `Provider` interface (stateless, zero-arg constructor, same pattern as `anthropic.go` and `openai.go`).
- `Name()` returns `"moonshot"`.
- `Detect()` matches requests to `Host` containing `"api.moonshot.cn"`.
- `FormatMessages()` builds OpenAI-compatible `/v1/chat/completions` request bodies with Moonshot-specific defaults:
  - `max_tokens` default: `16384` (when `opts.MaxTokens == 0`).
  - **Mode-aware defaults** (see Fix Detail):
    - `Plan` mode: `temperature: 1.0`, `extra_body: {"thinking": {"type": "enabled"}}`
    - `Build` mode: `temperature: 0.6`, `extra_body: {"thinking": {"type": "disabled"}}`
    - `Auto` / default: `temperature: 1.0`, thinking enabled
  - User override (`*Temperature != nil`) always wins and disables mode-aware logic entirely.
- `ParseStreamEvent()` decodes OpenAI-format SSE deltas. `reasoning_content` fields are silently dropped (not emitted as visible tokens). Handling reasoning display is out of scope.
- `ParseRequest()`, `ParseResponse()`, `ReassembleStream()` use the same OpenAI wire-format helpers as the existing OpenAI provider.

### Provider registration: `internal/provider/`
- **`internal/provider/registry.go`** — No central switch exists; register by adding `provider.Adapters().Register(provider.NewMoonshotProvider())` to `internal/cli/bff_wiring.go` alongside Anthropic/OpenAI/Ollama.
- **`internal/cli/bff_wiring.go`** — Register `NewMoonshotProvider()` in `startBFFServer`.

### BFF dispatch wiring: `internal/bff/`
- **`internal/bff/dispatch.go`** — Add `Mode protocol.Mode` to `DispatchOpts`. Wire it through `TurnDispatcher.Dispatch` into `FormatMessagesOpts.Mode`. Regression tests inspect the formatted Moonshot request body for Plan/Build/Auto temperature values.
- **`internal/bff/turn.go`** — Set `dispatchOpts.Mode = submit.Mode` in `handleTurnSubmit`.
- **`internal/bff/providers.go`** (if endpoint path constants exist) — Add `providerEndpointPath("moonshot")` resolving to `"/v1/chat/completions"`. If no such helper exists, document that Moonshot uses the same POST path as OpenAI (`/v1/chat/completions`) and the dispatcher resolves it via the endpoint's `Host`.

### Config: `docs/sample-config.yaml`
- Add a commented Moonshot provider block. Use only fields supported by `config.ProviderConfig`: `type`, `api_key`, `host`.
- `kimi-k2-6` model routing is handled by the BFF model registry (`bff.models` or automatic resolution), not by the provider adapter. Document that `model` selection occurs at the routing layer.

### Tests: `internal/provider/moonshot_test.go`
Table-driven tests covering:
- `FormatMessages` — verify correct JSON body shape
- Mode-aware temperature and thinking control (Plan, Build, Auto)
- User temperature override disables mode logic
- Default `max_tokens` injection when `opts.MaxTokens == 0`
- Stream parsing — `reasoning_content` dropped silently
- Error propagation on 4xx/5xx

### Guide: `docs/guides/tuning-kimi-k2.6.md`
- Comprehensive tuning guide (see Fix Detail).

### Build & vet — `go build ./...` and `go vet ./...` clean.

## Out of Scope

- Harness slash commands (`/kimi`, `/reasoning`) — UI sugar, belongs to a future feature if desired.
- Provider-aware attachment strategies (send full files vs. chunk) — architectural decision that affects all providers, needs an ADR or feature spec.
- Reasoning/thinking display in harness viewport — `reasoning_content` is silently dropped for now; surfacing it requires protocol changes (`EventReasoningDelta`, `StreamCompleteMsg.Reasoning`, BFF relay, persistence) that are out of scope.
- Dynamic token budget negotiation per provider — cross-provider change, too broad for this patch.
- MCP server wrapping Kimi's built-in web search — exploratory, not implementation-scoped.
- Protocol changes (`EventReasoningDelta`, `StreamCompleteMsg.Reasoning`) — deferred; reasoning content is silently dropped.

## Checklist

- [ ] `internal/provider/moonshot.go` compiles; `var _ provider.Provider = (*MoonshotProvider)(nil)`
- [ ] `internal/provider/moonshot.go` uses stateless zero-arg constructor: `NewMoonshotProvider()`
- [ ] `internal/cli/bff_wiring.go` registers `NewMoonshotProvider()` alongside Anthropic/OpenAI/Ollama
- [ ] `internal/bff/dispatch.go` adds `Mode protocol.Mode` to `DispatchOpts`
- [ ] `internal/bff/turn.go` sets `dispatchOpts.Mode = submit.Mode`
- [ ] `internal/bff/dispatch.go` passes `Mode` into `FormatMessagesOpts.Mode`
- [ ] `docs/sample-config.yaml` contains Moonshot provider block with only supported config fields
- [ ] `internal/provider/moonshot_test.go` passes (`go test ./internal/provider/ -run Moonshot`)
- [ ] `docs/guides/tuning-kimi-k2.6.md` created and linked from sample config header comments
- [ ] `go vet ./...` clean
- [ ] User can run:
  ```
  echo 'MOONSHOT_API_KEY=sk-...' >> ~/.modeltap/.env
  modeltap harness --model kimi-k2-6
  ```
  and see a successful `TurnSubmitAck`.

## Fix Detail

### Stateless constructor

All in-tree providers are stateless zero-arg constructors:

```go
// internal/provider/openai.go
func NewOpenAIProvider() *OpenAIProvider { return &OpenAIProvider{} }

// internal/provider/anthropic.go
func NewAnthropicProvider() *AnthropicProvider { return &AnthropicProvider{} }
```

Moonshot follows the exact same pattern:

```go
// internal/provider/moonshot.go
func NewMoonshotProvider() *MoonshotProvider { return &MoonshotProvider{} }
```

Runtime configuration (host, API key, model) lives in the BFF's `ProviderEndpoint` and `ModelEntry`, not in the adapter. The adapter's job is wire-format translation only.

### BFF registration

In `internal/cli/bff_wiring.go`, add alongside the existing three:

```go
srv.Adapters().Register(provider.NewMoonshotProvider())
```

Config entries become `ProviderEndpoint` records:

```go
for name, pc := range cfg.Providers {
    if pc.Type == "" { continue }
    host := resolveProviderHost(pc, cfg.Port)
    ep := &bff.ProviderEndpoint{
        Name:     name,
        Type:     pc.Type,
        APIKey:   pc.APIKey,
        Host:     host,
        Discover: pc.Discover,
    }
    srv.Providers().Add(ep)
}
```

`type: moonshot` resolves to adapter `Name() == "moonshot"` and endpoint `Host` is the Moonshot base URL.

### Mode-aware defaults

`DispatchOpts` gains a `Mode` field, set from `submit.Mode` in `handleTurnSubmit` and forwarded into `FormatMessagesOpts.Mode` by `TurnDispatcher.Dispatch`.

```go
// In internal/bff/dispatch.go
type DispatchOpts struct {
    // ... existing fields ...
    Mode protocol.Mode // NEW
}
```

```go
// In internal/bff/turn.go (handleTurnSubmit)
dispatchOpts := DispatchOpts{
    // ... existing fields ...
    Mode: submit.Mode, // NEW
}
```

```go
// In internal/bff/dispatch.go (TurnDispatcher.Dispatch)
fmOpts := provider.FormatMessagesOpts{
    // ... existing fields ...
    Mode: opts.Mode, // NEW
}
```

In `moonshot.go`:

```go
func temperatureForMode(mode protocol.Mode, override *float64) (float64, bool) {
    if override != nil {
        return *override, false // user override: no thinking control
    }
    switch mode {
    case protocol.ModeBuild:
        return 0.6, false // Instant: low temp, thinking disabled
    case protocol.ModePlan:
        return 1.0, true // Thinking: high temp, thinking enabled
    default:
        return 1.0, true // Auto: conservative default
    }
}
```

### Thinking control in request body

Moonshot's API uses `extra_body` (or `extra_body` in the SDK) to toggle Thinking vs Instant:

| Mode | Request body field |
|---|---|
| Thinking (Plan/Auto) | `extra_body: {"thinking": {"type": "enabled"}}` |
| Instant (Build) | `extra_body: {"thinking": {"type": "disabled"}}` |

In `FormatMessages`, when the caller didn't override temperature (mode-aware path active), inject the thinking control:

```go
body := openAIRequest{ /* ... standard fields ... */ }
temp, thinkingEnabled := temperatureForMode(opts.Mode, opts.Temperature)
body.Temperature = temp
if thinkingEnabled || !thinkingEnabled { // always inject when mode-aware
    body.ExtraBody = map[string]any{
        "thinking": map[string]string{"type": "enabled"},
    }
    if !thinkingEnabled {
        body.ExtraBody["thinking"].(map[string]string)["type"] = "disabled"
    }
}
```

When `opts.Temperature != nil` (user override), do NOT inject `extra_body.thinking` at all — respect whatever the user's endpoint config specifies.

### Reasoning stream handling

Moonshot's SSE deltas may contain `reasoning_content` alongside `content`:

```json
{
  "choices": [{
    "delta": {
      "content": "the answer is",
      "reasoning_content": "First, I need to..."
    }
  }]
}
```

**For this patch:** `reasoning_content` is silently dropped. `ParseStreamEvent` only emits `StreamEventText` for `delta.content`. The reasoning buffer (`ReasoningContent string`) exists on the delta struct but is never emitted through the provider event system.

Rationale: protocol `StreamEvent` has no reasoning type, BFF `StreamRelay` only accumulates text/tool/usage, harness `StreamCompleteMsg` has no reasoning field, and persistence doesn't store it. Adding the full pipeline is cross-layer work that belongs to a feature spec, not this implementation-scoped patch.

### Config example

```yaml
providers:
  moonshot:
    type: moonshot
    # Host is the Moonshot API base URL. Use https://api.moonshot.cn/v1
    # for the official endpoint, or a local vLLM/SGLang endpoint.
    host: https://api.moonshot.cn/v1
    api_key: env:MOONSHOT_API_KEY

# Model routing is handled separately. Add kimi-k2-6 to the BFF model
# registry or rely on automatic resolution if your routing policy
# includes it.
```

Fields after `api_key` are not supported by `config.ProviderConfig` in the current schema. Temperature, max_tokens, and model selection happen at the dispatch layer (routing policy + `FormatMessages` defaults), not provider config.

### OpenAI-format helper

The adapter reuses OpenAI stream structs via an unexported `openaiformat.go` helper (already shared by the OpenAI provider). The delta struct gains `ReasoningContent string` for parsing but does not emit it:

```go
type openAIDelta struct {
    Content          string `json:"content"`
    ReasoningContent string `json:"reasoning_content,omitempty"` // parsed but dropped
}
```

If `openaiformat.go` does not yet exist, create it by extracting the shared structs from `openai.go`.
