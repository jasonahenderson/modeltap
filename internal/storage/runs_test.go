package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func seedRunSession(t *testing.T, store *SQLiteStore) {
	t.Helper()
	now := time.Now().UTC()
	if err := store.CreateSession(context.Background(), &Session{
		ID:        "sess-runs",
		UserID:    "user-1",
		Project:   "/repo",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
}

func TestRunStorage_CreateAppendAndReplay(t *testing.T) {
	store := newTestStore(t)
	seedRunSession(t, store)
	ctx := context.Background()
	now := time.Now().UTC()

	run := &Run{
		ID:              "run-1",
		TraceID:         "trace-1",
		IdempotencyKey:  "idem-1",
		UserID:          "user-1",
		Project:         "/repo",
		SessionID:       "sess-runs",
		Status:          RunStatusRunning,
		Stage:           RunStagePreflight,
		AttachmentState: RunAttachmentAttached,
		WorkflowType:    RunWorkflowImplementation,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := store.CreateRun(ctx, run, RunEvent{Type: "run.started"}, RunCheckpoint{}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	stage := RunStageModelCall
	status := RunStatusRunning
	cp := RunCheckpoint{Stage: stage, Status: status, PayloadJSON: json.RawMessage(`{"context":{},"artifacts":{},"policy":{},"workspace":{},"memory":{},"routing":{}}`)}
	seq, err := store.AppendRunEvent(ctx, run.ID, RunEvent{Type: "run.stage_changed", Stage: stage, Status: status}, RunStateUpdate{
		Stage:      &stage,
		Status:     &status,
		Checkpoint: &cp,
	})
	if err != nil {
		t.Fatalf("AppendRunEvent: %v", err)
	}
	if seq != 2 {
		t.Fatalf("seq = %d, want 2", seq)
	}

	events, err := store.ListRunEvents(ctx, run.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListRunEvents: %v", err)
	}
	if len(events) != 2 || events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("events = %+v", events)
	}

	got, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.LastEventSeq != 2 || got.Stage != RunStageModelCall || got.WorkflowType != RunWorkflowImplementation {
		t.Errorf("run state = %+v", got)
	}
}

func TestRunStorage_CreateRunWithTurnPersistsForegroundTransaction(t *testing.T) {
	store := newTestStore(t)
	seedRunSession(t, store)
	ctx := context.Background()
	now := time.Now().UTC()
	sessionID := "sess-runs"

	run := &Run{
		ID:              "run-with-turn",
		TraceID:         "trace-with-turn",
		IdempotencyKey:  "idem-with-turn",
		UserID:          "user-1",
		Project:         "/repo",
		SessionID:       sessionID,
		Status:          RunStatusRunning,
		Stage:           RunStagePreflight,
		AttachmentState: RunAttachmentAttached,
		WorkflowType:    RunWorkflowImplementation,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	turn := &Turn{ID: "turn-with-run", SessionID: sessionID, Sequence: 1, Role: "user", Content: json.RawMessage(`"hello"`), CreatedAt: now}
	history := &CommandHistoryEntry{UserID: "user-1", Project: "/repo", SessionID: &sessionID, Content: "hello", CreatedAt: now}

	if err := store.CreateRunWithTurn(ctx, run, RunEvent{Type: "run.started"}, RunCheckpoint{}, turn, turn.Role, turn.Sequence, history); err != nil {
		t.Fatalf("CreateRunWithTurn: %v", err)
	}

	got, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.LastEventSeq != 1 || got.LastCheckpointID == "" {
		t.Fatalf("run sequence/checkpoint = %d/%q, want 1/non-empty", got.LastEventSeq, got.LastCheckpointID)
	}
	turnIDs, err := store.ListRunTurnIDs(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListRunTurnIDs: %v", err)
	}
	if len(turnIDs) != 1 || turnIDs[0] != turn.ID {
		t.Fatalf("turnIDs = %+v, want [%s]", turnIDs, turn.ID)
	}
	turns, err := store.ListTurns(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListTurns: %v", err)
	}
	if len(turns) != 1 || turns[0].ID != turn.ID {
		t.Fatalf("turns = %+v, want one foreground turn", turns)
	}
	historyRows, err := store.ListCommandHistory(ctx, CommandHistoryFilter{UserID: "user-1", SessionID: sessionID, Limit: 10})
	if err != nil {
		t.Fatalf("ListCommandHistory: %v", err)
	}
	if len(historyRows) != 1 || historyRows[0].Content != "hello" {
		t.Fatalf("history = %+v, want one hello entry", historyRows)
	}
}

func TestRunStorage_CreateRunWithTurnRollsBackOnTurnFailure(t *testing.T) {
	store := newTestStore(t)
	seedRunSession(t, store)
	ctx := context.Background()
	now := time.Now().UTC()

	duplicate := &Turn{ID: "turn-duplicate", SessionID: "sess-runs", Sequence: 1, Role: "user", Content: json.RawMessage(`"existing"`), CreatedAt: now}
	if err := store.CreateTurn(ctx, duplicate); err != nil {
		t.Fatalf("CreateTurn duplicate seed: %v", err)
	}

	run := &Run{
		ID:              "run-rolled-back",
		TraceID:         "trace-rolled-back",
		IdempotencyKey:  "idem-rolled-back",
		UserID:          "user-1",
		Project:         "/repo",
		SessionID:       "sess-runs",
		Status:          RunStatusRunning,
		Stage:           RunStagePreflight,
		AttachmentState: RunAttachmentAttached,
		WorkflowType:    RunWorkflowImplementation,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	turn := &Turn{ID: duplicate.ID, SessionID: "sess-runs", Sequence: 2, Role: "user", Content: json.RawMessage(`"new"`), CreatedAt: now}
	if err := store.CreateRunWithTurn(ctx, run, RunEvent{Type: "run.started"}, RunCheckpoint{}, turn, turn.Role, turn.Sequence, nil); err == nil {
		t.Fatalf("CreateRunWithTurn succeeded with duplicate turn id")
	}
	if _, err := store.GetRun(ctx, run.ID); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("GetRun after rollback err = %v, want ErrRunNotFound", err)
	}
}

func TestRunStorage_LinkTurnToRunEnforcesSingleRunOwnership(t *testing.T) {
	store := newTestStore(t)
	seedRunSession(t, store)
	ctx := context.Background()
	now := time.Now().UTC()

	turn := &Turn{ID: "turn-owned-once", SessionID: "sess-runs", Sequence: 1, Role: "user", Content: json.RawMessage(`"hello"`), CreatedAt: now}
	if err := store.CreateTurn(ctx, turn); err != nil {
		t.Fatalf("CreateTurn: %v", err)
	}
	for _, id := range []string{"run-owner-1", "run-owner-2"} {
		run := &Run{
			ID:              id,
			TraceID:         "trace-" + id,
			IdempotencyKey:  "idem-" + id,
			UserID:          "user-1",
			Project:         "/repo",
			SessionID:       "sess-runs",
			Status:          RunStatusRunning,
			Stage:           RunStagePreflight,
			AttachmentState: RunAttachmentAttached,
			WorkflowType:    RunWorkflowImplementation,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := store.CreateRun(ctx, run, RunEvent{Type: "run.started"}, RunCheckpoint{}); err != nil {
			t.Fatalf("CreateRun(%s): %v", id, err)
		}
	}
	if err := store.LinkTurnToRun(ctx, "run-owner-1", turn.ID, turn.Role, turn.Sequence); err != nil {
		t.Fatalf("LinkTurnToRun first: %v", err)
	}
	if err := store.LinkTurnToRun(ctx, "run-owner-2", turn.ID, turn.Role, turn.Sequence); err == nil {
		t.Fatalf("LinkTurnToRun second succeeded, want UNIQUE(turn_id) failure")
	}
}

func TestRunStorage_WorkflowValidationAndAccountingIdempotency(t *testing.T) {
	store := newTestStore(t)
	seedRunSession(t, store)
	ctx := context.Background()

	err := store.CreateRun(ctx, &Run{
		ID:              "bad-workflow",
		TraceID:         "trace-bad",
		IdempotencyKey:  "bad",
		UserID:          "user-1",
		Project:         "/repo",
		SessionID:       "sess-runs",
		WorkflowType:    "unknown",
		Status:          RunStatusQueued,
		Stage:           RunStagePreflight,
		AttachmentState: RunAttachmentDetached,
	}, RunEvent{}, RunCheckpoint{})
	if !errors.Is(err, ErrInvalidWorkflowType) {
		t.Fatalf("CreateRun invalid workflow err = %v, want ErrInvalidWorkflowType", err)
	}

	run := &Run{
		ID:              "run-accounting",
		TraceID:         "trace-accounting",
		IdempotencyKey:  "accounting",
		UserID:          "user-1",
		Project:         "/repo",
		SessionID:       "sess-runs",
		Status:          RunStatusRunning,
		Stage:           RunStageModelCall,
		AttachmentState: RunAttachmentAttached,
	}
	if err := store.CreateRun(ctx, run, RunEvent{}, RunCheckpoint{}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	call := RunModelCall{
		ModelCallID:  "mc-1",
		RunID:        run.ID,
		Provider:     "anthropic",
		Model:        "claude",
		Status:       "completed",
		InputTokens:  10,
		OutputTokens: 20,
		TotalCost:    0.03,
	}
	created, err := store.RecordRunModelCall(ctx, call)
	if err != nil || !created {
		t.Fatalf("RecordRunModelCall first = %v %v", created, err)
	}
	created, err = store.RecordRunModelCall(ctx, call)
	if err != nil || created {
		t.Fatalf("RecordRunModelCall duplicate = %v %v", created, err)
	}
	got, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.InputTokens != 10 || got.OutputTokens != 20 || got.TotalCost != 0.03 {
		t.Errorf("totals double-counted or missing: %+v", got)
	}
}
