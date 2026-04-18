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

// ParseStreamEvent decodes a single OpenAI SSE data payload. The "[DONE]"
// sentinel maps to StreamEventDone; otherwise the chunk is parsed for
// content/tool_call deltas and final usage stats. Returns (nil, nil)
// for chunks with no useful payload.
func (p *OpenAIProvider) ParseStreamEvent(data []byte) (*StreamEvent, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "[DONE]" {
		return &StreamEvent{Type: StreamEventDone}, nil
	}

	var sc struct {
		Choices []struct {
			Delta struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("openai: parse stream event: %w", err)
	}

	// Usage statistics arrive on a final chunk (with `stream_options:
	// {"include_usage": true}` enabled in the request).
	if sc.Usage != nil {
		return &StreamEvent{
			Type: StreamEventUsage,
			Usage: &StreamUsage{
				InputTokens:  sc.Usage.PromptTokens,
				OutputTokens: sc.Usage.CompletionTokens,
			},
		}, nil
	}

	if len(sc.Choices) == 0 {
		return nil, nil
	}
	choice := sc.Choices[0]

	if len(choice.Delta.ToolCalls) > 0 {
		tc := choice.Delta.ToolCalls[0]
		// OpenAI emits tool calls in pieces: the first chunk has id+name,
		// subsequent chunks add to arguments. Discriminate via id presence.
		if tc.ID != "" {
			return &StreamEvent{
				Type: StreamEventToolCallStart,
				ToolCall: &StreamToolCall{
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: tc.Function.Arguments,
				},
			}, nil
		}
		return &StreamEvent{
			Type:     StreamEventToolCallDelta,
			ToolCall: &StreamToolCall{Input: tc.Function.Arguments},
		}, nil
	}

	if choice.Delta.Content != "" {
		return &StreamEvent{Type: StreamEventText, Content: choice.Delta.Content}, nil
	}

	if choice.FinishReason != nil {
		return &StreamEvent{Type: StreamEventToolCallEnd}, nil
	}
	return nil, nil
}

// useMaxCompletionTokens returns true for model families that require
// max_completion_tokens instead of max_tokens (o1/o3/o4 reasoning models
// and gpt-5 family).
func useMaxCompletionTokens(model string) bool {
	prefixes := []string{"o1-", "o1", "o3-", "o3", "o4-", "o4", "gpt-5"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}

// hasCapability checks if a capability is present in the list.
func hasCapability(caps []string, cap string) bool {
	for _, c := range caps {
		if c == cap {
			return true
		}
	}
	return false
}

// isImageContentType returns true for image/* MIME types.
func isImageContentType(ct string) bool {
	return strings.HasPrefix(ct, "image/")
}

// openaiToolCallWire is the wire format for a single tool call.
type openaiToolCallWire struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openaiToolCallFunction `json:"function"`
}

type openaiToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// openaiMessageWire is the wire format for an OpenAI chat message.
// Content can be either a string or array of content blocks.
type openaiMessageWire struct {
	Role      string              `json:"role"`
	Content   any                 `json:"content"`
	ToolCalls []openaiToolCallWire `json:"tool_calls,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
}

// openaiContentBlock represents a content block in the array form.
type openaiContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openaiImageURL `json:"image_url,omitempty"`
}

type openaiImageURL struct {
	URL string `json:"url"`
}

// openaiRequestBody is the complete wire format for an OpenAI chat completions request.
type openaiRequestBody struct {
	Model               string              `json:"model"`
	MaxTokens           *int                `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                `json:"max_completion_tokens,omitempty"`
	Messages            []json.RawMessage   `json:"messages"`
	Tools               []openaiToolDefWire `json:"tools,omitempty"`
	Temperature         *float64            `json:"temperature,omitempty"`
	Stream              bool                `json:"stream,omitempty"`
}

// openaiToolDefWire is the wire format for an OpenAI tool definition.
type openaiToolDefWire struct {
	Type     string              `json:"type"`
	Function openaiToolDefFunc   `json:"function"`
}

type openaiToolDefFunc struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// FormatMessages translates the canonical conversation into an OpenAI
// Chat Completions request body.
func (p *OpenAIProvider) FormatMessages(opts FormatMessagesOpts) ([]byte, error) {
	if len(opts.Messages) == 0 {
		return nil, ErrEmptyMessages
	}

	// Truncate if WindowSize is set.
	msgs := opts.Messages
	if opts.WindowSize > 0 {
		truncated, err := Truncate(msgs, opts.SystemPrompt, opts.WindowSize)
		if err != nil {
			return nil, err
		}
		msgs = truncated
	}

	var wireMsgs []json.RawMessage

	// System prompt as first message.
	if opts.SystemPrompt != "" {
		sysMsg, err := json.Marshal(map[string]string{
			"role":    "system",
			"content": opts.SystemPrompt,
		})
		if err != nil {
			return nil, fmt.Errorf("openai: marshaling system message: %w", err)
		}
		wireMsgs = append(wireMsgs, sysMsg)
	}

	// Convert each canonical message.
	for _, m := range msgs {
		converted, err := p.formatMessage(m, opts.Capabilities)
		if err != nil {
			return nil, err
		}
		wireMsgs = append(wireMsgs, converted...)
	}

	// Build the request body.
	body := openaiRequestBody{
		Model:    opts.Model,
		Messages: wireMsgs,
	}

	// MaxTokens vs MaxCompletionTokens.
	if opts.MaxTokens > 0 {
		if useMaxCompletionTokens(opts.Model) {
			body.MaxCompletionTokens = &opts.MaxTokens
		} else {
			body.MaxTokens = &opts.MaxTokens
		}
	}

	if opts.Temperature != nil {
		body.Temperature = opts.Temperature
	}

	if opts.Stream {
		body.Stream = true
	}

	// Tools.
	if len(opts.Tools) > 0 {
		toolDefs, err := p.buildToolDefs(opts.Tools)
		if err != nil {
			return nil, err
		}
		body.Tools = toolDefs
	}

	return json.Marshal(body)
}

// formatMessage converts a single canonical Message into one or more
// OpenAI wire-format messages (tool results expand into multiple).
func (p *OpenAIProvider) formatMessage(m Message, caps []string) ([]json.RawMessage, error) {
	switch {
	case m.Role == "tool" || len(m.ToolResults) > 0:
		return p.formatToolResults(m)
	case m.Role == "assistant":
		return p.formatAssistant(m)
	case m.Role == "user":
		return p.formatUser(m, caps)
	default:
		// Pass through other roles as simple messages.
		raw, err := json.Marshal(map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
		return []json.RawMessage{raw}, err
	}
}

// formatUser converts a canonical user message to OpenAI wire format.
func (p *OpenAIProvider) formatUser(m Message, caps []string) ([]json.RawMessage, error) {
	if len(m.Attachments) == 0 {
		// String form.
		raw, err := json.Marshal(map[string]string{
			"role":    "user",
			"content": m.Content,
		})
		return []json.RawMessage{raw}, err
	}

	// Array form: text + attachment blocks.
	var blocks []openaiContentBlock

	if m.Content != "" {
		blocks = append(blocks, openaiContentBlock{
			Type: "text",
			Text: m.Content,
		})
	}

	for _, att := range m.Attachments {
		if isImageContentType(att.ContentType) {
			if !hasCapability(caps, "vision") {
				blocks = append(blocks, openaiContentBlock{
					Type: "text",
					Text: "[image omitted: model lacks vision capability]",
				})
			} else {
				dataURL := "data:" + att.ContentType + ";base64," + att.Raw
				blocks = append(blocks, openaiContentBlock{
					Type:     "image_url",
					ImageURL: &openaiImageURL{URL: dataURL},
				})
			}
		} else {
			// Text attachment — inline as text block.
			blocks = append(blocks, openaiContentBlock{
				Type: "text",
				Text: att.Content,
			})
		}
	}

	raw, err := json.Marshal(map[string]any{
		"role":    "user",
		"content": blocks,
	})
	return []json.RawMessage{raw}, err
}

// formatAssistant converts a canonical assistant message to OpenAI wire format.
func (p *OpenAIProvider) formatAssistant(m Message) ([]json.RawMessage, error) {
	if len(m.ToolCalls) == 0 {
		// Simple string form.
		raw, err := json.Marshal(map[string]string{
			"role":    "assistant",
			"content": m.Content,
		})
		return []json.RawMessage{raw}, err
	}

	// Build tool_calls wire format.
	var wireCalls []openaiToolCallWire
	for _, call := range m.ToolCalls {
		if !json.Valid(call.Input) {
			return nil, fmt.Errorf("openai: tool_call %q has invalid JSON input: %w", call.ID, ErrInvalidToolInput)
		}
		wireCalls = append(wireCalls, openaiToolCallWire{
			ID:   call.ID,
			Type: "function",
			Function: openaiToolCallFunction{
				Name:      call.Name,
				Arguments: string(call.Input),
			},
		})
	}

	// Build the message: content is null when empty, string when non-empty.
	msg := map[string]any{
		"role":       "assistant",
		"tool_calls": wireCalls,
	}
	if m.Content == "" {
		msg["content"] = nil
	} else {
		msg["content"] = m.Content
	}

	raw, err := json.Marshal(msg)
	return []json.RawMessage{raw}, err
}

// formatToolResults converts canonical tool results into individual
// OpenAI tool messages (one per result).
func (p *OpenAIProvider) formatToolResults(m Message) ([]json.RawMessage, error) {
	var results []json.RawMessage

	for _, r := range m.ToolResults {
		content := r.Output
		switch r.Status {
		case "error":
			content = "[error: " + r.Error + "] " + r.Output
		case "rejected":
			content = "[rejected: " + r.Reason + "] " + r.Output
		}

		raw, err := json.Marshal(map[string]string{
			"role":         "tool",
			"tool_call_id": r.ToolCallID,
			"content":      content,
		})
		if err != nil {
			return nil, fmt.Errorf("openai: marshaling tool result: %w", err)
		}
		results = append(results, raw)
	}

	return results, nil
}

// buildToolDefs converts canonical tool definitions to OpenAI wire format.
func (p *OpenAIProvider) buildToolDefs(tools []protocol.ToolDefinition) ([]openaiToolDefWire, error) {
	var defs []openaiToolDefWire
	for _, t := range tools {
		defs = append(defs, openaiToolDefWire{
			Type: "function",
			Function: openaiToolDefFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return defs, nil
}

// FormatToolDefinitions translates a canonical tool catalog into the
// OpenAI-specific tool-definitions wire shape.
func (p *OpenAIProvider) FormatToolDefinitions(tools []protocol.ToolDefinition) ([]byte, error) {
	defs, err := p.buildToolDefs(tools)
	if err != nil {
		return nil, err
	}
	return json.Marshal(defs)
}
