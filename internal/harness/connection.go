package harness

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// ProgramSender is the slice of *tea.Program that ConnectionManager
// uses to deliver Bubbletea messages. *tea.Program satisfies it
// directly; tests inject a recording fake.
type ProgramSender interface {
	Send(tea.Msg)
}

// Default timing knobs per Bundle 6 design D3.1.
const (
	DefaultStartTimeout        = 5 * time.Second
	DefaultHeartbeatInterval   = 15 * time.Second
	DefaultHeartbeatTimeout    = 30 * time.Second
	DefaultReconnectInitial    = 1 * time.Second
	DefaultReconnectMax        = 30 * time.Second
	DefaultReconnectMaxRetries = 10

	// Heartbeat degradation thresholds per FEAT-0008.
	missedPongsDegraded     = 3
	missedPongsReconnecting = 5

	// Auto-start polling cadence.
	autoStartPollInterval = 200 * time.Millisecond
)

// ConnectionConfig controls a ConnectionManager.
type ConnectionConfig struct {
	// SocketPath / TLSAddress: exactly one must be set.
	SocketPath string
	TLSAddress string
	TLSConfig  *tls.Config

	AutoStart    bool
	ServerBinary string
	ServerArgs   []string
	StartTimeout time.Duration

	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration

	ReconnectInitial    time.Duration
	ReconnectMax        time.Duration
	ReconnectMaxRetries int

	Registration *protocol.CapabilitiesRegister

	// startServerFn lets tests substitute the auto-start subprocess.
	// Production callers leave it nil so the default real-exec path is used.
	startServerFn func(ctx context.Context, cfg ConnectionConfig) error
}

// applyDefaults fills zero-valued fields with sensible defaults.
func (c *ConnectionConfig) applyDefaults() {
	if c.StartTimeout == 0 {
		c.StartTimeout = DefaultStartTimeout
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if c.HeartbeatTimeout == 0 {
		c.HeartbeatTimeout = DefaultHeartbeatTimeout
	}
	if c.ReconnectInitial == 0 {
		c.ReconnectInitial = DefaultReconnectInitial
	}
	if c.ReconnectMax == 0 {
		c.ReconnectMax = DefaultReconnectMax
	}
	if c.ReconnectMaxRetries == 0 {
		c.ReconnectMaxRetries = DefaultReconnectMaxRetries
	}
}

// validHarnessTransitions defines the harness-side state transition
// graph per design D3.2.
var validHarnessTransitions = map[string][]string{
	ConnStateDiscovering:    {ConnStateStarting, ConnStateConnecting, ConnStateFailed},
	ConnStateStarting:       {ConnStateConnecting, ConnStateFailed},
	ConnStateConnecting:     {ConnStateAuthenticating, ConnStateRegistering, ConnStateFailed, ConnStateReconnecting},
	ConnStateAuthenticating: {ConnStateRegistering, ConnStateFailed, ConnStateReconnecting},
	ConnStateRegistering:    {ConnStateReady, ConnStateFailed, ConnStateReconnecting},
	ConnStateReady:          {ConnStateDegraded, ConnStateReconnecting, ConnStateFailed},
	ConnStateDegraded:       {ConnStateReady, ConnStateReconnecting, ConnStateFailed},
	ConnStateReconnecting:   {ConnStateConnecting, ConnStateFailed},
}

// ConnectionManager owns the harness-to-server connection lifecycle:
// discovery, optional auto-start, dial, register, heartbeat,
// disconnect detection, exponential-backoff reconnect, and the event
// bridge that turns server notifications into Bubbletea messages.
type ConnectionManager struct {
	config ConnectionConfig
	sender ProgramSender

	mu     sync.RWMutex
	state  string
	client *ProtocolClient

	// missedPongs tracks consecutive heartbeat failures. Reset on a
	// successful pong. 3 → degraded, 5 → reconnecting.
	missedPongs int

	reconnectAttempt int

	heartbeatCancel context.CancelFunc
	heartbeatDone   chan struct{}

	loopCancel context.CancelFunc
	loopDone   chan struct{}

	closed atomic.Bool
}

// NewConnectionManager constructs a manager rooted at config and
// sender. sender may be nil — useful for tests that don't care about
// program-side Send delivery.
func NewConnectionManager(config ConnectionConfig, sender ProgramSender) *ConnectionManager {
	config.applyDefaults()
	return &ConnectionManager{
		config: config,
		sender: sender,
		state:  ConnStateDiscovering,
	}
}

// State returns the current connection state.
func (cm *ConnectionManager) State() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.state
}

