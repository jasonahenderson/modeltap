package runtime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
)

// ServerVersion is the Runtime server version advertised in
// connection.health responses. It is overridable at link time with
// -ldflags "-X github.com/jasonahenderson/modeltap/internal/runtime.ServerVersion=..."
// and defaults to "dev" for local builds.
var ServerVersion = "dev"

// Server manages Runtime protocol listeners and accepts connections.
//
// One Server owns all listeners (unix socket and/or TLS), the
// Dispatcher, and the set of live Connections. Ownership semantics:
//   - Start() binds listeners and spawns accept loops.
//   - Shutdown() closes listeners, cancels the server context, and
//     waits for active connections to drain.
//   - Connections are created by the accept path (handleConnection)
//     and removed when Run() returns.
type Server struct {
	store      storage.Store
	dispatcher *Dispatcher
	config     ServerConfig
	startTime  time.Time
	sessions   *SessionManager
	providers  *ProviderRegistry
	adapters   *provider.Registry
	models     *ModelRegistry
	routing    *RoutingPolicy
	prompts    *PromptEngine
	dispatch   *TurnDispatcher
	cost       *CostTracker
	turns      *turnTracker
	runs       *runRegistry

	mu    sync.Mutex
	conns map[*Connection]struct{}

	socketListener net.Listener
	tlsListener    net.Listener

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ServerConfig configures the Runtime server. Defaults are returned by
// DefaultServerConfig — tests use small intervals so heartbeat / grace
// timers fire in milliseconds.
type ServerConfig struct {
	// Unix socket path (solo/local profile).
	SocketPath string
	// Socket file mode (default 0600).
	SocketMode os.FileMode

	// TLS endpoint (team/enterprise profile).
	TLSAddress      string
	TLSCertFile     string
	TLSKeyFile      string
	TLSClientCAFile string // WU-094 H-14: required when TLSAddress is set

	// Connection limits.
	MaxConnections int

	// Heartbeat / grace timing per FEAT-0008 §"Heartbeat".
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
	GracePeriod       time.Duration

	// MaxAttachmentSize is advertised in CapabilitiesRegisterResponse.
	// Harness-side attachments larger than this must be rejected before
	// transmission. Default 5 MiB per FEAT-0008 (WU-039 review A-05).
	MaxAttachmentSize int
}

// DefaultServerConfig returns the FEAT-0008 default timing (heartbeat
// 15s / timeout 30s / grace 10s = 40s total). Pinned by
// TestConnection_GracePeriod_TimingMath.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		SocketMode:        0o600,
		MaxConnections:    100,
		HeartbeatInterval: 15 * time.Second,
		HeartbeatTimeout:  30 * time.Second,
		GracePeriod:       10 * time.Second,
		MaxAttachmentSize: 5 * 1024 * 1024,
	}
}

// NewServer constructs a Server with the given storage backend and
// configuration. The dispatcher is initialized and WU-047-owned
// handlers (connection.ping / health / ready) are registered. Other
// method handlers are registered by their owning WUs (capabilities.*
// in WU-049, session.* in WU-050, etc.).
func NewServer(store storage.Store, config ServerConfig) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		store:      store,
		dispatcher: NewDispatcher(),
		config:     config,
		startTime:  time.Now(),
		conns:      make(map[*Connection]struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}
	s.sessions = NewSessionManager(store)
	s.providers = NewProviderRegistry()
	s.adapters = provider.NewRegistry()
	s.models = NewModelRegistry(s.providers)
	s.routing = NewRoutingPolicy()
	s.prompts = NewPromptEngine(PromptEngineOpts{})
	s.dispatch = NewTurnDispatcher(s.providers, s.adapters)
	s.cost = NewCostTracker(s.models, s.store)
	s.turns = newTurnTracker()
	s.runs = newRunRegistry()
	s.registerCoreHandlers()
	s.sessions.Register(s.dispatcher)
	s.dispatcher.Register(protocol.MethodModelList, handleModelList)
	s.dispatcher.Register(protocol.MethodModelSwitch, handleModelSwitch)
	s.dispatcher.Register(protocol.MethodTurnSubmit, handleTurnSubmit)
	s.dispatcher.Register(protocol.MethodTurnCancel, handleTurnCancel)
	s.dispatcher.Register(protocol.MethodToolResult, handleToolResult)
	s.dispatcher.Register(protocol.MethodHistoryAppend, handleHistoryAppend)
	s.dispatcher.Register(protocol.MethodHistoryList, handleHistoryList)
	s.dispatcher.Register(protocol.MethodSessionSync, handleSessionSync)
	s.dispatcher.Register(protocol.MethodContentTransform, handleContentTransform)
	s.dispatcher.Register(protocol.MethodSessionCompact, handleSessionCompact)
	s.dispatcher.Register(protocol.MethodCompactApply, handleCompactApply)
	s.registerRunHandlers()
	return s
}

