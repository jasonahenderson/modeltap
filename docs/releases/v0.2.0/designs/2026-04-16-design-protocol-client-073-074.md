# 2026-04-16 — Design: Protocol Client Bundle (WU-073 + WU-074)

## Scope

This bundle covers the harness-side protocol client and connection manager in `internal/harness/`:

- **WU-073** — Protocol client (`client.go`): JSON-RPC client over Unix socket or TLS, request/response correlation, streaming event dispatch, reconnection support.
- **WU-074** — Connection manager (`connection.go`): 9-state lifecycle, local auto-start, heartbeat, exponential backoff, state change notifications to UI.

**Out of scope:** BFF server-side transport (WU-046), server-side connection lifecycle (WU-048), session management (WU-050), tool framework (WU-075), MCP client (WU-081).

## Bundle Rationale

The protocol client (073) provides the wire-level transport that the connection manager (074) orchestrates through lifecycle states. They share a single goroutine model (read loop + heartbeat monitor) and the connection manager directly wraps the client. Designing them together prevents mismatched assumptions about error handling, reconnection, and state transitions.

## Design Decisions

### D1. Package structure

```
internal/harness/
  client.go         — WU-073: ProtocolClient, request/response/event handling
  client_test.go
  connection.go     — WU-074: ConnectionManager, state machine, auto-start, heartbeat
  connection_test.go
```

Note: `internal/harness/connection.go` is the harness-side connection manager, distinct from `internal/bff/connection.go` (server-side connection, WU-048). The harness manages the 9-state lifecycle from the client perspective including auto-start and reconnection; the server manages it from the accept/dispatch perspective.

### D2. Protocol client (WU-073)

#### D2.1. ProtocolClient

```go
// ProtocolClient provides JSON-RPC 2.0 communication with the BFF server.
// It handles request/response correlation, streaming event dispatch, and
// reconnection at the wire level. The ConnectionManager (WU-074) owns
// lifecycle decisions; ProtocolClient just reads and writes.
type ProtocolClient struct {
    transport *protocol.FrameReader
    writer    *protocol.FrameWriter
    conn      net.Conn

    // Request/response correlation
    mu       sync.Mutex
    pending  map[string]chan *protocol.Response // keyed by request ID (string form)
    nextID   int64

    // Event dispatch
    eventHandler EventHandler

    // Read loop lifecycle
    ctx    context.Context
    cancel context.CancelFunc
    done   chan struct{}
}

// EventHandler processes server-initiated notifications (streaming events).
// Implementations must be non-blocking — if processing takes time,
// the handler should dispatch to a channel or goroutine.
type EventHandler interface {
    // HandleEvent is called for every server notification.
    // method is the JSON-RPC method (e.g., "token.delta", "tool.call").
    // params is the raw JSON params payload.
    HandleEvent(method string, params json.RawMessage)
}

// EventHandlerFunc adapts a function to the EventHandler interface.
type EventHandlerFunc func(method string, params json.RawMessage)

func (f EventHandlerFunc) HandleEvent(method string, params json.RawMessage) {
    f(method, params)
}
```

#### D2.2. Connection and lifecycle

```go
// Dial connects to the BFF server via Unix socket or TLS.
func Dial(ctx context.Context, opts DialOptions) (*ProtocolClient, error)

type DialOptions struct {
    // Exactly one of SocketPath or TLSAddress must be set
    SocketPath string
    TLSAddress string
    TLSConfig  *tls.Config

    // Event handler for streaming notifications
    EventHandler EventHandler

    // Timeouts
    DialTimeout time.Duration // default: 5s
}

// Close shuts down the client: stops the read loop and closes the connection.
func (c *ProtocolClient) Close() error

// Done returns a channel that is closed when the client's read loop exits
// (due to error, server disconnect, or Close).
func (c *ProtocolClient) Done() <-chan struct{}

// Err returns the error that caused the read loop to exit, or nil if
// closed cleanly via Close().
func (c *ProtocolClient) Err() error
```

#### D2.3. Request/response exchange

