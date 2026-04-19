package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// sessionCmdTimeout bounds session.* RPCs. Session ops are server-side
// state mutations that rarely take more than a second; 10s is slack
// for a sluggish BFF.
const sessionCmdTimeout = 10 * time.Second

// SessionListLoadedMsg fires when /sessions successfully fetches the
// list. The App renders it as a multi-line banner — a dedicated
// explorer component (design D5 long-form) is deferred.
type SessionListLoadedMsg struct {
	Response *protocol.SessionListResponse
}

// SessionResumedMsg fires when /session resume <id> succeeds.
type SessionResumedMsg struct {
	Response *protocol.SessionResumeResponse
}

// SessionClearedMsg fires when /session clear succeeds.
type SessionClearedMsg struct {
	Response *protocol.SessionClearResponse
}

// SessionForkedMsg fires when /session fork succeeds.
type SessionForkedMsg struct {
	Response *protocol.SessionForkResponse
}

// SessionErrMsg carries a failure from any session RPC. Command is
// the slash-command verb that produced the call ("sessions", "session
// resume", etc.) so the banner can name the exact action.
type SessionErrMsg struct {
	Command string
	Err     error
}

// handleSessionCommand dispatches /sessions and /session subcommands.
//
//	/sessions                  → list
//	/session                   → show current id + banner summary
//	/session resume <id>       → resume
//	/session clear             → clear current session context
//	/session fork              → fork current session
//	/session <anything else>   → unknown subcommand
func (a *App) handleSessionCommand(msg SubmitMsg) tea.Cmd {
	switch strings.ToLower(msg.Command) {
	case "sessions":
		return a.dispatchSessionList()
	case "session":
		args := strings.TrimSpace(msg.CommandArgs)
		if args == "" {
			return a.showCurrentSession()
		}
		parts := strings.SplitN(args, " ", 2)
		sub := strings.ToLower(parts[0])
		rest := ""
		if len(parts) == 2 {
			rest = strings.TrimSpace(parts[1])
		}
		switch sub {
		case "list":
			return a.dispatchSessionList()
		case "resume":
			if rest == "" {
				return func() tea.Msg {
					return BannerMsg{Text: "Usage: /session resume <session-id>", Duration: 4 * time.Second}
				}
			}
			return a.dispatchSessionResume(rest)
		case "clear":
			return a.dispatchSessionClear()
		case "fork":
			return a.dispatchSessionFork()
		}
		return func() tea.Msg {
			return BannerMsg{
				Text:     fmt.Sprintf("Unknown /session subcommand: %q", sub),
				Duration: 4 * time.Second,
			}
		}
	}
	return nil
}

func (a *App) showCurrentSession() tea.Cmd {
	line := "Session: " + nonEmpty(a.state.SessionID, "(new session)")
	banner := BannerMsg{Text: line, Duration: 4 * time.Second}
	return func() tea.Msg { return banner }
}

func (a *App) dispatchSessionList() tea.Cmd {
	conn := a.conn
	return func() tea.Msg {
		client, err := resolveSessionClient(conn)
		if err != nil {
			return SessionErrMsg{Command: "sessions", Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), sessionCmdTimeout)
		defer cancel()
		resp, err := client.SessionList(ctx)
		if err != nil {
			return SessionErrMsg{Command: "sessions", Err: err}
		}
		return SessionListLoadedMsg{Response: resp}
	}
}

func (a *App) dispatchSessionResume(id string) tea.Cmd {
	conn := a.conn
	return func() tea.Msg {
		client, err := resolveSessionClient(conn)
		if err != nil {
			return SessionErrMsg{Command: "session resume", Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), sessionCmdTimeout)
		defer cancel()
		// ProjectContext is supplied empty here; the BFF uses the
		// registered project from capabilities.register. Future WUs may
		// override when the harness cares about cross-project resumes.
		resp, err := client.SessionResume(ctx, id, protocol.ProjectContext{})
		if err != nil {
			return SessionErrMsg{Command: "session resume", Err: err}
		}
		return SessionResumedMsg{Response: resp}
	}
}

func (a *App) dispatchSessionClear() tea.Cmd {
	conn := a.conn
	sessionID := a.state.SessionID
	return func() tea.Msg {
		if sessionID == "" {
			return SessionErrMsg{Command: "session clear", Err: errNoActiveSession}
		}
		client, err := resolveSessionClient(conn)
		if err != nil {
			return SessionErrMsg{Command: "session clear", Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), sessionCmdTimeout)
		defer cancel()
		resp, err := client.SessionClear(ctx, sessionID)
		if err != nil {
			return SessionErrMsg{Command: "session clear", Err: err}
		}
		return SessionClearedMsg{Response: resp}
	}
}

func (a *App) dispatchSessionFork() tea.Cmd {
	conn := a.conn
	sessionID := a.state.SessionID
	return func() tea.Msg {
		if sessionID == "" {
			return SessionErrMsg{Command: "session fork", Err: errNoActiveSession}
		}
		client, err := resolveSessionClient(conn)
		if err != nil {
			return SessionErrMsg{Command: "session fork", Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), sessionCmdTimeout)
		defer cancel()
		resp, err := client.SessionFork(ctx, sessionID)
		if err != nil {
			return SessionErrMsg{Command: "session fork", Err: err}
		}
		return SessionForkedMsg{Response: resp}
	}
}

// resolveSessionClient extracts the live protocol client from conn or
// returns an informative error if the App isn't wired up.
func resolveSessionClient(conn ConnSurface) (ConnProtocolClient, error) {
	if conn == nil {
		return nil, errNoConnection
	}
	client := conn.Client()
	if client == nil {
		return nil, errNotConnected
	}
	return client, nil
}

// formatSessionList composes the multi-line banner for /sessions.
// One line per session with id prefix, summary, context %, cost, and
// turn count.
func formatSessionList(resp *protocol.SessionListResponse) string {
	if resp == nil || len(resp.Sessions) == 0 {
		return "No sessions yet."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Sessions (%d):", len(resp.Sessions))
	for _, s := range resp.Sessions {
		label := s.Summary
		if label == "" {
			label = s.LastTurnSummary
		}
		if label == "" {
			label = "(no summary)"
		}
		fmt.Fprintf(&b, "\n  %s — %s [ctx %.0f%% · $%.4f · %d turns]",
			s.ID, label, s.ContextPct*100, s.TotalCost, s.TurnCount)
	}
	return b.String()
}

var errNoActiveSession = &noSessionError{}

type noSessionError struct{}

func (*noSessionError) Error() string { return "no active session" }
