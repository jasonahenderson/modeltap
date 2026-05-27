package protocol

// Run lifecycle constants are protocol wire values.
const (
	RunStatusQueued            = "queued"
	RunStatusRunning           = "running"
	RunStatusWaitingPermission = "waiting_permission"
	RunStatusWaitingUser       = "waiting_user"
	RunStatusCheckpointed      = "checkpointed"
	RunStatusCompleted         = "completed"
	RunStatusFailed            = "failed"
	RunStatusCancelled         = "cancelled"

	RunStagePreflight       = "preflight"
	RunStageContextPlan     = "context_plan"
	RunStagePromptPlan      = "prompt_plan"
	RunStageModelCall       = "model_call"
	RunStageToolLoop        = "tool_loop"
	RunStageValidation      = "validation"
	RunStageArtifactCapture = "artifact_capture"
	RunStageCheckpoint      = "checkpoint"
	RunStageCompletion      = "completion"

	RunAttachmentAttached = "attached"
	RunAttachmentDetached = "detached"

	RunReplayFull    = "full"
	RunReplaySummary = "summary"
)

// RunCreate creates a queued durable run without provider dispatch.
type RunCreate struct {
	SessionID      string `json:"session_id"`
	IdempotencyKey string `json:"idempotency_key"`
	WorkflowType   string `json:"workflow_type,omitempty"`
	Title          string `json:"title,omitempty"`
	ParentRunID    string `json:"parent_run_id,omitempty"`
}

// RunList requests run summaries.
type RunList struct {
	SessionID string `json:"session_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

// RunControl addresses a run-native control command.
type RunControl struct {
	RunID  string `json:"run_id"`
	Reason string `json:"reason,omitempty"`
}

// RunDetails requests one run detail view.
type RunDetails struct {
	RunID string `json:"run_id"`
}

// RunAttach claims attachment for a run and asks for event replay.
type RunAttach struct {
	RunID           string `json:"run_id"`
	LastObservedSeq int64  `json:"last_observed_seq,omitempty"`
}

// RunDetach releases the attachment lease for a run.
type RunDetach struct {
	RunID string `json:"run_id"`
}

// RunEvents requests ordered run events after a sequence number.
type RunEvents struct {
	RunID    string `json:"run_id"`
	AfterSeq int64  `json:"after_seq"`
	Limit    int    `json:"limit,omitempty"`
}

// RunPermissions lists blockers for a run or active session.
type RunPermissions struct {
	RunID     string `json:"run_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// RunResolvePermission applies a decision to a run-correlated permission.
type RunResolvePermission struct {
	RunID     string `json:"run_id"`
	RequestID string `json:"request_id"`
	Decision  string `json:"decision"`
}

// RunHeartbeat reports attached harness or executor liveness.
type RunHeartbeat struct {
	RunID           string `json:"run_id"`
	HostFingerprint string `json:"host_fingerprint,omitempty"`
	LastObservedSeq int64  `json:"last_observed_seq,omitempty"`
	Stage           string `json:"stage,omitempty"`
}

// RunListResponse is the result of run.list.
type RunListResponse struct {
	Runs []RunSummary `json:"runs"`
}

// RunSummary is the compact list/detail projection.
type RunSummary struct {
	RunID            string  `json:"run_id"`
	TraceID          string  `json:"trace_id,omitempty"`
	SessionID        string  `json:"session_id"`
	ParentRunID      string  `json:"parent_run_id,omitempty"`
	Title            string  `json:"title"`
	WorkflowType     string  `json:"workflow_type"`
	Status           string  `json:"status"`
	Stage            string  `json:"stage"`
	AttachmentState  string  `json:"attachment_state"`
	Model            string  `json:"model,omitempty"`
	Provider         string  `json:"provider,omitempty"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	TotalCost        float64 `json:"total_cost"`
	LastEventSeq     int64   `json:"last_event_seq"`
	LastCheckpointID string  `json:"last_checkpoint_id,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
	LastAdvancedAt   string  `json:"last_advanced_at"`
	TerminalAt       string  `json:"terminal_at,omitempty"`
	InputRequired    bool    `json:"input_required"`
	Stuck            bool    `json:"stuck"`
	StuckSeconds     int64   `json:"stuck_seconds"`
	Summary          string  `json:"summary,omitempty"`
}

// RunCheckpointSummary is the protocol projection of the latest checkpoint.
type RunCheckpointSummary struct {
	CheckpointID       string   `json:"checkpoint_id"`
	RunID              string   `json:"run_id"`
	Sequence           int64    `json:"sequence"`
	Stage              string   `json:"stage"`
	Status             string   `json:"status"`
	Reason             string   `json:"reason,omitempty"`
	TurnIDs            []string `json:"turn_ids,omitempty"`
	ModelCallIDs       []string `json:"model_call_ids,omitempty"`
	PendingToolCallIDs []string `json:"pending_tool_call_ids,omitempty"`
	Summary            string   `json:"summary,omitempty"`
	CreatedAt          string   `json:"created_at"`
	SchemaVersion      int      `json:"schema_version"`
}

// RunDetailsResponse is the result of run.details.
type RunDetailsResponse struct {
	Run        RunSummary            `json:"run"`
	TurnIDs    []string              `json:"turn_ids"`
	Turns      []TurnSummary         `json:"turns,omitempty"`
	Checkpoint *RunCheckpointSummary `json:"checkpoint,omitempty"`
	Events     []RunEventPayload     `json:"events"`
}

// RunAttachResponse is the result of run.attach.
type RunAttachResponse struct {
	Run             RunSummary            `json:"run"`
	AttachmentState string                `json:"attachment_state"`
	ReplayAvailable bool                  `json:"replay_available"`
	Fidelity        string                `json:"fidelity"`
	Events          []RunEventPayload     `json:"events,omitempty"`
	Checkpoint      *RunCheckpointSummary `json:"checkpoint,omitempty"`
}

// RunDetachResponse is the result of run.detach.
type RunDetachResponse struct {
	RunID           string `json:"run_id"`
	AttachmentState string `json:"attachment_state"`
}

// RunControlResponse is shared by cancel/retry/continue/fork controls.
type RunControlResponse struct {
	RunID    string      `json:"run_id"`
	Accepted bool        `json:"accepted"`
	Status   string      `json:"status,omitempty"`
	Run      *RunSummary `json:"run,omitempty"`
	Message  string      `json:"message,omitempty"`
}

// RunEventsResponse is the result of run.events.
type RunEventsResponse struct {
	Events          []RunEventPayload     `json:"events"`
	LatestSeq       int64                 `json:"latest_seq"`
	HasMore         bool                  `json:"has_more"`
	ReplayAvailable bool                  `json:"replay_available"`
	Fidelity        string                `json:"fidelity"`
	Checkpoint      *RunCheckpointSummary `json:"checkpoint,omitempty"`
}

// RunPermissionsResponse is the result of run.permissions.
type RunPermissionsResponse struct {
	Permissions []RunPermission `json:"permissions"`
}

// RunPermission is a pending permission or user-input blocker.
type RunPermission struct {
	RunID     string `json:"run_id"`
	RequestID string `json:"request_id"`
	Type      string `json:"type"`
	Reason    string `json:"reason,omitempty"`
}

// RunHeartbeatResponse returns latest Runtime-observed run status.
type RunHeartbeatResponse struct {
	RunID     string `json:"run_id"`
	Status    string `json:"status"`
	Stage     string `json:"stage"`
	LatestSeq int64  `json:"latest_seq"`
}