// Client returns the active ProtocolClient, or nil when not connected.
func (cm *ConnectionManager) Client() *ProtocolClient {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.client
}

// Connect drives the lifecycle to ConnStateReady (or Failed) once. It
// is non-blocking: returns a tea.Cmd that runs the connect goroutine
// when Bubbletea executes it. Tests may call ConnectSync to drive the
// flow inline.
func (cm *ConnectionManager) Connect() tea.Cmd {
	return func() tea.Msg {
		_ = cm.ConnectSync(context.Background())
		return nil
	}
}

// ConnectSync runs the discover → connect → register flow inline.
// Returns the final transition error (or nil on Ready). Useful from
// tests; production code calls Connect via a tea.Cmd.
func (cm *ConnectionManager) ConnectSync(ctx context.Context) error {
	if cm.closed.Load() {
		return errors.New("connection manager closed")
	}
	cm.transition(ConnStateDiscovering, "checking server")

	// Discovering: probe the socket. If absent or unreachable and
	// AutoStart is enabled, transition through Starting.
	needsStart := false
	if cm.config.SocketPath != "" {
		if _, err := os.Stat(cm.config.SocketPath); err != nil {
			needsStart = true
		} else {
			// Socket exists; try a quick dial to confirm a listener.
			probeCtx, probeCancel := context.WithTimeout(ctx, 200*time.Millisecond)
			var d net.Dialer
			c, derr := d.DialContext(probeCtx, "unix", cm.config.SocketPath)
			probeCancel()
			if derr != nil {
				needsStart = true
			} else {
				_ = c.Close()
			}
		}
	}

	if needsStart {
		if !cm.config.AutoStart {
			return cm.fail("server not running and AutoStart disabled")
		}
		cm.transition(ConnStateStarting, "auto-starting modeltap server")
		startCtx, startCancel := context.WithTimeout(ctx, cm.config.StartTimeout)
		err := cm.autoStartServer(startCtx)
		startCancel()
		if err != nil {
			return cm.fail("auto-start: " + err.Error())
		}
	}

	cm.transition(ConnStateConnecting, "dialing")
	client, err := Dial(ctx, DialOptions{
		SocketPath:   cm.config.SocketPath,
		TLSAddress:   cm.config.TLSAddress,
		TLSConfig:    cm.config.TLSConfig,
		EventHandler: cm,
		DialTimeout:  cm.config.StartTimeout,
	})
	if err != nil {
		return cm.fail("dial: " + err.Error())
	}
	cm.mu.Lock()
	cm.client = client
	cm.mu.Unlock()

	if cm.config.TLSAddress != "" {
		// TLS connections traverse the auth state. We don't yet have
		// OIDC; the transition is immediate.
		cm.transition(ConnStateAuthenticating, "tls handshake done")
	}

	cm.transition(ConnStateRegistering, "registering capabilities")
	if cm.config.Registration != nil {
		registerCtx, registerCancel := context.WithTimeout(ctx, cm.config.StartTimeout)
		_, err := client.Register(registerCtx, cm.config.Registration)
		registerCancel()
		if err != nil {
			if IsRPCError(err, -32002) { // CodeVersionMismatch
				return cm.fail("version mismatch: " + err.Error())
			}
			return cm.fail("register: " + err.Error())
		}
	}

	cm.transition(ConnStateReady, "")
	cm.reconnectAttempt = 0
	cm.missedPongs = 0
	cm.startHeartbeat()
	cm.startDisconnectWatcher()
	return nil
}

// fail records a terminal failure and returns the wrapped error.
func (cm *ConnectionManager) fail(detail string) error {
	cm.transition(ConnStateFailed, detail)
	return errors.New(detail)
}

// transition validates and applies a state change, then notifies the
// program. Invalid transitions are silently ignored to keep the
// runtime resilient (the state machine is already best-effort).
func (cm *ConnectionManager) transition(to, detail string) {
	cm.mu.Lock()
	from := cm.state
	if !canHarnessTransition(from, to) && from != to {
		cm.mu.Unlock()
		return
	}
	cm.state = to
	cm.mu.Unlock()

	if cm.sender != nil {
		cm.sender.Send(ConnStateMsg{
			Info: ConnStateInfo{
				State:      to,
				Detail:     detail,
				Attempt:    cm.reconnectAttempt,
				MaxRetries: cm.config.ReconnectMaxRetries,
			},
		})
	}
}

