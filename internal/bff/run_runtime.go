package bff

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

const runStuckThreshold = 300 * time.Second

func (s *Server) registerRunHandlers() {
	s.dispatcher.Register(protocol.MethodRunList, handleRunList)
	s.dispatcher.Register(protocol.MethodRunCreate, handleRunCreate)
	s.dispatcher.Register(protocol.MethodRunDetails, handleRunDetails)
	s.dispatcher.Register(protocol.MethodRunAttach, handleRunAttach)
	s.dispatcher.Register(protocol.MethodRunDetach, handleRunDetach)
	s.dispatcher.Register(protocol.MethodRunCancel, handleRunCancel)
	s.dispatcher.Register(protocol.MethodRunRetry, handleRunRetry)
	s.dispatcher.Register(protocol.MethodRunContinue, handleRunContinue)
	s.dispatcher.Register(protocol.MethodRunFork, handleRunFork)
	s.dispatcher.Register(protocol.MethodRunEvents, handleRunEvents)
	s.dispatcher.Register(protocol.MethodRunPermissions, handleRunPermissions)
	s.dispatcher.Register(protocol.MethodRunResolvePermission, handleRunResolvePermission)
	s.dispatcher.Register(protocol.MethodRunHeartbeat, handleRunHeartbeat)
}

