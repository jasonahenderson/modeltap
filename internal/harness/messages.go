// Package harness implements the modeltap terminal harness — a
// Bubbletea-based TUI that talks to the BFF over JSON-RPC. The package
// is structured around components owned by a top-level App tea.Model:
//
//   - app.go / model.go: App, AppState, layout
//   - statusbar.go: status line at the bottom
//   - input.go: multi-line input area above the status line
//   - viewport.go: scrollable conversation area above the input
//   - markdown.go: streaming-aware Glamour wrapper used by viewport
//
// The protocol client (WU-073) and connection manager (WU-074) live in
// later WUs; this scaffold defines the message types and orchestration
// glue so those can plug in without breaking the layout.
package harness

import (
	"encoding/json"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// SubmitMsg is sent when the user submits input from the input area.
// IsCommand is true when Content begins with "/" — Command and
// CommandArgs are the parsed values. Otherwise Content is the raw turn
// text and Attachments lists `@file` references found.
type SubmitMsg struct {
	Content     string
	Attachments []string
	IsCommand   bool
	Command     string
	CommandArgs string
}

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

// ConnStateMsg notifies the App that the connection state changed.
type ConnStateMsg struct {
	Info ConnStateInfo
}

// ModelUpdateMsg notifies the App that the routing-resolved model
// changed (either a model.selected event or model.switch result).
type ModelUpdateMsg struct {
	Name     string
	Override bool
	Routing  string
}

// ContextUpdateMsg notifies the App of the current context window
// pressure.
type ContextUpdateMsg struct {
	Pct  float64
	Used int
	Max  int
}

// CostUpdateMsg notifies the App of the running session cost.
type CostUpdateMsg struct {
	Total float64
}

// BannerMsg displays a transient banner above the input area.
// Duration of 0 means "persistent until cleared".
type BannerMsg struct {
	Text     string
	Duration time.Duration
}

// BannerClearMsg clears any active banner.
type BannerClearMsg struct{}

// ModeChangeMsg toggles the execution mode (plan / build / auto).
type ModeChangeMsg struct {
	Mode protocol.Mode
}

// ToolCallMsg signals an incoming tool call from the BFF.
type ToolCallMsg struct {
	TurnID     string
	ToolCallID string
	ToolName   string
	Namespace  string
	Input      json.RawMessage
}

// ToolResultMsg signals that a tool finished executing — used by the
// viewport to render the result block.
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

// PermissionResponseMsg carries the user's approval decision.
type PermissionResponseMsg struct {
	ToolCallID string
	Approved   bool
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

// PasteDetectedMsg signals that the input area received a large paste
// that should be staged via content.transform rather than included
// verbatim.
type PasteDetectedMsg struct {
	Content   string
	ByteSize  int
	LineCount int
	Preview   string
}

// TickMsg is the periodic tick driving the call duration display in
// the status bar.
type TickMsg time.Time