func canHarnessTransition(from, to string) bool {
	allowed, ok := validHarnessTransitions[from]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}

// Disconnect stops heartbeats, closes the client, and marks the
// manager closed so subsequent Connects fail.
func (cm *ConnectionManager) Disconnect() {
	if cm.closed.Swap(true) {
		return
	}
	cm.stopHeartbeat()
	if cm.loopCancel != nil {
		cm.loopCancel()
	}
	cm.mu.Lock()
	client := cm.client
	cm.client = nil
	cm.mu.Unlock()
	if client != nil {
		_ = client.Close()
	}
}

// Reconnect cancels any in-flight retry loop and forces an immediate
// reconnect attempt. Returns a tea.Cmd that runs the attempt
// asynchronously.
func (cm *ConnectionManager) Reconnect() tea.Cmd {
	return func() tea.Msg {
		cm.stopHeartbeat()
		cm.mu.Lock()
		client := cm.client
		cm.client = nil
		cm.mu.Unlock()
		if client != nil {
			_ = client.Close()
		}
		cm.reconnectAttempt = 0
		_ = cm.ConnectSync(context.Background())
		return nil
	}
}

// -----------------------------------------------------------------------
// Heartbeat
// -----------------------------------------------------------------------

func (cm *ConnectionManager) startHeartbeat() {
	cm.mu.Lock()
	if cm.heartbeatCancel != nil {
		cm.mu.Unlock()
		return
	}
	hbCtx, hbCancel := context.WithCancel(context.Background())
	cm.heartbeatCancel = hbCancel
	cm.heartbeatDone = make(chan struct{})
	interval := cm.config.HeartbeatInterval
	timeout := cm.config.HeartbeatTimeout
	client := cm.client
	cm.mu.Unlock()

	go func() {
		defer close(cm.heartbeatDone)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				cm.sendHeartbeat(hbCtx, client, timeout)
			}
		}
	}()
}

