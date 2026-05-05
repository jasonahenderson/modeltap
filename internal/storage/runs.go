package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var validRunStatuses = map[string]bool{
	RunStatusQueued:            true,
	RunStatusRunning:           true,
	RunStatusWaitingPermission: true,
	RunStatusWaitingUser:       true,
	RunStatusCheckpointed:      true,
	RunStatusCompleted:         true,
	RunStatusFailed:            true,
	RunStatusCancelled:         true,
	"":                         true,
}

var validRunStages = map[string]bool{
	RunStagePreflight:       true,
	RunStageContextPlan:     true,
	RunStagePromptPlan:      true,
	RunStageModelCall:       true,
	RunStageToolLoop:        true,
	RunStageValidation:      true,
	RunStageArtifactCapture: true,
	RunStageCheckpoint:      true,
	RunStageCompletion:      true,
	"":                      true,
}

var validWorkflowTypes = map[string]bool{
	RunWorkflowExploration:    true,
	RunWorkflowFeature:        true,
	RunWorkflowADR:            true,
	RunWorkflowRelease:        true,
	RunWorkflowImplementation: true,
	RunWorkflowDebug:          true,
	RunWorkflowDocs:           true,
	RunWorkflowDevOps:         true,
}

// CreateRun inserts a durable run, its attachment row, the initial event, and
// the initial checkpoint in one transaction.
func (s *SQLiteStore) CreateRun(ctx context.Context, run *Run, initial RunEvent, cp RunCheckpoint) error {
	if err := normalizeRunForCreate(run); err != nil {
		return err
	}
	if initial.Type == "" {
		initial.Type = "run.started"
	}
	if initial.Stage == "" {
		initial.Stage = run.Stage
	}
	if initial.Status == "" {
		initial.Status = run.Status
	}
	if initial.PayloadJSON == nil {
		initial.PayloadJSON = json.RawMessage(`{}`)
	}
	if initial.PayloadSchemaVersion == 0 {
		initial.PayloadSchemaVersion = 1
	}
	if initial.CreatedAt.IsZero() {
		initial.CreatedAt = run.CreatedAt
	}
	initial.RunID = run.ID
	initial.Seq = 1

	normalizeCheckpointForCreate(&cp, run, initial.Seq)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning create run transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertRun(ctx, tx, run); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO run_attachments (run_id, state, attached_connection_id, grace_deadline, updated_at)
VALUES (?, ?, ?, ?, ?)`,
		run.ID,
		run.AttachmentState,
		run.AttachedConnectionID,
		formatOptionalTime(run.AttachmentGraceDeadline),
		run.UpdatedAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("inserting run attachment: %w", err)
	}
	if err := insertRunEvent(ctx, tx, initial); err != nil {
		return err
	}
	if err := insertRunCheckpoint(ctx, tx, cp); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE runs SET last_event_seq = ?, last_checkpoint_id = ?, updated_at = ? WHERE id = ?`,
		initial.Seq, cp.ID, run.UpdatedAt.UTC().Format(time.RFC3339Nano), run.ID,
	); err != nil {
		return fmt.Errorf("updating created run sequence: %w", err)
	}
	run.LastEventSeq = initial.Seq
	run.LastCheckpointID = cp.ID

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing create run: %w", err)
	}
	return nil
}

