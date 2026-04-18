package provider

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// helper: unmarshal JSON bytes into map for structural comparison.
func unmarshalMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v\n%s", err, string(data))
	}
	return m
}

// helper: re-marshal and unmarshal for deep comparison.
func jsonRoundTrip(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	return out
}

func TestOpenAI_FormatMessages_SimpleTextTurn(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		SystemPrompt: "You are helpful.",
		Model:        "gpt-4-turbo",
		MaxTokens:    1024,
		WindowSize:   100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)

	// Check model
	if m["model"] != "gpt-4-turbo" {
		t.Errorf("model = %v, want gpt-4-turbo", m["model"])
	}

	// Check max_tokens
	if m["max_tokens"] != float64(1024) {
		t.Errorf("max_tokens = %v, want 1024", m["max_tokens"])
	}

	// Check messages: system + user
	msgs, ok := m["messages"].([]any)
	if !ok {
		t.Fatalf("messages is not an array: %T", m["messages"])
	}
	if len(msgs) != 2 {
		t.Fatalf("messages length = %d, want 2", len(msgs))
	}

	// System message
	sys := msgs[0].(map[string]any)
	if sys["role"] != "system" {
		t.Errorf("first message role = %v, want system", sys["role"])
	}
	if sys["content"] != "You are helpful." {
		t.Errorf("system content = %v", sys["content"])
	}

	// User message (string form, no attachments)
	usr := msgs[1].(map[string]any)
	if usr["role"] != "user" {
		t.Errorf("second message role = %v, want user", usr["role"])
	}
	if usr["content"] != "Hello" {
		t.Errorf("user content = %v, want Hello", usr["content"])
	}
}

func TestOpenAI_FormatMessages_NoSystemPrompt(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  512,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages length = %d, want 1 (no system)", len(msgs))
	}
	if msgs[0].(map[string]any)["role"] != "user" {
		t.Error("expected first message to be user when no system prompt")
	}
}

func TestOpenAI_FormatMessages_MultiTurn(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
			{Role: "assistant", Content: "I'm fine."},
			{Role: "user", Content: "Bye"},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	if len(msgs) != 5 {
		t.Fatalf("messages length = %d, want 5", len(msgs))
	}

	expectedRoles := []string{"user", "assistant", "user", "assistant", "user"}
	for i, role := range expectedRoles {
		if msgs[i].(map[string]any)["role"] != role {
			t.Errorf("message %d role = %v, want %s", i, msgs[i].(map[string]any)["role"], role)
		}
	}

	// Assistant messages should use string form
	for _, idx := range []int{1, 3} {
		content := msgs[idx].(map[string]any)["content"]
		if _, ok := content.(string); !ok {
			t.Errorf("assistant message %d content should be string, got %T", idx, content)
		}
	}
}

func TestOpenAI_FormatMessages_ToolCalls_ArgumentsDoubleEncoded(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Search for cats"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{
						ID:    "call_123",
						Name:  "search",
						Input: json.RawMessage(`{"query":"cats"}`),
					},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{
						ToolCallID: "call_123",
						Output:     "Found 5 cats",
						Status:     "success",
					},
				},
			},
			{Role: "assistant", Content: "I found 5 cats for you."},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)

	// Message 0: user
	// Message 1: assistant with tool_calls
	assistantMsg := msgs[1].(map[string]any)

	// content must be null for assistant with only tool_calls
	if assistantMsg["content"] != nil {
		t.Errorf("assistant content should be null, got %v", assistantMsg["content"])
	}

	toolCalls := assistantMsg["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("tool_calls length = %d, want 1", len(toolCalls))
	}

	tc := toolCalls[0].(map[string]any)
	if tc["id"] != "call_123" {
		t.Errorf("tool_call id = %v, want call_123", tc["id"])
	}
	if tc["type"] != "function" {
		t.Errorf("tool_call type = %v, want function", tc["type"])
	}

	fn := tc["function"].(map[string]any)
	if fn["name"] != "search" {
		t.Errorf("function name = %v, want search", fn["name"])
	}

	// arguments must be a JSON string (double-encoded), not an object
	argsVal, ok := fn["arguments"].(string)
	if !ok {
		t.Fatalf("arguments should be string (double-encoded), got %T: %v", fn["arguments"], fn["arguments"])
	}
	// The string value should be valid JSON that can be parsed
	var parsed map[string]any
	if err := json.Unmarshal([]byte(argsVal), &parsed); err != nil {
		t.Errorf("arguments string is not valid JSON: %v", err)
	}
	if parsed["query"] != "cats" {
		t.Errorf("parsed arguments query = %v, want cats", parsed["query"])
	}

	// Message 2: tool result
	toolMsg := msgs[2].(map[string]any)
	if toolMsg["role"] != "tool" {
		t.Errorf("tool message role = %v, want tool", toolMsg["role"])
	}
	if toolMsg["tool_call_id"] != "call_123" {
		t.Errorf("tool_call_id = %v, want call_123", toolMsg["tool_call_id"])
	}
	if toolMsg["content"] != "Found 5 cats" {
		t.Errorf("tool content = %v, want 'Found 5 cats'", toolMsg["content"])
	}
}

