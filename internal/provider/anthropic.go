package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AnthropicProvider implements the Provider interface for the Anthropic Messages API.
type AnthropicProvider struct{}

// NewAnthropicProvider creates a new Anthropic provider adapter.
func NewAnthropicProvider() *AnthropicProvider {
	return &AnthropicProvider{}
}

// Name returns "anthropic".
func (a *AnthropicProvider) Name() string {
	return "anthropic"
}

// Detect returns true if the request targets the Anthropic API.
// It matches on host (api.anthropic.com), the presence of an anthropic-version
// header, or a path containing /v1/messages with an x-api-key header.
func (a *AnthropicProvider) Detect(r *http.Request) bool {
	if r.Host == "api.anthropic.com" || r.URL.Host == "api.anthropic.com" {
		return true
	}
	if r.Header.Get("anthropic-version") != "" {
		return true
	}
	if strings.Contains(r.URL.Path, "/v1/messages") && r.Header.Get("x-api-key") != "" {
		return true
	}
	return false
}

// ParseRequest extracts metadata from an Anthropic Messages API request body.
func (a *AnthropicProvider) ParseRequest(body []byte, headers http.Header) (*RequestMetadata, error) {
	var req struct {
		Model       string   `json:"model"`
		MaxTokens   int      `json:"max_tokens"`
		Messages    []any    `json:"messages"`
		Temperature *float64 `json:"temperature"`
		Stream      bool     `json:"stream"`
		System      any      `json:"system"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("anthropic: failed to parse request body: %w", err)
	}

	meta := &RequestMetadata{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Messages:  len(req.Messages),
		Stream:    req.Stream,
	}
	if req.Temperature != nil {
		meta.Temperature = req.Temperature
	}

	// System prompt can be a string or an array of content blocks.
	switch s := req.System.(type) {
	case string:
		meta.SystemPrompt = s
	}

	return meta, nil
}

// ParseResponse extracts metadata from an Anthropic Messages API response body.
func (a *AnthropicProvider) ParseResponse(body []byte, headers http.Header, statusCode int) (*ResponseMetadata, error) {
	var resp struct {
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("anthropic: failed to parse response body: %w", err)
	}

	return &ResponseMetadata{
		Model:        resp.Model,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		StopReason:   resp.StopReason,
	}, nil
}

// ReassembleStream reconstructs a complete response from Anthropic SSE stream chunks.
// It handles message_start, content_block_delta, message_delta, and message_stop events.
func (a *AnthropicProvider) ReassembleStream(chunks []StreamChunk) (*ResponseMetadata, string, error) {
	meta := &ResponseMetadata{}
	var textBuilder strings.Builder

	for _, chunk := range chunks {
		switch chunk.EventType {
		case "message_start":
			var envelope struct {
				Message struct {
					Model string `json:"model"`
					Usage struct {
						InputTokens int64 `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal(chunk.Data, &envelope); err != nil {
				return nil, "", fmt.Errorf("anthropic: failed to parse message_start: %w", err)
			}
			meta.Model = envelope.Message.Model
			meta.InputTokens = envelope.Message.Usage.InputTokens

		case "content_block_delta":
			var delta struct {
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(chunk.Data, &delta); err != nil {
				return nil, "", fmt.Errorf("anthropic: failed to parse content_block_delta: %w", err)
			}
			textBuilder.WriteString(delta.Delta.Text)

		case "message_delta":
			var delta struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int64 `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(chunk.Data, &delta); err != nil {
				return nil, "", fmt.Errorf("anthropic: failed to parse message_delta: %w", err)
			}
			meta.StopReason = delta.Delta.StopReason
			meta.OutputTokens = delta.Usage.OutputTokens

		case "content_block_start", "content_block_stop", "message_stop", "ping":
			// These events don't carry metadata we need to extract.
			continue

		default:
			// Ignore unknown event types for forward compatibility.
		}
	}

	return meta, textBuilder.String(), nil
}
