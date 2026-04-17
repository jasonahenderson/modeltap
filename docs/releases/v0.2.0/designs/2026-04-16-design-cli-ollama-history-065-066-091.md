# 2026-04-16 — Design: CLI, Ollama Provider, Command History Bundle (WU-065 + WU-066 + WU-091)

## Scope

- **WU-065** — CLI commands (`internal/cli/`): `modeltap serve`, `server status`, `server sessions`, `server session <id>`, `session unlock`.
- **WU-066** — Ollama provider adapter (`internal/provider/ollama.go`): full `Provider` interface including `FormatMessages`, `FormatToolDefinitions`, `ParseStreamEvent`, NDJSON streaming, tool use, `Detect`.
- **WU-091** — Command history storage and protocol (`internal/bff/history.go`, `internal/storage/`): `command_history` table, `history.append`, `history.list` handlers, automatic capture on `turn.submit`.

**Out of scope:** Harness-side history traversal (WU-092), harness launch integration (WU-089).

## Bundle Rationale

These three WUs are independent in scope but share a dependency layer (all depend on BFF foundation and sessions). Bundling them keeps the design count manageable while respecting that they don't deeply interleave. Each WU has its own section.

## Design Decisions

### D1. CLI commands (WU-065)

#### D1.1. New Cobra commands

```go
// internal/cli/serve.go — enhanced modeltap serve
// internal/cli/server.go — new server status/sessions commands
// internal/cli/session.go — new session unlock command
```

Command tree:
```
modeltap serve              — starts proxy + BFF server
modeltap server status      — query BFF health
modeltap server sessions    — list active/recent sessions
modeltap server session <id> — show session details
modeltap session unlock <id> [--force] — force-release session lock
```

#### D1.2. `modeltap serve` enhancements

Already partially designed in Bundle 4 (WU-047 D3.4). WU-065 adds:
- CLI flag `--socket <path>` overriding config socket path
- CLI flag `--tls-address <addr>` for TLS listener
- CLI flag `--no-bff` to start proxy only (backwards compatibility)

#### D1.3. `server status`

Connects to BFF via local socket, sends `connection.health` request, renders result:

```
Server Status
  Version:  0.2.0
  Protocol: 1
  Uptime:   2h 15m

Dependencies
  Storage:      ready (/path/to/data.db)
  Auth:         ready
  Capabilities: ready (14 tools)
  Routing:      ready

Providers
  anthropic-prod: ready
  ollama-local:   ready (3 models)
  openai-prod:    unavailable (timeout)

Active Session
  ID:    sess_a8f3c2
  Owner: alice
```

#### D1.4. `server sessions`

Connects to BFF, sends `session.list` request, renders table:

```
ID          Project              Status   Context   Cost    Turns  Model
sess_a8f3c2 /Users/alice/myproj  active   47%       $0.42   12     claude-opus-4-6
sess_b1d4e7 /Users/alice/other   active   23%       $0.15   5      claude-sonnet-4-6
sess_c3f5g8 /Users/bob/work      completed 0%       $1.23   45     gpt-5
```

#### D1.5. `server session <id>`

Sends `session.details` request, renders detailed view:

```
Session sess_a8f3c2
  Project: /Users/alice/myproj
  Model:   claude-opus-4-6 [override]
  Status:  active
  Context: 47% (38K/80K)
  Cost:    $0.42 (12 turns)
  Created: 2026-04-16 14:30:00

Timeline (last 5 turns)
  #10 user    "Can you add a test for..."    $0.03  1.2s
  #11 assistant [claude-opus-4-6]              $0.08  4.1s
  #12 user    "Now fix the import..."         $0.02  0.8s

Files Touched: 5 | Modified: 3
Server Events: 1 (auto_compact at 14:45)
```

#### D1.6. `session unlock <id>`

```go
func handleSessionUnlock(id string, force bool) error
```

