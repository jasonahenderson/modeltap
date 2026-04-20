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

// ToolActivityObserver receives start/end notifications for every
// tool.call the dispatcher handles. The CLI wires an observer that
// translates these into ToolActivityMsg on the tea.Program so the
// viewport can render tool-call activity inline. Optional; nil
// observer is a no-op.
type ToolActivityObserver interface {
	OnToolStart(call protocol.ToolCall, summary string)
	OnToolEnd(call protocol.ToolCall, status, output string, elapsed time.Duration)
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
	observer ToolActivityObserver

	timeout time.Duration

	// interceptedCount / executedCount are cheap counters tests use
	// to assert the dispatcher actually routed a call somewhere.
	mu               sync.Mutex
	interceptedCount int
	executedCount    int

	// seen tracks tool_call_ids that have been dispatched — WU-094
	// H-3 idempotency. A misbehaving or malicious BFF that re-emits
	// the same tool.call would otherwise drive the tool N times.
	// Capped to avoid unbounded growth across long sessions; the
	// cap evicts LRU via a ring of timestamped entries.
	seen     map[string]struct{}
	seenRing []string
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
		seen:     make(map[string]struct{}),
	}
}

// seenCapacity bounds the idempotency set so a long-running session
// doesn't grow the map forever. Old entries evict LRU-ish via the
// ring buffer.
const seenCapacity = 1024

// SetTimeout overrides the per-tool execution timeout. Tests use this
// to drive the cancellation path without sleeping for minutes.
func (d *ToolDispatcher) SetTimeout(t time.Duration) { d.timeout = t }

// SetObserver installs a ToolActivityObserver. Pass nil to disable.
func (d *ToolDispatcher) SetObserver(o ToolActivityObserver) { d.observer = o }

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

	// Idempotency — reject duplicate tool_call_ids (WU-094 H-3). A
	// malicious or buggy BFF that re-emits the same call would
	// otherwise drive the tool N times.
	if call.ToolCallID != "" {
		d.mu.Lock()
		if _, ok := d.seen[call.ToolCallID]; ok {
			d.mu.Unlock()
			return d.sendError(call, "duplicate tool_call_id")
		}
		d.seen[call.ToolCallID] = struct{}{}
		d.seenRing = append(d.seenRing, call.ToolCallID)
		if len(d.seenRing) > seenCapacity {
			evict := d.seenRing[0]
			d.seenRing = d.seenRing[1:]
			delete(d.seen, evict)
		}
		d.mu.Unlock()
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

	summary := defaultPlanSummary(call.Tool, call.Input)
	if d.observer != nil {
		d.observer.OnToolStart(call, summary)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	res, err := d.executor.Execute(ctx, call.Tool, call.Input)
	elapsed := time.Since(start)
	if err != nil {
		if d.observer != nil {
			d.observer.OnToolEnd(call, tools.StatusError, err.Error(), elapsed)
		}
		return d.sendError(call, err.Error())
	}
	if d.observer != nil {
		d.observer.OnToolEnd(call, res.Status, toolEndSummary(res), elapsed)
	}
	return d.sendResult(ctx, call, res)
}

// toolEndSummary produces the one-line outcome rendered in the
// viewport when a tool finishes. For success: output (truncated); for
// error/rejected: the Error or Reason string. Long outputs are
// truncated at 200 chars so the viewport stays scannable.
func toolEndSummary(res *tools.ToolExecResult) string {
	switch res.Status {
	case tools.StatusError:
		return res.Error
	case tools.StatusRejected:
		return res.Reason
	}
	out := res.Output
	if len(out) > 200 {
		out = out[:200] + "…"
	}
	return out
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

// SendErrorResult is the public error-reporting entrypoint so the
// ConnectionManager's panic recovery can surface a tool.result on
// our behalf when the dispatch goroutine panics (WU-094 H-2).
func (d *ToolDispatcher) SendErrorResult(call protocol.ToolCall, reason string) error {
	if d == nil || d.sender == nil {
		return fmt.Errorf("tool dispatcher not wired")
	}
	return d.sendError(call, reason)
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
