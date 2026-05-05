package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOllama_Name(t *testing.T) {
	if NewOllamaProvider().Name() != "ollama" {
		t.Errorf("Name mismatch")
	}
}

func TestOllama_FormatMessages(t *testing.T) {
	o := NewOllamaProvider()
	body, err := o.FormatMessages(FormatMessagesOpts{
		Model:        "llama-3.1-8b",
		SystemPrompt: "be terse",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
		MaxTokens: 100,
		Stream:    true,
	})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	var req ollamaChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Model != "llama-3.1-8b" {
		t.Errorf("model = %q", req.Model)
	}
	if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Content != "hi" {
		t.Errorf("messages = %+v", req.Messages)
	}
	if !req.Stream {
		t.Errorf("stream not set")
	}
	if req.Options["num_predict"] != float64(100) {
		t.Errorf("num_predict = %v", req.Options["num_predict"])
	}
}

func TestOllama_FormatMessages_ToolResultsFlattened(t *testing.T) {
	o := NewOllamaProvider()
	body, _ := o.FormatMessages(FormatMessagesOpts{
		Model: "x",
		Messages: []Message{
			{
				Role:    "user",
				Content: "what's the file?",
				ToolResults: []ToolResult{
					{ToolCallID: "c1", Status: "success", Output: "hello world"},
				},
			},
		},
	})
	var req ollamaChatRequest
	_ = json.Unmarshal(body, &req)
	if !strings.Contains(req.Messages[0].Content, "[tool result c1") {
		t.Errorf("tool result not flattened: %q", req.Messages[0].Content)
	}
}

func TestOllama_ParseStreamEvent_Text(t *testing.T) {
	o := NewOllamaProvider()
	ev, err := o.ParseStreamEvent([]byte(`{"message":{"role":"assistant","content":"hi"},"done":false}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventText || ev.Content != "hi" {
		t.Errorf("ev = %+v", ev)
	}
}

func TestOllama_ParseStreamEvent_Done(t *testing.T) {
	o := NewOllamaProvider()
	ev, err := o.ParseStreamEvent([]byte(`{"done":true,"done_reason":"stop","prompt_eval_count":15,"eval_count":42}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventDone {
		t.Fatalf("ev = %+v", ev)
	}
	if ev.Usage == nil || ev.Usage.InputTokens != 15 || ev.Usage.OutputTokens != 42 {
		t.Errorf("usage = %+v", ev.Usage)
	}
}

func TestOllama_ParseStreamEvent_EmptySkipped(t *testing.T) {
	o := NewOllamaProvider()
	ev, err := o.ParseStreamEvent(nil)
	if err != nil || ev != nil {
		t.Errorf("expected (nil,nil); got (%+v, %v)", ev, err)
	}
}

func TestOllama_ParseResponse(t *testing.T) {
	o := NewOllamaProvider()
	body := []byte(`{"model":"llama","message":{"role":"assistant","content":"hi"},"done":true,"done_reason":"stop","prompt_eval_count":7,"eval_count":3}`)
	meta, err := o.ParseResponse(body, nil, 200)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if meta.Model != "llama" || meta.InputTokens != 7 || meta.OutputTokens != 3 || meta.StopReason != "stop" {
		t.Errorf("meta = %+v", meta)
	}
}

func TestOllama_ReassembleStream(t *testing.T) {
	o := NewOllamaProvider()
	chunks := []StreamChunk{
		{Data: []byte(`{"model":"llama","message":{"role":"assistant","content":"part1"},"done":false}`)},
		{Data: []byte(`{"model":"llama","message":{"role":"assistant","content":" part2"},"done":false}`)},
		{Data: []byte(`{"done":true,"done_reason":"stop","prompt_eval_count":5,"eval_count":10}`)},
	}
	meta, text, err := o.ReassembleStream(chunks)
	if err != nil {
		t.Fatalf("reassemble: %v", err)
	}
	if text != "part1 part2" {
		t.Errorf("text = %q", text)
	}
	if meta.InputTokens != 5 || meta.OutputTokens != 10 {
		t.Errorf("meta = %+v", meta)
	}
}
