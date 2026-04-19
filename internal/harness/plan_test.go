package harness

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestPlanAccumulator_AppendAndSteps(t *testing.T) {
	acc := NewPlanAccumulator()
	if acc.Len() != 0 {
		t.Fatal("fresh accumulator should be empty")
	}
	acc.Append("Write", json.RawMessage(`{"file_path":"/tmp/out.txt"}`), "")
	acc.Append("Bash", json.RawMessage(`{"command":"ls -la"}`), "list dir")

	if acc.Len() != 2 {
		t.Fatalf("Len = %d, want 2", acc.Len())
	}
	steps := acc.Steps()
	if steps[0].ToolName != "Write" {
		t.Errorf("step 0 ToolName = %q", steps[0].ToolName)
	}
	// Default summary pulls file_path when explicit summary is empty.
	if !strings.Contains(steps[0].Summary, "/tmp/out.txt") {
		t.Errorf("step 0 default summary should include file_path; got %q", steps[0].Summary)
	}
	if steps[1].Summary != "list dir" {
		t.Errorf("step 1 Summary = %q, want explicit value", steps[1].Summary)
	}
}

func TestPlanAccumulator_Clear(t *testing.T) {
	acc := NewPlanAccumulator()
	acc.Append("Write", json.RawMessage(`{}`), "w")
	acc.Clear()
	if acc.Len() != 0 {
		t.Errorf("Clear should reset; Len=%d", acc.Len())
	}
}

func TestPlanAccumulator_Format(t *testing.T) {
	acc := NewPlanAccumulator()
	if got := acc.FormatSteps(); !strings.Contains(got, "no plan steps") {
		t.Errorf("empty FormatSteps should say none; got %q", got)
	}
	acc.Append("Edit", json.RawMessage(`{"file_path":"foo.go"}`), "")
	acc.Append("Bash", json.RawMessage(`{"command":"make"}`), "")
	got := acc.FormatSteps()
	for _, want := range []string{"Plan (2", "1.", "2.", "foo.go", "make"} {
		if !strings.Contains(got, want) {
			t.Errorf("format missing %q:\n%s", want, got)
		}
	}
}

func TestApp_ModeCommand_Plan(t *testing.T) {
	app := NewApp(AppOptions{}) // default Build
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "plan"})
	if cmd == nil {
		t.Fatal("expected batched cmd")
	}
	msgs := flattenBatch(cmd())
	var sawChange, sawBanner bool
	for _, m := range msgs {
		if mc, ok := m.(ModeChangeMsg); ok {
			if mc.Mode != protocol.ModePlan {
				t.Errorf("Mode = %q", mc.Mode)
			}
			sawChange = true
		}
		if b, ok := m.(BannerMsg); ok {
			if !strings.Contains(b.Text, "plan") {
				t.Errorf("banner should mention plan: %q", b.Text)
			}
			sawBanner = true
		}
	}
	if !sawChange || !sawBanner {
		t.Errorf("missing mode change or banner: change=%v banner=%v", sawChange, sawBanner)
	}
}

func TestApp_ModeCommand_Build(t *testing.T) {
	app := NewApp(AppOptions{InitialMode: protocol.ModePlan})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "build"})
	msgs := flattenBatch(cmd())
	var mode protocol.Mode
	for _, m := range msgs {
		if mc, ok := m.(ModeChangeMsg); ok {
			mode = mc.Mode
		}
	}
	if mode != protocol.ModeBuild {
		t.Errorf("/build should set Build; got %q", mode)
	}
}

func TestApp_ModeCommand_Auto(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "auto"})
	msgs := flattenBatch(cmd())
	var mode protocol.Mode
	for _, m := range msgs {
		if mc, ok := m.(ModeChangeMsg); ok {
			mode = mc.Mode
		}
	}
	if mode != protocol.ModeAuto {
		t.Errorf("/auto should set Auto; got %q", mode)
	}
}

func TestApp_ModeCommand_Idempotent(t *testing.T) {
	app := NewApp(AppOptions{InitialMode: protocol.ModePlan})
	// /plan while already in plan mode should be a no-op.
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "plan"})
	if cmd != nil {
		// Running the cmd shouldn't produce a ModeChangeMsg.
		for _, m := range flattenBatch(cmd()) {
			if _, ok := m.(ModeChangeMsg); ok {
				t.Errorf("idempotent /plan should not emit ModeChangeMsg")
			}
		}
	}
}

func TestDefaultPlanSummary_PreferredKeys(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"Write", `{"file_path":"/a","content":"x"}`, "Write /a"},
		{"Bash", `{"command":"ls -la"}`, "Bash ls -la"},
		{"WebFetch", `{"url":"https://x.test"}`, "WebFetch https://x.test"},
		{"Glob", `{"pattern":"*.go"}`, "Glob *.go"},
		{"Unknown", `{"irrelevant":true}`, "Unknown"},
		{"Broken", `{`, "Broken"},
	}
	for _, c := range cases {
		got := defaultPlanSummary(c.name, []byte(c.input))
		if got != c.want {
			t.Errorf("defaultPlanSummary(%q, %q) = %q, want %q", c.name, c.input, got, c.want)
		}
	}
}

// sanity — tea.Msg import stays used.
var _ tea.Msg = BannerMsg{}