func createRunRecord(ctx context.Context, srv *Server, conn *Connection, sess *storage.Session, opts createRunOptions) (*storage.Run, error) {
	key := opts.IdempotencyKey
	if key == "" {
		key = "run:" + uuid.NewString()
	}
	if existing, err := srv.store.GetRunByIdempotency(ctx, sess.UserID, sess.Project, key); err == nil {
		return existing, nil
	} else if !errors.Is(err, storage.ErrRunNotFound) {
		return nil, err
	}

	now := time.Now().UTC()
	parent := stringPtrOrNil(opts.ParentRunID)
	run := &storage.Run{
		ID:                   "run-" + uuid.NewString(),
		TraceID:              "trace-" + uuid.NewString(),
		IdempotencyKey:       key,
		UserID:               sess.UserID,
		Project:              sess.Project,
		SessionID:            sess.ID,
		ParentRunID:          parent,
		InitiatorType:        "user",
		Title:                opts.Title,
		WorkflowType:         opts.WorkflowType,
		Status:               opts.Status,
		Stage:                storage.RunStagePreflight,
		AttachmentState:      opts.AttachmentState,
		AttachedConnectionID: opts.AttachedConnectionID,
		LastAdvancedAt:       now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	payload, _ := json.Marshal(map[string]any{
		"trace_id":      run.TraceID,
		"workflow_type": run.WorkflowType,
	})
	initial := storage.RunEvent{
		Type:        protocol.EventRunStarted,
		Stage:       run.Stage,
		Status:      run.Status,
		PayloadJSON: payload,
		CreatedAt:   now,
	}
	cp := storage.RunCheckpoint{
		Stage:       run.Stage,
		Status:      run.Status,
		Summary:     run.Summary,
		PayloadJSON: defaultCheckpointPayload(),
		CreatedAt:   now,
	}
	if err := srv.store.CreateRun(ctx, run, initial, cp); err != nil {
		return nil, err
	}
	if conn != nil {
		emitStoredRunEvent(conn, run, storage.RunEvent{RunID: run.ID, Seq: 1, Type: initial.Type, Stage: initial.Stage, Status: initial.Status, PayloadJSON: initial.PayloadJSON, CreatedAt: initial.CreatedAt}, "")
	}
	return run, nil
}

func createForegroundRunWithTurn(ctx context.Context, srv *Server, conn *Connection, sess *storage.Session, opts createRunOptions, turn *storage.Turn, history *storage.CommandHistoryEntry) (*storage.Run, error) {
	key := opts.IdempotencyKey
	if key == "" {
		key = "turn:" + sess.ID + ":" + turn.ID
	}

	now := time.Now().UTC()
	run := &storage.Run{
		ID:                   "run-" + uuid.NewString(),
		TraceID:              "trace-" + uuid.NewString(),
		IdempotencyKey:       key,
		UserID:               sess.UserID,
		Project:              sess.Project,
		SessionID:            sess.ID,
		ParentRunID:          stringPtrOrNil(opts.ParentRunID),
		InitiatorType:        "user",
		Title:                opts.Title,
		WorkflowType:         opts.WorkflowType,
		Status:               opts.Status,
		Stage:                storage.RunStagePreflight,
		AttachmentState:      opts.AttachmentState,
		AttachedConnectionID: opts.AttachedConnectionID,
		LastAdvancedAt:       now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	payload, _ := json.Marshal(map[string]any{
		"trace_id":      run.TraceID,
		"workflow_type": run.WorkflowType,
	})
	initial := storage.RunEvent{
		Type:        protocol.EventRunStarted,
		Stage:       run.Stage,
		Status:      run.Status,
		PayloadJSON: payload,
		CreatedAt:   now,
	}
	cp := storage.RunCheckpoint{
		Stage:       run.Stage,
		Status:      run.Status,
		Summary:     run.Summary,
		TurnIDs:     []string{turn.ID},
		PayloadJSON: defaultCheckpointPayload(),
		CreatedAt:   now,
	}
	if err := srv.store.CreateRunWithTurn(ctx, run, initial, cp, turn, turn.Role, turn.Sequence, history); err != nil {
		return nil, err
	}
	if conn != nil {
		emitStoredRunEvent(conn, run, storage.RunEvent{RunID: run.ID, Seq: 1, Type: initial.Type, Stage: initial.Stage, Status: initial.Status, PayloadJSON: initial.PayloadJSON, CreatedAt: initial.CreatedAt}, turn.ID)
	}
	return run, nil
}

type createRunOptions struct {
	IdempotencyKey       string
	WorkflowType         string
	Title                string
	ParentRunID          string
	Status               string
	AttachmentState      string
	AttachedConnectionID string
}

func appendRunLifecycle(ctx context.Context, conn *Connection, run *storage.Run, typ, stage, status, reason string, payload any) (int64, error) {
	raw := json.RawMessage(`{}`)
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			raw = b
		}
	}
	now := time.Now().UTC()
	ev := storage.RunEvent{
		Type:        typ,
		Stage:       stage,
		Status:      status,
		Reason:      reason,
		PayloadJSON: raw,
		CreatedAt:   now,
	}
	update := storage.RunStateUpdate{}
	if stage != "" {
		update.Stage = &stage
	}
	if status != "" {
		update.Status = &status
	}
	if typ == protocol.EventRunCompleted || typ == protocol.EventRunFailed || typ == protocol.EventRunCancelled {
		update.TerminalAt = &now
	}
	cp := storage.RunCheckpoint{
		Stage:       stage,
		Status:      status,
		Reason:      reason,
		Summary:     run.Summary,
		PayloadJSON: defaultCheckpointPayload(),
		CreatedAt:   now,
	}
	update.Checkpoint = &cp
	seq, err := conn.server.store.AppendRunEvent(ctx, run.ID, ev, update)
	if err != nil {
		return 0, err
	}
	ev.RunID = run.ID
	ev.Seq = seq
	emitStoredRunEvent(conn, run, ev, "")
	run.LastEventSeq = seq
	run.Stage = stage
	run.Status = status
	run.UpdatedAt = now
	run.LastAdvancedAt = now
	return seq, nil
}

