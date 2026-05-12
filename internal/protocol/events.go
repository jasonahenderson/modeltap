package protocol

import "encoding/json"

// This file declares the 16 server->harness streaming and non-streaming
// event types for WU-040. Each type is the params payload of a JSON-RPC
// 2.0 Notification (no id). The transport wraps each in a Notification
// envelope (protocol.go).
//
// All events carry turn_id as a required field (per FEAT-0008
// §"Correlation") except ConnectionPong and CapabilitiesRequestEvent.
// Multi-model branching events carry branch_id (optional, omitempty).

// Event method-name constants. Values are the JSON-RPC "method" strings
// that identify each event type on the wire.
const (
	EventTokenDelta          = "token.delta"
	EventBranchStarted       = "branch.started"
	EventBranchComplete      = "branch.complete"
	EventBranchError         = "branch.error"
	EventToolCall            = "tool.call"
	EventStatusUpdate        = "status.update"
	EventKnowledgeHit        = "knowledge.hit"
	EventCostUpdate          = "cost.update"
	EventCompactPlan         = "compact.plan"
	EventCompactSuggest      = "compact.suggest"
	EventCompactNotice       = "compact.notice"
	EventTurnComplete        = "turn.complete"
	EventModelSelected       = "model.selected"
	EventError               = "error"
	EventCapabilitiesRequest = "capabilities.request"
	EventConnectionPong      = "connection.pong"
)

// TokenDelta is an incremental text chunk streamed during a turn.
type TokenDelta struct {
	TurnID   string `json:"turn_id"`
	RunID    string `json:"run_id,omitempty"`
	BranchID string `json:"branch_id,omitempty"`
	Text     string `json:"text"`
}

// BranchStarted signals the start of a multi-model branch.
type BranchStarted struct {
	TurnID   string `json:"turn_id"`
	BranchID string `json:"branch_id"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}

// BranchComplete signals that a multi-model branch finished successfully.
type BranchComplete struct {
	TurnID            string `json:"turn_id"`
	BranchID          string `json:"branch_id"`
	FinalInputTokens  int    `json:"final_input_tokens"`
	FinalOutputTokens int    `json:"final_output_tokens"`
	Model             string `json:"model"`
	Provider          string `json:"provider"`
}

// BranchError signals that a multi-model branch failed.
type BranchError struct {
	TurnID         string         `json:"turn_id"`
	BranchID       string         `json:"branch_id"`
	Error          string         `json:"error"`
	Message        string         `json:"message"`
	DiagnosticCode DiagnosticCode `json:"diagnostic_code"`
	Model          string         `json:"model"`
	Provider       string         `json:"provider"`
}

// ToolCall requests that the harness execute a tool. Input is
// json.RawMessage so the tool's input_schema payload passes through
// without interpretation.
type ToolCall struct {
	TurnID     string          `json:"turn_id"`
	ToolCallID string          `json:"tool_call_id"`
	Tool       string          `json:"tool"`
	Namespace  string          `json:"namespace"`
	Input      json.RawMessage `json:"input"`
}

// StatusUpdate reports a phase transition during turn processing.
type StatusUpdate struct {
	TurnID    string `json:"turn_id"`
	Phase     string `json:"phase"`
	Detail    string `json:"detail"`
	Timestamp string `json:"timestamp"`
}

// KnowledgeHit reports a relevant knowledge-layer match found during
// turn processing.
type KnowledgeHit struct {
	TurnID     string  `json:"turn_id"`
	Summary    string  `json:"summary"`
	SourceDate string  `json:"source_date"`
	Relevance  float64 `json:"relevance"`
}

// CostUpdate provides incremental cost accounting during a turn.
type CostUpdate struct {
	TurnID       string  `json:"turn_id"`
	BranchID     string  `json:"branch_id,omitempty"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	InputCost    float64 `json:"input_cost"`
	OutputCost   float64 `json:"output_cost"`
	TotalCost    float64 `json:"total_cost"`
}

// CompactSuggest advises the harness that context usage is high and
// compaction should be considered.
type CompactSuggest struct {
	TurnID     string  `json:"turn_id"`
	ContextPct float64 `json:"context_pct"`
	Threshold  float64 `json:"threshold"`
	Message    string  `json:"message"`
}

// CompactNotice reports that an automatic compaction occurred.
type CompactNotice struct {
	TurnID      string `json:"turn_id"`
	TriggeredBy string `json:"triggered_by"`
	TokensFreed int    `json:"tokens_freed"`
	Summary     string `json:"summary"`
}

// TurnComplete signals that a turn finished processing.
type TurnComplete struct {
	RunID             string  `json:"run_id,omitempty"`
	TurnID            string  `json:"turn_id"`
	FinalInputTokens  int     `json:"final_input_tokens"`
	FinalOutputTokens int     `json:"final_output_tokens"`
	TotalCost         float64 `json:"total_cost"`
	Model             string  `json:"model"`
	Provider          string  `json:"provider"`
	LatencyMs         int     `json:"latency_ms"`
	Cancelled         bool    `json:"cancelled"`
}

// ModelSelected reports which model(s) were selected for a turn.
// Model and Provider are json.RawMessage because they may be a single
// string (single-model turn) or an array of strings (multi-model turn).
type ModelSelected struct {
	TurnID   string          `json:"turn_id"`
	Model    json.RawMessage `json:"model"`
	Provider json.RawMessage `json:"provider"`
	Reason   string          `json:"reason"`
}

// IsMulti reports whether this is a multi-model selection (Model is a
// JSON array rather than a string).
func (ms ModelSelected) IsMulti() bool {
	if len(ms.Model) == 0 {
		return false
	}
	return ms.Model[0] == '['
}

// SingleModel returns the model and provider as strings. Returns an
// error if the selection is multi-model.
func (ms ModelSelected) SingleModel() (string, string, error) {
	if ms.IsMulti() {
		return "", "", json.Unmarshal(ms.Model, new(string)) // force error
	}
	var model, provider string
	if err := json.Unmarshal(ms.Model, &model); err != nil {
		return "", "", err
	}
	if err := json.Unmarshal(ms.Provider, &provider); err != nil {
		return "", "", err
	}
	return model, provider, nil
}

// MultiModels returns the models and providers as string slices. Returns
// an error if the selection is single-model.
func (ms ModelSelected) MultiModels() ([]string, []string, error) {
	if !ms.IsMulti() {
		return nil, nil, json.Unmarshal(ms.Model, new([]string)) // force error
	}
	var models, providers []string
	if err := json.Unmarshal(ms.Model, &models); err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(ms.Provider, &providers); err != nil {
		return nil, nil, err
	}
	return models, providers, nil
}

// ServerError is a structured error event sent to the harness. Code is a
// coarse error bucket (e.g., "provider_error", "budget_exceeded"); the
// specific MT-CONN-* code lives in the nested Diagnostic.
type ServerError struct {
	TurnID     string     `json:"turn_id,omitempty"`
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Diagnostic Diagnostic `json:"diagnostic"`
}

// CapabilitiesRequestEvent is a server-initiated request for the harness
// to re-register capabilities. Reason is optional.
type CapabilitiesRequestEvent struct {
	Reason string `json:"reason,omitempty"`
}

// ConnectionPong is the server's response to a heartbeat ping, sent as
// a Notification (no id). No fields.
type ConnectionPong struct{}
