# 2026-04-16 — Design: Track Integration Tests Bundle (WU-067 + WU-087)

## Scope

- **WU-067** — BFF server integration tests (`internal/bff/integration_test.go`): E2E with real BFF + in-memory storage + mock provider.
- **WU-087** — Harness integration tests (`internal/harness/integration_test.go`): E2E with mock BFF server + real harness components.

**Out of scope:** Cross-stack integration (WU-088 — harness → BFF → provider), security review (WU-094), performance benchmarks (WU-095).

## Bundle Rationale

Both WUs are integration test suites at the track level. They share test infrastructure patterns (mock servers, test helpers) and together cover the full protocol surface from both sides. Designing them together ensures the mock BFF used by Track B tests is protocol-compatible with the real BFF tested in Track A.

## Design Decisions

### D1. BFF server integration tests (WU-067)

#### D1.1. Test infrastructure

```go
// internal/bff/integration_test.go

// testBFF creates a BFF server with in-memory storage and mock providers.
func testBFF(t *testing.T) (*Server, *testClient, func()) {
    store := storage.NewSQLiteStore(":memory:")
    server := NewServer(store, ServerConfig{SocketPath: tempSocket(t)})
    server.Start()
    
    client := newTestClient(t, server.SocketPath())
    cleanup := func() { server.Shutdown(context.Background()); os.Remove(server.SocketPath()) }
    return server, client, cleanup
}

// testClient wraps ProtocolClient for test convenience.
type testClient struct {
    *ProtocolClient
    t *testing.T
}

func (tc *testClient) mustRegister(tools []protocol.ToolDefinition)
func (tc *testClient) mustSubmitTurn(content string) string // returns turn_id
func (tc *testClient) collectEvents(turnID string, timeout time.Duration) []protocol.Notification
```

#### D1.2. Mock provider

```go
// mockProvider implements provider.Provider with scripted responses.
type mockProvider struct {
    responses map[string]string // model → response text
    streaming bool
    delay     time.Duration
}

func (mp *mockProvider) FormatMessages(opts provider.FormatMessagesOpts) ([]byte, error)
func (mp *mockProvider) ParseStreamEvent(data []byte) (*provider.StreamEvent, error)
```

#### D1.3. Test matrix

| Test | Coverage |
|------|----------|
| `TestBFF_Connect_Register` | Connect, capabilities.register with version negotiation and project context |
| `TestBFF_Turn_Streaming` | turn.submit → token.delta events → turn.complete with metrics |
| `TestBFF_Turn_ToolRoundTrip` | turn.submit → tool.call → tool.result → resumed streaming |
| `TestBFF_Session_CRUD` | Create (implicit), session.list, session.details, session.clear, session.fork |
| `TestBFF_Session_Resume` | Disconnect, reconnect, session.resume with conversation restored |
| `TestBFF_Session_Lock` | Two clients competing for same session → MT-CONN-008 |
| `TestBFF_Session_GracePeriod` | Client disconnect → lock held for 40s → released |
| `TestBFF_ModelSwitch` | model.switch set/clear, model.selected event |
| `TestBFF_ModelList` | model.list returns registry + routing policy |
| `TestBFF_Compaction` | session.compact → plan → compact.apply → tokens freed |
| `TestBFF_MultiModel` | Multi-model routing → parallel branches → aggregate turn.complete |
| `TestBFF_CommandHistory` | turn.submit appends history; history.list returns scoped results |
| `TestBFF_ServerSessions` | server sessions/session detail payloads match spec |
| `TestBFF_Health` | connection.health returns all dependency statuses |
| `TestBFF_Diagnostics` | Provider error → MT-CONN-009 diagnostic event |
| `TestBFF_Idempotency_Turn` | Duplicate turn.submit → existing status returned |
| `TestBFF_Idempotency_ToolResult` | Duplicate tool.result → acknowledged, not reprocessed |
| `TestBFF_SessionSync` | session.sync returns active turn state |
| `TestBFF_ContentTransform` | content.transform → summarized response |
| `TestBFF_CostTracking` | Token counts → cost calculation → cost.update events |
| `TestBFF_ContextPressure` | Context at 78% → compact.suggest; at 92% → auto-compact |