Connects to BFF, calls internal admin handler `ForceReleaseSessionLock(id)`. If the session has an actively-streaming turn and `--force` is not passed, prints warning and exits with error.

### D2. Ollama provider adapter (WU-066)

#### D2.1. OllamaProvider

```go
// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
    host   string
    client *http.Client
}

func NewOllamaProvider(host string) *OllamaProvider

// Detect returns true if the host matches Ollama patterns.
func (p *OllamaProvider) Detect(host string) bool {
    // Matches localhost:11434 or any host with /api/tags endpoint
    return strings.Contains(host, ":11434") || p.probeOllamaAPI(host)
}
```

#### D2.2. FormatMessages

Ollama uses the `/api/chat` endpoint with its own message format:

```go
func (p *OllamaProvider) FormatMessages(opts provider.FormatMessagesOpts) ([]byte, error)
```

Ollama wire format:
```json
{
  "model": "llama-3.1-8b",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ],
  "stream": true,
  "tools": [...],
  "options": {
    "temperature": 0.7,
    "num_predict": 4096
  }
}
```

Translation rules:
- `SystemPrompt` → first message with `role: "system"`
- User messages → `{"role": "user", "content": "..."}`
- Assistant messages → `{"role": "assistant", "content": "..."}`
- Tool calls → `{"role": "assistant", "content": "", "tool_calls": [{"function": {"name": "...", "arguments": {...}}}]}`
- Tool results → `{"role": "tool", "content": "..."}`
- `MaxTokens` → `options.num_predict`
- `Temperature` → `options.temperature`
- Images: `{"role": "user", "images": ["base64data..."]}` (Ollama uses `images` array, not content blocks)

#### D2.3. FormatToolDefinitions

```go
func (p *OllamaProvider) FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error)
```

Ollama tool format:
```json
{
  "type": "function",
  "function": {
    "name": "Read",
    "description": "Read a file",
    "parameters": { /* JSON Schema */ }
  }
}
```

#### D2.4. ParseStreamEvent

Ollama streams NDJSON (not SSE). Each line is a complete JSON object:

```json
{"model":"llama-3.1-8b","created_at":"...","message":{"role":"assistant","content":"Hello"},"done":false}
```

```go
func (p *OllamaProvider) ParseStreamEvent(data []byte) (*provider.StreamEvent, error)
```

- `done: false` with `message.content` → `StreamEvent{Type: "text", Content: content}`
- `done: true` → `StreamEvent{Type: "done", Usage: &Usage{...}}`
- `message.tool_calls` → `StreamEvent{Type: "tool_call_start", ToolCall: ...}`

#### D2.5. Context window truncation

Uses the same truncation logic from `provider.FormatMessagesOpts.WindowSize` (WU-042 design). Ollama models have smaller context windows, so truncation is more likely to trigger.

### D3. Command history (WU-091)

#### D3.1. History handlers

```go
// internal/bff/history.go

// handleHistoryAppend handles history.append requests.
// Optional — the server automatically appends on turn.submit.
// This handler allows the harness to record unsent drafts.
func handleHistoryAppend(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)

// handleHistoryList handles history.list requests with scoping.
func handleHistoryList(ctx context.Context, conn *Connection, params json.RawMessage) (any, error)
```

#### D3.2. Automatic capture on turn.submit

Already wired in Bundle 8 (WU-052 D4.5 step 5). Every `turn.submit` appends to `command_history` via `store.AppendCommandHistory()`. Idempotent by `turn_id` — if the turn was already recorded (duplicate submit), the append is skipped.

#### D3.3. history.list scoping

Params:
```go
type HistoryList struct {
    Scope  string `json:"scope"`  // "user", "project", "session"
    Limit  int    `json:"limit"`  // default: 50
    Before string `json:"before"` // cursor (ISO8601 timestamp, opaque to harness)
}
```

