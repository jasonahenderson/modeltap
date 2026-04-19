package harness

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jasonahenderson/modeltap/internal/harness/tools"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// ToolResultSender is the narrow slice of ConnProtocolClient the
// dispatcher needs — it only needs to post tool results back to the
// server. Extracting it keeps the dispatcher testable with a fake
// client without pulling every RPC method into the fake.
type ToolResultSender interface {
	SendToolResult(ctx context.Context, result *protocol.ToolResult) error
}

// ModeReader reports the current execution mode. Implemented by
// *AppState (plus anything else that exposes a Mode). Kept narrow so
// the dispatcher doesn't import the full state struct.
type ModeReader interface {
	CurrentMode() protocol.Mode
}

// ToolDispatcher is the harness-side layer that sits between server
// tool.call notifications and the local tool registry. It mirrors the
// Claude-Code / OpenCode pattern: plan mode is a policy decorator on
// the dispatcher rather than a branch in the UI event loop.
//
//	plan mode + risk > read_only  → PlanAccumulator.Append +
//	                                synthetic "queued" tool.result
//	otherwise                     → Executor.Execute + real
//	                                tool.result
//
// The read-only tools (Read/Glob/Grep/Git-read) always run — plan
// mode should still let the model reason about the code, just not
// mutate it.
type ToolDispatcher struct {
	executor *tools.Executor
	sender   ToolResultSender
	plan     *PlanAccumulator
	mode     ModeReader

	timeout time.Duration

	// interceptedCount / executedCount are cheap counters tests use
	// to assert the dispatcher actually routed a call somewhere.
	mu               sync.Mutex
	interceptedCount int
	executedCount    int
}

// NewToolDispatcher constructs a dispatcher. executor and sender are
// required; plan and mode may be nil when plan-mode interception is
// not desired (e.g. in contexts without a PlanAccumulator wired).
func NewToolDispatcher(executor *tools.Executor, sender ToolResultSender, plan *PlanAccumulator, mode ModeReader) *ToolDispatcher {
	return &ToolDispatcher{
		executor: executor,
		sender:   sender,
		plan:     plan,
		mode:     mode,
		timeout:  5 * time.Minute,
	}
}

// SetTimeout overrides the per-tool execution timeout. Tests use this
// to drive the cancellation path without sleeping for minutes.
func (d *ToolDispatcher) SetTimeout(t time.Duration) { d.timeout = t }

// Intercepted returns the count of plan-mode interceptions. Tests
// only.
func (d *ToolDispatcher) Intercepted() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.interceptedCount
}

// Executed returns the count of real executions the dispatcher drove.
// Tests only.
func (d *ToolDispatcher) Executed() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.executedCount
}

// HandleToolCall is the entry point the ConnectionManager calls when
// a tool.call notification arrives. Returns an error only for truly
// fatal problems (nil executor, nil sender); tool-execution errors
// are reported back to the server via tool.result with status=error
// and are not propagated up.
func (d *ToolDispatcher) HandleToolCall(call protocol.ToolCall) error {
	if d == nil || d.executor == nil || d.sender == nil {
		return fmt.Errorf("tool dispatcher not fully wired")
	}

	tool := d.executor.Registry().Get(call.Tool)
	if tool == nil {
		return d.sendError(call, fmt.Sprintf("unknown tool: %s", call.Tool))
	}

	// Plan-mode policy decorator. Read-only tools bypass because the
	// agent still needs to inspect the repo; anything higher-risk is
	// queued for the accumulator.
	if d.planShouldIntercept(tool) {
		d.mu.Lock()
		d.interceptedCount++
		d.mu.Unlock()
		if d.plan != nil {
			d.plan.Append(call.Tool, call.Input, "")
		}
		return d.sendQueued(call)
	}

	d.mu.Lock()
	d.executedCount++
	d.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	res, err := d.executor.Execute(ctx, call.Tool, call.Input)
	if err != nil {
		return d.sendError(call, err.Error())
	}
	return d.sendResult(ctx, call, res)
}

// planShouldIntercept reports whether the tool call should be
// intercepted by plan mode rather than executed. Criteria: mode
// is Plan AND the tool's risk classification is anything but
// read-only.
func (d *ToolDispatcher) planShouldIntercept(tool tools.Tool) bool {
	if d.mode == nil {
		return false
	}
	if d.mode.CurrentMode() != protocol.ModePlan {
		return false
	}
	return tool.RiskLevel() != tools.RiskReadOnly
}

// sendQueued posts a synthetic tool.result announcing that the call
// was captured in the plan. Status stays "success" so the server
// doesn't flag the turn as errored, but Output clearly marks the
// call as deferred for user review.
func (d *ToolDispatcher) sendQueued(call protocol.ToolCall) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := &protocol.ToolResult{
		ToolCallID: call.ToolCallID,
		Status:     tools.StatusSuccess,
		Output:     "[plan mode — step queued, awaiting /approve-plan]",
		OutputType: "text",
	}
	return d.sender.SendToolResult(ctx, result)
}

// sendResult posts a real execution result. Maps the ToolExecResult
// status fields verbatim onto the wire protocol.ToolResult.
func (d *ToolDispatcher) sendResult(ctx context.Context, call protocol.ToolCall, res *tools.ToolExecResult) error {
	result := &protocol.ToolResult{
		ToolCallID: call.ToolCallID,
		Status:     res.Status,
		Output:     res.Output,
		OutputType: res.OutputType,
		Error:      res.Error,
		Reason:     res.Reason,
	}
	return d.sender.SendToolResult(ctx, result)
}

// sendError posts a terminal failure result for a tool.call that
// couldn't even start (unknown tool, missing dispatcher). Uses a
// fresh context with a short timeout since we're already in an
// error path.
func (d *ToolDispatcher) sendError(call protocol.ToolCall, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := &protocol.ToolResult{
		ToolCallID: call.ToolCallID,
		Status:     tools.StatusError,
		Error:      reason,
	}
	return d.sender.SendToolResult(ctx, result)
}

// AppState CurrentMode satisfies ModeReader. Declared here (not
// model.go) so the accessor lives next to the only consumer.
func (s *AppState) CurrentMode() protocol.Mode {
	if s == nil {
		return ""
	}
	return s.Mode
}
