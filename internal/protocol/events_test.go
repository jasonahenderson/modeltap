package protocol

import (
	"encoding/json"
	"testing"
)

// -----------------------------------------------------------------------
// Event method-name constants
// -----------------------------------------------------------------------

func TestEventMethodConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"TokenDelta", EventTokenDelta, "token.delta"},
		{"BranchStarted", EventBranchStarted, "branch.started"},
		{"BranchComplete", EventBranchComplete, "branch.complete"},
		{"BranchError", EventBranchError, "branch.error"},
		{"ToolCall", EventToolCall, "tool.call"},
		{"StatusUpdate", EventStatusUpdate, "status.update"},
		{"KnowledgeHit", EventKnowledgeHit, "knowledge.hit"},
		{"CostUpdate", EventCostUpdate, "cost.update"},
		{"CompactPlan", EventCompactPlan, "compact.plan"},
		{"CompactSuggest", EventCompactSuggest, "compact.suggest"},
		{"CompactNotice", EventCompactNotice, "compact.notice"},
		{"TurnComplete", EventTurnComplete, "turn.complete"},
		{"ModelSelected", EventModelSelected, "model.selected"},
		{"Error", EventError, "error"},
		{"CapabilitiesRequest", EventCapabilitiesRequest, "capabilities.request"},
		{"ConnectionPong", EventConnectionPong, "connection.pong"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("Event%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Event type round-trip tests
// -----------------------------------------------------------------------

func TestTokenDelta_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  TokenDelta
	}{
		{
			name: "full",
			val:  TokenDelta{TurnID: "t1", BranchID: "b1", Text: "hello"},
		},
		{
			name: "no_branch",
			val:  TokenDelta{TurnID: "t1", Text: "hello"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.val)
			if err != nil {
				t.Fatal(err)
			}
			var got TokenDelta
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if got != tc.val {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tc.val)
			}
		})
	}
}

func TestTokenDelta_OmitEmpty(t *testing.T) {
	v := TokenDelta{TurnID: "t1", Text: "hello"}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["branch_id"]; ok {
		t.Error("branch_id should be omitted when empty")
	}
	if _, ok := m["turn_id"]; !ok {
		t.Error("turn_id must always be present")
	}
	if _, ok := m["text"]; !ok {
		t.Error("text must always be present")
	}
}

