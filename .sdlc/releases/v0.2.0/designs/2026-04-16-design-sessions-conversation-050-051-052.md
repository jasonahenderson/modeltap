# 2026-04-16 — Design: Sessions & Conversation Bundle (WU-050 + WU-051 + WU-052)

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Scope

This bundle covers session lifecycle and conversation state management in `internal/bff/`:

- **WU-050** — Session management (`session.go`): create, resume, list, details, lock, clear, fork. Auto-generated summaries.
- **WU-051** — Conversation state (`conversation.go`): canonical format, turn persistence, tool call/result correlation, attachment storage, sequence tracking.
- **WU-052** — Provider format translation and dispatch (`dispatch.go`): canonical → provider format via `FormatMessages`, HTTP request, error wrapping.

**Out of scope:** Streaming relay (WU-053), system prompt assembly (WU-054–055), cost tracking (WU-056), model routing (WU-057–060), compaction (WU-061), recovery/idempotency (WU-064).

## Bundle Rationale

These three WUs form a pipeline: sessions (050) own the lifecycle, conversations (051) own the message history within a session, and dispatch (052) translates conversations into provider calls. They share the `internal/bff/` package and deeply interleave: `session.resume` restores conversation state, `turn.submit` appends to conversation then dispatches, and session details query conversation turns. Designing them together prevents mismatched assumptions about turn ownership, sequence numbering, and lock semantics.

## Design Decisions

### D1. Package structure

```
internal/bff/
  session.go         — WU-050: SessionManager, handlers for session.* methods
  session_test.go
  conversation.go    — WU-051: Conversation, turn management, correlation
  conversation_test.go
  dispatch.go        — WU-052: Dispatcher (provider format translation + HTTP call)
  dispatch_test.go
```

### D2. Session management (WU-050)

#### D2.1. SessionManager

```go
// SessionManager handles session lifecycle operations.
// One SessionManager per Server; it uses the shared Store.
type SessionManager struct {
    store     storage.Store
    mu        sync.Mutex
    active    map[string]*ActiveSession // keyed by session ID; in-memory state for active sessions
}

// ActiveSession tracks the in-memory state of a session that is currently
// bound to a connection. Complements the persisted storage.Session.
type ActiveSession struct {
    ID            string
    Conversation  *Conversation     // in-memory conversation state
    ConnID        string            // owning connection ID
    LockExpiry    time.Time
    GraceCancel   context.CancelFunc // cancel grace-period release (from WU-048)
    ModelOverride string            // set by model.switch (WU-059); empty = use routing policy
    TotalCost     float64           // running session cost total
    TotalInputTokens  int64
    TotalOutputTokens int64
    ContextPct    float64           // last observed context pressure
    ActiveTurn    *ActiveTurnInfo   // non-nil during streaming; used by session.sync (WU-064)
    BranchManager *BranchManager   // non-nil during multi-model turn (WU-060)
}

type ActiveTurnInfo struct {
    TurnID          string
    Status          string // "streaming", "pending_tool_result", etc.
    CompletedTokens int
}

func (at *ActiveTurnInfo) Summary() string

func NewSessionManager(store storage.Store) *SessionManager

// GetActiveSession returns the in-memory state for an active session.
// Returns nil if the session is not currently active on any connection.
// Exported for WU-064 (session.sync handler) which needs to read
// conversation state, pending tool calls, and branch state.
func (sm *SessionManager) GetActiveSession(sessionID string) *ActiveSession
```

#### D2.2. Session creation (implicit on first turn.submit)

Sessions are created implicitly when a `turn.submit` arrives with no active session. The handler:

1. Generate session ID (UUID)
2. Create `storage.Session` with:
   - `UserID`: from connection auth context (empty string for solo profile)
   - `Project`: from `conn.capabilities.ProjectContext().Root`
   - `Status`: `"active"`
3. Persist via `store.CreateSession()`
4. Acquire lock via `store.AcquireSessionLock(sessionID, conn.id, expiry)`
5. Create in-memory `ActiveSession` with empty `Conversation`
6. Bind to connection: `conn.sessionID = sessionID`

No explicit `session.create` protocol method exists — creation is a side effect of the first `turn.submit`.

#### D2.3. Session resume handler

