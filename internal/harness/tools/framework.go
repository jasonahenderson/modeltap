// Package tools implements the modeltap harness tool framework. The
// framework owns the Tool interface, the Registry that holds in-tree
// and MCP-sourced tools, an Executor that combines permission gating
// with execution, and supporting types (FileTracker, dangerous-command
// detection).
//
// The 13 built-in tool implementations land in WU-076 / WU-077 /
// WU-078 / WU-079; this file holds only the framework primitives.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// RiskLevel is one of the four wire-legal risk classifications. The
// values are exactly the strings the runtime server expects in
// capabilities.register / ToolDefinition.RiskLevel.
type RiskLevel string

const (
	RiskReadOnly    RiskLevel = "read_only"
	RiskWrite       RiskLevel = "write"
	RiskExecute     RiskLevel = "execute"
	RiskDestructive RiskLevel = "destructive"
)

// IsValid reports whether r is one of the four wire-legal risk levels.
func (r RiskLevel) IsValid() bool {
	switch r {
	case RiskReadOnly, RiskWrite, RiskExecute, RiskDestructive:
		return true
	}
	return false
}

// Tool is the harness-side contract every executable tool implements.
// The four metadata methods are called once per registration to build
// the protocol.ToolDefinition catalog the harness ships in
// capabilities.register; Execute is called per tool.call event.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	OutputEnvelope() string
	RiskLevel() RiskLevel
	Execute(ctx context.Context, input json.RawMessage) (*ToolExecResult, error)
}

// ToolExecResult is the outcome of a tool execution. Status is one of
// "success", "rejected", "error". Reason is populated for "rejected";
// Error for "error". OutputType is "text"/"json"/"binary"/"image".
type ToolExecResult struct {
	Status     string
	Output     string
	OutputType string
	Error      string
	Reason     string
}

// Result statuses (kept as constants so callers don't sprinkle
// stringly-typed values across the codebase).
const (
	StatusSuccess  = "success"
	StatusRejected = "rejected"
	StatusError    = "error"
)

// ToProtocol converts the result into the wire shape the harness sends
// back to the runtime server as tool.result. The caller supplies the
// tool_call_id from the original tool.call event.
func (r *ToolExecResult) ToProtocol(toolCallID string) protocol.ToolResult {
	return protocol.ToolResult{
		ToolCallID: toolCallID,
		Status:     r.Status,
		Output:     r.Output,
		OutputType: r.OutputType,
		Error:      r.Error,
		Reason:     r.Reason,
	}
}

// SuccessResult is the common success constructor.
func SuccessResult(output, outputType string) *ToolExecResult {
	return &ToolExecResult{Status: StatusSuccess, Output: output, OutputType: outputType}
}

// ErrorResult builds an error result with the given message.
func ErrorResult(format string, args ...any) *ToolExecResult {
	return &ToolExecResult{Status: StatusError, Error: fmt.Sprintf(format, args...)}
}

// RejectedResult builds a permission-rejected result with the given reason.
func RejectedResult(reason string) *ToolExecResult {
	return &ToolExecResult{Status: StatusRejected, Reason: reason}
}

// Registry holds all available tools. Built-in tools register at
// startup; MCP-discovered tools (WU-081) register at runtime via the
// same Register call.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string // insertion order — used by All() for deterministic output
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. Panics on duplicate name — duplicate
// registration is always a programming error and silently overwriting
// would mask wire-visible behavior changes.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t == nil {
		panic("tools: Register received nil tool")
	}
	name := t.Name()
	if name == "" {
		panic("tools: Register called with empty tool name")
	}
	if !t.RiskLevel().IsValid() {
		panic(fmt.Sprintf("tools: Register %q: invalid risk level %q", name, t.RiskLevel()))
	}
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("tools: tool %q already registered", name))
	}
	r.tools[name] = t
	r.order = append(r.order, name)
}

