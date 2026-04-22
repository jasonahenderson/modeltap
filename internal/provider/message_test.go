package provider

import (
	"encoding/json"
	"testing"
)

func TestMessage_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
	}{
		{
			name: "simple user message",
			msg: Message{
				Role:    "user",
				Content: "Hello, world!",
			},
		},
		{
			name: "assistant with tool calls",
			msg: Message{
				Role:    "assistant",
				Content: "Let me check that.",
				ToolCalls: []ToolCall{
					{
						ID:    "call_001",
						Name:  "search",
						Input: json.RawMessage(`{"query":"weather"}`),
					},
				},
			},
		},
		{
			name: "tool role with results",
			msg: Message{
				Role: "tool",
				ToolResults: []ToolResult{
					{
						ToolCallID: "call_001",
						Output:     "Sunny, 72F",
						Status:     "success",
					},
					{
						ToolCallID: "call_002",
						Output:     "",
						Status:     "error",
						Error:      "timeout",
					},
					{
						ToolCallID: "call_003",
						Output:     "",
						Status:     "rejected",
						Reason:     "permission denied",
					},
				},
			},
		},
		{
			name: "message with attachments",
			msg: Message{
				Role:    "user",
				Content: "See this image.",
				Attachments: []Attachment{
					{
						Path:        "images/photo.png",
						Raw:         "iVBORw0KGgo=",
						Content:     "",
						ContentType: "image/png",
						Transform:   "",
					},
					{
						Path:        "docs/readme.txt",
						Content:     "Hello from readme",
						ContentType: "text/plain",
						Transform:   "text-extract",
					},
				},
			},
		},
		{
			name: "message with metadata",
			msg: Message{
				Role:    "assistant",
				Content: "Response text",
				Metadata: map[string]any{
					"turn_id":   "t-42",
					"branch_id": "b-1",
				},
			},
		},
		{
			name: "empty message",
			msg:  Message{Role: "user"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var got Message
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			// Re-marshal and compare bytes for structural equality.
			data2, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("re-Marshal error: %v", err)
			}
			if string(data) != string(data2) {
				t.Errorf("round-trip mismatch:\n  original: %s\n  got:      %s", data, data2)
			}
		})
	}
}

func TestToolCall_JSONRoundTrip(t *testing.T) {
	tc := ToolCall{
		ID:    "tc_abc",
		Name:  "read_file",
		Input: json.RawMessage(`{"path":"/tmp/test.go","line":42}`),
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got ToolCall
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if got.ID != tc.ID {
		t.Errorf("ID = %q, want %q", got.ID, tc.ID)
	}
	if got.Name != tc.Name {
		t.Errorf("Name = %q, want %q", got.Name, tc.Name)
	}
	if string(got.Input) != string(tc.Input) {
		t.Errorf("Input = %s, want %s", got.Input, tc.Input)
	}
}

func TestToolResult_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		tr   ToolResult
	}{
		{
			name: "success",
			tr: ToolResult{
				ToolCallID: "call_1",
				Output:     "result data",
				Status:     "success",
			},
		},
		{
			name: "error",
			tr: ToolResult{
				ToolCallID: "call_2",
				Output:     "",
				Status:     "error",
				Error:      "something broke",
			},
		},
		{
			name: "rejected",
			tr: ToolResult{
				ToolCallID: "call_3",
				Output:     "",
				Status:     "rejected",
				Reason:     "not allowed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.tr)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}

			var got ToolResult
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal error: %v", err)
			}

			if got.ToolCallID != tt.tr.ToolCallID {
				t.Errorf("ToolCallID = %q, want %q", got.ToolCallID, tt.tr.ToolCallID)
			}
			if got.Status != tt.tr.Status {
				t.Errorf("Status = %q, want %q", got.Status, tt.tr.Status)
			}
			if got.Error != tt.tr.Error {
				t.Errorf("Error = %q, want %q", got.Error, tt.tr.Error)
			}
			if got.Reason != tt.tr.Reason {
				t.Errorf("Reason = %q, want %q", got.Reason, tt.tr.Reason)
			}
		})
	}
}

func TestAttachment_JSONRoundTrip(t *testing.T) {
	att := Attachment{
		Path:        "docs/file.pdf",
		Raw:         "base64data==",
		Content:     "extracted text content",
		ContentType: "application/pdf",
		Transform:   "text-extract",
	}

	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got Attachment
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if got.Path != att.Path {
		t.Errorf("Path = %q, want %q", got.Path, att.Path)
	}
	if got.Raw != att.Raw {
		t.Errorf("Raw = %q, want %q", got.Raw, att.Raw)
	}
	if got.Content != att.Content {
		t.Errorf("Content = %q, want %q", got.Content, att.Content)
	}
	if got.ContentType != att.ContentType {
		t.Errorf("ContentType = %q, want %q", got.ContentType, att.ContentType)
	}
	if got.Transform != att.Transform {
		t.Errorf("Transform = %q, want %q", got.Transform, att.Transform)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "short string",
			input: "Hi",
			want:  0, // 2 chars / 4 = 0 (integer division)
		},
		{
			name:  "four chars",
			input: "test",
			want:  1,
		},
		{
			name:  "eight chars",
			input: "hello wo",
			want:  2,
		},
		{
			name:  "100 chars",
			input: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 100 a's
			want:  25,                                                                                                     // 100/4 = 25
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			if got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d (len=%d)", tt.input, got, tt.want, len(tt.input))
			}
		})
	}
}

