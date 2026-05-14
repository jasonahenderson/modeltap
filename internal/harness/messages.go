// Package harness implements the modeltap terminal harness — a
// Bubbletea-based TUI that talks to the Runtime over JSON-RPC. After the
// v0.2.2 WU-100 extraction, this package retains only the runtime
// plumbing (connection / protocol / tool dispatcher / context manager
// / MCP); the App-level surfaces (input area, viewport, statusbar,
// markdown rendering, modal dialogs, slash-command handlers) moved
// into internal/harnessshell + internal/harnesshost. The msg types
// below are the runtime-event subset the harnesshost projection
// layer translates into harnessshell.HostEvents — every type here is
// imported by internal/harnesshost/projection.go. App-only msg types
// (SubmitMsg, BannerMsg, ModeChangeMsg, PasteDetectedMsg, etc.) were
// removed in WU-106.
package harness

import (
	"encoding/json"
	"time"
)

// StreamTokenMsg carries one chunk of streamed assistant output.
// BranchID is empty for single-model turns.
type StreamTokenMsg struct {
	TurnID   string
	BranchID string
	Delta    string
}

// StreamCompleteMsg signals that a streaming turn finished.
type StreamCompleteMsg struct {
	TurnID   string
	BranchID string
	Tokens   TokenInfo
	Cost     float64
	Duration time.Duration
	Model    string
}

// ConnStateMsg notifies subscribers that the connection state changed.
type ConnStateMsg struct {
	Info ConnStateInfo
}

// ModelUpdateMsg notifies subscribers that the routing-resolved model
// changed (either a model.selected event or model.switch result).
type ModelUpdateMsg struct {
	Name     string
	Override bool
	Routing  string
}

// ContextUpdateMsg notifies subscribers of the current context window
// pressure.
type ContextUpdateMsg struct {
	Pct  float64
	Used int
	Max  int
}

// CostUpdateMsg notifies subscribers of the running session cost.
type CostUpdateMsg struct {
	Total float64
}

// ToolCallMsg signals an incoming tool call from the Runtime.
type ToolCallMsg struct {
	TurnID     string
	ToolCallID string
	ToolName   string
	Namespace  string
	Input      json.RawMessage
}

// ToolResultMsg signals that a tool finished executing.
type ToolResultMsg struct {
	ToolCallID string
	Status     string
	Output     string
}

// PermissionPromptMsg requests user approval for a tool execution.
type PermissionPromptMsg struct {
	ToolName    string
	RiskLevel   string
	Description string
	Input       json.RawMessage
	ToolCallID  string
}

// BranchStartedMsg signals a multi-model branch began streaming.
type BranchStartedMsg struct {
	TurnID   string
	BranchID string
	Model    string
	Provider string
}

// BranchCompleteMsg signals a multi-model branch finished.
type BranchCompleteMsg struct {
	TurnID       string
	BranchID     string
	Model        string
	InputTokens  int
	OutputTokens int
	Cost         float64
	Duration     time.Duration
}

// BranchErrorMsg signals a multi-model branch failed.
type BranchErrorMsg struct {
	TurnID   string
	BranchID string
	Model    string
	Error    string
}

// StatusUpdateMsg displays a transient status line below the
// streaming output ("routing to claude-opus...", etc.).
type StatusUpdateMsg struct {
	TurnID  string
	Message string
}

// TurnSubmittedMsg fires after the harness has dispatched a turn via
// the protocol client. TurnID is the server-assigned id from the
// turn.submit ack; Err is non-nil if the dispatch failed. Retained in
// case future host code wants to observe the legacy bridge; the
// post-extraction ProductionRuntime calls Client.SubmitTurn
// synchronously and does not depend on this msg.
type TurnSubmittedMsg struct {
	TurnID    string
	SessionID string
	Err       error
}

// ToolActivityPhase discriminates start vs end of a tool execution.
type ToolActivityPhase string

const (
	// ToolActivityStart fires before the dispatcher invokes a tool.
	ToolActivityStart ToolActivityPhase = "start"
	// ToolActivityEnd fires after the tool returns (success or error).
	ToolActivityEnd ToolActivityPhase = "end"
)

// ToolActivityMsg reports tool execution progress so a host can
// render inline tool_call / tool_result lines.
type ToolActivityMsg struct {
	Phase      ToolActivityPhase
	ToolName   string
	ToolCallID string
	Summary    string
	Status     string
	Duration   time.Duration
}
