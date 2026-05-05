// Package harnessdemo provides a fake/demo Runtime implementation for
// internal/harnesshost. It is the post-extraction home for the demo
// behaviors that previously lived in internal/harnessspike: fake reply
// generation, fake stream timing, /perm-style permission demos, and
// the demo CLI entrypoint per WU-099 §"Where Demo Behavior Lives" and
// WU-100 Stage E.
//
// The package has no production runtime dependencies. A demo CLI
// program (or a test fixture) constructs a FakeRuntime, wraps it in
// harnesshost.Adapter, and runs the resulting tea.Model. The Driver
// tea.Model wrapper in driver.go orchestrates fake stream emission so
// the shell sees realistic stream/complete lifecycle events without
// needing a real BFF.
package harnessdemo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jasonahenderson/modeltap/internal/harnesshost"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// FakeRuntime implements harnesshost.Runtime with fake reply
// generation, fake stream timing, and a permission demo. It is safe
// for concurrent use: SubmitTurn and the related lifecycle hooks are
// called from tea.Cmd goroutines, while the Driver's Update method
// drains stream chunks from the main goroutine.
type FakeRuntime struct {
	mu sync.Mutex

	// activeStreams stores the per-RunID chunk sequence that
	// PopStreamChunk drains.
	activeStreams map[string]*fakeStream

	// unstartedRuns is the list of RunIDs SubmitTurn has accepted but
	// the Driver has not yet started ticking. The Driver drains it
	// on every Update tick to schedule the first stream tick.
	unstartedRuns []string

	// pendingPermissions tracks runs waiting on a permission demo
	// resolution. The Driver pauses streaming for these runs until
	// ResolvePermission lifts the gate.
	pendingPermissions map[string]string // requestID → runID

	nextRunID   int
	streamDelay time.Duration
}

type fakeStream struct {
	chunks []string
	cursor int
	paused bool
}

// New constructs a FakeRuntime with default streaming cadence
// (35ms/chunk, matching the spike's default).
func New() *FakeRuntime {
	return &FakeRuntime{
		activeStreams:      map[string]*fakeStream{},
		pendingPermissions: map[string]string{},
		streamDelay:        35 * time.Millisecond,
	}
}

// WithStreamDelay overrides the default per-chunk stream delay.
func (f *FakeRuntime) WithStreamDelay(d time.Duration) *FakeRuntime {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.streamDelay = d
	return f
}

// SubmitTurn accepts a turn, generates a fake reply, and schedules the
// stream to be ticked out by the Driver. The /perm command short-
// circuits into a permission demo path that the Driver picks up via
// the pending-permission set.
func (f *FakeRuntime) SubmitTurn(ctx context.Context, req harnesshost.SubmitRequest) (harnesshost.SubmitAccepted, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextRunID++
	runID := fmt.Sprintf("fake-run-%d", f.nextRunID)

	if isPermissionDemoTurn(req.Text) {
		// The Driver's tick will inspect this run, see no chunks,
		// detect the permission demo flag, and emit a permission
		// request instead of a stream.
		f.activeStreams[runID] = &fakeStream{paused: true}
		f.unstartedRuns = append(f.unstartedRuns, runID)
		return harnesshost.SubmitAccepted{RunID: runID, Label: "fake-kimi-demo"}, nil
	}

	f.activeStreams[runID] = &fakeStream{
		chunks: splitForStreaming(fakeReply(req.Text)),
	}
	f.unstartedRuns = append(f.unstartedRuns, runID)
	return harnesshost.SubmitAccepted{RunID: runID, Label: "fake-kimi-demo"}, nil
}

// InterruptRun marks an active run as exhausted; the Driver's next
// tick will see no remaining chunks and emit StreamComplete.
func (f *FakeRuntime) InterruptRun(ctx context.Context, runID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.activeStreams, runID)
	return nil
}

// DispatchCommand is a no-op for the fake runtime — host-native
// command handling is not part of the demo surface.
func (f *FakeRuntime) DispatchCommand(ctx context.Context, cmd harnesshost.HostCommand) error {
	return nil
}

// ResolvePermission applies the user's decision. The Driver picks up
// the resolution on its next tick and resumes the paused stream
// (approve) or marks it complete (deny).
func (f *FakeRuntime) ResolvePermission(ctx context.Context, requestID string, decision harnessshell.PermissionDecision) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	runID, ok := f.pendingPermissions[requestID]
	if !ok {
		return nil
	}
	delete(f.pendingPermissions, requestID)
	stream, exists := f.activeStreams[runID]
	if !exists {
		return nil
	}
	switch decision {
	case harnessshell.DecisionApproveOnce, harnessshell.DecisionApproveSession:
		stream.paused = false
		stream.chunks = splitForStreaming(permissionGrantedReply())
		stream.cursor = 0
	case harnessshell.DecisionDeny:
		// Empty chunks → next tick emits StreamComplete.
		stream.paused = false
		stream.chunks = nil
		stream.cursor = 0
	}
	f.unstartedRuns = append(f.unstartedRuns, runID)
	return nil
}

