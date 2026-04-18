package provider

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestAnthropicFormatMessages_UserOnly(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello, Claude!"},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("model = %v, want claude-sonnet-4-20250514", result["model"])
	}
	if result["max_tokens"] != float64(1024) {
		t.Errorf("max_tokens = %v, want 1024", result["max_tokens"])
	}

	msgs, ok := result["messages"].([]any)
	if !ok {
		t.Fatalf("messages is not an array: %T", result["messages"])
	}
	if len(msgs) != 1 {
		t.Fatalf("messages length = %d, want 1", len(msgs))
	}

	msg := msgs[0].(map[string]any)
	if msg["role"] != "user" {
		t.Errorf("role = %v, want user", msg["role"])
	}

	content, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("content is not an array: %T", msg["content"])
	}
	if len(content) != 1 {
		t.Fatalf("content length = %d, want 1", len(content))
	}
	block := content[0].(map[string]any)
	if block["type"] != "text" {
		t.Errorf("content[0].type = %v, want text", block["type"])
	}
	if block["text"] != "Hello, Claude!" {
		t.Errorf("content[0].text = %v, want Hello, Claude!", block["text"])
	}

	// No system field when system prompt is empty.
	if _, exists := result["system"]; exists {
		t.Error("system field should not be present when system prompt is empty")
	}
}

func TestAnthropicFormatMessages_SystemPrompt(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		SystemPrompt: "You are a helpful assistant.",
		Model:        "claude-sonnet-4-20250514",
		MaxTokens:    1024,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result["system"] != "You are a helpful assistant." {
		t.Errorf("system = %v, want 'You are a helpful assistant.'", result["system"])
	}
}

func TestAnthropicFormatMessages_MultiTurn(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
			{Role: "assistant", Content: "Doing well!"},
			{Role: "user", Content: "Great"},
			{Role: "assistant", Content: "Glad to hear it."},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	msgs := result["messages"].([]any)
	if len(msgs) != 6 {
		t.Fatalf("messages length = %d, want 6", len(msgs))
	}

	expectedRoles := []string{"user", "assistant", "user", "assistant", "user", "assistant"}
	for i, m := range msgs {
		msg := m.(map[string]any)
		if msg["role"] != expectedRoles[i] {
			t.Errorf("messages[%d].role = %v, want %v", i, msg["role"], expectedRoles[i])
		}
	}
}

func TestAnthropicFormatMessages_ToolCallsAndResults(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "What's the weather?"},
			{
				Role:    "assistant",
				Content: "Let me check.",
				ToolCalls: []ToolCall{
					{
						ID:    "toolu_01abc",
						Name:  "get_weather",
						Input: json.RawMessage(`{"city":"London"}`),
					},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{
						ToolCallID: "toolu_01abc",
						Output:     "Sunny, 22C",
						Status:     "success",
					},
				},
			},
			{Role: "assistant", Content: "It's sunny and 22C in London!"},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	msgs := result["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("messages length = %d, want 4", len(msgs))
	}

	// Message 1 (index 1): assistant with text + tool_use.
	assistantMsg := msgs[1].(map[string]any)
	if assistantMsg["role"] != "assistant" {
		t.Errorf("msg[1].role = %v, want assistant", assistantMsg["role"])
	}
	assistantContent := assistantMsg["content"].([]any)
	if len(assistantContent) != 2 {
		t.Fatalf("msg[1].content length = %d, want 2 (text + tool_use)", len(assistantContent))
	}

	textBlock := assistantContent[0].(map[string]any)
	if textBlock["type"] != "text" {
		t.Errorf("text block type = %v, want text", textBlock["type"])
	}

	toolUseBlock := assistantContent[1].(map[string]any)
	if toolUseBlock["type"] != "tool_use" {
		t.Errorf("tool_use block type = %v, want tool_use", toolUseBlock["type"])
	}
	if toolUseBlock["id"] != "toolu_01abc" {
		t.Errorf("tool_use id = %v, want toolu_01abc", toolUseBlock["id"])
	}
	if toolUseBlock["name"] != "get_weather" {
		t.Errorf("tool_use name = %v, want get_weather", toolUseBlock["name"])
	}

	// Verify input is an object, not a string.
	inputObj, ok := toolUseBlock["input"].(map[string]any)
	if !ok {
		t.Fatalf("tool_use input is not an object: %T", toolUseBlock["input"])
	}
	if inputObj["city"] != "London" {
		t.Errorf("input.city = %v, want London", inputObj["city"])
	}

	// Message 2 (index 2): tool_result under user role.
	toolResultMsg := msgs[2].(map[string]any)
	if toolResultMsg["role"] != "user" {
		t.Errorf("tool_result msg role = %v, want user", toolResultMsg["role"])
	}
	toolResultContent := toolResultMsg["content"].([]any)
	if len(toolResultContent) != 1 {
		t.Fatalf("tool_result content length = %d, want 1", len(toolResultContent))
	}
	trBlock := toolResultContent[0].(map[string]any)
	if trBlock["type"] != "tool_result" {
		t.Errorf("tool_result block type = %v, want tool_result", trBlock["type"])
	}
	if trBlock["tool_use_id"] != "toolu_01abc" {
		t.Errorf("tool_use_id = %v, want toolu_01abc", trBlock["tool_use_id"])
	}
	if trBlock["content"] != "Sunny, 22C" {
		t.Errorf("content = %v, want Sunny, 22C", trBlock["content"])
	}
	// is_error should not be present or should be false for success.
	if isErr, ok := trBlock["is_error"]; ok && isErr == true {
		t.Error("is_error should not be true for success status")
	}
}

