# 2026-04-16 — Design: Model Config & Routing Bundle (WU-057 + WU-058 + WU-059 + WU-060)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Scope

This bundle covers the three-layer model configuration and multi-model branching in `internal/bff/`:

- **WU-057** — Provider endpoints (`providers.go`): config parsing, health checking, discovery polling, status tracking.
- **WU-058** — Model registry (`registry.go`): auto-discovery + manual config, built-in catalog, duplicate resolution, refresh.
- **WU-059** — Hierarchical routing policy (`routing.go`): dot-path resolution, single/multi-model roles, `model.list`/`model.switch` handlers.
- **WU-060** — Multi-model branching (`branch.go`): parallel goroutines, branch-tagged events, aggregate completion, cancel-all.

**Out of scope:** Streaming relay internals (WU-053, though WU-060 uses it), cost tracking (WU-056), context management (WU-061), Ollama provider adapter (WU-066 — a separate WU that implements the `Provider` interface).

## Bundle Rationale

These four WUs form the model resolution pipeline: endpoints (057) → registry (058) → routing (059) → branching (060). Each depends on the previous, and they share types like `ModelInfo`, `ProviderEndpoint`, and routing policy structures. Designing them together ensures consistent resolution semantics across the pipeline.

## Design Decisions

### D1. Package structure

```
internal/bff/
  providers.go     — WU-057: ProviderRegistry, endpoint config, health checking
  providers_test.go
  registry.go      — WU-058: ModelRegistry, auto-discovery, built-in catalog
  registry_test.go
  routing.go       — WU-059: RoutingPolicy, dot-path resolution, model.list/switch handlers
  routing_test.go
  branch.go        — WU-060: BranchManager, parallel dispatch, event tagging
  branch_test.go
```

### D2. Provider endpoints (WU-057)

#### D2.1. ProviderRegistry

```go
// ProviderRegistry manages configured provider endpoints and their health.
type ProviderRegistry struct {
    mu        sync.RWMutex
    endpoints map[string]*ProviderEndpoint // keyed by endpoint name
    
    // Health check loop
    ctx    context.Context
    cancel context.CancelFunc
}

type ProviderEndpoint struct {
    Name     string // user-defined name (e.g., "anthropic-prod", "ollama-local")
    Type     string // "anthropic", "openai", "ollama", "mlx"
    APIKey   string // secret; never logged or exposed in protocol events
    Host     string // base URL (e.g., "https://api.anthropic.com", "http://localhost:11434")
    Discover bool   // if true, poll for available models (Ollama, MLX)
    
    // Runtime state
    Status   ProviderStatus // "ready", "unavailable", "error"
    Error    string         // last error message (if status != ready)
    Models   []string       // discovered models (if Discover is true)
    LastCheck time.Time
}

type ProviderStatus string

const (
    StatusReady       ProviderStatus = "ready"
    StatusUnavailable ProviderStatus = "unavailable"
    StatusError       ProviderStatus = "error"
)

func NewProviderRegistry(config ProviderConfig) *ProviderRegistry
```

#### D2.2. Config parsing

Config lives under the `providers` key in `config.yaml` (via Viper). **Map format keyed by name** per FEAT-0008 (resolves B-01):

```yaml
providers:
  anthropic-prod:
    type: anthropic
    api_key: ${ANTHROPIC_API_KEY}
    # host defaults to https://api.anthropic.com for type: anthropic
  openai-prod:
    type: openai
    api_key: ${OPENAI_API_KEY}
  ollama-local:
    type: ollama
    host: http://localhost:11434
    discover: true
  mlx-local:
    type: mlx
    host: http://localhost:8080
    discover: true
```

```go
type ProviderConfig struct {
    Endpoints map[string]ProviderEndpointConfig `mapstructure:"providers"` // keyed by name
}

type ProviderEndpointConfig struct {
    Type     string `mapstructure:"type"`
    APIKey   string `mapstructure:"api_key"` // supports ${ENV_VAR} expansion
    Host     string `mapstructure:"host"`
    Discover bool   `mapstructure:"discover"`
}
// The map key is the endpoint name (e.g., "anthropic-prod").
```

Environment variable expansion: API key values starting with `${` and ending with `}` are resolved from `os.Getenv`. Missing env vars → endpoint marked as `StatusUnavailable` with error message.

#### D2.3. Health checking

