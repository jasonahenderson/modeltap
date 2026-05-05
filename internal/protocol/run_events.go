package protocol

import "encoding/json"

const (
	EventRunStarted            = "run.started"
	EventRunStageChanged       = "run.stage_changed"
	EventRunStatusChanged      = "run.status_changed"
	EventRunBlocked            = "run.blocked"
	EventRunUnblocked          = "run.unblocked"
	EventRunStageTimeout       = "run.stage_timeout"
	EventRunProgress           = "run.progress"
	EventRunToolCallRequested  = "run.tool_call_requested"
	EventRunToolResultRecorded = "run.tool_result_recorded"
	EventRunArtifactRecorded   = "run.artifact_recorded"
	EventRunCheckpointRecorded = "run.checkpoint_recorded"
	EventRunAttached           = "run.attached"
	EventRunDetached           = "run.detached"
	EventRunCompleted          = "run.completed"
	EventRunFailed             = "run.failed"
	EventRunCancelled          = "run.cancelled"
)

// RunEventPayload is the canonical server notification and replay shape for
// run events.
type RunEventPayload struct {
	RunID     string          `json:"run_id"`
	Seq       int64           `json:"seq"`
	SessionID string          `json:"session_id"`
	TurnID    string          `json:"turn_id,omitempty"`
	Stage     string          `json:"stage,omitempty"`
	Status    string          `json:"status,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	CreatedAt string          `json:"created_at"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// Validate reports whether the event has the minimum required replay fields.
func (e RunEventPayload) Validate() bool {
	return e.RunID != "" && e.Seq > 0
}
