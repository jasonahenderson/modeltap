# Pre-Review: Model Config & Routing Bundle (WU-057 + WU-058 + WU-059 + WU-060)

**Reviewer:** Claude subagent (pre-review)
**Date:** 2026-04-16
**Design doc:** `docs/history/2026-04-16-design-model-config-routing-057-058-059-060.md`

## Summary

The design covers the four-stage model resolution pipeline: provider endpoints (057), model registry (058), hierarchical routing policy (059), and multi-model branching (060). Overall the design is solid and aligns well with FEAT-0008. Findings below.

---

## Blocking

### B-01: Provider config YAML structure diverges from FEAT-0008

**Design (D2.2):** providers is a YAML list of objects with a `name` field:
```yaml
providers:
  - name: anthropic-prod
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
```

**FEAT-0008 (lines 1117-1132):** providers is a YAML map keyed by endpoint name:
```yaml
providers:
  anthropic:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
```

The design introduces `ProviderEndpointConfig` with `mapstructure:"providers"` as a slice, while FEAT-0008 defines it as a map. This is a user-facing config format change. The `ProviderConfig.Endpoints` field is typed `[]ProviderEndpointConfig` but should be `map[string]ProviderEndpointConfig` with the key serving as the endpoint name.

**Impact:** Config files written to the FEAT-0008 spec will fail to parse. Either the design must match the spec, or a FEAT-0008 amendment must be filed and accepted.

### B-02: Model registry manual config format diverges from FEAT-0008

**Design (D3.4):** manual models config is a YAML list:
```yaml
models:
  - name: llama-3.1-8b
    provider: ollama-local
    context_window: 8192
    capabilities: [tool_use]
    roles: [cheap, coding]
```

**FEAT-0008 (lines 1134-1138):** models is a YAML map keyed by model name:
```yaml
models:
  llama-3.1-70b:
    provider: ollama-gpu
    description: "Strong local model, security review"
```

Same issue as B-01. The wire format must agree with the spec or an amendment must be filed.

### B-03: Routing resolution algorithm step 3 does not match FEAT-0008

**Design (D4.1):** Resolution algorithm step 3 says "Drop the last segment (e.g., `coding.default` -> `coding`) -> if found, return" and then "Recurse until reaching top-level `default`."

**FEAT-0008 (line 798):** Resolution order is explicitly: `category.role` -> `category.default` -> `default`

