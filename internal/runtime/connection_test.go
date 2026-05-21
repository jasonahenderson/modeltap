package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// recordingStore embeds storage.Store (left nil) and overrides the
// methods Connection actually exercises in WU-048 tests
// (ReleaseSessionLock). Other methods will nil-pointer panic — none are
// called by the connection state machine.
type recordingStore struct {
	storage.Store

	mu       sync.Mutex
	released []releaseCall
	pingErr  error
}

type releaseCall struct {
	sessionID string
	owner     string
}

func (s *recordingStore) ReleaseSessionLock(_ context.Context, sessionID, owner string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.released = append(s.released, releaseCall{sessionID: sessionID, owner: owner})
	return nil
}

func (s *recordingStore) Ping(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pingErr
}

func (s *recordingStore) setPingErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pingErr = err
}

func (s *recordingStore) releases() []releaseCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]releaseCall, len(s.released))
	copy(out, s.released)
	return out
}

// newTestServer returns a Server configured for unit testing — small
// timeouts so heartbeat / grace period tests run in milliseconds.
func newTestServer(t *testing.T, store storage.Store) *Server {
	t.Helper()
	return &Server{
		store:      store,
		dispatcher: NewDispatcher(),
		config: ServerConfig{
			HeartbeatInterval: 10 * time.Millisecond,
			HeartbeatTimeout:  50 * time.Millisecond,
			GracePeriod:       30 * time.Millisecond,
			MaxConnections:    100,
		},
		startTime: time.Now(),
	}
}

func newPipeConnection(t *testing.T, srv *Server, requiresAuth bool) (*Connection, net.Conn) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() { _ = clientConn.Close() })
	t.Cleanup(func() { _ = serverConn.Close() })

	tr := NewFrameTransport(serverConn)
	c := NewConnection("conn-test", tr, srv, requiresAuth)
	return c, clientConn
}

func TestConnState_StringValues(t *testing.T) {
	// Pin every wire-visible state name to its FEAT-0008 canonical form.
	cases := []struct {
		state ConnState
		want  string
	}{
		{ConnDiscovering, "discovering"},
		{ConnStarting, "starting"},
		{ConnConnecting, "connecting"},
		{ConnAuthenticating, "authenticating"},
		{ConnRegistering, "registering"},
		{ConnReady, "ready"},
		{ConnDegraded, "degraded"},
		{ConnReconnecting, "reconnecting"},
		{ConnFailed, "failed"},
	}
	for _, c := range cases {
		if got := c.state.String(); got != c.want {
			t.Errorf("ConnState(%d).String() = %q, want %q", c.state, got, c.want)
		}
	}
}

func TestConnection_StateTransitions(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	c, _ := newPipeConnection(t, srv, false)

	// All transitions per validTransitions must succeed.
	cases := []struct {
		name     string
		from, to ConnState
	}{
		{"connecting->authenticating", ConnConnecting, ConnAuthenticating},
		{"connecting->registering", ConnConnecting, ConnRegistering},
		{"connecting->failed", ConnConnecting, ConnFailed},
		{"authenticating->registering", ConnAuthenticating, ConnRegistering},
		{"authenticating->failed", ConnAuthenticating, ConnFailed},
		{"registering->ready", ConnRegistering, ConnReady},
		{"registering->failed", ConnRegistering, ConnFailed},
		{"ready->degraded", ConnReady, ConnDegraded},
		{"ready->failed", ConnReady, ConnFailed},
		{"degraded->ready", ConnDegraded, ConnReady},
		{"degraded->failed", ConnDegraded, ConnFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c.setStateForTest(tc.from)
			if !c.transition(tc.to) {
				t.Errorf("transition %v->%v should succeed", tc.from, tc.to)
			}
			if c.State() != tc.to {
				t.Errorf("state = %v, want %v", c.State(), tc.to)
			}
		})
	}
}

