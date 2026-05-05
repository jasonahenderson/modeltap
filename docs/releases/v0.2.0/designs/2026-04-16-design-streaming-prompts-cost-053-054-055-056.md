# 2026-04-16 — Design: Streaming, Prompts, Cost Bundle (WU-053 + WU-054 + WU-055 + WU-056)

## Scope

This bundle covers streaming relay, system prompt assembly, and cost tracking in `internal/bff/`:

- **WU-053** — Streaming relay (`streaming.go`): SSE parsing from providers, `token.delta` emission, response accumulation, `turn.complete`, cancellation, background logging.
- **WU-054** — System prompt engine layers 1-5 (`prompt.go`): core behavioral, tool-use, domain, project instructions, mode layer. Token counting.
- **WU-055** — System prompt engine layers 6-7 and assembly (`prompt.go`): knowledge stub, session state, full pipeline with trimming, per-turn reassembly.
- **WU-056** — Cost tracking and metrics (`cost.go`): per-turn cost from tokens + pricing, session totals, `cost.update` events, aggregation table feed.

**Out of scope:** Content transform (WU-062), compaction (WU-061), multi-model branching streaming (WU-060 — though streaming relay serves as the per-branch implementation).

## Bundle Rationale

These four WUs form the hot path of a turn: the system prompt is assembled (054/055), the provider response is streamed back (053), and cost is tracked (056). The prompt engine feeds into the dispatch options, the streaming relay uses the prompt's token budget, and cost tracking depends on streaming completion. Designing them together ensures consistent token accounting and prompt budget integration.

## Design Decisions

### D1. Package structure

```
internal/bff/
  streaming.go     — WU-053: StreamRelay, SSE parsing, event emission
  streaming_test.go
  prompt.go        — WU-054/055: PromptEngine, 7-layer assembly, trimming
  prompt_test.go
  cost.go          — WU-056: CostTracker, pricing table, event emission
  cost_test.go
```

### D2. Streaming relay (WU-053)

#### D2.1. StreamRelay

```go
// StreamRelay receives SSE chunks from a provider HTTP response,
// translates them into protocol events, and sends them to the harness
// via the connection's transport.
type StreamRelay struct {
    conn     *Connection
    session  *ActiveSession
    turnID   string
    branchID string // empty for single-model turns
}

func NewStreamRelay(conn *Connection, session *ActiveSession, turnID, branchID string) *StreamRelay
```

#### D2.2. Relay flow

```go
// Relay reads the provider HTTP response as SSE, emits protocol events,
// accumulates the full response, and persists the assistant turn on completion.
// Runs in a dedicated goroutine (launched by turn.submit handler in WU-052).
func (sr *StreamRelay) Relay(ctx context.Context, httpResp *http.Response, adapter provider.Provider) error
```

Flow:
1. Create provider-specific stream parser via `adapter.ParseStream(httpResp.Body)` (existing `ReassembleStream` method on the Provider interface, extended to return a channel or iterator of stream events)
2. For each SSE event from the provider:
   a. **Text chunk** → emit `token.delta` notification:
      ```go
      conn.transport.SendNotification(&protocol.Notification{
          Method: "token.delta",
          Params: marshal(TokenDelta{TurnID: turnID, BranchID: branchID, Text: chunk}),
      })
      ```
   b. **Tool call** → accumulate tool call, emit `tool.call` notification
   c. **Usage stats** → accumulate token counts
3. On stream completion:
   a. Build `AssistantResponse` from accumulated data
   b. `session.Conversation.AppendAssistantTurn(response)`
   c. Persist turn via `store.CreateTurn()`
   d. Update session metrics (total tokens, total cost, context_pct)
   e. Emit `turn.complete` notification
   f. Update cost tracker (WU-056)
   g. Log to capture store (ADR-0005) in background goroutine
4. On error (provider stream error, connection closed):
   a. Emit `error` notification with diagnostic
   b. Still persist partial response if any tokens received
5. On cancellation (`turn.cancel`):
   a. Close HTTP response body (cancels upstream request)
   b. Emit `turn.complete` with `cancelled: true`
   c. Persist partial response

#### D2.3. SSE parsing

```go
// SSEParser reads Server-Sent Events from an io.Reader.
// Provider adapters return pre-parsed chunks; this is the generic SSE
// framing layer that splits `data:` lines and handles `[DONE]` sentinels.
type SSEParser struct {
    reader *bufio.Reader
}

func NewSSEParser(r io.Reader) *SSEParser

// Next returns the next SSE data payload, or io.EOF when done.
func (p *SSEParser) Next() ([]byte, error)
```