func TestOpenAI_FormatMessages_AssistantContentNull(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Do something"},
			{
				Role:    "assistant",
				Content: "", // empty content with tool calls
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "do_thing", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{ToolCallID: "call_1", Output: "done", Status: "success"},
				},
			},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	// Verify content is explicitly null in JSON
	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	assistantMsg := msgs[1].(map[string]any)

	// content key must exist and be null
	contentVal, exists := assistantMsg["content"]
	if !exists {
		t.Fatal("assistant message must have 'content' key when tool_calls present")
	}
	if contentVal != nil {
		t.Errorf("assistant content should be null, got %v (%T)", contentVal, contentVal)
	}
}

func TestOpenAI_FormatMessages_AssistantContentWithToolCalls(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Do something"},
			{
				Role:    "assistant",
				Content: "Let me help with that.",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "do_thing", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{ToolCallID: "call_1", Output: "done", Status: "success"},
				},
			},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	assistantMsg := msgs[1].(map[string]any)

	if assistantMsg["content"] != "Let me help with that." {
		t.Errorf("assistant content should be preserved when non-empty, got %v", assistantMsg["content"])
	}
	if assistantMsg["tool_calls"] == nil {
		t.Error("tool_calls should be present")
	}
}

func TestOpenAI_FormatMessages_ToolResultError(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Run tool"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_err", Name: "failing_tool", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{
						ToolCallID: "call_err",
						Output:     "partial output",
						Status:     "error",
						Error:      "timeout exceeded",
					},
				},
			},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	toolMsg := msgs[2].(map[string]any)
	content := toolMsg["content"].(string)
	if content != "[error: timeout exceeded] partial output" {
		t.Errorf("tool error content = %q, want '[error: timeout exceeded] partial output'", content)
	}
}

func TestOpenAI_FormatMessages_ToolResultRejected(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Run tool"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_rej", Name: "dangerous_tool", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{
						ToolCallID: "call_rej",
						Output:     "",
						Status:     "rejected",
						Reason:     "user denied",
					},
				},
			},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	toolMsg := msgs[2].(map[string]any)
	content := toolMsg["content"].(string)
	if content != "[rejected: user denied] " {
		t.Errorf("tool rejected content = %q, want '[rejected: user denied] '", content)
	}
}

func TestOpenAI_FormatMessages_MultipleToolResults(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Do two things"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_a", Name: "tool_a", Input: json.RawMessage(`{}`)},
					{ID: "call_b", Name: "tool_b", Input: json.RawMessage(`{}`)},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{ToolCallID: "call_a", Output: "result a", Status: "success"},
					{ToolCallID: "call_b", Output: "result b", Status: "success"},
				},
			},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)

	// Multiple tool results expand to multiple tool messages
	// user + assistant + tool_a + tool_b = 4
	if len(msgs) != 4 {
		t.Fatalf("messages length = %d, want 4 (multiple tool results expand)", len(msgs))
	}

	toolA := msgs[2].(map[string]any)
	if toolA["tool_call_id"] != "call_a" {
		t.Errorf("first tool message tool_call_id = %v, want call_a", toolA["tool_call_id"])
	}

	toolB := msgs[3].(map[string]any)
	if toolB["tool_call_id"] != "call_b" {
		t.Errorf("second tool message tool_call_id = %v, want call_b", toolB["tool_call_id"])
	}
}

