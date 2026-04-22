package protocol

import (
	"encoding/json"
	"testing"
)

// -----------------------------------------------------------------------
// WU-041: Response type round-trip tests (messages.go additions)
// -----------------------------------------------------------------------

func TestTurnSubmitResponse_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  TurnSubmitResponse
	}{
		{
			name: "accepted",
			val:  TurnSubmitResponse{TurnID: "t1", Status: "accepted"},
		},
		{
			name: "replay_with_sync",
			val: TurnSubmitResponse{
				TurnID: "t1",
				Status: "in_flight",
				Sync: &SessionSyncResponse{
					SessionID: "s1",
					ActiveTurn: ActiveTurnState{
						TurnID:               "t1",
						Status:               "streaming",
						TokenReplayAvailable: false,
						Summary:              "in progress",
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.val)
			if err != nil {
				t.Fatal(err)
			}
			var got TurnSubmitResponse
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatal(err)
			}
			if got.TurnID != tc.val.TurnID || got.Status != tc.val.Status {
				t.Errorf("scalar mismatch: got (%q,%q), want (%q,%q)", got.TurnID, got.Status, tc.val.TurnID, tc.val.Status)
			}
			if (got.Sync == nil) != (tc.val.Sync == nil) {
				t.Errorf("Sync nil mismatch")
			}
		})
	}
}

func TestTurnSubmitResponse_OmitEmpty(t *testing.T) {
	v := TurnSubmitResponse{TurnID: "t1", Status: "accepted"}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["sync"]; ok {
		t.Error("sync should be omitted when nil")
	}
}

