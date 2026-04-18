package bff

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

func newTrackerWithModels(t *testing.T) (*CostTracker, *Server) {
	t.Helper()
	srv := newServerWithRealStore(t)
	_ = srv.providers.Add(&ProviderEndpoint{Name: "ant", Type: ProviderTypeAnthropic, APIKey: "k"})
	srv.models.Refresh()
	return NewCostTracker(srv.models, srv.store), srv
}

func TestCost_ComputeTurnCost(t *testing.T) {
	tr, _ := newTrackerWithModels(t)
	// claude-sonnet-4-6 builtins: 0.003 input, 0.015 output per 1k.
	got := tr.ComputeTurnCost("claude-sonnet-4-6", 1000, 1000)
	want := 0.003 + 0.015
	if got < want-1e-9 || got > want+1e-9 {
		t.Errorf("cost = %v, want %v", got, want)
	}
}

func TestCost_UnknownModel_ZeroCost(t *testing.T) {
	tr, _ := newTrackerWithModels(t)
	if got := tr.ComputeTurnCost("does-not-exist", 1000, 1000); got != 0 {
		t.Errorf("unknown model cost = %v, want 0", got)
	}
}

func TestCost_UpdateAfterTurn_PersistsAndEmits(t *testing.T) {
	tr, srv := newTrackerWithModels(t)

	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "cost test")
	c, frames := newRelayConnection(t, srv)
	c.SetSessionID(sid)
	active := srv.sessions.EnsureActive(sid, c)
	active.UserID = SoloUserID
	active.Project = "/tmp/proj"

	turn := &storage.Turn{
		ID:           "t1",
		SessionID:    sid,
		Sequence:     1,
		Role:         "assistant",
		Model:        "claude-sonnet-4-6",
		InputTokens:  500,
		OutputTokens: 1500,
	}

	cost := tr.UpdateAfterTurn(context.Background(), c, active, turn)
	wantCost := (500.0/1000.0)*0.003 + (1500.0/1000.0)*0.015
	if cost < wantCost-1e-9 || cost > wantCost+1e-9 {
		t.Errorf("cost = %v, want %v", cost, wantCost)
	}
	if turn.Cost != cost {
		t.Errorf("turn.Cost = %v, want %v", turn.Cost, cost)
	}
	if active.TotalCost != cost {
		t.Errorf("session total cost = %v, want %v", active.TotalCost, cost)
	}
	if active.TotalInputTokens != 500 || active.TotalOutputTokens != 1500 {
		t.Errorf("token totals = (%d, %d)", active.TotalInputTokens, active.TotalOutputTokens)
	}

	updates := frames.waitForFrame(t, protocol.EventCostUpdate)
	if len(updates) != 1 {
		t.Fatalf("cost.update count = %d", len(updates))
	}
	var cu protocol.CostUpdate
	_ = json.Unmarshal(updates[0], &cu)
	if cu.InputTokens != 500 || cu.OutputTokens != 1500 {
		t.Errorf("cost.update tokens = (%d, %d)", cu.InputTokens, cu.OutputTokens)
	}
	if cu.TotalCost != cost {
		t.Errorf("cost.update total = %v, want %v", cu.TotalCost, cost)
	}
}

func TestCost_UpdateAfterTurn_AccumulatesAcrossTurns(t *testing.T) {
	tr, srv := newTrackerWithModels(t)
	sid := seedSession(t, srv.store, SoloUserID, "/tmp/proj", "accumulate")
	c, _ := newRelayConnection(t, srv)
	c.SetSessionID(sid)
	active := srv.sessions.EnsureActive(sid, c)

	for i := 0; i < 3; i++ {
		turn := &storage.Turn{
			SessionID: sid, Sequence: i + 1, Role: "assistant",
			Model: "claude-sonnet-4-6", InputTokens: 100, OutputTokens: 100,
		}
		tr.UpdateAfterTurn(context.Background(), c, active, turn)
	}
	wantTotal := 3 * ((100.0/1000.0)*0.003 + (100.0/1000.0)*0.015)
	if active.TotalCost < wantTotal-1e-9 || active.TotalCost > wantTotal+1e-9 {
		t.Errorf("total cost = %v, want %v", active.TotalCost, wantTotal)
	}
	if active.TotalInputTokens != 300 || active.TotalOutputTokens != 300 {
		t.Errorf("token totals = (%d, %d)", active.TotalInputTokens, active.TotalOutputTokens)
	}
}

func TestCost_NilGuards(t *testing.T) {
	tr, _ := newTrackerWithModels(t)
	if got := tr.UpdateAfterTurn(context.Background(), nil, nil, nil); got != 0 {
		t.Errorf("nil inputs should yield 0 cost; got %v", got)
	}
}
