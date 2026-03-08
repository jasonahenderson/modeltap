package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newOpenAI() *OpenAIProvider {
	return NewOpenAIProvider()
}

func TestOpenAIProviderName(t *testing.T) {
	p := newOpenAI()
	if got := p.Name(); got != "openai" {
		t.Errorf("Name() = %q, want %q", got, "openai")
	}
}

func TestOpenAIDetect(t *testing.T) {
	p := newOpenAI()

	tests := []struct {
		name   string
		url    string
		headers map[string]string
		want   bool
	}{
		{
			name: "host api.openai.com",
			url:  "https://api.openai.com/v1/chat/completions",
			want: true,
		},
		{
			name: "host api.openai.com with different path",
			url:  "https://api.openai.com/v1/embeddings",
			want: true,
		},
		{
			name:    "path /v1/chat/completions with Bearer token",
			url:     "http://localhost:8080/v1/chat/completions",
			headers: map[string]string{"Authorization": "Bearer sk-test123"},
			want:    true,
		},
		{
			name:    "path /v1/chat/completions without auth header",
			url:     "http://localhost:8080/v1/chat/completions",
			headers: map[string]string{},
			want:    true,
		},
		{
			name:    "path /v1/chat/completions with anthropic-version header should not match",
			url:     "http://localhost:8080/v1/chat/completions",
			headers: map[string]string{"Anthropic-Version": "2024-01-01"},
			want:    false,
		},
		{
			name: "unrelated host and path",
			url:  "https://api.anthropic.com/v1/messages",
			want: false,
		},
		{
			name: "unrelated host with unrelated path",
			url:  "https://example.com/v1/embeddings",
			want: false,
		},
		{
			name: "nested path containing /v1/chat/completions",
			url:  "http://myproxy.com/api/v1/chat/completions",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.url, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if got := p.Detect(req); got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpenAIParseRequest(t *testing.T) {
	p := newOpenAI()

	tests := []struct {
		name          string
		body          string
		wantModel     string
		wantMaxTokens int
		wantMessages  int
		wantStream    bool
		wantErr       bool
	}{
		{
			name: "basic chat completions request",
			body: `{
				"model": "gpt-4o",
				"max_tokens": 1024,
				"messages": [
					{"role": "system", "content": "You are a helpful assistant."},
					{"role": "user", "content": "Hello!"}
				]
			}`,
			wantModel:     "gpt-4o",
			wantMaxTokens: 1024,
			wantMessages:  2,
			wantStream:    false,
		},
		{
			name: "streaming request with max_completion_tokens",
			body: `{
				"model": "gpt-4o-mini",
				"max_completion_tokens": 2048,
				"stream": true,
				"messages": [
					{"role": "user", "content": "Tell me a story."}
				],
				"stream_options": {"include_usage": true}
			}`,
			wantModel:     "gpt-4o-mini",
			wantMaxTokens: 2048,
			wantMessages:  1,
			wantStream:    true,
		},
		{
			name: "request with temperature",
			body: `{
				"model": "gpt-3.5-turbo",
				"max_tokens": 500,
				"temperature": 0.7,
				"messages": [
					{"role": "user", "content": "Hi"}
				]
			}`,
			wantModel:     "gpt-3.5-turbo",
			wantMaxTokens: 500,
			wantMessages:  1,
			wantStream:    false,
		},
		{
			name:    "invalid JSON",
			body:    `{invalid`,
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
			if meta.MaxTokens != tt.wantMaxTokens {
				t.Errorf("MaxTokens = %d, want %d", meta.MaxTokens, tt.wantMaxTokens)
			}
			if meta.Messages != tt.wantMessages {
				t.Errorf("Messages = %d, want %d", meta.Messages, tt.wantMessages)
			}
			if meta.Stream != tt.wantStream {
				t.Errorf("Stream = %v, want %v", meta.Stream, tt.wantStream)
			}
		})
	}
}

func TestOpenAIParseResponse(t *testing.T) {
	p := newOpenAI()

	tests := []struct {
		name             string
		body             string
		statusCode       int
		wantModel        string
		wantInputTokens  int64
		wantOutputTokens int64
		wantStopReason   string
		wantErr          bool
	}{
		{
			name: "successful chat completion response",
			body: `{
				"id": "chatcmpl-abc123",
				"object": "chat.completion",
				"created": 1677858242,
				"model": "gpt-4o-2024-08-06",
				"usage": {
					"prompt_tokens": 13,
					"completion_tokens": 7,
					"total_tokens": 20
				},
				"choices": [
					{
						"message": {
							"role": "assistant",
							"content": "This is a test!"
						},
						"finish_reason": "stop",
						"index": 0
					}
				]
			}`,
			statusCode:       200,
			wantModel:        "gpt-4o-2024-08-06",
			wantInputTokens:  13,
			wantOutputTokens: 7,
			wantStopReason:   "stop",
		},
		{
			name: "response with length finish reason",
			body: `{
				"id": "chatcmpl-xyz789",
				"object": "chat.completion",
				"created": 1677858242,
				"model": "gpt-4o-mini",
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 4096,
					"total_tokens": 4196
				},
				"choices": [
					{
						"message": {
							"role": "assistant",
							"content": "Long response..."
						},
						"finish_reason": "length",
						"index": 0
					}
				]
			}`,
			statusCode:       200,
			wantModel:        "gpt-4o-mini",
			wantInputTokens:  100,
			wantOutputTokens: 4096,
			wantStopReason:   "length",
		},
		{
			name:    "invalid JSON",
			body:    `{not valid json`,
			statusCode: 200,
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
			if meta.InputTokens != tt.wantInputTokens {
				t.Errorf("InputTokens = %d, want %d", meta.InputTokens, tt.wantInputTokens)
			}
			if meta.OutputTokens != tt.wantOutputTokens {
				t.Errorf("OutputTokens = %d, want %d", meta.OutputTokens, tt.wantOutputTokens)
			}
			if meta.StopReason != tt.wantStopReason {
				t.Errorf("StopReason = %q, want %q", meta.StopReason, tt.wantStopReason)
			}
		})
	}
}

func TestOpenAIReassembleStream(t *testing.T) {
	p := newOpenAI()

	tests := []struct {
		name             string
		chunks           []StreamChunk
		wantText         string
		wantStopReason   string
		wantInputTokens  int64
		wantOutputTokens int64
		wantModel        string
		wantErr          bool
	}{
		{
			name: "basic streaming response",
			chunks: []StreamChunk{
				{Data: []byte(`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)},
				{Data: []byte(`data: [DONE]`)},
			},
			wantText:       "Hello world!",
			wantStopReason: "stop",
			wantModel:      "gpt-4o",
		},
		{
			name: "streaming with usage in final chunk",
			chunks: []StreamChunk{
				{Data: []byte(`data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-xyz","object":"chat.completion.chunk","model":"gpt-4o-mini","choices":[],"usage":{"prompt_tokens":25,"completion_tokens":2,"total_tokens":27}}`)},
				{Data: []byte(`data: [DONE]`)},
			},
			wantText:         "Hi!",
			wantStopReason:   "stop",
			wantModel:        "gpt-4o-mini",
			wantInputTokens:  25,
			wantOutputTokens: 2,
		},
		{
			name: "streaming with length stop reason",
			chunks: []StreamChunk{
				{Data: []byte(`data: {"id":"chatcmpl-len","object":"chat.completion.chunk","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-len","object":"chat.completion.chunk","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"truncated"},"finish_reason":null}]}`)},
				{Data: []byte(`data: {"id":"chatcmpl-len","object":"chat.completion.chunk","model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":"length"}]}`)},
				{Data: []byte(`data: [DONE]`)},
			},
			wantText:       "truncated",
			wantStopReason: "length",
			wantModel:      "gpt-3.5-turbo",
		},
		{
			name:     "empty chunks list",
			chunks:   []StreamChunk{},
			wantText: "",
		},
		{
			name: "only DONE chunk",
			chunks: []StreamChunk{
				{Data: []byte(`data: [DONE]`)},
			},
			wantText: "",
		},
		{
			name: "multiline data in single chunk",
			chunks: []StreamChunk{
				{Data: []byte("data: {\"id\":\"chatcmpl-m\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"A\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"chatcmpl-m\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"B\"},\"finish_reason\":null}]}")},
				{Data: []byte(`data: {"id":"chatcmpl-m","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)},
				{Data: []byte(`data: [DONE]`)},
			},
			wantText:       "AB",
			wantStopReason: "stop",
			wantModel:      "gpt-4o",
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
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
			if meta.StopReason != tt.wantStopReason {
				t.Errorf("StopReason = %q, want %q", meta.StopReason, tt.wantStopReason)
			}
			if tt.wantModel != "" && meta.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", meta.Model, tt.wantModel)
			}
			if tt.wantInputTokens != 0 && meta.InputTokens != tt.wantInputTokens {
				t.Errorf("InputTokens = %d, want %d", meta.InputTokens, tt.wantInputTokens)
			}
			if tt.wantOutputTokens != 0 && meta.OutputTokens != tt.wantOutputTokens {
				t.Errorf("OutputTokens = %d, want %d", meta.OutputTokens, tt.wantOutputTokens)
			}
		})
	}
}