### D2. Harness integration tests (WU-087)

#### D2.1. Mock BFF server

```go
// internal/harness/integration_test.go

// mockBFF provides a test BFF that responds to all protocol methods
// with scripted responses and records incoming requests.
type mockBFF struct {
    listener net.Listener
    handlers map[string]mockHandler
    events   chan protocol.Notification // for scripted streaming events
    received []protocol.Request       // recorded requests
    mu       sync.Mutex
}

type mockHandler func(params json.RawMessage) (any, error)

func newMockBFF(t *testing.T) (*mockBFF, string) // returns mock and socket path
func (m *mockBFF) OnMethod(method string, h mockHandler)
func (m *mockBFF) SendEvent(notif protocol.Notification) // push event to connected client
```

#### D2.2. Test matrix

| Test | Coverage |
|------|----------|
| `TestHarness_Launch` | App initializes, three zones render |
| `TestHarness_Connect_Register` | Connects to mock BFF, capabilities.register sent with tools |
| `TestHarness_Turn_Streaming` | Submit text → turn.submit sent → token.delta received → viewport renders |
| `TestHarness_Tool_Read` | tool.call → Read executes → tool.result sent |
| `TestHarness_Tool_Write` | tool.call → Write with permission prompt → approved → result sent |
| `TestHarness_Tool_Bash` | tool.call → Bash with permission → result sent |
| `TestHarness_Tool_Dangerous` | Dangerous command → always prompted regardless of level |
| `TestHarness_Tool_AllTools` | Each of 13 tools responds correctly (Read, Write, Edit, Bash, Git, Glob, Grep, WebSearch, WebFetch + format variants) |
| `TestHarness_Permission_Default` | Default level: read auto-allowed, write prompts |
| `TestHarness_Permission_AcceptEdits` | Accept-edits: write auto-allowed, bash prompts |
| `TestHarness_Permission_Autonomous` | Autonomous: all auto-allowed except dangerous |
| `TestHarness_PlanMode` | Plan mode intercepts writes, accumulates plan |
| `TestHarness_PlanMode_Approve` | Approve → switch to build, execute |
| `TestHarness_PlanMode_StepThrough` | Step-through executes one at a time |
| `TestHarness_SessionExplorer` | Explorer shows sessions, resume works |
| `TestHarness_ModelDisplay` | /models renders list, /model sets override |
| `TestHarness_MultiModel_Display` | Branch events render progressive completion |
| `TestHarness_Compaction_UI` | /compact shows categories, apply works |
| `TestHarness_CommandHistory` | Up/down arrows traverse BFF-sourced history |
| `TestHarness_History_ScopeSwitch` | /history project changes scope |
| `TestHarness_Connection_States` | All 9 states render with correct indicators and banners |
| `TestHarness_Connection_Reconnect` | Disconnect → reconnect → session.sync |
| `TestHarness_Diagnostics_Render` | Diagnostic events render with code, cause, suggestion |
| `TestHarness_MCP_Connect` | MCP server discovered, tools added, capabilities.update sent |
| `TestHarness_MCP_Failure` | MCP failure → banner, harness continues |
| `TestHarness_FileContext` | @file loads, /context lists, /drop removes |
| `TestHarness_Paste_Large` | Large paste → preview → choices work |

### D3. Shared test protocol fixtures

Both test suites should use the golden fixtures from WU-093 (`internal/protocol/fixtures/`) for constructing and validating protocol messages. This ensures the mock BFF and test client agree on wire format.

## Key Files

| Action | Path | WU |
|--------|------|----|
| NEW | `internal/bff/integration_test.go` | 067 |
| NEW | `internal/harness/integration_test.go` | 087 |

## Dependencies

- WU-067 depends on: all Track A WUs (046-066, 091)
- WU-087 depends on: all Track B WUs (068-086, 092)
- Both depend on: WU-093 protocol fixtures