// Adapters returns the provider-adapter registry. Callers (e.g., CLI
// startup) populate it with the in-tree Anthropic / OpenAI / Ollama
// adapters before serving.
func (s *Server) Adapters() *provider.Registry { return s.adapters }

// Prompts returns the system-prompt engine. Exposed so tests and CLI
// can override the default behavioral text or domain config.
func (s *Server) Prompts() *PromptEngine { return s.prompts }

// Dispatch returns the turn dispatcher. Mainly for tests that want to
// inject a stubbed HTTP client.
func (s *Server) Dispatch() *TurnDispatcher { return s.dispatch }

// Providers returns the server's provider endpoint registry.
func (s *Server) Providers() *ProviderRegistry { return s.providers }

// Models returns the server's model registry.
func (s *Server) Models() *ModelRegistry { return s.models }

// Routing returns the server's routing policy.
func (s *Server) Routing() *RoutingPolicy { return s.routing }

// Sessions returns the server's session manager. Exposed for downstream
// WUs (e.g., WU-064 session.sync) and tests.
func (s *Server) Sessions() *SessionManager { return s.sessions }

// registerCoreHandlers registers the connection-lifecycle and
// capability-handshake handlers the Server owns directly.
// Application handlers (session.*, turn.*, history.*, etc.) are
// registered inline in NewServer; capabilities.register and
// capabilities.update land here because they're prerequisites for
// every other RPC — the server that accepts a connection must
// answer the handshake.
func (s *Server) registerCoreHandlers() {
	s.dispatcher.Register(protocol.MethodConnectionPing, handleConnectionPing)
	s.dispatcher.Register(protocol.MethodConnectionHealth, handleConnectionHealth)
	s.dispatcher.Register(protocol.MethodConnectionReady, handleConnectionReady)
	s.dispatcher.Register(protocol.MethodCapabilitiesRegister, handleCapabilitiesRegister)
	s.dispatcher.Register(protocol.MethodCapabilitiesUpdate, handleCapabilitiesUpdate)
}

// Dispatcher returns the server's request dispatcher so downstream WUs
// can register their handlers (capabilities.*, session.*, turn.*, etc.).
func (s *Server) Dispatcher() *Dispatcher { return s.dispatcher }

// Store returns the server's storage backend. Exposed for handlers and
// for tests.
func (s *Server) Store() storage.Store { return s.store }

// Config returns the server's configuration snapshot.
func (s *Server) Config() ServerConfig { return s.config }

// Start binds all configured listeners and launches their accept loops.
// Returns when listeners are bound (does not block for connections).
// Errors from either listener are returned immediately without partial
// startup — if the socket listener binds but the TLS listener fails,
// the socket listener is closed before Start returns.
func (s *Server) Start() error {
	if s.config.SocketPath != "" {
		ln, err := s.startSocketListener()
		if err != nil {
			return fmt.Errorf("socket listener: %w", err)
		}
		s.socketListener = ln
		s.wg.Add(1)
		go s.acceptLoop(ln, false /* requiresAuth */)
	}

	if s.config.TLSAddress != "" {
		ln, err := s.startTLSListener()
		if err != nil {
			// Roll back the socket listener if it was already bound.
			if s.socketListener != nil {
				_ = s.socketListener.Close()
				s.socketListener = nil
			}
			return fmt.Errorf("tls listener: %w", err)
		}
		s.tlsListener = ln
		s.wg.Add(1)
		go s.acceptLoop(ln, true /* requiresAuth */)
	}

	return nil
}

