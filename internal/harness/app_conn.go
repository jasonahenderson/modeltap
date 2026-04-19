package harness

import (
	"context"
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// ConnSurface is the narrow App-facing view of a ConnectionManager.
// The App depends on this interface rather than *ConnectionManager so
// tests can inject a fake and so the manager's internals (heartbeat,
// reconnect loop, event bridge) stay out of the App's type graph.
//
// *ConnectionManager satisfies ConnSurface; the compile-time
// assertion lives at the bottom of this file.
type ConnSurface interface {
	// State returns the current connection state (one of the
	// ConnState* constants).
	State() string

	// Reconnect cancels any in-flight retry loop and triggers an
	// immediate reconnect attempt, returning the tea.Cmd that runs it.
	Reconnect() tea.Cmd

	// Client returns the active ProtocolClient (as a narrow
	// interface), or nil when the manager has no live connection.
	// Callers must null-check before every use.
	Client() ConnProtocolClient
}

// ConnProtocolClient is the narrow App-facing view of a ProtocolClient.
// Covers the subset of RPC methods the App invokes directly;
// streaming / tool-call / notification traffic flows through the
// ConnectionManager's event bridge and doesn't touch this interface.
type ConnProtocolClient interface {
	// ContentTransform calls content.transform. Used by WU-083 paste
	// summarize and by future commands that stage large content
	// server-side (WU-062).
	ContentTransform(ctx context.Context, req *protocol.ContentTransform) (json.RawMessage, error)

	// SubmitTurn calls turn.submit. Streaming events arrive as
	// separate notifications the ConnectionManager translates into
	// StreamTokenMsg / StreamCompleteMsg.
	SubmitTurn(ctx context.Context, submit *protocol.TurnSubmit) (*TurnSubmitAck, error)

	// HistoryList calls history.list. Used by WU-092 HistoryController
	// to populate cross-session command history for arrow-up
	// traversal.
	HistoryList(ctx context.Context, req *protocol.HistoryList) (*protocol.HistoryListResponse, error)
}

// connAdapter wraps *ConnectionManager so its Client() returns
// ConnProtocolClient (an interface) rather than the concrete
// *ProtocolClient. Unwrapping here keeps the App free of concrete
// type dependencies while still letting the manager return a real
// *ProtocolClient at the top of the stack.
type connAdapter struct {
	cm *ConnectionManager
}

// WrapConnectionManager returns a ConnSurface backed by cm. Passing
// nil returns nil so callers can wire an optional manager.
func WrapConnectionManager(cm *ConnectionManager) ConnSurface {
	if cm == nil {
		return nil
	}
	return &connAdapter{cm: cm}
}

func (a *connAdapter) State() string      { return a.cm.State() }
func (a *connAdapter) Reconnect() tea.Cmd { return a.cm.Reconnect() }

func (a *connAdapter) Client() ConnProtocolClient {
	c := a.cm.Client()
	if c == nil {
		return nil
	}
	return c
}

// Compile-time proofs that the real types satisfy the interfaces
// App code depends on.
var (
	_ ConnProtocolClient = (*ProtocolClient)(nil)
)
