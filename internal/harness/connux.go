package harness

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ConnectionUX translates connection-state transitions into user-
// visible banners. The App owns one of these; on every ConnStateMsg it
// invokes HandleConnState and returns the produced tea.Cmd so the
// banner is updated in the same Update tick as state.ConnState. Per
// design D7 the mapping is deterministic and pure — all mutation goes
// through BannerMsg / BannerClearMsg on the Bubbletea message bus, not
// directly into AppState.
type ConnectionUX struct {
	state *AppState
}

// NewConnectionUX constructs a ConnectionUX bound to the shared
// AppState pointer (used only to read, not mutate, current state).
func NewConnectionUX(state *AppState) *ConnectionUX {
	return &ConnectionUX{state: state}
}

// bannerFlashDuration is how long a transient (progress) banner stays
// on screen before BannerClearMsg fires. Persistent states (degraded,
// failed, reconnecting) set Duration = 0 and rely on the next
// transition to clear.
const bannerFlashDuration = 4 * time.Second

// HandleConnState returns a tea.Cmd that emits the correct
// BannerMsg / BannerClearMsg for the given connection state.
// Callers wire this into the App's Update handler right after the
// state mutation so the banner tracks the ConnState in lockstep.
func (cux *ConnectionUX) HandleConnState(msg ConnStateMsg) tea.Cmd {
	text, persistent := cux.banner(msg.Info)
	if text == "" {
		return func() tea.Msg { return BannerClearMsg{} }
	}
	duration := bannerFlashDuration
	if persistent {
		duration = 0
	}
	out := BannerMsg{Text: text, Duration: duration}
	return func() tea.Msg { return out }
}

// banner returns the banner text plus a persistent flag for a given
// ConnStateInfo. Empty text means "clear any current banner".
func (cux *ConnectionUX) banner(info ConnStateInfo) (string, bool) {
	switch info.State {
	case ConnStateReady:
		return "", false

	case ConnStateDiscovering:
		return "Discovering modeltap server…", false
	case ConnStateStarting:
		return "Starting local modeltap server…", false
	case ConnStateConnecting:
		return "Connecting to modeltap server…", false
	case ConnStateAuthenticating:
		return "Authenticating with the BFF…", false

	case ConnStateRegistering:
		if d := strings.TrimSpace(info.Detail); d != "" {
			return fmt.Sprintf("Registering tools (%s)…", d), false
		}
		return "Registering tools…", false

	case ConnStateDegraded:
		if d := strings.TrimSpace(info.Detail); d != "" {
			return fmt.Sprintf("Connection degraded: %s", d), true
		}
		return "Connection degraded.", true

	case ConnStateReconnecting:
		attempt := info.Attempt
		if attempt < 1 {
			attempt = 1
		}
		max := info.MaxRetries
		if max <= 0 {
			return fmt.Sprintf("Connection lost. Reconnecting (attempt %d)…", attempt), true
		}
		return fmt.Sprintf("Connection lost. Reconnecting (attempt %d/%d)…", attempt, max), true

	case ConnStateFailed:
		if d := strings.TrimSpace(info.Detail); d != "" {
			return fmt.Sprintf("Connection failed: %s", d), true
		}
		return "Connection failed.", true
	}
	// Unknown state: do not clobber an existing banner.
	return "", false
}