// CreateRunWithTurn inserts a foreground run, initial event/checkpoint, user
// turn, run-turn link, and optional command history in one transaction.
func (s *SQLiteStore) CreateRunWithTurn(ctx context.Context, run *Run, initial RunEvent, cp RunCheckpoint, turn *Turn, linkRole string, linkSeq int, history *CommandHistoryEntry) error {
	if turn == nil {
		return fmt.Errorf("storage: turn is required")
	}
	if err := normalizeRunForCreate(run); err != nil {
		return err
	}
	if initial.Type == "" {
		initial.Type = "run.started"
	}
	if initial.Stage == "" {
		initial.Stage = run.Stage
	}
	if initial.Status == "" {
		initial.Status = run.Status
	}
	if initial.PayloadJSON == nil {
		initial.PayloadJSON = json.RawMessage(`{}`)
	}
	if initial.PayloadSchemaVersion == 0 {
		initial.PayloadSchemaVersion = 1
	}
	if initial.CreatedAt.IsZero() {
		initial.CreatedAt = run.CreatedAt
	}
	initial.RunID = run.ID
	initial.Seq = 1
	normalizeCheckpointForCreate(&cp, run, initial.Seq)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning create foreground run transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertRun(ctx, tx, run); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO run_attachments (run_id, state, attached_connection_id, grace_deadline, updated_at)
VALUES (?, ?, ?, ?, ?)`,
		run.ID,
		run.AttachmentState,
		run.AttachedConnectionID,
		formatOptionalTime(run.AttachmentGraceDeadline),
		run.UpdatedAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("inserting run attachment: %w", err)
	}
	if err := insertRunEvent(ctx, tx, initial); err != nil {
		return err
	}
	if err := insertRunCheckpoint(ctx, tx, cp); err != nil {
		return err
	}
	if err := insertTurn(ctx, tx, turn); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO run_turns (run_id, turn_id, sequence, role, created_at)
VALUES (?, ?, ?, ?, ?)`, run.ID, turn.ID, linkSeq, linkRole, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("linking turn to run: %w", err)
	}
	if history != nil && history.Content != "" {
		sessionID := history.SessionID
		_, err := tx.ExecContext(ctx, `
INSERT INTO command_history (user_id, project, session_id, content, created_at)
VALUES (?, ?, ?, ?, ?)`,
			history.UserID,
			history.Project,
			sessionID,
			history.Content,
			history.CreatedAt.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("inserting command history: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE runs SET last_event_seq = ?, last_checkpoint_id = ?, updated_at = ? WHERE id = ?`,
		initial.Seq, cp.ID, run.UpdatedAt.UTC().Format(time.RFC3339Nano), run.ID,
	); err != nil {
		return fmt.Errorf("updating created run sequence: %w", err)
	}
	run.LastEventSeq = initial.Seq
	run.LastCheckpointID = cp.ID

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing create foreground run: %w", err)
	}
	return nil
}

// GetRun retrieves one run by ID.
func (s *SQLiteStore) GetRun(ctx context.Context, id string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, selectRunSQL()+` WHERE id = ?`, id)
	return scanRun(row)
}

// GetRunByIdempotency retrieves a run by the user/project/idempotency boundary.
func (s *SQLiteStore) GetRunByIdempotency(ctx context.Context, userID, project, key string) (*Run, error) {
	row := s.db.QueryRowContext(ctx, selectRunSQL()+` WHERE user_id = ? AND project = ? AND idempotency_key = ?`, userID, project, key)
	return scanRun(row)
}

// ListRuns returns runs matching the filter, newest first.
func (s *SQLiteStore) ListRuns(ctx context.Context, filter RunFilter) ([]Run, error) {
	if filter.UserID == "" {
		return nil, ErrUserIDRequired
	}
	if !validRunStatuses[filter.Status] {
		return nil, ErrInvalidRunStatus
	}

	conditions := []string{"user_id = ?"}
	args := []any{filter.UserID}
	if filter.Project != "" {
		conditions = append(conditions, "project = ?")
		args = append(args, filter.Project)
	}
	if filter.SessionID != "" {
		conditions = append(conditions, "session_id = ?")
		args = append(args, filter.SessionID)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := selectRunSQL() + ` WHERE ` + strings.Join(conditions, " AND ") + ` ORDER BY updated_at DESC LIMIT ?`
	args = append(args, limit)
	if filter.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}
	defer rows.Close()
	var runs []Run
	for rows.Next() {
		run, err := scanRunRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run rows: %w", err)
	}
	return runs, nil
}