func TestEstimateMessageTokens(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		min  int // minimum expected tokens (we test approximate ranges)
	}{
		{
			name: "content only",
			msg: Message{
				Role:    "user",
				Content: "Hello, how are you doing today?", // 30 chars -> ~7 tokens
			},
			min: 7,
		},
		{
			name: "content with tool calls",
			msg: Message{
				Role:    "assistant",
				Content: "Let me check.",
				ToolCalls: []ToolCall{
					{
						ID:    "call_1",
						Name:  "search",
						Input: json.RawMessage(`{"query":"test query string"}`),
					},
				},
			},
			min: 3, // at least content tokens
		},
		{
			name: "content with tool results",
			msg: Message{
				Role: "tool",
				ToolResults: []ToolResult{
					{
						ToolCallID: "call_1",
						Output:     "This is a long output result from the tool execution.",
						Status:     "success",
					},
				},
			},
			min: 5, // tool result output contributes
		},
		{
			name: "content with attachments - Content counted, Raw not",
			msg: Message{
				Role:    "user",
				Content: "See attached.",
				Attachments: []Attachment{
					{
						Path:        "file.txt",
						Raw:         "dGhpcyBpcyBhIGxvbmcgYmFzZTY0IGVuY29kZWQgc3RyaW5nIHRoYXQgc2hvdWxkIG5vdCBiZSBjb3VudGVk",
						Content:     "short extracted text",
						ContentType: "text/plain",
					},
				},
			},
			min: 3, // content + attachment Content, NOT Raw
		},
		{
			name: "empty message",
			msg: Message{
				Role: "user",
			},
			min: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateMessageTokens(tt.msg)
			if got < tt.min {
				t.Errorf("EstimateMessageTokens() = %d, want >= %d", got, tt.min)
			}
		})
	}

	// Specific test: Raw bytes are NOT counted, Content IS counted.
	msgWithRaw := Message{
		Role: "user",
		Attachments: []Attachment{
			{
				Path:        "file.txt",
				Raw:         "dGhpcyBpcyBhIHZlcnkgdmVyeSBsb25nIGJhc2U2NCBlbmNvZGVkIHN0cmluZw==",
				Content:     "short",
				ContentType: "text/plain",
			},
		},
	}
	msgWithoutRaw := Message{
		Role: "user",
		Attachments: []Attachment{
			{
				Path:        "file.txt",
				Raw:         "",
				Content:     "short",
				ContentType: "text/plain",
			},
		},
	}
	tokensWithRaw := EstimateMessageTokens(msgWithRaw)
	tokensWithoutRaw := EstimateMessageTokens(msgWithoutRaw)
	if tokensWithRaw != tokensWithoutRaw {
		t.Errorf("Raw should not affect token count: withRaw=%d, withoutRaw=%d", tokensWithRaw, tokensWithoutRaw)
	}
}

func TestFormatMessagesOpts_Construction(t *testing.T) {
	// Verify that FormatMessagesOpts can be constructed with all fields.
	temp := 0.7
	opts := FormatMessagesOpts{
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
		},
		SystemPrompt: "You are helpful.",
		WindowSize:   4096,
		Model:        "claude-sonnet-4-20250514",
		MaxTokens:    1024,
		Temperature:  &temp,
		Stream:       true,
		Capabilities: []string{"vision", "tool_use"},
	}

	if len(opts.Messages) != 2 {
		t.Errorf("Messages len = %d, want 2", len(opts.Messages))
	}
	if opts.SystemPrompt != "You are helpful." {
		t.Errorf("SystemPrompt = %q", opts.SystemPrompt)
	}
	if opts.WindowSize != 4096 {
		t.Errorf("WindowSize = %d", opts.WindowSize)
	}
	if opts.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q", opts.Model)
	}
	if opts.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d", opts.MaxTokens)
	}
	if opts.Temperature == nil || *opts.Temperature != 0.7 {
		t.Errorf("Temperature = %v", opts.Temperature)
	}
	if !opts.Stream {
		t.Error("Stream = false, want true")
	}
	if len(opts.Capabilities) != 2 {
		t.Errorf("Capabilities len = %d, want 2", len(opts.Capabilities))
	}
}
