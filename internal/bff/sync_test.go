package bff

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
)

func TestSessionSync_NotActive(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	params, _ := json.Marshal(&protocol.SessionSync{SessionID: "missing"})
	_, err := handleSessionSync(context.Background(), c, params)
	if err == nil {
		t.Fatalf("expected not-found")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeSessionNotFound {
		t.Errorf("expected CodeSessionNotFound, got %T %v", err, err)
	}
}

func TestSessionSync_StatusComplete(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	c.SetSessionID("sess-1")
	srv.sessions.EnsureActive("sess-1", c)

	params, _ := json.Marshal(&protocol.SessionSync{SessionID: "sess-1"})
	resp, err := handleSessionSync(context.Background(), c, params)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	sr := resp.(*protocol.SessionSyncResponse)
	if sr.ActiveTurn.Status != "complete" {
		t.Errorf("status = %q, want complete", sr.ActiveTurn.Status)
	}
	if sr.ActiveTurn.TokenReplayAvailable {
		t.Errorf("TokenReplayAvailable should be false")
	}
}

func TestSessionSync_PendingTool(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	c.SetSessionID("sess-1")
	active := srv.sessions.EnsureActive("sess-1", c)

	_, _ = active.Conversation.AppendUserTurn(&protocol.TurnSubmit{
		TurnID: "u1", SessionID: "sess-1", Sequence: 1, Mode: protocol.ModeBuild, Content: "go",
	})
	_, _ = active.Conversation.AppendAssistantTurn(AssistantResponse{
		Content: "calling read",
		Model:   "claude-sonnet-4-6",
		ToolCalls: []provider.ToolCall{
			{ID: "call-a", Name: "read"},
		},
	})

	params, _ := json.Marshal(&protocol.SessionSync{SessionID: "sess-1"})
	resp, err := handleSessionSync(context.Background(), c, params)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	sr := resp.(*protocol.SessionSyncResponse)
	if sr.ActiveTurn.Status != "pending_tool_result" {
		t.Errorf("status = %q, want pending_tool_result", sr.ActiveTurn.Status)
	}
	if len(sr.ActiveTurn.PendingToolCalls) != 1 ||
		sr.ActiveTurn.PendingToolCalls[0].ToolCallID != "call-a" {
		t.Errorf("pending = %+v", sr.ActiveTurn.PendingToolCalls)
	}
}

func TestSessionSync_StreamingActiveTurn(t *testing.T) {
	srv := newServerWithRealStore(t)
	c := newReadyConnection(t, srv)
	c.SetSessionID("sess-1")
	srv.sessions.EnsureActive("sess-1", c)

	srv.turns.register("turn-active", func() {})
	t.Cleanup(func() { srv.turns.deregister("turn-active") })

	params, _ := json.Marshal(&protocol.SessionSync{SessionID: "sess-1"})
	resp, err := handleSessionSync(context.Background(), c, params)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	sr := resp.(*protocol.SessionSyncResponse)
	if sr.ActiveTurn.TurnID != "turn-active" {
		t.Errorf("TurnID = %q, want turn-active", sr.ActiveTurn.TurnID)
	}
	if sr.ActiveTurn.Status != "streaming" {
		t.Errorf("status = %q, want streaming", sr.ActiveTurn.Status)
	}
}
