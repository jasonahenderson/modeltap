package bff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// Allowed enum values for tool catalog validation per design D5.2.
var (
	validRiskLevels = map[string]bool{
		"read_only":   true,
		"write":       true,
		"execute":     true,
		"destructive": true,
	}
	validOutputEnvelopes = map[string]bool{
		"text":   true,
		"json":   true,
		"binary": true,
		"image":  true,
	}
)

// CapabilityManager tracks tools, protocol version, and project context
// for a single connection. All methods are safe for concurrent use.
type CapabilityManager struct {
	mu sync.RWMutex

	negotiatedVersion string
	tools             map[string]protocol.ToolDefinition // keyed by "namespace:name"
	project           protocol.ProjectContext
}

// NewCapabilityManager returns an empty manager. Populated on
// capabilities.register.
func NewCapabilityManager() *CapabilityManager {
	return &CapabilityManager{
		tools: make(map[string]protocol.ToolDefinition),
	}
}

// NegotiatedVersion returns the protocol version selected during
// registration, or "" if registration has not completed.
func (cm *CapabilityManager) NegotiatedVersion() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.negotiatedVersion
}

// ProjectContext returns the current project context.
func (cm *CapabilityManager) ProjectContext() protocol.ProjectContext {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.project
}

// UpdateProjectContext replaces the stored project context. Called on
// session.resume with the harness's current config snapshot.
func (cm *CapabilityManager) UpdateProjectContext(pc protocol.ProjectContext) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.project = pc
}

// Tools returns a defensive copy of the current tool catalog. Callers
// may mutate the returned slice without affecting manager state.
func (cm *CapabilityManager) Tools() []protocol.ToolDefinition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	out := make([]protocol.ToolDefinition, 0, len(cm.tools))
	for _, t := range cm.tools {
		out = append(out, t)
	}
	return out
}

// RequestReregistration sends a capabilities.request notification asking
// the harness to re-send capabilities.register. reason is one of
// FEAT-0008's known reasons ("reconnection", "tool_schema_drift") or a
// caller-defined string.
func (cm *CapabilityManager) RequestReregistration(conn *Connection, reason string) error {
	params, err := json.Marshal(&protocol.CapabilitiesRequestEvent{Reason: reason})
	if err != nil {
		return fmt.Errorf("marshal reason: %w", err)
	}
	return conn.transport.SendNotification(&protocol.Notification{
		JSONRPC: "2.0",
		Method:  protocol.EventCapabilitiesRequest,
		Params:  params,
	})
}

// setVersion is an internal mutator used by handleCapabilitiesRegister
// and tests.
func (cm *CapabilityManager) setVersion(v string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.negotiatedVersion = v
}

// replaceTools atomically swaps the full tool catalog. Internal to the
// register handler (and tests).
func (cm *CapabilityManager) replaceTools(tools []protocol.ToolDefinition) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.tools = make(map[string]protocol.ToolDefinition, len(tools))
	for _, t := range tools {
		cm.tools[toolKey(t)] = t
	}
}

// applyUpdate atomically applies an add/remove update. Returns the
// number of tools added and removed.
func (cm *CapabilityManager) applyUpdate(added []protocol.ToolDefinition, removed []string) (int, int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, name := range removed {
		// Removed names are plain names; locate any entry with matching Name.
		for k, t := range cm.tools {
			if t.Name == name {
				delete(cm.tools, k)
			}
		}
	}
	for _, t := range added {
		cm.tools[toolKey(t)] = t
	}
	return len(added), len(removed)
}

func toolKey(t protocol.ToolDefinition) string {
	if t.Namespace == "" {
		return t.Name
	}
	return t.Namespace + ":" + t.Name
}

// validateTool returns "" when t passes the D5.2 catalog validation or
// a human-readable reason otherwise. The reason is propagated into
// RejectedTool.Reason.
func validateTool(t protocol.ToolDefinition) string {
	if strings.TrimSpace(t.Name) == "" {
		return "name is required"
	}
	if strings.TrimSpace(t.Description) == "" {
		return "description is required"
	}
	if len(t.InputSchema) == 0 {
		return "input_schema is required"
	}
	var probe any
	if err := json.Unmarshal(t.InputSchema, &probe); err != nil {
		return "input_schema is not valid JSON: " + err.Error()
	}
	if !validRiskLevels[t.RiskLevel] {
		return "risk_level must be one of read_only, write, execute, destructive"
	}
	if !validOutputEnvelopes[t.OutputEnvelope] {
		return "output_envelope must be one of text, json, binary, image"
	}
	return ""
}

