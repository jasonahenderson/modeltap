package harnessdemo

// Driver wraps a harnesshost.Adapter and orchestrates the fake stream
// emission needed to drive a FakeRuntime end-to-end. Demo CLI programs
// run Driver as their tea.Program model; tests use it as a fixture.

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
	"github.com/jasonahenderson/modeltap/internal/harnesshost"
)

// Driver is a tea.Model that owns a harnesshost.Adapter plus the
// stream orchestration loop for FakeRuntime.
type Driver struct {
	adapter           harnesshost.Adapter
	runtime           *FakeRuntime
	nextPermRequestID int
}

// NewDriver constructs a Driver wrapping the given shell model and
// FakeRuntime. The shell is wrapped in a harnesshost.Adapter so the
// Driver inherits the full action-consumer + projection + pause
// buffer pipeline.
func NewDriver(shell harnessshell.Model, runtime *FakeRuntime) Driver {
	return Driver{
		adapter: harnesshost.New(shell, runtime),
		runtime: runtime,
	}
}

// Init forwards to the inner adapter.
func (d Driver) Init() tea.Cmd {
	return d.adapter.Init()
}

// streamTickMsg drives the per-RunID stream loop.
type streamTickMsg struct {
	RunID string
}

// View forwards to the inner adapter.
func (d Driver) View() string {
	return d.adapter.View()
}

// Update routes Bubble Tea messages through the orchestration loop.
// streamTickMsg emits fake runtime tea.Msgs (StreamTokenMsg /
// StreamCompleteMsg / PermissionPromptMsg) which the inner adapter
// projects to HostEvents.
func (d Driver) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if tick, ok := msg.(streamTickMsg); ok {
		return d.handleStreamTick(tick)
	}

	inner, cmd := d.adapter.Update(msg)
	d.adapter = inner.(harnesshost.Adapter)

	startCmd := d.scheduleUnstartedRuns()
	return d, batchCmds(cmd, startCmd)
}

// handleStreamTick advances the stream for a single run.
func (d Driver) handleStreamTick(tick streamTickMsg) (tea.Model, tea.Cmd) {
	if d.runtime.IsPermissionDemoRun(tick.RunID) {
		d.nextPermRequestID++
		reqID := fmt.Sprintf("fake-perm-%d", d.nextPermRequestID)
		d.runtime.RegisterPermissionRequest(reqID, tick.RunID)
		prompt := harness.PermissionPromptMsg{
			ToolCallID:  reqID,
			ToolName:    "Read workspace/README.md",
			RiskLevel:   "low",
			Description: "Read a workspace file to summarize current state",
			Input:       []byte(`{"path":"workspace/README.md"}`),
		}
		return d, func() tea.Msg { return prompt }
	}

	chunk, paused, ok := d.runtime.PopStreamChunk(tick.RunID)
	if paused {
		return d, nil
	}
	if !ok {
		runID := tick.RunID
		return d, func() tea.Msg {
			return harness.StreamCompleteMsg{TurnID: runID}
		}
	}

	emitChunk := func() tea.Msg {
		return harness.StreamTokenMsg{TurnID: tick.RunID, Delta: chunk}
	}
	tickAgain := tea.Tick(d.runtime.StreamDelay(), func(time.Time) tea.Msg {
		return streamTickMsg{RunID: tick.RunID}
	})
	return d, tea.Batch(emitChunk, tickAgain)
}

// scheduleUnstartedRuns drains FakeRuntime.TakeUnstartedRuns and
// returns a tea.Cmd that schedules an immediate tick for each newly-
// started run.
func (d Driver) scheduleUnstartedRuns() tea.Cmd {
	runs := d.runtime.TakeUnstartedRuns()
	if len(runs) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, len(runs))
	for i, runID := range runs {
		rid := runID
		cmds[i] = tea.Tick(0, func(time.Time) tea.Msg {
			return streamTickMsg{RunID: rid}
		})
	}
	return tea.Batch(cmds...)
}

// batchCmds composes two tea.Cmds, dropping nils.
func batchCmds(a, b tea.Cmd) tea.Cmd {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return b
	case b == nil:
		return a
	}
	return tea.Batch(a, b)
}
