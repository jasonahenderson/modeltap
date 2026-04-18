package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jasonahenderson/modeltap/internal/protocol"
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

// ParseStreamEvent decodes a single Anthropic SSE data payload. The
// JSON object's "type" field discriminates between message_start,
// content_block_start, content_block_delta, message_delta, and
// message_stop. Returns (nil, nil) for events the relay should skip
// (ping, message_start without useful info, content_block_stop).
func (a *AnthropicProvider) ParseStreamEvent(data []byte) (*StreamEvent, error) {
	// Probe the type field first; everything else is conditional.
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("anthropic: parse stream event: %w", err)
	}

	switch probe.Type {
	case "content_block_delta":
		var ev struct {
			Index int `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic: content_block_delta: %w", err)
		}
		switch ev.Delta.Type {
		case "text_delta":
			if ev.Delta.Text == "" {
				return nil, nil
			}
			return &StreamEvent{Type: StreamEventText, Content: ev.Delta.Text}, nil
		case "input_json_delta":
			return &StreamEvent{
				Type:     StreamEventToolCallDelta,
				ToolCall: &StreamToolCall{Input: ev.Delta.PartialJSON},
			}, nil
		}
		return nil, nil

	case "content_block_start":
		var ev struct {
			Index        int `json:"index"`
			ContentBlock struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				Name  string `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic: content_block_start: %w", err)
		}
		if ev.ContentBlock.Type != "tool_use" {
			return nil, nil
		}
		return &StreamEvent{
			Type: StreamEventToolCallStart,
			ToolCall: &StreamToolCall{
				ID:    ev.ContentBlock.ID,
				Name:  ev.ContentBlock.Name,
				Input: string(ev.ContentBlock.Input),
			},
		}, nil

	case "content_block_stop":
		// We can't tell from this event alone whether the block was a
		// tool_use or text. The relay tracks active tool calls itself.
		return &StreamEvent{Type: StreamEventToolCallEnd}, nil

	case "message_delta":
		var ev struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic: message_delta: %w", err)
		}
		return &StreamEvent{
			Type:  StreamEventUsage,
			Usage: &StreamUsage{OutputTokens: ev.Usage.OutputTokens},
		}, nil

	case "message_start":
		var ev struct {
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("anthropic: message_start: %w", err)
		}
		if ev.Message.Usage.InputTokens == 0 {
			return nil, nil
		}
		return &StreamEvent{
			Type:  StreamEventUsage,
			Usage: &StreamUsage{InputTokens: ev.Message.Usage.InputTokens},
		}, nil

	case "message_stop":
		return &StreamEvent{Type: StreamEventDone}, nil

	case "ping", "":
		return nil, nil
	}

	// Forward-compatible: unknown event types are skipped.
	return nil, nil
}

// FormatMessages translates a canonical conversation into an Anthropic
// Messages API request body. It handles system prompts, tool calls/results,
// image attachments, vision gating, and context window truncation.
func (a *AnthropicProvider) FormatMessages(opts FormatMessagesOpts) ([]byte, error) {
	if len(opts.Messages) == 0 {
		return nil, ErrEmptyMessages
	}

	// Truncate if a window size is specified.
	msgs := opts.Messages
	if opts.WindowSize > 0 {
		truncated, err := Truncate(msgs, opts.SystemPrompt, opts.WindowSize)
		if err != nil {
			return nil, err
		}
		msgs = truncated
	}

	hasVision := false
	for _, cap := range opts.Capabilities {
		if cap == "vision" {
			hasVision = true
			break
		}
	}

	// Build the messages array.
	wireMsgs := make([]anthropicMessage, 0, len(msgs))
	for _, m := range msgs {
		switch {
		case m.Role == "tool" || (m.Role == "user" && len(m.ToolResults) > 0):
			// Tool results are emitted under "user" role in Anthropic.
			blocks := make([]any, 0, len(m.ToolResults))
			for _, r := range m.ToolResults {
				blocks = append(blocks, a.formatToolResult(r))
			}
			// If there's also text content on the message, add it.
			if m.Content != "" {
				blocks = append([]any{anthropicTextBlock{Type: "text", Text: m.Content}}, blocks...)
			}
			wireMsgs = append(wireMsgs, anthropicMessage{Role: "user", Content: blocks})

		case m.Role == "assistant":
			blocks := a.formatAssistantContent(m, hasVision)
			wireMsgs = append(wireMsgs, anthropicMessage{Role: "assistant", Content: blocks})

		case m.Role == "user":
			blocks := a.formatUserContent(m, hasVision)
			wireMsgs = append(wireMsgs, anthropicMessage{Role: "user", Content: blocks})

		default:
			// Skip system messages here; handled via top-level system field.
		}
	}

	// Build the request body.
	body := anthropicRequestBody{
		Model:     opts.Model,
		MaxTokens: opts.MaxTokens,
		Messages:  wireMsgs,
	}

	if opts.SystemPrompt != "" {
		body.System = &opts.SystemPrompt
	}

	if opts.Temperature != nil {
		body.Temperature = opts.Temperature
	}

	if opts.Stream {
		body.Stream = &opts.Stream
	}

	// Splice tools if provided.
	if len(opts.Tools) > 0 {
		toolDefs := a.buildToolDefinitions(opts.Tools)
		body.Tools = toolDefs
	}

	return json.Marshal(body)
}

