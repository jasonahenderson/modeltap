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
