package harness

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harness/theme"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestNewApp_Defaults(t *testing.T) {
	app := NewApp(AppOptions{})
	if app.state.Mode != protocol.ModeBuild {
		t.Errorf("default Mode = %q, want build", app.state.Mode)
	}
	if app.state.Focus != InputFocus {
		t.Errorf("default Focus = %v, want InputFocus", app.state.Focus)
	}
	if app.state.ConnState.State != ConnStateDiscovering {
		t.Errorf("initial ConnState = %q", app.state.ConnState.State)
	}
}

func TestNewApp_InitialModeOption(t *testing.T) {
	app := NewApp(AppOptions{InitialMode: protocol.ModePlan})
	if app.state.Mode != protocol.ModePlan {
		t.Errorf("Mode = %q, want plan", app.state.Mode)
	}
}

func TestApp_WindowSize_RecalculatesLayout(t *testing.T) {
	app := NewApp(AppOptions{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)
	if app.width != 80 || app.height != 24 {
		t.Errorf("size = (%d, %d)", app.width, app.height)
	}
	if app.statusBar.Width() != 80 {
		t.Errorf("statusBar width not propagated: %d", app.statusBar.Width())
	}
}

func TestApp_QuitOnCtrlC(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatalf("expected Quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("cmd produced %T, want tea.QuitMsg", msg)
	}
}

// TestApp_ToggleMode exercises the WU-080 design D1 Ctrl+P semantics:
// plan↔build toggle, auto→build. cycleMode now returns the target and
// the App dispatches ModeChangeMsg + a banner via tea.Batch, so the
// test drains the batched messages back through Update.
func TestApp_ToggleMode(t *testing.T) {
	applyToggle := func(app App) App {
		model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
		app = model.(App)
		if cmd == nil {
			return app
		}
		for _, m := range flattenBatch(cmd()) {
			if mc, ok := m.(ModeChangeMsg); ok {
				model, _ := app.Update(mc)
				app = model.(App)
			}
		}
		return app
	}

	// default is Build. Ctrl+P → plan → build → plan …
	app := NewApp(AppOptions{})
	app = applyToggle(app)
	if app.state.Mode != protocol.ModePlan {
		t.Errorf("build→plan failed; got %q", app.state.Mode)
	}
	app = applyToggle(app)
	if app.state.Mode != protocol.ModeBuild {
		t.Errorf("plan→build failed; got %q", app.state.Mode)
	}

	// Enter auto explicitly, then Ctrl+P should drop to build.
	app.state.Mode = protocol.ModeAuto
	app = applyToggle(app)
	if app.state.Mode != protocol.ModeBuild {
		t.Errorf("auto→build failed; got %q", app.state.Mode)
	}
}

func TestApp_ConnStateMsg_UpdatesState(t *testing.T) {
	app := NewApp(AppOptions{})
	model, _ := app.Update(ConnStateMsg{Info: ConnStateInfo{State: ConnStateReady, Detail: "ok"}})
	app = model.(App)
	if app.state.ConnState.State != ConnStateReady {
		t.Errorf("state = %q", app.state.ConnState.State)
	}
}

func TestApp_ModelUpdateMsg(t *testing.T) {
	app := NewApp(AppOptions{})
	model, _ := app.Update(ModelUpdateMsg{Name: "claude-sonnet-4-6", Override: true, Routing: "user_override"})
	app = model.(App)
	if app.state.ModelName != "claude-sonnet-4-6" || !app.state.ModelOverride || app.state.ModelRouting != "user_override" {
		t.Errorf("state = %+v", app.state)
	}
}

func TestApp_ContextUpdateMsg(t *testing.T) {
	app := NewApp(AppOptions{})
	model, _ := app.Update(ContextUpdateMsg{Pct: 0.42, Used: 4200, Max: 10000})
	app = model.(App)
	if app.state.ContextPct != 0.42 || app.state.ContextUsed != 4200 || app.state.ContextMax != 10000 {
		t.Errorf("state = %+v", app.state)
	}
}

func TestApp_BannerMsg_ThenClear(t *testing.T) {
	app := NewApp(AppOptions{})
	model, _ := app.Update(BannerMsg{Text: "hi"})
	app = model.(App)
	if app.state.Banner != "hi" {
		t.Errorf("banner not set")
	}
	model, _ = app.Update(BannerClearMsg{})
	app = model.(App)
	if app.state.Banner != "" {
		t.Errorf("banner not cleared")
	}
}

func TestApp_BannerMsg_AutoClearOnTick(t *testing.T) {
	app := NewApp(AppOptions{})
	model, _ := app.Update(BannerMsg{Text: "transient", Duration: 1 * time.Millisecond})
	app = model.(App)
	if app.state.Banner != "transient" {
		t.Errorf("banner not set")
	}
	// Tick after duration → cleared.
	model, _ = app.Update(TickMsg(time.Now().Add(10 * time.Millisecond)))
	app = model.(App)
	if app.state.Banner != "" {
		t.Errorf("banner not auto-cleared on tick: %q", app.state.Banner)
	}
}

func TestApp_StreamingDelta_AppendsToActiveAssistantMessage(t *testing.T) {
	app := NewApp(AppOptions{})
	model, _ := app.Update(StreamTokenMsg{TurnID: "t1", Delta: "hello "})
	app = model.(App)
	model, _ = app.Update(StreamTokenMsg{TurnID: "t1", Delta: "world"})
	app = model.(App)
	if len(app.state.Messages) != 1 {
		t.Fatalf("messages = %d", len(app.state.Messages))
	}
	if app.state.Messages[0].Content != "hello world" {
		t.Errorf("content = %q", app.state.Messages[0].Content)
	}
	if !app.state.Messages[0].Streaming {
		t.Errorf("Streaming flag should be true mid-stream")
	}
}

func TestApp_StreamComplete_FinalizesMetadata(t *testing.T) {
	app := NewApp(AppOptions{})
	model, _ := app.Update(StreamTokenMsg{TurnID: "t1", Delta: "hi"})
	app = model.(App)
	model, _ = app.Update(StreamCompleteMsg{
		TurnID: "t1", Tokens: TokenInfo{Input: 10, Output: 20},
		Cost: 0.001, Duration: 100 * time.Millisecond, Model: "claude-sonnet-4-6",
	})
	app = model.(App)
	m := app.state.Messages[0]
	if m.Streaming {
		t.Errorf("Streaming flag not cleared")
	}
	if m.Tokens.Input != 10 || m.Tokens.Output != 20 || m.Cost != 0.001 || m.Model != "claude-sonnet-4-6" {
		t.Errorf("metadata = %+v", m)
	}
}

func TestApp_FocusSwitch_UpAtTopMovesToViewport(t *testing.T) {
	app := NewApp(AppOptions{})
	app.state.Focus = InputFocus
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyUp})
	app = model.(App)
	if app.state.Focus != ViewportFocus {
		t.Errorf("focus = %v, want viewport (input cursor at top)", app.state.Focus)
	}
}

