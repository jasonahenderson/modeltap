package bff

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// RoutingPolicy resolves model names from a hierarchical routing config
// using the FEAT-0008 flat 3-step algorithm. Safe for concurrent use.
//
// Example tree:
//
//	"default":      "claude-sonnet-4-6"
//	"coding.review": ["claude-opus-4-6", "gpt-5"]
//	"coding.default": "claude-sonnet-4-6"
//	"cheap":        "claude-haiku-4-5"
type RoutingPolicy struct {
	mu   sync.RWMutex
	tree protocol.RoutingPolicy
}

// NewRoutingPolicy returns an empty policy. Populate via Replace.
func NewRoutingPolicy() *RoutingPolicy {
	return &RoutingPolicy{tree: protocol.RoutingPolicy{}}
}

// Replace atomically swaps the routing tree.
func (rp *RoutingPolicy) Replace(tree protocol.RoutingPolicy) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.tree = tree
}

// Tree returns a shallow copy of the routing tree.
func (rp *RoutingPolicy) Tree() protocol.RoutingPolicy {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	out := make(protocol.RoutingPolicy, len(rp.tree))
	for k, v := range rp.tree {
		out[k] = v
	}
	return out
}

// Resolve walks the 3-step dot-path resolution per design D4.1.
// Returns (models, isMulti, found). Example with path "coding.review":
//  1. Look up "coding.review"   → if present, return
//  2. Look up "coding.default"  → if present, return
//  3. Look up "default"         → if present, return
//
// If path has no dot and is not "default", step 2 is skipped (step 1
// already tried the segment itself; step 3 falls back to "default").
func (rp *RoutingPolicy) Resolve(path string) ([]string, bool, bool) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	// Step 1: exact match.
	if models, isMulti, ok := extract(rp.tree, path); ok {
		return models, isMulti, true
	}

	// Step 2: replace last segment with "default".
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		fallback := path[:idx] + ".default"
		if models, isMulti, ok := extract(rp.tree, fallback); ok {
			return models, isMulti, true
		}
	}

	// Step 3: top-level default.
	if path != "default" {
		if models, isMulti, ok := extract(rp.tree, "default"); ok {
			return models, isMulti, true
		}
	}
	return nil, false, false
}

// ResolveForTurn picks models for a turn, honoring the session override
// first and falling back to routing policy keyed by mode per design D4.2.
func (rp *RoutingPolicy) ResolveForTurn(session *ActiveSession, mode protocol.Mode) ([]string, bool) {
	if session != nil && session.ModelOverride != "" {
		return []string{session.ModelOverride}, false
	}
	path := string(mode)
	if models, isMulti, ok := rp.Resolve(path); ok {
		return models, isMulti
	}
	// If nothing matched, return nothing — caller decides on error surface.
	return nil, false
}

// extract returns the models list and isMulti flag for a tree value.
// Accepts a JSON string (single model) or a JSON array of strings.
func extract(tree protocol.RoutingPolicy, key string) ([]string, bool, bool) {
	raw, ok := tree[key]
	if !ok {
		return nil, false, false
	}
	// Try string first.
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, false, true
	}
	// Try array.
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		if len(arr) == 0 {
			return nil, false, false
		}
		return arr, len(arr) > 1, true
	}
	return nil, false, false
}

// -----------------------------------------------------------------------
// Handlers (model.list, model.switch) — design D4.3, D4.4
// -----------------------------------------------------------------------

// handleModelList returns the full catalog, the active override, and
// the routing tree.
func handleModelList(_ context.Context, conn *Connection, _ json.RawMessage) (any, error) {
	srv := conn.server
	if srv.models == nil {
		return &protocol.ModelListResponse{
			Models:        []protocol.ModelInfo{},
			RoutingPolicy: protocol.RoutingPolicy{},
		}, nil
	}
	resp := &protocol.ModelListResponse{
		Models: srv.models.All(),
	}
	if srv.routing != nil {
		resp.RoutingPolicy = srv.routing.Tree()
	} else {
		resp.RoutingPolicy = protocol.RoutingPolicy{}
	}

	// CurrentOverride comes from the active session bound to this
	// connection, not from any particular session the client named.
	if sid := conn.SessionID(); sid != "" {
		if active := srv.sessions.GetActiveSession(sid); active != nil {
			resp.CurrentOverride = active.ModelOverride
		}
	}
	return resp, nil
}

// handleModelSwitch applies (or clears) a session-level model override.
// "auto" is the sentinel to clear.
func handleModelSwitch(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.ModelSwitch
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "decode model.switch: " + err.Error()}
	}
	if req.SessionID == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "session_id is required"}
	}
	if req.Model == "" {
		return nil, &TransportError{Code: CodeInvalidParams, Message: "model is required (use \"auto\" to clear)"}
	}

	srv := conn.server
	sess, _ := srv.store.GetSession(ctx, req.SessionID)
	if sess != nil {
		if err := verifySessionAccess(conn, sess); err != nil {
			return nil, err
		}
	}
	if sess == nil {
		// Auto-create session so model.switch works before the first
		// turn.submit (consistent with handleTurnSubmit).
		sess = &storage.Session{
			ID:        req.SessionID,
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

	// Ensure the connection is bound to this session.
	if conn.SessionID() == "" {
		conn.SetSessionID(req.SessionID)
	}
	_ = srv.sessions.EnsureActive(req.SessionID, conn)

	var resp protocol.ModelSwitchResponse
	if req.Model == "auto" {
		// Clear override.
		sess.ModelOverride = nil
		if err := srv.store.UpdateSession(ctx, sess); err != nil {
			return nil, &TransportError{Code: CodeInternalError, Message: "persist session: " + err.Error()}
		}
		if active := srv.sessions.GetActiveSession(req.SessionID); active != nil {
			active.ModelOverride = ""
		}
		resp = protocol.ModelSwitchResponse{OverrideSet: false, Reason: "override cleared"}
	} else {
		if srv.models == nil || !srv.models.Has(req.Model) {
			diag := protocol.Diagnostic{
				Code:     protocol.DiagModelUnavailable,
				Category: "model",
				Cause:    fmt.Sprintf("model %q is not in the registry", req.Model),
			}
			diagRaw, _ := json.Marshal(diag)
			return nil, &TransportError{
				Code:    CodeModelUnavailable,
				Message: fmt.Sprintf("model %q is not available", req.Model),
				Data:    json.RawMessage(diagRaw),
			}
		}
		override := req.Model
		sess.ModelOverride = &override
		if err := srv.store.UpdateSession(ctx, sess); err != nil {
			return nil, &TransportError{Code: CodeInternalError, Message: "persist session: " + err.Error()}
		}
		if active := srv.sessions.GetActiveSession(req.SessionID); active != nil {
			active.ModelOverride = req.Model
		}
		resp = protocol.ModelSwitchResponse{OverrideSet: true, Model: req.Model, Reason: "override applied"}

		// Emit model.selected notification (best-effort).
		modelRaw, _ := json.Marshal(req.Model)
		providerRaw := json.RawMessage(`""`)
		if entry := srv.models.Get(req.Model); entry != nil {
			providerRaw, _ = json.Marshal(entry.Provider)
		}
		selected := protocol.ModelSelected{
			Model:    modelRaw,
			Provider: providerRaw,
			Reason:   "model.switch",
		}
		raw, _ := json.Marshal(selected)
		_ = conn.transport.SendNotification(&protocol.Notification{
			JSONRPC: "2.0",
			Method:  protocol.EventModelSelected,
			Params:  raw,
		})
	}

	return &resp, nil
}
