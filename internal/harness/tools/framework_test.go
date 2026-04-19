package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// stubTool is a minimal Tool implementation used by framework tests.
type stubTool struct {
	name   string
	risk   RiskLevel
	output string
	err    error
}

func (s *stubTool) Name() string                  { return s.name }
func (s *stubTool) Description() string           { return s.name + " stub" }
func (s *stubTool) InputSchema() json.RawMessage  { return json.RawMessage(`{"type":"object"}`) }
func (s *stubTool) OutputEnvelope() string        { return "text" }
func (s *stubTool) RiskLevel() RiskLevel          { return s.risk }
func (s *stubTool) Execute(_ context.Context, _ json.RawMessage) (*ToolExecResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return SuccessResult(s.output, "text"), nil
}

func TestRiskLevel_IsValid(t *testing.T) {
	for _, r := range []RiskLevel{RiskReadOnly, RiskWrite, RiskExecute, RiskDestructive} {
		if !r.IsValid() {
			t.Errorf("%q should be valid", r)
		}
	}
	if RiskLevel("WHATEVER").IsValid() {
		t.Errorf("garbage risk should not be valid")
	}
}

func TestRegistry_Register_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "x", risk: RiskReadOnly})
	defer func() {
		if recover() == nil {
			t.Errorf("duplicate Register should panic")
		}
	}()
	r.Register(&stubTool{name: "x", risk: RiskReadOnly})
}

func TestRegistry_Register_InvalidRiskPanics(t *testing.T) {
	r := NewRegistry()
	defer func() {
		if recover() == nil {
			t.Errorf("invalid risk Register should panic")
		}
	}()
	r.Register(&stubTool{name: "x", risk: RiskLevel("garbage")})
}

func TestRegistry_All_PreservesInsertionOrder(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "a", risk: RiskReadOnly})
	r.Register(&stubTool{name: "c", risk: RiskWrite})
	r.Register(&stubTool{name: "b", risk: RiskExecute})

	got := r.Names()
	want := []string{"a", "c", "b"}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("Names()[%d] = %q, want %q", i, got[i], n)
		}
	}
	all := r.All()
	if len(all) != 3 || all[0].Name != "a" || all[2].Name != "b" {
		t.Errorf("All() order wrong: %+v", all)
	}
	for _, td := range all {
		if td.OutputEnvelope != "text" {
			t.Errorf("output envelope = %q", td.OutputEnvelope)
		}
	}
}

func TestExecutor_NotFound(t *testing.T) {
	r := NewRegistry()
	exec := NewExecutor(r, NewPermissionEnforcer(PermAutonomous))
	_, err := exec.Execute(context.Background(), "missing", json.RawMessage(`{}`))
	if !errors.Is(err, ErrToolNotFound) {
		t.Errorf("expected ErrToolNotFound, got %v", err)
	}
}

func TestExecutor_AllowReadOnly(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "ro", risk: RiskReadOnly, output: "ok"})
	exec := NewExecutor(r, NewPermissionEnforcer(PermDefault))

	res, err := exec.Execute(context.Background(), "ro", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess || res.Output != "ok" {
		t.Errorf("result = %+v", res)
	}
}

func TestExecutor_PromptDenied_NoCallback(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "wr", risk: RiskWrite})
	exec := NewExecutor(r, NewPermissionEnforcer(PermDefault))

	res, _ := exec.Execute(context.Background(), "wr", json.RawMessage(`{}`))
	if res.Status != StatusRejected {
		t.Errorf("expected rejected when no callback, got %+v", res)
	}
}

func TestExecutor_PromptApproved_RemembersTool(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "wr", risk: RiskWrite, output: "wrote"})
	exec := NewExecutor(r, NewPermissionEnforcer(PermDefault))

	prompts := 0
	exec.SetPromptCallback(func(ctx context.Context, t Tool, input json.RawMessage) bool {
		prompts++
		return true
	})

	for i := 0; i < 3; i++ {
		res, err := exec.Execute(context.Background(), "wr", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Execute %d: %v", i, err)
		}
		if res.Status != StatusSuccess {
			t.Fatalf("Execute %d status = %q", i, res.Status)
		}
	}
	if prompts != 1 {
		t.Errorf("expected 1 prompt across 3 invocations, got %d", prompts)
	}
}

func TestExecutor_PromptDenied_RejectsResult(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "wr", risk: RiskWrite})
	exec := NewExecutor(r, NewPermissionEnforcer(PermDefault))

	exec.SetPromptCallback(func(ctx context.Context, t Tool, input json.RawMessage) bool {
		return false
	})

	res, _ := exec.Execute(context.Background(), "wr", json.RawMessage(`{}`))
	if res.Status != StatusRejected || res.Reason != "user denied" {
		t.Errorf("result = %+v", res)
	}
}

func TestExecutor_AutonomousLevel_AllowsExecuteRisk(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "exec", risk: RiskExecute, output: "did the thing"})
	exec := NewExecutor(r, NewPermissionEnforcer(PermAutonomous))

	res, err := exec.Execute(context.Background(), "exec", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Errorf("autonomous should allow execute risk, got %+v", res)
	}
}

func TestExecutor_DestructiveAlwaysPrompts(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubTool{name: "destroy", risk: RiskDestructive})
	exec := NewExecutor(r, NewPermissionEnforcer(PermAutonomous))

	prompted := false
	exec.SetPromptCallback(func(_ context.Context, _ Tool, _ json.RawMessage) bool {
		prompted = true
		return false
	})
	res, _ := exec.Execute(context.Background(), "destroy", json.RawMessage(`{}`))
	if !prompted {
		t.Errorf("destructive should prompt even in autonomous mode")
	}
	if res.Status != StatusRejected {
		t.Errorf("result = %+v", res)
	}
}

func TestToolExecResult_ToProtocol(t *testing.T) {
	r := SuccessResult("hello", "text")
	wire := r.ToProtocol("call-1")
	if wire.ToolCallID != "call-1" || wire.Status != StatusSuccess || wire.Output != "hello" {
		t.Errorf("wire = %+v", wire)
	}
}
