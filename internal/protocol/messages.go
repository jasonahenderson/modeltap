package protocol

// This file declares the 19 harness->server request types defined by
// FEAT-0008 for v0.2.0 (WU-039). Each type is the params payload of a
// JSON-RPC 2.0 method call; the transport wraps it in a Request envelope
// (protocol.go).
//
// Canonical field naming is snake_case (see FEAT-0008 "Canonical Field
// Names"). Red-phase stubs below carry the Go field set needed for tests
// to compile; the green phase adds the `json:"..."` tags, correct method
// constant values, and any optional-field pointer/omitempty choices.

// Method name constants. Red-phase values are empty strings so method
// constant tests fail until the green phase assigns the canonical values.
const (
	MethodTurnSubmit           = ""
	MethodTurnCancel           = ""
	MethodToolResult           = ""
	MethodContentTransform     = ""
	MethodSessionResume        = ""
	MethodSessionList          = ""
	MethodSessionDetails       = ""
	MethodSessionCompact       = ""
	MethodCompactApply         = ""
	MethodSessionClear         = ""
	MethodSessionFork          = ""
	MethodSessionSync          = ""
	MethodModelSwitch          = ""
	MethodModelList            = ""
	MethodContextList          = ""
	MethodCapabilitiesRegister = ""
	MethodCapabilitiesUpdate   = ""
	MethodConnectionPing       = ""
	MethodConnectionHealth     = ""
)

// -----------------------------------------------------------------------
// Shared nested types
// -----------------------------------------------------------------------

// Attachment is a harness-supplied file carried in turn.submit.
type Attachment struct {
	Path        string
	Raw         string
	Content     string
	ContentType string
	Transform   string
}

// Paste is the payload for a large paste in turn.submit.
type Paste struct {
	Raw     string
	Content string
	Intent  string
}

// ToolResult is a tool-execution result. The nested form is embedded in
// turn.submit; the standalone form is a dedicated request (see
// ToolResultRequest, which is an alias of this type).
type ToolResult struct {
	ToolCallID string
	Status     string
	Output     string
	OutputType string
	Error      string
	Reason     string
}

// ToolResultRequest is the standalone tool.result request payload. It
// shares the wire shape of ToolResult.
type ToolResultRequest = ToolResult

// ProjectContext describes the harness-side project scope supplied on
// capabilities.register and session.resume.
type ProjectContext struct {
	Root          string
	ConfigFile    string
	ConfigContent string
}

// ToolDefinition is a harness-registered tool catalog entry. Full schema
// details live in FEAT-0008 "Tool Catalog Schema"; extension types (e.g.,
// for server-side tool routing) land in WU-041.
type ToolDefinition struct {
	Name                 string
	Namespace            string
	Description          string
	InputSchema          []byte // json.RawMessage passthrough
	OutputEnvelope       string
	RiskLevel            string
	CapabilitiesRequired []string
}

// -----------------------------------------------------------------------
// 19 harness -> server request types
// -----------------------------------------------------------------------

// TurnSubmit starts (or continues) a turn.
type TurnSubmit struct {
	TurnID      string
	SessionID   string
	Sequence    int
	Mode        Mode
	Content     string
	Attachments []Attachment
	Paste       *Paste
	ToolResults []ToolResult
}

// TurnCancel cancels an in-flight turn.
type TurnCancel struct {
	TurnID string
}

// ContentTransform requests a server-side transformation (e.g., summarize)
// of raw content. Raw is captured per ADR-0005; cost is attributed
// separately from conversation turns.
type ContentTransform struct {
	Transform       string
	RawContent      string
	ContentType     string
	MaxOutputTokens int
}

// SessionResume rehydrates an existing session for the current connection.
type SessionResume struct {
	SessionID string
	Project   ProjectContext
}

// SessionList requests a summary of the caller's sessions. No params.
type SessionList struct{}

// SessionDetails requests a single session's full detail view.
type SessionDetails struct {
	SessionID string
}

// SessionCompact requests a compaction plan for the named session.
type SessionCompact struct {
	SessionID string
}

// CompactApply applies a compaction plan. Actions maps compaction category
// name to one of: keep, summarize, drop, pin.
type CompactApply struct {
	SessionID string
	Actions   map[string]string
}

// SessionClear clears the live context of a session (storage is retained).
type SessionClear struct {
	SessionID string
}

// SessionFork creates an independent copy of a session.
type SessionFork struct {
	SessionID string
}

// SessionSync reports in-flight turn state after a harness reconnection.
type SessionSync struct {
	SessionID string
}

// ModelSwitch sets or clears a session-level model override. Use "auto"
// to clear.
type ModelSwitch struct {
	SessionID string
	Model     string
}

// ModelList requests the registered model catalog. No params.
type ModelList struct{}

// ContextList requests the session's context window breakdown.
type ContextList struct {
	SessionID string
}

// CapabilitiesRegister is the first handshake message after connect,
// declaring protocol version, harness identity, tool catalog, and
// project context.
type CapabilitiesRegister struct {
	ProtocolVersion string
	HarnessVersion  string
	HarnessPlatform string
	Tools           []ToolDefinition
	Project         ProjectContext
}

// CapabilitiesUpdate adds or removes tools after register. SessionID is
// optional; when nil, the update applies to the connection as a whole.
type CapabilitiesUpdate struct {
	SessionID    *string
	AddedTools   []ToolDefinition
	RemovedTools []string
}

// ConnectionPing is the 15-second heartbeat. No params.
type ConnectionPing struct{}

// ConnectionHealth requests structured dependency status. No params.
type ConnectionHealth struct{}
