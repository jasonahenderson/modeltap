package protocol

// This file declares session-related response and payload types for
// WU-041. All types are params payloads for JSON-RPC 2.0 Response
// envelopes (protocol.go).

// SessionSummary is an element of SessionListResponse.
type SessionSummary struct {
	ID              string   `json:"id"`
	Project         string   `json:"project"`
	Status          string   `json:"status"`
	Summary         string   `json:"summary"`
	LastActive      string   `json:"last_active"`
	ContextPct      float64  `json:"context_pct"`
	TotalCost       float64  `json:"total_cost"`
	TurnCount       int      `json:"turn_count"`
	Model           string   `json:"model"`
	ModelOverride   string   `json:"model_override,omitempty"`
	LastTurnSummary string   `json:"last_turn_summary"`
	FilesTouched    []string `json:"files_touched"`
	PinnedCount     int      `json:"pinned_count"`
}

// SessionListResponse is the response to session.list.
type SessionListResponse struct {
	Sessions []SessionSummary `json:"sessions"`
}

// SessionDetail is the response to session.details. The name
// intentionally breaks the *Response suffix pattern per the track-0
// spec.
type SessionDetail struct {
	ID            string               `json:"id"`
	Summary       string               `json:"summary"`
	CreatedAt     string               `json:"created_at"`
	LastActive    string               `json:"last_active"`
	Model         string               `json:"model"`
	ModelOverride string               `json:"model_override,omitempty"`
	ContextPct    float64              `json:"context_pct"`
	TotalCost     float64              `json:"total_cost"`
	Turns         []TurnSummary        `json:"turns"`
	PinnedItems   []string             `json:"pinned_items,omitempty"`
	FilesTouched  []string             `json:"files_touched"`
	FilesModified []string             `json:"files_modified"`
	ServerEvents  []ServerSessionEvent `json:"server_events,omitempty"`
}

// TurnSummary is an element of SessionDetail.Turns.
type TurnSummary struct {
	Sequence      int     `json:"sequence"`
	Summary       string  `json:"summary"`
	Compacted     bool    `json:"compacted"`
	OriginalTurns []int   `json:"original_turns,omitempty"`
	Model         string  `json:"model"`
	Cost          float64 `json:"cost"`
}

// ServerSessionEvent is an element of SessionDetail.ServerEvents.
// Renamed from FEAT-0008's "ServerEvent" to avoid conflict with
// ServerError (events.go).
type ServerSessionEvent struct {
	Type        string `json:"type"`
	At          string `json:"at"`
	FreedTokens int    `json:"freed_tokens,omitempty"`
	Detail      string `json:"detail"`
}

// SessionSyncResponse is the response to session.sync. MultiModel is
// nil for single-model turns.
type SessionSyncResponse struct {
	SessionID  string           `json:"session_id"`
	ActiveTurn ActiveTurnState  `json:"active_turn"`
	MultiModel *MultiModelState `json:"multi_model,omitempty"`
}

// ActiveTurnState describes the in-flight turn state for session.sync.
type ActiveTurnState struct {
	TurnID               string            `json:"turn_id"`
	Status               string            `json:"status"`
	PendingToolCalls     []PendingToolCall `json:"pending_tool_calls,omitempty"`
	CompletedTokens      int               `json:"completed_tokens,omitempty"`
	TokenReplayAvailable bool              `json:"token_replay_available"`
	Summary              string            `json:"summary"`
}

// PendingToolCall describes a tool call awaiting a result.
type PendingToolCall struct {
	ToolCallID string `json:"tool_call_id"`
	Tool       string `json:"tool"`
	Status     string `json:"status"`
}

// MultiModelState is nested in SessionSyncResponse; nil for
// single-model turns.
type MultiModelState struct {
	Reviewers []ReviewerState `json:"reviewers"`
}

// ReviewerState describes a branch in a multi-model turn.
type ReviewerState struct {
	Model    string `json:"model"`
	Status   string `json:"status"`
	Tokens   int    `json:"tokens"`
	BranchID string `json:"branch_id,omitempty"`
}

// SessionCreateResponse is the response to session.create. The server
// returns the freshly-minted session id and echoes the project context
// it bound. Per PATCH-0028, the harness calls session.create on
// ConnStateReady so session-scoped RPCs (model.switch, context.list,
// etc.) work before any turn.submit has run.
type SessionCreateResponse struct {
	SessionID string         `json:"session_id"`
	Project   ProjectContext `json:"project"`
}

// SessionResumeResponse is the response to session.resume.
type SessionResumeResponse struct {
	SessionID     string         `json:"session_id"`
	Model         string         `json:"model"`
	ModelOverride string         `json:"model_override,omitempty"`
	Project       ProjectContext `json:"project"`
}

// SessionClearResponse is the response to session.clear.
type SessionClearResponse struct {
	ClearedTurns      int  `json:"cleared_turns"`
	RetainedInStorage bool `json:"retained_in_storage"`
}

// SessionForkResponse is the response to session.fork.
type SessionForkResponse struct {
	NewSessionID      string `json:"new_session_id"`
	OriginalSessionID string `json:"original_session_id"`
}

// ContextListResponse is the response to context.list.
type ContextListResponse struct {
	Files                    []ContextFile        `json:"files"`
	KnowledgeInjections      []KnowledgeInjection `json:"knowledge_injections"`
	PinnedItems              []string             `json:"pinned_items"`
	ContextTokens            int                  `json:"context_tokens"`
	ContextWindow            int                  `json:"context_window"`
	ContextPct               float64              `json:"context_pct"`
	SystemPromptTokens       int                  `json:"system_prompt_tokens"`
	KnowledgeInjectionTokens int                  `json:"knowledge_injection_tokens"`
}

// ContextFile describes a file in the context window.
type ContextFile struct {
	Path         string `json:"path"`
	SizeBytes    int    `json:"size_bytes"`
	AttachedTurn int    `json:"attached_turn"`
	Stale        bool   `json:"stale"`
}

// KnowledgeInjection describes a knowledge-layer injection in the
// context window.
type KnowledgeInjection struct {
	Summary    string  `json:"summary"`
	SourceDate string  `json:"source_date"`
	Relevance  float64 `json:"relevance"`
}
