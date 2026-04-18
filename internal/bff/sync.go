package bff

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// handleSessionSync implements the session.sync recovery handler per
// FEAT-0008 §"Reconnection flow". A reconnecting harness calls this to
// learn the in-flight turn state (active turn, pending tool calls,
// completed token count) so it can resume rendering without
// re-streaming from the start.
//
// Today the BFF doesn't yet retain a token-replay buffer (that's a
// future addition). The response carries the active turn id when one
// is in flight and reports token_replay_available=false.
func handleSessionSync(_ context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.SessionSync
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode session.sync: " + err.Error()}
	}
	if req.SessionID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "session_id is required"}
	}

	srv := conn.server
	active := srv.sessions.GetActiveSession(req.SessionID)
	if active == nil {
		return nil, &TransportError{
			Code:    CodeSessionNotFound,
			Message: fmt.Sprintf("session %q is not active on this server", req.SessionID),
		}
	}

	// Pending tool calls from the most recent assistant turn are the
	// canonical "is the harness expected to send tool.result?" signal.
	pending := active.Conversation.PendingToolCalls()
	pendingProto := make([]protocol.PendingToolCall, 0, len(pending))
	for _, tc := range pending {
		pendingProto = append(pendingProto, protocol.PendingToolCall{
			ToolCallID: tc.ID,
			Tool:       tc.Name,
			Status:     "awaiting_result",
		})
	}

	// Look up an in-flight turn id, if any. The presence in turnTracker
	// means a streaming goroutine is still running.
	srv.turns.mu.Lock()
	var inFlightID string
	for id := range srv.turns.byTurnID {
		// Pick any in-flight turn — single-model invariant means at
		// most one will exist per session today.
		inFlightID = id
		break
	}
	srv.turns.mu.Unlock()

	status := "complete"
	if inFlightID != "" {
		status = "streaming"
	} else if len(pending) > 0 {
		status = "pending_tool_result"
	}

	resp := &protocol.SessionSyncResponse{
		SessionID: req.SessionID,
		ActiveTurn: protocol.ActiveTurnState{
			TurnID:               inFlightID,
			Status:               status,
			PendingToolCalls:     pendingProto,
			TokenReplayAvailable: false,
		},
	}
	return resp, nil
}