func TestOpenAI_FormatMessages_ImageAttachment(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{
				Role:    "user",
				Content: "What's in this image?",
				Attachments: []Attachment{
					{
						Path:        "screenshot.png",
						Raw:         "iVBORw0KGgo=", // fake base64
						ContentType: "image/png",
					},
				},
			},
		},
		Model:        "gpt-4-turbo",
		MaxTokens:    1024,
		WindowSize:   100000,
		Capabilities: []string{"vision"},
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	userMsg := msgs[0].(map[string]any)

	// Content must be array form when attachments present
	contentArr, ok := userMsg["content"].([]any)
	if !ok {
		t.Fatalf("user content should be array when attachments present, got %T", userMsg["content"])
	}
	if len(contentArr) != 2 {
		t.Fatalf("content array length = %d, want 2 (text + image)", len(contentArr))
	}

	// First: text block
	textBlock := contentArr[0].(map[string]any)
	if textBlock["type"] != "text" {
		t.Errorf("first content block type = %v, want text", textBlock["type"])
	}
	if textBlock["text"] != "What's in this image?" {
		t.Errorf("text content = %v", textBlock["text"])
	}

	// Second: image_url block
	imgBlock := contentArr[1].(map[string]any)
	if imgBlock["type"] != "image_url" {
		t.Errorf("second content block type = %v, want image_url", imgBlock["type"])
	}
	imgURL := imgBlock["image_url"].(map[string]any)
	expectedURL := "data:image/png;base64,iVBORw0KGgo="
	if imgURL["url"] != expectedURL {
		t.Errorf("image url = %v, want %s", imgURL["url"], expectedURL)
	}
}

func TestOpenAI_FormatMessages_TextAttachment(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{
				Role:    "user",
				Content: "Review this code",
				Attachments: []Attachment{
					{
						Path:        "main.go",
						Content:     "package main\nfunc main() {}",
						ContentType: "text/plain",
					},
				},
			},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	userMsg := msgs[0].(map[string]any)

	// Content must be array form with text attachment
	contentArr := userMsg["content"].([]any)
	if len(contentArr) != 2 {
		t.Fatalf("content array length = %d, want 2 (text + attachment)", len(contentArr))
	}

	attBlock := contentArr[1].(map[string]any)
	if attBlock["type"] != "text" {
		t.Errorf("attachment block type = %v, want text", attBlock["type"])
	}
}

func TestOpenAI_FormatMessages_VisionGating(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{
				Role:    "user",
				Content: "What's this?",
				Attachments: []Attachment{
					{
						Path:        "photo.jpg",
						Raw:         "base64data",
						ContentType: "image/jpeg",
					},
				},
			},
		},
		Model:        "gpt-3.5-turbo",
		MaxTokens:    1024,
		WindowSize:   100000,
		Capabilities: []string{}, // no vision
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	userMsg := msgs[0].(map[string]any)

	// With vision gating, image should be replaced with placeholder
	contentArr := userMsg["content"].([]any)
	if len(contentArr) != 2 {
		t.Fatalf("content array length = %d, want 2", len(contentArr))
	}

	// Second block should be text placeholder, not image_url
	placeholder := contentArr[1].(map[string]any)
	if placeholder["type"] != "text" {
		t.Errorf("gated image should be text type, got %v", placeholder["type"])
	}
	text := placeholder["text"].(string)
	if text != "[image omitted: model lacks vision capability]" {
		t.Errorf("placeholder text = %q", text)
	}
}