Provider-specific stream parsing:
- **Anthropic**: `content_block_delta` events with `delta.text`, `content_block_start` for tool calls
- **OpenAI**: `choices[0].delta.content` for text, `choices[0].delta.tool_calls` for tool calls

The `Provider` interface (from WU-042) must expose a method to parse its native streaming format:

```go
// ParseStreamEvent converts a provider-specific SSE data payload into
// a canonical StreamEvent. Returns nil for events that should be skipped.
func (p *AnthropicProvider) ParseStreamEvent(data []byte) (*StreamEvent, error)

type StreamEvent struct {
    Type      string // "text", "tool_call_start", "tool_call_delta", "tool_call_end", "usage", "done", "error"
    Content   string // text content (for "text" type)
    ToolCall  *provider.ToolCall // partial or complete tool call
    Usage     *Usage
    Error     string
}

type Usage struct {
    InputTokens  int
    OutputTokens int
}
```

**Note:** This `ParseStreamEvent` method is a new addition to the `Provider` interface beyond what WU-042 designed. It will be added as an amendment to the provider formatting design. The existing `ReassembleStream` method from v0.1 proxy handles raw capture; `ParseStreamEvent` adds semantic parsing for the BFF's streaming relay.

#### D2.4. Background logging (ADR-0005)

After streaming completes, the full request/response is logged to the existing capture store:

```go
go func() {
    // Log to v0.1 capture store for metrics/export compatibility
    store.SaveRequest(ctx, &storage.CapturedRequest{...})
}()
```

This runs in a background goroutine to not block the streaming response path.

### D3. System prompt engine — layers 1-5 (WU-054)

#### D3.1. PromptEngine

```go
// PromptEngine assembles the system prompt from 7 layers.
// Each layer contributes a text segment with a token budget.
type PromptEngine struct {
    // Layer sources
    coreBehavioral string // Layer 1: bundled asset
    toolCatalog    func() []protocol.ToolDefinition // Layer 2: from CapabilityManager
    domainConfig   func() string                     // Layer 3: from config
    projectCtx     func() protocol.ProjectContext     // Layer 4: from CapabilityManager
    modeLayer      func(protocol.Mode) string        // Layer 5: mode-specific instructions
    
    // Token accounting
    windowSize int // total context window for the resolved model
}

func NewPromptEngine(opts PromptEngineOpts) *PromptEngine

type PromptEngineOpts struct {
    CoreBehavioral string
    WindowSize     int
}
```

#### D3.2. Layer definitions

| Layer | Source | Content | Priority (trim order) |
|-------|--------|---------|----------------------|
| 1 | Bundled asset (`internal/bff/assets/core_behavioral.md`) | Core behavioral instructions: role definition, safety, output format | Never trimmed |
| 2 | `CapabilityManager.Tools()` | Tool-use instructions: "You have access to these tools: [catalog]" | Never trimmed |
| 3 | Config `domain_instructions` key | Domain-specific instructions from config file | Trim 3rd |
| 4 | `CapabilityManager.ProjectContext().ConfigContent` | Project instructions (CLAUDE.md, .cursorrules, etc.) | Trim 2nd |
| 5 | Mode-dependent | Plan: "analyze but do not modify"; Build: "execute directly"; Auto: "decide when to act" | Never trimmed |

```go
// AssembleLayers1to5 builds the system prompt layers 1-5.
func (pe *PromptEngine) AssembleLayers1to5(mode protocol.Mode) []PromptLayer

type PromptLayer struct {
    Number  int
    Name    string
    Content string
    Tokens  int // estimated tokens
    Pinned  bool // if true, never trimmed
}
```

#### D3.3. Tool-use instructions (Layer 2)

```go
func (pe *PromptEngine) toolUseInstructions() string {
    tools := pe.toolCatalog()
    if len(tools) == 0 {
        return ""
    }
    var sb strings.Builder
    sb.WriteString("You have access to the following tools:\n\n")
    for _, t := range tools {
        sb.WriteString(fmt.Sprintf("## %s\n%s\n\nInput schema:\n```json\n%s\n```\n\n",
            t.Name, t.Description, string(t.InputSchema)))
    }
    return sb.String()
}
```

#### D3.4. Mode-specific instructions (Layer 5)

