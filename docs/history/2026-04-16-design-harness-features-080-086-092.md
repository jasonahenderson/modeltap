# 2026-04-16 — Design: Harness Features Bundle (WU-080 through WU-086 + WU-092)

## Scope

This bundle covers the harness feature layer in `internal/harness/`:

- **WU-080** — Plan/build/auto modes with Ctrl+P toggle (`modes.go`)
- **WU-081** — MCP client: stdio transport and tool discovery (`mcp.go`)
- **WU-082** — File context management: @file, drag-drop, /context, /drop (`context.go`)
- **WU-083** — Large paste handling: content.transform (`paste.go`)
- **WU-084** — Session explorer and session commands (`explorer.go`)
- **WU-085** — Model commands and multi-model branch display (`models.go`)
- **WU-086** — Connection UX: states, banners, diagnostics (`connux.go`)
- **WU-092** — Command history: BFF-sourced traversal (`history.go`)

**Out of scope:** BFF server logic (Track A), tool implementations (WU-075–079), harness integration tests (WU-087).

## Bundle Rationale

These 8 WUs all build on the scaffold (Bundle 5), protocol client (Bundle 6), and tool framework (Bundle 7). They are the user-facing features of the harness — each adds a command, UI component, or interaction pattern. They share the `internal/harness/` package and interact through `AppState` and Bubbletea messages. Designing them together prevents conflicting message types and command name collisions.

## Design Decisions

### D1. Plan/build/auto modes (WU-080)

```go
// internal/harness/modes.go

// ModeManager handles mode switching and plan mode interception.
type ModeManager struct {
    state *AppState
    conn  *ConnectionManager
}

// HandleModeToggle processes Ctrl+P: plan ↔ build. From auto → build.
func (mm *ModeManager) HandleModeToggle() tea.Cmd

// HandleModeCommand processes /plan, /build, /auto commands.
func (mm *ModeManager) HandleModeCommand(mode protocol.Mode) tea.Cmd
```

Plan mode interception: when `AppState.Mode == protocol.ModePlan`, tool calls with `RiskLevel` of `write`, `execute`, or `destructive` are intercepted and accumulated in a plan display instead of executed.

```go
// PlanAccumulator collects intercepted actions in plan mode.
type PlanAccumulator struct {
    Steps []PlanStep
}

type PlanStep struct {
    ToolName string
    Input    json.RawMessage
    Summary  string // human-readable description
}

// PlanUI renders the accumulated plan with action buttons.
// Returns [a]pprove, [e]dit, [s]tep through, [c]ancel.
type PlanUI struct {
    plan   *PlanAccumulator
    state  *AppState
}
```

Approve → switch to build mode, submit plan as context. Step-through → execute one step at a time with per-step pause.

### D2. MCP client (WU-081)

```go
// internal/harness/mcp.go

// MCPManager manages MCP server connections and tool discovery.
type MCPManager struct {
    servers map[string]*MCPServer
    registry *tools.Registry
    conn     *ConnectionManager
    config   []MCPServerConfig
}

type MCPServerConfig struct {
    Name    string            `mapstructure:"name"`
    Command string            `mapstructure:"command"`
    Args    []string          `mapstructure:"args"`
    Env     map[string]string `mapstructure:"env"`
    Timeout time.Duration     `mapstructure:"timeout"` // default: 5s
}

type MCPServer struct {
    Name    string
    Status  string // "connected", "retrying", "failed"
    Process *exec.Cmd
    Tools   []protocol.ToolDefinition
}
```

Startup behavior (non-blocking per WU-081 track spec):
- Each MCP server launched asynchronously with configurable timeout
- Success → tools added to registry, `capabilities.update` sent to BFF
- Failure → transient banner, background retry with exponential backoff
- `/mcp status` shows per-server state; `/mcp reconnect <name>` forces retry

### D3. File context management (WU-082)

```go
// internal/harness/context.go

// ContextManager handles @file loading, glob expansion, and drag-drop.
type ContextManager struct {
    readTool  tools.Tool
    state     *AppState
}

// ProcessAttachments resolves @file references into protocol.Attachment structs.
// Evolves SubmitMsg.Attachments from []string (WU-068) to []protocol.Attachment.
func (cm *ContextManager) ProcessAttachments(refs []string) ([]protocol.Attachment, error)

// HandleContextCommand processes /context (list) and /drop <file> (remove).
func (cm *ContextManager) HandleContextCommand(cmd, args string) tea.Cmd
```

@file processing:
1. `@path/to/file` → single file read via ReadTool
2. `@src/**/*.go` → glob expansion via GlobTool, then read each
3. Drag-drop (detected by InputArea) → treat each path as @file

Each attachment populated with: `Path`, `Raw` (base64), `Content` (extracted text), `ContentType` (MIME), `Transform` (extraction method).

### D4. Large paste handling (WU-083)

```go
// internal/harness/paste.go

// PasteHandler processes PasteDetectedMsg from the input area.
type PasteHandler struct {
    conn *ConnectionManager
}

// HandlePaste shows preview and offers choices.
func (ph *PasteHandler) HandlePaste(msg PasteDetectedMsg) tea.Cmd
```

On `PasteDetectedMsg` (content > 2KB threshold):
1. Show preview (first 5 lines + size info)
2. Offer choices: `[s]ummarize` | `[f]ull` | `[t]runcate` | `[c]ancel`
3. Summarize → `content.transform` via BFF (WU-062)
4. Full → include as-is
5. Truncate → first 2KB
6. Cancel → discard

