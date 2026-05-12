// Package storage defines the Store interface and types for persisting
// proxied API request/response data.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// MaxKnownSchemaVersion is the highest schema version this binary supports.
// NewSQLiteStore rejects databases with user_version above this value.
const MaxKnownSchemaVersion = 4

// Sentinel errors.
var (
	ErrSessionNotFound      = errors.New("storage: session not found")
	ErrSessionLockContended = errors.New("storage: session lock held by another owner")
	ErrSchemaTooNew         = errors.New("storage: database schema version is newer than this binary supports")
	ErrUserIDRequired       = errors.New("storage: user_id is required")
	ErrInvalidStatus        = errors.New("storage: invalid session status")
	ErrRunNotFound          = errors.New("storage: run not found")
	ErrInvalidRunStatus     = errors.New("storage: invalid run status")
	ErrInvalidRunStage      = errors.New("storage: invalid run stage")
	ErrInvalidWorkflowType  = errors.New("storage: invalid workflow_type")
)

// ValidSessionStatuses defines the allowed values for Session.Status.
var ValidSessionStatuses = map[string]bool{
	"active":    true,
	"suspended": true,
	"completed": true,
	"":          true,
}

// Session represents a persisted conversation session.
type Session struct {
	ID                string
	UserID            string
	Project           string
	Summary           string
	ActiveModel       string
	ModelOverride     *string
	RoutingOverrides  json.RawMessage // JSON object
	PinnedItems       json.RawMessage // JSON array
	CompactionState   json.RawMessage // JSON object
	TotalCost         float64
	TotalInputTokens  int64
	TotalOutputTokens int64
	ContextPct        float64
	Status            string // active | suspended | completed
	LockOwner         *string
	LockExpiresAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Turn represents a single persisted conversation turn.
type Turn struct {
	ID               string
	SessionID        string
	Sequence         int
	Role             string
	Content          json.RawMessage // canonical message JSON
	Model            string
	Provider         string
	InputTokens      int64
	OutputTokens     int64
	Cost             float64
	LatencyMs        int64
	ToolCalls        json.RawMessage // JSON array
	FilesTouched     []string        // marshalled via JSON on the wire
	FilesModified    []string
	Compacted        bool
	CompactedSummary string
	OriginalTurns    []int
	CreatedAt        time.Time
}

// SessionFilter scopes ListSessions queries.
type SessionFilter struct {
	UserID  string     // required — no cross-user listing
	Project string     // optional
	Status  string     // optional — active | suspended | completed
	Since   *time.Time // optional
	Limit   int        // 0 = use default
	Offset  int
}

// ServerSessionEvent matches the protocol.ServerSessionEvent wire shape
// but is storage-internal (adds session_id and id).
type ServerSessionEvent struct {
	ID        int64
	SessionID string
	Type      string
	Detail    string
	Payload   json.RawMessage
	At        time.Time
}

// CommandHistoryEntry is one persisted command-history record.
type CommandHistoryEntry struct {
	ID        int64
	UserID    string
	Project   string
	SessionID *string
	Content   string
	CreatedAt time.Time
}

// CommandHistoryFilter scopes history lookups per WU-091 protocol.
type CommandHistoryFilter struct {
	UserID    string // required
	Project   string // if non-empty, scopes to user+project
	SessionID string // if non-empty, scopes to user+session
	Limit     int
	Before    *time.Time // cursor for pagination (exclusive)
	BeforeID  *int64     // compound cursor: tie-break on id
}

// Canonical run statuses, stages, attachment states, and workflow types.
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

	RunWorkflowExploration    = "exploration"
	RunWorkflowFeature        = "feature"
	RunWorkflowADR            = "adr"
	RunWorkflowRelease        = "release"
	RunWorkflowImplementation = "implementation"
	RunWorkflowDebug          = "debug"
	RunWorkflowDocs           = "docs"
	RunWorkflowDevOps         = "devops"
)