// FormatToolDefinitions translates a canonical tool catalog into the
// Anthropic tool-definitions wire shape. Returns nil for empty/nil input.
func (a *AnthropicProvider) FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	defs := a.buildToolDefinitions(tools)
	return json.Marshal(defs)
}

// --- Anthropic wire format types ---

type anthropicRequestBody struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      *string            `json:"system,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      *bool              `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // []any of content blocks
}

type anthropicTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicToolUseBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type anthropicToolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

type anthropicImageBlock struct {
	Type   string               `json:"type"`
	Source anthropicImageSource `json:"source"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// --- Helper methods ---

func (a *AnthropicProvider) formatUserContent(m Message, hasVision bool) []any {
	blocks := make([]any, 0, 1+len(m.Attachments))

	if m.Content != "" {
		blocks = append(blocks, anthropicTextBlock{Type: "text", Text: m.Content})
	}

	for _, att := range m.Attachments {
		blocks = append(blocks, a.formatAttachment(att, hasVision))
	}

	if len(blocks) == 0 {
		blocks = append(blocks, anthropicTextBlock{Type: "text", Text: ""})
	}

	return blocks
}

func (a *AnthropicProvider) formatAssistantContent(m Message, hasVision bool) []any {
	blocks := make([]any, 0, 1+len(m.ToolCalls))

	if m.Content != "" {
		blocks = append(blocks, anthropicTextBlock{Type: "text", Text: m.Content})
	}

	for _, call := range m.ToolCalls {
		blocks = append(blocks, anthropicToolUseBlock{
			Type:  "tool_use",
			ID:    call.ID,
			Name:  call.Name,
			Input: call.Input,
		})
	}

	if len(blocks) == 0 {
		blocks = append(blocks, anthropicTextBlock{Type: "text", Text: ""})
	}

	return blocks
}

func (a *AnthropicProvider) formatToolResult(r ToolResult) anthropicToolResultBlock {
	content := r.Output
	isError := false

	switch r.Status {
	case "error":
		isError = true
		content = "[error: " + r.Error + "] " + r.Output
	case "rejected":
		isError = true
		content = "[rejected: " + r.Reason + "] " + r.Output
	}

	return anthropicToolResultBlock{
		Type:      "tool_result",
		ToolUseID: r.ToolCallID,
		Content:   content,
		IsError:   isError,
	}
}

func (a *AnthropicProvider) formatAttachment(att Attachment, hasVision bool) any {
	if strings.HasPrefix(att.ContentType, "image/") {
		if !hasVision {
			return anthropicTextBlock{
				Type: "text",
				Text: "[image omitted: model lacks vision capability]",
			}
		}
		return anthropicImageBlock{
			Type: "image",
			Source: anthropicImageSource{
				Type:      "base64",
				MediaType: att.ContentType,
				Data:      att.Raw,
			},
		}
	}

	// Text attachment — use extracted Content.
	return anthropicTextBlock{
		Type: "text",
		Text: att.Content,
	}
}

func (a *AnthropicProvider) buildToolDefinitions(tools []protocol.ToolDefinition) []anthropicTool {
	defs := make([]anthropicTool, len(tools))
	for i, t := range tools {
		defs[i] = anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return defs
}