// Get returns a tool by name, or nil when absent.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Deregister removes a tool by name. Returns true when a tool was
// actually removed, false when the name wasn't registered. Used by
// the MCP manager (WU-081) on reconnect so old tool entries don't
// linger in the registry.
func (r *Registry) Deregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[name]; !ok {
		return false
	}
	delete(r.tools, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return true
}

// Names returns registered tool names in insertion order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// All returns all registered tools as protocol.ToolDefinition slice
// (used by the App when assembling capabilities.register). Order
// matches insertion.
func (r *Registry) All() []protocol.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]protocol.ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		out = append(out, protocol.ToolDefinition{
			Name:           t.Name(),
			Description:    t.Description(),
			InputSchema:    t.InputSchema(),
			OutputEnvelope: t.OutputEnvelope(),
			RiskLevel:      string(t.RiskLevel()),
		})
	}
	return out
}

// ErrToolNotFound is returned by Executor.Execute when the named tool
// is not registered.
var ErrToolNotFound = errors.New("tool not found")

// PromptCallback is invoked when the executor needs user approval
// before running a tool. The callback should return true to allow,
// false to deny. Implementations typically dispatch a Bubbletea
// PermissionPromptMsg and block on a response. If nil, the executor
// treats Prompt as Deny.
type PromptCallback func(ctx context.Context, tool Tool, input json.RawMessage) (approved bool)

// Executor combines the registry, permission enforcer, and (for tests)
// a prompt callback into the single entry point handlers use to run a
// tool.call event.
type Executor struct {
	registry    *Registry
	permissions *PermissionEnforcer
	prompt      PromptCallback
	tracker     *FileTracker
}

// NewExecutor constructs an Executor.
func NewExecutor(registry *Registry, permissions *PermissionEnforcer) *Executor {
	return &Executor{
		registry:    registry,
		permissions: permissions,
		tracker:     NewFileTracker(),
	}
}

// SetPromptCallback installs the user-prompt callback. Production code
// hands in a closure that dispatches a PermissionPromptMsg through
// the Bubbletea program and waits on a response channel. Tests pass
// inline approve / deny lambdas.
func (e *Executor) SetPromptCallback(cb PromptCallback) { e.prompt = cb }

// Tracker returns the file-read tracker (used by Edit / Write to
// enforce "must Read before Edit" semantics).
func (e *Executor) Tracker() *FileTracker { return e.tracker }

// Registry returns the tool registry the executor dispatches through.
// Exposed for callers (e.g., the harness ToolDispatcher) that need
// to look up a tool's metadata before deciding how to route a call.
func (e *Executor) Registry() *Registry { return e.registry }

// Permissions returns the permission enforcer so callers can mutate
// approvals (Approve / ApproveDomain) after a successful prompt.
func (e *Executor) Permissions() *PermissionEnforcer { return e.permissions }

// Execute runs the named tool with the given input. Permission gating:
//   - PermAllow      → run immediately
//   - PermPrompt     → invoke the prompt callback; deny if nil/false
//   - PermDeny       → return RejectedResult without running
//
// The tool_call_id correlation is the caller's responsibility — pass
// the returned ToolExecResult to result.ToProtocol(toolCallID) when
// building the tool.result wire payload.
func (e *Executor) Execute(ctx context.Context, name string, input json.RawMessage) (*ToolExecResult, error) {
	tool := e.registry.Get(name)
	if tool == nil {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}

	if e.permissions != nil {
		switch e.permissions.Check(tool, input) {
		case PermDeny:
			return RejectedResult("denied by permission policy"), nil
		case PermPrompt:
			if e.prompt == nil {
				return RejectedResult("requires user prompt; no prompt handler"), nil
			}
			if !e.prompt(ctx, tool, input) {
				return RejectedResult("user denied"), nil
			}
			// Remember approval so subsequent invocations of the same
			// tool in the same session don't re-prompt (per design
			// D3.2 "first-use per tool per session"). Dangerous
			// commands re-evaluate via PermissionEnforcer.Check on
			// every call.
			e.permissions.Approve(tool.Name())
		case PermAllow:
			// proceed
		}
	}

	return tool.Execute(ctx, input)
}
