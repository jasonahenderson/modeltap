package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

type gapReplayStore struct {
	storage.Store
	run    storage.Run
	events []storage.RunEvent
}

func (s *gapReplayStore) GetRun(_ context.Context, id string) (*storage.Run, error) {
	if id != s.run.ID {
		return nil, storage.ErrRunNotFound
	}
	return &s.run, nil
}

func (s *gapReplayStore) ListRunEvents(_ context.Context, runID string, afterSeq int64, limit int) ([]storage.RunEvent, error) {
	var out []storage.RunEvent
	for _, ev := range s.events {
		if ev.RunID == runID && ev.Seq > afterSeq {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (s *gapReplayStore) GetLatestRunCheckpoint(_ context.Context, runID string) (*storage.RunCheckpoint, error) {
	return &storage.RunCheckpoint{ID: "cp-gap", RunID: runID, Seq: s.run.LastEventSeq, Stage: s.run.Stage, Status: s.run.Status}, nil
}

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

func TestRunDetailsIncludesTurnSummaries(t *testing.T) {
	srv := newServerWithRealStore(t)
	run := seedRunForControls(t, srv, "")
	c, _ := newRelayConnection(t, srv)

	userRaw, _ := json.Marshal(provider.Message{Role: "user", Content: "please inspect the session turns"})
	turn := &storage.Turn{
		ID:        "turn-run-detail",
		SessionID: run.SessionID,
		Sequence:  1,
		Role:      "user",
		Content:   userRaw,
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateTurn(context.Background(), turn); err != nil {
		t.Fatalf("CreateTurn: %v", err)
	}
	if err := srv.store.LinkTurnToRun(context.Background(), run.ID, turn.ID, turn.Role, turn.Sequence); err != nil {
		t.Fatalf("LinkTurnToRun: %v", err)
	}

	params, _ := json.Marshal(protocol.RunDetails{RunID: run.ID})
	raw, err := handleRunDetails(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleRunDetails: %v", err)
	}
	details := raw.(protocol.RunDetailsResponse)
	if len(details.Turns) != 1 {
		t.Fatalf("Turns = %d, want 1: %+v", len(details.Turns), details.Turns)
	}
	if details.Turns[0].Summary != "please inspect the session turns" {
		t.Fatalf("turn summary = %q", details.Turns[0].Summary)
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

func TestRunEventsReportsSummaryFidelityOnReplayGap(t *testing.T) {
	store := &gapReplayStore{
		run: storage.Run{
			ID:              "run-gap",
			UserID:          SoloUserID,
			SessionID:       "sess-gap",
			Status:          storage.RunStatusRunning,
			Stage:           storage.RunStageModelCall,
			AttachmentState: storage.RunAttachmentAttached,
			LastEventSeq:    5,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
			LastAdvancedAt:  time.Now().UTC(),
		},
		events: []storage.RunEvent{{RunID: "run-gap", Seq: 5, Type: protocol.EventRunCompleted}},
	}
	srv := NewServer(store, shortServerConfig(""))
	c, _ := newRelayConnection(t, srv)

	params, _ := json.Marshal(protocol.RunEvents{RunID: "run-gap", AfterSeq: 1, Limit: 10})
	resp, err := handleRunEvents(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleRunEvents: %v", err)
	}
	got := resp.(protocol.RunEventsResponse)
	if got.ReplayAvailable {
		t.Fatalf("ReplayAvailable = true, want false")
	}
	if got.Fidelity != protocol.RunReplaySummary {
		t.Fatalf("Fidelity = %q, want summary", got.Fidelity)
	}
	if got.Checkpoint == nil || got.Checkpoint.CheckpointID != "cp-gap" {
		t.Fatalf("checkpoint = %+v", got.Checkpoint)
	}
}

func TestRunHeartbeatAppendsProgressAndClearsStuck(t *testing.T) {
	srv := newServerWithRealStore(t)
	run := seedRunForControls(t, srv, "")
	c, _ := newRelayConnection(t, srv)

	old := time.Now().UTC().Add(-runStuckThreshold - time.Minute)
	if _, err := srv.store.AppendRunEvent(context.Background(), run.ID, storage.RunEvent{
		Type:      protocol.EventRunProgress,
		Stage:     storage.RunStageModelCall,
		Status:    storage.RunStatusRunning,
		CreatedAt: old,
	}, storage.RunStateUpdate{}); err != nil {
		t.Fatalf("AppendRunEvent old progress: %v", err)
	}

	detailsParams, _ := json.Marshal(protocol.RunDetails{RunID: run.ID})
	before, err := handleRunDetails(context.Background(), c, detailsParams)
	if err != nil {
		t.Fatalf("handleRunDetails before: %v", err)
	}
	if !before.(protocol.RunDetailsResponse).Run.Stuck {
		t.Fatalf("run should be stuck before heartbeat")
	}

	hbParams, _ := json.Marshal(protocol.RunHeartbeat{RunID: run.ID, HostFingerprint: "host-1", LastObservedSeq: 2})
	resp, err := handleRunHeartbeat(context.Background(), c, hbParams)
	if err != nil {
		t.Fatalf("handleRunHeartbeat: %v", err)
	}
	heartbeat := resp.(protocol.RunHeartbeatResponse)
	if heartbeat.LatestSeq != 3 {
		t.Fatalf("heartbeat latest seq = %d, want 3", heartbeat.LatestSeq)
	}

	after, err := handleRunDetails(context.Background(), c, detailsParams)
	if err != nil {
		t.Fatalf("handleRunDetails after: %v", err)
	}
	if after.(protocol.RunDetailsResponse).Run.Stuck {
		t.Fatalf("run should not be stuck after heartbeat")
	}
}

func TestRunHeartbeatDoesNotAppendTerminalRun(t *testing.T) {
	srv := newServerWithRealStore(t)
	run := seedRunForControls(t, srv, "")
	c, _ := newRelayConnection(t, srv)

	status := storage.RunStatusCompleted
	stage := storage.RunStageCompletion
	terminal := time.Now().UTC()
	if _, err := srv.store.AppendRunEvent(context.Background(), run.ID, storage.RunEvent{
		Type:      protocol.EventRunCompleted,
		Stage:     stage,
		Status:    status,
		CreatedAt: terminal,
	}, storage.RunStateUpdate{Status: &status, Stage: &stage, TerminalAt: &terminal}); err != nil {
		t.Fatalf("AppendRunEvent terminal: %v", err)
	}

	params, _ := json.Marshal(protocol.RunHeartbeat{RunID: run.ID})
	resp, err := handleRunHeartbeat(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleRunHeartbeat: %v", err)
	}
	heartbeat := resp.(protocol.RunHeartbeatResponse)
	if heartbeat.LatestSeq != 2 {
		t.Fatalf("heartbeat latest seq = %d, want unchanged 2", heartbeat.LatestSeq)
	}
}

func TestRunResolvePermissionRejectsTerminalRun(t *testing.T) {
	srv := newServerWithRealStore(t)
	run := seedRunForControls(t, srv, "")
	c, _ := newRelayConnection(t, srv)

	status := storage.RunStatusCompleted
	stage := storage.RunStageCompletion
	terminal := time.Now().UTC()
	if _, err := srv.store.AppendRunEvent(context.Background(), run.ID, storage.RunEvent{
		Type:      protocol.EventRunCompleted,
		Stage:     stage,
		Status:    status,
		CreatedAt: terminal,
	}, storage.RunStateUpdate{Status: &status, Stage: &stage, TerminalAt: &terminal}); err != nil {
		t.Fatalf("AppendRunEvent terminal: %v", err)
	}

	params, _ := json.Marshal(protocol.RunResolvePermission{RunID: run.ID, RequestID: "req-1", Decision: "allow"})
	resp, err := handleRunResolvePermission(context.Background(), c, params)
	if err != nil {
		t.Fatalf("handleRunResolvePermission: %v", err)
	}
	got := resp.(protocol.RunControlResponse)
	if got.Accepted || got.Status != storage.RunStatusCompleted {
		t.Fatalf("resolve terminal response = %+v, want rejected completed", got)
	}
}