func (cm *ConnectionManager) stopHeartbeat() {
	cm.mu.Lock()
	cancel := cm.heartbeatCancel
	cm.heartbeatCancel = nil
	cm.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (cm *ConnectionManager) sendHeartbeat(ctx context.Context, client *ProtocolClient, timeout time.Duration) {
	pingCtx, pingCancel := context.WithTimeout(ctx, timeout)
	defer pingCancel()
	err := client.Ping(pingCtx)
	cm.mu.Lock()
	if err != nil {
		cm.missedPongs++
	} else {
		cm.missedPongs = 0
	}
	missed := cm.missedPongs
	state := cm.state
	cm.mu.Unlock()

	switch {
	case missed >= missedPongsReconnecting:
		if state != ConnStateReconnecting {
			cm.transition(ConnStateReconnecting,
				fmt.Sprintf("heartbeat lost (%d missed)", missed))
			cm.stopHeartbeat()
			go cm.reconnectLoop(context.Background())
		}
	case missed >= missedPongsDegraded:
		if state == ConnStateReady {
			cm.transition(ConnStateDegraded,
				fmt.Sprintf("missed %d heartbeats", missed))
		}
	default:
		if state == ConnStateDegraded {
			cm.transition(ConnStateReady, "heartbeat recovered")
		}
	}
}

// startDisconnectWatcher monitors the client's Done channel; on EOF /
// network error from the read loop, transitions immediately into
// Reconnecting and starts the backoff loop.
func (cm *ConnectionManager) startDisconnectWatcher() {
	cm.mu.RLock()
	client := cm.client
	cm.mu.RUnlock()
	if client == nil {
		return
	}
	loopCtx, loopCancel := context.WithCancel(context.Background())
	cm.loopCancel = loopCancel
	cm.loopDone = make(chan struct{})

	go func() {
		defer close(cm.loopDone)
		select {
		case <-loopCtx.Done():
			return
		case <-client.Done():
		}
		if cm.closed.Load() {
			return
		}
		cm.stopHeartbeat()
		cm.transition(ConnStateReconnecting, "connection closed")
		cm.reconnectLoop(loopCtx)
	}()
}

// -----------------------------------------------------------------------
// Reconnection
// -----------------------------------------------------------------------

func (cm *ConnectionManager) reconnectLoop(ctx context.Context) {
	for cm.reconnectAttempt = 1; cm.reconnectAttempt <= cm.config.ReconnectMaxRetries; cm.reconnectAttempt++ {
		if cm.closed.Load() {
			return
		}
		delay := backoffDelay(cm.reconnectAttempt-1, cm.config.ReconnectInitial, cm.config.ReconnectMax)
		cm.transition(ConnStateReconnecting,
			fmt.Sprintf("attempt %d/%d in %s", cm.reconnectAttempt, cm.config.ReconnectMaxRetries, delay))

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		if err := cm.ConnectSync(ctx); err == nil {
			return
		}
	}
	cm.transition(ConnStateFailed, "reconnection retries exhausted")
}

// backoffDelay returns the next delay using exponential growth with
// ±20% jitter, capped at max.
func backoffDelay(attempt int, initial, max time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := initial * time.Duration(1<<uint(min(attempt, 30)))
	if delay > max || delay <= 0 {
		delay = max
	}
	jitterMax := int64(delay) / 5
	if jitterMax <= 0 {
		return delay
	}
	jitter := rand.Int64N(jitterMax*2) - jitterMax
	return delay + time.Duration(jitter)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// -----------------------------------------------------------------------
// Auto-start
// -----------------------------------------------------------------------

func (cm *ConnectionManager) autoStartServer(ctx context.Context) error {
	if cm.config.startServerFn != nil {
		return cm.config.startServerFn(ctx, cm.config)
	}
	return defaultStartServer(ctx, cm.config)
}

// defaultStartServer launches `modeltap start` (or the configured
// binary) as a detached subprocess and polls for the socket. The
// process detaches via Setpgid so the harness exit doesn't kill the
// server.
func defaultStartServer(ctx context.Context, cfg ConnectionConfig) error {
	if cfg.ServerBinary == "" {
		return errors.New("ServerBinary is empty (no auto-start binary configured)")
	}
	args := cfg.ServerArgs
	if len(args) == 0 {
		args = []string{"start"}
	}
	cmd := exec.Command(cfg.ServerBinary, args...)
	// Detach: server outlives harness.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch %s: %w", cfg.ServerBinary, err)
	}
	// Don't wait — let the OS reap when the process eventually exits.
	go func() { _ = cmd.Wait() }()

	return waitForSocket(ctx, cfg.SocketPath)
}

// waitForSocket polls the socket path until it accepts connections or
// the context expires.
func waitForSocket(ctx context.Context, socketPath string) error {
	ticker := time.NewTicker(autoStartPollInterval)
	defer ticker.Stop()
	var d net.Dialer
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for socket %s: %w", socketPath, ctx.Err())
		case <-ticker.C:
			probeCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			conn, err := d.DialContext(probeCtx, "unix", socketPath)
			cancel()
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

// -----------------------------------------------------------------------
// Event bridge (HandleEvent)
// -----------------------------------------------------------------------

// HandleEvent translates server notifications into Bubbletea messages.
// Unknown methods are dropped (the server may emit events the harness
// doesn't yet care about).
func (cm *ConnectionManager) HandleEvent(method string, params json.RawMessage) {
	if cm.sender == nil {
		return
	}
	switch method {
	case protocol.EventTokenDelta:
		var ev protocol.TokenDelta
		if err := json.Unmarshal(params, &ev); err == nil {
			cm.sender.Send(StreamTokenMsg{
				TurnID: ev.TurnID, BranchID: ev.BranchID, Delta: ev.Text,
			})
		}
	case protocol.EventTurnComplete:
		var ev protocol.TurnComplete
		if err := json.Unmarshal(params, &ev); err == nil {
			cm.sender.Send(StreamCompleteMsg{
				TurnID:   ev.TurnID,
				Tokens:   TokenInfo{Input: ev.FinalInputTokens, Output: ev.FinalOutputTokens},
				Cost:     ev.TotalCost,
				Model:    ev.Model,
				Duration: time.Duration(ev.LatencyMs) * time.Millisecond,
			})
		}
	case protocol.EventToolCall:
		var ev protocol.ToolCall
		if err := json.Unmarshal(params, &ev); err == nil {
			cm.sender.Send(ToolCallMsg{
				TurnID:     ev.TurnID,
				ToolCallID: ev.ToolCallID,
				ToolName:   ev.Tool,
				Namespace:  ev.Namespace,
				Input:      ev.Input,
			})
		}
	case protocol.EventCostUpdate:
		var ev protocol.CostUpdate
		if err := json.Unmarshal(params, &ev); err == nil {
			cm.sender.Send(CostUpdateMsg{Total: ev.TotalCost})
		}
	case protocol.EventModelSelected:
		var ev protocol.ModelSelected
		if err := json.Unmarshal(params, &ev); err == nil {
			// Model is either a bare string (single-model turn) or a
			// JSON array (multi-model). For single-model, hand the
			// name to the status bar via ModelUpdateMsg. For
			// multi-model we join the names so the user sees all
			// branches at a glance; branch-started / branch-complete
			// events drive the richer per-branch rendering.
			if !ev.IsMulti() {
				name, _, err := ev.SingleModel()
				if err == nil {
					cm.sender.Send(ModelUpdateMsg{Name: name, Routing: ev.Reason})
				}
			} else if models, _, err := ev.MultiModels(); err == nil {
				cm.sender.Send(ModelUpdateMsg{
					Name:    strings.Join(models, ", "),
					Routing: ev.Reason,
				})
			}
		}
	case protocol.EventBranchStarted:
		var ev protocol.BranchStarted
		if err := json.Unmarshal(params, &ev); err == nil {
			cm.sender.Send(BranchStartedMsg{
				TurnID: ev.TurnID, BranchID: ev.BranchID, Model: ev.Model, Provider: ev.Provider,
			})
		}
	case protocol.EventBranchComplete:
		var ev protocol.BranchComplete
		if err := json.Unmarshal(params, &ev); err == nil {
			cm.sender.Send(BranchCompleteMsg{
				TurnID: ev.TurnID, BranchID: ev.BranchID, Model: ev.Model,
				InputTokens: int(ev.FinalInputTokens), OutputTokens: int(ev.FinalOutputTokens),
			})
		}
	case protocol.EventBranchError:
		var ev protocol.BranchError
		if err := json.Unmarshal(params, &ev); err == nil {
			cm.sender.Send(BranchErrorMsg{
				TurnID: ev.TurnID, BranchID: ev.BranchID, Error: ev.Message,
			})
		}
	case protocol.EventCompactSuggest:
		var ev protocol.CompactSuggest
		text := "Context pressure — consider /compact"
		if err := json.Unmarshal(params, &ev); err == nil {
			if ev.Message != "" {
				text = ev.Message
			} else if ev.ContextPct > 0 {
				text = fmt.Sprintf("Context at %.0f%% — consider /compact", ev.ContextPct*100)
			}
		}
		cm.sender.Send(BannerMsg{Text: text, Duration: 8 * time.Second})

	case protocol.EventCompactNotice:
		var ev protocol.CompactNotice
		text := "Context auto-compacted"
		if err := json.Unmarshal(params, &ev); err == nil {
			if ev.Summary != "" {
				text = ev.Summary
			} else if ev.TokensFreed > 0 {
				text = fmt.Sprintf("Auto-compacted: freed %d tokens", ev.TokensFreed)
			}
		}
		cm.sender.Send(BannerMsg{Text: text, Duration: 5 * time.Second})

	case protocol.EventCompactPlan:
		// Compaction plan UI is WU-061 territory (still in design);
		// surface the event as a transient banner so the user at
		// least knows a plan arrived.
		cm.sender.Send(BannerMsg{Text: "Compaction plan available — /compact to apply", Duration: 8 * time.Second})

	case protocol.EventKnowledgeHit:
		var ev protocol.KnowledgeHit
		if err := json.Unmarshal(params, &ev); err == nil && ev.Summary != "" {
			cm.sender.Send(BannerMsg{
				Text:     "Knowledge: " + ev.Summary,
				Duration: 4 * time.Second,
			})
		}

	case protocol.EventError:
		var ev protocol.ServerError
		if err := json.Unmarshal(params, &ev); err == nil {
			text := ev.Message
			if ev.Diagnostic.Code != "" {
				text = string(ev.Diagnostic.Code) + ": " + text
			}
			if text == "" {
				text = "Server error"
			}
			cm.sender.Send(BannerMsg{Text: text, Duration: 8 * time.Second})
		}

	case protocol.EventStatusUpdate:
		var ev protocol.StatusUpdate
		if err := json.Unmarshal(params, &ev); err == nil {
			cm.sender.Send(StatusUpdateMsg{TurnID: ev.TurnID, Message: ev.Detail})
		}
	case protocol.EventCapabilitiesRequest:
		// Server is asking us to re-register capabilities; a future
		// task can wire this into a re-register flow.
	case protocol.EventConnectionPong:
		// Heartbeat responses flow through the RPC client's reply
		// path, not this event bridge. Explicit no-op so unknown-
		// method falls through cleanly to nothing (the default
		// branch is intentionally absent).
	}
}
