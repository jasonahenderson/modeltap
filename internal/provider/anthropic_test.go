package provider

import (
	"net/http"
	"net/url"
	"testing"
)

func TestAnthropicProvider_Name(t *testing.T) {
	p := NewAnthropicProvider()
	if got := p.Name(); got != "anthropic" {
		t.Errorf("Name() = %q, want %q", got, "anthropic")
	}
}

func TestAnthropicProvider_Detect(t *testing.T) {
	p := NewAnthropicProvider()

	tests := []struct {
		name   string
		req    *http.Request
		expect bool
	}{
		{
			name: "matches api.anthropic.com host",
			req: &http.Request{
				Host:   "api.anthropic.com",
				URL:    &url.URL{Path: "/v1/messages"},
				Header: http.Header{},
			},
			expect: true,
		},
		{
			name: "matches api.anthropic.com in URL host",
			req: &http.Request{
				URL:    &url.URL{Host: "api.anthropic.com", Path: "/v1/messages"},
				Header: http.Header{},
			},
			expect: true,
		},
		{
			name: "matches anthropic-version header",
			req: &http.Request{
				Host: "proxy.example.com",
				URL:  &url.URL{Path: "/v1/messages"},
				Header: http.Header{
					"Anthropic-Version": []string{"2023-06-01"},
				},
			},
			expect: true,
		},
		{
			name: "matches /v1/messages path with x-api-key header",
			req: &http.Request{
				Host: "proxy.example.com",
				URL:  &url.URL{Path: "/v1/messages"},
				Header: http.Header{
					"X-Api-Key": []string{"sk-ant-api03-xxxx"},
				},
			},
			expect: true,
		},
		{
			name: "matches /v1/messages subpath with x-api-key",
			req: &http.Request{
				Host: "proxy.example.com",
				URL:  &url.URL{Path: "/api/v1/messages"},
				Header: http.Header{
					"X-Api-Key": []string{"sk-ant-api03-xxxx"},
				},
			},
			expect: true,
		},
		{
			name: "no match - different host, no special headers",
			req: &http.Request{
				Host:   "api.openai.com",
				URL:    &url.URL{Path: "/v1/chat/completions"},
				Header: http.Header{},
			},
			expect: false,
		},
		{
			name: "no match - /v1/messages path but no x-api-key header",
			req: &http.Request{
				Host: "proxy.example.com",
				URL:  &url.URL{Path: "/v1/messages"},
				Header: http.Header{
					"Authorization": []string{"Bearer sk-xxxx"},
				},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Detect(tt.req)
			if got != tt.expect {
				t.Errorf("Detect() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestAnthropicProvider_ParseRequest(t *testing.T) {
	p := NewAnthropicProvider()

	tests := []struct {
		name       string
		body       string
		wantModel  string
		wantMax    int
		wantMsgs   int
		wantStream bool
		wantTemp   *float64
		wantSystem string
		wantErr    bool
	}{
		{
			name: "typical messages request",
			body: `{
				"model": "claude-sonnet-4-20250514",
				"max_tokens": 1024,
				"messages": [
					{"role": "user", "content": "Hello, Claude"}
				]
			}`,
			wantModel: "claude-sonnet-4-20250514",
			wantMax:   1024,
			wantMsgs:  1,
		},
		{
			name: "streaming request with system prompt and temperature",
			body: `{
				"model": "claude-sonnet-4-20250514",
				"max_tokens": 4096,
				"stream": true,
				"temperature": 0.7,
				"system": "You are a helpful assistant.",
				"messages": [
					{"role": "user", "content": "What is the weather?"},
					{"role": "assistant", "content": "I cannot check the weather."},
					{"role": "user", "content": "Okay, thanks."}
				]
			}`,
			wantModel:  "claude-sonnet-4-20250514",
			wantMax:    4096,
			wantMsgs:   3,
			wantStream: true,
			wantTemp:   float64Ptr(0.7),
			wantSystem: "You are a helpful assistant.",
		},
		{
			name: "multi-turn conversation",
			body: `{
				"model": "claude-opus-4-20250514",
				"max_tokens": 2048,
				"messages": [
					{"role": "user", "content": "Hi"},
					{"role": "assistant", "content": "Hello!"},
					{"role": "user", "content": "How are you?"},
					{"role": "assistant", "content": "I'm doing well!"},
					{"role": "user", "content": "Great"}
				]
			}`,
			wantModel: "claude-opus-4-20250514",
			wantMax:   2048,
			wantMsgs:  5,
		},
		{
			name:    "invalid JSON",
			body:    `{not json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := p.ParseRequest([]byte(tt.body), http.Header{})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", meta.Model, tt.wantModel)
			}
			if meta.MaxTokens != tt.wantMax {
				t.Errorf("MaxTokens = %d, want %d", meta.MaxTokens, tt.wantMax)
			}
			if meta.Messages != tt.wantMsgs {
				t.Errorf("Messages = %d, want %d", meta.Messages, tt.wantMsgs)
			}
			if meta.Stream != tt.wantStream {
				t.Errorf("Stream = %v, want %v", meta.Stream, tt.wantStream)
			}
			if tt.wantTemp != nil {
				if meta.Temperature == nil {
					t.Error("Temperature = nil, want non-nil")
				} else if *meta.Temperature != *tt.wantTemp {
					t.Errorf("Temperature = %f, want %f", *meta.Temperature, *tt.wantTemp)
				}
			}
			if meta.SystemPrompt != tt.wantSystem {
				t.Errorf("SystemPrompt = %q, want %q", meta.SystemPrompt, tt.wantSystem)
			}
		})
	}
}

func TestAnthropicProvider_ParseResponse(t *testing.T) {
	p := NewAnthropicProvider()

	tests := []struct {
		name       string
		body       string
		statusCode int
		wantModel  string
		wantIn     int64
		wantOut    int64
		wantStop   string
		wantErr    bool
	}{
		{
			name: "successful non-streaming response",
			body: `{
				"id": "msg_01XFDUDYJgAACzvnptvVoYEL",
				"type": "message",
				"role": "assistant",
				"content": [
					{
						"type": "text",
						"text": "Hello! How can I help you today?"
					}
				],
				"model": "claude-sonnet-4-20250514",
				"stop_reason": "end_turn",
				"stop_sequence": null,
				"usage": {
					"input_tokens": 25,
					"output_tokens": 150
				}
			}`,
			statusCode: 200,
			wantModel:  "claude-sonnet-4-20250514",
			wantIn:     25,
			wantOut:    150,
			wantStop:   "end_turn",
		},
		{
			name: "response with max_tokens stop reason",
			body: `{
				"id": "msg_abc123",
				"type": "message",
				"role": "assistant",
				"content": [{"type": "text", "text": "..."}],
				"model": "claude-opus-4-20250514",
				"stop_reason": "max_tokens",
				"usage": {
					"input_tokens": 500,
					"output_tokens": 4096
				}
			}`,
			statusCode: 200,
			wantModel:  "claude-opus-4-20250514",
			wantIn:     500,
			wantOut:    4096,
			wantStop:   "max_tokens",
		},
		{
			name:    "invalid JSON",
			body:    `{broken`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := p.ParseResponse([]byte(tt.body), http.Header{}, tt.statusCode)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", meta.Model, tt.wantModel)
			}
			if meta.InputTokens != tt.wantIn {
				t.Errorf("InputTokens = %d, want %d", meta.InputTokens, tt.wantIn)
			}
			if meta.OutputTokens != tt.wantOut {
				t.Errorf("OutputTokens = %d, want %d", meta.OutputTokens, tt.wantOut)
			}
			if meta.StopReason != tt.wantStop {
				t.Errorf("StopReason = %q, want %q", meta.StopReason, tt.wantStop)
			}
		})
	}
}

func TestAnthropicProvider_ReassembleStream(t *testing.T) {
	p := NewAnthropicProvider()

	tests := []struct {
		name     string
		chunks   []StreamChunk
		wantMeta *ResponseMetadata
		wantText string
		wantErr  bool
	}{
		{
			name: "full streaming conversation",
			chunks: []StreamChunk{
				{
					EventType: "message_start",
					Data: []byte(`{
						"type": "message_start",
						"message": {
							"id": "msg_01XFDUDYJgAACzvnptvVoYEL",
							"type": "message",
							"role": "assistant",
							"content": [],
							"model": "claude-sonnet-4-20250514",
							"stop_reason": null,
							"stop_sequence": null,
							"usage": {
								"input_tokens": 25,
								"output_tokens": 0
							}
						}
					}`),
				},
				{
					EventType: "content_block_start",
					Data:      []byte(`{"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}`),
				},
				{
					EventType: "ping",
					Data:      []byte(`{"type": "ping"}`),
				},
				{
					EventType: "content_block_delta",
					Data:      []byte(`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}`),
				},
				{
					EventType: "content_block_delta",
					Data:      []byte(`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "! How can"}}`),
				},
				{
					EventType: "content_block_delta",
					Data:      []byte(`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": " I help you"}}`),
				},
				{
					EventType: "content_block_delta",
					Data:      []byte(`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": " today?"}}`),
				},
				{
					EventType: "content_block_stop",
					Data:      []byte(`{"type": "content_block_stop", "index": 0}`),
				},
				{
					EventType: "message_delta",
					Data:      []byte(`{"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}, "usage": {"output_tokens": 12}}`),
				},
				{
					EventType: "message_stop",
					Data:      []byte(`{"type": "message_stop"}`),
				},
			},
			wantMeta: &ResponseMetadata{
				Model:        "claude-sonnet-4-20250514",
				InputTokens:  25,
				OutputTokens: 12,
				StopReason:   "end_turn",
			},
			wantText: "Hello! How can I help you today?",
		},
		{
			name: "stream with max_tokens stop",
			chunks: []StreamChunk{
				{
					EventType: "message_start",
					Data: []byte(`{
						"type": "message_start",
						"message": {
							"id": "msg_abc123",
							"type": "message",
							"role": "assistant",
							"content": [],
							"model": "claude-opus-4-20250514",
							"stop_reason": null,
							"usage": {"input_tokens": 100, "output_tokens": 0}
						}
					}`),
				},
				{
					EventType: "content_block_start",
					Data:      []byte(`{"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}`),
				},
				{
					EventType: "content_block_delta",
					Data:      []byte(`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Once upon a time"}}`),
				},
				{
					EventType: "content_block_stop",
					Data:      []byte(`{"type": "content_block_stop", "index": 0}`),
				},
				{
					EventType: "message_delta",
					Data:      []byte(`{"type": "message_delta", "delta": {"stop_reason": "max_tokens"}, "usage": {"output_tokens": 4096}}`),
				},
				{
					EventType: "message_stop",
					Data:      []byte(`{"type": "message_stop"}`),
				},
			},
			wantMeta: &ResponseMetadata{
				Model:        "claude-opus-4-20250514",
				InputTokens:  100,
				OutputTokens: 4096,
				StopReason:   "max_tokens",
			},
			wantText: "Once upon a time",
		},
		{
			name:   "empty chunks",
			chunks: []StreamChunk{},
			wantMeta: &ResponseMetadata{
				Model:        "",
				InputTokens:  0,
				OutputTokens: 0,
				StopReason:   "",
			},
			wantText: "",
		},
		{
			name: "invalid message_start JSON",
			chunks: []StreamChunk{
				{
					EventType: "message_start",
					Data:      []byte(`{broken`),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid content_block_delta JSON",
			chunks: []StreamChunk{
				{
					EventType: "content_block_delta",
					Data:      []byte(`{broken`),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid message_delta JSON",
			chunks: []StreamChunk{
				{
					EventType: "message_delta",
					Data:      []byte(`{broken`),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, text, err := p.ReassembleStream(tt.chunks)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.Model != tt.wantMeta.Model {
				t.Errorf("Model = %q, want %q", meta.Model, tt.wantMeta.Model)
			}
			if meta.InputTokens != tt.wantMeta.InputTokens {
				t.Errorf("InputTokens = %d, want %d", meta.InputTokens, tt.wantMeta.InputTokens)
			}
			if meta.OutputTokens != tt.wantMeta.OutputTokens {
				t.Errorf("OutputTokens = %d, want %d", meta.OutputTokens, tt.wantMeta.OutputTokens)
			}
			if meta.StopReason != tt.wantMeta.StopReason {
				t.Errorf("StopReason = %q, want %q", meta.StopReason, tt.wantMeta.StopReason)
			}
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
		})
	}
}

// TestAnthropicProvider_ImplementsInterface verifies the provider satisfies the interface at compile time.
var _ Provider = (*AnthropicProvider)(nil)

func float64Ptr(f float64) *float64 {
	return &f
}