func TestConnection_InvalidTransition(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	c, _ := newPipeConnection(t, srv, false)

	// A few invalid transitions per validTransitions.
	cases := []struct {
		name     string
		from, to ConnState
	}{
		{"ready->connecting", ConnReady, ConnConnecting},
		{"ready->registering", ConnReady, ConnRegistering},
		{"failed->ready", ConnFailed, ConnReady},
		{"connecting->ready", ConnConnecting, ConnReady}, // must go through registering
		{"registering->degraded", ConnRegistering, ConnDegraded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c.setStateForTest(tc.from)
			if c.transition(tc.to) {
				t.Errorf("transition %v->%v should fail", tc.from, tc.to)
			}
			if c.State() != tc.from {
				t.Errorf("invalid transition mutated state to %v", c.State())
			}
		})
	}
}

func TestConnection_SocketSkipsAuth(t *testing.T) {
	// requiresAuth=false (unix socket): initialize() goes Connecting -> Registering directly.
	srv := newTestServer(t, &recordingStore{})
	c, _ := newPipeConnection(t, srv, false)

	c.initialize()
	if c.State() != ConnRegistering {
		t.Errorf("unix socket initialize() ended in state %v, want registering", c.State())
	}
	if c.Visited(ConnAuthenticating) {
		t.Errorf("unix socket should NOT visit authenticating")
	}
}

func TestConnection_TLSGoesThroughAuth(t *testing.T) {
	// requiresAuth=true (TLS): initialize() goes Connecting -> Authenticating -> Registering.
	srv := newTestServer(t, &recordingStore{})
	c, _ := newPipeConnection(t, srv, true)

	c.initialize()
	if c.State() != ConnRegistering {
		t.Errorf("TLS initialize() ended in state %v, want registering", c.State())
	}
	if !c.Visited(ConnAuthenticating) {
		t.Errorf("TLS should visit authenticating")
	}
}

func TestConnection_DispatchGating_NotReady(t *testing.T) {
	// In ConnRegistering, methods other than capabilities.register and
	// connection.ping must be rejected with CodeNotReady.
	srv := newTestServer(t, &recordingStore{})
	srv.dispatcher.Register("test.method", func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
		return "should not run", nil
	})

	c, _ := newPipeConnection(t, srv, false)
	c.setStateForTest(ConnRegistering)

	_, err := c.dispatchForTest(context.Background(), &protocol.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "test.method",
	})
	if err == nil {
		t.Fatalf("expected gating to reject test.method in registering")
	}
	var te *TransportError
	if !errors.As(err, &te) || te.Code != CodeNotReady {
		t.Errorf("expected CodeNotReady, got %T %v", err, err)
	}
}

func TestConnection_DispatchGating_RegisterAllowed(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	called := false
	srv.dispatcher.Register(protocol.MethodCapabilitiesRegister, func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
		called = true
		return map[string]string{}, nil
	})

	c, _ := newPipeConnection(t, srv, false)
	c.setStateForTest(ConnRegistering)

	if _, err := c.dispatchForTest(context.Background(), &protocol.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodCapabilitiesRegister,
	}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !called {
		t.Errorf("capabilities.register handler not called")
	}
}

func TestConnection_DispatchGating_PingAllowed(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	srv.dispatcher.Register(protocol.MethodConnectionPing, handleConnectionPing)

	c, _ := newPipeConnection(t, srv, false)

	for _, state := range []ConnState{ConnAuthenticating, ConnRegistering, ConnReady, ConnDegraded} {
		t.Run(state.String(), func(t *testing.T) {
			c.setStateForTest(state)
			if _, err := c.dispatchForTest(context.Background(), &protocol.Request{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`1`),
				Method:  protocol.MethodConnectionPing,
			}); err != nil {
				t.Errorf("ping rejected in state %v: %v", state, err)
			}
		})
	}
}

func TestConnection_Heartbeat_PingUpdates(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	c, _ := newPipeConnection(t, srv, false)

	before := c.LastPing()
	time.Sleep(2 * time.Millisecond)
	if _, err := handleConnectionPing(context.Background(), c, nil); err != nil {
		t.Fatalf("handleConnectionPing: %v", err)
	}
	after := c.LastPing()
	if !after.After(before) {
		t.Errorf("LastPing not advanced (before=%v after=%v)", before, after)
	}
}

