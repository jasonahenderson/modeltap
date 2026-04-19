package harness

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/harness/tools"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// fakeResultSender captures tool.result payloads the dispatcher
// posts so tests can assert on them without a real RPC client.
type fakeResultSender struct {
	mu      sync.Mutex
	results []*protocol.ToolResult
	err     error
}

func (f *fakeResultSender) SendToolResult(ctx context.Context, result *protocol.ToolResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = append(f.results, result)
	return f.err
}

func (f *fakeResultSender) last() *protocol.ToolResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.results) == 0 {
		return nil
	}
	return f.results[len(f.results)-1]
}

// fakeMode implements ModeReader for test scenarios that don't want
// to thread a full AppState in.
type fakeMode struct {
	m protocol.Mode
}

func (f fakeMode) CurrentMode() protocol.Mode { return f.m }

// fakeTool is a tools.Tool implementation whose RiskLevel and
// execution result are test-controlled.
type fakeTool struct {
	name   string
	risk   tools.RiskLevel
	output string
	status string
}

func (f *fakeTool) Name() string                 { return f.name }
func (f *fakeTool) Description() string          { return "fake " + f.name }
func (f *fakeTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (f *fakeTool) OutputEnvelope() string       { return "text" }
func (f *fakeTool) RiskLevel() tools.RiskLevel   { return f.risk }
func (f *fakeTool) Execute(ctx context.Context, input json.RawMessage) (*tools.ToolExecResult, error) {
	status := f.status
	if status == "" {
		status = tools.StatusSuccess
	}
	return &tools.ToolExecResult{Status: status, Output: f.output, OutputType: "text"}, nil
}

func newDispatcher(t *testing.T, mode protocol.Mode, fakes ...*fakeTool) (*ToolDispatcher, *fakeResultSender, *PlanAccumulator) {
	t.Helper()
	registry := tools.NewRegistry()
	for _, ft := range fakes {
		registry.Register(ft)
	}
	exec := tools.NewExecutor(registry, tools.NewPermissionEnforcer(tools.PermAutonomous))
	sender := &fakeResultSender{}
	plan := NewPlanAccumulator()
	d := NewToolDispatcher(exec, sender, plan, fakeMode{m: mode})
	return d, sender, plan
}

func TestToolDispatcher_ReadOnly_PassesThroughInBuildMode(t *testing.T) {
	d, sender, plan := newDispatcher(t, protocol.ModeBuild,
		&fakeTool{name: "Read", risk: tools.RiskReadOnly, output: "hello"},
	)

	err := d.HandleToolCall(protocol.ToolCall{
		TurnID: "t", ToolCallID: "tc-1", Tool: "Read",
		Input: json.RawMessage(`{"file_path":"x"}`),
	})
	if err != nil {
		t.Fatalf("HandleToolCall: %v", err)
	}
	if d.Executed() != 1 || d.Intercepted() != 0 {
		t.Errorf("counters: executed=%d intercepted=%d", d.Executed(), d.Intercepted())
	}
	if plan.Len() != 0 {
		t.Errorf("plan should be empty; got len=%d", plan.Len())
	}
	res := sender.last()
	if res == nil || res.ToolCallID != "tc-1" || res.Output != "hello" {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestToolDispatcher_ReadOnly_PassesThroughInPlanMode(t *testing.T) {
	d, _, plan := newDispatcher(t, protocol.ModePlan,
		&fakeTool{name: "Read", risk: tools.RiskReadOnly, output: "hello"},
	)
	if err := d.HandleToolCall(protocol.ToolCall{
		TurnID: "t", ToolCallID: "tc", Tool: "Read",
		Input: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("HandleToolCall: %v", err)
	}
	if d.Executed() != 1 {
		t.Errorf("read-only should execute even in plan mode; executed=%d", d.Executed())
	}
	if plan.Len() != 0 {
		t.Errorf("read-only should not land in plan; len=%d", plan.Len())
	}
}

func TestToolDispatcher_PlanMode_InterceptsWrite(t *testing.T) {
	d, sender, plan := newDispatcher(t, protocol.ModePlan,
		&fakeTool{name: "Write", risk: tools.RiskWrite},
	)
	err := d.HandleToolCall(protocol.ToolCall{
		ToolCallID: "tc-write", Tool: "Write",
		Input: json.RawMessage(`{"file_path":"/tmp/o.txt","content":"x"}`),
	})
	if err != nil {
		t.Fatalf("HandleToolCall: %v", err)
	}
	if d.Intercepted() != 1 || d.Executed() != 0 {
		t.Errorf("counters: intercepted=%d executed=%d", d.Intercepted(), d.Executed())
	}
	if plan.Len() != 1 {
		t.Fatalf("plan should have 1 step; got %d", plan.Len())
	}
	step := plan.Steps()[0]
	if step.ToolName != "Write" {
		t.Errorf("step ToolName = %q", step.ToolName)
	}
	// Server must receive a synthetic result so it doesn't hang.
	res := sender.last()
	if res == nil || res.ToolCallID != "tc-write" {
		t.Fatalf("expected synthetic result; got %+v", res)
	}
	if !strings.Contains(strings.ToLower(res.Output), "plan mode") {
		t.Errorf("synthetic result should mention plan mode; got %q", res.Output)
	}
	if res.Status != tools.StatusSuccess {
		t.Errorf("synthetic result should be success; got %q", res.Status)
	}
}

func TestToolDispatcher_PlanMode_InterceptsExecute(t *testing.T) {
	d, _, plan := newDispatcher(t, protocol.ModePlan,
		&fakeTool{name: "Bash", risk: tools.RiskExecute},
	)
	_ = d.HandleToolCall(protocol.ToolCall{
		ToolCallID: "tc-bash", Tool: "Bash",
		Input: json.RawMessage(`{"command":"make"}`),
	})
	if plan.Len() != 1 {
		t.Errorf("execute-risk tool should be intercepted; plan len=%d", plan.Len())
	}
}

func TestToolDispatcher_PlanMode_InterceptsDestructive(t *testing.T) {
	d, _, plan := newDispatcher(t, protocol.ModePlan,
		&fakeTool{name: "Destructive", risk: tools.RiskDestructive},
	)
	_ = d.HandleToolCall(protocol.ToolCall{
		ToolCallID: "tc-d", Tool: "Destructive",
		Input: json.RawMessage(`{}`),
	})
	if plan.Len() != 1 {
		t.Errorf("destructive tool should be intercepted; plan len=%d", plan.Len())
	}
}

func TestToolDispatcher_UnknownTool_ErrorResult(t *testing.T) {
	d, sender, _ := newDispatcher(t, protocol.ModeBuild)
	err := d.HandleToolCall(protocol.ToolCall{
		ToolCallID: "tc-missing", Tool: "Nonexistent",
	})
	if err != nil {
		t.Fatalf("HandleToolCall should not surface error for unknown tool; got %v", err)
	}
	res := sender.last()
	if res == nil || res.Status != tools.StatusError {
		t.Fatalf("expected error result; got %+v", res)
	}
	if !strings.Contains(res.Error, "Nonexistent") {
		t.Errorf("error should name the missing tool; got %q", res.Error)
	}
}

func TestToolDispatcher_NilSender_Errors(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&fakeTool{name: "X", risk: tools.RiskReadOnly})
	exec := tools.NewExecutor(registry, tools.NewPermissionEnforcer(tools.PermAutonomous))
	d := NewToolDispatcher(exec, nil, nil, nil)

	err := d.HandleToolCall(protocol.ToolCall{Tool: "X"})
	if err == nil {
		t.Error("expected error when sender is nil")
	}
}

func TestToolDispatcher_NilMode_NoIntercept(t *testing.T) {
	// Plan mode interception needs a ModeReader. When mode is nil,
	// every tool executes (no plan mode exists).
	registry := tools.NewRegistry()
	registry.Register(&fakeTool{name: "Write", risk: tools.RiskWrite, output: "wrote"})
	exec := tools.NewExecutor(registry, tools.NewPermissionEnforcer(tools.PermAutonomous))
	sender := &fakeResultSender{}
	d := NewToolDispatcher(exec, sender, NewPlanAccumulator(), nil)

	if err := d.HandleToolCall(protocol.ToolCall{
		ToolCallID: "tc", Tool: "Write",
		Input: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("HandleToolCall: %v", err)
	}
	if d.Executed() != 1 {
		t.Errorf("nil mode should execute directly; executed=%d", d.Executed())
	}
}

func TestToolDispatcher_AppStateAsModeReader(t *testing.T) {
	// AppState itself implements ModeReader — verify that path works
	// with a real state pointer and reflects mode changes.
	state := NewAppState()
	state.Mode = protocol.ModePlan

	registry := tools.NewRegistry()
	registry.Register(&fakeTool{name: "Write", risk: tools.RiskWrite})
	exec := tools.NewExecutor(registry, tools.NewPermissionEnforcer(tools.PermAutonomous))
	sender := &fakeResultSender{}
	plan := NewPlanAccumulator()
	d := NewToolDispatcher(exec, sender, plan, state)

	_ = d.HandleToolCall(protocol.ToolCall{
		ToolCallID: "tc", Tool: "Write",
		Input: json.RawMessage(`{}`),
	})
	if d.Intercepted() != 1 {
		t.Errorf("plan mode via AppState should intercept; intercepted=%d", d.Intercepted())
	}

	// Flip to build and a second call should execute.
	state.Mode = protocol.ModeBuild
	_ = d.HandleToolCall(protocol.ToolCall{
		ToolCallID: "tc2", Tool: "Write",
		Input: json.RawMessage(`{}`),
	})
	if d.Executed() != 1 {
		t.Errorf("build mode should execute; executed=%d", d.Executed())
	}
}