func TestBranchStarted_RoundTrip(t *testing.T) {
	v := BranchStarted{TurnID: "t1", BranchID: "b1", Model: "claude", Provider: "anthropic"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got BranchStarted
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestBranchComplete_RoundTrip(t *testing.T) {
	v := BranchComplete{
		TurnID:            "t1",
		BranchID:          "b1",
		FinalInputTokens:  100,
		FinalOutputTokens: 50,
		Model:             "claude",
		Provider:          "anthropic",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got BranchComplete
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestBranchError_RoundTrip(t *testing.T) {
	v := BranchError{
		TurnID:         "t1",
		BranchID:       "b1",
		Error:          "timeout",
		Message:        "provider timed out",
		DiagnosticCode: DiagProviderUnavailable,
		Model:          "claude",
		Provider:       "anthropic",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got BranchError
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestToolCall_RoundTrip(t *testing.T) {
	input := json.RawMessage(`{"path":"/tmp/foo"}`)
	v := ToolCall{
		TurnID:     "t1",
		ToolCallID: "tc1",
		Tool:       "read_file",
		Namespace:  "fs",
		Input:      input,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ToolCall
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.TurnID != v.TurnID || got.ToolCallID != v.ToolCallID ||
		got.Tool != v.Tool || got.Namespace != v.Namespace ||
		string(got.Input) != string(v.Input) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestStatusUpdate_RoundTrip(t *testing.T) {
	v := StatusUpdate{
		TurnID:    "t1",
		Phase:     "routing",
		Detail:    "selecting provider",
		Timestamp: "2026-04-16T10:00:00Z",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got StatusUpdate
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestKnowledgeHit_RoundTrip(t *testing.T) {
	v := KnowledgeHit{
		TurnID:     "t1",
		Summary:    "relevant docs found",
		SourceDate: "2026-01-15",
		Relevance:  0.85,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got KnowledgeHit
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestCostUpdate_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  CostUpdate
	}{
		{
			name: "with_branch",
			val: CostUpdate{
				TurnID:       "t1",
				BranchID:     "b1",
				InputTokens:  100,
				OutputTokens: 50,
				InputCost:    0.003,
				OutputCost:   0.015,
				TotalCost:    0.018,
			},
		},
		{
			name: "no_branch",
			val: CostUpdate{
				TurnID:       "t1",
				InputTokens:  100,
				OutputTokens: 50,
				InputCost:    0.003,
				OutputCost:   0.015,
				TotalCost:    0.018,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.val)
			if err != nil {
				t.Fatal(err)
			}
			var got CostUpdate
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if got != tc.val {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tc.val)
			}
		})
	}
}

func TestCostUpdate_OmitEmpty(t *testing.T) {
	v := CostUpdate{TurnID: "t1", InputTokens: 10, OutputTokens: 5, InputCost: 0.01, OutputCost: 0.02, TotalCost: 0.03}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["branch_id"]; ok {
		t.Error("branch_id should be omitted when empty")
	}
}

func TestCompactSuggest_RoundTrip(t *testing.T) {
	v := CompactSuggest{
		TurnID:     "t1",
		ContextPct: 85.5,
		Threshold:  80.0,
		Message:    "context usage high",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got CompactSuggest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestCompactNotice_RoundTrip(t *testing.T) {
	v := CompactNotice{
		TurnID:      "t1",
		TriggeredBy: "threshold_exceeded",
		TokensFreed: 5000,
		Summary:     "compacted 3 turns",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got CompactNotice
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestTurnComplete_RoundTrip(t *testing.T) {
	v := TurnComplete{
		TurnID:            "t1",
		FinalInputTokens:  1000,
		FinalOutputTokens: 500,
		TotalCost:         0.05,
		Model:             "claude",
		Provider:          "anthropic",
		LatencyMs:         1234,
		Cancelled:         false,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got TurnComplete
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestModelSelected_RoundTrip_Single(t *testing.T) {
	v := ModelSelected{
		TurnID:   "t1",
		Model:    json.RawMessage(`"claude-opus-4-6"`),
		Provider: json.RawMessage(`"anthropic"`),
		Reason:   "default routing",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ModelSelected
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.TurnID != v.TurnID || string(got.Model) != string(v.Model) ||
		string(got.Provider) != string(v.Provider) || got.Reason != v.Reason {
		t.Errorf("round-trip mismatch")
	}
}

func TestModelSelected_IsMulti(t *testing.T) {
	tests := []struct {
		name  string
		model json.RawMessage
		want  bool
	}{
		{"single_string", json.RawMessage(`"claude"`), false},
		{"array", json.RawMessage(`["claude","llama"]`), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := ModelSelected{Model: tc.model}
			if got := ms.IsMulti(); got != tc.want {
				t.Errorf("IsMulti() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestModelSelected_SingleModel(t *testing.T) {
	ms := ModelSelected{
		Model:    json.RawMessage(`"claude"`),
		Provider: json.RawMessage(`"anthropic"`),
	}
	model, provider, err := ms.SingleModel()
	if err != nil {
		t.Fatal(err)
	}
	if model != "claude" || provider != "anthropic" {
		t.Errorf("SingleModel() = (%q, %q), want (claude, anthropic)", model, provider)
	}
}

func TestModelSelected_MultiModels(t *testing.T) {
	ms := ModelSelected{
		Model:    json.RawMessage(`["claude","llama"]`),
		Provider: json.RawMessage(`["anthropic","meta"]`),
	}
	models, providers, err := ms.MultiModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "claude" || models[1] != "llama" {
		t.Errorf("unexpected models: %v", models)
	}
	if len(providers) != 2 || providers[0] != "anthropic" || providers[1] != "meta" {
		t.Errorf("unexpected providers: %v", providers)
	}
}

func TestModelSelected_SingleModel_ErrorOnArray(t *testing.T) {
	ms := ModelSelected{
		Model:    json.RawMessage(`["claude","llama"]`),
		Provider: json.RawMessage(`["anthropic","meta"]`),
	}
	_, _, err := ms.SingleModel()
	if err == nil {
		t.Error("expected error for array model on SingleModel()")
	}
}

func TestModelSelected_MultiModels_ErrorOnString(t *testing.T) {
	ms := ModelSelected{
		Model:    json.RawMessage(`"claude"`),
		Provider: json.RawMessage(`"anthropic"`),
	}
	_, _, err := ms.MultiModels()
	if err == nil {
		t.Error("expected error for string model on MultiModels()")
	}
}

func TestServerError_RoundTrip(t *testing.T) {
	v := ServerError{
		TurnID:  "t1",
		Code:    "provider_error",
		Message: "upstream timeout",
		Diagnostic: Diagnostic{
			Code:                DiagProviderUnavailable,
			Category:            "connection",
			Cause:               "timeout",
			AutoRepairAttempted: false,
		},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ServerError
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.TurnID != v.TurnID || got.Code != v.Code || got.Message != v.Message {
		t.Errorf("round-trip mismatch on scalar fields")
	}
	if got.Diagnostic.Code != v.Diagnostic.Code {
		t.Errorf("diagnostic code mismatch: got %q, want %q", got.Diagnostic.Code, v.Diagnostic.Code)
	}
}

func TestServerError_OmitEmpty(t *testing.T) {
	v := ServerError{
		Code:    "provider_error",
		Message: "error",
		Diagnostic: Diagnostic{
			Code:     DiagProviderUnavailable,
			Category: "connection",
			Cause:    "timeout",
		},
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["turn_id"]; ok {
		t.Error("turn_id should be omitted when empty")
	}
}

func TestCapabilitiesRequestEvent_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  CapabilitiesRequestEvent
	}{
		{"with_reason", CapabilitiesRequestEvent{Reason: "reconnection"}},
		{"no_reason", CapabilitiesRequestEvent{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.val)
			if err != nil {
				t.Fatal(err)
			}
			var got CapabilitiesRequestEvent
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if got != tc.val {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tc.val)
			}
		})
	}
}

func TestCapabilitiesRequestEvent_OmitEmpty(t *testing.T) {
	v := CapabilitiesRequestEvent{}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["reason"]; ok {
		t.Error("reason should be omitted when empty")
	}
}

func TestConnectionPong_RoundTrip(t *testing.T) {
	v := ConnectionPong{}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ConnectionPong
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
}