// Run is one durable workflow execution.
type Run struct {
	ID                      string
	TraceID                 string
	IdempotencyKey          string
	UserID                  string
	Project                 string
	SessionID               string
	ParentRunID             *string
	InitiatorType           string
	Title                   string
	WorkflowType            string
	Status                  string
	Stage                   string
	AttachmentState         string
	AttachedConnectionID    string
	AttachmentGraceDeadline *time.Time
	Summary                 string
	LastAdvancedAt          time.Time
	Model                   string
	Provider                string
	InputTokens             int64
	OutputTokens            int64
	TotalCost               float64
	LastEventSeq            int64
	LastCheckpointID        string
	ExtensionJSON           json.RawMessage
	RetentionClass          string
	ExpiresAt               *time.Time
	SchemaVersion           int
	CreatedAt               time.Time
	UpdatedAt               time.Time
	TerminalAt              *time.Time
}

// RunEvent is an append-only sequenced run event.
type RunEvent struct {
	RunID                string
	Seq                  int64
	Type                 string
	Stage                string
	Status               string
	Reason               string
	PayloadJSON          json.RawMessage
	PayloadSchemaVersion int
	CreatedAt            time.Time
}

// RunCheckpoint stores point-in-time recovery metadata.
type RunCheckpoint struct {
	ID                 string
	RunID              string
	Seq                int64
	Stage              string
	Status             string
	Reason             string
	TurnIDs            []string
	ModelCallIDs       []string
	PendingToolCallIDs []string
	Summary            string
	PayloadJSON        json.RawMessage
	SchemaVersion      int
	CreatedAt          time.Time
}

// RunStateUpdate carries lifecycle changes paired with an appended event.
type RunStateUpdate struct {
	Status                  *string
	Stage                   *string
	Reason                  string
	AttachmentState         *string
	AttachedConnectionID    *string
	AttachmentGraceDeadline *time.Time
	Summary                 *string
	Model                   *string
	Provider                *string
	InputTokens             *int64
	OutputTokens            *int64
	TotalCost               *float64
	LastCheckpointID        *string
	TerminalAt              *time.Time
	Checkpoint              *RunCheckpoint
}

// RunFilter scopes ListRuns queries.
type RunFilter struct {
	UserID    string
	Project   string
	SessionID string
	Status    string
	Limit     int
	Offset    int
}