```go
func handleSessionResume(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Params: `protocol.SessionResume{SessionID, Project}`.

Flow:
1. Decode `SessionResume`
2. Load session from store: `store.GetSession(sessionID)`
   - Not found → `CodeSessionNotFound`
3. Acquire lock: `store.AcquireSessionLock(sessionID, conn.id, expiry)`
   - Contended → `CodeSessionLocked` with `MT-CONN-008` diagnostic, include current owner and lock expiry
4. Update project context: `conn.capabilities.UpdateProjectContext(params.Project)`
5. Load conversation from store: `store.ListTurns(sessionID)`
6. Build in-memory `Conversation` from persisted turns
7. Create `ActiveSession`, bind to connection
8. If the session's tool catalog differs from connection's current catalog → send `capabilities.request` with reason `"reconnection"` (per Bundle 4 D5.5)
9. Return `protocol.SessionResumeResponse` (from WU-041 `sessions.go`) — lightweight response with `session_id`, `model`, `model_override`, `project`. **NOT** `SessionDetail` (that's for `session.details`, a separate heavier query). Resolves B-02.

#### D2.4. Session list handler

```go
func handleSessionList(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Params: `protocol.SessionList{}` (empty struct — no params per WU-039).

Flow:
1. Build `storage.SessionFilter` from connection state:
   - `UserID`: from connection auth context (enforces user isolation)
   - `Project`: from `conn.capabilities.ProjectContext().Root` (scopes to current project)
   - `Status`, `Limit`, `Offset`: defaults (`""`, `50`, `0`)
2. Call `store.SessionSummaries(filter)`
3. Return `protocol.SessionListResponse{Sessions: summaries}`

Note: `SessionList` has no params because the server derives all filter criteria from the connection context. If per-request filtering is needed later, fields can be added to the struct without breaking existing clients (empty JSON object `{}` is forward-compatible).

#### D2.5. Session details handler

```go
func handleSessionDetails(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Returns `protocol.SessionDetail` with:
- `Turns`: from `store.ListTurns(sessionID)` → mapped to `protocol.TurnSummary` slice
- `ServerEvents`: from `store.ListServerEvents(sessionID)` → mapped to `protocol.ServerSessionEvent` slice
- `FilesTouched` / `FilesModified`: aggregated from turns via `store.SessionFilesTouched/Modified()`
- `PinnedItems`, `CompactionState`, `CostHistory`: from session record

#### D2.6. Session clear handler

```go
func handleSessionClear(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Clears the in-memory conversation (resets turn sequence to 0) but preserves the session in storage. Appends a `session_events` entry of type `"manual_clear"`. The harness sees a fresh context but can still see historical turns via `session.details`.

#### D2.7. Session fork handler

```go
func handleSessionFork(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

Creates an independent copy of the session:
1. Create new session with new ID, new `created_at`/`updated_at`
2. Copy turns: all turns duplicated with new session_id, same sequence numbers
3. Copy fields: `summary`, `active_model`, `pinned_items`, `compaction_state`
4. Reset fields: `total_cost=0`, `total_input_tokens=0`, `total_output_tokens=0`, `context_pct=0`, `status="active"`
5. Do NOT copy: `lock_owner`, `lock_expires_at`, `model_override`, `routing_overrides`
6. Bind the new session to the current connection (release lock on old session)
7. Append `session_events` entry of type `"fork"` to the source session
8. Return `protocol.SessionForkResponse{NewSessionID, OriginalSessionID}`

#### D2.8. Session lock mechanics

Lock timing constants (from WU-048 design):
- Lock acquired on `session.resume` or session creation
- Lock released on graceful disconnect or on connection timeout
- Grace period: `HeartbeatTimeout + GracePeriod` = 40s from last harness ping
- `MT-CONN-008` on lock contention: includes `lock_owner` fingerprint and `lock_expires_at`

Force-release via `session unlock` CLI (WU-065): calls `store.ForceReleaseSessionLock(sessionID)`. Rejects unlock of sessions with an actively-streaming turn unless `--force` is passed.

#### D2.9. Auto-generated session summaries

After each assistant turn completes, the session manager updates `session.Summary` with a brief description. Implementation options:
1. First user message content (truncated to 100 chars) — used initially
2. LLM-generated summary via cheap model (deferred to WU-062 content transform)

```go
func (sm *SessionManager) updateSummary(session *ActiveSession, turn *storage.Turn) {
    if session.Conversation.TurnCount() == 1 {
        // First user message: use as summary
        summary := turn.Content
        if len(summary) > 100 {
            summary = summary[:100] + "..."
        }
        sm.store.UpdateSession(ctx, &storage.Session{ID: session.ID, Summary: summary})
    }
}
```

### D3. Conversation state (WU-051)

#### D3.1. Conversation

```go
// Conversation manages the ordered sequence of turns for an active session.
// It maintains the canonical message format that provider adapters translate from.
type Conversation struct {
    sessionID string
    turns     []provider.Message // canonical format from WU-042
    sequence  int                // monotonically increasing turn counter
    mu        sync.RWMutex
}

func NewConversation(sessionID string) *Conversation

// RestoreFromTurns rebuilds conversation state from persisted storage turns.
// Called during session.resume.
func (c *Conversation) RestoreFromTurns(turns []storage.Turn) error

// TurnCount returns the number of turns in the conversation.
func (c *Conversation) TurnCount() int

// Sequence returns the current sequence number (for validating turn.submit).
func (c *Conversation) Sequence() int

// Messages returns a snapshot of the canonical message history.
func (c *Conversation) Messages() []provider.Message
```

#### D3.2. Appending user turns

```go
// AppendUserTurn adds a user message from turn.submit to the conversation.
// Validates sequence number (must equal current sequence + 1).
// Returns the created storage.Turn for persistence.
func (c *Conversation) AppendUserTurn(submit *protocol.TurnSubmit) (*storage.Turn, error)
```

Flow:
1. Validate `submit.Sequence == c.sequence + 1` → error if mismatch
2. Build `provider.Message{Role: "user", Content: submit.Content, Attachments: convertAttachments(submit.Attachments)}`
3. If `submit.ToolResults` is non-empty, add to message
4. Append to `c.turns`
5. Increment `c.sequence`
6. Build and return `storage.Turn` for persistence

#### D3.3. Appending assistant turns

```go
// AppendAssistantTurn adds the model's response to the conversation.
// Called when streaming completes (turn.complete event).
func (c *Conversation) AppendAssistantTurn(response AssistantResponse) (*storage.Turn, error)

type AssistantResponse struct {
    Content      string
    ToolCalls    []provider.ToolCall
    Model        string
    Provider     string
    InputTokens  int64
    OutputTokens int64
    Cost         float64
    LatencyMs    int64
}
```

Flow:
1. Build `provider.Message{Role: "assistant", Content: response.Content, ToolCalls: response.ToolCalls}`
2. Append to `c.turns`
3. Increment `c.sequence`
4. Build and return `storage.Turn` with all metrics

#### D3.4. Tool call/result correlation

When the assistant emits `tool.call` events during streaming, the conversation tracks pending tool calls:

```go
// PendingToolCalls returns tool calls from the last assistant turn
// that have not yet received results.
func (c *Conversation) PendingToolCalls() []provider.ToolCall

// MatchToolResult validates that a tool result matches a pending call.
func (c *Conversation) MatchToolResult(toolCallID string) (provider.ToolCall, bool)
```

The `tool.result` request handler (wired in the dispatcher) calls `MatchToolResult` to verify the `tool_call_id` matches a pending call. Unmatched results are rejected with `CodeInvalidParams`.

#### D3.5. Attachment and paste storage

Attachments from `turn.submit` are stored in the `storage.Turn.Content` field as part of the canonical message JSON. The `provider.Message.Attachments` slice carries the full attachment data (path, raw, content, content_type, transform). Raw bytes (base64) are stored in the turn for ADR-0005 full-capture compliance.

Pastes are stored as part of the user message content. If the harness used `content.transform` to summarize a paste (WU-062), both the original and summary are stored (original in `Attachments[].Raw`, summary in `Content`).

#### D3.6. Canonical turn serialization format (resolves B-01)

The `storage.Turn.Content` field (`json.RawMessage`) stores a JSON-serialized `provider.Message`. The serialization format is:

```json
{
  "role": "user",
  "content": "the message text",
  "tool_calls": [],
  "tool_results": [],
  "attachments": [{"path": "...", "raw": "base64...", "content": "...", "content_type": "...", "transform": "..."}],
  "metadata": {"turn_id": "...", "sequence": 1}
}
```

This is a direct `json.Marshal(provider.Message)`. The `provider.Message.Content` field is a plain string; it becomes a JSON string inside the serialized blob. On restore, `json.Unmarshal(turn.Content, &msg)` recovers the canonical form.

**Round-trip contract:** `json.Unmarshal(json.Marshal(msg))` must produce an identical `provider.Message`. Tests assert this for each message variant (user, assistant, tool call, tool result, with attachments).

#### D3.7. Conversion between storage and canonical formats

```go
// turnToMessage converts a persisted storage.Turn to a canonical provider.Message.
// Unmarshals Turn.Content (json.RawMessage) into provider.Message.
func turnToMessage(t *storage.Turn) (provider.Message, error)

// messageToTurn converts a canonical provider.Message to a storage.Turn for persistence.
// Marshals the Message into Turn.Content as JSON.
func messageToTurn(sessionID string, sequence int, msg provider.Message, meta TurnMetadata) *storage.Turn

type TurnMetadata struct {
    Model        string
    Provider     string
    InputTokens  int64
    OutputTokens int64
    Cost         float64
    LatencyMs    int64
    ToolCalls    []provider.ToolCall
    FilesTouched []string
    FilesModified []string
}
```

### D4. Provider format translation and dispatch (WU-052)

#### D4.1. TurnDispatcher

```go
// TurnDispatcher translates canonical conversation to provider-specific format
// and sends HTTP requests to provider endpoints.
// Uses ProviderRegistry (WU-057) for thread-safe provider lookup — not a static map.
// This ensures runtime discovery changes (Ollama models appearing/disappearing)
// are visible to dispatch without restart.
type TurnDispatcher struct {
    registry   *ProviderRegistry // thread-safe; from WU-057
    httpClient *http.Client
}

func NewTurnDispatcher(registry *ProviderRegistry) *TurnDispatcher
```

#### D4.2. Dispatch flow

```go
// Dispatch translates the conversation to provider format and sends the request.
// Returns the raw HTTP response for streaming relay (WU-053).
// The caller (turn.submit handler) is responsible for streaming the response.
func (d *TurnDispatcher) Dispatch(ctx context.Context, opts DispatchOpts) (*http.Response, error)

type DispatchOpts struct {
    Conversation *Conversation
    SystemPrompt string                     // pre-assembled by WU-055
    Model        string                     // resolved model identifier
    ProviderName string                     // which provider adapter to use
    MaxTokens    int                        // output token cap
    Temperature  *float64
    Tools        []protocol.ToolDefinition  // from capability manager
    Capabilities []string                   // e.g., ["vision", "tool_use"] — gates image handling in FormatMessages
    Stream       bool                       // always true for turn.submit; false for content.transform
    WindowSize   int                        // context window budget
}
```

Flow:
1. Look up provider adapter by `ProviderName`
   - Not found → error with `MT-CONN-009` diagnostic
2. Build `provider.FormatMessagesOpts` from `DispatchOpts`:
   - `Messages`: `opts.Conversation.Messages()`
   - `SystemPrompt`: `opts.SystemPrompt`
   - `Model`: `opts.Model`
   - `MaxTokens`: `opts.MaxTokens`
   - `Temperature`: `opts.Temperature`
   - `Tools`: `opts.Tools`
   - `Stream`: `opts.Stream`
   - `WindowSize`: `opts.WindowSize`
3. Call `adapter.FormatMessages(fmOpts)` → request body bytes
   - On `provider.ErrWindowTooSmall` → error with appropriate diagnostic
   - On `provider.ErrTruncationEmpty` → error with diagnostic
4. Build HTTP request:
   - URL from provider endpoint config (WU-057; for now, hardcoded base URLs)
   - Headers: `Authorization: Bearer <api_key>`, `Content-Type: application/json`
   - For Anthropic: `x-api-key: <key>`, `anthropic-version: 2023-06-01`
5. Send via `httpClient.Do(req)`
6. On HTTP error (non-2xx): wrap in `RPCError` with `CodeProviderError` and diagnostic including status code, provider error message
7. Return raw `*http.Response` for streaming relay

#### D4.3. Non-streaming dispatch

For `content.transform` (WU-062) and other non-streaming calls:

```go
// DispatchSync sends a non-streaming request and returns the complete response.
func (d *TurnDispatcher) DispatchSync(ctx context.Context, opts DispatchOpts) (*provider.Message, error)
```

Reads the full response body, unmarshals via the provider adapter's response parser, returns a canonical `provider.Message`.

#### D4.4. Error wrapping with diagnostic codes

```go
// dispatchError wraps a provider-layer error with the appropriate diagnostic code.
func dispatchError(err error, providerName string) error {
    switch {
    case errors.Is(err, provider.ErrWindowTooSmall):
        return &RPCError{Code: CodeProviderError, Message: "context window too small",
            Data: marshalDiagnostic("MT-CONN-009", "budget_exceeded", err.Error())}
    case errors.Is(err, provider.ErrNotImplemented):
        return &RPCError{Code: CodeProviderError, Message: "provider does not support this operation"}
    default:
        return &RPCError{Code: CodeProviderError, Message: err.Error(),
            Data: marshalDiagnostic("MT-CONN-009", "provider_unavailable", err.Error())}
    }
}
```

#### D4.5. turn.submit handler wiring

The `turn.submit` handler (registered in the dispatcher by `Server.Start()`) orchestrates the full pipeline:

```go
func handleTurnSubmit(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
    // 1. Validate (sequence, mode) — per Bundle 4 D2.5
    submit, err := ValidateTurnSubmit(params)
    
    // 2. Session: create if needed, or verify active
    session := conn.server.sessions.GetOrCreate(conn)
    
    // 3. Conversation: append user turn
    userTurn, err := session.Conversation.AppendUserTurn(submit)
    
    // 4. Persist user turn
    conn.server.store.CreateTurn(ctx, userTurn)
    
    // 5. Record in command history (WU-091)
    conn.server.store.AppendCommandHistory(ctx, &storage.CommandHistoryEntry{
        UserID:    session.UserID,
        Project:   session.Project,
        SessionID: &session.ID,
        Content:   submit.Content,
    })
    
    // 6. Assemble system prompt (WU-054/055 — stub until then)
    systemPrompt := "" // placeholder
    
    // 7. Resolve model (WU-059 — stub until then)
    model, providerName := "default", "default"
    
    // 8. Dispatch to provider
    httpResp, err := conn.server.dispatcher.Dispatch(ctx, DispatchOpts{...})
    
    // 9. Start streaming relay (WU-053 — stub: just read and close for now)
    go conn.server.streamRelay(ctx, conn, session, httpResp, submit.TurnID)
    
    // 10. Return ack
    return &protocol.TurnSubmitResponse{TurnID: submit.TurnID, Status: "accepted"}, nil
}
```

Steps 6, 7, and 9 are stubs that will be filled by downstream WUs (054/055, 059, 053).

## Test Strategy

### WU-050 tests (`session_test.go`)

| Test | Description |
|------|-------------|
| `TestSession_CreateOnFirstTurn` | First turn.submit creates session implicitly |
| `TestSession_Resume_Success` | Resume existing session, load conversation |
| `TestSession_Resume_NotFound` | Unknown session ID → CodeSessionNotFound |
| `TestSession_Resume_Locked` | Session locked by another → CodeSessionLocked with MT-CONN-008 |
| `TestSession_Resume_LockExpired` | Expired lock → acquire succeeds |
| `TestSession_Resume_ProjectContextRefresh` | Resume updates project context |
| `TestSession_List_UserIsolation` | List only returns current user's sessions |
| `TestSession_List_ProjectFilter` | List filters by project |
| `TestSession_Details_Timeline` | Details returns turns with metadata |
| `TestSession_Details_ServerEvents` | Details returns server events |
| `TestSession_Details_FilesTouched` | Details aggregates files from turns |
| `TestSession_Clear` | Clear resets conversation, preserves session |
| `TestSession_Fork` | Fork creates independent copy |
| `TestSession_Fork_NoLockCopy` | Forked session is unlocked |
| `TestSession_Lock_GracePeriod` | Lock released 40s after last ping |
| `TestSession_Lock_GraceCancel` | Reconnect cancels grace release |
| `TestSession_Lock_ForceRelease` | Force-release works regardless of grace |
| `TestSession_Summary_FirstMessage` | Summary set from first user message |

### WU-051 tests (`conversation_test.go`)

| Test | Description |
|------|-------------|
| `TestConversation_AppendUserTurn` | User turn appended, sequence incremented |
| `TestConversation_AppendUserTurn_BadSequence` | Wrong sequence → error |
| `TestConversation_AppendAssistantTurn` | Assistant turn appended with metrics |
| `TestConversation_ToolCallCorrelation` | Pending tool calls tracked correctly |
| `TestConversation_ToolResult_Matched` | Tool result matches pending call |
| `TestConversation_ToolResult_Unmatched` | Unknown tool_call_id → rejected |
| `TestConversation_RestoreFromTurns` | Restore from storage rebuilds state |
| `TestConversation_RestoreFromTurns_WithToolCalls` | Restore handles tool call/result pairs |
| `TestConversation_Messages_Snapshot` | Messages() returns copy |
| `TestConversation_Attachments_Stored` | Attachments persisted in turn content |
| `TestConversation_TurnToMessage` | storage.Turn → provider.Message round-trip |
| `TestConversation_MessageToTurn` | provider.Message → storage.Turn round-trip |

### WU-052 tests (`dispatch_test.go`)

Tests use `httptest.Server` as mock provider:

| Test | Description |
|------|-------------|
| `TestDispatch_Anthropic` | Dispatch to Anthropic adapter, verify request body format |
| `TestDispatch_OpenAI` | Dispatch to OpenAI adapter, verify request body format |
| `TestDispatch_ProviderNotFound` | Unknown provider → error with MT-CONN-009 |
| `TestDispatch_WindowTooSmall` | FormatMessages returns ErrWindowTooSmall → diagnostic |
| `TestDispatch_TruncationEmpty` | FormatMessages returns ErrTruncationEmpty → diagnostic |
| `TestDispatch_HTTPError` | Provider returns 4xx/5xx → CodeProviderError with details |
| `TestDispatch_Streaming` | Stream=true returns raw *http.Response |
| `TestDispatchSync_Success` | Non-streaming round-trip with parsed response |
| `TestDispatch_AuthHeaders_Anthropic` | x-api-key header set for Anthropic |
| `TestDispatch_AuthHeaders_OpenAI` | Authorization: Bearer header set for OpenAI |
| `TestTurnSubmit_FullPipeline` | turn.submit → session create → conversation append → dispatch |

## Key Files

| Action | Path | WU |
|--------|------|----|
| NEW | `internal/bff/session.go` | 050 |
| NEW | `internal/bff/session_test.go` | 050 |
| NEW | `internal/bff/conversation.go` | 051 |
| NEW | `internal/bff/conversation_test.go` | 051 |
| NEW | `internal/bff/dispatch.go` | 052 |
| NEW | `internal/bff/dispatch_test.go` | 052 |

## Dependencies Consumed

- `internal/bff/connection.go` (WU-048): `Connection`, `conn.sessionID`, `conn.capabilities`
- `internal/bff/capabilities.go` (WU-049): `CapabilityManager.ProjectContext()`, `CapabilityManager.Tools()`, `RequestReregistration()`
- `internal/bff/transport.go` (WU-046): `FrameTransport.SendNotification()`
- `internal/protocol/messages.go` (WU-039): `TurnSubmit`, `SessionResume`, `SessionList`, `ToolResult`, method constants
- `internal/protocol/sessions.go` (WU-041): `SessionDetail`, `SessionListResponse`, `SessionSummary`, `TurnSummary`, `SessionSyncResponse`
- `internal/protocol/tools.go` (WU-041): `TurnSubmitResponse`, `ToolResultResponse`
- `internal/provider/provider.go` (WU-042): `Provider.FormatMessages()`, `FormatMessagesOpts`, `Message`, `ToolCall`, `ToolResult`, `Attachment`
- `internal/provider/anthropic.go` (WU-043): Anthropic adapter
- `internal/provider/openai.go` (WU-044): OpenAI adapter
- `internal/storage/store.go` (WU-045): `Store` interface (session CRUD, turn CRUD, lock methods, command history)

## Interfaces Exported (consumed by downstream WUs)

- **WU-053 (Streaming)**: uses `Conversation.AppendAssistantTurn()` after stream completes
- **WU-054/055 (Prompts)**: uses `Conversation.Messages()` for prompt assembly context
- **WU-056 (Cost)**: uses `Conversation.AppendAssistantTurn()` metrics for cost tracking
- **WU-059 (Routing)**: integrates with `DispatchOpts` model/provider selection
- **WU-060 (Multi-model)**: uses `TurnDispatcher.Dispatch()` for parallel provider calls
- **WU-061 (Compaction)**: uses `Conversation.Messages()` for token counting
- **WU-064 (Recovery)**: uses `Conversation.PendingToolCalls()` for `session.sync`. **Note (resolves B-03):** `SessionManager` exports `GetActiveSession(sessionID string) *ActiveSession` so WU-064 can build the sync response from `ActiveSession.Conversation.PendingToolCalls()`, `ActiveSession.ActiveTurn`, and `ActiveSession.BranchManager`. The `session.sync` handler itself lives in WU-064's `recovery.go` but calls into `SessionManager` and `Conversation` exported methods.
- **WU-091 (Command history)**: wired via `store.AppendCommandHistory()` in turn.submit handler
