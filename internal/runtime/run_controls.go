package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

func handleRunCreate(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunCreate
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.create: " + err.Error())
	}
	if req.SessionID == "" {
		return nil, transportInvalidParams("session_id is required")
	}
	sess, err := conn.server.store.GetSession(ctx, req.SessionID)
	if err != nil || sess == nil {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: "session not found"}
	}
	if err := verifySessionAccess(conn, sess); err != nil {
		return nil, err
	}
	run, err := createRunRecord(ctx, conn.server, conn, sess, createRunOptions{
		IdempotencyKey:  req.IdempotencyKey,
		WorkflowType:    req.WorkflowType,
		Title:           req.Title,
		ParentRunID:     req.ParentRunID,
		Status:          storage.RunStatusQueued,
		AttachmentState: storage.RunAttachmentDetached,
	})
	if err != nil {
		if errors.Is(err, storage.ErrInvalidWorkflowType) {
			return nil, transportInvalidParams("invalid workflow_type")
		}
		return nil, transportInternal("create run", err)
	}
	return runSummary(*run), nil
}

func handleRunList(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunList
	if len(params) > 0 {
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, transportInvalidParams("decode run.list: " + err.Error())
		}
	}
	runs, err := conn.server.store.ListRuns(ctx, storage.RunFilter{
		UserID:    conn.UserID(),
		Project:   conn.Capabilities().ProjectContext().Root,
		SessionID: req.SessionID,
		Status:    req.Status,
		Limit:     req.Limit,
		Offset:    req.Offset,
	})
	if err != nil {
		return nil, transportInternal("list runs", err)
	}
	resp := protocol.RunListResponse{Runs: make([]protocol.RunSummary, 0, len(runs))}
	for _, run := range runs {
		resp.Runs = append(resp.Runs, runSummary(run))
	}
	return resp, nil
}

func handleRunDetails(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunDetails
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.details: " + err.Error())
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	turnIDs, _ := conn.server.store.ListRunTurnIDs(ctx, run.ID)
	cp, _ := conn.server.store.GetLatestRunCheckpoint(ctx, run.ID)
	events, err := conn.server.store.ListRunEvents(ctx, run.ID, maxInt64(0, run.LastEventSeq-25), 25)
	if err != nil {
		return nil, transportInternal("list run events", err)
	}
	out := protocol.RunDetailsResponse{
		Run:        runSummary(*run),
		TurnIDs:    turnIDs,
		Checkpoint: checkpointSummary(cp),
		Events:     make([]protocol.RunEventPayload, 0, len(events)),
	}
	for _, ev := range events {
		out.Events = append(out.Events, protocolRunEvent(*run, ev))
	}
	return out, nil
}

func handleRunAttach(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunAttach
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.attach: " + err.Error())
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	if isTerminalRunStatus(run.Status) {
		return nil, transportInvalidParams("cannot attach terminal run")
	}
	if run.AttachmentState == storage.RunAttachmentAttached && run.AttachedConnectionID != "" && run.AttachedConnectionID != conn.ID() {
		return nil, transportInvalidParams("attachment conflict: run is attached elsewhere")
	}
	state := storage.RunAttachmentAttached
	connID := conn.ID()
	ev := storage.RunEvent{Type: protocol.EventRunAttached, Stage: run.Stage, Status: run.Status, CreatedAt: time.Now().UTC()}
	update := storage.RunStateUpdate{AttachmentState: &state, AttachedConnectionID: &connID}
	seq, err := conn.server.store.AppendRunEvent(ctx, run.ID, ev, update)
	if err != nil {
		return nil, transportInternal("attach run", err)
	}
	run.AttachmentState = state
	run.AttachedConnectionID = connID
	run.LastEventSeq = seq
	events, _ := conn.server.store.ListRunEvents(ctx, run.ID, req.LastObservedSeq, 100)
	cp, _ := conn.server.store.GetLatestRunCheckpoint(ctx, run.ID)
	resp := protocol.RunAttachResponse{
		Run:             runSummary(*run),
		AttachmentState: state,
		ReplayAvailable: replayAvailable(events, req.LastObservedSeq),
		Fidelity:        replayFidelity(events, req.LastObservedSeq, run.LastEventSeq),
		Checkpoint:      checkpointSummary(cp),
	}
	for _, ev := range events {
		resp.Events = append(resp.Events, protocolRunEvent(*run, ev))
	}
	return resp, nil
}

