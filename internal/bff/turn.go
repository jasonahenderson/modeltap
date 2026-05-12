package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// turnTracker is owned by Server and lets handleTurnCancel find the
// goroutine that owns a given turn id.
type turnTracker struct {
	mu       sync.Mutex
	byTurnID map[string]context.CancelFunc
}

const maxTrackedTurns = 4096

func newTurnTracker() *turnTracker {
	return &turnTracker{byTurnID: make(map[string]context.CancelFunc)}
}

func (tt *turnTracker) register(turnID string, cancel context.CancelFunc) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	if _, exists := tt.byTurnID[turnID]; !exists && len(tt.byTurnID) >= maxTrackedTurns {
		for oldTurnID := range tt.byTurnID {
			delete(tt.byTurnID, oldTurnID)
			break
		}
	}
	tt.byTurnID[turnID] = cancel
}

func (tt *turnTracker) cancel(turnID string) bool {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	c, ok := tt.byTurnID[turnID]
	if !ok {
		return false
	}
	c()
	delete(tt.byTurnID, turnID)
	return true
}

func (tt *turnTracker) deregister(turnID string) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	delete(tt.byTurnID, turnID)
}

// handleTurnSubmit is the central orchestration handler per Bundle 8
// design D4.5. It validates the request, ensures (or creates) a
// session, appends the user turn to the conversation, persists the
// user turn, resolves the model via routing policy, dispatches to the
// upstream provider, and runs the streaming relay in a background
// goroutine while returning an immediate accept response.
//
// The streaming relay runs on a goroutine launched here; cancellation
// is wired through the server's turnTracker so that turn.cancel can
// abort an in-flight stream.
func handleTurnSubmit(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	submit, err := ValidateTurnSubmit(params)
	if err != nil {
		return nil, err
	}

	srv := conn.server

	// Auto-create a new session when the harness sends an empty
	// session_id. This is the first-turn UX: the harness generates
	// a new UUID on startup but the BFF may not have seen it yet.
	if submit.SessionID == "" {
		submit.SessionID = uuid.NewString()
	}

	// Enforce the advertised MaxAttachmentSize (WU-094 H-5). The
	// cap was returned in CapabilitiesRegisterResponse as a
	// contract but no code enforced it, so a client could push
	// up-to-frame-size attachments with every turn. Per-attachment
	// and total caps both apply; total cap is the advertised value,
	// any single attachment must fit under the same bound.
	if max := srv.config.MaxAttachmentSize; max > 0 {
		total := 0
		for i, a := range submit.Attachments {
			size := len(a.Raw) + len(a.Content)
			if size > max {
				return nil, &TransportError{
					Code:    CodeInvalidParams,
					Message: fmt.Sprintf("attachment %d exceeds max_attachment_size (%d > %d bytes)", i, size, max),
				}
			}
			total += size
			if total > max {
				return nil, &TransportError{
					Code:    CodeInvalidParams,
					Message: fmt.Sprintf("attachments total (%d bytes) exceeds max_attachment_size (%d)", total, max),
				}
			}
		}
	}

	// Ensure the session exists in storage; create on first turn (design D2.2).
	sess, _ := srv.store.GetSession(ctx, submit.SessionID)
	if sess != nil {
		if err := verifySessionAccess(conn, sess); err != nil {
			return nil, err
		}
		if sess.UserID == "" {
			sess.UserID = conn.UserID()
		}
	}
	if sess == nil {
		sess = &storage.Session{
			ID:        submit.SessionID,
			UserID:    conn.UserID(),
			Project:   conn.Capabilities().ProjectContext().Root,
			Status:    "active",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := srv.store.CreateSession(ctx, sess); err != nil {
			return nil, &TransportError{Code: CodeInternalError, Message: "create session: " + err.Error()}
		}
		expiry := time.Now().Add(SessionLockTTL)
		if _, _, err := srv.store.AcquireSessionLock(ctx, sess.ID, conn.ID(), expiry); err != nil {
			return nil, &TransportError{Code: CodeInternalError, Message: "acquire lock: " + err.Error()}
		}
	}

	// Bind the connection if it isn't already.
	if conn.SessionID() == "" {
		conn.SetSessionID(submit.SessionID)
	}
	active := srv.sessions.EnsureActive(submit.SessionID, conn)
	if active.UserID == "" {
		active.UserID = sess.UserID
	}
	if active.Project == "" {
		active.Project = sess.Project
	}

	idempotencyKey := submit.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = "turn:" + submit.SessionID + ":" + submit.TurnID
	}
	if existing, err := srv.store.GetRunByIdempotency(ctx, sess.UserID, sess.Project, idempotencyKey); err == nil {
		return &protocol.TurnSubmitResponse{
			TurnID:    submit.TurnID,
			SessionID: submit.SessionID,
			Status:    "accepted",
			RunID:     existing.ID,
		}, nil
	} else if !errors.Is(err, storage.ErrRunNotFound) {
		return nil, &TransportError{Code: CodeInternalError, Message: "lookup run idempotency: " + err.Error()}
	}

	// Append the user turn to the in-memory conversation and persist the
	// foreground run, initial event/checkpoint, turn, run link, and command
	// history in one durable transaction before provider dispatch.
	userTurn, err := active.Conversation.AppendUserTurn(submit)
	if err != nil {
		return nil, err
	}
	history := &storage.CommandHistoryEntry{
		UserID:    active.UserID,
		Project:   active.Project,
		SessionID: stringPtr(active.ID),
		Content:   submit.Content,
		CreatedAt: time.Now().UTC(),
	}
	run, err := createForegroundRunWithTurn(ctx, srv, conn, sess, createRunOptions{
		IdempotencyKey:       idempotencyKey,
		WorkflowType:         storage.RunWorkflowImplementation,
		Title:                submit.Content,
		Status:               storage.RunStatusRunning,
		AttachmentState:      storage.RunAttachmentAttached,
		AttachedConnectionID: conn.ID(),
	}, userTurn, history)
	if err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "create foreground run: " + err.Error()}
	}

	// Resolve the model via routing.
	models, isMulti := srv.routing.ResolveForTurn(active, submit.Mode)
	if len(models) == 0 {
		_, _ = appendRunLifecycle(ctx, conn, run, protocol.EventRunFailed, storage.RunStagePreflight, storage.RunStatusFailed, "model_unavailable", nil)
		return nil, &TransportError{
			Code:    CodeModelUnavailable,
			Message: "no model resolved by routing policy and no session override set",
		}
	}
	if isMulti {
		_, _ = appendRunLifecycle(ctx, conn, run, protocol.EventRunFailed, storage.RunStagePreflight, storage.RunStatusFailed, "multi_model_not_supported", nil)
		// Multi-model fan-out is intentionally NOT implemented at the
		// BFF level. The use case is handled by sub-agents (FEAT-0013):
		// each model runs as its own sub-agent with isolated context;
		// reconciliation (picking / synthesizing across results) is a
		// separate concern handled by a synthesizer agent or a
		// harness-side picker UI. See WU-060 in
		// docs/releases/v0.2.0/track-a-bff-server.md for the deferral
		// rationale.
		return nil, &TransportError{
			Code:    CodeProviderError,
			Message: "multi-model routing is not implemented at the BFF level; use sub-agents (FEAT-0013) for parallel model execution",
		}
	}
	modelName := models[0]
	entry := srv.models.Get(modelName)
	if entry == nil {
		_, _ = appendRunLifecycle(ctx, conn, run, protocol.EventRunFailed, storage.RunStagePreflight, storage.RunStatusFailed, "model_unavailable", nil)
		return nil, &TransportError{
			Code:    CodeModelUnavailable,
			Message: fmt.Sprintf("model %q not in registry", modelName),
		}
	}
	_, _ = appendRunLifecycle(ctx, conn, run, protocol.EventRunStageChanged, storage.RunStagePromptPlan, storage.RunStatusRunning, "", map[string]string{"model": modelName, "provider": entry.Provider})
	_, _ = appendRunLifecycle(ctx, conn, run, protocol.EventRunStageChanged, storage.RunStageModelCall, storage.RunStatusRunning, "", map[string]string{"model": modelName, "provider": entry.Provider})

	// Build dispatch options. The system prompt is reassembled per turn.
	var prompt string
	var promptTokens int
	if srv.prompts != nil {
		srv.prompts.SetWindowSize(entry.Info.ContextWindow)
		prompt, promptTokens = srv.prompts.Assemble(conn.Capabilities(), active, submit.Mode)
	}
	_ = promptTokens

	dispatchOpts := DispatchOpts{
		Conversation: active.Conversation,
		SystemPrompt: prompt,
		Model:        modelName,
		EndpointName: entry.Provider,
		RunID:        run.ID,
		TraceID:      run.TraceID,
		Tools:        conn.Capabilities().Tools(),
		Stream:       true,
		WindowSize:   entry.Info.ContextWindow,
		// PATCH-0022: a positive max_tokens is required by Anthropic
		// and ignored-when-larger-than-supported by OpenAI. Zero (Go's
		// default) reaches the wire as "max_tokens": 0 and produces a
		// 400 from Anthropic. 4096 is the conservative fallback used
		// by transform.go and accepted by every v0.3.0 catalog model.
		MaxTokens: 4096,
	}

	// Provider dispatch (synchronous: returns the streaming response).
	resp, err := srv.dispatch.Dispatch(ctx, dispatchOpts)
	if err != nil {
		_, _ = appendRunLifecycle(ctx, conn, run, protocol.EventRunFailed, storage.RunStageModelCall, storage.RunStatusFailed, "provider_dispatch_failed", map[string]string{"error": err.Error()})
		return nil, err
	}

	// Hand off to the streaming relay on a background goroutine. The
	// caller's response (TurnSubmitResponse) returns immediately with
	// status="accepted"; events flow asynchronously via notifications.
	relayCtx, cancel := context.WithCancel(ctx)
	srv.turns.register(submit.TurnID, cancel)
	srv.runs.register(run.ID, submit.TurnID, submit.SessionID, conn.ID(), cancel)

	relay := NewStreamRelay(conn, active, submit.TurnID, "", modelName, entry.Provider).WithRun(run.ID)
	go func() {
		defer srv.turns.deregister(submit.TurnID)
		defer srv.runs.deregister(run.ID, submit.TurnID)
		turn, relayErr := relay.Relay(relayCtx, resp.Body, srv.adapterFor(entry.Provider))
		if relayErr != nil {
			slog.Error("run relay failed", "run_id", run.ID, "turn_id", submit.TurnID, "error", relayErr)
			if got, err := srv.store.GetRun(context.Background(), run.ID); err == nil && !isTerminalRunStatus(got.Status) {
				_, _ = appendRunLifecycle(context.Background(), conn, got, protocol.EventRunFailed, got.Stage, storage.RunStatusFailed, "relay_failed", map[string]string{"error": relayErr.Error()})
			}
		}
		if turn != nil && srv.cost != nil {
			srv.cost.UpdateAfterTurn(ctx, conn, active, turn)
		}
	}()

	return &protocol.TurnSubmitResponse{
		TurnID:    submit.TurnID,
		SessionID: submit.SessionID,
		Status:    "accepted",
		RunID:     run.ID,
	}, nil
}