func TestAnthropicFormatMessages_ToolResultTriState(t *testing.T) {
	p := NewAnthropicProvider()

	tests := []struct {
		name       string
		result     ToolResult
		wantError  bool
		wantPrefix string
	}{
		{
			name: "success",
			result: ToolResult{
				ToolCallID: "toolu_01",
				Output:     "result data",
				Status:     "success",
			},
			wantError:  false,
			wantPrefix: "",
		},
		{
			name: "error status",
			result: ToolResult{
				ToolCallID: "toolu_02",
				Output:     "partial output",
				Status:     "error",
				Error:      "timeout exceeded",
			},
			wantError:  true,
			wantPrefix: "[error: timeout exceeded] ",
		},
		{
			name: "rejected status",
			result: ToolResult{
				ToolCallID: "toolu_03",
				Output:     "some output",
				Status:     "rejected",
				Reason:     "permission denied",
			},
			wantError:  true,
			wantPrefix: "[rejected: permission denied] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := FormatMessagesOpts{
				Messages: []Message{
					{Role: "user", Content: "Do something"},
					{
						Role: "assistant",
						ToolCalls: []ToolCall{
							{ID: tt.result.ToolCallID, Name: "test_tool", Input: json.RawMessage(`{}`)},
						},
					},
					{
						Role:        "tool",
						ToolResults: []ToolResult{tt.result},
					},
					{Role: "assistant", Content: "Done."},
				},
				Model:     "claude-sonnet-4-20250514",
				MaxTokens: 1024,
			}

			body, err := p.FormatMessages(opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result map[string]any
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}

			msgs := result["messages"].([]any)
			// Index 2 is the tool result message (under user role).
			trMsg := msgs[2].(map[string]any)
			trContent := trMsg["content"].([]any)
			trBlock := trContent[0].(map[string]any)

			if tt.wantError {
				isErr, ok := trBlock["is_error"]
				if !ok || isErr != true {
					t.Errorf("is_error = %v, want true", isErr)
				}
				content := trBlock["content"].(string)
				if !strings.HasPrefix(content, tt.wantPrefix) {
					t.Errorf("content = %q, want prefix %q", content, tt.wantPrefix)
				}
			} else {
				// is_error should be absent or false.
				if isErr, ok := trBlock["is_error"]; ok && isErr == true {
					t.Error("is_error should not be true for success")
				}
			}
		})
	}
}

func TestAnthropicFormatMessages_ImageAttachment_WithVision(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{
				Role:    "user",
				Content: "What's in this image?",
				Attachments: []Attachment{
					{
						Path:        "photo.png",
						Raw:         "iVBORw0KGgoAAAANSUhEUg==",
						ContentType: "image/png",
					},
				},
			},
		},
		Model:        "claude-sonnet-4-20250514",
		MaxTokens:    1024,
		Capabilities: []string{"vision"},
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	msgs := result["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].([]any)

	// Should have text block + image block.
	if len(content) != 2 {
		t.Fatalf("content length = %d, want 2", len(content))
	}

	imgBlock := content[1].(map[string]any)
	if imgBlock["type"] != "image" {
		t.Errorf("type = %v, want image", imgBlock["type"])
	}
	source := imgBlock["source"].(map[string]any)
	if source["type"] != "base64" {
		t.Errorf("source.type = %v, want base64", source["type"])
	}
	if source["media_type"] != "image/png" {
		t.Errorf("source.media_type = %v, want image/png", source["media_type"])
	}
	if source["data"] != "iVBORw0KGgoAAAANSUhEUg==" {
		t.Errorf("source.data = %v, want iVBORw0KGgoAAAANSUhEUg==", source["data"])
	}
}