```go
// Call sends a JSON-RPC request and waits for the matching response.
// Returns the Response.Result payload or an error (including JSON-RPC
// error responses, timeouts, and connection failures).
func (c *ProtocolClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error)

// CallInto sends a JSON-RPC request, waits for the response, and
// unmarshals Result into dest. Convenience wrapper around Call.
func (c *ProtocolClient) CallInto(ctx context.Context, method string, params any, dest any) error

// Note (resolves A-04): No public Notify method. FEAT-0008 requires all
// harness→server frames to use Request (with id), not Notification.
// If a test-only notification sender is needed, use a private method.
```

`Call` flow:
1. Generate monotonic ID (`c.nextID++`, serialized as string)
2. Marshal `protocol.Request{JSONRPC: "2.0", ID: idBytes, Method: method, Params: paramsBytes}`
3. Register response channel in `c.pending[idStr]`
4. Write frame via `c.writer.WriteFrame()`
5. Wait on response channel or ctx.Done()
6. On response: if `Response.Error != nil`, return `*RPCError`; else return `Response.Result`
7. On timeout/cancel: remove from pending, return context error

```go
// RPCError wraps a JSON-RPC error response for typed error handling.
type RPCError struct {
    Code    int
    Message string
    Data    json.RawMessage
}

func (e *RPCError) Error() string {
    return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// IsRPCError checks if an error is an RPCError with the given code.
func IsRPCError(err error, code int) bool
```

#### D2.4. Read loop

```go
// readLoop reads frames from the connection and dispatches them.
// Runs in a dedicated goroutine started by Dial.
func (c *ProtocolClient) readLoop()
```