func handleRunDetach(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunDetach
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.detach: " + err.Error())
	}
	if req.RunID == "" {
		var ctrl protocol.RunControl
		if err := json.Unmarshal(params, &ctrl); err == nil {
			req.RunID = ctrl.RunID
		}
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	state := storage.RunAttachmentDetached
	connID := ""
	ev := storage.RunEvent{Type: protocol.EventRunDetached, Stage: run.Stage, Status: run.Status, CreatedAt: time.Now().UTC()}
	update := storage.RunStateUpdate{AttachmentState: &state, AttachedConnectionID: &connID}
	if _, err := conn.server.store.AppendRunEvent(ctx, run.ID, ev, update); err != nil {
		return nil, transportInternal("detach run", err)
	}
	return protocol.RunDetachResponse{RunID: run.ID, AttachmentState: state}, nil
}

func handleRunCancel(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunControl
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.cancel: " + err.Error())
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	if isTerminalRunStatus(run.Status) {
		return protocol.RunControlResponse{RunID: run.ID, Accepted: false, Status: run.Status, Message: "run is already terminal"}, nil
	}
	cancelled := conn.server.runs.cancel(run.ID)
	status := storage.RunStatusCancelled
	stage := run.Stage
	terminal := time.Now().UTC()
	update := storage.RunStateUpdate{Status: &status, Stage: &stage, TerminalAt: &terminal}
	cp := storage.RunCheckpoint{Stage: stage, Status: status, Reason: req.Reason, PayloadJSON: defaultCheckpointPayload(), CreatedAt: terminal}
	update.Checkpoint = &cp
	ev := storage.RunEvent{Type: protocol.EventRunCancelled, Stage: stage, Status: status, Reason: req.Reason, CreatedAt: terminal}
	if _, err := conn.server.store.AppendRunEvent(ctx, run.ID, ev, update); err != nil {
		return nil, transportInternal("cancel run", err)
	}
	return protocol.RunControlResponse{RunID: run.ID, Accepted: true, Status: status, Message: boolMessage(cancelled, "active run cancelled", "run marked cancelled")}, nil
}

func handleRunRetry(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunControl
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.retry: " + err.Error())
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	return protocol.RunControlResponse{RunID: run.ID, Accepted: false, Status: run.Status, Message: "retry is checkpoint-aware but not enabled in v0.3.0"}, nil
}

func handleRunContinue(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunControl
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.continue: " + err.Error())
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	return protocol.RunControlResponse{RunID: run.ID, Accepted: false, Status: run.Status, Message: "continue is limited to future checkpointed stages"}, nil
}

func handleRunFork(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunControl
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.fork: " + err.Error())
	}
	parent, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	child, err := createRunRecord(ctx, conn.server, conn, &storage.Session{
		ID: parent.SessionID, UserID: parent.UserID, Project: parent.Project,
	}, createRunOptions{
		IdempotencyKey:  "fork:" + parent.ID + ":" + uuid.NewString(),
		WorkflowType:    parent.WorkflowType,
		Title:           parent.Title,
		ParentRunID:     parent.ID,
		Status:          storage.RunStatusQueued,
		AttachmentState: storage.RunAttachmentDetached,
	})
	if err != nil {
		return nil, transportInternal("fork run", err)
	}
	sum := runSummary(*child)
	return protocol.RunControlResponse{RunID: child.ID, Accepted: true, Status: child.Status, Run: &sum}, nil
}

func handleRunEvents(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunEvents
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.events: " + err.Error())
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	events, err := conn.server.store.ListRunEvents(ctx, run.ID, req.AfterSeq, req.Limit)
	if err != nil {
		return nil, transportInternal("list run events", err)
	}
	cp, _ := conn.server.store.GetLatestRunCheckpoint(ctx, run.ID)
	resp := protocol.RunEventsResponse{
		Events:          make([]protocol.RunEventPayload, 0, len(events)),
		LatestSeq:       run.LastEventSeq,
		HasMore:         len(events) > 0 && int64(len(events))+req.AfterSeq < run.LastEventSeq,
		ReplayAvailable: replayAvailable(events, req.AfterSeq),
		Fidelity:        replayFidelity(events, req.AfterSeq, run.LastEventSeq),
		Checkpoint:      checkpointSummary(cp),
	}
	for _, ev := range events {
		resp.Events = append(resp.Events, protocolRunEvent(*run, ev))
	}
	return resp, nil
}

