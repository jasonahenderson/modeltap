# 2026-04-16 — Design: WU-039 Protocol Core Messages and Framing

**Work Unit:** WU-039 (Track 0)
**Agent role:** Designer
**Inputs:** FEAT-0008 §"Protocol Messages", §"Protocol Payload Schemas", §"Canonical Field Names", §"Protocol Specification"; `docs/releases/v0.2.0/track-0-shared.md` WU-039 spec.

## Scope

WU-039 delivers the **core messages + framing** slice of `internal/protocol/`:

- `internal/protocol/protocol.go` — protocol version constants, JSON-RPC 2.0 envelope types, NDJSON framing reader/writer, `Mode` type, canonical field-name doc.
- `internal/protocol/messages.go` — 19 harness→server request types and their method-name constants.
- `internal/protocol/protocol_test.go` — round-trip marshal/unmarshal for every message type, framing tests, Mode tests.

**Out of scope (handled in later WUs):**
- WU-040 — server→harness streaming events (`token.delta`, `tool.call`, `turn.complete`, etc.)
- WU-041 — tool/session/model/health/error/compact payload types
- WU-093 — golden fixtures and conformance tests
- All business logic, dispatch, validation beyond zero-value / JSON shape round-trip

## Package Layout

```
internal/protocol/
├── protocol.go   # envelope, framing, version, Mode, field-name doc
├── messages.go   # 19 harness→server request types
└── protocol_test.go
```

Subsequent WUs add `events.go`, `tools.go`, `sessions.go`, `models.go`, `health.go`, `errors.go`, `compact.go`, and (WU-093) `fixtures/` + `conformance_test.go`.

## Design Decisions

### D1. Canonical field naming: **snake_case**
FEAT-0008 §"Canonical Field Names" mandates snake_case on the wire. All JSON tags use snake_case; Go field identifiers remain CamelCase per Go convention. Every field carries an explicit `json:"..."` tag (no implicit lowercasing).

### D2. JSON-RPC 2.0 envelope
Each wire message is a JSON-RPC 2.0 frame. Envelope types live in `protocol.go`:

```go
type Request struct {
    JSONRPC string          `json:"jsonrpc"` // always "2.0"
    ID      json.RawMessage `json:"id,omitempty"` // string | number | null
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"` // always "2.0"
    ID      json.RawMessage `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *ErrorObject    `json:"error,omitempty"`
}

type ErrorObject struct {
    Code    int             `json:"code"`
    Message string          `json:"message"`
    Data    json.RawMessage `json:"data,omitempty"`
}
```

`json.RawMessage` preserves params/result for deferred decoding into the correct typed struct based on `Method`. Keeping the envelope dumb decouples WU-039 from dispatch (WU-046).

### D3. NDJSON framing
Each JSON frame is a complete JSON object terminated by `\n`. `protocol.go` exposes:

```go
const MaxFrameSize = 10 * 1024 * 1024 // 10 MB

type FrameReader struct { ... }
func NewFrameReader(r io.Reader) *FrameReader
func (fr *FrameReader) ReadFrame() ([]byte, error) // returns the raw JSON bytes (no trailing \n)

type FrameWriter struct { ... }
func NewFrameWriter(w io.Writer) *FrameWriter
func (fw *FrameWriter) WriteFrame(b []byte) error // caller supplies raw JSON; writer appends \n

// Errors
var ErrFrameTooLarge = errors.New("protocol: frame exceeds max size")
var ErrInvalidFrame  = errors.New("protocol: invalid frame")
```

**Rationale for 10 MiB cap:** `turn.submit` may carry attachments (base64-encoded raw bytes + extracted content) and `paste` payloads. A 10 MiB cap (binary megabytes, `10 * 1024 * 1024` bytes) covers typical docs, images, and spreadsheets without allowing trivial memory exhaustion. Enforced on the reader side to prevent malformed input from bloating buffers. **Open item:** FEAT-0008 does not ratify this cap (review finding A-05). A follow-up amendment should either bless this default or expose it in the capability handshake (WU-049) so the harness can refuse oversize attachments before serializing.

`FrameReader` uses a `bufio.Reader` with a per-read `io.LimitedReader` of `MaxFrameSize`; exceeding the cap returns `ErrFrameTooLarge` (the connection should be terminated by higher layers, not by this package).