func TestAnthropicFormatMessages_ImageAttachment_WithoutVision(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{
				Role:    "user",
				Content: "What's in this image?",
				Attachments: []Attachment{
					{
						Path:        "photo.png",
						Raw:         "iVBORw0KGgoAAAANSUhEUg==",
						ContentType: "image/png",
					},
				},
			},
		},
		Model:        "claude-sonnet-4-20250514",
		MaxTokens:    1024,
		Capabilities: []string{}, // no vision
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	msgs := result["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].([]any)

	// Should have text block + placeholder text block (no image).
	if len(content) != 2 {
		t.Fatalf("content length = %d, want 2", len(content))
	}

	placeholderBlock := content[1].(map[string]any)
	if placeholderBlock["type"] != "text" {
		t.Errorf("type = %v, want text", placeholderBlock["type"])
	}
	text := placeholderBlock["text"].(string)
	if !strings.Contains(text, "image omitted") {
		t.Errorf("placeholder text = %q, want to contain 'image omitted'", text)
	}
}

func TestAnthropicFormatMessages_TextAttachment(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{
				Role:    "user",
				Content: "Review this file.",
				Attachments: []Attachment{
					{
						Path:        "readme.txt",
						Content:     "This is the file content.",
						ContentType: "text/plain",
						Transform:   "text-extract",
					},
				},
			},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	msgs := result["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].([]any)

	if len(content) != 2 {
		t.Fatalf("content length = %d, want 2", len(content))
	}

	textBlock := content[1].(map[string]any)
	if textBlock["type"] != "text" {
		t.Errorf("type = %v, want text", textBlock["type"])
	}
	if textBlock["text"] != "This is the file content." {
		t.Errorf("text = %v, want 'This is the file content.'", textBlock["text"])
	}
}

func TestAnthropicFormatToolDefinitions(t *testing.T) {
	p := NewAnthropicProvider()
	tools := []protocol.ToolDefinition{
		{
			Name:        "search",
			Description: "Search the web",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		},
		{
			Name:        "read_file",
			Description: "Read a file from disk",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		},
	}

	body, err := p.FormatToolDefinitions(tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON array: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("tools length = %d, want 2", len(result))
	}

	// Check first tool.
	if result[0]["name"] != "search" {
		t.Errorf("tools[0].name = %v, want search", result[0]["name"])
	}
	if result[0]["description"] != "Search the web" {
		t.Errorf("tools[0].description = %v", result[0]["description"])
	}
	schema := result[0]["input_schema"].(map[string]any)
	if schema["type"] != "object" {
		t.Errorf("tools[0].input_schema.type = %v, want object", schema["type"])
	}
}

func TestAnthropicFormatToolDefinitions_Empty(t *testing.T) {
	p := NewAnthropicProvider()
	body, err := p.FormatToolDefinitions(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != nil {
		t.Errorf("expected nil for empty tools, got %s", body)
	}
}

func TestAnthropicFormatMessages_WithTools(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Search for Go tutorials"},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Tools: []protocol.ToolDefinition{
			{
				Name:        "search",
				Description: "Search the web",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
			},
		},
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatal("tools field not present or not an array")
	}
	if len(tools) != 1 {
		t.Fatalf("tools length = %d, want 1", len(tools))
	}

	tool := tools[0].(map[string]any)
	if tool["name"] != "search" {
		t.Errorf("tools[0].name = %v, want search", tool["name"])
	}
}

func TestAnthropicFormatMessages_NoToolsField(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Tools:     nil,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, exists := result["tools"]; exists {
		t.Error("tools field should not be present when no tools given")
	}
}

func TestAnthropicFormatMessages_Truncation(t *testing.T) {
	p := NewAnthropicProvider()

	// Create 10 messages, each with substantial content.
	msgs := make([]Message, 0, 10)
	for i := 0; i < 10; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		// Each message ~100 chars = ~25 tokens.
		content := strings.Repeat("word ", 20)
		msgs = append(msgs, Message{Role: role, Content: content})
	}

	opts := FormatMessagesOpts{
		Messages:   msgs,
		Model:      "claude-sonnet-4-20250514",
		MaxTokens:  1024,
		WindowSize: 100, // Very small window → must truncate.
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	outputMsgs := result["messages"].([]any)
	if len(outputMsgs) >= 10 {
		t.Errorf("expected truncation to reduce messages, got %d", len(outputMsgs))
	}
	if len(outputMsgs) == 0 {
		t.Error("truncation should not produce zero messages")
	}
}

func TestAnthropicFormatMessages_EmptyMessages(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages:  []Message{},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	_, err := p.FormatMessages(opts)
	if !errors.Is(err, ErrEmptyMessages) {
		t.Errorf("err = %v, want ErrEmptyMessages", err)
	}
}

