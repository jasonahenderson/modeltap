package harness

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// PlanStep records one tool call that would have run if the harness
// were in Build / Auto mode. Plan mode intercepts write / execute /
// destructive tools and accumulates them here instead of executing
// them, letting the model lay out its intentions before the user
// approves, edits, or cancels.
type PlanStep struct {
	ToolName string
	Input    json.RawMessage
	Summary  string    // human-readable one-liner
	At       time.Time // when the step was intercepted
}

// PlanAccumulator is the data structure Bundle 13 D1 calls for. It
// collects intercepted tool calls during plan mode and hands them to
// a UI (or future approve flow) once the user decides to act. Safe
// for concurrent append / read.
type PlanAccumulator struct {
	mu    sync.Mutex
	steps []PlanStep
}

// NewPlanAccumulator returns an empty accumulator.
func NewPlanAccumulator() *PlanAccumulator {
	return &PlanAccumulator{}
}

// Append records an intercepted tool call. Summary is a short human
// description — typically the tool name plus the most salient input
// field (file_path, command, url). Callers that don't have a
// summary on hand should pass "".
func (p *PlanAccumulator) Append(toolName string, input json.RawMessage, summary string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if summary == "" {
		summary = defaultPlanSummary(toolName, input)
	}
	p.steps = append(p.steps, PlanStep{
		ToolName: toolName,
		Input:    input,
		Summary:  summary,
		At:       time.Now(),
	})
}

// Steps returns a snapshot of the accumulated steps.
func (p *PlanAccumulator) Steps() []PlanStep {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]PlanStep, len(p.steps))
	copy(out, p.steps)
	return out
}

// Len reports the number of accumulated steps.
func (p *PlanAccumulator) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.steps)
}

// Clear empties the accumulator. Called after the user approves,
// cancels, or switches out of plan mode.
func (p *PlanAccumulator) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = nil
}

// FormatSteps renders the accumulated steps as a compact numbered
// list for banner display. Empty when no steps exist.
func (p *PlanAccumulator) FormatSteps() string {
	steps := p.Steps()
	if len(steps) == 0 {
		return "(no plan steps yet)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Plan (%d step(s)):", len(steps))
	for i, s := range steps {
		fmt.Fprintf(&b, "\n  %d. %s", i+1, s.Summary)
	}
	return b.String()
}

// defaultPlanSummary builds a best-effort one-liner from tool name +
// input. Pulls common fields (file_path, command, url, pattern) when
// present so the banner reads naturally without every caller having
// to supply a summary.
func defaultPlanSummary(toolName string, input json.RawMessage) string {
	var generic map[string]any
	if err := json.Unmarshal(input, &generic); err != nil || generic == nil {
		return toolName
	}
	for _, key := range []string{"file_path", "command", "url", "pattern", "path"} {
		if v, ok := generic[key]; ok {
			return fmt.Sprintf("%s %v", toolName, v)
		}
	}
	return toolName
}

// handleModeCommand handles /plan, /build, /auto. Each sets the mode
// directly (no conn round-trip) and emits ModeChangeMsg which the
// App Update handler already applies to AppState. Plan entry is
// announced via a transient banner so the user knows the next tool
// call won't execute.
func (a *App) handleModeCommand(msg SubmitMsg) tea.Cmd {
	var target protocol.Mode
	switch strings.ToLower(msg.Command) {
	case "plan":
		target = protocol.ModePlan
	case "build":
		target = protocol.ModeBuild
	case "auto":
		target = protocol.ModeAuto
	default:
		return nil
	}
	return a.setMode(target)
}

// setMode is the shared entrypoint for both slash commands and
// Ctrl+P. Idempotent — switching to the current mode is a no-op.
func (a *App) setMode(m protocol.Mode) tea.Cmd {
	if a.state.Mode == m {
		return nil
	}
	banner := BannerMsg{
		Text:     fmt.Sprintf("Mode: %s", m),
		Duration: 3 * time.Second,
	}
	announce := func() tea.Msg { return banner }
	change := func() tea.Msg { return ModeChangeMsg{Mode: m} }
	return tea.Batch(change, announce)
}
