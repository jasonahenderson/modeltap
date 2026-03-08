// Package provider defines the interface for LLM API provider adapters.
// Each provider (e.g., Anthropic, OpenAI) implements the Provider interface
// to enable provider-agnostic request/response parsing and metadata extraction.
package provider

import "net/http"

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
