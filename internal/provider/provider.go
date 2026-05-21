// Package provider defines the interface for LLM API provider adapters.
// Each provider (e.g., Anthropic, OpenAI) implements the Provider interface
// to enable provider-agnostic request/response parsing and metadata extraction.
package provider

import (
	"errors"
	"net/http"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// Provider defines the adapter interface for LLM API providers.
// Implementations detect whether a proxied request targets their API
// and parse provider-specific request/response formats into
// provider-agnostic metadata.
type Provider interface {
	// Name returns the unique identifier for this provider (e.g., "anthropic", "openai").
	Name() string

	// Detect returns true if the given HTTP request is destined for this provider's API.
	Detect(r *http.Request) bool

	// ParseRequest extracts provider-agnostic metadata from a raw request body and headers.
	ParseRequest(body []byte, headers http.Header) (*RequestMetadata, error)

	// ParseResponse extracts provider-agnostic metadata from a raw response body, headers,
	// and HTTP status code.
	ParseResponse(body []byte, headers http.Header, statusCode int) (*ResponseMetadata, error)

	// ReassembleStream reconstructs a complete response from collected SSE stream chunks,
	// returning the response metadata and the reassembled response body.
	ReassembleStream(chunks []StreamChunk) (*ResponseMetadata, string, error)

	// FormatMessages translates the canonical conversation into a
	// provider-specific request body. The returned bytes are the complete
	// HTTP request body (JSON-serialized) ready to send to the provider's
	// API endpoint.
	FormatMessages(opts FormatMessagesOpts) ([]byte, error)

	// FormatToolDefinitions translates a canonical tool catalog into the
	// provider-specific tool-definitions wire shape. Returns the bytes of
	// what would be emitted as the "tools" field value (a JSON array).
	FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error)

	// ParseStreamEvent decodes one provider-specific SSE data payload
	// into a canonical StreamEvent. data is the bytes after `data: ` on
	// a single SSE line, with no trailing newline. Returns (nil, nil)
	// for events that should be skipped (heartbeats, no-op deltas).
	//
	// Used by the Runtime streaming relay (WU-053) to translate streamed
	// tokens / tool calls / usage stats into protocol notifications in
	// real time. Distinct from ReassembleStream, which buffers a full
	// response for the v0.1 capture path; ParseStreamEvent is the
	// per-event semantic parser.
	ParseStreamEvent(data []byte) (*StreamEvent, error)
}

// StreamEvent is the canonical per-event payload emitted while a
// provider response streams. Exactly one of Content / ToolCall / Usage /
// Error is populated, depending on Type.
type StreamEvent struct {
	Type     StreamEventType
	Content  string          // populated for StreamEventText
	ToolCall *StreamToolCall // populated for StreamEventToolCall*
	Usage    *StreamUsage    // populated for StreamEventUsage and StreamEventDone
	Error    string          // populated for StreamEventError
}

// StreamEventType discriminates StreamEvent payloads.
type StreamEventType string

const (
	StreamEventText          StreamEventType = "text"
	StreamEventToolCallStart StreamEventType = "tool_call_start"
	StreamEventToolCallDelta StreamEventType = "tool_call_delta"
	StreamEventToolCallEnd   StreamEventType = "tool_call_end"
	StreamEventUsage         StreamEventType = "usage"
	StreamEventDone          StreamEventType = "done"
	StreamEventError         StreamEventType = "error"
)

// StreamToolCall carries an in-progress or completed tool call from
// streaming events. ID and Name appear on the start event; Input is
// accumulated across delta events; the end event signals completion.
type StreamToolCall struct {
	ID    string
	Name  string
	Input string // raw JSON; may be partial during deltas
}

// StreamUsage carries token-count usage stats. Providers emit this on
// the final stream event (Anthropic's message_delta, OpenAI's last
// chunk with usage attached).
type StreamUsage struct {
	InputTokens  int
	OutputTokens int
}

// RequestMetadata holds provider-agnostic fields extracted from an LLM API request.
type RequestMetadata struct {
	Model        string
	MaxTokens    int
	Messages     int // number of messages in the conversation
	Temperature  *float64
	Stream       bool
	SystemPrompt string
}

// ResponseMetadata holds provider-agnostic fields extracted from an LLM API response.
type ResponseMetadata struct {
	Model        string
	InputTokens  int64
	OutputTokens int64
	StopReason   string
}

// StreamChunk represents a single chunk from an SSE stream response.
type StreamChunk struct {
	Data      []byte
	EventType string // SSE event type (e.g., "message_start", "content_block_delta")
}

// FormatMessagesOpts groups the inputs to FormatMessages.
type FormatMessagesOpts struct {
	Messages     []Message // canonical conversation, in turn order (oldest first)
	SystemPrompt string    // pre-assembled by prompt engine
	WindowSize   int       // max total tokens for truncation (approximate)
	Model        string    // provider-specific model identifier
	MaxTokens    int       // output token cap
	Temperature  *float64  // optional
	Stream       bool
	Tools        []protocol.ToolDefinition // optional; empty slice means no tool-use
	Capabilities []string                  // model capabilities (e.g., "vision", "tool_use")
}

// Error sentinels for provider formatting operations.
var (
	ErrNotImplemented        = errors.New("provider: method not implemented")
	ErrWindowTooSmall        = errors.New("provider: context window too small even for system prompt")
	ErrEmptyMessages         = errors.New("provider: messages slice is empty")
	ErrTruncationEmpty       = errors.New("provider: truncation produced no viable messages")
	ErrInvalidToolInput      = errors.New("provider: tool_call input is not valid JSON")
	ErrUnsupportedOutputType = errors.New("provider: tool_result output_type is not supported by provider")
)
