package bff

import (
	"os"
	"sync"
	"time"

	"github.com/jasonahenderson/modeltap/internal/storage"
)

// Server manages BFF protocol listeners and accepts connections.
//
// File scope (WU-048 stub): just enough to back the Connection state
// machine — store, dispatcher, config, startTime, and a connection map.
// WU-047 fleshes out listener management, accept loop, graceful shutdown,
// and the `modeltap serve` integration.
type Server struct {
	store      storage.Store
	dispatcher *Dispatcher
	config     ServerConfig
	startTime  time.Time

	mu    sync.Mutex
	conns map[*Connection]struct{}
}

// ServerConfig configures the BFF server. Defaults are returned by
// DefaultServerConfig — tests use small intervals so heartbeat / grace
// timers fire in milliseconds.
type ServerConfig struct {
	// Unix socket path (solo/local profile).
	SocketPath string
	// Socket file mode (default 0600).
	SocketMode os.FileMode

	// TLS endpoint (team/enterprise profile).
	TLSAddress  string
	TLSCertFile string
	TLSKeyFile  string

	// Connection limits.
	MaxConnections int

	// Heartbeat / grace timing per FEAT-0008 §"Heartbeat".
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
	GracePeriod       time.Duration
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
	}
}