// RunModelCall records idempotent provider accounting for a run.
type RunModelCall struct {
	ModelCallID  string
	RunID        string
	Provider     string
	Model        string
	Stage        string
	Status       string
	InputTokens  int64
	OutputTokens int64
	TotalCost    float64
	LatencyMs    int64
	PayloadJSON  json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RunToolResult records idempotent tool-result delivery for a run.
type RunToolResult struct {
	ToolCallID    string
	RunID         string
	Tool          string
	Namespace     string
	Stage         string
	Status        string
	ResultID      string
	DurationMs    int64
	EstimatedCost float64
	PayloadJSON   json.RawMessage
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SessionSummary is the projected shape used by session.list (protocol layer).
type SessionSummary struct {
	ID              string
	Project         string
	Status          string
	Summary         string
	LastActive      time.Time
	ContextPct      float64
	TotalCost       float64
	TurnCount       int
	Model           string
	ModelOverride   *string
	LastTurnSummary string
	FilesTouched    []string
	PinnedCount     int
}

// Request represents a captured API request/response pair.
type Request struct {
	ID               string
	Timestamp        time.Time
	RunID            string
	TraceID          string
	Provider         string
	Model            string
	Method           string
	URL              string
	RequestHeaders   string // JSON
	RequestBody      string
	ResponseStatus   int
	ResponseHeaders  string // JSON
	ResponseBody     string
	InputTokens      int64
	OutputTokens     int64
	LatencyMs        int64
	EstimatedCostUSD float64
}

// ListFilter defines criteria for filtering and paginating request listings.
type ListFilter struct {
	Provider   string
	Model      string
	RunID      string
	TraceID    string
	Since      *time.Time
	Until      *time.Time
	StatusCode *int
	Limit      int
	Offset     int
}

// UsageMetrics holds aggregated usage data for a time period.
type UsageMetrics struct {
	Period        string
	Provider      string
	Model         string
	RequestCount  int64
	InputTokens   int64
	OutputTokens  int64
	EstimatedCost float64
	AvgLatencyMs  int64
	ErrorCount    int64
}

// MetricsFilter defines criteria for querying aggregated metrics.
type MetricsFilter struct {
	Since    *time.Time
	Until    *time.Time
	Provider string
	Model    string
	GroupBy  string // "hour", "day", "provider", "model"
}

// Store is the interface for persisting and querying captured requests.
type Store interface {
	SaveRequest(ctx context.Context, req *Request) error
	GetRequest(ctx context.Context, id string) (*Request, error)
	ListRequests(ctx context.Context, filter ListFilter) ([]Request, error)
	CountRequests(ctx context.Context, filter ListFilter) (int64, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
	QueryHourlyMetrics(ctx context.Context, filter MetricsFilter) ([]UsageMetrics, error)
	QueryDailyMetrics(ctx context.Context, filter MetricsFilter) ([]UsageMetrics, error)
	RebuildMetrics(ctx context.Context) error
	Ping(ctx context.Context) error
	Close() error

	// Session CRUD
	CreateSession(ctx context.Context, s *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	UpdateSession(ctx context.Context, s *Session) error
	ListSessions(ctx context.Context, filter SessionFilter) ([]Session, error)
	DeleteSessionsBefore(ctx context.Context, before time.Time) (int64, error)

	// Session lock mechanics
	AcquireSessionLock(ctx context.Context, sessionID, owner string, expiresAt time.Time) (acquired bool, currentOwner string, err error)
	ReleaseSessionLock(ctx context.Context, sessionID, owner string) error
	ForceReleaseSessionLock(ctx context.Context, sessionID string) error

	// Turn CRUD
	CreateTurn(ctx context.Context, t *Turn) error
	GetTurn(ctx context.Context, id string) (*Turn, error)
	ListTurns(ctx context.Context, sessionID string) ([]Turn, error)
	// DeleteTurn scopes by (sessionID, turnID) so a caller can't
	// accidentally delete a turn from a session they don't own —
	// WU-094 H-12. Idempotent: deleting an absent pair is not an
	// error.
	DeleteTurn(ctx context.Context, sessionID, turnID string) error

	// Session aggregation helpers
	SessionSummaries(ctx context.Context, filter SessionFilter) ([]SessionSummary, error)
	SessionFilesTouched(ctx context.Context, sessionID string) ([]string, error)
	SessionFilesModified(ctx context.Context, sessionID string) ([]string, error)

	// Session events
	AppendServerEvent(ctx context.Context, e *ServerSessionEvent) error
	ListServerEvents(ctx context.Context, sessionID string) ([]ServerSessionEvent, error)

	// Command history (WU-091)
	AppendCommandHistory(ctx context.Context, entry *CommandHistoryEntry) error
	ListCommandHistory(ctx context.Context, filter CommandHistoryFilter) ([]CommandHistoryEntry, error)

	// Durable runs
	CreateRun(ctx context.Context, run *Run, initial RunEvent, cp RunCheckpoint) error
	CreateRunWithTurn(ctx context.Context, run *Run, initial RunEvent, cp RunCheckpoint, turn *Turn, linkRole string, linkSeq int, history *CommandHistoryEntry) error
	GetRun(ctx context.Context, id string) (*Run, error)
	GetRunByIdempotency(ctx context.Context, userID, project, idempotencyKey string) (*Run, error)
	ListRuns(ctx context.Context, filter RunFilter) ([]Run, error)
	AppendRunEvent(ctx context.Context, runID string, ev RunEvent, update RunStateUpdate) (int64, error)
	CreateRunCheckpoint(ctx context.Context, cp RunCheckpoint) error
	GetLatestRunCheckpoint(ctx context.Context, runID string) (*RunCheckpoint, error)
	ListRunEvents(ctx context.Context, runID string, afterSeq int64, limit int) ([]RunEvent, error)
	LinkTurnToRun(ctx context.Context, runID, turnID, role string, seq int) error
	ListRunTurnIDs(ctx context.Context, runID string) ([]string, error)
	RecordRunModelCall(ctx context.Context, call RunModelCall) (bool, error)
	RecordRunToolResult(ctx context.Context, result RunToolResult) (bool, error)
}
