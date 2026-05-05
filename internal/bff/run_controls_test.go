package bff

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

func seedRunForControls(t *testing.T, srv *Server, attachedConnectionID string) *storage.Run {
	t.Helper()
	now := time.Now().UTC()
	sess := &storage.Session{
		ID:        "sess-run-controls",
		UserID:    SoloUserID,
		Project:   "",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := srv.store.CreateSession(context.Background(), sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	state := storage.RunAttachmentDetached
	if attachedConnectionID != "" {
		state = storage.RunAttachmentAttached
	}
	run := &storage.Run{
		ID:                   "run-controls",
		TraceID:              "trace-controls",
		IdempotencyKey:       "controls",
		UserID:               SoloUserID,
		Project:              "",
		SessionID:            sess.ID,
		Status:               storage.RunStatusRunning,
		Stage:                storage.RunStageModelCall,
		AttachmentState:      state,
		AttachedConnectionID: attachedConnectionID,
	}
	if err := srv.store.CreateRun(context.Background(), run, storage.RunEvent{Type: protocol.EventRunStarted}, storage.RunCheckpoint{}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	return run
}

func TestRunAttachRejectsConflictingConnection(t *testing.T) {
	srv := newServerWithRealStore(t)
	run := seedRunForControls(t, srv, "other-conn")
	c, _ := newRelayConnection(t, srv)

	params, _ := json.Marshal(protocol.RunAttach{RunID: run.ID})
	_, err := handleRunAttach(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected attach conflict")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeInvalidParams || !strings.Contains(te.Message, "attached elsewhere") {
		t.Fatalf("err = %T %v, want attachment conflict invalid params", err, err)
	}
}

func TestRunDetachClearsAttachedConnectionID(t *testing.T) {
	srv := newServerWithRealStore(t)
	run := seedRunForControls(t, srv, "conn-relay")
	c, _ := newRelayConnection(t, srv)

	params, _ := json.Marshal(protocol.RunDetach{RunID: run.ID})
	if _, err := handleRunDetach(context.Background(), c, params); err != nil {
		t.Fatalf("handleRunDetach: %v", err)
	}
	got, err := srv.store.GetRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.AttachmentState != storage.RunAttachmentDetached || got.AttachedConnectionID != "" {
		t.Fatalf("attachment = %q/%q, want detached/empty", got.AttachmentState, got.AttachedConnectionID)
	}
}

func TestRunEventsReplayIncludesEventType(t *testing.T) {
	srv := newServerWithRealStore(t)
	run := seedRunForControls(t, srv, "")
	c, _ := newRelayConnection(t, srv)

	params, _ := json.Marshal(protocol.RunEvents{RunID: run.ID, AfterSeq: 0, Limit: 10})
	resp, err := handleRunEvents(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleRunEvents: %v", err)
	}
	events := resp.(protocol.RunEventsResponse).Events
	if len(events) == 0 {
		t.Fatalf("no replay events")
	}
	if events[0].Type != protocol.EventRunStarted {
		t.Fatalf("event type = %q, want %q", events[0].Type, protocol.EventRunStarted)
	}
}

func TestRunPermissionsReturnsStoredBlockerDetails(t *testing.T) {
	srv := newServerWithRealStore(t)
	run := seedRunForControls(t, srv, "")
	c, _ := newRelayConnection(t, srv)

	status := storage.RunStatusWaitingPermission
	stage := storage.RunStageToolLoop
	payload := json.RawMessage(`{"request_id":"perm-1","type":"waiting_permission","reason":"write_file"}`)
	if _, err := srv.store.AppendRunEvent(context.Background(), run.ID, storage.RunEvent{
		Type:        protocol.EventRunBlocked,
		Stage:       stage,
		Status:      status,
		PayloadJSON: payload,
	}, storage.RunStateUpdate{Status: &status, Stage: &stage}); err != nil {
		t.Fatalf("AppendRunEvent: %v", err)
	}

	params, _ := json.Marshal(protocol.RunPermissions{RunID: run.ID})
	resp, err := handleRunPermissions(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleRunPermissions: %v", err)
	}
	perms := resp.(protocol.RunPermissionsResponse).Permissions
	if len(perms) != 1 {
		t.Fatalf("permissions = %d, want 1", len(perms))
	}
	if perms[0].RequestID != "perm-1" || perms[0].Reason != "write_file" || perms[0].Type != storage.RunStatusWaitingPermission {
		t.Fatalf("permission = %+v", perms[0])
	}
}