func TestTurnCancelResponse_RoundTrip(t *testing.T) {
	v := TurnCancelResponse{TurnID: "t1", Accepted: true}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got TurnCancelResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestToolResultResponse_RoundTrip(t *testing.T) {
	v := ToolResultResponse{ToolCallID: "tc1", Accepted: true}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ToolResultResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestContentTransformResponse_RoundTrip(t *testing.T) {
	v := ContentTransformResponse{Content: "summary text", ModelUsed: "claude", Cost: 0.001}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ContentTransformResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestCapabilitiesUpdateResponse_RoundTrip(t *testing.T) {
	v := CapabilitiesUpdateResponse{AddedCount: 2, RemovedCount: 1, UpdatedAt: "2026-04-16T10:00:00Z"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got CapabilitiesUpdateResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

// -----------------------------------------------------------------------
// tools.go types
// -----------------------------------------------------------------------

func TestToolCatalog_RoundTrip(t *testing.T) {
	v := ToolCatalog{
		Tools: []ToolDefinition{
			{Name: "read", Namespace: "fs", Description: "read file", InputSchema: json.RawMessage(`{}`), OutputEnvelope: "text", RiskLevel: "low"},
		},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ToolCatalog
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Tools) != 1 || got.Tools[0].Name != "read" {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
}

func TestCapabilitiesRegisterResponse_RoundTrip(t *testing.T) {
	v := CapabilitiesRegisterResponse{
		Registered: []ToolDefinition{
			{Name: "read", Namespace: "fs", Description: "read", InputSchema: json.RawMessage(`{}`), OutputEnvelope: "text", RiskLevel: "low"},
		},
		ServerCapabilities: ServerCapabilities{
			ProtocolVersion:   "1",
			MaxFrameSize:      10485760,
			MaxAttachmentSize: 5242880,
		},
		Rejected: []RejectedTool{
			{Name: "dangerous", Reason: "not allowed"},
		},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got CapabilitiesRegisterResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Registered) != 1 || got.Registered[0].Name != "read" {
		t.Errorf("registered mismatch")
	}
	if got.ServerCapabilities.ProtocolVersion != "1" {
		t.Errorf("server capabilities mismatch")
	}
	if len(got.Rejected) != 1 || got.Rejected[0].Name != "dangerous" {
		t.Errorf("rejected mismatch")
	}
}

func TestCapabilitiesRegisterResponse_OmitEmpty(t *testing.T) {
	v := CapabilitiesRegisterResponse{
		Registered: []ToolDefinition{},
		ServerCapabilities: ServerCapabilities{
			ProtocolVersion:   "1",
			MaxFrameSize:      10485760,
			MaxAttachmentSize: 5242880,
		},
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["rejected"]; ok {
		t.Error("rejected should be omitted when nil/empty")
	}
}

func TestRejectedTool_RoundTrip(t *testing.T) {
	v := RejectedTool{Name: "bad_tool", Reason: "not supported"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got RejectedTool
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

// -----------------------------------------------------------------------
// sessions.go types
// -----------------------------------------------------------------------

func TestSessionSummary_RoundTrip(t *testing.T) {
	v := SessionSummary{
		ID:              "s1",
		Project:         "myproject",
		Status:          "active",
		Summary:         "working on feature",
		LastActive:      "2026-04-16T10:00:00Z",
		ContextPct:      45.0,
		TotalCost:       0.05,
		TurnCount:       10,
		Model:           "claude",
		LastTurnSummary: "completed task",
		FilesTouched:    []string{"main.go"},
		PinnedCount:     2,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got SessionSummary
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != v.ID || got.Project != v.Project || got.TurnCount != v.TurnCount {
		t.Errorf("round-trip mismatch on key fields")
	}
}

func TestSessionSummary_OmitEmpty(t *testing.T) {
	v := SessionSummary{
		ID:              "s1",
		Project:         "p",
		Status:          "active",
		Summary:         "s",
		LastActive:      "2026-04-16T10:00:00Z",
		ContextPct:      0,
		TotalCost:       0,
		TurnCount:       0,
		Model:           "claude",
		LastTurnSummary: "x",
		FilesTouched:    []string{},
		PinnedCount:     0,
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["model_override"]; ok {
		t.Error("model_override should be omitted when empty")
	}
}

func TestSessionListResponse_RoundTrip(t *testing.T) {
	v := SessionListResponse{
		Sessions: []SessionSummary{
			{ID: "s1", Project: "p", Status: "active", Summary: "s", LastActive: "2026-04-16T10:00:00Z", Model: "claude", LastTurnSummary: "x", FilesTouched: []string{}},
		},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got SessionListResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions) != 1 || got.Sessions[0].ID != "s1" {
		t.Errorf("round-trip mismatch")
	}
}

func TestSessionDetail_RoundTrip(t *testing.T) {
	v := SessionDetail{
		ID:            "s1",
		Summary:       "working",
		CreatedAt:     "2026-04-16T09:00:00Z",
		LastActive:    "2026-04-16T10:00:00Z",
		Model:         "claude",
		ContextPct:    50.0,
		TotalCost:     0.10,
		Turns:         []TurnSummary{{Sequence: 1, Summary: "initial", Model: "claude", Cost: 0.01}},
		FilesTouched:  []string{"main.go"},
		FilesModified: []string{"main.go"},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got SessionDetail
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != v.ID || len(got.Turns) != 1 {
		t.Errorf("round-trip mismatch")
	}
}

func TestSessionDetail_OmitEmpty(t *testing.T) {
	v := SessionDetail{
		ID: "s1", Summary: "s", CreatedAt: "t", LastActive: "t",
		Model: "m", Turns: []TurnSummary{}, FilesTouched: []string{}, FilesModified: []string{},
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["model_override"]; ok {
		t.Error("model_override should be omitted when empty")
	}
	if _, ok := m["pinned_items"]; ok {
		t.Error("pinned_items should be omitted when empty")
	}
	if _, ok := m["server_events"]; ok {
		t.Error("server_events should be omitted when empty")
	}
}

func TestTurnSummary_RoundTrip(t *testing.T) {
	v := TurnSummary{Sequence: 1, Summary: "did stuff", Compacted: false, Model: "claude", Cost: 0.01}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got TurnSummary
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Sequence != v.Sequence || got.Summary != v.Summary || got.Model != v.Model || got.Cost != v.Cost {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestTurnSummary_OmitEmpty(t *testing.T) {
	v := TurnSummary{Sequence: 1, Summary: "s", Model: "m", Cost: 0}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["original_turns"]; ok {
		t.Error("original_turns should be omitted when empty")
	}
}

func TestServerSessionEvent_RoundTrip(t *testing.T) {
	v := ServerSessionEvent{Type: "compaction", At: "2026-04-16T10:00:00Z", FreedTokens: 500, Detail: "freed tokens"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ServerSessionEvent
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != v.Type || got.At != v.At || got.Detail != v.Detail {
		t.Errorf("round-trip mismatch")
	}
}

func TestServerSessionEvent_OmitEmpty(t *testing.T) {
	v := ServerSessionEvent{Type: "x", At: "t", Detail: "d"}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["freed_tokens"]; ok {
		t.Error("freed_tokens should be omitted when zero")
	}
}

func TestSessionSyncResponse_RoundTrip(t *testing.T) {
	v := SessionSyncResponse{
		SessionID: "s1",
		ActiveTurn: ActiveTurnState{
			TurnID:               "t1",
			Status:               "streaming",
			TokenReplayAvailable: true,
			Summary:              "in progress",
		},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got SessionSyncResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.SessionID != v.SessionID || got.ActiveTurn.TurnID != v.ActiveTurn.TurnID {
		t.Errorf("round-trip mismatch")
	}
}

func TestSessionSyncResponse_MultiModel(t *testing.T) {
	v := SessionSyncResponse{
		SessionID: "s1",
		ActiveTurn: ActiveTurnState{
			TurnID:               "t1",
			Status:               "streaming",
			TokenReplayAvailable: false,
			Summary:              "branching",
		},
		MultiModel: &MultiModelState{
			Reviewers: []ReviewerState{
				{Model: "claude", Status: "complete", Tokens: 100, BranchID: "b1"},
				{Model: "llama", Status: "streaming", Tokens: 50},
			},
		},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got SessionSyncResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.MultiModel == nil {
		t.Fatal("MultiModel should not be nil")
	}
	if len(got.MultiModel.Reviewers) != 2 {
		t.Errorf("expected 2 reviewers, got %d", len(got.MultiModel.Reviewers))
	}
}

func TestActiveTurnState_PendingToolCalls(t *testing.T) {
	v := ActiveTurnState{
		TurnID: "t1",
		Status: "pending_tool_result",
		PendingToolCalls: []PendingToolCall{
			{ToolCallID: "tc1", Tool: "read", Status: "awaiting_result"},
		},
		TokenReplayAvailable: false,
		Summary:              "waiting",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ActiveTurnState
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.PendingToolCalls) != 1 || got.PendingToolCalls[0].ToolCallID != "tc1" {
		t.Errorf("pending tool calls mismatch")
	}
}

func TestActiveTurnState_OmitEmpty(t *testing.T) {
	v := ActiveTurnState{TurnID: "t1", Status: "streaming", TokenReplayAvailable: false, Summary: "s"}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["pending_tool_calls"]; ok {
		t.Error("pending_tool_calls should be omitted when empty")
	}
	if _, ok := m["completed_tokens"]; ok {
		t.Error("completed_tokens should be omitted when zero")
	}
}

func TestReviewerState_OmitEmpty(t *testing.T) {
	v := ReviewerState{Model: "claude", Status: "complete", Tokens: 100}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["branch_id"]; ok {
		t.Error("branch_id should be omitted when empty")
	}
}

func TestSessionResumeResponse_RoundTrip(t *testing.T) {
	v := SessionResumeResponse{
		SessionID: "s1",
		Model:     "claude",
		Project:   ProjectContext{Root: "/tmp", ConfigFile: "config.yaml", ConfigContent: "{}"},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got SessionResumeResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.SessionID != v.SessionID || got.Model != v.Model {
		t.Errorf("round-trip mismatch")
	}
}

func TestSessionResumeResponse_OmitEmpty(t *testing.T) {
	v := SessionResumeResponse{SessionID: "s1", Model: "m", Project: ProjectContext{Root: "/"}}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["model_override"]; ok {
		t.Error("model_override should be omitted when empty")
	}
}

func TestSessionClearResponse_RoundTrip(t *testing.T) {
	v := SessionClearResponse{ClearedTurns: 5, RetainedInStorage: true}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got SessionClearResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestSessionForkResponse_RoundTrip(t *testing.T) {
	v := SessionForkResponse{NewSessionID: "s2", OriginalSessionID: "s1"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got SessionForkResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestContextListResponse_RoundTrip(t *testing.T) {
	v := ContextListResponse{
		Files: []ContextFile{
			{Path: "main.go", SizeBytes: 1024, AttachedTurn: 1, Stale: false},
		},
		KnowledgeInjections: []KnowledgeInjection{
			{Summary: "relevant", SourceDate: "2026-01-01", Relevance: 0.9},
		},
		PinnedItems:              []string{"item1"},
		ContextTokens:            5000,
		ContextWindow:            100000,
		ContextPct:               5.0,
		SystemPromptTokens:       500,
		KnowledgeInjectionTokens: 200,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ContextListResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "main.go" {
		t.Errorf("files mismatch")
	}
	if got.ContextTokens != 5000 {
		t.Errorf("context_tokens mismatch")
	}
}

// -----------------------------------------------------------------------
// models.go types
// -----------------------------------------------------------------------

func TestModelInfo_RoundTrip(t *testing.T) {
	v := ModelInfo{
		Name:            "claude-opus",
		Provider:        "anthropic",
		Roles:           []string{"primary"},
		Capabilities:    []string{"code"},
		ContextWindow:   200000,
		CostPer1kInput:  0.015,
		CostPer1kOutput: 0.075,
		Description:     "flagship model",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ModelInfo
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != v.Name || got.ContextWindow != v.ContextWindow {
		t.Errorf("round-trip mismatch")
	}
}

func TestModelInfo_OmitEmpty(t *testing.T) {
	v := ModelInfo{
		Name: "m", Provider: "p", Roles: []string{}, Capabilities: []string{},
		Description: "d",
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["status"]; ok {
		t.Error("status should be omitted when empty")
	}
	if _, ok := m["access"]; ok {
		t.Error("access should be omitted when empty")
	}
}

func TestModelListResponse_RoundTrip(t *testing.T) {
	v := ModelListResponse{
		Models: []ModelInfo{
			{Name: "claude", Provider: "anthropic", Roles: []string{}, Capabilities: []string{}, Description: "d"},
		},
		RoutingPolicy: RoutingPolicy{
			"primary": json.RawMessage(`"claude"`),
		},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ModelListResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Models) != 1 {
		t.Errorf("models mismatch")
	}
	if _, ok := got.RoutingPolicy["primary"]; !ok {
		t.Errorf("routing_policy missing key")
	}
}

func TestModelListResponse_OmitEmpty(t *testing.T) {
	v := ModelListResponse{
		Models:        []ModelInfo{},
		RoutingPolicy: RoutingPolicy{},
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["current_override"]; ok {
		t.Error("current_override should be omitted when empty")
	}
}

func TestModelSwitchResponse_RoundTrip(t *testing.T) {
	v := ModelSwitchResponse{OverrideSet: true, Model: "claude", Reason: "override_set"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ModelSwitchResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestModelSwitchResponse_OmitEmpty(t *testing.T) {
	v := ModelSwitchResponse{OverrideSet: false, Reason: "override_cleared"}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["model"]; ok {
		t.Error("model should be omitted when empty")
	}
}

// -----------------------------------------------------------------------
// health.go types
// -----------------------------------------------------------------------

func TestHealthResponse_RoundTrip(t *testing.T) {
	v := HealthResponse{
		ServerVersion:   "0.2.0",
		ProtocolVersion: "1",
		UptimeSeconds:   3600,
		Auth:            DependencyStatus{Status: "ready"},
		Storage:         DependencyStatus{Status: "ready"},
		Capabilities:    DependencyStatus{Status: "ready"},
		Providers: map[string]ProviderStatus{
			"anthropic": {Status: "ready", Models: 3},
		},
		Routing: DependencyStatus{Status: "ready"},
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got HealthResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ServerVersion != v.ServerVersion || got.UptimeSeconds != v.UptimeSeconds {
		t.Errorf("round-trip mismatch")
	}
	if p, ok := got.Providers["anthropic"]; !ok || p.Status != "ready" {
		t.Errorf("providers mismatch")
	}
}

func TestHealthResponse_OmitEmpty(t *testing.T) {
	v := HealthResponse{
		ServerVersion: "v", ProtocolVersion: "1",
		Auth: DependencyStatus{Status: "ready"}, Storage: DependencyStatus{Status: "ready"},
		Capabilities: DependencyStatus{Status: "ready"},
		Providers:    map[string]ProviderStatus{},
		Routing:      DependencyStatus{Status: "ready"},
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["active_session"]; ok {
		t.Error("active_session should be omitted when nil")
	}
}

func TestReadyResponse_RoundTrip(t *testing.T) {
	v := ReadyResponse{Ready: true}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ReadyResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch")
	}
}

func TestDependencyStatus_RoundTrip(t *testing.T) {
	v := DependencyStatus{Status: "ready", Method: "local", Path: "/tmp/db", Reason: "ok"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got DependencyStatus
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestDependencyStatus_OmitEmpty(t *testing.T) {
	v := DependencyStatus{Status: "ready"}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	for _, key := range []string{"method", "path", "reason"} {
		if _, ok := m[key]; ok {
			t.Errorf("%s should be omitted when empty", key)
		}
	}
}

func TestProviderStatus_RoundTrip(t *testing.T) {
	v := ProviderStatus{Status: "ready", Models: 5}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ProviderStatus
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != v.Status || got.Models != v.Models {
		t.Errorf("round-trip mismatch")
	}
}

func TestProviderStatus_OmitEmpty(t *testing.T) {
	v := ProviderStatus{Status: "ready"}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["error"]; ok {
		t.Error("error should be omitted when empty")
	}
	if _, ok := m["models"]; ok {
		t.Error("models should be omitted when zero")
	}
}

func TestActiveSessionInfo_RoundTrip(t *testing.T) {
	v := ActiveSessionInfo{ID: "s1", Owner: "user1"}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ActiveSessionInfo
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestServerCapabilities_RoundTrip(t *testing.T) {
	v := ServerCapabilities{
		ProtocolVersion:      "1",
		ProtocolVersionRange: "1-3",
		SupportedTransforms:  []string{"summarize"},
		MaxFrameSize:         10485760,
		MaxAttachmentSize:    5242880,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got ServerCapabilities
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ProtocolVersion != v.ProtocolVersion || got.MaxFrameSize != v.MaxFrameSize {
		t.Errorf("round-trip mismatch")
	}
}

func TestServerCapabilities_OmitEmpty(t *testing.T) {
	v := ServerCapabilities{ProtocolVersion: "1", MaxFrameSize: 10, MaxAttachmentSize: 5}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["protocol_version_range"]; ok {
		t.Error("protocol_version_range should be omitted when empty")
	}
	if _, ok := m["supported_transforms"]; ok {
		t.Error("supported_transforms should be omitted when empty")
	}
}

// -----------------------------------------------------------------------
// errors.go types
// -----------------------------------------------------------------------

func TestDiagnosticCode_Constants(t *testing.T) {
	codes := []struct {
		code DiagnosticCode
		want string
	}{
		{DiagServiceNotRunning, "MT-CONN-001"},
		{DiagStaleSocket, "MT-CONN-002"},
		{DiagSocketPermission, "MT-CONN-003"},
		{DiagVersionMismatch, "MT-CONN-004"},
		{DiagTLSUntrusted, "MT-CONN-005"},
		{DiagAuthExpired, "MT-CONN-006"},
		{DiagStorageUnready, "MT-CONN-007"},
		{DiagSessionLocked, "MT-CONN-008"},
		{DiagProviderUnavailable, "MT-CONN-009"},
		{DiagCapabilityRegistrationFailed, "MT-CONN-010"},
		{DiagModelUnavailable, "MT-CONN-011"},
		{DiagHeartbeatTimeout, "MT-CONN-012"},
		{DiagAttachmentTooLarge, "MT-CONN-013"},
	}
	for _, tc := range codes {
		if string(tc.code) != tc.want {
			t.Errorf("DiagnosticCode %q != %q", tc.code, tc.want)
		}
	}
}

func TestDiagnostic_RoundTrip(t *testing.T) {
	v := Diagnostic{
		Code:                DiagProviderUnavailable,
		Category:            "connection",
		Cause:               "timeout",
		AutoRepairAttempted: true,
		RepairResult:        "retried",
		SuggestedCommand:    "modeltap restart",
		PathOrEndpoint:      "/api/v1",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got Diagnostic
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != v.Code || got.Category != v.Category || got.Cause != v.Cause {
		t.Errorf("round-trip mismatch on required fields")
	}
	if got.RepairResult != v.RepairResult || got.SuggestedCommand != v.SuggestedCommand {
		t.Errorf("round-trip mismatch on optional fields")
	}
}

func TestDiagnostic_OmitEmpty(t *testing.T) {
	v := Diagnostic{
		Code:     DiagServiceNotRunning,
		Category: "connection",
		Cause:    "not started",
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	for _, key := range []string{"repair_result", "suggested_command", "path_or_endpoint"} {
		if _, ok := m[key]; ok {
			t.Errorf("%s should be omitted when empty", key)
		}
	}
}

// -----------------------------------------------------------------------
// compact.go types
// -----------------------------------------------------------------------

func TestCompactCategory_RoundTrip(t *testing.T) {
	v := CompactCategory{
		Name:            "old_turns",
		TokenCount:      5000,
		ValueScore:      0.3,
		SuggestedAction: "summarize",
		SummaryPreview:  "3 old turns",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got CompactCategory
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != v.Name || got.TokenCount != v.TokenCount {
		t.Errorf("round-trip mismatch")
	}
}

func TestCompactCategory_OmitEmpty(t *testing.T) {
	v := CompactCategory{Name: "x", TokenCount: 0, ValueScore: 0, SuggestedAction: "keep"}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["summary_preview"]; ok {
		t.Error("summary_preview should be omitted when empty")
	}
}

func TestCompactFileBreakdown_RoundTrip(t *testing.T) {
	v := CompactFileBreakdown{
		Path:            "main.go",
		TokenCount:      1000,
		AttachedTurn:    3,
		Stale:           true,
		SuggestedAction: "drop",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got CompactFileBreakdown
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestCompactPlan_RoundTrip(t *testing.T) {
	v := CompactPlan{
		Categories: []CompactCategory{
			{Name: "old", TokenCount: 5000, ValueScore: 0.3, SuggestedAction: "summarize"},
		},
		FilesBreakdown: []CompactFileBreakdown{
			{Path: "main.go", TokenCount: 1000, AttachedTurn: 3, Stale: true, SuggestedAction: "drop"},
		},
		EstimatedTokensFreed: 4000,
		ContextPctBefore:     85.0,
		ContextPctAfter:      60.0,
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got CompactPlan
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Categories) != 1 || got.EstimatedTokensFreed != 4000 {
		t.Errorf("round-trip mismatch")
	}
}

func TestCompactPlan_OmitEmpty(t *testing.T) {
	v := CompactPlan{
		Categories:           []CompactCategory{},
		EstimatedTokensFreed: 0,
		ContextPctBefore:     0,
		ContextPctAfter:      0,
	}
	b, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(b, &m)
	if _, ok := m["files_breakdown"]; ok {
		t.Error("files_breakdown should be omitted when empty")
	}
}

func TestCompactApplyResponse_RoundTrip(t *testing.T) {
	v := CompactApplyResponse{
		Applied:         true,
		TokensFreed:     4000,
		ContextPctAfter: 60.0,
		Summary:         "compacted 3 turns",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got CompactApplyResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

// -----------------------------------------------------------------------
// HistoryAppendResponse and HistoryListResponse
// -----------------------------------------------------------------------

func TestHistoryAppendResponse_RoundTrip(t *testing.T) {
	v := HistoryAppendResponse{Accepted: true}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got HistoryAppendResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got != v {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, v)
	}
}

func TestHistoryListResponse_RoundTrip(t *testing.T) {
	v := HistoryListResponse{
		Entries: []HistoryEntry{
			{Content: "test command", SessionID: "s1", Timestamp: "2026-04-16T10:00:00Z"},
		},
		HasMore: true,
		Cursor:  "abc123",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got HistoryListResponse
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 1 || got.HasMore != true || got.Cursor != "abc123" {
		t.Errorf("round-trip mismatch")
	}
}
