package provider

import (
	"encoding/json"
)

// Message is the canonical representation of a single conversational turn
// or continuation. The BFF assembles []Message from persisted session
// state; each Provider.FormatMessages translates the canonical form into
// the provider's wire format.
type Message struct {
	Role        string         `json:"role"`                   // user | assistant | system | tool
	Content     string         `json:"content"`                // primary text content; may be empty if tool_calls/results are the payload
	ToolCalls   []ToolCall     `json:"tool_calls,omitempty"`   // assistant role: tool invocations produced by the model
	ToolResults []ToolResult   `json:"tool_results,omitempty"` // tool role OR user continuations: results of executed tools
	Attachments []Attachment   `json:"attachments,omitempty"`  // files/images attached to this message
	Metadata    map[string]any `json:"metadata,omitempty"`     // optional provenance: turn_id, branch_id, timestamps
}

// ToolCall represents a tool invocation produced by an assistant message.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"` // arguments matching the tool's input_schema
}

// ToolResult represents the result of an executed tool call.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Status     string `json:"status"`           // success | rejected | error
	Error      string `json:"error,omitempty"`  // populated when Status == "error"
	Reason     string `json:"reason,omitempty"` // populated when Status == "rejected"
}

// Attachment represents a file or image attached to a message.
type Attachment struct {
	Path        string `json:"path"`    // project-relative file path
	Raw         string `json:"raw"`     // base64-encoded original bytes
	Content     string `json:"content"` // extracted text representation
	ContentType string `json:"content_type"`
	Transform   string `json:"transform"`
}

// Reserved metadata keys.
const (
	MetaKeyTurnID    = "turn_id"
	MetaKeyBranchID  = "branch_id"
	MetaKeyTimestamp = "timestamp"
)

// EstimateTokens returns an approximate token count for arbitrary text
// using a chars/4 heuristic. Precise counts come from provider API
// responses after dispatch.
func EstimateTokens(s string) int {
	return len(s) / 4
}

// EstimateMessageTokens returns an approximate token count for a full
// canonical Message: Content + json.Marshal(ToolCalls) +
// json.Marshal(ToolResults) + sum(Attachments.Content).
// Attachments.Raw (base64) is NOT counted — it is the transformed
// Content that the model sees.
func EstimateMessageTokens(m Message) int {
	total := EstimateTokens(m.Content)

	if len(m.ToolCalls) > 0 {
		if data, err := json.Marshal(m.ToolCalls); err == nil {
			total += EstimateTokens(string(data))
		}
	}

	if len(m.ToolResults) > 0 {
		if data, err := json.Marshal(m.ToolResults); err == nil {
			total += EstimateTokens(string(data))
		}
	}

	for _, att := range m.Attachments {
		total += EstimateTokens(att.Content)
	}

	return total
}