The FEAT-0008 resolution is three-step, not recursive. There is no intermediate step of resolving the bare `coding` key as a node (which in FEAT-0008's YAML examples would be a map, not a string). The design's step 3 ("drop the last segment") would try to match `coding` as a leaf value, but in the FEAT-0008 config `coding:` is a map containing sub-keys. This step would either silently fail (no harm) or incorrectly match if a category happens to also have a direct string assignment at the same level as nested keys (which YAML does not allow anyway).

More critically, the design implies deeper-than-2-level nesting works (with recursive fallback), but FEAT-0008 only specifies two-level hierarchy (`category.role`). The design should either explicitly limit to two levels matching the spec, or document this as an intentional extension and flag it for FEAT-0008 amendment.

**Impact:** The resolution algorithm's recursive behavior is underspecified for deeper nesting. For two-level paths it likely produces correct results, but the extra complexity is unjustified against the spec.

---

## Attention

### A-01: `RoutingPolicy.ResolveForTurn` uses `mode` as the routing path, but FEAT-0008 uses domain roles

**Design (D4.2):** `ResolveForTurn` converts `protocol.Mode` (plan/build/auto) to a routing path string and looks it up.

**FEAT-0008 (lines 757-806):** Routing paths are domain-based (`backend.review`, `infrastructure.code`, `planning.default`), not mode-based. The spec says "the BFF classifies the current turn's intent based on the conversation context, system prompt, and any explicit user request" (line 804).

The design conflates protocol `Mode` with routing policy paths. Mode values are `plan`, `build`, `auto` -- these are not the same as routing categories like `backend`, `frontend`, `coding`. The routing lookup should use intent/role classification, with mode being one signal into that classification, not the direct lookup key.

This may work for a simplified v1 (where mode names happen to be routing keys), but it misrepresents the FEAT-0008 design intent and will need rework when real intent classification lands. At minimum, the design should acknowledge this simplification and document a TODO.

### A-02: `ModelInfo` field names differ between design and protocol-types

**Design (D3.2):** Built-in catalog uses `CostPer1kInput` and `CostPer1kOutput` as `ModelInfo` fields.

**Protocol-types (models.go, WU-041):** `ModelInfo` has fields `cost_per_1k_input` (float) and `cost_per_1k_output` (float), matching the design.

However, **FEAT-0008 (lines 733-734)** uses `cost: { input: $/1K, output: $/1K }` as a nested object for manual model config, and the model list response (line 862-863) uses `cost_per_1k_input` / `cost_per_1k_output` as flat fields. The design correctly follows the protocol-types flat format for `ModelInfo`, but the manual config parsing (D3.4) does not show how the nested `cost` config format from FEAT-0008 maps to the flat `ModelInfo` fields. This needs a conversion step.

### A-03: `MultiModelOpts` duplicates fields from `DispatchOpts`

**Design (D5.2):** `MultiModelOpts` has `Conversation`, `SystemPrompt`, `Tools`, `MaxTokens`, `Temperature`, `WindowSize` -- all of which also exist in `DispatchOpts` from WU-052.

This is not a bug, but the duplication suggests `MultiModelOpts` should embed or reference `DispatchOpts` plus the `Models` list, rather than duplicating fields. This reduces the risk of the two structs drifting apart.

### A-04: `BranchComplete` event in design is missing `cost` field

**Design (D5.3, event 3):** Branch complete JSON example shows `tokens_in`, `tokens_out`, `cost`.

**Protocol-types (events.go, WU-040):** `BranchComplete` has `final_input_tokens`, `final_output_tokens`, `model`, `provider` -- but no `cost` field.

The design's branch complete event example includes a `cost` field, but the protocol-types `BranchComplete` struct does not define one. Either the protocol-types need a `cost` field added to `BranchComplete`, or the design should remove it from the example and note that cost is reported via separate `cost.update` events per branch.

### A-05: `BranchResult.Cost` computed but never emitted as event field

Related to A-04: the design's `BranchResult` struct (D5.2) has a `Cost float64` field, and `AggregateResult` sums `TotalCost`. This cost data feeds `turn.complete`'s `total_cost`, but there is no protocol event to emit per-branch cost at completion. The `cost.update` event (protocol-types) serves as the running cost notification, but the final per-branch cost is lost unless `branch.complete` carries it or a final `cost.update` is emitted per branch.

### A-06: `ActiveSession.ModelOverride` field referenced but not declared

**Design (D4.2, D4.4):** `ResolveForTurn` reads `session.ModelOverride` and `model.switch` sets `session.ModelOverride`.

**Sessions design (WU-050):** `ActiveSession` struct in D2.1 does not include a `ModelOverride` field. It has `ID`, `Conversation`, `ConnID`, `LockExpiry`, `GraceCancel`.

The `ModelOverride` field needs to be added to `ActiveSession` in the sessions design, or this design needs to declare it as a new field it introduces.

### A-07: `BranchError` event has `diagnostic_code` but design emits plain error string

**Protocol-types (WU-040):** `BranchError` requires `diagnostic_code DiagnosticCode` (required field).

**Design (D5.4, step 6):** "On error: emit `branch.error`" -- but the `executeBranch` function description does not specify which diagnostic code to use. Provider errors should map to `MT-CONN-009` (provider_unavailable), cancellation should use a code (none currently defined for cancellation), and other errors need mapping. The design should specify the diagnostic code selection logic.

---

## Nit

### N-01: `ProviderStatus` type name collision with protocol-types

The design defines `ProviderStatus` as a `string` type in `providers.go` with values `"ready"`, `"unavailable"`, `"error"`. The protocol-types design (health.go, WU-041) also defines `ProviderStatus` as a struct with fields `status`, `error`, `models`. These are different types in different packages (`internal/bff/` vs `internal/protocol/`), so there is no Go compilation error, but the name collision may cause confusion in code reviews and documentation. Consider renaming the BFF-internal one to `EndpointStatus`.

### N-02: Built-in model catalog uses speculative model names

The design (D3.2) references `claude-opus-4-6`, `claude-sonnet-4-6`, `claude-haiku-4-5`, `gpt-5`, `o4-mini`. These are fine as examples but should be clearly documented as placeholder values that will be updated during implementation. The built-in catalog should be a data file or easily-updated Go constant, not buried in business logic.

### N-03: `ByRole` method on `ModelRegistry` implies roles are stored on `ModelEntry`

**Design (D3.6):** `ByRole(role string) []*ModelEntry` exists, and D3.4 shows `roles: [cheap, coding]` in manual config.

**FEAT-0008:** roles are assigned in the routing policy, not on the model itself. A model's roles are derived by inverting the routing tree (find all paths that resolve to this model). The `roles` field in `model.list` response (line 858) is computed, not stored. The design should clarify that `ByRole` queries the routing policy, not a `Roles` field on `ModelEntry`.

### N-04: Health check treats 401 as success for cloud providers

**Design (D2.3):** "Success = any 2xx or 401 (auth works, just no valid request)."

This is a reasonable pragmatic choice, but a 401 means the API key is invalid. The endpoint is reachable but unusable. Consider marking 401 as `StatusReady` but emitting a warning, or using a distinct status like `StatusAuthError`. At minimum document why 401 is treated as healthy.

### N-05: Test table for WU-060 missing `cost.update` event test

FEAT-0008 (line 823) lists `cost.update` as a per-branch event tagged with `branch_id`. The test table for WU-060 has no test for `cost.update` events during branching. The cost.update events are defined in protocol-types with an optional `branch_id` field, and the BranchManager should emit them, but no test verifies this.

---

## Cross-Bundle Consistency

### Confirmed alignments

- `ModelInfo` struct matches protocol-types WU-041 `models.go` definition
- `ModelListResponse` and `ModelSwitchResponse` match protocol-types
- `ModelSelected` event with `json.RawMessage` for model/provider matches protocol-types D5
- `BranchStarted`, `BranchComplete`, `BranchError` event field names match protocol-types
- Branch events use `turn_id` + `branch_id` correlation per protocol-types D2
- `TurnDispatcher.Dispatch()` interface from WU-052 is correctly consumed
- `Connection` from WU-048 is correctly referenced for event delivery
- `RoutingPolicy` as `map[string]json.RawMessage` matches protocol-types definition

### Dependencies verified

- Protocol types (WU-040/041): event types and model types referenced correctly
- BFF foundation (WU-046/048): transport and connection interfaces consumed correctly
- Sessions (WU-050): `ActiveSession` used but needs `ModelOverride` field (see A-06)
- Dispatch (WU-052): `DispatchOpts` and `TurnDispatcher` consumed correctly