Scoping rules (from storage design WU-045):
- `"user"`: `WHERE user_id = ?` — all commands by this user across all projects/sessions
- `"project"`: `WHERE user_id = ? AND project = ?` — commands in the current project
- `"session"`: `WHERE session_id = ? AND user_id = ?` — commands in the current session

`user_id` is always enforced (no cross-user history access).

Response:
```go
type HistoryListResponse struct {
    Entries    []HistoryEntry `json:"entries"`
    NextCursor string         `json:"next_cursor,omitempty"` // empty if no more pages
}

type HistoryEntry struct {
    Content   string `json:"content"`
    Project   string `json:"project"`
    SessionID string `json:"session_id,omitempty"`
    CreatedAt string `json:"created_at"`
}
```

## Test Strategy

### WU-065 tests

| Test | Description |
|------|-------------|
| `TestCLI_Serve_StartsBFF` | `serve` starts both proxy and BFF |
| `TestCLI_Serve_NoBFF` | `--no-bff` starts proxy only |
| `TestCLI_ServerStatus` | `server status` renders health |
| `TestCLI_ServerSessions` | `server sessions` renders table |
| `TestCLI_ServerSession_Details` | `server session <id>` renders details |
| `TestCLI_SessionUnlock` | `session unlock` releases lock |
| `TestCLI_SessionUnlock_ActiveStream` | Active stream without --force → error |
| `TestCLI_SessionUnlock_Force` | `--force` overrides active stream check |

### WU-066 tests

| Test | Description |
|------|-------------|
| `TestOllama_FormatMessages` | Canonical → Ollama format |
| `TestOllama_FormatMessages_SystemPrompt` | System prompt as first message |
| `TestOllama_FormatMessages_ToolCalls` | Tool calls formatted correctly |
| `TestOllama_FormatMessages_Images` | Images in `images` array |
| `TestOllama_FormatToolDefinitions` | Tools formatted as functions |
| `TestOllama_ParseStreamEvent_Text` | NDJSON text chunk parsed |
| `TestOllama_ParseStreamEvent_Done` | done:true returns usage |
| `TestOllama_ParseStreamEvent_ToolCall` | Tool call event parsed |
| `TestOllama_Detect` | Ollama host pattern detected |
| `TestOllama_Truncation` | Context window truncation works |

### WU-091 tests

| Test | Description |
|------|-------------|
| `TestHistory_AutoAppend` | turn.submit appends to history |
| `TestHistory_AutoAppend_Idempotent` | Duplicate turn → no duplicate entry |
| `TestHistory_List_UserScope` | User scope returns all user's commands |
| `TestHistory_List_ProjectScope` | Project scope filters by project |
| `TestHistory_List_SessionScope` | Session scope filters by session |
| `TestHistory_List_Pagination` | Before cursor paginates correctly |
| `TestHistory_List_UserIsolation` | Cannot see other users' history |
| `TestHistory_Append_Draft` | Manual append records unsent draft |

## Key Files

| Action | Path | WU |
|--------|------|----|
| MODIFY | `internal/cli/serve.go` | 065 |
| NEW | `internal/cli/server.go` | 065 |
| NEW | `internal/cli/session.go` | 065 |
| NEW | `internal/provider/ollama.go` | 066 |
| NEW | `internal/provider/ollama_test.go` | 066 |
| NEW | `internal/bff/history.go` | 091 |
| NEW | `internal/bff/history_test.go` | 091 |

## Dependencies Consumed

- `internal/bff/server.go` (WU-047): `Server.Start()`, `Server.Shutdown()`
- `internal/bff/session.go` (WU-050): session lock force-release
- `internal/provider/provider.go` (WU-042): `Provider` interface, `FormatMessagesOpts`
- `internal/protocol/messages.go` (WU-039): method constants, `ToolDefinition`
- `internal/storage/store.go` (WU-045): `Store.AppendCommandHistory()`, `Store.ListCommandHistory()`