// AppendRunEvent appends one event and atomically updates run state.
func (s *SQLiteStore) AppendRunEvent(ctx context.Context, runID string, ev RunEvent, update RunStateUpdate) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning append run event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lastSeq int64
	if err := tx.QueryRowContext(ctx, `SELECT last_event_seq FROM runs WHERE id = ?`, runID).Scan(&lastSeq); err != nil {
		if err == sql.ErrNoRows {
			return 0, ErrRunNotFound
		}
		return 0, fmt.Errorf("reading run sequence: %w", err)
	}
	ev.RunID = runID
	ev.Seq = lastSeq + 1
	if ev.PayloadJSON == nil {
		ev.PayloadJSON = json.RawMessage(`{}`)
	}
	if ev.PayloadSchemaVersion == 0 {
		ev.PayloadSchemaVersion = 1
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	if err := insertRunEvent(ctx, tx, ev); err != nil {
		return 0, err
	}

	if update.Checkpoint != nil {
		normalizeCheckpointForAppend(update.Checkpoint, runID, ev.Seq)
		if err := insertRunCheckpoint(ctx, tx, *update.Checkpoint); err != nil {
			return 0, err
		}
		cpID := update.Checkpoint.ID
		update.LastCheckpointID = &cpID
	}
	if err := applyRunStateUpdate(ctx, tx, runID, ev.Seq, update, ev.CreatedAt); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing append run event: %w", err)
	}
	return ev.Seq, nil
}

// CreateRunCheckpoint inserts a checkpoint outside a lifecycle event.
func (s *SQLiteStore) CreateRunCheckpoint(ctx context.Context, cp RunCheckpoint) error {
	normalizeCheckpointForAppend(&cp, cp.RunID, cp.Seq)
	if _, err := s.db.ExecContext(ctx, `SELECT 1 FROM runs WHERE id = ?`, cp.RunID); err != nil {
		return fmt.Errorf("checking run for checkpoint: %w", err)
	}
	return insertRunCheckpoint(ctx, s.db, cp)
}

// GetLatestRunCheckpoint returns the newest checkpoint for a run.
func (s *SQLiteStore) GetLatestRunCheckpoint(ctx context.Context, runID string) (*RunCheckpoint, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, run_id, seq, stage, status, reason, turn_ids_json, model_call_ids_json,
       pending_tool_call_ids_json, summary, payload_json, schema_version, created_at
FROM run_checkpoints WHERE run_id = ? ORDER BY seq DESC LIMIT 1`, runID)
	return scanRunCheckpoint(row)
}

// ListRunEvents returns events after afterSeq in ascending order.
func (s *SQLiteStore) ListRunEvents(ctx context.Context, runID string, afterSeq int64, limit int) ([]RunEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT run_id, seq, type, stage, status, reason, payload_json, payload_schema_version, created_at
FROM run_events WHERE run_id = ? AND seq > ? ORDER BY seq ASC LIMIT ?`, runID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("listing run events: %w", err)
	}
	defer rows.Close()
	var events []RunEvent
	for rows.Next() {
		ev, err := scanRunEventRows(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating run events: %w", err)
	}
	return events, nil
}