// Shutdown gracefully stops the server. It closes listeners, cancels
// live connections' contexts, stops the provider health-check poll
// loop, and waits for the accept loops and per-connection Run
// goroutines to return. Returns nil on clean drainage or ctx.Err() if
// the provided context deadline is reached.
func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel() // signal per-connection contexts
	if s.providers != nil {
		s.providers.Stop()
	}
	if s.socketListener != nil {
		_ = s.socketListener.Close()
		// Best-effort: remove the socket file so subsequent Start calls
		// don't see it as stale and trigger the removal path.
		if s.config.SocketPath != "" {
			_ = os.Remove(s.config.SocketPath)
		}
	}
	if s.tlsListener != nil {
		_ = s.tlsListener.Close()
	}

	// Close live transports so Connection.Run goroutines exit their
	// read loops promptly.
	s.mu.Lock()
	for c := range s.conns {
		_ = c.transport.Close()
	}
	s.mu.Unlock()

	doneC := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(doneC)
	}()
	select {
	case <-doneC:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TLSAddr returns the actual address the TLS listener bound to, or ""
// if no TLS listener is active. Useful for ephemeral-port tests.
func (s *Server) TLSAddr() string {
	if s.tlsListener == nil {
		return ""
	}
	return s.tlsListener.Addr().String()
}

// startSocketListener binds the unix-domain socket listener. If the
// configured path already exists, the server attempts to detect whether
// an active listener holds it: net.Dial success -> return error;
// dial failure -> remove the stale file and rebind.
func (s *Server) startSocketListener() (net.Listener, error) {
	path := s.config.SocketPath
	if _, err := os.Stat(path); err == nil {
		// Something exists at the path. Probe whether a listener is active.
		probeCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		var d net.Dialer
		probeConn, dialErr := d.DialContext(probeCtx, "unix", path)
		if dialErr == nil {
			_ = probeConn.Close()
			return nil, fmt.Errorf("another process is listening on %s", path)
		}
		// Stale socket; remove and rebind.
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("removing stale socket: %w", err)
		}
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	mode := s.config.SocketMode
	if mode == 0 {
		mode = 0o600
	}
	if err := os.Chmod(path, mode); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("chmod socket: %w", err)
	}
	return ln, nil
}

// startTLSListener binds the TLS listener using the configured cert
// and key. The cert is loaded once at Start; hot reload is out of
// scope for WU-047.
//
// Refuses to bind without client-CA config (WU-094 H-14): the
// current auth state machine is a no-op stub, so any TLS peer that
// completes the handshake is accepted as SoloUserID with full
// session access. Until the auth WU lands real credential exchange,
// mTLS is the only access control we can offer on the TLS profile.
// Callers who need the TLS listener must supply TLSClientCAFile so
// client certs are required and verified.
func (s *Server) startTLSListener() (net.Listener, error) {
	if s.config.TLSClientCAFile == "" {
		return nil, fmt.Errorf(
			"TLS listener refuses to bind without tls_client_ca_file configured: " +
				"auth is not yet implemented, so any reachable client would be accepted " +
				"as the solo user. Configure mTLS or use the unix socket profile")
	}
	cert, err := tls.LoadX509KeyPair(s.config.TLSCertFile, s.config.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load cert: %w", err)
	}
	caPEM, err := os.ReadFile(s.config.TLSClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("load client CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("client CA file %s contains no parseable certs", s.config.TLSClientCAFile)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
	}
	ln, err := tls.Listen("tcp", s.config.TLSAddress, tlsConfig)
	if err != nil {
		return nil, err
	}
	return ln, nil
}

// acceptLoop runs the accept loop for a single listener until the
// listener is closed or the server context is cancelled.
func (s *Server) acceptLoop(ln net.Listener, requiresAuth bool) {
	defer s.wg.Done()
	for {
		conn, err := ln.Accept()
		if err != nil {
			// ln.Close() produces a net.ErrClosed — treat as clean exit.
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if s.ctx.Err() != nil {
				return
			}
			// Transient accept errors (e.g., per-connection refused):
			// back off briefly and continue.
			time.Sleep(10 * time.Millisecond)
			continue
		}
		// Increment wg here (inside a goroutine that already holds a wg
		// reference) so concurrent Shutdown -> wg.Wait cannot race with
		// Add(1) in the handler goroutine. See sync.WaitGroup docs:
		// Add with positive delta is safe while the counter is known >0.
		s.wg.Add(1)
		go s.handleConnection(conn, requiresAuth)
	}
}

// handleConnection wraps a freshly-accepted net.Conn in a Connection
// and runs its lifecycle. Enforces MaxConnections; over-limit connections
// are closed without protocol reply. The caller (acceptLoop) must have
// already called s.wg.Add(1); this function is responsible for the
// matching Done on every return path.
func (s *Server) handleConnection(netConn net.Conn, requiresAuth bool) {
	defer s.wg.Done()

	s.mu.Lock()
	if s.config.MaxConnections > 0 && len(s.conns) >= s.config.MaxConnections {
		s.mu.Unlock()
		_ = netConn.Close()
		return
	}

	transport := NewFrameTransport(netConn)
	c := NewConnection(uuid.NewString(), transport, s, requiresAuth)
	s.conns[c] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
	}()

	c.Run()
}

