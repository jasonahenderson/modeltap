package harness

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// runCmd drains a tea.Cmd into zero or one produced message, which is
// enough for ConnectionUX tests (HandleConnState always emits at most
// one follow-up).
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestConnectionUX_Ready_ClearsBanner(t *testing.T) {
	state := NewAppState()
	state.Banner = "stale banner"
	cux := NewConnectionUX(state)

	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{State: ConnStateReady}}))
	if _, ok := msg.(BannerClearMsg); !ok {
		t.Fatalf("expected BannerClearMsg, got %T (%+v)", msg, msg)
	}
}

func TestConnectionUX_Starting_SetsBanner(t *testing.T) {
	cux := NewConnectionUX(NewAppState())
	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{State: ConnStateStarting}}))
	b, ok := msg.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", msg)
	}
	if !strings.Contains(strings.ToLower(b.Text), "starting") {
		t.Errorf("banner should mention starting; got %q", b.Text)
	}
}

func TestConnectionUX_Discovering_SetsBanner(t *testing.T) {
	cux := NewConnectionUX(NewAppState())
	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{State: ConnStateDiscovering}}))
	b, ok := msg.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", msg)
	}
	if !strings.Contains(strings.ToLower(b.Text), "discover") &&
		!strings.Contains(strings.ToLower(b.Text), "starting") {
		t.Errorf("banner should describe discovery; got %q", b.Text)
	}
}

func TestConnectionUX_Authenticating_SetsBanner(t *testing.T) {
	cux := NewConnectionUX(NewAppState())
	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{State: ConnStateAuthenticating}}))
	b, ok := msg.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", msg)
	}
	if !strings.Contains(strings.ToLower(b.Text), "auth") {
		t.Errorf("banner should mention authentication; got %q", b.Text)
	}
}

func TestConnectionUX_Registering_IncludesDetail(t *testing.T) {
	cux := NewConnectionUX(NewAppState())
	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{
		State:  ConnStateRegistering,
		Detail: "13 built-in + 2 MCP",
	}}))
	b, ok := msg.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", msg)
	}
	if !strings.Contains(b.Text, "13 built-in + 2 MCP") {
		t.Errorf("banner should include Detail; got %q", b.Text)
	}
}

func TestConnectionUX_Degraded_PersistentBanner(t *testing.T) {
	cux := NewConnectionUX(NewAppState())
	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{
		State:  ConnStateDegraded,
		Detail: "2 heartbeats missed",
	}}))
	b, ok := msg.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", msg)
	}
	if b.Duration != 0 {
		t.Errorf("degraded banner should be persistent (Duration=0); got %v", b.Duration)
	}
	if !strings.Contains(strings.ToLower(b.Text), "degrad") {
		t.Errorf("banner should mention degradation; got %q", b.Text)
	}
	if !strings.Contains(b.Text, "2 heartbeats missed") {
		t.Errorf("banner should include Detail; got %q", b.Text)
	}
}

func TestConnectionUX_Reconnecting_IncludesAttempt(t *testing.T) {
	cux := NewConnectionUX(NewAppState())
	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{
		State:      ConnStateReconnecting,
		Attempt:    3,
		MaxRetries: 5,
	}}))
	b, ok := msg.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", msg)
	}
	if !strings.Contains(b.Text, "3") || !strings.Contains(b.Text, "5") {
		t.Errorf("banner should include attempt N/M; got %q", b.Text)
	}
	if !strings.Contains(strings.ToLower(b.Text), "reconnect") {
		t.Errorf("banner should mention reconnecting; got %q", b.Text)
	}
}

func TestConnectionUX_Failed_PersistentWithDiagnostic(t *testing.T) {
	cux := NewConnectionUX(NewAppState())
	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{
		State:  ConnStateFailed,
		Detail: "MT-CONN-001 socket refused — run `modeltap start`",
	}}))
	b, ok := msg.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", msg)
	}
	if b.Duration != 0 {
		t.Errorf("failed banner should be persistent (Duration=0); got %v", b.Duration)
	}
	if !strings.Contains(b.Text, "MT-CONN-001") {
		t.Errorf("banner should surface diagnostic code; got %q", b.Text)
	}
}

func TestConnectionUX_Connecting_TransientBanner(t *testing.T) {
	cux := NewConnectionUX(NewAppState())
	msg := runCmd(cux.HandleConnState(ConnStateMsg{Info: ConnStateInfo{State: ConnStateConnecting}}))
	b, ok := msg.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg, got %T", msg)
	}
	// Connecting is a transient progress state — banner should not be empty.
	if b.Text == "" {
		t.Error("connecting should surface a banner")
	}
}

func TestApp_ConnStateMsg_UpdatesBanner(t *testing.T) {
	app := NewApp(AppOptions{})
	// Transition to reconnecting — the App should end up with a non-empty
	// banner after the follow-up cmd from ConnectionUX runs.
	model, cmd := app.Update(ConnStateMsg{Info: ConnStateInfo{
		State:      ConnStateReconnecting,
		Attempt:    2,
		MaxRetries: 5,
	}})
	a, ok := model.(App)
	if !ok {
		t.Fatalf("unexpected model type %T", model)
	}
	if a.state.ConnState.State != ConnStateReconnecting {
		t.Errorf("state not updated: %q", a.state.ConnState.State)
	}
	if cmd == nil {
		t.Fatal("expected a follow-up Cmd from ConnectionUX")
	}
	follow := cmd()
	b, ok := follow.(BannerMsg)
	if !ok {
		t.Fatalf("expected BannerMsg from follow-up, got %T", follow)
	}
	if !strings.Contains(b.Text, "2") {
		t.Errorf("follow-up banner should include attempt: %q", b.Text)
	}
}

func TestApp_ConnStateMsg_Ready_ClearsBanner(t *testing.T) {
	app := NewApp(AppOptions{})
	app.state.Banner = "old banner"

	_, cmd := app.Update(ConnStateMsg{Info: ConnStateInfo{State: ConnStateReady}})
	if cmd == nil {
		t.Fatal("expected a follow-up Cmd clearing the banner")
	}
	follow := cmd()
	// The ConnStateMsg handler may batch the banner-clear with a
	// background history load; unwrap BatchMsg to find BannerClearMsg.
	if batch, ok := follow.(tea.BatchMsg); ok {
		for _, c := range batch {
			m := c()
			if _, ok := m.(BannerClearMsg); ok {
				return
			}
		}
		t.Fatalf("expected BannerClearMsg inside BatchMsg, got %T", follow)
	}
	if _, ok := follow.(BannerClearMsg); !ok {
		t.Fatalf("expected BannerClearMsg, got %T", follow)
	}
}

// sanity: time.Duration import used above
var _ = time.Second