```go
var modeInstructions = map[protocol.Mode]string{
    protocol.ModePlan:  "You are in PLAN mode. Analyze the request and propose a plan. Do NOT make file modifications, execute commands, or take actions. Only read files and describe what you would do.",
    protocol.ModeBuild: "You are in BUILD mode. Execute the user's request directly. Make file changes, run commands, and take actions as needed.",
    protocol.ModeAuto:  "You are in AUTO mode. Decide whether to plan or execute based on the complexity of the request. For simple tasks, execute directly. For complex tasks, propose a plan first.",
}
```

### D4. System prompt engine — layers 6-7 and assembly (WU-055)

#### D4.1. Layer 6 — Knowledge injection (stub)

```go
// Layer 6 is a placeholder for FEAT-0011 knowledge injection.
// Currently returns empty string. When implemented, this layer will
// inject relevant knowledge snippets from the vector store.
func (pe *PromptEngine) knowledgeLayer() string {
    return "" // FEAT-0011 stub
}
```

#### D4.2. Layer 7 — Session state

```go
// sessionStateLayer builds Layer 7 from the session's current state.
func (pe *PromptEngine) sessionStateLayer(session *ActiveSession) string
```

Layer 7 includes:
- **Pinned items**: user-pinned context that survives compaction
- **Active plan**: if a plan was proposed and not yet executed
- **Compaction summaries**: summaries of compacted turns
- **File context**: currently attached files
- **Model override info**: if the user overrode the routing policy

```go
func (pe *PromptEngine) sessionStateLayer(session *ActiveSession) string {
    var parts []string
    
    // Pinned items
    pinned := session.PinnedItems()
    if len(pinned) > 0 {
        parts = append(parts, "## Pinned Context\n"+strings.Join(pinned, "\n\n"))
    }
    
    // Compaction summaries
    summaries := session.CompactionSummaries()
    if len(summaries) > 0 {
        parts = append(parts, "## Previous Context (compacted)\n"+strings.Join(summaries, "\n\n"))
    }
    
    // Model override notice
    if session.ModelOverride != "" {
        parts = append(parts, fmt.Sprintf("## Model Override\nUser has overridden routing to: %s", session.ModelOverride))
    }
    
    return strings.Join(parts, "\n\n")
}
```

#### D4.3. Full assembly pipeline

```go
// Assemble builds the complete system prompt from all 7 layers,
// trimming if the total exceeds the budget.
func (pe *PromptEngine) Assemble(mode protocol.Mode, session *ActiveSession) (string, int) {
    layers := pe.AssembleLayers1to5(mode)
    
    // Layer 6 (knowledge — stub)
    if knowledge := pe.knowledgeLayer(); knowledge != "" {
        layers = append(layers, PromptLayer{Number: 6, Name: "knowledge", Content: knowledge,
            Tokens: provider.EstimateTokens(knowledge), Pinned: false})
    }
    
    // Layer 7 (session state)
    if sessionState := pe.sessionStateLayer(session); sessionState != "" {
        layers = append(layers, PromptLayer{Number: 7, Name: "session_state", Content: sessionState,
            Tokens: provider.EstimateTokens(sessionState), Pinned: false})
    }
    
    // Trim if over budget
    totalTokens := sumTokens(layers)
    budget := pe.windowSize / 4 // system prompt gets 25% of context window
    
    if totalTokens > budget {
        layers = pe.trim(layers, budget)
    }
    
    return joinLayers(layers), sumTokens(layers)
}
```

#### D4.4. Trimming strategy

Trim order per FEAT-0008 "preserving Layers 1-5" (resolves BLOCKING-02):
1. Layer 6 (knowledge) — trim first since it's supplementary
2. Layer 7 (session state) — trim second (compaction summaries first, then pinned items)
3. Layers 1-5 — NEVER trimmed (core behavioral, tool-use, domain, project, mode)

If the system prompt exceeds budget after trimming layers 6 and 7, return as-is with a warning log. The design already handles the "pinned exceeds budget" case this way. Layers 3 (domain) and 4 (project) are essential for behavior quality per FEAT-0008 and must not be dropped.

