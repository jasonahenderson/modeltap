# PATCH-0008 Findings: Moonshot Provider Adapter

```text
total_findings: 6
blocking: 4
significant: 2
advisory: 0
top_line: PATCH-0008 is not implementation-ready as written because it omits required BFF/provider endpoint integration, references unsupported config fields, and leaves reasoning/Thinking-mode behavior underspecified.
```

## F1: BFF Endpoint Integration Is Missing

**Reviewer:** Implementation Readiness  
**Severity:** blocking  
**Affected sections:** Scope, Checklist, Registry integration

The patch targets `internal/provider/registry.go` for `"moonshot"` registration, but the current adapter registry has no `NewProvider` switch. Harness routing also depends on the BFF endpoint registry and dispatch path: `bff.ProviderTypeMoonshot`, provider type validation, default host selection, health checking, endpoint path selection, auth headers, and CLI adapter registration.

As written, `type: moonshot` will still be rejected by the BFF provider registry or dispatched with an empty endpoint path even if `internal/provider/moonshot.go` exists.

**Recommendation:** Expand scope to include `internal/bff/providers.go`, `internal/bff/dispatch.go`, `internal/cli/bff_wiring.go`, relevant tests, and proxy/start registration if Moonshot capture is expected outside the harness path.

**Disposition:** null

## F2: Config Example Uses Unsupported Fields

**Reviewer:** Implementation Readiness  
**Severity:** blocking  
**Affected sections:** Scope, Config example, Checklist

The patch examples use provider-level `base_url`, `model`, and `max_tokens`, but current `config.ProviderConfig` supports only `upstream`, `type`, `api_key`, `host`, and `discover`. Those fields will be ignored by the loader. In addition, `kimi-k2-6` will not resolve through routing unless the model registry gains built-in Moonshot models or the sample config includes a `bff.models` manual override.

**Recommendation:** Either revise examples to use existing fields (`host`, `bff.models`, `bff.routing`) or explicitly include config schema changes and model registry changes in the patch scope.

**Disposition:** null

## F3: Mode Is Not Passed To Providers

**Reviewer:** Implementation Readiness  
**Severity:** blocking  
**Affected sections:** Scope, Mode-aware temperature implementation

The patch says the BFF turn dispatcher already passes `turn.Mode` into `FormatMessagesOpts.Mode`, but current `DispatchOpts` has no `Mode`, `handleTurnSubmit` does not set one, and `Dispatch` cannot forward it into `FormatMessagesOpts`.

The proposed mode-aware temperature defaults will not activate unless the BFF dispatch contract is changed end to end.

**Recommendation:** Add `Mode protocol.Mode` to `bff.DispatchOpts`, set it from `submit.Mode` in `handleTurnSubmit`, pass it through in `TurnDispatcher.Dispatch`, and add regression tests that inspect the formatted Moonshot request for Plan, Build, and Auto.

**Disposition:** null

## F4: Build Mode Does Not Disable Thinking

**Reviewer:** Implementation Readiness  
**Severity:** blocking  
**Affected sections:** Why defaults differ from OpenAI, Mode-aware temperature implementation

The patch maps Build mode to Moonshot Instant mode / thinking disabled, but the implementation detail only changes temperature. If Moonshot defaults thinking to enabled, Build mode will still use Thinking mode unless the request body also emits Moonshot's thinking control field.

**Recommendation:** Specify the exact request-body shape for Thinking enabled/disabled, include it in the Moonshot request struct, and test that Plan/Auto enable thinking while Build disables it unless the user explicitly overrides provider behavior.

**Disposition:** null

## F5: Reasoning Stream Handling Is Underspecified

**Reviewer:** Implementation Readiness  
**Severity:** significant  
**Affected sections:** Response format: reasoning_content

The patch says to parse and buffer `reasoning_content` and emit it in `StreamCompleteMsg.Reasoning` or metadata. Current `provider.StreamEvent`, `protocol.TurnComplete`, BFF stream relay accumulation, and harness `StreamCompleteMsg` have no reasoning field or event type. The relay only accumulates text, tool calls, and usage.

This creates ambiguity: either reasoning is silently discarded, stored only in proxy capture metadata, emitted through protocol, or persisted with the assistant turn.

**Recommendation:** Pick one behavior in the patch doc. If reasoning is retained, include the protocol, BFF relay, conformance fixture, harness message, and persistence changes. If it is discarded for now, state that explicitly and remove references to `StreamCompleteMsg.Reasoning`.

**Disposition:** null

## F6: Constructor Shape Does Not Match Current Providers

**Reviewer:** Implementation Readiness  
**Severity:** significant  
**Affected sections:** Registry integration

The proposed `NewMoonshotProvider(cfg ProviderConfig)` does not match the current in-tree adapter pattern. Anthropic, OpenAI, and Ollama constructors are stateless and receive runtime endpoint data through the BFF endpoint registry and dispatch path, not provider adapter construction.

**Recommendation:** Use a stateless `NewMoonshotProvider()` unless the patch also introduces and justifies a new provider-adapter configuration surface. Keep endpoint host/API key/model defaults in the existing BFF/config/model-registry layers.

**Disposition:** null
