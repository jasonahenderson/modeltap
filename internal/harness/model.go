package harness

// Post-WU-106 trim: this file retains only the runtime-shared types
// referenced from outside the package — specifically by
// internal/harnesshost/projection.go (ConnStateInfo, TokenInfo) and
// internal/harness/connection.go's state-transition machinery
// (ConnState* constants). The App-internal types (FocusZone,
// AppState, DisplayMessage, RoleUser/Assistant/etc., NewAppState,
// NextSequence, ResetSequence) were removed alongside the App
// surfaces in the same WU.

// Connection state constants matching FEAT-0008 §"Connection states"
// verbatim. These string values are wire-visible (`HealthResponse`,
// `Diagnostic`, `session.sync` payloads) so the harness renders
// against the same canonical names the Runtime emits.
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

// ConnStateInfo carries connection state for status banners. The
// internal/harnesshost projection layer reads it to drive
// HostStatusEvent translations (per the connection.go event bridge's
// ConnStateMsg sends).
type ConnStateInfo struct {
	State      string
	Detail     string
	Attempt    int
	MaxRetries int
}

// TokenInfo aggregates per-turn token counts. Carried in
// StreamCompleteMsg.
type TokenInfo struct {
	Input  int
	Output int
}