The read loop:
1. Read frame via `c.transport.ReadFrame()`
2. On error (including `ErrFrameTooLarge`, `io.EOF`, `net.ErrClosed`): set `c.err`, close `c.done`, return
3. Decode JSON-RPC envelope
4. If Response (has `id`): look up `c.pending[id]`, send to channel, remove from pending
5. If Notification (no `id`): dispatch to `c.eventHandler.HandleEvent(method, params)`
6. If malformed: log and skip (don't crash the read loop)

#### D2.5. Streaming correlation helpers

The protocol client provides typed helpers for common streaming patterns:

```go
// SubmitTurn sends a turn.submit request and returns the acknowledgment.
// Streaming events arrive via the EventHandler, not through this call.
func (c *ProtocolClient) SubmitTurn(ctx context.Context, submit *protocol.TurnSubmit) (*TurnSubmitAck, error)

// TurnSubmitAck mirrors protocol.TurnSubmitResponse (WU-041).
// Sync is populated when the turn was already submitted (idempotent replay
// returns current state as an implicit session.sync).
type TurnSubmitAck struct {
    TurnID string          `json:"turn_id"`
    Status string          `json:"status"` // accepted | in_flight | complete | error | cancelled
    Sync   json.RawMessage `json:"sync,omitempty"` // SessionSyncResponse on replay
}

// SendToolResult sends a tool.result request.
func (c *ProtocolClient) SendToolResult(ctx context.Context, result *protocol.ToolResult) error

// CancelTurn sends a turn.cancel request.
func (c *ProtocolClient) CancelTurn(ctx context.Context, turnID string) error

// Register sends capabilities.register and returns the server's response.
func (c *ProtocolClient) Register(ctx context.Context, reg *protocol.CapabilitiesRegister) (*RegisterResponse, error)

type RegisterResponse struct {
    NegotiatedVersion string `json:"negotiated_version"`
    ServerVersion     string `json:"server_version"`
    MaxFrameSize      int    `json:"max_frame_size"`
    MaxAttachmentSize int    `json:"max_attachment_size"`
}

// Ping sends a connection.ping request (heartbeat pong).
func (c *ProtocolClient) Ping(ctx context.Context) error

// Health sends a connection.health request.
func (c *ProtocolClient) Health(ctx context.Context) (json.RawMessage, error)

// SessionResume sends session.resume and returns session details.
func (c *ProtocolClient) SessionResume(ctx context.Context, sessionID string, project protocol.ProjectContext) (json.RawMessage, error)

// SessionList sends session.list with filters.
func (c *ProtocolClient) SessionList(ctx context.Context, filter json.RawMessage) (json.RawMessage, error)
```

### D3. Connection manager (WU-074)

#### D3.1. ConnectionManager

```go
// ConnectionManager orchestrates the harness-to-server connection lifecycle.
// It owns the 9-state lifecycle, auto-start, heartbeat, and reconnection.
// The App receives state changes via Bubbletea messages.
type ConnectionManager struct {
    config  ConnectionConfig
    client  *ProtocolClient
    program *tea.Program // for sending Bubbletea messages

    // State machine
    mu    sync.RWMutex
    state ConnState

    // Heartbeat
    heartbeatTicker *time.Ticker
    lastPong        time.Time

    // Reconnection
    reconnectAttempt int
    reconnectCancel  context.CancelFunc

    // Lifecycle
    ctx    context.Context
    cancel context.CancelFunc
}

type ConnectionConfig struct {
    // Connection target
    SocketPath string // default: ~/.local/share/modeltap/server.sock
    TLSAddress string
    TLSConfig  *tls.Config

    // Auto-start (solo profile)
    AutoStart       bool          // default: true for solo
    ServerBinary    string        // path to modeltap binary
    StartTimeout    time.Duration // default: 5s

    // Heartbeat
    HeartbeatInterval time.Duration // default: 15s
    HeartbeatTimeout  time.Duration // default: 30s

    // Reconnection
    ReconnectInitial  time.Duration // default: 1s
    ReconnectMax      time.Duration // default: 30s
    ReconnectMaxRetries int         // default: 10

    // Registration
    Registration *protocol.CapabilitiesRegister
}

func NewConnectionManager(config ConnectionConfig, program *tea.Program) *ConnectionManager
```

#### D3.2. State machine (9 states, harness perspective)

```go
type ConnState int

const (
    StateDiscovering    ConnState = iota // checking if server is reachable
    StateStarting                        // auto-starting local server
    StateConnecting                      // TCP/socket dial in progress
    StateAuthenticating                  // OIDC/auth (TLS connections)
    StateRegistering                     // capabilities.register sent
    StateReady                           // fully operational
    StateDegraded                        // server reachable but degraded
    StateReconnecting                    // connection lost, retrying
    StateFailed                          // terminal (retries exhausted or unrecoverable)
)

func (s ConnState) String() string
```

Valid transitions (harness-side):

```go
var harnessTransitions = map[ConnState][]ConnState{
    StateDiscovering:    {StateStarting, StateConnecting, StateFailed},
    StateStarting:       {StateConnecting, StateFailed},
    StateConnecting:     {StateAuthenticating, StateRegistering, StateFailed, StateReconnecting},
    StateAuthenticating: {StateRegistering, StateFailed, StateReconnecting},
    StateRegistering:    {StateReady, StateFailed, StateReconnecting},
    StateReady:          {StateDegraded, StateReconnecting, StateFailed},
    StateDegraded:       {StateReady, StateReconnecting, StateFailed},
    StateReconnecting:   {StateConnecting, StateFailed},
}
```

#### D3.3. Lifecycle flow

```go
// Connect initiates the connection lifecycle. Non-blocking; runs in
// a goroutine and sends ConnStateMsg to the Bubbletea program.
func (cm *ConnectionManager) Connect() tea.Cmd

// Disconnect gracefully disconnects. Releases locks via protocol.
func (cm *ConnectionManager) Disconnect()

// Reconnect forces a reconnection attempt (e.g., from /reconnect command).
func (cm *ConnectionManager) Reconnect() tea.Cmd

// Client returns the current ProtocolClient, or nil if not connected.
func (cm *ConnectionManager) Client() *ProtocolClient

// State returns the current connection state.
func (cm *ConnectionManager) State() ConnState
```

`Connect()` flow:
1. **Discovering**: check if socket exists and is connectable
   - Socket exists + dial succeeds → `Connecting` (server is running)
   - Socket exists + dial fails → check if stale
   - Socket missing → `Starting` (if AutoStart) or `Failed`
2. **Starting**: launch `modeltap serve` as subprocess
   - Wait up to `StartTimeout` for socket to become connectable
   - Poll every 200ms
   - On success → `Connecting`
   - On timeout → `Failed` with diagnostic banner
3. **Connecting**: `Dial()` to server
   - On success → `Authenticating` (TLS) or `Registering` (socket)
   - On failure → `Reconnecting` (if retriable) or `Failed` (if terminal)
4. **Authenticating**: (TLS only, placeholder for OIDC)
   - On success → `Registering`
5. **Registering**: send `capabilities.register`
   - On success → `Ready`
   - On version mismatch → `Failed` (terminal, `MT-CONN-004`)
   - On other error → `Reconnecting`
6. **Ready**: start heartbeat, notify UI
7. **Degraded**: enter when server reports degraded health; stay operational
8. **Reconnecting**: exponential backoff retry loop
9. **Failed**: terminal; user must `/reconnect` or restart

Each transition sends `ConnStateMsg` to the Bubbletea program:

```go
func (cm *ConnectionManager) transition(to ConnState, detail string) {
    cm.mu.Lock()
    cm.state = to
    cm.mu.Unlock()
    
    if cm.program != nil {
        cm.program.Send(ConnStateMsg{
            Info: ConnStateInfo{
                State:      to.String(),
                Detail:     detail,
                Attempt:    cm.reconnectAttempt,
                MaxRetries: cm.config.ReconnectMaxRetries,
            },
        })
    }
}
```

#### D3.4. Auto-start (solo profile)

```go
// autoStartServer launches `modeltap serve` as a background subprocess
// and waits for the socket to become connectable.
func (cm *ConnectionManager) autoStartServer(ctx context.Context) error
```

Implementation:
1. Check if socket path already exists
   - If yes, attempt `net.Dial`. If succeeds → server already running, return nil
   - If dial fails → check if stale. If stale, remove socket file
2. Launch `exec.Command(cm.config.ServerBinary, "serve")` with:
   - Stdout/stderr redirected to a log file (e.g., `~/.local/share/modeltap/server.log`)
   - `SysProcAttr.Setpgid = true` so the server survives harness exit
3. Poll for socket availability every 200ms up to `StartTimeout`
4. On success: return nil (caller proceeds to `Connecting`)
5. On timeout: kill process, return error

Stale socket detection: same as server-side (WU-047) — attempt dial, connection refused = stale.

#### D3.5. Heartbeat

```go
// startHeartbeat begins the heartbeat loop. Sends connection.ping
// every HeartbeatInterval, monitors for pong timeout.
func (cm *ConnectionManager) startHeartbeat()

// stopHeartbeat stops the heartbeat loop.
func (cm *ConnectionManager) stopHeartbeat()
```

Heartbeat loop (resolves B-02 — two-stage degradation per FEAT-0008 §"Heartbeat"):
1. Ticker fires every `HeartbeatInterval` (15s)
2. Send `connection.ping` via `cm.client.Ping(ctx)`
3. On success: update `cm.lastPong`, reset `cm.missedPongs` to 0
4. On error (timeout or connection issue): increment `cm.missedPongs`
5. Check `missedPongs` threshold:
   - `missedPongs >= 3`: `transition(StateDegraded, "missed 3 heartbeats")`
   - `missedPongs >= 5`: `transition(StateReconnecting, "missed 5 heartbeats")`, stop heartbeat, start reconnection

```go
// missedPongs tracks consecutive heartbeat failures.
// Reset to 0 on successful pong. Per FEAT-0008:
//   3 missed → StateDegraded (user sees [◐])
//   5 missed → StateReconnecting (user sees [↻])
missedPongs int
```

Note: the BFF server-side design (Bundle 4) was amended to use passive heartbeat monitoring (server does NOT initiate pings). The harness is the sole ping initiator, matching FEAT-0008.

**Connection closure detection (resolves timing mismatch):** The harness has two disconnection detection paths:
1. **Read loop EOF** — when the server closes the connection (timeout, crash, shutdown), the ProtocolClient's read loop receives io.EOF and closes `Done()`. The ConnectionManager watches `Done()` and immediately transitions to `StateReconnecting` — no pong-counting delay.
2. **Missed pongs** — the slower path (3 missed = 45s → degraded, 5 missed = 75s → reconnecting) handles the case where the connection stays open but the server stops responding to pings (degraded upstream).

In practice, server-side connection timeout (30s heartbeat timeout) closes the TCP connection, triggering path 1 within milliseconds. The session lock grace period (10s after close = 40s total) is sufficient because the harness detects closure via EOF and starts reconnecting immediately — well before the lock expires.

#### D3.6. Exponential backoff reconnection

```go
// reconnectLoop attempts to reconnect with exponential backoff.
func (cm *ConnectionManager) reconnectLoop(ctx context.Context)
```

Backoff parameters:
- Initial delay: `ReconnectInitial` (1s)
- Maximum delay: `ReconnectMax` (30s)
- Factor: 2x
- Jitter: ±20%
- Max attempts: `ReconnectMaxRetries` (10)

```go
func backoffDelay(attempt int, initial, max time.Duration) time.Duration {
    delay := initial * time.Duration(1<<uint(attempt))
    if delay > max {
        delay = max
    }
    // ±20% jitter
    jitter := time.Duration(rand.Int63n(int64(delay) * 2 / 5)) - time.Duration(int64(delay)/5)
    return delay + jitter
}
```

On each attempt (resolves A-05 — multi-step transitions, not atomic):
1. `transition(StateReconnecting, fmt.Sprintf("attempt %d/%d", attempt, max))`
2. Wait `backoffDelay(attempt, ...)`
3. `transition(StateConnecting, "dialing")` → `Dial()`
4. On dial success: `transition(StateRegistering, "registering")` → `Register()`
5. On register success: `transition(StateReady, ...)`, reset attempt counter and `missedPongs`, start heartbeat
6. On failure at any step: increment attempt, loop back to step 1
7. On max attempts exhausted: `transition(StateFailed, "reconnection exhausted")`

The reconnect goes through `Reconnecting → Connecting → Registering → Ready` (three state transitions), not a single atomic step.

#### D3.7. Event bridge to Bubbletea

The ConnectionManager implements `EventHandler` and translates server notifications into Bubbletea messages:

```go
func (cm *ConnectionManager) HandleEvent(method string, params json.RawMessage) {
    switch method {
    case "token.delta":
        var delta TokenDelta
        json.Unmarshal(params, &delta)
        cm.program.Send(StreamTokenMsg{
            TurnID:   delta.TurnID,
            BranchID: delta.BranchID,
            Delta:    delta.Content,
        })
    case "turn.complete":
        var complete TurnComplete
        json.Unmarshal(params, &complete)
        cm.program.Send(StreamCompleteMsg{...})
    case "tool.call":
        var call ToolCallEvent
        json.Unmarshal(params, &call)
        cm.program.Send(ToolCallMsg{...})
    case "cost.update":
        var cost CostUpdate
        json.Unmarshal(params, &cost)
        cm.program.Send(CostUpdateMsg{Total: cost.SessionTotal})
    case "compact.suggest":
        cm.program.Send(BannerMsg{Text: "Context pressure — consider /compact", Duration: 0})
    case "status.update":
        // provider health changes → may trigger degraded state
    // ... other event types
    }
}
```

## Test Strategy

### WU-073 tests (`client_test.go`)

Tests use a mock server: a goroutine that accepts a `net.Pipe()` connection and sends scripted responses/notifications.

```go
// mockServer provides a test server that reads requests and sends
// scripted responses and notifications.
type mockServer struct {
    transport *protocol.FrameReader
    writer    *protocol.FrameWriter
    conn      net.Conn
}

func newMockServer() (client net.Conn, server *mockServer)
```

| Test | Description |
|------|-------------|
| `TestClient_Dial_Socket` | Dial Unix socket, verify connected |
| `TestClient_Call_Success` | Send request, receive response with matching ID |
| `TestClient_Call_Error` | Send request, receive JSON-RPC error → RPCError |
| `TestClient_Call_Timeout` | Context timeout → context.DeadlineExceeded |
| `TestClient_Call_Concurrent` | Multiple concurrent calls, responses correlated correctly |
| `TestClient_Notify` | Send notification, no response expected |
| `TestClient_EventDispatch` | Server sends notification → EventHandler called |
| `TestClient_EventDispatch_MultipleTypes` | Different event methods dispatched correctly |
| `TestClient_ReadLoop_EOF` | Server closes → Done() channel closed, Err() = io.EOF |
| `TestClient_ReadLoop_MalformedFrame` | Bad JSON → logged and skipped, read loop continues |
| `TestClient_ReadLoop_OversizeFrame` | Oversize frame → connection closed |
| `TestClient_Close` | Close() stops read loop, closes connection |
| `TestClient_SubmitTurn` | SubmitTurn helper sends correct method and params |
| `TestClient_SendToolResult` | SendToolResult sends correct method |
| `TestClient_Register` | Register sends capabilities.register, parses response |
| `TestClient_Ping` | Ping sends connection.ping |

### WU-074 tests (`connection_test.go`)

| Test | Description |
|------|-------------|
| `TestConnMgr_StateTransitions` | All valid transitions succeed |
| `TestConnMgr_InvalidTransition` | Invalid transitions rejected |
| `TestConnMgr_Connect_SocketExists` | Existing socket → skip auto-start, connect |
| `TestConnMgr_Connect_AutoStart` | No socket → start server, wait for socket, connect |
| `TestConnMgr_Connect_AutoStart_Timeout` | Server doesn't start in time → Failed |
| `TestConnMgr_Connect_StaleSocket` | Stale socket → remove, auto-start |
| `TestConnMgr_Connect_Register` | After connect → register → Ready |
| `TestConnMgr_Connect_VersionMismatch` | Register version mismatch → Failed (terminal) |
| `TestConnMgr_Heartbeat_Success` | Pings sent at interval, pongs tracked |
| `TestConnMgr_Heartbeat_3Missed_Degraded` | 3 missed pongs → StateDegraded |
| `TestConnMgr_Heartbeat_5Missed_Reconnecting` | 5 missed pongs → StateReconnecting |
| `TestConnMgr_Heartbeat_Reset` | Successful pong resets missedPongs counter |
| `TestConnMgr_Reconnect_Success` | Lost connection → reconnect with backoff → Ready |
| `TestConnMgr_Reconnect_Exhausted` | Max retries → Failed |
| `TestConnMgr_Reconnect_Backoff` | Delays increase exponentially with jitter |
| `TestConnMgr_Reconnect_Manual` | /reconnect → force reconnect from any state |
| `TestConnMgr_Disconnect_Graceful` | Disconnect releases resources |
| `TestConnMgr_EventBridge_TokenDelta` | token.delta → StreamTokenMsg |
| `TestConnMgr_EventBridge_TurnComplete` | turn.complete → StreamCompleteMsg |
| `TestConnMgr_EventBridge_ToolCall` | tool.call → ToolCallMsg |
| `TestConnMgr_EventBridge_CostUpdate` | cost.update → CostUpdateMsg |
| `TestConnMgr_UINotification` | State changes → ConnStateMsg sent to program |

## Key Files

| Action | Path | WU |
|--------|------|----|
| NEW | `internal/harness/client.go` | 073 |
| NEW | `internal/harness/client_test.go` | 073 |
| NEW | `internal/harness/connection.go` | 074 |
| NEW | `internal/harness/connection_test.go` | 074 |

## Dependencies Consumed

- `internal/protocol/protocol.go` (WU-039): `FrameReader`, `FrameWriter`, `Request`, `Response`, `Notification`, `MaxFrameSize`, `ErrFrameTooLarge`
- `internal/protocol/messages.go` (WU-039): `TurnSubmit`, `ToolResult`, `CapabilitiesRegister`, `ProjectContext`, method constants
- `internal/protocol/events.go` (WU-040): streaming event payload types (`TokenDelta`, `ToolCallEvent`, `TurnComplete`, `CostUpdate`, etc.)
- `internal/protocol/sessions.go` (WU-041): `SessionSyncResponse`
- `internal/harness/messages.go` (WU-068): Bubbletea Msg types (`StreamTokenMsg`, `ConnStateMsg`, etc.)

## Interfaces Exported (consumed by downstream WUs)

- **WU-075 (Tool framework)**: uses `ProtocolClient.SendToolResult()` to return tool execution results
- **WU-080 (Modes)**: uses `ProtocolClient.SubmitTurn()` with mode field
- **WU-081 (MCP)**: uses `ProtocolClient.Call()` for `capabilities.update`
- **WU-082 (File context)**: uses `ProtocolClient.SubmitTurn()` with attachments
- **WU-083 (Paste)**: uses `ProtocolClient.Call()` for `content.transform`
- **WU-084 (Session explorer)**: uses `ProtocolClient.SessionList()`, `ProtocolClient.SessionResume()`
- **WU-085 (Models)**: uses `ProtocolClient.Call()` for `model.list`, `model.switch`
- **WU-086 (Connection UX)**: uses `ConnectionManager.State()`, `ConnectionManager.Reconnect()`
- **WU-092 (Command history)**: uses `ProtocolClient.Call()` for `history.list`