### D4. `Mode` type
```go
type Mode string

const (
    ModePlan  Mode = "plan"
    ModeBuild Mode = "build"
    ModeAuto  Mode = "auto"
)

func (m Mode) Valid() bool // returns true iff m in {plan, build, auto}
```

`Valid()` is the only behavior — no dispatch, no defaulting. Higher layers (WU-080 harness modes, WU-046 transport) enforce.

### D5. Required vs. optional field representation
- **Required scalar** (string, int): plain type; round-trip test sends a non-zero value.
- **Optional scalar with presence semantics** (may be absent OR explicitly null): pointer type (`*string`, `*int`, `*bool`) + `omitempty`.
- **Optional scalar where zero-value is indistinguishable from absent** (and that's acceptable): plain type + `omitempty`.
- **Required array/map**: plain type, no `omitempty`.
- **Optional array/map**: plain type + `omitempty`.

Rule of thumb: if the server needs to distinguish "field omitted" from "field set to zero", use a pointer. Otherwise prefer plain types for ergonomics.

### D6. Method-name constants
Each request type declares its method-name constant alongside the struct:

```go
const MethodTurnSubmit = "turn.submit"

type TurnSubmit struct {
    TurnID      string        `json:"turn_id"`
    SessionID   string        `json:"session_id"`
    Sequence    int           `json:"sequence"`
    Mode        Mode          `json:"mode"`
    Content     string        `json:"content,omitempty"`
    Attachments []Attachment  `json:"attachments,omitempty"`
    Paste       *Paste        `json:"paste,omitempty"`
    ToolResults []ToolResult  `json:"tool_results,omitempty"`
}
```

Dispatch (WU-046) will consume these constants; WU-039 does not itself dispatch.

### D7. No runtime validation
Per WU-039 DoD: "only types and serialization — no business logic." Beyond `Mode.Valid()` (a trivial enum check that belongs with the type), there is no validation in this package. A missing required field, an unknown `Mode` value, or a bad UUID round-trips cleanly in WU-039; WU-046 (transport) and handler-layer WUs are responsible for rejecting malformed requests with the correct diagnostic codes.

## Type Catalog — 20 Harness→Server Request Types

All types live in `messages.go`. Method-name constants are colocated.

> Revision: updated from 19 to 20 after B-01 finding in the retroactive review added `ConnectionReady` (FEAT-0008 line 211 classifies `connection.ready` as harness→server; the WU-039 spec in track-0-shared.md line 15 lists it; earlier draft dropped it during catalog compilation).

### Shared nested types (also in `messages.go`)

```go
type Attachment struct {
    Path        string `json:"path"`
    Raw         string `json:"raw"`          // base64
    Content     string `json:"content"`
    ContentType string `json:"content_type"`
    Transform   string `json:"transform"`    // none | pdf_text_extract | docx_text_extract | xlsx_parse | csv_parse | base64_encode
}

type Paste struct {
    Raw     string `json:"raw"`
    Content string `json:"content"`
    Intent  string `json:"intent"`           // full | truncated | summarized
}

type ToolResult struct {
    ToolCallID string `json:"tool_call_id"`
    Status     string `json:"status"`        // success | rejected | error
    Output     string `json:"output"`
    OutputType string `json:"output_type"`   // text | json | binary | image
    Error      string `json:"error,omitempty"`
    Reason     string `json:"reason,omitempty"`
}

type ProjectContext struct {
    Root          string `json:"root"`
    ConfigFile    string `json:"config_file"`
    ConfigContent string `json:"config_content"`
}

// ToolDefinition lives in messages.go for WU-039 because CapabilitiesRegister
// embeds it. WU-041's tools.go may extend it, but the struct shape is
// already fixed by FEAT-0008's protocol freeze.
type ToolDefinition struct {
    Name                 string          `json:"name"`
    Namespace            string          `json:"namespace"`       // builtin | mcp:<server>
    Description          string          `json:"description"`
    InputSchema          json.RawMessage `json:"input_schema"`    // JSON Schema passthrough
    OutputEnvelope       string          `json:"output_envelope"` // text | json | binary | image
    RiskLevel            string          `json:"risk_level"`      // read_only | write | execute | destructive
    CapabilitiesRequired []string        `json:"capabilities_required"`
}
```

### 19 request types

| # | Go type | Method constant | JSON-RPC method |
|---|---------|-----------------|------------------|
| 1 | `TurnSubmit` | `MethodTurnSubmit` | `turn.submit` |
| 2 | `TurnCancel` | `MethodTurnCancel` | `turn.cancel` |
| 3 | `ToolResultRequest` | `MethodToolResult` | `tool.result` |
| 4 | `ContentTransform` | `MethodContentTransform` | `content.transform` |
| 5 | `SessionResume` | `MethodSessionResume` | `session.resume` |
| 6 | `SessionList` | `MethodSessionList` | `session.list` |
| 7 | `SessionDetails` | `MethodSessionDetails` | `session.details` |
| 8 | `SessionCompact` | `MethodSessionCompact` | `session.compact` |
| 9 | `CompactApply` | `MethodCompactApply` | `compact.apply` |
| 10 | `SessionClear` | `MethodSessionClear` | `session.clear` |
| 11 | `SessionFork` | `MethodSessionFork` | `session.fork` |
| 12 | `SessionSync` | `MethodSessionSync` | `session.sync` |
| 13 | `ModelSwitch` | `MethodModelSwitch` | `model.switch` |
| 14 | `ModelList` | `MethodModelList` | `model.list` |
| 15 | `ContextList` | `MethodContextList` | `context.list` |
| 16 | `CapabilitiesRegister` | `MethodCapabilitiesRegister` | `capabilities.register` |
| 17 | `CapabilitiesUpdate` | `MethodCapabilitiesUpdate` | `capabilities.update` |
| 18 | `ConnectionPing` | `MethodConnectionPing` | `connection.ping` |
| 19 | `ConnectionHealth` | `MethodConnectionHealth` | `connection.health` |
| 20 | `ConnectionReady` | `MethodConnectionReady` | `connection.ready` |

**Naming note — `ToolResultRequest`:** the standalone message `tool.result` uses the same payload shape as the `ToolResult` nested object in `TurnSubmit.ToolResults`. To keep one wire shape but avoid a naming clash, WU-039 names the nested struct `ToolResult` and the standalone request `ToolResultRequest` (a type alias in Go: `type ToolResultRequest = ToolResult`). This documents the semantic equivalence while allowing distinct method-name routing.

### Field-level catalog

Exhaustive per-type field list below. Required/optional marked (R) or (O). All JSON tags snake_case. Source: FEAT-0008 "Protocol Payload Schemas" as extracted in the WU-039 planning research.

#### TurnSubmit
- `turn_id string` (R)
- `session_id string` (R)
- `sequence int` (R)
- `mode Mode` (R)
- `content string` (O)
- `attachments []Attachment` (O)
- `paste *Paste` (O)
- `tool_results []ToolResult` (O)
- Constraint (documented, not enforced here): at least one of `content` or `tool_results` must be present.

#### TurnCancel
- `turn_id string` (R)

#### ToolResultRequest (alias of `ToolResult`)
- Fields as in nested `ToolResult`.

#### ContentTransform
- `transform string` (R)           // e.g., "summarize"
- `raw_content string` (R)
- `content_type string` (R)
- `max_output_tokens int` (R)

#### SessionResume
- `session_id string` (R)
- `project ProjectContext` (R)

#### SessionList
- No params. Represented as `type SessionList struct{}`.

#### SessionDetails
- `session_id string` (R)

#### SessionCompact
- `session_id string` (R)

#### CompactApply
- `session_id string` (R)
- `actions map[string]string` (R)  // category → keep|summarize|drop|pin

#### SessionClear
- `session_id string` (R)

#### SessionFork
- `session_id string` (R)

#### SessionSync
- `session_id string` (R)

#### ModelSwitch
- `session_id string` (R)
- `model string` (R)               // model name or "auto"

#### ModelList
- No params. `type ModelList struct{}`.

#### ContextList
- `session_id string` (R)

#### CapabilitiesRegister
- `protocol_version string` (R)
- `harness_version string` (R)
- `harness_platform string` (R)
- `tools []ToolDefinition` (R)
- `project ProjectContext` (R)

#### CapabilitiesUpdate
- `session_id *string` (O)         // optional: nil → server-scope update
- `added_tools []ToolDefinition` (O)
- `removed_tools []string` (O)     // tool names

#### ConnectionPing
- No params. `type ConnectionPing struct{}`.

#### ConnectionHealth
- No params. `type ConnectionHealth struct{}`.

#### ConnectionReady
- No params. `type ConnectionReady struct{}`.
- Simplified boolean readiness probe used during local auto-start (FEAT-0008 §"Local service auto-start"). Distinct from `ConnectionHealth`: `Ready` is a single bool, `Health` is a full dependency status.

### Protocol version constant

```go
const ProtocolVersion = "1"
```

Exported as a string for wire use. `internal/bff/capabilities.go` (WU-049) will compare client-supplied version against a supported-range constant list derived from this.

## Test Plan (`protocol_test.go`)

### T1. Round-trip per request type
Table-driven test: for each of the 19 types, construct a representative instance with all fields populated (including optional), marshal, unmarshal back into a fresh instance of the same type, and `reflect.DeepEqual` the original and the reconstructed.

Two tables per type when optional fields exist:
- **Full**: every field populated.
- **Minimal**: only required fields.

For `TurnSubmit` specifically, three cases:
- content-only turn
- tool_results-only turn
- both + attachments + paste

### T2. Envelope round-trip
Wrap a `TurnSubmit` in `Request`, marshal, unmarshal, decode `Params` into a fresh `TurnSubmit`, and compare. Same for `Response` with a `json.RawMessage` result. Error-object path with non-nil `Data` also round-tripped.

### T3. NDJSON framing
- Write N frames via `FrameWriter`, read via `FrameReader`, compare byte-equality.
- Write a frame larger than `MaxFrameSize` → reader returns `ErrFrameTooLarge`.
- Write a frame with embedded `\n` inside a string — must round-trip (our framing splits on the delimiter at the outer level; we rely on the writer supplying a valid JSON object whose serialized form has no literal newline). **Decision:** `FrameWriter.WriteFrame` asserts the input contains no literal `\n` byte (standard `json.Marshal` output never does, and callers must re-marshal if they have whitespace-containing input). Test enforces this contract.
- EOF handling: reader returns `io.EOF` cleanly after last frame.
- Partial read: reader blocks/returns bytes as expected given `io.Reader` semantics (use `bytes.Buffer`).

### T4. Mode
- `ModePlan.Valid()` / `ModeBuild.Valid()` / `ModeAuto.Valid()` → true.
- `Mode("bogus").Valid()` → false.
- `Mode("")` → false.
- JSON round-trip of each valid mode inside a `TurnSubmit` preserves the exact string.

### T5. Method-name constants
Test that every `Method*` constant equals the string value documented in the catalog. This catches accidental rename.

### T6. Canonical field names
For a representative instance of each type, assert the marshaled JSON contains expected snake_case keys (e.g., `turn_id`, `tool_call_id`, `content_type`). One test per type with a golden field-name set. This is the WU-039 bulwark against a CamelCase regression; WU-093 strengthens it with full fixtures.

## Files Modified / Created

**New:**
- `internal/protocol/protocol.go`
- `internal/protocol/messages.go`
- `internal/protocol/protocol_test.go`

**Modified:**
- `go.mod` — only if new dependencies are required (none expected; stdlib `encoding/json`, `bufio`, `io`, `errors` suffice).

## Risks and Non-Risks

- **Non-risk:** no runtime validation means a bad `Mode` or missing required field round-trips. This is intentional; transport-layer WUs reject malformed requests. WU-093 golden fixtures will flag any drift from the catalog.
- **Risk:** choosing pointer vs. plain types for optional fields locks in presence semantics. If wrong, downstream WUs may need to change signatures. Mitigation: follow the D5 rule consistently; if unsure, prefer pointers (more expressive, matches JSON-level distinction).
- **Risk:** `MaxFrameSize = 10 MB` might be too low for large attachments. Mitigation: constant is exposed and tested; raise in a follow-up patch if field data shows need.
- **Non-risk:** `ToolResultRequest = ToolResult` alias. If WU-041 or later adds fields to the nested `ToolResult` that don't make sense for the standalone request, split the alias. Not foreseen.

## Ready for Tester

This design gives the Test Engineer enough to write `protocol_test.go` with failing tests before implementation begins.