// LoadPreview returns a synthetic preview payload for any token.
func (f *FakeRuntime) LoadPreview(ctx context.Context, req harnesshost.PreviewRequest) (harnessshell.PreviewPayload, error) {
	content := fmt.Sprintf("Fake preview for %s\n\nSource: %s\nPath: %s",
		req.TokenID, req.Source, req.Path)
	return harnessshell.PreviewPayload{
		Title:   "preview: " + req.TokenID,
		Content: content,
	}, nil
}

// SummarizePaste is a passthrough — the shell's local summarizer is
// already adequate for the demo surface.
func (f *FakeRuntime) SummarizePaste(ctx context.Context, raw string) (string, error) {
	return raw, nil
}

// PopStreamChunk pops the next chunk for a run. Returns ("", false,
// false) when the stream has no more chunks (i.e. is complete or the
// run is missing). The middle bool indicates whether the run is
// currently paused waiting for a permission resolution; callers use
// it to suppress further ticks until ResolvePermission fires.
func (f *FakeRuntime) PopStreamChunk(runID string) (chunk string, paused bool, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, exists := f.activeStreams[runID]
	if !exists {
		return "", false, false
	}
	if s.paused {
		return "", true, false
	}
	if s.cursor >= len(s.chunks) {
		delete(f.activeStreams, runID)
		return "", false, false
	}
	c := s.chunks[s.cursor]
	s.cursor++
	return c, false, true
}

// IsPermissionDemoRun reports whether the named run is a permission-
// demo run that has not yet been gated. The Driver checks this on the
// first tick to emit the permission request before streaming begins.
func (f *FakeRuntime) IsPermissionDemoRun(runID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.activeStreams[runID]
	return ok && s.paused && len(s.chunks) == 0
}

// RegisterPermissionRequest records the permission request ID that
// gates the named run. The Driver calls this immediately after
// emitting the PermissionPromptMsg so ResolvePermission can correlate.
func (f *FakeRuntime) RegisterPermissionRequest(requestID, runID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pendingPermissions[requestID] = runID
}

// TakeUnstartedRuns returns the RunIDs that have been accepted by
// SubmitTurn (or have just had their permission resolved) but have
// not yet been ticked by the Driver. The list is cleared in the
// process; callers should schedule one tick per returned ID.
func (f *FakeRuntime) TakeUnstartedRuns() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	runs := f.unstartedRuns
	f.unstartedRuns = nil
	return runs
}

// StreamDelay returns the per-chunk delay the Driver uses for
// scheduling stream ticks.
func (f *FakeRuntime) StreamDelay() time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.streamDelay
}

// fakeReply chooses a fake response for the given prompt, mirroring
// the spike's behavior so demo programs feel familiar.
func fakeReply(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lower, "/demo"):
		return demoReply()
	case strings.Contains(lower, "why"):
		return "This demo exists to prove the shell before wiring the backend. If the shell feels wrong with fake data, real integrations will only make it worse."
	default:
		return "Fake response for: " + trimmed + "\n\nThis is intentionally dumb, but it should feel responsive:\n- immediate echo\n- progressive stream\n- stable transcript\n- predictable focus"
	}
}

// permissionGrantedReply returns the assistant text the demo emits
// after a permission demo is approved.
func permissionGrantedReply() string {
	return "Read the README. The demo is iterating on a replacement harness shell with inline tool events, transcript scroll stability, and queued follow-up messages."
}

// demoReply produces a long-running stream useful for testing the
// queue, the working indicator, and interrupt handling.
func demoReply() string {
	segments := []string{
		"Demo stream mode engaged for long-run testing.",
		"Watch the working indicator pulse while tokens continue to arrive.",
		"Use this mode to test queue visibility, interrupt handling, and transcript stability.",
		"Each numbered step is intentionally verbose so the stream lasts long enough to judge behavior clearly.",
	}
	var parts []string
	for i := 1; i <= 12; i++ {
		parts = append(parts, fmt.Sprintf("Step %02d.", i))
		parts = append(parts, segments...)
	}
	parts = append(parts, "Demo stream complete. The queue should now release the next waiting message.")
	return strings.Join(parts, " ")
}

// splitForStreaming chunks a reply into word-level pieces with a
// trailing space on every chunk except the last, so the streaming
// output reads naturally as it accumulates.
func splitForStreaming(s string) []string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return []string{""}
	}
	out := make([]string, 0, len(parts))
	for i, part := range parts {
		if i == len(parts)-1 {
			out = append(out, part)
		} else {
			out = append(out, part+" ")
		}
	}
	return out
}

// isPermissionDemoTurn reports whether a turn text triggers the
// permission demo (the spike's /perm command behavior).
func isPermissionDemoTurn(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(text)), "/perm")
}