// LinkTurnToRun records the durable one-run ownership for a turn.
func (s *SQLiteStore) LinkTurnToRun(ctx context.Context, runID, turnID, role string, seq int) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO run_turns (run_id, turn_id, sequence, role, created_at)
VALUES (?, ?, ?, ?, ?)`, runID, turnID, seq, role, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("linking turn to run: %w", err)
	}
	return nil
}

// ListRunTurnIDs returns turn IDs linked to a run in sequence order.
func (s *SQLiteStore) ListRunTurnIDs(ctx context.Context, runID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT turn_id FROM run_turns WHERE run_id = ? ORDER BY sequence ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("listing run turn ids: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning run turn id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// RecordRunModelCall records idempotent model-call accounting.
func (s *SQLiteStore) RecordRunModelCall(ctx context.Context, call RunModelCall) (bool, error) {
	if call.PayloadJSON == nil {
		call.PayloadJSON = json.RawMessage(`{}`)
	}
	if call.Stage == "" {
		call.Stage = RunStageModelCall
	}
	now := time.Now().UTC()
	if call.CreatedAt.IsZero() {
		call.CreatedAt = now
	}
	if call.UpdatedAt.IsZero() {
		call.UpdatedAt = now
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("beginning record model call transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO run_model_calls (
	model_call_id, run_id, provider, model, stage, status, input_tokens, output_tokens,
	total_cost, latency_ms, payload_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		call.ModelCallID, call.RunID, call.Provider, call.Model, call.Stage, call.Status,
		call.InputTokens, call.OutputTokens, call.TotalCost, call.LatencyMs,
		string(call.PayloadJSON), call.CreatedAt.UTC().Format(time.RFC3339Nano), call.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, fmt.Errorf("recording model call: %w", err)
	}
	created, _ := res.RowsAffected()
	if created > 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE runs SET input_tokens = input_tokens + ?, output_tokens = output_tokens + ?,
	total_cost = total_cost + ?, model = CASE WHEN model = '' THEN ? ELSE model END,
	provider = CASE WHEN provider = '' THEN ? ELSE provider END, updated_at = ?
WHERE id = ?`, call.InputTokens, call.OutputTokens, call.TotalCost, call.Model, call.Provider, now.Format(time.RFC3339Nano), call.RunID); err != nil {
			return false, fmt.Errorf("updating run totals from model call: %w", err)
		}
	}
	return created > 0, tx.Commit()
}

