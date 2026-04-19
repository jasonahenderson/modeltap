package harness

import (
	"strings"
	"time"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// FocusZone identifies which child component currently owns keyboard
// input. The App routes raw key events to the focused zone after its
// own global key bindings (quit, mode toggle, etc.) take precedence.
type FocusZone int

const (
	InputFocus FocusZone = iota
	ViewportFocus
)

// String returns the wire-friendly focus name. Used by tests and by
// the status bar's optional debug overlay.
func (f FocusZone) String() string {
	switch f {
	case InputFocus:
		return "input"
	case ViewportFocus:
		return "viewport"
	default:
		return "unknown"
	}
}

// Connection state constants matching FEAT-0008 §"Connection states"
// verbatim. These string values are wire-visible (`HealthResponse`,
// `Diagnostic`, `session.sync` payloads) so the harness side renders
// against the same canonical names the BFF emits.
const (
	ConnStateDiscovering    = "discovering"
	ConnStateStarting       = "starting"
	ConnStateConnecting     = "connecting"
	ConnStateAuthenticating = "authenticating"
	ConnStateRegistering    = "registering"
	ConnStateReady          = "ready"
	ConnStateDegraded       = "degraded"
	ConnStateReconnecting   = "reconnecting"
	ConnStateFailed         = "failed"
)

// Display-role constants for type-safe DisplayMessage construction.
// The viewport switches on these to choose styling.
const (
	RoleUser       = "user"
	RoleAssistant  = "assistant"
	RoleToolCall   = "tool_call"
	RoleToolResult = "tool_result"
	RoleSystem     = "system"
)

// ConnStateInfo carries connection state for the status bar and any
// banners. WU-074 will populate this from real connection events.
type ConnStateInfo struct {
	State      string
	Detail     string
	Attempt    int
	MaxRetries int
}

// TokenInfo aggregates per-turn token counts.
type TokenInfo struct {
	Input  int
	Output int
}

// DisplayMessage is one rendered conversation entry the viewport
// scrolls. Streaming==true means tokens are still arriving and the
// renderer should keep re-rendering Content as it grows.
type DisplayMessage struct {
	Role      string
	Content   string
	Rendered  string
	Model     string
	Routing   string
	Override  bool
	Tokens    TokenInfo
	Cost      float64
	Duration  time.Duration
	TurnID    string
	BranchID  string
	Streaming bool
}

// AppState is the shared mutable state the App owns. Child components
// receive *AppState (not by value) so they can read it during View()
// and write to it during Update() without round-tripping through
// messages.
type AppState struct {
	Focus FocusZone

	ConnState ConnStateInfo

	SessionID string
	Mode      protocol.Mode

	ModelName     string
	ModelOverride bool
	ModelRouting  string

	ContextPct  float64
	ContextUsed int
	ContextMax  int

	SessionCost float64

	CallActive    bool
	CallStartTime time.Time

	Messages []DisplayMessage

	StreamingTurnID string
	StreamingBuf    strings.Builder

	// Banner is the current transient banner text (BannerMsg). Empty
	// when no banner is active.
	Banner string

	// nextSeq is the per-session monotonic turn sequence number. App
	// hands NextSequence() to the protocol client when building
	// turn.submit payloads.
	nextSeq int
}

// NewAppState returns a freshly-initialized AppState with Mode set to
// the default ModeBuild and Focus on the input area.
func NewAppState() *AppState {
	return &AppState{
		Focus: InputFocus,
		Mode:  protocol.ModeBuild,
		ConnState: ConnStateInfo{
			State: ConnStateDiscovering,
		},
	}
}

// NextSequence increments and returns the session's turn sequence.
// Reset on new session.
func (s *AppState) NextSequence() int {
	s.nextSeq++
	return s.nextSeq
}

// ResetSequence resets the per-session turn sequence to zero — called
// when binding a new session id.
func (s *AppState) ResetSequence() { s.nextSeq = 0 }
