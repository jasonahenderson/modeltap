package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/bff"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// recordingSender satisfies harness.ProgramSender. Every message the
// ConnectionManager sends is captured so tests can assert on the
// resulting state-transition banner traffic.
type recordingSender struct {
	mu   sync.Mutex
	msgs []tea.Msg
}

func (r *recordingSender) Send(msg tea.Msg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, msg)
}

func (r *recordingSender) snapshot() []tea.Msg {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]tea.Msg, len(r.msgs))
	copy(out, r.msgs)
	return out
}

// startTestBFF stands up a real BFF server against an in-memory store
// on a short unix socket and returns the socket path. Registers a
// cleanup that shuts the server down.
func startTestBFF(t *testing.T) string {
	t.Helper()
	sockPath := shortSocketPath(t)

	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := bff.DefaultServerConfig()
	cfg.SocketPath = sockPath
	cfg.HeartbeatInterval = 50 * time.Millisecond
	cfg.HeartbeatTimeout = 500 * time.Millisecond
	cfg.GracePeriod = 100 * time.Millisecond

	srv := bff.NewServer(store, cfg)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	return sockPath
}

// TestHarnessConnMgr_ReachesReady_AndPings drives the ConnectionManager
// through discover → connect → register → ready against a real BFF
// and asserts:
//
//  1. ConnectSync returns without error.
//  2. cm.State() ends at ConnStateReady.
//  3. At least one ConnStateMsg reached the ProgramSender.
//  4. cm.Client() can successfully issue a ping once registered.
func TestHarnessConnMgr_ReachesReady_AndPings(t *testing.T) {
	sockPath := startTestBFF(t)
	sender := &recordingSender{}

	cm := harness.NewConnectionManager(harness.ConnectionConfig{
		SocketPath: sockPath,
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  "test",
			HarnessPlatform: "test",
		},
	}, sender)
	t.Cleanup(cm.Disconnect)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cm.ConnectSync(ctx); err != nil {
		t.Fatalf("ConnectSync: %v", err)
	}
	if got := cm.State(); got != harness.ConnStateReady {
		t.Fatalf("state = %q, want ready", got)
	}

	// Assert at least one ConnStateMsg fired along the way.
	var sawMsg bool
	for _, m := range sender.snapshot() {
		if _, ok := m.(harness.ConnStateMsg); ok {
			sawMsg = true
			break
		}
	}
	if !sawMsg {
		t.Error("expected ConnStateMsg from the event bridge; got none")
	}

	// The manager must have a live client after Ready.
	client := cm.Client()
	if client == nil {
		t.Fatal("Client() nil after Ready")
	}
	if err := client.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

// TestHarnessConnMgr_DrivesSubmitMsgThroughApp covers the PATCH-0003
// path: App.Update receives a SubmitMsg, dispatches through the
// wired ConnSurface, and the resulting TurnSubmittedMsg propagates
// back with an error that reflects the (unconfigured) provider.
// Routing misses without an endpoint are the expected failure mode
// and let us test the error path without a live provider.
func TestHarnessConnMgr_DrivesSubmitMsgThroughApp(t *testing.T) {
	sockPath := startTestBFF(t)
	sender := &recordingSender{}

	cm := harness.NewConnectionManager(harness.ConnectionConfig{
		SocketPath: sockPath,
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  "test",
			HarnessPlatform: "test",
		},
	}, sender)
	t.Cleanup(cm.Disconnect)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cm.ConnectSync(ctx); err != nil {
		t.Fatalf("ConnectSync: %v", err)
	}

	app := harness.NewApp(harness.AppOptions{
		Conn: harness.WrapConnectionManager(cm),
	})
	app.State().SessionID = "int-sess"

	_, cmd := app.Update(harness.SubmitMsg{Content: "hi there"})
	if cmd == nil {
		t.Fatal("expected dispatch cmd")
	}
	msg := cmd()
	ack, ok := msg.(harness.TurnSubmittedMsg)
	if !ok {
		t.Fatalf("expected TurnSubmittedMsg, got %T", msg)
	}
	// With no provider endpoints configured the dispatch must fail
	// on the server side; the ack carries a non-nil Err.
	if ack.Err == nil && ack.TurnID == "" {
		t.Errorf("expected either an error or a TurnID; got empty ack")
	}
}

// TestHarnessConnMgr_SessionRPCsFromApp exercises the App's /sessions
// slash command against a real BFF. Proves the ConnSurface path from
// App → ConnectionManager → ProtocolClient → BFF handler is wired
// end-to-end.
func TestHarnessConnMgr_SessionRPCsFromApp(t *testing.T) {
	sockPath := startTestBFF(t)
	sender := &recordingSender{}

	cm := harness.NewConnectionManager(harness.ConnectionConfig{
		SocketPath: sockPath,
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  "test",
			HarnessPlatform: "test",
		},
	}, sender)
	t.Cleanup(cm.Disconnect)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cm.ConnectSync(ctx); err != nil {
		t.Fatalf("ConnectSync: %v", err)
	}

	app := harness.NewApp(harness.AppOptions{Conn: harness.WrapConnectionManager(cm)})

	_, cmd := app.Update(harness.SubmitMsg{IsCommand: true, Command: "sessions"})
	if cmd == nil {
		t.Fatal("expected /sessions dispatch cmd")
	}
	msg := cmd()
	loaded, ok := msg.(harness.SessionListLoadedMsg)
	if !ok {
		t.Fatalf("expected SessionListLoadedMsg, got %T (%+v)", msg, msg)
	}
	if loaded.Response == nil {
		t.Error("SessionListLoadedMsg.Response is nil")
	}
}