### D5. Session explorer (WU-084)

```go
// internal/harness/explorer.go

// SessionExplorer is a Bubbletea component for session browsing.
type SessionExplorer struct {
    sessions []protocol.SessionSummary
    selected int
    detail   *protocol.SessionDetail
    conn     *ConnectionManager
    state    *AppState
}
```

Shown on harness launch (unless auto-resume applies). Features:
- List view with summary/context/cost per session
- TUI navigation (arrows, enter to select)
- Detail view on enter (calls `session.details`)
- Actions: resume, fork, compact-before-resume
- Auto-resume: if single session + no server events since last connection → auto-resume without showing explorer

Commands: `/sessions` re-shows explorer. `/compact` shows interactive category UI. `/cost` shows cost breakdown. `/trace` shows routing for last turn. `/clear` clears context. `/fork` branches session. `/help` shows command list.

### D6. Model commands and branch display (WU-085)

```go
// internal/harness/models.go

// ModelUI handles /model, /models commands and multi-model rendering.
type ModelUI struct {
    conn  *ConnectionManager
    state *AppState
}

// HandleModelCommand processes /model <name>, /model auto, /models.
func (mu *ModelUI) HandleModelCommand(cmd, args string) tea.Cmd

// RenderBranches renders progressive multi-model completion.
func (mu *ModelUI) RenderBranches(branches []BranchDisplay) string
```

Multi-model rendering: each branch gets a labeled section with spinner while streaming, progressive text as tokens arrive, per-branch cost/timing on completion. Branch sections use the model routing indicator format.

### D7. Connection UX (WU-086)

```go
// internal/harness/connux.go

// ConnectionUX translates connection state changes into visual feedback.
type ConnectionUX struct {
    state *AppState
}

// HandleConnState processes ConnStateMsg and returns banner/status updates.
func (cux *ConnectionUX) HandleConnState(msg ConnStateMsg) tea.Cmd
```

State-specific rendering:
- `discovering`/`starting` → banner: "Starting local server..."
- `authenticating` → banner: "Authenticating (OIDC)..."
- `registering` → banner: "Registering tools (N built-in + M MCP)..."
- `ready` → clear banner, green indicator
- `degraded` → persistent banner with reason, yellow indicator
- `reconnecting` → banner: "Connection lost. Reconnecting (attempt N/M, next in Ns)..."
- `failed` → persistent banner with diagnostic code, cause, and suggested command

Commands: `/status` shows connection health. `/reconnect` forces reconnection. `/session unlock` force-releases stuck lock.

### D8. Command history traversal (WU-092)

```go
// internal/harness/history.go

// HistoryController implements HistorySource (from WU-068/070) using
// BFF-sourced command history.
type HistoryController struct {
    conn   *ConnectionManager
    cache  []string // cached entries (newest first)
    cursor string   // pagination cursor
    scope  string   // "user", "project", "session"
    total  int      // total cached entries
}

func NewHistoryController(conn *ConnectionManager) *HistoryController

// Implements HistorySource interface from WU-070
func (hc *HistoryController) Entry(index int) (string, bool)
func (hc *HistoryController) Len() int

// Load fetches initial history from BFF on harness launch.
func (hc *HistoryController) Load(ctx context.Context) error

// SetScope changes the history scope and refreshes.
func (hc *HistoryController) SetScope(scope string) tea.Cmd
```

Fetches history on launch via `history.list` (default: user scope, limit 200). Caches locally; pages older entries on demand as user arrows past cache. Degrades gracefully if BFF unavailable (empty history).

Commands: `/history project`, `/history session`, `/history user` change scope.

## Test Strategy

Tests for each WU follow the table-driven pattern. Key tests per WU:

| WU | Key Tests |
|----|-----------|
| 080 | Mode toggle plan↔build, Ctrl+P from auto→build, plan accumulation, approve/step/edit/cancel, read-only tools pass through in plan mode |
| 081 | MCP connect/disconnect, tool discovery, capabilities.update, startup failure non-blocking, `/mcp status`, retry with backoff |
| 082 | @file load, @glob expansion, drag-drop detection, /context list, /drop remove, attachment formatting with raw+content+content_type |
| 083 | Paste detection >2KB, preview rendering, summarize via BFF, full/truncate/cancel |
| 084 | Session list rendering, detail view, resume/fork/compact, auto-resume single session, /sessions, /compact interactive UI, /cost, /clear, /fork |
| 085 | /model set, /model auto (clear), /models list, multi-model branch rendering, progressive completion, per-branch spinners |
| 086 | All 9 connection states render correctly, banners appear/disappear on transitions, diagnostics render with code+cause+suggestion, /status, /reconnect |
| 092 | Up/down arrow traversal, scope switching, paging older entries, graceful degradation when BFF unavailable, draft preservation |

## Key Files

| Action | Path | WU |
|--------|------|----|
| NEW | `internal/harness/modes.go` | 080 |
| NEW | `internal/harness/mcp.go` | 081 |
| NEW | `internal/harness/context.go` | 082 |
| NEW | `internal/harness/paste.go` | 083 |
| NEW | `internal/harness/explorer.go` | 084 |
| NEW | `internal/harness/models.go` | 085 |
| NEW | `internal/harness/connux.go` | 086 |
| NEW | `internal/harness/history.go` | 092 |
| NEW | `internal/harness/*_test.go` | all |