func TestApp_FocusSwitch_DownAtBottomMovesToInput(t *testing.T) {
	app := NewApp(AppOptions{})
	app.state.Focus = ViewportFocus
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = model.(App)
	if app.state.Focus != InputFocus {
		t.Errorf("focus = %v, want input (viewport at bottom)", app.state.Focus)
	}
}

func TestApp_FocusSwitch_PrintableInViewportSwitchesToInput(t *testing.T) {
	app := NewApp(AppOptions{})
	app.state.Focus = ViewportFocus
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	app = model.(App)
	if app.state.Focus != InputFocus {
		t.Errorf("focus = %v, want input (printable rune)", app.state.Focus)
	}
}

func TestApp_DispatchSubmit_ProducesSubmitMsg(t *testing.T) {
	app := NewApp(AppOptions{})
	app.input.SetValue("hello world")
	cmd := app.dispatchSubmit()
	if cmd == nil {
		t.Fatalf("nil cmd")
	}
	msg := cmd()
	sm, ok := msg.(SubmitMsg)
	if !ok {
		t.Fatalf("msg type = %T", msg)
	}
	if sm.Content != "hello world" || sm.IsCommand {
		t.Errorf("submit = %+v", sm)
	}
	if app.input.Value() != "" {
		t.Errorf("input not cleared")
	}
}