// handleTurnCancel cancels an in-flight turn. Returns accepted=false
// when the turn is unknown — typically because it has already
// completed or never existed.
func handleTurnCancel(_ context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.TurnCancel
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode turn.cancel: " + err.Error()}
	}
	if req.TurnID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "turn_id is required"}
	}
	runID := conn.server.runs.runIDForTurn(req.TurnID)
	cancelled := false
	if runID != "" {
		cancelled = conn.server.runs.cancel(runID)
	}
	if !cancelled {
		cancelled = conn.server.turns.cancel(req.TurnID)
	}
	return &protocol.TurnCancelResponse{TurnID: req.TurnID, Accepted: cancelled}, nil
}

// handleToolResult records a tool result on the active session. The
// result is appended to the conversation as a user-role message
// carrying the ToolResult; the assistant's next turn includes the
// tool result in its context. (For a v1 simplification, results are
// treated as inline pieces of the next turn.submit; the standalone
// tool.result method just stages them.)
func handleToolResult(_ context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.ToolResultRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode tool.result: " + err.Error()}
	}
	if req.ToolCallID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "tool_call_id is required"}
	}

	srv := conn.server
	if conn.SessionID() == "" {
		return nil, &TransportError{Code: CodeNotReady, Message: "no active session bound to this connection"}
	}
	active := srv.sessions.GetActiveSession(conn.SessionID())
	if active == nil {
		return nil, &TransportError{Code: CodeSessionNotFound, Message: "active session not found"}
	}
	if _, ok := active.Conversation.MatchToolResult(req.ToolCallID); !ok {
		return nil, &TransportError{
			Code:    CodeInvalidParams,
			Message: fmt.Sprintf("tool_call_id %q does not match any pending call", req.ToolCallID),
		}
	}
	// Stage the result for the next turn by appending a user-role message.
	next := protocol.TurnSubmit{
		TurnID:      newID(),
		SessionID:   conn.SessionID(),
		Sequence:    active.Conversation.Sequence() + 1,
		Mode:        protocol.ModeBuild,
		Content:     "",
		ToolResults: []protocol.ToolResult{req},
	}
	turn, err := active.Conversation.AppendUserTurn(&next)
	if err != nil {
		return nil, err
	}
	if err := srv.store.CreateTurn(context.Background(), turn); err != nil {
		return nil, &TransportError{Code: CodeInternalError, Message: "persist tool result turn: " + err.Error()}
	}
	return &protocol.ToolResultResponse{ToolCallID: req.ToolCallID, Accepted: true}, nil
}

func stringPtr(s string) *string { return &s }

// newID is a small wrapper so handlers can mint UUIDs without each
// growing its own import.
func newID() string {
	return fmt.Sprintf("turn-%d", time.Now().UnixNano())
}

// adapterFor returns the registered Provider adapter for the given
// endpoint name, or nil when the endpoint is unknown. Wired on Server.
// This is used by the streaming relay path to find the per-event
// parser.
func (s *Server) adapterFor(endpointName string) provider.Provider {
	if s == nil || s.providers == nil || s.adapters == nil {
		return nil
	}
	ep := s.providers.Get(endpointName)
	if ep == nil {
		return nil
	}
	return s.adapters.Get(ep.Type)
}

// Pull errors into the package's import surface so future error
// constants land consistently.
var _ = errors.New
