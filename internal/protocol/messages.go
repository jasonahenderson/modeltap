package protocol

import "encoding/json"

// This file declares the 19 harness->server request types defined by
// FEAT-0008 for v0.2.0 (WU-039). Each type is the params payload of a
// JSON-RPC 2.0 method call; the transport wraps it in a Request envelope
// (protocol.go).
//
// Canonical field naming is snake_case (see FEAT-0008 "Canonical Field
// Names"). Every field carries an explicit JSON tag so that default
// lowercasing cannot leak a CamelCase form onto the wire.

// Method-name constants. Values are the JSON-RPC "method" strings that
// identify each request type on the wire.
const (
	MethodTurnSubmit           = "turn.submit"
	MethodTurnCancel           = "turn.cancel"
	MethodToolResult           = "tool.result"
	MethodContentTransform     = "content.transform"
	MethodSessionResume        = "session.resume"
	MethodSessionList          = "session.list"
	MethodSessionDetails       = "session.details"
	MethodSessionCompact       = "session.compact"
	MethodCompactApply         = "compact.apply"
	MethodSessionClear         = "session.clear"
	MethodSessionFork          = "session.fork"
	MethodSessionSync          = "session.sync"
	MethodModelSwitch          = "model.switch"
	MethodModelList            = "model.list"
	MethodContextList          = "context.list"
	MethodCapabilitiesRegister = "capabilities.register"
	MethodCapabilitiesUpdate   = "capabilities.update"
	MethodConnectionPing       = "connection.ping"
	MethodConnectionHealth     = "connection.health"
)

// -----------------------------------------------------------------------
// Shared nested types
// -----------------------------------------------------------------------

// Attachment is a harness-supplied file carried in turn.submit.
type Attachment struct {
	Path        string `json:"path"`
	Raw         string `json:"raw"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Transform   string `json:"transform"`
}

// Paste is the payload for a large paste in turn.submit.
type Paste struct {
	Raw     string `json:"raw"`
	Content string `json:"content"`
	Intent  string `json:"intent"`
}

// ToolResult is a tool-execution result. The nested form is embedded in
// turn.submit; the standalone form is a dedicated request (see
// ToolResultRequest, which is an alias of this type).
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Status     string `json:"status"`
	Output     string `json:"output"`
	OutputType string `json:"output_type"`
	Error      string `json:"error,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// ToolResultRequest is the standalone tool.result request payload. It
// shares the wire shape of ToolResult.
type ToolResultRequest = ToolResult

// ProjectContext describes the harness-side project scope supplied on
// capabilities.register and session.resume.
type ProjectContext struct {
	Root          string `json:"root"`
	ConfigFile    string `json:"config_file"`
	ConfigContent string `json:"config_content"`
}

// ToolDefinition is a harness-registered tool catalog entry. Full schema
// details live in FEAT-0008 "Tool Catalog Schema"; extension types (e.g.,
// server-side tool routing) land in WU-041. InputSchema is a
// json.RawMessage so JSON Schema payloads pass through without interpretation.
type ToolDefinition struct {
	Name                 string          `json:"name"`
	Namespace            string          `json:"namespace"`
	Description          string          `json:"description"`
	InputSchema          json.RawMessage `json:"input_schema"`
	OutputEnvelope       string          `json:"output_envelope"`
	RiskLevel            string          `json:"risk_level"`
	CapabilitiesRequired []string        `json:"capabilities_required"`
}

// -----------------------------------------------------------------------
// 19 harness -> server request types
// -----------------------------------------------------------------------

// TurnSubmit starts (or continues) a turn. At least one of Content or
// ToolResults must be set; this package does not enforce that constraint
// (see FEAT-0008 dispatch requirements for WU-046 / WU-051).
type TurnSubmit struct {
	TurnID      string       `json:"turn_id"`
	SessionID   string       `json:"session_id"`
	Sequence    int          `json:"sequence"`
	Mode        Mode         `json:"mode"`
	Content     string       `json:"content,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Paste       *Paste       `json:"paste,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// TurnCancel cancels an in-flight turn.
type TurnCancel struct {
	TurnID string `json:"turn_id"`
}

// ContentTransform requests a server-side transformation (e.g., summarize)
// of raw content. Raw is captured per ADR-0005; cost is attributed
// separately from conversation turns.
type ContentTransform struct {
	Transform       string `json:"transform"`
	RawContent      string `json:"raw_content"`
	ContentType     string `json:"content_type"`
	MaxOutputTokens int    `json:"max_output_tokens"`
}

// SessionResume rehydrates an existing session for the current connection.
type SessionResume struct {
	SessionID string         `json:"session_id"`
	Project   ProjectContext `json:"project"`
}

// SessionList requests a summary of the caller's sessions. No params.
type SessionList struct{}

// SessionDetails requests a single session's full detail view.
type SessionDetails struct {
	SessionID string `json:"session_id"`
}

// SessionCompact requests a compaction plan for the named session.
type SessionCompact struct {
	SessionID string `json:"session_id"`
}

// CompactApply applies a compaction plan. Actions maps compaction category
// name to one of: keep, summarize, drop, pin.
type CompactApply struct {
	SessionID string            `json:"session_id"`
	Actions   map[string]string `json:"actions"`
}

// SessionClear clears the live context of a session (storage is retained).
type SessionClear struct {
	SessionID string `json:"session_id"`
}

// SessionFork creates an independent copy of a session.
type SessionFork struct {
	SessionID string `json:"session_id"`
}

// SessionSync reports in-flight turn state after a harness reconnection.
type SessionSync struct {
	SessionID string `json:"session_id"`
}

// ModelSwitch sets or clears a session-level model override. Use "auto"
// to clear.
type ModelSwitch struct {
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}

// ModelList requests the registered model catalog. No params.
type ModelList struct{}

// ContextList requests the session's context window breakdown.
type ContextList struct {
	SessionID string `json:"session_id"`
}

// CapabilitiesRegister is the first handshake message after connect,
// declaring protocol version, harness identity, tool catalog, and
// project context.
type CapabilitiesRegister struct {
	ProtocolVersion string           `json:"protocol_version"`
	HarnessVersion  string           `json:"harness_version"`
	HarnessPlatform string           `json:"harness_platform"`
	Tools           []ToolDefinition `json:"tools"`
	Project         ProjectContext   `json:"project"`
}

// CapabilitiesUpdate adds or removes tools after register. SessionID is
// optional (pointer): when nil, the update applies to the connection as
// a whole; when non-nil, only to the identified session.
type CapabilitiesUpdate struct {
	SessionID    *string          `json:"session_id,omitempty"`
	AddedTools   []ToolDefinition `json:"added_tools,omitempty"`
	RemovedTools []string         `json:"removed_tools,omitempty"`
}

// ConnectionPing is the 15-second heartbeat. No params.
type ConnectionPing struct{}

// ConnectionHealth requests structured dependency status. No params.
type ConnectionHealth struct{}
