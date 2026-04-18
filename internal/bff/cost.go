package bff

import (
	"context"
	"encoding/json"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// CostTracker computes per-turn cost from token counts and the model
// registry's pricing table, maintains running session totals, and emits
// cost.update notifications. Persistence is delegated to the same
// storage.Store the rest of the BFF uses.
type CostTracker struct {
	registry *ModelRegistry
	store    storage.Store
}

// NewCostTracker constructs a tracker rooted at the given registry +
// store. Both are required.
func NewCostTracker(registry *ModelRegistry, store storage.Store) *CostTracker {
	return &CostTracker{registry: registry, store: store}
}

// ComputeTurnCost returns the per-turn cost in dollars given the model
// and token counts. Unknown models return 0 (no pricing data); callers
// should not surface this as an error — the catalog can lag behind the
// providers' real catalogs.
func (ct *CostTracker) ComputeTurnCost(model string, inputTokens, outputTokens int64) float64 {
	if ct.registry == nil {
		return 0
	}
	entry := ct.registry.Get(model)
	if entry == nil {
		return 0
	}
	inputCost := float64(inputTokens) / 1000.0 * entry.Info.CostPer1kInput
	outputCost := float64(outputTokens) / 1000.0 * entry.Info.CostPer1kOutput
	return inputCost + outputCost
}

// UpdateAfterTurn stamps the per-turn cost on turn, accumulates into
// the session's running totals, persists the session, and emits the
// cost.update notification on conn. It is safe to pass nil for conn
// (no notification) or store (no persistence) — those paths short-
// circuit so the tracker can be used in tests.
func (ct *CostTracker) UpdateAfterTurn(ctx context.Context, conn *Connection, session *ActiveSession, turn *storage.Turn) float64 {
	if turn == nil || session == nil {
		return 0
	}
	turnCost := ct.ComputeTurnCost(turn.Model, turn.InputTokens, turn.OutputTokens)
	turn.Cost = turnCost

	session.TotalCost += turnCost
	session.TotalInputTokens += turn.InputTokens
	session.TotalOutputTokens += turn.OutputTokens

	if ct.store != nil {
		// Best-effort persistence; the relay path keeps streaming the
		// next turn even if the session row write fails.
		_ = ct.store.UpdateSession(ctx, &storage.Session{
			ID:                session.ID,
			TotalCost:         session.TotalCost,
			TotalInputTokens:  session.TotalInputTokens,
			TotalOutputTokens: session.TotalOutputTokens,
		})
	}

	if conn != nil {
		ct.emitCostUpdate(conn, turn)
	}
	return turnCost
}

// emitCostUpdate sends the cost.update notification per design D5.3.
// Best-effort: write errors mean the harness will pick up totals via
// turn.complete or session.details instead.
func (ct *CostTracker) emitCostUpdate(conn *Connection, turn *storage.Turn) {
	entry := ct.registry.Get(turn.Model)
	var inputCost, outputCost float64
	if entry != nil {
		inputCost = float64(turn.InputTokens) / 1000.0 * entry.Info.CostPer1kInput
		outputCost = float64(turn.OutputTokens) / 1000.0 * entry.Info.CostPer1kOutput
	}
	ev := protocol.CostUpdate{
		InputTokens:  int(turn.InputTokens),
		OutputTokens: int(turn.OutputTokens),
		InputCost:    inputCost,
		OutputCost:   outputCost,
		TotalCost:    turn.Cost,
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_ = conn.transport.SendNotification(&protocol.Notification{
		JSONRPC: "2.0",
		Method:  protocol.EventCostUpdate,
		Params:  raw,
	})
}
