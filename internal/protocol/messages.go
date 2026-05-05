package protocol

// This file declares the 22 harness->server request types defined by
// FEAT-0008 for v0.2.0 (WU-039, amended for WU-091 command history),
// plus their paired response types (WU-041). Each request type is the
// params payload of a JSON-RPC 2.0 method call; the transport wraps it
// in a Request envelope (protocol.go). Response types are the result
// payload of a JSON-RPC 2.0 Response. Harness requests always carry an
// id; server->harness streaming events use the separate Notification
// envelope (WU-040).
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
	MethodConnectionReady      = "connection.ready"
	MethodHistoryAppend        = "history.append"
	MethodHistoryList          = "history.list"
	MethodRunList              = "run.list"
	MethodRunCreate            = "run.create"
	MethodRunDetails           = "run.details"
	MethodRunAttach            = "run.attach"
	MethodRunDetach            = "run.detach"
	MethodRunCancel            = "run.cancel"
	MethodRunRetry             = "run.retry"
	MethodRunContinue          = "run.continue"
	MethodRunFork              = "run.fork"
	MethodRunEvents            = "run.events"
	MethodRunPermissions       = "run.permissions"
	MethodRunResolvePermission = "run.resolve_permission"
	MethodRunHeartbeat         = "run.heartbeat"
)

// -----------------------------------------------------------------------
// Shared nested types
// -----------------------------------------------------------------------

// Attachment is a harness-supplied file carried in turn.submit.
//
// All five fields are required for a well-formed attachment — FEAT-0008
// §"Protocol Payload Schemas" describes the full payload, and WU-076 /
// WU-082 rely on all fields being present when an attachment is emitted.
// Omitting any field is a spec violation and WU-046 transport validation
// should reject it.
type Attachment struct {
	Path        string `json:"path"`
	Raw         string `json:"raw"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Transform   string `json:"transform"`
}

// Paste is the payload for a large paste in turn.submit.
//
// All three fields are required. Raw preserves the captured input per
// ADR-0005; Content carries whatever the harness chose to send to the
// model (full / truncated / summarized); Intent tags that choice.
type Paste struct {
	Raw     string `json:"raw"`
	Content string `json:"content"`
	Intent  string `json:"intent"`
}

// ToolResult is a tool-execution result. The nested form is embedded in
// turn.submit; the standalone form is a dedicated request (see
// ToolResultRequest, which is an alias of this type).
//
// Error is required only when Status == "error".
// Reason is required only when Status == "rejected".
// All other fields are always required.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Status     string `json:"status"`
	Output     string `json:"output"`
	OutputType string `json:"output_type"`
	Error      string `json:"error,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// ToolResultRequest is the standalone tool.result request payload.
//
// PROTOCOL-FREEZE CONTRACT: this alias encodes a wire-identity guarantee.
// The standalone tool.result request and each element of
// TurnSubmit.ToolResults[] MUST remain wire-identical — they are the same
// logical payload submitted via two transports (inline with a new turn or
// asynchronously after a tool.call).
//
// Any field added to ToolResult in WU-041 or later MUST apply to both
// forms and be reflected through this alias. Splitting the alias into
// two distinct types is a breaking protocol change that requires a
// FEAT-0008 amendment and a WU-093 fixture update.
type ToolResultRequest = ToolResult

// ProjectContext describes the harness-side project scope supplied on
// capabilities.register and session.resume.
type ProjectContext struct {
	Root          string `json:"root"`
	ConfigFile    string `json:"config_file"`
	ConfigContent string `json:"config_content"`
}

// ToolDefinition is declared in tools.go (same package). Moved there
// in WU-041 to co-locate with ToolCatalog and related types.
// Wire shape unchanged; all references within this package use the
// bare name.

// -----------------------------------------------------------------------
// 20 harness -> server request types
// -----------------------------------------------------------------------

// TurnSubmit starts (or continues) a turn. At least one of Content or
// ToolResults must be set; this package does not enforce that constraint
// (see FEAT-0008 dispatch requirements for WU-046 / WU-051).
type TurnSubmit struct {
	TurnID         string       `json:"turn_id"`
	SessionID      string       `json:"session_id"`
	Sequence       int          `json:"sequence"`
	Mode           Mode         `json:"mode"`
	Content        string       `json:"content,omitempty"`
	IdempotencyKey string       `json:"idempotency_key,omitempty"`
	Attachments    []Attachment `json:"attachments,omitempty"`
	Paste          *Paste       `json:"paste,omitempty"`
	ToolResults    []ToolResult `json:"tool_results,omitempty"`
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

// ConnectionReady is a simplified boolean readiness check used during
// auto-start probing (FEAT-0008 §"Local service auto-start"). It is
// distinct from ConnectionHealth: Ready returns a single bool indicating
// the server has completed startup and is accepting requests; Health
// returns full dependency status. No params.
type ConnectionReady struct{}

// HistoryAppend records a user command in the cross-session command history.
// The server also auto-appends on every turn.submit (idempotent by content+timestamp).
// This method is for recording unsent drafts.
type HistoryAppend struct {
	Content   string `json:"content"`
	SessionID string `json:"session_id,omitempty"`
}

// HistoryList requests command history entries with scoping.
type HistoryList struct {
	Scope  string `json:"scope"`            // "user", "project", "session"
	Limit  int    `json:"limit,omitempty"`  // default: 50
	Before string `json:"before,omitempty"` // pagination cursor (opaque)
}

// -----------------------------------------------------------------------
// WU-041: Response types paired with requests above
// -----------------------------------------------------------------------

// TurnSubmitResponse acknowledges a turn.submit. Status is "accepted" on
// first submission; on an idempotent replay, status reflects the current
// turn state and Sync is populated.
type TurnSubmitResponse struct {
	TurnID    string               `json:"turn_id"`
	SessionID string               `json:"session_id,omitempty"`
	Status    string               `json:"status"`
	RunID     string               `json:"run_id,omitempty"`
	Sync      *SessionSyncResponse `json:"sync,omitempty"`
}

// TurnCancelResponse confirms the cancel request was recorded.
type TurnCancelResponse struct {
	TurnID   string `json:"turn_id"`
	Accepted bool   `json:"accepted"`
}

// ToolResultResponse confirms receipt of a tool execution result.
type ToolResultResponse struct {
	ToolCallID string `json:"tool_call_id"`
	Accepted   bool   `json:"accepted"`
}

// ContentTransformResponse is the response to content.transform.
type ContentTransformResponse struct {
	Content   string  `json:"content"`
	ModelUsed string  `json:"model_used"`
	Cost      float64 `json:"cost"`
}

// CapabilitiesUpdateResponse is the response to capabilities.update.
type CapabilitiesUpdateResponse struct {
	AddedCount   int    `json:"added_count"`
	RemovedCount int    `json:"removed_count"`
	UpdatedAt    string `json:"updated_at"`
}

// HistoryAppendResponse is the response to history.append.
type HistoryAppendResponse struct {
	Accepted bool `json:"accepted"`
}

// HistoryEntry is an element of HistoryListResponse.Entries.
type HistoryEntry struct {
	Content   string `json:"content"`
	SessionID string `json:"session_id,omitempty"`
	Timestamp string `json:"timestamp"`
}

// HistoryListResponse is the response to history.list.
type HistoryListResponse struct {
	Entries []HistoryEntry `json:"entries"`
	HasMore bool           `json:"has_more"`
	Cursor  string         `json:"cursor,omitempty"`
}
