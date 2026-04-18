package provider

import (
	"testing"
)

func TestAnthropic_ParseStreamEvent_TextDelta(t *testing.T) {
	a := NewAnthropicProvider()
	data := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)
	ev, err := a.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventText || ev.Content != "hello" {
		t.Errorf("event = %+v", ev)
	}
}

func TestAnthropic_ParseStreamEvent_ToolUseStart(t *testing.T) {
	a := NewAnthropicProvider()
	data := []byte(`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call-a","name":"read","input":{}}}`)
	ev, err := a.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventToolCallStart {
		t.Fatalf("event = %+v", ev)
	}
	if ev.ToolCall == nil || ev.ToolCall.ID != "call-a" || ev.ToolCall.Name != "read" {
		t.Errorf("tool call = %+v", ev.ToolCall)
	}
}

func TestAnthropic_ParseStreamEvent_ToolInputDelta(t *testing.T) {
	a := NewAnthropicProvider()
	data := []byte(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\""}}`)
	ev, err := a.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventToolCallDelta {
		t.Fatalf("event = %+v", ev)
	}
	if ev.ToolCall == nil || ev.ToolCall.Input != `{"path"` {
		t.Errorf("tool call = %+v", ev.ToolCall)
	}
}

func TestAnthropic_ParseStreamEvent_MessageDeltaUsage(t *testing.T) {
	a := NewAnthropicProvider()
	data := []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}`)
	ev, err := a.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventUsage || ev.Usage == nil || ev.Usage.OutputTokens != 42 {
		t.Errorf("event = %+v", ev)
	}
}

func TestAnthropic_ParseStreamEvent_MessageStartUsage(t *testing.T) {
	a := NewAnthropicProvider()
	data := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":17}}}`)
	ev, err := a.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Usage == nil || ev.Usage.InputTokens != 17 {
		t.Errorf("event = %+v", ev)
	}
}

func TestAnthropic_ParseStreamEvent_MessageStop(t *testing.T) {
	a := NewAnthropicProvider()
	data := []byte(`{"type":"message_stop"}`)
	ev, err := a.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventDone {
		t.Errorf("event = %+v", ev)
	}
}

func TestAnthropic_ParseStreamEvent_PingSkipped(t *testing.T) {
	a := NewAnthropicProvider()
	data := []byte(`{"type":"ping"}`)
	ev, err := a.ParseStreamEvent(data)
	if err != nil || ev != nil {
		t.Errorf("ping should be skipped: ev=%+v err=%v", ev, err)
	}
}

func TestOpenAI_ParseStreamEvent_TextDelta(t *testing.T) {
	p := NewOpenAIProvider()
	data := []byte(`{"choices":[{"delta":{"content":"hello"}}]}`)
	ev, err := p.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventText || ev.Content != "hello" {
		t.Errorf("event = %+v", ev)
	}
}

func TestOpenAI_ParseStreamEvent_DoneSentinel(t *testing.T) {
	p := NewOpenAIProvider()
	ev, err := p.ParseStreamEvent([]byte("[DONE]"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventDone {
		t.Errorf("event = %+v", ev)
	}
}

func TestOpenAI_ParseStreamEvent_ToolCallStart(t *testing.T) {
	p := NewOpenAIProvider()
	data := []byte(`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","function":{"name":"read","arguments":""}}]}}]}`)
	ev, err := p.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventToolCallStart {
		t.Fatalf("event = %+v", ev)
	}
	if ev.ToolCall == nil || ev.ToolCall.ID != "call-1" || ev.ToolCall.Name != "read" {
		t.Errorf("tool call = %+v", ev.ToolCall)
	}
}

func TestOpenAI_ParseStreamEvent_ToolCallDelta(t *testing.T) {
	p := NewOpenAIProvider()
	data := []byte(`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\""}}]}}]}`)
	ev, err := p.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventToolCallDelta {
		t.Fatalf("event = %+v", ev)
	}
	if ev.ToolCall == nil || ev.ToolCall.Input != `{"path"` {
		t.Errorf("tool call = %+v", ev.ToolCall)
	}
}

func TestOpenAI_ParseStreamEvent_Usage(t *testing.T) {
	p := NewOpenAIProvider()
	data := []byte(`{"choices":[],"usage":{"prompt_tokens":12,"completion_tokens":34}}`)
	ev, err := p.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventUsage || ev.Usage.InputTokens != 12 || ev.Usage.OutputTokens != 34 {
		t.Errorf("event = %+v", ev)
	}
}

func TestOpenAI_ParseStreamEvent_FinishReason(t *testing.T) {
	p := NewOpenAIProvider()
	finish := "stop"
	data := []byte(`{"choices":[{"delta":{},"finish_reason":"` + finish + `"}]}`)
	ev, err := p.ParseStreamEvent(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev == nil || ev.Type != StreamEventToolCallEnd {
		t.Errorf("event = %+v", ev)
	}
}

func TestOpenAI_ParseStreamEvent_EmptyContentSkipped(t *testing.T) {
	p := NewOpenAIProvider()
	data := []byte(`{"choices":[{"delta":{}}]}`)
	ev, err := p.ParseStreamEvent(data)
	if err != nil || ev != nil {
		t.Errorf("empty delta should be skipped: ev=%+v err=%v", ev, err)
	}
}
