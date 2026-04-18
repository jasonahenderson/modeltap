package bff

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// ConnState is the lifecycle state of a single harness↔server connection.
// Wire-visible names are returned by ConnState.String() and pinned by
// TestConnState_StringValues.
type ConnState int

const (
	// ConnDiscovering is harness-side only — the harness searches for an
	// existing server. The server never enters this state.
	ConnDiscovering ConnState = iota

	// ConnStarting is harness-side only — the harness auto-launches the
	// server. The server never enters this state.
	ConnStarting

	// ConnConnecting: TCP/socket established, no protocol exchange yet.
	ConnConnecting

	// ConnAuthenticating: TLS handshake done, OIDC/auth in progress.
	// Skipped for unix-socket connections.
	ConnAuthenticating

	// ConnRegistering: capabilities.register received, being processed.
	ConnRegistering

	// ConnReady: fully operational. Requests accepted.
	ConnReady

	// ConnDegraded: operational but a dependency is unhealthy. Requests
	// still accepted; heartbeat continues.
	ConnDegraded

	// ConnReconnecting is harness-side only — heartbeat lost. The server
	// sees a dropped connection and never enters this state.
	ConnReconnecting

	// ConnFailed: terminal. The connection will be closed.
	ConnFailed
)

// String returns the FEAT-0008 canonical state name. These names appear
// on the wire (HealthResponse.dependencies, Diagnostic.cause, session.sync
// payloads); changing them is a wire-breaking change.
func (s ConnState) String() string {
	switch s {
	case ConnDiscovering:
		return "discovering"
	case ConnStarting:
		return "starting"
	case ConnConnecting:
		return "connecting"
	case ConnAuthenticating:
		return "authenticating"
	case ConnRegistering:
		return "registering"
	case ConnReady:
		return "ready"
	case ConnDegraded:
		return "degraded"
	case ConnReconnecting:
		return "reconnecting"
	case ConnFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// validTransitions defines the allowed state machine transitions per
// design D4.3. The server enforces these — transition() returns false
// for any unlisted (from, to) pair.
var validTransitions = map[ConnState][]ConnState{
	ConnConnecting:     {ConnAuthenticating, ConnRegistering, ConnFailed},
	ConnAuthenticating: {ConnRegistering, ConnFailed},
	ConnRegistering:    {ConnReady, ConnFailed},
	ConnReady:          {ConnDegraded, ConnFailed},
	ConnDegraded:       {ConnReady, ConnFailed},
}

// Connection represents a single harness-to-server protocol session.
// The read goroutine (Run) is the only reader; writes are serialized by
// the FrameTransport. State changes are guarded by mu.
type Connection struct {
	id           string
	transport    *FrameTransport
	server       *Server
	requiresAuth bool

	mu      sync.RWMutex
	state   ConnState
	visited map[ConnState]bool

	sessionID string

	// Capabilities — populated on capabilities.register (WU-049).
	capabilities *CapabilityManager

	// Heartbeat. lastPing is initialized to creation time so the monitor
	// does not flag a freshly-created connection as timed-out before the
	// harness has a chance to send its first ping (design D4.5).
	lastPing time.Time

	heartbeatCancel context.CancelFunc
	heartbeatDone   chan struct{}

	// Grace-period release timer for the bound session lock (design D4.7).
	graceCancel context.CancelFunc

	// Lifecycle.
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// NewConnection constructs a Connection in state ConnConnecting. If
// requiresAuth is true (TLS listener accepted the conn), initialize()
// transitions through ConnAuthenticating before ConnRegistering;
// otherwise it goes straight to ConnRegistering (design D4.1, unix
// socket skip-auth).
func NewConnection(id string, transport *FrameTransport, server *Server, requiresAuth bool) *Connection {
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	c := &Connection{
		id:           id,
		transport:    transport,
		server:       server,
		requiresAuth: requiresAuth,
		state:        ConnConnecting,
		visited:      map[ConnState]bool{ConnConnecting: true},
		lastPing:     now,
		capabilities: NewCapabilityManager(),
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
	}
	return c
}

// Capabilities returns the per-connection capability manager. Populated
// on capabilities.register and refreshed on session.resume.
func (c *Connection) Capabilities() *CapabilityManager {
	return c.capabilities
}

// ID returns the connection's unique identifier.
func (c *Connection) ID() string { return c.id }

// SoloUserID is the placeholder user identifier used for the solo
// profile (no auth). Auth WUs will add a real UserID() that draws from
// the authenticated principal; until then handlers scope data by this
// sentinel so storage constraints (user_id NOT NULL) are satisfied.
const SoloUserID = "local"

// UserID returns the authenticated user identifier for this connection.
// Currently always returns SoloUserID; will be extended by the auth WU.
func (c *Connection) UserID() string { return SoloUserID }

// State returns the current connection state.
func (c *Connection) State() ConnState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SessionID returns the bound session ID, or "" if no session is active.
func (c *Connection) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// SetSessionID binds (or unbinds with "") the active session for this
// connection. Called by the session.resume / session.create handlers.
func (c *Connection) SetSessionID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = id
}

// LastPing returns the timestamp of the last harness ping. Used by the
// heartbeat monitor and exposed for tests.
func (c *Connection) LastPing() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastPing
}

// transition atomically changes state. Returns false if the transition
// is not in validTransitions.
func (c *Connection) transition(to ConnState) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.canTransitionLocked(c.state, to) {
		return false
	}
	c.state = to
	c.visited[to] = true
	return true
}