// handleCapabilitiesRegister implements the capabilities.register handler
// per design D5.2. It enforces single registration (state must be
// ConnRegistering), negotiates protocol version, partitions the tool
// catalog into registered/rejected, captures project context, and
// transitions the connection to ConnReady on success.
func handleCapabilitiesRegister(_ context.Context, conn *Connection, params json.RawMessage) (any, error) {
	// Replay prevention: the handler MUST only run in ConnRegistering.
	// The dispatch gate already allows capabilities.register in
	// ConnRegistering; once the connection is Ready/Degraded, a replay
	// would bypass the gate. Enforce it here.
	if conn.State() != ConnRegistering {
		return nil, &TransportError{
			Code:    CodeNotReady,
			Message: "capabilities.register already completed on this connection",
		}
	}

	var req protocol.CapabilitiesRegister
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{
			Code:    CodeInvalidParams,
			Message: "decode capabilities.register: " + err.Error(),
		}
	}

	if req.ProtocolVersion != protocol.ProtocolVersion {
		conn.transition(ConnFailed)
		return nil, &TransportError{
			Code: CodeVersionMismatch,
			Message: fmt.Sprintf("protocol version %q not supported; server speaks %q",
				req.ProtocolVersion, protocol.ProtocolVersion),
		}
	}

	// Partition the tool catalog.
	registered := make([]protocol.ToolDefinition, 0, len(req.Tools))
	rejected := make([]protocol.RejectedTool, 0)
	for _, t := range req.Tools {
		if reason := validateTool(t); reason != "" {
			rejected = append(rejected, protocol.RejectedTool{Name: t.Name, Reason: reason})
			continue
		}
		registered = append(registered, t)
	}

	cm := conn.Capabilities()
	cm.setVersion(req.ProtocolVersion)
	cm.replaceTools(registered)
	cm.UpdateProjectContext(req.Project)

	// Only transition if we stay in a valid state. transition() is a
	// no-op on an invalid edge, but this call is Registering->Ready so
	// it always succeeds from this handler.
	conn.transition(ConnReady)

	cfg := conn.server.Config()
	maxAttach := cfg.MaxAttachmentSize
	if maxAttach <= 0 {
		maxAttach = 5 * 1024 * 1024
	}
	return &protocol.CapabilitiesRegisterResponse{
		Registered: registered,
		Rejected:   rejected,
		ServerCapabilities: protocol.ServerCapabilities{
			ProtocolVersion:   protocol.ProtocolVersion,
			MaxFrameSize:      protocol.MaxFrameSize,
			MaxAttachmentSize: maxAttach,
		},
	}, nil
}

// handleCapabilitiesUpdate implements capabilities.update per design D5.3.
// If any added tool fails validation, the entire update is rejected
// atomically (no partial state mutation).
func handleCapabilitiesUpdate(_ context.Context, conn *Connection, params json.RawMessage) (any, error) {
	var req protocol.CapabilitiesUpdate
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &TransportError{
			Code:    CodeInvalidParams,
			Message: "decode capabilities.update: " + err.Error(),
		}
	}

	for _, t := range req.AddedTools {
		if reason := validateTool(t); reason != "" {
			return nil, &TransportError{
				Code:    CodeCapabilityError,
				Message: fmt.Sprintf("rejected tool %q: %s", t.Name, reason),
			}
		}
	}

	added, removed := conn.Capabilities().applyUpdate(req.AddedTools, req.RemovedTools)
	return &protocol.CapabilitiesUpdateResponse{
		AddedCount:   added,
		RemovedCount: removed,
		UpdatedAt:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}

// Ensure errors package is imported via an unreferenced alias for
// future error-wrapping needs; avoids churn when capabilities handlers
// later use errors.Join.
var _ = errors.New