func TestConnection_Heartbeat_Timeout(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	c, _ := newPipeConnection(t, srv, false)
	c.setStateForTest(ConnReady)

	// Backdate lastPing past the timeout window so the monitor fires
	// on its next tick.
	c.setLastPingForTest(time.Now().Add(-1 * time.Second))

	c.startHeartbeatMonitor()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c.State() == ConnFailed {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("connection did not transition to ConnFailed within deadline; state=%v", c.State())
}

func TestConnection_Heartbeat_InitialGrace(t *testing.T) {
	// lastPing is initialized to time.Now() at construction, so the
	// monitor must NOT mark the connection failed immediately even when
	// no harness ping has yet been received.
	srv := newTestServer(t, &recordingStore{})
	c, _ := newPipeConnection(t, srv, false)
	c.setStateForTest(ConnReady)

	c.startHeartbeatMonitor()

	// Wait one heartbeat-interval check; with no pings but a fresh lastPing,
	// state must remain ConnReady (timeout is HeartbeatTimeout > Interval).
	time.Sleep(srv.config.HeartbeatInterval + 5*time.Millisecond)
	if c.State() != ConnReady {
		t.Errorf("connection failed within initial grace; state=%v", c.State())
	}
	c.stopHeartbeatMonitor()
}

func TestConnection_GracePeriod_Expires(t *testing.T) {
	store := &recordingStore{}
	srv := newTestServer(t, store)

	c, _ := newPipeConnection(t, srv, false)
	c.setStateForTest(ConnReady)
	c.SetSessionID("sess-1")

	c.scheduleGracePeriodRelease()

	// Wait beyond the grace period.
	time.Sleep(srv.config.GracePeriod + 30*time.Millisecond)

	releases := store.releases()
	if len(releases) != 1 {
		t.Fatalf("got %d release calls, want 1", len(releases))
	}
	if releases[0].sessionID != "sess-1" || releases[0].owner != c.ID() {
		t.Errorf("release call = %+v", releases[0])
	}
}

func TestConnection_GracePeriod_Cancelled(t *testing.T) {
	store := &recordingStore{}
	srv := newTestServer(t, store)

	c, _ := newPipeConnection(t, srv, false)
	c.setStateForTest(ConnReady)
	c.SetSessionID("sess-1")

	c.scheduleGracePeriodRelease()
	// Simulate a reconnect cancelling the timer before it fires.
	c.cancelGracePeriodRelease()

	time.Sleep(srv.config.GracePeriod + 30*time.Millisecond)
	if releases := store.releases(); len(releases) != 0 {
		t.Errorf("expected no releases after cancel, got %d: %+v", len(releases), releases)
	}
}

func TestConnection_GracePeriod_NoSession(t *testing.T) {
	store := &recordingStore{}
	srv := newTestServer(t, store)

	c, _ := newPipeConnection(t, srv, false)
	c.setStateForTest(ConnReady)
	// no SetSessionID

	c.scheduleGracePeriodRelease()
	time.Sleep(srv.config.GracePeriod + 30*time.Millisecond)
	if releases := store.releases(); len(releases) != 0 {
		t.Errorf("expected no releases with empty sessionID, got %d", len(releases))
	}
}

func TestConnection_OversizeFrame_ClosesConnection(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	srv.dispatcher.Register(protocol.MethodConnectionPing, handleConnectionPing)
	srv.dispatcher.Register(protocol.MethodCapabilitiesRegister, func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
		conn.transition(ConnReady)
		return map[string]string{}, nil
	})

	c, clientConn := newPipeConnection(t, srv, false)

	done := make(chan struct{})
	go func() {
		c.Run()
		close(done)
	}()

	// Write more than MaxFrameSize bytes with no newline so the reader
	// returns ErrFrameTooLarge.
	go func() {
		buf := make([]byte, protocol.MaxFrameSize+1024)
		for i := range buf {
			buf[i] = 'a'
		}
		_, _ = clientConn.Write(buf)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not return after oversize frame")
	}
	if c.State() != ConnFailed {
		t.Errorf("after oversize frame, state = %v, want ConnFailed", c.State())
	}
}