```go
func (pe *PromptEngine) trim(layers []PromptLayer, budget int) []PromptLayer {
    // Calculate pinned total
    pinnedTokens := 0
    for _, l := range layers {
        if l.Pinned {
            pinnedTokens += l.Tokens
        }
    }
    if pinnedTokens > budget {
        // Pinned layers alone exceed budget — cannot trim, return as-is with warning
        return layers
    }
    
    remaining := budget - pinnedTokens
    trimOrder := []int{6, 7} // only layers 6 and 7 are trimmable per FEAT-0008
    
    for _, layerNum := range trimOrder {
        if sumTokens(layers) <= budget {
            break
        }
        for i := range layers {
            if layers[i].Number == layerNum && !layers[i].Pinned {
                layers[i].Content = ""
                layers[i].Tokens = 0
            }
        }
    }
    return layers
}
```

#### D4.5. Per-turn reassembly

The system prompt is reassembled on every `turn.submit`:
- Layer 4 (project context) is re-read from `CapabilityManager` (so config edits take effect)
- Layer 5 (mode) reflects the current mode from the `turn.submit` payload
- Layer 7 (session state) reflects current pins, summaries, and overrides

This ensures the prompt is always current without caching stale state.

### D5. Cost tracking (WU-056)

#### D5.1. CostTracker

```go
// CostTracker computes per-turn cost from token counts and pricing,
// maintains session totals, and emits cost.update events.
type CostTracker struct {
    registry *ModelRegistry // for pricing lookup
}

func NewCostTracker(registry *ModelRegistry) *CostTracker
```

#### D5.2. Per-turn cost computation

```go
// ComputeTurnCost calculates the cost of a single turn.
func (ct *CostTracker) ComputeTurnCost(model string, inputTokens, outputTokens int64) float64 {
    entry := ct.registry.Get(model)
    if entry == nil {
        return 0 // unknown model, no pricing
    }
    inputCost := float64(inputTokens) / 1000.0 * entry.Info.CostPer1kInput
    outputCost := float64(outputTokens) / 1000.0 * entry.Info.CostPer1kOutput
    return inputCost + outputCost
}
```

#### D5.3. Session cost update

After each turn completes (called from StreamRelay):

```go
// UpdateSessionCost updates the session's running cost total and emits
// a cost.update event to the harness.
func (ct *CostTracker) UpdateSessionCost(conn *Connection, session *ActiveSession, turn *storage.Turn) {
    turnCost := ct.ComputeTurnCost(turn.Model, turn.InputTokens, turn.OutputTokens)
    turn.Cost = turnCost
    
    // Update session total
    session.TotalCost += turnCost
    session.TotalInputTokens += turn.InputTokens
    session.TotalOutputTokens += turn.OutputTokens
    
    // Persist
    store.UpdateSession(...)
    
    // Emit cost.update event
    conn.transport.SendNotification(&protocol.Notification{
        Method: "cost.update",
        Params: marshal(CostUpdate{
            TurnCost:     turnCost,
            SessionTotal: session.TotalCost,
            InputTokens:  turn.InputTokens,
            OutputTokens: turn.OutputTokens,
        }),
    })
}
```

#### D5.4. Aggregation table feed

Cost data feeds into the existing v0.1 aggregation tables (hourly_usage, daily_usage):

```go
// RecordUsage writes usage metrics to the aggregation tables.
func (ct *CostTracker) RecordUsage(turn *storage.Turn) {
    // Existing SaveRequest path in v0.1 storage handles this
    // The BFF records to the same tables that the proxy uses
}
```

#### D5.5. Cost in turn.complete

The `turn.complete` event (emitted by StreamRelay) includes cost:

```go
TurnComplete{
    TurnID:           turnID,
    FinalInputTokens:  inputTokens,
    FinalOutputTokens: outputTokens,
    TotalCost:         turnCost,
    Model:             model,
    Provider:          providerName,
    LatencyMs:         latencyMs,
    Cancelled:         false,
}
```

## Test Strategy

### WU-053 tests (`streaming_test.go`)

| Test | Description |
|------|-------------|
| `TestStreamRelay_TextChunks` | SSE text chunks → token.delta events |
| `TestStreamRelay_ToolCall` | Tool call SSE → tool.call event |
| `TestStreamRelay_Complete` | Stream end → turn.complete with metrics |
| `TestStreamRelay_Cancel` | turn.cancel → response body closed, cancelled=true |
| `TestStreamRelay_PartialPersist` | Error mid-stream → partial response persisted |
| `TestStreamRelay_BranchTagging` | Events tagged with branch_id when set |
| `TestStreamRelay_BackgroundCapture` | Full request/response logged in background |
| `TestSSEParser_DataLines` | Parse `data:` prefixed lines |
| `TestSSEParser_DoneSentinel` | `[DONE]` returns io.EOF |
| `TestSSEParser_EmptyLines` | Empty lines between events handled |
| `TestStreamRelay_Anthropic` | Anthropic SSE format parsed correctly |
| `TestStreamRelay_OpenAI` | OpenAI SSE format parsed correctly |