func emitStoredRunEvent(conn *Connection, run *storage.Run, ev storage.RunEvent, turnID string) {
	if conn == nil || run == nil {
		return
	}
	payload := protocol.RunEventPayload{
		RunID:     run.ID,
		Seq:       ev.Seq,
		Type:      ev.Type,
		SessionID: run.SessionID,
		TurnID:    turnID,
		Stage:     ev.Stage,
		Status:    ev.Status,
		Reason:    ev.Reason,
		CreatedAt: ev.CreatedAt.UTC().Format(time.RFC3339Nano),
		Payload:   ev.PayloadJSON,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = conn.transport.SendNotification(&protocol.Notification{
		JSONRPC: "2.0",
		Method:  ev.Type,
		Params:  raw,
	})
}

func runSummary(run storage.Run) protocol.RunSummary {
	now := time.Now().UTC()
	stuckSeconds := int64(0)
	stuck := false
	if !isTerminalRunStatus(run.Status) {
		stuckSeconds = int64(now.Sub(run.LastAdvancedAt).Seconds())
		stuck = now.Sub(run.LastAdvancedAt) >= runStuckThreshold
	}
	parent := ""
	if run.ParentRunID != nil {
		parent = *run.ParentRunID
	}
	terminal := ""
	if run.TerminalAt != nil {
		terminal = run.TerminalAt.UTC().Format(time.RFC3339Nano)
	}
	return protocol.RunSummary{
		RunID:            run.ID,
		TraceID:          run.TraceID,
		SessionID:        run.SessionID,
		ParentRunID:      parent,
		Title:            run.Title,
		WorkflowType:     run.WorkflowType,
		Status:           run.Status,
		Stage:            run.Stage,
		AttachmentState:  run.AttachmentState,
		Model:            run.Model,
		Provider:         run.Provider,
		InputTokens:      run.InputTokens,
		OutputTokens:     run.OutputTokens,
		TotalCost:        run.TotalCost,
		LastEventSeq:     run.LastEventSeq,
		LastCheckpointID: run.LastCheckpointID,
		CreatedAt:        run.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:        run.UpdatedAt.UTC().Format(time.RFC3339Nano),
		LastAdvancedAt:   run.LastAdvancedAt.UTC().Format(time.RFC3339Nano),
		TerminalAt:       terminal,
		InputRequired:    run.Status == storage.RunStatusWaitingPermission || run.Status == storage.RunStatusWaitingUser,
		Stuck:            stuck,
		StuckSeconds:     stuckSeconds,
		Summary:          run.Summary,
	}
}

func protocolRunEvent(run storage.Run, ev storage.RunEvent) protocol.RunEventPayload {
	var payload json.RawMessage
	if len(ev.PayloadJSON) > 0 && string(ev.PayloadJSON) != "{}" {
		payload = ev.PayloadJSON
	}
	return protocol.RunEventPayload{
		RunID:     ev.RunID,
		Seq:       ev.Seq,
		Type:      ev.Type,
		SessionID: run.SessionID,
		Stage:     ev.Stage,
		Status:    ev.Status,
		Reason:    ev.Reason,
		CreatedAt: ev.CreatedAt.UTC().Format(time.RFC3339Nano),
		Payload:   payload,
	}
}

func checkpointSummary(cp *storage.RunCheckpoint) *protocol.RunCheckpointSummary {
	if cp == nil {
		return nil
	}
	return &protocol.RunCheckpointSummary{
		CheckpointID:       cp.ID,
		RunID:              cp.RunID,
		Sequence:           cp.Seq,
		Stage:              cp.Stage,
		Status:             cp.Status,
		Reason:             cp.Reason,
		TurnIDs:            cp.TurnIDs,
		ModelCallIDs:       cp.ModelCallIDs,
		PendingToolCallIDs: cp.PendingToolCallIDs,
		Summary:            cp.Summary,
		CreatedAt:          cp.CreatedAt.UTC().Format(time.RFC3339Nano),
		SchemaVersion:      cp.SchemaVersion,
	}
}

func defaultCheckpointPayload() json.RawMessage {
	return json.RawMessage(`{"context":{},"artifacts":{},"policy":{},"workspace":{},"memory":{},"routing":{}}`)
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func isTerminalRunStatus(status string) bool {
	return status == storage.RunStatusCompleted || status == storage.RunStatusFailed || status == storage.RunStatusCancelled
}

func transportInvalidParams(message string) error {
	return &TransportError{Code: CodeInvalidParams, Message: message}
}

func transportInternal(op string, err error) error {
	return &TransportError{Code: CodeInternalError, Message: op + ": " + err.Error()}
}

func requireRunID(runID string) error {
	if runID == "" {
		return transportInvalidParams("run_id is required")
	}
	return nil
}

func getAuthorizedRun(ctx context.Context, conn *Connection, runID string) (*storage.Run, error) {
	if err := requireRunID(runID); err != nil {
		return nil, err
	}
	run, err := conn.server.store.GetRun(ctx, runID)
	if errors.Is(err, storage.ErrRunNotFound) {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: "run not found"}
	}
	if err != nil {
		return nil, transportInternal("get run", err)
	}
	if run.UserID != conn.UserID() {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: "run not found"}
	}
	return run, nil
}