func TestConnection_Run_ReadEOFEnds(t *testing.T) {
	// When the harness closes the connection cleanly, Run() must return.
	srv := newTestServer(t, &recordingStore{})
	srv.dispatcher.Register(protocol.MethodConnectionPing, handleConnectionPing)

	c, clientConn := newPipeConnection(t, srv, false)

	done := make(chan struct{})
	go func() {
		c.Run()
		close(done)
	}()

	// Close the client side immediately — server-side ReadMessage returns EOF.
	_ = clientConn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not return after client EOF")
	}
}

func TestHandleConnectionPing_ReturnsPong(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	c, _ := newPipeConnection(t, srv, false)

	out, err := handleConnectionPing(context.Background(), c, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handleConnectionPing: %v", err)
	}
	if _, ok := out.(*protocol.ConnectionPong); !ok {
		t.Errorf("expected *protocol.ConnectionPong, got %T", out)
	}
}

// TestConnection_GracePeriod_TimingMath documents the 40s total budget
// referenced in design D4.7: HeartbeatTimeout (30s) + GracePeriod (10s).
// The defaults live on ServerConfig — pin them so the design contract is
// not silently weakened.
func TestConnection_GracePeriod_TimingMath(t *testing.T) {
	cfg := DefaultServerConfig()
	if cfg.HeartbeatTimeout != 30*time.Second {
		t.Errorf("default HeartbeatTimeout = %v, want 30s", cfg.HeartbeatTimeout)
	}
	if cfg.GracePeriod != 10*time.Second {
		t.Errorf("default GracePeriod = %v, want 10s", cfg.GracePeriod)
	}
	if cfg.HeartbeatInterval != 15*time.Second {
		t.Errorf("default HeartbeatInterval = %v, want 15s", cfg.HeartbeatInterval)
	}
	total := cfg.HeartbeatTimeout + cfg.GracePeriod
	if total != 40*time.Second {
		t.Errorf("total timeout budget = %v, want 40s", total)
	}
}

// TestConnection_Run_DispatchesPing exercises the full read loop: send a
// ping from the client, expect the handler to fire and a pong frame to
// arrive back on the wire.
func TestConnection_Run_DispatchesPing(t *testing.T) {
	srv := newTestServer(t, &recordingStore{})
	var pingCount int32
	srv.dispatcher.Register(protocol.MethodConnectionPing, func(ctx context.Context, conn *Connection, params json.RawMessage) (any, error) {
		atomic.AddInt32(&pingCount, 1)
		return &protocol.ConnectionPong{}, nil
	})

	c, clientConn := newPipeConnection(t, srv, false)
	c.setStateForTest(ConnReady)

	done := make(chan struct{})
	go func() {
		c.Run()
		close(done)
	}()

	// Send a ping request frame.
	pingReq := &protocol.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  protocol.MethodConnectionPing,
	}
	raw, err := json.Marshal(pingReq)
	if err != nil {
		t.Fatalf("marshal ping: %v", err)
	}
	fw := protocol.NewFrameWriter(clientConn)
	if err := fw.WriteFrame(raw); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	// Read the pong response back.
	fr := protocol.NewFrameReader(clientConn)
	respBytes, err := fr.ReadFrame()
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	var resp protocol.Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("unmarshal pong: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("pong returned error: %+v", resp.Error)
	}
	if string(resp.ID) != "1" {
		t.Errorf("pong id = %s, want 1", resp.ID)
	}

	_ = clientConn.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not return after EOF")
	}
	if atomic.LoadInt32(&pingCount) == 0 {
		t.Errorf("ping handler was never invoked")
	}
}