func (c *Connection) canTransitionLocked(from, to ConnState) bool {
	for _, allowed := range validTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// initialize advances the state from ConnConnecting to ConnRegistering,
// passing through ConnAuthenticating only when requiresAuth is true. The
// auth phase itself is a no-op stub (no OIDC implementation yet); future
// auth work will gate the second transition on a successful credential
// exchange.
func (c *Connection) initialize() {
	if c.requiresAuth {
		c.transition(ConnAuthenticating)
	}
	c.transition(ConnRegistering)
}

// startHeartbeatMonitor launches a goroutine that fails the connection
// when the harness stops sending pings. The server is passive — it does
// not initiate pings; it only watches lastPing freshness.
func (c *Connection) startHeartbeatMonitor() {
	c.mu.Lock()
	if c.heartbeatCancel != nil {
		c.mu.Unlock()
		return
	}
	hbCtx, hbCancel := context.WithCancel(c.ctx)
	c.heartbeatCancel = hbCancel
	c.heartbeatDone = make(chan struct{})
	cfg := c.server.config
	c.mu.Unlock()

	go func() {
		defer close(c.heartbeatDone)
		ticker := time.NewTicker(cfg.HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				if time.Since(c.LastPing()) > cfg.HeartbeatTimeout {
					c.transition(ConnFailed)
					_ = c.transport.Close()
					return
				}
			}
		}
	}()
}

