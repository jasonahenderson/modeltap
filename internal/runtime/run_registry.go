package runtime

import (
	"context"
	"sync"
)

type runRegistry struct {
	mu       sync.Mutex
	byRunID  map[string]*activeRun
	byTurnID map[string]string
}

const maxTrackedRuns = 4096

type activeRun struct {
	id                   string
	turnID               string
	sessionID            string
	attachedConnectionID string
	cancel               context.CancelFunc
}

func newRunRegistry() *runRegistry {
	return &runRegistry{
		byRunID:  make(map[string]*activeRun),
		byTurnID: make(map[string]string),
	}
}

func (rr *runRegistry) register(runID, turnID, sessionID, connectionID string, cancel context.CancelFunc) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if existing := rr.byRunID[runID]; existing != nil && existing.turnID != "" && existing.turnID != turnID {
		delete(rr.byTurnID, existing.turnID)
	}
	if _, exists := rr.byRunID[runID]; !exists && len(rr.byRunID) >= maxTrackedRuns {
		for oldRunID, oldRun := range rr.byRunID {
			delete(rr.byRunID, oldRunID)
			if oldRun.turnID != "" {
				delete(rr.byTurnID, oldRun.turnID)
			}
			break
		}
	}
	rr.byRunID[runID] = &activeRun{id: runID, turnID: turnID, sessionID: sessionID, attachedConnectionID: connectionID, cancel: cancel}
	if turnID != "" {
		rr.byTurnID[turnID] = runID
	}
}

func (rr *runRegistry) cancel(runID string) bool {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	r, ok := rr.byRunID[runID]
	if !ok || r.cancel == nil {
		return false
	}
	r.cancel()
	delete(rr.byRunID, runID)
	if r.turnID != "" {
		delete(rr.byTurnID, r.turnID)
	}
	return true
}

func (rr *runRegistry) runIDForTurn(turnID string) string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.byTurnID[turnID]
}

func (rr *runRegistry) deregister(runID, turnID string) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	delete(rr.byRunID, runID)
	if turnID != "" {
		delete(rr.byTurnID, turnID)
	}
}