```go
// StartHealthChecks begins periodic health checking of all endpoints.
// Ollama/MLX endpoints with discover=true are polled for model lists.
func (r *ProviderRegistry) StartHealthChecks(interval time.Duration)

// CheckEndpoint validates a single endpoint's availability.
func (r *ProviderRegistry) CheckEndpoint(ctx context.Context, ep *ProviderEndpoint) error
```

Per-type health checks:
- **Anthropic/OpenAI**: HEAD request to base URL with auth header. Success = any 2xx or 401 (auth works, just no valid request). Timeout = 5s.
- **Ollama**: GET `{host}/api/tags`. Success = 200 with model list. Updates `ep.Models`.
- **MLX**: GET `{host}/v1/models`. Success = 200 with model list.

Health check interval: configurable, default 60s. First check at startup (blocking — server waits for initial health before accepting connections).

#### D2.4. API key security

API keys MUST NOT appear in:
- Log output (masked as `"****"`)
- Protocol events (`HealthResponse`, diagnostics, errors)
- `session.details` or any export path
- Config file permission warnings (emit warning if config file is world-readable `0644` or worse)

```go
// MaskAPIKey returns a masked version of an API key for logging.
func MaskAPIKey(key string) string // "sk-ant-...1234" → "sk-ant-****1234"
```

### D3. Model registry (WU-058)

#### D3.1. ModelRegistry

```go
// ModelRegistry maintains the catalog of available models across all providers.
type ModelRegistry struct {
    mu       sync.RWMutex
    models   map[string]*ModelEntry // keyed by canonical model name
    providers *ProviderRegistry
}

type ModelEntry struct {
    Info       protocol.ModelInfo
    Provider   string // endpoint name that serves this model
    Source     string // "builtin", "discovered", "manual"
    Available  bool   // false if provider is unavailable
}

func NewModelRegistry(providers *ProviderRegistry) *ModelRegistry
```

#### D3.2. Built-in catalog

Cloud providers have well-known model lists:

```go
var builtinModels = map[string][]protocol.ModelInfo{
    "anthropic": {
        {Name: "claude-opus-4-6", Provider: "anthropic", ContextWindow: 1000000, CostPer1kInput: 0.015, CostPer1kOutput: 0.075},
        {Name: "claude-sonnet-4-6", Provider: "anthropic", ContextWindow: 200000, CostPer1kInput: 0.003, CostPer1kOutput: 0.015},
        {Name: "claude-haiku-4-5", Provider: "anthropic", ContextWindow: 200000, CostPer1kInput: 0.0008, CostPer1kOutput: 0.004},
        // ... additional models
    },
    "openai": {
        {Name: "gpt-5", Provider: "openai", ContextWindow: 256000, CostPer1kInput: 0.005, CostPer1kOutput: 0.015},
        {Name: "o4-mini", Provider: "openai", ContextWindow: 200000, CostPer1kInput: 0.0011, CostPer1kOutput: 0.0044},
        // ... additional models
    },
}
```

#### D3.3. Auto-discovery

For endpoints with `discover: true`, the registry polls the provider's model list:

```go
// RefreshDiscovery updates the registry with models from discoverable endpoints.
func (r *ModelRegistry) RefreshDiscovery()
```

Discovery sources:
- **Ollama**: parses `/api/tags` response → model names
- **MLX**: parses `/v1/models` response → model names

Discovered models are added with `Source: "discovered"`. If a discovered model has the same name as a builtin, the discovered version takes precedence (it may have different context window or capabilities based on the local instance).

#### D3.4. Manual overrides

Users can define custom model entries in config. **Map format keyed by model name** per FEAT-0008 (resolves B-02):

```yaml
models:
  llama-3.1-8b:
    provider: ollama-local
    context_window: 8192
    capabilities: [tool_use]
    description: "Fast local model for coding tasks"
```

```go
type ModelConfig map[string]ModelOverrideConfig // keyed by model name

type ModelOverrideConfig struct {
    Provider      string   `mapstructure:"provider"`
    ContextWindow int      `mapstructure:"context_window"`
    Capabilities  []string `mapstructure:"capabilities"`
    Description   string   `mapstructure:"description"`
}
```

Manual entries override both builtin and discovered entries.

#### D3.5. Duplicate resolution

Resolution order (highest priority wins):
1. Manual config entries
2. Discovered models
3. Built-in catalog

When duplicates exist across providers (e.g., same model name from two Ollama instances), the first provider in config order wins.

#### D3.6. Lookup