// RecordRunToolResult records idempotent tool-result delivery.
func (s *SQLiteStore) RecordRunToolResult(ctx context.Context, result RunToolResult) (bool, error) {
	if result.PayloadJSON == nil {
		result.PayloadJSON = json.RawMessage(`{}`)
	}
	if result.Stage == "" {
		result.Stage = RunStageToolLoop
	}
	now := time.Now().UTC()
	if result.CreatedAt.IsZero() {
		result.CreatedAt = now
	}
	if result.UpdatedAt.IsZero() {
		result.UpdatedAt = now
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("beginning record tool result transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO run_tool_results (
	tool_call_id, run_id, tool, namespace, stage, status, result_id, duration_ms,
	estimated_cost, payload_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.ToolCallID, result.RunID, result.Tool, result.Namespace, result.Stage,
		result.Status, result.ResultID, result.DurationMs, result.EstimatedCost,
		string(result.PayloadJSON), result.CreatedAt.UTC().Format(time.RFC3339Nano), result.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, fmt.Errorf("recording tool result: %w", err)
	}
	created, _ := res.RowsAffected()
	if created > 0 && result.EstimatedCost != 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE runs SET total_cost = total_cost + ?, updated_at = ? WHERE id = ?`,
			result.EstimatedCost, now.Format(time.RFC3339Nano), result.RunID); err != nil {
			return false, fmt.Errorf("updating run totals from tool result: %w", err)
		}
	}
	return created > 0, tx.Commit()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func selectRunSQL() string {
	return `SELECT id, trace_id, idempotency_key, user_id, project, session_id, parent_run_id,
	initiator_type, title, workflow_type, status, stage, attachment_state, attached_connection_id,
	attachment_grace_deadline, summary, last_advanced_at, model, provider, input_tokens,
	output_tokens, total_cost, last_event_seq, last_checkpoint_id, extension_json,
	retention_class, expires_at, schema_version, created_at, updated_at, terminal_at FROM runs`
}

func scanRun(row rowScanner) (*Run, error) {
	run, err := scanRunWith(row)
	if err == sql.ErrNoRows {
		return nil, ErrRunNotFound
	}
	return run, err
}

func scanRunRows(row rowScanner) (*Run, error) {
	return scanRunWith(row)
}

func scanRunWith(row rowScanner) (*Run, error) {
	var r Run
	var parentRunID, attachDeadline, expiresAt, terminalAt *string
	var lastAdvancedAt, createdAt, updatedAt, extensionJSON string
	if err := row.Scan(
		&r.ID, &r.TraceID, &r.IdempotencyKey, &r.UserID, &r.Project, &r.SessionID, &parentRunID,
		&r.InitiatorType, &r.Title, &r.WorkflowType, &r.Status, &r.Stage, &r.AttachmentState, &r.AttachedConnectionID,
		&attachDeadline, &r.Summary, &lastAdvancedAt, &r.Model, &r.Provider, &r.InputTokens,
		&r.OutputTokens, &r.TotalCost, &r.LastEventSeq, &r.LastCheckpointID, &extensionJSON,
		&r.RetentionClass, &expiresAt, &r.SchemaVersion, &createdAt, &updatedAt, &terminalAt,
	); err != nil {
		return nil, err
	}
	var err error
	r.ParentRunID = parentRunID
	r.ExtensionJSON = json.RawMessage(extensionJSON)
	if r.LastAdvancedAt, err = parseTime(lastAdvancedAt); err != nil {
		return nil, err
	}
	if r.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if r.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	if r.AttachmentGraceDeadline, err = parseOptionalTime(attachDeadline); err != nil {
		return nil, err
	}
	if r.ExpiresAt, err = parseOptionalTime(expiresAt); err != nil {
		return nil, err
	}
	if r.TerminalAt, err = parseOptionalTime(terminalAt); err != nil {
		return nil, err
	}
	return &r, nil
}

func scanRunEventRows(row rowScanner) (*RunEvent, error) {
	var ev RunEvent
	var payload, createdAt string
	if err := row.Scan(&ev.RunID, &ev.Seq, &ev.Type, &ev.Stage, &ev.Status, &ev.Reason, &payload, &ev.PayloadSchemaVersion, &createdAt); err != nil {
		return nil, fmt.Errorf("scanning run event: %w", err)
	}
	ev.PayloadJSON = json.RawMessage(payload)
	var err error
	ev.CreatedAt, err = parseTime(createdAt)
	return &ev, err
}

func scanRunCheckpoint(row rowScanner) (*RunCheckpoint, error) {
	var cp RunCheckpoint
	var turnIDs, modelCallIDs, pendingToolCallIDs, payload, createdAt string
	if err := row.Scan(&cp.ID, &cp.RunID, &cp.Seq, &cp.Stage, &cp.Status, &cp.Reason, &turnIDs, &modelCallIDs,
		&pendingToolCallIDs, &cp.Summary, &payload, &cp.SchemaVersion, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning run checkpoint: %w", err)
	}
	_ = json.Unmarshal([]byte(turnIDs), &cp.TurnIDs)
	_ = json.Unmarshal([]byte(modelCallIDs), &cp.ModelCallIDs)
	_ = json.Unmarshal([]byte(pendingToolCallIDs), &cp.PendingToolCallIDs)
	cp.PayloadJSON = json.RawMessage(payload)
	var err error
	cp.CreatedAt, err = parseTime(createdAt)
	return &cp, err
}

func normalizeRunForCreate(run *Run) error {
	if run.ID == "" {
		run.ID = "run-" + uuid.NewString()
	}
	if run.TraceID == "" {
		run.TraceID = "trace-" + uuid.NewString()
	}
	if run.WorkflowType == "" {
		run.WorkflowType = RunWorkflowImplementation
	}
	if !validWorkflowTypes[run.WorkflowType] {
		return ErrInvalidWorkflowType
	}
	if run.Status == "" {
		run.Status = RunStatusQueued
	}
	if !validRunStatuses[run.Status] {
		return ErrInvalidRunStatus
	}
	if run.Stage == "" {
		run.Stage = RunStagePreflight
	}
	if !validRunStages[run.Stage] {
		return ErrInvalidRunStage
	}
	if run.AttachmentState == "" {
		run.AttachmentState = RunAttachmentDetached
	}
	if run.ExtensionJSON == nil {
		run.ExtensionJSON = json.RawMessage(`{}`)
	}
	if run.RetentionClass == "" {
		run.RetentionClass = "standard"
	}
	if run.SchemaVersion == 0 {
		run.SchemaVersion = 1
	}
	now := time.Now().UTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}
	if run.LastAdvancedAt.IsZero() {
		run.LastAdvancedAt = run.CreatedAt
	}
	if run.InitiatorType == "" {
		run.InitiatorType = "user"
	}
	return nil
}

func normalizeCheckpointForCreate(cp *RunCheckpoint, run *Run, seq int64) {
	if cp.RunID == "" {
		cp.RunID = run.ID
	}
	normalizeCheckpointForAppend(cp, run.ID, seq)
	if cp.Stage == "" {
		cp.Stage = run.Stage
	}
	if cp.Status == "" {
		cp.Status = run.Status
	}
	if cp.Summary == "" {
		cp.Summary = run.Summary
	}
}

func normalizeCheckpointForAppend(cp *RunCheckpoint, runID string, seq int64) {
	if cp.ID == "" {
		cp.ID = "cp-" + uuid.NewString()
	}
	if cp.RunID == "" {
		cp.RunID = runID
	}
	if cp.Seq == 0 {
		cp.Seq = seq
	}
	if cp.PayloadJSON == nil {
		cp.PayloadJSON = json.RawMessage(`{"context":{},"artifacts":{},"policy":{},"workspace":{},"memory":{},"routing":{}}`)
	}
	if cp.SchemaVersion == 0 {
		cp.SchemaVersion = 1
	}
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
}

func insertRun(ctx context.Context, tx *sql.Tx, r *Run) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO runs (
	id, trace_id, idempotency_key, user_id, project, session_id, parent_run_id,
	initiator_type, title, workflow_type, status, stage, attachment_state,
	attached_connection_id, attachment_grace_deadline, summary, last_advanced_at,
	model, provider, input_tokens, output_tokens, total_cost, last_event_seq,
	last_checkpoint_id, extension_json, retention_class, expires_at, schema_version,
	created_at, updated_at, terminal_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.TraceID, r.IdempotencyKey, r.UserID, r.Project, r.SessionID, r.ParentRunID,
		r.InitiatorType, r.Title, r.WorkflowType, r.Status, r.Stage, r.AttachmentState,
		r.AttachedConnectionID, formatOptionalTime(r.AttachmentGraceDeadline), r.Summary, r.LastAdvancedAt.UTC().Format(time.RFC3339Nano),
		r.Model, r.Provider, r.InputTokens, r.OutputTokens, r.TotalCost, r.LastEventSeq,
		r.LastCheckpointID, string(r.ExtensionJSON), r.RetentionClass, formatOptionalTime(r.ExpiresAt), r.SchemaVersion,
		r.CreatedAt.UTC().Format(time.RFC3339Nano), r.UpdatedAt.UTC().Format(time.RFC3339Nano), formatOptionalTime(r.TerminalAt))
	if err != nil {
		return fmt.Errorf("inserting run: %w", err)
	}
	return nil
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertRunEvent(ctx context.Context, e execer, ev RunEvent) error {
	_, err := e.ExecContext(ctx, `
INSERT INTO run_events (run_id, seq, type, stage, status, reason, payload_json, payload_schema_version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.RunID, ev.Seq, ev.Type, ev.Stage, ev.Status, ev.Reason, string(ev.PayloadJSON), ev.PayloadSchemaVersion, ev.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("inserting run event: %w", err)
	}
	return nil
}

func insertRunCheckpoint(ctx context.Context, e execer, cp RunCheckpoint) error {
	turnIDs, _ := json.Marshal(cp.TurnIDs)
	modelCallIDs, _ := json.Marshal(cp.ModelCallIDs)
	pendingToolCallIDs, _ := json.Marshal(cp.PendingToolCallIDs)
	_, err := e.ExecContext(ctx, `
INSERT INTO run_checkpoints (
	id, run_id, seq, stage, status, reason, turn_ids_json, model_call_ids_json,
	pending_tool_call_ids_json, summary, payload_json, schema_version, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cp.ID, cp.RunID, cp.Seq, cp.Stage, cp.Status, cp.Reason, string(turnIDs), string(modelCallIDs),
		string(pendingToolCallIDs), cp.Summary, string(cp.PayloadJSON), cp.SchemaVersion, cp.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("inserting run checkpoint: %w", err)
	}
	return nil
}

func applyRunStateUpdate(ctx context.Context, tx *sql.Tx, runID string, seq int64, update RunStateUpdate, at time.Time) error {
	sets := []string{"last_event_seq = ?", "updated_at = ?", "last_advanced_at = ?"}
	args := []any{seq, at.UTC().Format(time.RFC3339Nano), at.UTC().Format(time.RFC3339Nano)}
	if update.Status != nil {
		if !validRunStatuses[*update.Status] {
			return ErrInvalidRunStatus
		}
		sets = append(sets, "status = ?")
		args = append(args, *update.Status)
	}
	if update.Stage != nil {
		if !validRunStages[*update.Stage] {
			return ErrInvalidRunStage
		}
		sets = append(sets, "stage = ?")
		args = append(args, *update.Stage)
	}
	if update.AttachmentState != nil {
		sets = append(sets, "attachment_state = ?")
		args = append(args, *update.AttachmentState)
	}
	if update.AttachedConnectionID != nil {
		sets = append(sets, "attached_connection_id = ?")
		args = append(args, *update.AttachedConnectionID)
	}
	if update.AttachmentGraceDeadline != nil {
		sets = append(sets, "attachment_grace_deadline = ?")
		args = append(args, update.AttachmentGraceDeadline.UTC().Format(time.RFC3339Nano))
	}
	if update.Summary != nil {
		sets = append(sets, "summary = ?")
		args = append(args, *update.Summary)
	}
	if update.Model != nil {
		sets = append(sets, "model = ?")
		args = append(args, *update.Model)
	}
	if update.Provider != nil {
		sets = append(sets, "provider = ?")
		args = append(args, *update.Provider)
	}
	if update.InputTokens != nil {
		sets = append(sets, "input_tokens = ?")
		args = append(args, *update.InputTokens)
	}
	if update.OutputTokens != nil {
		sets = append(sets, "output_tokens = ?")
		args = append(args, *update.OutputTokens)
	}
	if update.TotalCost != nil {
		sets = append(sets, "total_cost = ?")
		args = append(args, *update.TotalCost)
	}
	if update.LastCheckpointID != nil {
		sets = append(sets, "last_checkpoint_id = ?")
		args = append(args, *update.LastCheckpointID)
	}
	if update.TerminalAt != nil {
		sets = append(sets, "terminal_at = ?")
		args = append(args, update.TerminalAt.UTC().Format(time.RFC3339Nano))
	}
	args = append(args, runID)
	res, err := tx.ExecContext(ctx, `UPDATE runs SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("updating run state: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking run update rows affected: %w", err)
	}
	if n == 0 {
		return ErrRunNotFound
	}
	if update.AttachmentState != nil || update.AttachedConnectionID != nil || update.AttachmentGraceDeadline != nil {
		attachmentSets := []string{"updated_at = ?"}
		attachmentArgs := []any{at.UTC().Format(time.RFC3339Nano)}
		if update.AttachmentState != nil {
			attachmentSets = append(attachmentSets, "state = ?")
			attachmentArgs = append(attachmentArgs, *update.AttachmentState)
		}
		if update.AttachedConnectionID != nil {
			attachmentSets = append(attachmentSets, "attached_connection_id = ?")
			attachmentArgs = append(attachmentArgs, *update.AttachedConnectionID)
		}
		if update.AttachmentGraceDeadline != nil {
			attachmentSets = append(attachmentSets, "grace_deadline = ?")
			attachmentArgs = append(attachmentArgs, update.AttachmentGraceDeadline.UTC().Format(time.RFC3339Nano))
		}
		attachmentArgs = append(attachmentArgs, runID)
		_, err := tx.ExecContext(ctx, `UPDATE run_attachments SET `+strings.Join(attachmentSets, ", ")+` WHERE run_id = ?`, attachmentArgs...)
		if err != nil {
			return fmt.Errorf("updating run attachment state: %w", err)
		}
	}
	return nil
}

func parseTime(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing time %q: %w", raw, err)
	}
	return t, nil
}

func parseOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := parseTime(*raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func formatOptionalTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}