### WU-054/055 tests (`prompt_test.go`)

| Test | Description |
|------|-------------|
| `TestPrompt_Layer1_CoreBehavioral` | Layer 1 loaded from bundled asset |
| `TestPrompt_Layer2_ToolInstructions` | Tools formatted with name, description, schema |
| `TestPrompt_Layer2_NoTools` | Empty tool catalog → empty layer |
| `TestPrompt_Layer3_DomainConfig` | Domain instructions from config |
| `TestPrompt_Layer4_ProjectContext` | Project instructions from harness |
| `TestPrompt_Layer5_PlanMode` | Plan mode instructions |
| `TestPrompt_Layer5_BuildMode` | Build mode instructions |
| `TestPrompt_Layer5_AutoMode` | Auto mode instructions |
| `TestPrompt_Layer6_Stub` | Knowledge layer returns empty (FEAT-0011 stub) |
| `TestPrompt_Layer7_PinnedItems` | Pinned items included |
| `TestPrompt_Layer7_CompactionSummaries` | Summaries included |
| `TestPrompt_Layer7_ModelOverride` | Override notice included |
| `TestPrompt_Assembly_AllLayers` | All 7 layers assembled in order |
| `TestPrompt_Assembly_WithinBudget` | Total within budget → no trimming |
| `TestPrompt_Assembly_OverBudget` | Over budget → layers trimmed in order |
| `TestPrompt_Trimming_Order` | Layer 6 trimmed first, then 7, 4, 3 |
| `TestPrompt_Trimming_PinnedPreserved` | Pinned layers never trimmed |
| `TestPrompt_PerTurnReassembly` | Prompt changes when mode/context changes |
| `TestPrompt_TokenCounting` | Token estimates match expected values |

### WU-056 tests (`cost_test.go`)

| Test | Description |
|------|-------------|
| `TestCost_ComputeTurnCost` | Input + output tokens × pricing = correct cost |
| `TestCost_UnknownModel` | Unknown model → zero cost |
| `TestCost_SessionTotal` | Running total accumulates across turns |
| `TestCost_UpdateEvent` | cost.update emitted with correct totals |
| `TestCost_TurnCompleteIncludesCost` | turn.complete event includes cost |
| `TestCost_AggregationFeed` | Usage recorded in aggregation tables |
| `TestCost_MultiModelBranch` | Branch costs summed in aggregate |

## Key Files

| Action | Path | WU |
|--------|------|----|
| NEW | `internal/bff/streaming.go` | 053 |
| NEW | `internal/bff/prompt.go` | 054, 055 |
| NEW | `internal/bff/cost.go` | 056 |
| NEW | `internal/bff/assets/core_behavioral.md` | 054 |
| NEW | `internal/bff/*_test.go` | all |
| MODIFY | `internal/provider/provider.go` | 053 (add ParseStreamEvent) |

## Dependencies Consumed

- `internal/bff/connection.go` (WU-048): `Connection.transport.SendNotification()`
- `internal/bff/session.go` (WU-050): `ActiveSession`
- `internal/bff/conversation.go` (WU-051): `Conversation.AppendAssistantTurn()`
- `internal/bff/capabilities.go` (WU-049): `CapabilityManager.Tools()`, `.ProjectContext()`
- `internal/bff/registry.go` (WU-058): `ModelRegistry.Get()` for pricing
- `internal/protocol/events.go` (WU-040): `TokenDelta`, `ToolCallEvent`, `TurnComplete`, `CostUpdate`, `CompactNotice`
- `internal/provider/provider.go` (WU-042): `Provider` interface, `Message`, token estimation
- `internal/storage/store.go` (WU-045): `Store.CreateTurn()`, `Store.UpdateSession()`

## Interfaces Exported (consumed by downstream WUs)

- **WU-060 (Multi-model)**: uses `StreamRelay` per-branch for parallel streaming
- **WU-061 (Compaction)**: uses `PromptEngine.Assemble()` for system prompt budget, triggers `compact.notice` via streaming
- **WU-062 (Content transform)**: uses routing for cheap model resolution, PromptEngine for prompt assembly
- **WU-064 (Recovery)**: uses StreamRelay's partial-persist for idempotent turn recovery
