package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// OpenAIProvider implements the Provider interface for OpenAI's API.
type OpenAIProvider struct{}

// NewOpenAIProvider creates a new OpenAI provider adapter.
func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{}
}

// Name returns "openai".
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Detect returns true if the request targets the OpenAI API.
// It matches on host (api.openai.com) or path containing /v1/chat/completions
// (unless the anthropic-version header is present, indicating an Anthropic-compatible proxy).
func (p *OpenAIProvider) Detect(r *http.Request) bool {
	if r.Host == "api.openai.com" || r.URL.Host == "api.openai.com" {
		return true
	}

	if strings.Contains(r.URL.Path, "/v1/chat/completions") {
		// Exclude requests with Anthropic-Version header to avoid misdetection.
		if r.Header.Get("Anthropic-Version") != "" {
			return false
		}
		return true
	}

	return false
}

// openaiRequest represents the JSON body of an OpenAI chat completions request.
type openaiRequest struct {
	Model              string          `json:"model"`
	MaxTokens          int             `json:"max_tokens"`
	MaxCompletionTokens int            `json:"max_completion_tokens"`
	Messages           json.RawMessage `json:"messages"`
	Temperature        *float64        `json:"temperature"`
	Stream             bool            `json:"stream"`
}

// ParseRequest extracts metadata from an OpenAI chat completions request body.
func (p *OpenAIProvider) ParseRequest(body []byte, headers http.Header) (*RequestMetadata, error) {
	var req openaiRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("openai: failed to parse request body: %w", err)
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = req.MaxCompletionTokens
	}

	// Count messages.
	var messages []json.RawMessage
	if err := json.Unmarshal(req.Messages, &messages); err != nil {
		// If messages parsing fails, still return what we have.
		messages = nil
	}

	return &RequestMetadata{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Messages:    len(messages),
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}, nil
}

// openaiResponse represents the JSON body of an OpenAI chat completions response.
type openaiResponse struct {
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// ParseResponse extracts metadata from an OpenAI chat completions response.
func (p *OpenAIProvider) ParseResponse(body []byte, headers http.Header, statusCode int) (*ResponseMetadata, error) {
	var resp openaiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openai: failed to parse response body: %w", err)
	}

	meta := &ResponseMetadata{
		Model:        resp.Model,
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}

	if len(resp.Choices) > 0 {
		meta.StopReason = resp.Choices[0].FinishReason
	}

	return meta, nil
}

// openaiStreamChunk represents a single SSE chunk from an OpenAI streaming response.
type openaiStreamChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

// ReassembleStream reconstructs a complete response from OpenAI SSE stream chunks.
func (p *OpenAIProvider) ReassembleStream(chunks []StreamChunk) (*ResponseMetadata, string, error) {
	meta := &ResponseMetadata{}
	var contentBuilder strings.Builder

	for _, chunk := range chunks {
		// Each chunk.Data may contain multiple SSE data lines separated by newlines.
		lines := strings.Split(string(chunk.Data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Strip "data: " prefix.
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")

			// Skip the [DONE] terminator.
			if payload == "[DONE]" {
				continue
			}

			var sc openaiStreamChunk
			if err := json.Unmarshal([]byte(payload), &sc); err != nil {
				return nil, "", fmt.Errorf("openai: failed to parse stream chunk: %w", err)
			}

			if sc.Model != "" {
				meta.Model = sc.Model
			}

			if len(sc.Choices) > 0 {
				contentBuilder.WriteString(sc.Choices[0].Delta.Content)
				if sc.Choices[0].FinishReason != nil {
					meta.StopReason = *sc.Choices[0].FinishReason
				}
			}

			if sc.Usage != nil {
				meta.InputTokens = sc.Usage.PromptTokens
				meta.OutputTokens = sc.Usage.CompletionTokens
			}
		}
	}

	return meta, contentBuilder.String(), nil
}

// FormatMessages is a stub that returns ErrNotImplemented.
// Full implementation lands in WU-044.
func (p *OpenAIProvider) FormatMessages(opts FormatMessagesOpts) ([]byte, error) {
	return nil, ErrNotImplemented
}

// FormatToolDefinitions is a stub that returns ErrNotImplemented.
// Full implementation lands in WU-044.
func (p *OpenAIProvider) FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error) {
	return nil, ErrNotImplemented
}
