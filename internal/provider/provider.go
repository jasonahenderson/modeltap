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
}

// RequestMetadata holds provider-agnostic fields extracted from an LLM API request.
type RequestMetadata struct {
	Model       string
	MaxTokens   int
	Messages    int    // number of messages in the conversation
	Temperature *float64
	Stream      bool
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
	Messages     []Message                  // canonical conversation, in turn order (oldest first)
	SystemPrompt string                     // pre-assembled by prompt engine
	WindowSize   int                        // max total tokens for truncation (approximate)
	Model        string                     // provider-specific model identifier
	MaxTokens    int                        // output token cap
	Temperature  *float64                   // optional
	Stream       bool
	Tools        []protocol.ToolDefinition  // optional; empty slice means no tool-use
	Capabilities []string                   // model capabilities (e.g., "vision", "tool_use")
}

// Error sentinels for provider formatting operations.
var (
	ErrNotImplemented      = errors.New("provider: method not implemented")
	ErrWindowTooSmall      = errors.New("provider: context window too small even for system prompt")
	ErrEmptyMessages       = errors.New("provider: messages slice is empty")
	ErrTruncationEmpty     = errors.New("provider: truncation produced no viable messages")
	ErrInvalidToolInput    = errors.New("provider: tool_call input is not valid JSON")
	ErrUnsupportedOutputType = errors.New("provider: tool_result output_type is not supported by provider")
)