func TestApp_DispatchSubmit_ParsesCommand(t *testing.T) {
	app := NewApp(AppOptions{})
	app.input.SetValue("/model claude-sonnet-4-6")
	cmd := app.dispatchSubmit()
	msg := cmd().(SubmitMsg)
	if !msg.IsCommand || msg.Command != "model" || msg.CommandArgs != "claude-sonnet-4-6" {
		t.Errorf("submit = %+v", msg)
	}
}

func TestApp_DispatchSubmit_EmptyValueProducesNoCmd(t *testing.T) {
	app := NewApp(AppOptions{})
	app.input.SetValue("   ")
	if cmd := app.dispatchSubmit(); cmd != nil {
		t.Errorf("expected nil cmd for blank input, got %v", cmd)
	}
}

func TestApp_View_ContainsZones(t *testing.T) {
	app := NewApp(AppOptions{})
	app.state.ConnState = ConnStateInfo{State: ConnStateReady}
	app.state.Messages = []DisplayMessage{{Role: RoleAssistant, Content: "hello!"}}
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)
	view := app.View()
	if !strings.Contains(view, "hello!") {
		t.Errorf("View missing assistant content:\n%s", view)
	}
	// Status bar renders the ready state as a colored badge "[●]"
	// (WU-069). Assert the badge is present rather than the literal
	// state string.
	if !strings.Contains(view, "[●]") {
		t.Errorf("View missing status bar ready badge:\n%s", view)
	}
}

// TestApp_SetTheme_Smoke verifies that SetTheme propagates to all child
// components and View renders without panic when a theme is active.
func TestApp_SetTheme_Smoke(t *testing.T) {
	app := NewApp(AppOptions{})

	// The "system" theme is registered on package init.
	theme := theme.CurrentTheme()
	if theme == nil {
		t.Skip("no theme registered")
	}

	app.SetTheme(theme)

	// Set a size so the viewport and textarea allocate width.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	// Add a message so the viewport has content to render.
	app.state.Messages = []DisplayMessage{
		{Role: RoleUser, Content: "test message"},
	}

	// View should render without panic and contain the themed prefix.
	view := app.View()
	if !strings.Contains(view, "test message") {
		t.Errorf("View missing user content:\n%s", view)
	}
}

func TestAppState_NextSequence_Monotonic(t *testing.T) {
	s := NewAppState()
	if got := s.NextSequence(); got != 1 {
		t.Errorf("first NextSequence = %d, want 1", got)
	}
	if got := s.NextSequence(); got != 2 {
		t.Errorf("second NextSequence = %d, want 2", got)
	}
	s.ResetSequence()
	if got := s.NextSequence(); got != 1 {
		t.Errorf("after reset, NextSequence = %d, want 1", got)
	}
}

func TestFocusZone_String(t *testing.T) {
	if InputFocus.String() != "input" || ViewportFocus.String() != "viewport" {
		t.Errorf("FocusZone.String() mismatch")
	}
}

func TestTerminalResponseFilter(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.Msg
		want tea.Msg
	}{
		{"nil passthrough", nil, nil},
		{"WindowSize passthrough", tea.WindowSizeMsg{Width: 80, Height: 24}, tea.WindowSizeMsg{Width: 80, Height: 24}},
		{"normal key passthrough", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}},
		{"single bracket passthrough", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}},
		{"single close bracket passthrough", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}},
		{"OSC response dropped", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]11;rgb:1919/1a1a/1b1b")}, nil},
		{"CPR dropped", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[1;1R")}, nil},
		{"CSI focus-in dropped", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[I")}, nil},
		{"CSI focus-out dropped", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[O")}, nil},
		{"ST terminator dropped", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\\'}, Alt: true}, nil},
		{"ESC prefix dropped", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\x1b[?2004h")}, nil},
		{"Alt backslash passthrough (not ST)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', '\\'}, Alt: true}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', '\\'}, Alt: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TerminalResponseFilter(nil, tt.msg)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TerminalResponseFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}