func TestOpenAI_FormatMessages_MaxCompletionTokens_ReasoningModels(t *testing.T) {
	p := NewOpenAIProvider()

	reasoningModels := []string{
		"o1-preview",
		"o1-mini",
		"o3-mini",
		"o4-mini",
		"gpt-5",
		"gpt-5-turbo",
	}

	for _, model := range reasoningModels {
		t.Run(model, func(t *testing.T) {
			opts := FormatMessagesOpts{
				Messages: []Message{
					{Role: "user", Content: "Hi"},
				},
				Model:      model,
				MaxTokens:  2048,
				WindowSize: 100000,
			}

			got, err := p.FormatMessages(opts)
			if err != nil {
				t.Fatalf("FormatMessages() error = %v", err)
			}

			m := unmarshalMap(t, got)

			// Should have max_completion_tokens, NOT max_tokens
			if _, ok := m["max_tokens"]; ok {
				t.Error("reasoning model should not have max_tokens field")
			}
			if m["max_completion_tokens"] != float64(2048) {
				t.Errorf("max_completion_tokens = %v, want 2048", m["max_completion_tokens"])
			}
		})
	}
}

func TestOpenAI_FormatMessages_MaxTokens_LegacyModels(t *testing.T) {
	p := NewOpenAIProvider()
	legacyModels := []string{
		"gpt-4-turbo",
		"gpt-4o",
		"gpt-3.5-turbo",
		"gpt-4",
	}

	for _, model := range legacyModels {
		t.Run(model, func(t *testing.T) {
			opts := FormatMessagesOpts{
				Messages: []Message{
					{Role: "user", Content: "Hi"},
				},
				Model:      model,
				MaxTokens:  2048,
				WindowSize: 100000,
			}

			got, err := p.FormatMessages(opts)
			if err != nil {
				t.Fatalf("FormatMessages() error = %v", err)
			}

			m := unmarshalMap(t, got)

			// Should have max_tokens, NOT max_completion_tokens
			if _, ok := m["max_completion_tokens"]; ok {
				t.Error("legacy model should not have max_completion_tokens field")
			}
			if m["max_tokens"] != float64(2048) {
				t.Errorf("max_tokens = %v, want 2048", m["max_tokens"])
			}
		})
	}
}

func TestOpenAI_FormatMessages_InvalidToolInput(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Go"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_bad", Name: "tool", Input: json.RawMessage(`{invalid json}`)},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{ToolCallID: "call_bad", Output: "whatever", Status: "success"},
				},
			},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	_, err := p.FormatMessages(opts)
	if err == nil {
		t.Fatal("expected error for invalid tool input JSON")
	}
	if !errors.Is(err, ErrInvalidToolInput) {
		t.Errorf("error = %v, want ErrInvalidToolInput", err)
	}
}

func TestOpenAI_FormatMessages_EmptyMessages(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages:   []Message{},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	_, err := p.FormatMessages(opts)
	if !errors.Is(err, ErrEmptyMessages) {
		t.Errorf("error = %v, want ErrEmptyMessages", err)
	}
}

func TestOpenAI_FormatMessages_Truncation(t *testing.T) {
	p := NewOpenAIProvider()

	// Create messages that exceed a tiny window.
	// Each message is ~25 tokens (100 chars / 4). 10 messages = ~250 tokens.
	msgs := make([]Message, 10)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = Message{Role: "user", Content: "This is a user message with some content that is long enough to consume tokens in our budget estimate"}
		} else {
			msgs[i] = Message{Role: "assistant", Content: "This is an assistant response with additional content to consume more tokens in the budget estimate"}
		}
	}

	opts := FormatMessagesOpts{
		Messages:   msgs,
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 60, // small window — fits ~2-3 messages
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	resultMsgs := m["messages"].([]any)

	// Should have fewer messages than original due to truncation
	if len(resultMsgs) >= 10 {
		t.Errorf("expected truncation to reduce message count, got %d", len(resultMsgs))
	}
}

func TestOpenAI_FormatMessages_Stream(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
		Stream:     true,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	if m["stream"] != true {
		t.Errorf("stream = %v, want true", m["stream"])
	}
}

func TestOpenAI_FormatMessages_Temperature(t *testing.T) {
	p := NewOpenAIProvider()
	temp := 0.7
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:       "gpt-4-turbo",
		MaxTokens:   1024,
		WindowSize:  100000,
		Temperature: &temp,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	if m["temperature"] != 0.7 {
		t.Errorf("temperature = %v, want 0.7", m["temperature"])
	}
}