```go
// Get returns a model entry by canonical name. Returns nil if not found.
func (r *ModelRegistry) Get(name string) *ModelEntry

// All returns all available models.
func (r *ModelRegistry) All() []protocol.ModelInfo

// ByProvider returns models from a specific provider endpoint.
func (r *ModelRegistry) ByProvider(endpointName string) []protocol.ModelInfo

// ByRole returns models that are assigned to a given routing role.
func (r *ModelRegistry) ByRole(role string) []*ModelEntry
```

### D4. Hierarchical routing policy (WU-059)

#### D4.1. Routing policy structure

```yaml
routing:
  default: claude-sonnet-4-6
  coding:
    default: claude-sonnet-4-6
    review: [claude-opus-4-6, gpt-5]    # multi-model
  writing:
    default: claude-opus-4-6
  cheap: claude-haiku-4-5
```

Dot-path resolution: `coding.review` → `coding.default` → `default`.

```go
// RoutingPolicy resolves model names from a hierarchical routing config.
type RoutingPolicy struct {
    tree map[string]json.RawMessage // matches protocol.RoutingPolicy (map[string]json.RawMessage)
}

func NewRoutingPolicy(config map[string]json.RawMessage) *RoutingPolicy

// Resolve walks the dot-path from most-specific to least-specific,
// returning the first match. Returns (models, isMulti, found).
// For single-model: models has one entry, isMulti=false.
// For multi-model: models has multiple entries, isMulti=true.
func (rp *RoutingPolicy) Resolve(path string) (models []string, isMulti bool, found bool)
```

Resolution algorithm — flat 3-step per FEAT-0008 (resolves B-03):
1. Look up `path` exactly in tree (e.g., `coding.review`) → if found, return
2. Replace last segment with `default` (e.g., `coding.review` → `coding.default`) → if found, return
3. Look up top-level `default` → if found, return
4. Not found

**No deeper recursion.** The resolution is exactly 3 lookups, not recursive. FEAT-0008 defines a flat `category.role → category.default → default` chain, not an arbitrarily nested tree. Deeper nesting in the config (e.g., `coding.review.fast`) is allowed for organizational purposes but only the leaf path and its two fallbacks are checked.

Value parsing: each tree value is either a JSON string (single model) or a JSON array of strings (multi-model).

#### D4.2. Model resolution for turn.submit

```go
// ResolveForTurn determines which model(s) to use for a turn.
// Checks session override first, then routing policy.
//
// v1 simplification: uses mode name ("plan", "build", "auto") as the routing path.
// FEAT-0008 envisions domain roles (coding.review, infrastructure.code) which
// would require task-classification logic not yet implemented. For v1, the mode
// name serves as a simple routing key. Domain-based routing is deferred.
func (rp *RoutingPolicy) ResolveForTurn(session *ActiveSession, mode protocol.Mode) (models []string, isMulti bool) {
    // 1. Session-level override (from model.switch)
    if session.ModelOverride != "" {
        return []string{session.ModelOverride}, false
    }
    
    // 2. Route by mode as routing path (v1 simplification)
    path := string(mode) // "plan", "build", "auto"
    models, isMulti, found := rp.Resolve(path)
    if found {
        return models, isMulti
    }
    
    // 3. Fall back to default
    models, isMulti, _ = rp.Resolve("default")
    return models, isMulti
}
```

#### D4.3. model.list handler