// handleConnectionHealth implements the connection.health handler per
// design D4.6. Unimplemented dependencies (auth, providers, routing)
// report as "ready" stubs until their owning WUs land; storage health
// is probed via Store.Ping.
func handleConnectionHealth(ctx context.Context, conn *Connection, _ json.RawMessage) (any, error) {
	srv := conn.server
	hr := &protocol.HealthResponse{
		ServerVersion:   ServerVersion,
		ProtocolVersion: protocol.ProtocolVersion,
		UptimeSeconds:   int(time.Since(srv.startTime).Seconds()),
		Auth:            protocol.DependencyStatus{Status: "ready"},
		Capabilities:    protocol.DependencyStatus{Status: "ready", Method: "register"},
		Providers:       map[string]protocol.ProviderStatus{},
		Routing:         protocol.DependencyStatus{Status: "ready"},
	}

	// Storage probe.
	if err := srv.store.Ping(ctx); err != nil {
		hr.Storage = protocol.DependencyStatus{Status: "unavailable", Reason: err.Error()}
	} else {
		hr.Storage = protocol.DependencyStatus{Status: "ready"}
	}

	// Capabilities dependency status: WU-049 will track registration on
	// the connection; for now derive a stub answer from the connection
	// state. A connection that has reached Ready/Degraded has by
	// definition completed capabilities.register.
	switch conn.State() {
	case ConnReady, ConnDegraded:
		hr.Capabilities = protocol.DependencyStatus{Status: "ready", Method: "register"}
	default:
		hr.Capabilities = protocol.DependencyStatus{Status: "unavailable", Reason: "capabilities.register has not completed"}
	}

	if sid := conn.SessionID(); sid != "" {
		hr.ActiveSession = &protocol.ActiveSessionInfo{
			ID:    sid,
			Owner: conn.ID(),
		}
	}
	return hr, nil
}

// handleConnectionReady implements the connection.ready handler per
// design D4.6. Ready is true only when the connection state is Ready
// (implies registration complete) AND storage is healthy. Provider/auth
// checks are stubbed until WU-057 / auth WU land.
func handleConnectionReady(ctx context.Context, conn *Connection, _ json.RawMessage) (any, error) {
	ready := conn.State() == ConnReady
	if ready {
		if err := conn.server.store.Ping(ctx); err != nil {
			ready = false
		}
	}
	return &protocol.ReadyResponse{Ready: ready}, nil
}

// -----------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------

// forceAllReadyForTest walks every live connection and sets state to
// ConnReady. Used by TestServer_GracefulShutdown to exercise the drain
// path without running a full registration handshake.
func (s *Server) forceAllReadyForTest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.conns {
		c.setStateForTest(ConnReady)
	}
}