func TestOpenAI_FormatMessages_ToolCallUTF8Arguments(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Search"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call_utf8", Name: "search", Input: json.RawMessage(`{"name":"café"}`)},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{ToolCallID: "call_utf8", Output: "found", Status: "success"},
				},
			},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	msgs := m["messages"].([]any)
	assistantMsg := msgs[1].(map[string]any)
	toolCalls := assistantMsg["tool_calls"].([]any)
	tc := toolCalls[0].(map[string]any)
	fn := tc["function"].(map[string]any)

	argsStr := fn["arguments"].(string)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(argsStr), &parsed); err != nil {
		t.Fatalf("failed to parse arguments: %v", err)
	}
	if parsed["name"] != "café" {
		t.Errorf("expected café, got %v", parsed["name"])
	}
}

func TestOpenAI_FormatToolDefinitions(t *testing.T) {
	p := NewOpenAIProvider()
	tools := []protocol.ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read a file from disk",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
		},
		{
			Name:        "write_file",
			Description: "Write a file to disk",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
		},
	}

	got, err := p.FormatToolDefinitions(tools)
	if err != nil {
		t.Fatalf("FormatToolDefinitions() error = %v", err)
	}

	var toolArr []any
	if err := json.Unmarshal(got, &toolArr); err != nil {
		t.Fatalf("failed to unmarshal tools: %v", err)
	}

	if len(toolArr) != 2 {
		t.Fatalf("tools length = %d, want 2", len(toolArr))
	}

	// First tool
	tool0 := toolArr[0].(map[string]any)
	if tool0["type"] != "function" {
		t.Errorf("tool type = %v, want function", tool0["type"])
	}
	fn := tool0["function"].(map[string]any)
	if fn["name"] != "read_file" {
		t.Errorf("function name = %v, want read_file", fn["name"])
	}
	if fn["description"] != "Read a file from disk" {
		t.Errorf("function description = %v", fn["description"])
	}
	// parameters should be the raw input schema
	params := fn["parameters"].(map[string]any)
	if params["type"] != "object" {
		t.Errorf("parameters type = %v, want object", params["type"])
	}
}

func TestOpenAI_FormatToolDefinitions_Empty(t *testing.T) {
	p := NewOpenAIProvider()
	got, err := p.FormatToolDefinitions([]protocol.ToolDefinition{})
	if err != nil {
		t.Fatalf("FormatToolDefinitions() error = %v", err)
	}

	// Empty tools should return empty JSON array
	var toolArr []any
	if err := json.Unmarshal(got, &toolArr); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(toolArr) != 0 {
		t.Errorf("expected empty array, got %d elements", len(toolArr))
	}
}

func TestOpenAI_FormatMessages_WithTools(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
		Tools: []protocol.ToolDefinition{
			{
				Name:        "search",
				Description: "Search the web",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
			},
		},
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)

	// tools field should be present
	tools := m["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools length = %d, want 1", len(tools))
	}

	tool := tools[0].(map[string]any)
	if tool["type"] != "function" {
		t.Errorf("tool type = %v, want function", tool["type"])
	}
}

func TestOpenAI_FormatMessages_NoToolsField(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:      "gpt-4-turbo",
		MaxTokens:  1024,
		WindowSize: 100000,
		Tools:      nil, // no tools
	}

	got, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("FormatMessages() error = %v", err)
	}

	m := unmarshalMap(t, got)
	if _, ok := m["tools"]; ok {
		t.Error("tools field should be absent when no tools provided")
	}
}

func TestOpenAI_FormatMessages_WindowTooSmall(t *testing.T) {
	p := NewOpenAIProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		SystemPrompt: "This is a very long system prompt that takes up a lot of tokens and should exceed the tiny window size we set",
		Model:        "gpt-4-turbo",
		MaxTokens:    1024,
		WindowSize:   5, // absurdly small
	}

	_, err := p.FormatMessages(opts)
	if !errors.Is(err, ErrWindowTooSmall) {
		t.Errorf("error = %v, want ErrWindowTooSmall", err)
	}
}