func handleRunPermissions(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunPermissions
	if len(params) > 0 {
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, transportInvalidParams("decode run.permissions: " + err.Error())
		}
	}
	resp := protocol.RunPermissionsResponse{}
	if req.RunID != "" {
		run, err := getAuthorizedRun(ctx, conn, req.RunID)
		if err != nil {
			return nil, err
		}
		if run.Status == storage.RunStatusWaitingPermission || run.Status == storage.RunStatusWaitingUser {
			resp.Permissions = append(resp.Permissions, latestRunBlocker(ctx, conn.server.store, *run))
		}
	}
	return resp, nil
}

func handleRunResolvePermission(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunResolvePermission
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.resolve_permission: " + err.Error())
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	if isTerminalRunStatus(run.Status) {
		return protocol.RunControlResponse{RunID: run.ID, Accepted: false, Status: run.Status, Message: "run is already terminal"}, nil
	}
	if req.RequestID == "" || req.Decision == "" {
		return nil, transportInvalidParams("request_id and decision are required")
	}
	if run.Status == storage.RunStatusWaitingPermission {
		status := storage.RunStatusRunning
		stage := run.Stage
		payload := map[string]string{"request_id": req.RequestID, "decision": req.Decision}
		raw, _ := json.Marshal(payload)
		ev := storage.RunEvent{Type: protocol.EventRunUnblocked, Stage: stage, Status: status, PayloadJSON: raw, CreatedAt: time.Now().UTC()}
		cp := storage.RunCheckpoint{Stage: stage, Status: status, PayloadJSON: defaultCheckpointPayload(), CreatedAt: time.Now().UTC()}
		if _, err := conn.server.store.AppendRunEvent(ctx, run.ID, ev, storage.RunStateUpdate{Status: &status, Stage: &stage, Checkpoint: &cp}); err != nil {
			return nil, transportInternal("resolve run permission", err)
		}
	}
	return protocol.RunControlResponse{RunID: run.ID, Accepted: true, Status: storage.RunStatusRunning}, nil
}

func handleRunHeartbeat(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.RunHeartbeat
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, transportInvalidParams("decode run.heartbeat: " + err.Error())
	}
	run, err := getAuthorizedRun(ctx, conn, req.RunID)
	if err != nil {
		return nil, err
	}
	if isTerminalRunStatus(run.Status) {
		return protocol.RunHeartbeatResponse{RunID: run.ID, Status: run.Status, Stage: run.Stage, LatestSeq: run.LastEventSeq}, nil
	}
	stage := run.Stage
	if req.Stage != "" {
		stage = req.Stage
	}
	raw, _ := json.Marshal(map[string]any{
		"host_fingerprint":  req.HostFingerprint,
		"last_observed_seq": req.LastObservedSeq,
		"stage":             stage,
	})
	ev := storage.RunEvent{Type: protocol.EventRunProgress, Stage: stage, Status: run.Status, PayloadJSON: raw, CreatedAt: time.Now().UTC()}
	seq, err := conn.server.store.AppendRunEvent(ctx, run.ID, ev, storage.RunStateUpdate{})
	if err != nil {
		return nil, transportInternal("heartbeat run", err)
	}
	return protocol.RunHeartbeatResponse{RunID: run.ID, Status: run.Status, Stage: stage, LatestSeq: seq}, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func boolMessage(ok bool, ifTrue, ifFalse string) string {
	if ok {
		return ifTrue
	}
	return ifFalse
}

func replayAvailable(events []storage.RunEvent, afterSeq int64) bool {
	return len(events) == 0 || events[0].Seq <= afterSeq+1
}

func replayFidelity(events []storage.RunEvent, afterSeq, latestSeq int64) string {
	if !replayAvailable(events, afterSeq) {
		return protocol.RunReplaySummary
	}
	if len(events) == 0 && afterSeq < latestSeq {
		return protocol.RunReplaySummary
	}
	return protocol.RunReplayFull
}

func latestRunBlocker(ctx context.Context, store storage.Store, run storage.Run) protocol.RunPermission {
	perm := protocol.RunPermission{RunID: run.ID, Type: run.Status}
	events, err := store.ListRunEvents(ctx, run.ID, 0, 500)
	if err != nil {
		return perm
	}
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if ev.Type != protocol.EventRunBlocked {
			continue
		}
		var payload struct {
			RequestID string `json:"request_id"`
			Type      string `json:"type"`
			Reason    string `json:"reason"`
		}
		if err := json.Unmarshal(ev.PayloadJSON, &payload); err != nil {
			return perm
		}
		if payload.RequestID != "" {
			perm.RequestID = payload.RequestID
		}
		if payload.Type != "" {
			perm.Type = payload.Type
		}
		if payload.Reason != "" {
			perm.Reason = payload.Reason
		}
		return perm
	}
	return perm
}