func TestAnthropicFormatMessages_WindowTooSmall(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		SystemPrompt: strings.Repeat("a", 1000), // ~250 tokens system prompt.
		Model:        "claude-sonnet-4-20250514",
		MaxTokens:    1024,
		WindowSize:   10, // Way too small for the system prompt.
	}

	_, err := p.FormatMessages(opts)
	if !errors.Is(err, ErrWindowTooSmall) {
		t.Errorf("err = %v, want ErrWindowTooSmall", err)
	}
}

func TestAnthropicFormatMessages_Temperature(t *testing.T) {
	p := NewAnthropicProvider()
	temp := 0.5
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:       "claude-sonnet-4-20250514",
		MaxTokens:   1024,
		Temperature: &temp,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result["temperature"] != 0.5 {
		t.Errorf("temperature = %v, want 0.5", result["temperature"])
	}
}

func TestAnthropicFormatMessages_NoTemperature(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, exists := result["temperature"]; exists {
		t.Error("temperature field should not be present when nil")
	}
}

func TestAnthropicFormatMessages_Stream(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Stream:    true,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result["stream"] != true {
		t.Errorf("stream = %v, want true", result["stream"])
	}
}

func TestAnthropicFormatMessages_AssistantTextOnly(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	msgs := result["messages"].([]any)
	assistantMsg := msgs[1].(map[string]any)
	content := assistantMsg["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("assistant content length = %d, want 1", len(content))
	}
	block := content[0].(map[string]any)
	if block["type"] != "text" {
		t.Errorf("type = %v, want text", block["type"])
	}
	if block["text"] != "Hi there!" {
		t.Errorf("text = %v, want 'Hi there!'", block["text"])
	}
}

func TestAnthropicFormatMessages_ToolCallOnlyNoContent(t *testing.T) {
	p := NewAnthropicProvider()
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Check weather"},
			{
				Role: "assistant",
				// No Content, only ToolCalls.
				ToolCalls: []ToolCall{
					{ID: "toolu_01", Name: "weather", Input: json.RawMessage(`{"city":"NYC"}`)},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{ToolCallID: "toolu_01", Output: "Rainy", Status: "success"},
				},
			},
			{Role: "assistant", Content: "It's rainy."},
		},
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	msgs := result["messages"].([]any)
	assistantMsg := msgs[1].(map[string]any)
	content := assistantMsg["content"].([]any)
	// Should only have tool_use block, no text block.
	if len(content) != 1 {
		t.Fatalf("content length = %d, want 1 (tool_use only)", len(content))
	}
	if content[0].(map[string]any)["type"] != "tool_use" {
		t.Errorf("type = %v, want tool_use", content[0].(map[string]any)["type"])
	}
}

func TestAnthropicFormatMessages_Truncation_PairReconciliation(t *testing.T) {
	p := NewAnthropicProvider()

	// Create a conversation with a tool call/result pair early, then normal messages.
	// With a small window, the tool pair should be dropped together.
	msgs := []Message{
		{Role: "user", Content: strings.Repeat("old context ", 20)},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{ID: "toolu_old", Name: "search", Input: json.RawMessage(`{"q":"old"}`)},
			},
		},
		{
			Role: "tool",
			ToolResults: []ToolResult{
				{ToolCallID: "toolu_old", Output: strings.Repeat("old result ", 20), Status: "success"},
			},
		},
		{Role: "assistant", Content: strings.Repeat("old response ", 20)},
		{Role: "user", Content: "Recent question"},
		{Role: "assistant", Content: "Recent answer"},
	}

	opts := FormatMessagesOpts{
		Messages:   msgs,
		Model:      "claude-sonnet-4-20250514",
		MaxTokens:  1024,
		WindowSize: 50, // Very small - should keep only recent messages.
	}

	body, err := p.FormatMessages(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	outputMsgs := result["messages"].([]any)

	// After truncation and reconciliation, there should be no orphan tool_results.
	for i, m := range outputMsgs {
		msg := m.(map[string]any)
		if msg["role"] == "user" {
			contentArr, ok := msg["content"].([]any)
			if ok {
				for _, block := range contentArr {
					b := block.(map[string]any)
					if b["type"] == "tool_result" {
						// Verify the matching tool_use exists.
						toolUseID := b["tool_use_id"].(string)
						found := false
						for j := 0; j < i; j++ {
							prev := outputMsgs[j].(map[string]any)
							if prev["role"] == "assistant" {
								prevContent, ok := prev["content"].([]any)
								if ok {
									for _, pb := range prevContent {
										pBlock := pb.(map[string]any)
										if pBlock["type"] == "tool_use" && pBlock["id"] == toolUseID {
											found = true
										}
									}
								}
							}
						}
						if !found {
							t.Errorf("orphan tool_result with tool_use_id=%q found at message index %d", toolUseID, i)
						}
					}
				}
			}
		}
	}
}