// stopHeartbeatMonitor cancels the monitor goroutine. Used by tests and
// by the cleanup path in Run.
func (c *Connection) stopHeartbeatMonitor() {
	c.mu.Lock()
	cancel := c.heartbeatCancel
	c.heartbeatCancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// scheduleGracePeriodRelease starts a timer to release the bound session
// lock after GracePeriod elapses. If no session is bound, the call is a
// no-op. Cancellation is exposed via cancelGracePeriodRelease so that a
// reconnecting harness can rescue the session lock before release fires.
func (c *Connection) scheduleGracePeriodRelease() {
	c.mu.Lock()
	if c.sessionID == "" {
		c.mu.Unlock()
		return
	}
	if c.graceCancel != nil {
		// Already scheduled.
		c.mu.Unlock()
		return
	}
	sessionID := c.sessionID
	owner := c.id
	cfg := c.server.config
	store := c.server.store

	graceCtx, graceCancel := context.WithCancel(context.Background())
	c.graceCancel = graceCancel
	c.mu.Unlock()

	go func() {
		timer := time.NewTimer(cfg.GracePeriod)
		defer timer.Stop()
		select {
		case <-graceCtx.Done():
			return
		case <-timer.C:
			// Best-effort release — store may be unreachable; we cannot
			// surface the error to a caller from a background timer.
			_ = store.ReleaseSessionLock(context.Background(), sessionID, owner)
		}
	}()
}

// cancelGracePeriodRelease aborts a pending grace-period release. Called
// by session.resume when the harness reclaims its session.
func (c *Connection) cancelGracePeriodRelease() {
	c.mu.Lock()
	cancel := c.graceCancel
	c.graceCancel = nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// gateMethodLocked returns nil if a request with the given method name
// may be dispatched in the current state, or a *TransportError with
// CodeNotReady. State must be read under c.mu (caller's responsibility).
func gateMethodLocked(state ConnState, method string) error {
	switch state {
	case ConnReady, ConnDegraded:
		return nil
	case ConnRegistering:
		if method == protocol.MethodCapabilitiesRegister || method == protocol.MethodConnectionPing {
			return nil
		}
	case ConnAuthenticating:
		if method == protocol.MethodConnectionPing {
			return nil
		}
	}
	return &TransportError{
		Code:    CodeNotReady,
		Message: "connection is in state " + state.String() + "; method " + method + " not allowed",
	}
}

// Run is the connection's read loop. It blocks until the underlying
// transport returns EOF, an unrecoverable error, or the connection's
// context is cancelled. On exit, the heartbeat monitor is stopped, the
// transport is closed, and a grace-period session-lock release is
// scheduled if a session was bound.
func (c *Connection) Run() {
	defer close(c.done)
	defer c.cancel()
	defer c.stopHeartbeatMonitor()
	defer func() { _ = c.transport.Close() }()
	defer func() {
		// Best-effort: if we somehow exit without going through ConnFailed,
		// transition there now so external observers see the terminal state.
		c.mu.Lock()
		if c.state != ConnFailed {
			if c.canTransitionLocked(c.state, ConnFailed) {
				c.state = ConnFailed
				c.visited[ConnFailed] = true
			}
		}
		c.mu.Unlock()
		c.scheduleGracePeriodRelease()
	}()

	c.initialize()

	for {
		env, err := c.transport.ReadMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			var te *TransportError
			if errors.As(err, &te) {
				if te.Close {
					// Wire is unrecoverable (e.g., oversize frame leaves
					// the reader mid-frame). Skip the best-effort error
					// response so we don't block on a peer that may not
					// be reading, and tear down.
					return
				}
				_ = c.transport.SendError(nil, te.Code, te.Message, nil)
				continue
			}
			// Network error or unexpected parse failure — terminate.
			return
		}

		switch {
		case env.Request != nil:
			c.handleRequest(env.Request)
		case env.Notification != nil:
			// Harness-originating notifications are not part of the
			// FEAT-0008 surface; ignore silently rather than tear down.
		case env.Response != nil:
			// Harness-originating responses are unexpected on the server
			// (the server sends notifications, not requests). Drop.
		}
	}
}

// handleRequest applies the dispatch gate, runs the handler, and writes
// the response or error frame.
func (c *Connection) handleRequest(req *protocol.Request) {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	if err := gateMethodLocked(state, req.Method); err != nil {
		var te *TransportError
		_ = errors.As(err, &te)
		_ = c.transport.SendError(req.ID, te.Code, te.Message, nil)
		return
	}

	result, err := c.server.dispatcher.Dispatch(c.ctx, c, req)
	if err != nil {
		var te *TransportError
		if errors.As(err, &te) {
			_ = c.transport.SendError(req.ID, te.Code, te.Message, nil)
			return
		}
		_ = c.transport.SendError(req.ID, CodeInternalError, err.Error(), nil)
		return
	}

	var raw json.RawMessage
	if result != nil {
		b, mErr := json.Marshal(result)
		if mErr != nil {
			_ = c.transport.SendError(req.ID, CodeInternalError, "marshal result: "+mErr.Error(), nil)
			return
		}
		raw = b
	} else {
		raw = json.RawMessage(`null`)
	}
	_ = c.transport.SendResponse(&protocol.Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  raw,
	})
}

// handleConnectionPing implements the connection.ping handler. It
// updates lastPing and returns an empty ConnectionPong. Registered by
// the server during dispatcher setup (WU-047).
func handleConnectionPing(_ context.Context, conn *Connection, _ json.RawMessage) (any, error) {
	conn.mu.Lock()
	conn.lastPing = time.Now()
	conn.mu.Unlock()
	return &protocol.ConnectionPong{}, nil
}

// -----------------------------------------------------------------------
// Test helpers.
// -----------------------------------------------------------------------
// These are exported only inside the package's test files (lowercase
// receivers; the *_test.go files use them). They are NOT part of the
// public API — no external package imports internal/bff.

func (c *Connection) setStateForTest(s ConnState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = s
	if c.visited == nil {
		c.visited = make(map[ConnState]bool)
	}
	c.visited[s] = true
}

func (c *Connection) setLastPingForTest(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPing = t
}

func (c *Connection) recordPingForTest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPing = time.Now()
}

// dispatchForTest exposes the gating + dispatcher path for unit tests
// without spinning up the full Run loop.
func (c *Connection) dispatchForTest(ctx context.Context, req *protocol.Request) (any, error) {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()
	if err := gateMethodLocked(state, req.Method); err != nil {
		return nil, err
	}
	return c.server.dispatcher.Dispatch(ctx, c, req)
}

// visited reports whether the connection ever held the given state.
func (c *Connection) Visited(s ConnState) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.visited[s]
}
