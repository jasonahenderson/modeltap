package harness

import (
	"errors"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func TestApp_ShowCurrentModel_NoConn(t *testing.T) {
	app := NewApp(AppOptions{})
	app.state.ModelName = "gpt-5"
	app.state.ModelOverride = true
	app.state.ModelRouting = "coding.review"

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "model"})
	b, ok := drainCmdAny(cmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", cmd())
	}
	for _, want := range []string{"gpt-5", "override", "coding.review"} {
		if !strings.Contains(b.Text, want) {
			t.Errorf("banner should include %q: %q", want, b.Text)
		}
	}
}

func TestApp_ModelsCommand_Success(t *testing.T) {
	fc := &fakeClient{modelListResp: protocol.ModelListResponse{
		Models: []protocol.ModelInfo{
			{Name: "claude-opus-4-7", Provider: "anthropic", Description: "flagship"},
			{Name: "gpt-5", Provider: "openai", Description: "general"},
		},
		CurrentOverride: "gpt-5",
	}}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "models"})
	if cmd == nil {
		t.Fatal("expected dispatch cmd")
	}
	msg := drainCmdAny(cmd)
	loaded, ok := msg.(ModelListLoadedMsg)
	if !ok {
		t.Fatalf("expected ModelListLoadedMsg, got %T", msg)
	}
	if loaded.Response == nil || len(loaded.Response.Models) != 2 {
		t.Fatalf("unexpected response %+v", loaded.Response)
	}
	if fc.modelListCalls != 1 {
		t.Errorf("modelListCalls = %d, want 1", fc.modelListCalls)
	}

	// Feed the loaded msg back through Update to confirm banner formatting.
	_, bannerCmd := app.Update(loaded)
	b, ok := drainCmdAny(bannerCmd).(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", bannerCmd())
	}
	for _, want := range []string{"claude-opus-4-7", "gpt-5", "(current)", "flagship", "general"} {
		if !strings.Contains(b.Text, want) {
			t.Errorf("banner missing %q:\n%s", want, b.Text)
		}
	}
}

func TestApp_ModelsCommand_Empty(t *testing.T) {
	fc := &fakeClient{modelListResp: protocol.ModelListResponse{}}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "models"})
	loaded := drainCmdAny(cmd).(ModelListLoadedMsg)
	_, bc := app.Update(loaded)
	b := drainCmdAny(bc).(BannerMsg)
	if !strings.Contains(strings.ToLower(b.Text), "no models") {
		t.Errorf("empty catalog should say no models; got %q", b.Text)
	}
}

func TestApp_ModelsCommand_NoConn_ErrorsSurface(t *testing.T) {
	app := NewApp(AppOptions{})
	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "models"})
	msg := drainCmdAny(cmd)
	e, ok := msg.(ModelErrMsg)
	if !ok {
		t.Fatalf("expected ModelErrMsg, got %T", msg)
	}
	if e.Command != "models" {
		t.Errorf("Command = %q", e.Command)
	}

	// Feed back to get banner.
	_, bc := app.Update(e)
	b := drainCmdAny(bc).(BannerMsg)
	if !strings.Contains(b.Text, "/models") {
		t.Errorf("banner should mention /models: %q", b.Text)
	}
}

func TestApp_ModelSwitch_Success(t *testing.T) {
	fc := &fakeClient{modelSwitchResp: protocol.ModelSwitchResponse{
		OverrideSet: true,
		Model:       "claude-opus-4-7",
		Reason:      "user override",
	}}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.state.SessionID = "sess-7"

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "model", CommandArgs: "claude-opus-4-7"})
	switched := drainCmdAny(cmd).(ModelSwitchedMsg)
	if switched.Response == nil || switched.Response.Model != "claude-opus-4-7" {
		t.Fatalf("unexpected response %+v", switched.Response)
	}
	if len(fc.modelSwitchCalls) != 1 {
		t.Fatalf("modelSwitchCalls = %d", len(fc.modelSwitchCalls))
	}
	call := fc.modelSwitchCalls[0]
	if call.SessionID != "sess-7" || call.Model != "claude-opus-4-7" {
		t.Errorf("switch call wrong: %+v", call)
	}

	// Apply msg to state + banner.
	model, bc := app.Update(switched)
	a, _ := model.(App)
	if !a.state.ModelOverride || a.state.ModelName != "claude-opus-4-7" {
		t.Errorf("state not updated: override=%v name=%q", a.state.ModelOverride, a.state.ModelName)
	}
	b := drainCmdAny(bc).(BannerMsg)
	if !strings.Contains(b.Text, "claude-opus-4-7") || !strings.Contains(b.Text, "user override") {
		t.Errorf("banner should include model + reason: %q", b.Text)
	}
}

func TestApp_ModelSwitch_Auto_ClearsOverride(t *testing.T) {
	fc := &fakeClient{modelSwitchResp: protocol.ModelSwitchResponse{
		OverrideSet: false,
		Reason:      "override cleared",
	}}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})
	app.state.ModelOverride = true
	app.state.ModelName = "stale"

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "model", CommandArgs: "auto"})
	switched := drainCmdAny(cmd).(ModelSwitchedMsg)

	model, bc := app.Update(switched)
	a, _ := model.(App)
	if a.state.ModelOverride {
		t.Error("override should be cleared")
	}
	// Model name is preserved from previous state when Response.Model is empty.
	if a.state.ModelName != "stale" {
		t.Errorf("ModelName = %q; should not clobber when Response.Model is empty", a.state.ModelName)
	}
	b := drainCmdAny(bc).(BannerMsg)
	if !strings.Contains(b.Text, "override cleared") {
		t.Errorf("banner should surface reason: %q", b.Text)
	}
	// The switch call sent "auto" to the BFF.
	if fc.modelSwitchCalls[0].Model != "auto" {
		t.Errorf("expected model=auto; got %q", fc.modelSwitchCalls[0].Model)
	}
}

func TestApp_ModelSwitch_Error(t *testing.T) {
	fc := &fakeClient{modelSwitchErr: errors.New("unknown model")}
	conn := &fakeConn{state: ConnStateReady, client: fc}
	app := NewApp(AppOptions{Conn: conn})

	_, cmd := app.Update(SubmitMsg{IsCommand: true, Command: "model", CommandArgs: "bogus"})
	e := drainCmdAny(cmd).(ModelErrMsg)
	if e.Command != "model" {
		t.Errorf("Command = %q", e.Command)
	}
	if !strings.Contains(e.Err.Error(), "unknown model") {
		t.Errorf("Err = %v", e.Err)
	}
}

func TestFormatModelSwitched_Variants(t *testing.T) {
	cases := []struct {
		resp *protocol.ModelSwitchResponse
		want string
	}{
		{nil, "Model switch completed."},
		{&protocol.ModelSwitchResponse{OverrideSet: false}, "routing policy restored"},
		{&protocol.ModelSwitchResponse{OverrideSet: false, Reason: "explicit"}, "explicit"},
		{&protocol.ModelSwitchResponse{OverrideSet: true, Model: "m"}, "Model override set: m"},
	}
	for _, c := range cases {
		got := formatModelSwitched(c.resp)
		if !strings.Contains(got, c.want) {
			t.Errorf("formatModelSwitched(%+v) = %q, want substring %q", c.resp, got, c.want)
		}
	}
}