```go
func handleModelList(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Returns `protocol.ModelListResponse` with:
- `Models`: all models from registry with availability status
- `CurrentOverride`: session's model override (if any)
- `RoutingPolicy`: the routing tree from config

#### D4.4. model.switch handler

```go
func handleModelSwitch(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Params: `protocol.ModelSwitch{SessionID, Model}`. `Model` set to `"auto"` means clear override (per FEAT-0009 `/model auto`). No `Action` field — the sentinel value is sufficient.

Flow:
1. If `Model == "auto"`:
   - Clear `session.ModelOverride`
   - Persist to `store.UpdateSession()`
2. Otherwise:
   - Verify model exists in registry → `CodeModelUnavailable` with `MT-CONN-011` if not
   - Set `session.ModelOverride = model`
   - Persist
3. Emit `model.selected` notification with the resolved model and reason
4. Return `protocol.ModelSwitchResponse`

### D5. Multi-model branching (WU-060)

#### D5.1. BranchManager

```go
// BranchManager coordinates parallel provider calls for multi-model turns.
type BranchManager struct {
    conn       *Connection
    dispatcher *TurnDispatcher
}

func NewBranchManager(conn *Connection, dispatcher *TurnDispatcher) *BranchManager
```

#### D5.2. Branch execution

```go
// ExecuteMultiModel dispatches a turn to multiple models in parallel.
// Each model gets its own goroutine. Events are tagged with branch_id.
// Returns an aggregate result after all branches complete or are cancelled.
func (bm *BranchManager) ExecuteMultiModel(ctx context.Context, opts MultiModelOpts) (*AggregateResult, error)

type MultiModelOpts struct {
    DispatchOpts          // embedded — shares Conversation, SystemPrompt, Tools, etc.
    Models       []string // model names to dispatch to (overrides DispatchOpts.Model)
    TurnID       string
}

type AggregateResult struct {
    Branches []BranchResult
    TotalCost float64
    TotalLatency time.Duration
}

type BranchResult struct {
    BranchID     string
    Model        string
    Provider     string
    Response     *provider.Message
    InputTokens  int64
    OutputTokens int64
    Cost         float64
    Latency      time.Duration
    Error        error
}
```

#### D5.3. Branch lifecycle events

Per branch, the following events are emitted via the connection's transport:

1. **`branch.started`** — emitted when the branch goroutine begins
   ```json
   {"turn_id": "...", "branch_id": "br_001", "model": "claude-opus-4-6", "provider": "anthropic"}
   ```

2. **Branch-tagged streaming events** — `token.delta`, `tool.call`, etc. with `branch_id` field set
   ```json
   {"turn_id": "...", "branch_id": "br_001", "content": "Here is my analysis..."}
   ```

3. **`branch.complete`** or **`branch.error`** — per-branch terminal event
   ```json
   {"turn_id": "...", "branch_id": "br_001", "tokens_in": 1247, "tokens_out": 3891, "cost": 0.08}
   ```

4. **`turn.complete`** — aggregate event after ALL branches finish
   - `final_input_tokens`: sum of all branches
   - `final_output_tokens`: sum of all branches
   - `total_cost`: sum of all branches

#### D5.4. Branch execution flow

```go
func (bm *BranchManager) executeBranch(ctx context.Context, branchID, model string, opts MultiModelOpts) BranchResult
```

Per-branch goroutine:
1. Resolve model → provider endpoint via registry
2. Emit `branch.started` notification
3. Dispatch to provider (via `TurnDispatcher.Dispatch()`)
4. Stream relay: forward SSE chunks as `token.delta` with `branch_id` set
5. On completion: emit `branch.complete`
6. On error: emit `branch.error`
7. Return `BranchResult`

#### D5.5. Cancellation

`turn.cancel` cancels ALL branches:

```go
func (bm *BranchManager) CancelAll()
```

Each branch goroutine monitors a shared `ctx.Done()`. On cancellation:
- In-flight HTTP requests are cancelled via context
- Each branch emits `branch.error` with `cancelled: true`
- Aggregate `turn.complete` is emitted with `cancelled: true`

#### D5.6. Branch state for session.sync

The `BranchManager` exposes branch state for `session.sync` (WU-064):

```go
// BranchState returns the current state of all active branches.
// Used by session.sync to report multi-model state on reconnection.
func (bm *BranchManager) BranchState() *protocol.MultiModelState
```

Returns `nil` if no multi-model turn is active. Otherwise returns `protocol.MultiModelState{Reviewers: []ReviewerState{...}}` with per-branch status.

## Test Strategy

### WU-057 tests (`providers_test.go`)

| Test | Description |
|------|-------------|
| `TestProviderRegistry_ParseConfig` | Config parsed correctly, endpoints created |
| `TestProviderRegistry_EnvVarExpansion` | `${VAR}` in api_key resolved from env |
| `TestProviderRegistry_MissingEnvVar` | Missing env var → endpoint unavailable |
| `TestProviderRegistry_HealthCheck_Anthropic` | Successful health check → StatusReady |
| `TestProviderRegistry_HealthCheck_Ollama` | Ollama /api/tags → models discovered |
| `TestProviderRegistry_HealthCheck_Unavailable` | Failed check → StatusUnavailable |
| `TestProviderRegistry_PeriodicHealthCheck` | Health checks run at interval |
| `TestProviderRegistry_APIKeyMasking` | MaskAPIKey masks correctly |
| `TestProviderRegistry_MultipleEndpointsSameType` | Multiple Anthropic endpoints coexist |

### WU-058 tests (`registry_test.go`)

| Test | Description |
|------|-------------|
| `TestModelRegistry_BuiltinCatalog` | Built-in models populated for Anthropic/OpenAI |
| `TestModelRegistry_Discovery` | Discovered models added from Ollama |
| `TestModelRegistry_ManualOverride` | Manual config overrides builtin |
| `TestModelRegistry_DuplicateResolution` | First provider in config wins |
| `TestModelRegistry_Get` | Lookup by canonical name |
| `TestModelRegistry_ByProvider` | Filter by provider endpoint |
| `TestModelRegistry_ByRole` | Filter by routing role |
| `TestModelRegistry_Refresh` | RefreshDiscovery updates models |
| `TestModelRegistry_UnavailableProvider` | Models marked unavailable when provider down |

### WU-059 tests (`routing_test.go`)

| Test | Description |
|------|-------------|
| `TestRouting_ExactMatch` | `coding.review` matches exact path |
| `TestRouting_DefaultFallback` | `coding.review` → `coding.default` |
| `TestRouting_TopLevelFallback` | `unknown` → `default` |
| `TestRouting_MultiModel` | Array value returns multiple models |
| `TestRouting_SingleModel` | String value returns single model |
| `TestRouting_NoDefault` | No default configured → not found |
| `TestRouting_SessionOverride` | Override takes precedence over routing |
| `TestRouting_OverrideCleared` | After clear, routing policy resumes |
| `TestRouting_ModelList` | model.list returns full registry + routing |
| `TestRouting_ModelSwitch_Set` | model.switch set persists override |
| `TestRouting_ModelSwitch_Clear` | model.switch clear removes override |
| `TestRouting_ModelSwitch_Unknown` | Unknown model → CodeModelUnavailable |
| `TestRouting_ModelSelected_Event` | model.selected notification emitted |

### WU-060 tests (`branch_test.go`)

| Test | Description |
|------|-------------|
| `TestBranch_ParallelExecution` | Multiple models dispatched concurrently |
| `TestBranch_BranchStartedEvent` | branch.started emitted per branch |
| `TestBranch_StreamingWithBranchID` | token.delta events carry branch_id |
| `TestBranch_BranchCompleteEvent` | branch.complete emitted per branch |
| `TestBranch_BranchErrorEvent` | Provider error → branch.error |
| `TestBranch_AggregateComplete` | turn.complete sums all branch metrics |
| `TestBranch_CancelAll` | turn.cancel cancels all branches |
| `TestBranch_CancelInFlight` | Cancel during streaming stops all |
| `TestBranch_PartialFailure` | One branch fails, others continue |
| `TestBranch_BranchState` | BranchState() returns correct per-branch status |
| `TestBranch_SingleModel_NoBranching` | Single model → no branching overhead |

## Key Files

| Action | Path | WU |
|--------|------|----|
| NEW | `internal/bff/providers.go` | 057 |
| NEW | `internal/bff/registry.go` | 058 |
| NEW | `internal/bff/routing.go` | 059 |
| NEW | `internal/bff/branch.go` | 060 |
| NEW | `internal/bff/*_test.go` | all |
| MODIFY | `internal/config/config.go` | 057 |

## Dependencies Consumed

- `internal/bff/transport.go` (WU-046): `SendNotification` for branch events
- `internal/bff/connection.go` (WU-048): `Connection` for event delivery
- `internal/bff/dispatch.go` (WU-052): `TurnDispatcher.Dispatch()` for per-branch HTTP calls
- `internal/protocol/models.go` (WU-041): `ModelInfo`, `ModelListResponse`, `ModelSwitchResponse`, `RoutingPolicy`
- `internal/protocol/events.go` (WU-040): `ModelSelected`, `BranchStarted`, `BranchComplete`, `BranchError`
- `internal/provider/provider.go` (WU-042): `Provider` interface
- `internal/storage/store.go` (WU-045): `Store.UpdateSession()` for model override persistence

## Interfaces Exported (consumed by downstream WUs)

- **WU-052 (Dispatch)**: uses `ProviderRegistry` to look up provider adapters by name
- **WU-053 (Streaming)**: uses branch events for multi-model streaming relay
- **WU-061 (Compaction)**: uses routing policy to resolve `compact_model` via `cheap` role
- **WU-062 (Content transform)**: uses routing policy to resolve cheap model
- **WU-065 (CLI)**: uses `ProviderRegistry.Status()` for `server status` command
- **WU-066 (Ollama)**: registers as a provider in `ProviderRegistry`
