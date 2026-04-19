package harness

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// modelCmdTimeout bounds model.list and model.switch calls. Catalogs
// are small, so 10s is generous enough for a slow BFF without making
// the UI feel hung.
const modelCmdTimeout = 10 * time.Second

// ModelListLoadedMsg fires when /models successfully fetches the
// catalog. The App surfaces a transient multi-line banner with the
// model table. Keeping rendering out of the controller lets callers
// render however they like (banner today, a dedicated pane later).
type ModelListLoadedMsg struct {
	Response *protocol.ModelListResponse
}

// ModelSwitchedMsg fires after a successful model.switch. When
// OverrideSet is true, the session uses Model verbatim; when false,
// the session falls back to the routing policy (i.e. "auto").
type ModelSwitchedMsg struct {
	Response *protocol.ModelSwitchResponse
}

// ModelErrMsg carries a failure from either model.list or
// model.switch; the App surfaces a transient error banner.
type ModelErrMsg struct {
	Command string
	Err     error
}

// handleModelCommand dispatches the /model and /models slash commands.
// Expected forms:
//
//	/models            → fetch catalog, emit ModelListLoadedMsg
//	/model             → show current effective model (no RPC)
//	/model <name>      → override the session's model
//	/model auto        → clear the override (falls back to routing policy)
//
// Unknown subforms produce a "unknown" banner.
func (a *App) handleModelCommand(msg SubmitMsg) tea.Cmd {
	args := strings.TrimSpace(msg.CommandArgs)

	switch strings.ToLower(msg.Command) {
	case "models":
		// Fetch the catalog; ignore args.
		return a.dispatchModelList()

	case "model":
		if args == "" {
			return a.showCurrentModel()
		}
		return a.dispatchModelSwitch(args)
	}
	return nil
}

func (a *App) showCurrentModel() tea.Cmd {
	line := "Model: " + nonEmpty(a.state.ModelName, "(unset)")
	if a.state.ModelOverride {
		line += " (override)"
	}
	if r := a.state.ModelRouting; r != "" {
		line += " — routing: " + r
	}
	banner := BannerMsg{Text: line, Duration: 4 * time.Second}
	return func() tea.Msg { return banner }
}

// dispatchModelList runs model.list against the wired conn and emits
// ModelListLoadedMsg on success or ModelErrMsg on failure. When no
// conn is wired, emits a ModelErrMsg immediately so the banner
// surfaces the misconfiguration.
func (a *App) dispatchModelList() tea.Cmd {
	conn := a.conn
	return func() tea.Msg {
		if conn == nil {
			return ModelErrMsg{Command: "models", Err: errNoConnection}
		}
		client := conn.Client()
		if client == nil {
			return ModelErrMsg{Command: "models", Err: errNotConnected}
		}
		ctx, cancel := context.WithTimeout(context.Background(), modelCmdTimeout)
		defer cancel()
		resp, err := client.ModelList(ctx)
		if err != nil {
			return ModelErrMsg{Command: "models", Err: err}
		}
		return ModelListLoadedMsg{Response: resp}
	}
}

// dispatchModelSwitch runs model.switch with the supplied target.
// "auto" (case-insensitive) clears the override per the BFF's
// wire contract.
func (a *App) dispatchModelSwitch(target string) tea.Cmd {
	conn := a.conn
	sessionID := a.state.SessionID
	return func() tea.Msg {
		if conn == nil {
			return ModelErrMsg{Command: "model", Err: errNoConnection}
		}
		client := conn.Client()
		if client == nil {
			return ModelErrMsg{Command: "model", Err: errNotConnected}
		}
		ctx, cancel := context.WithTimeout(context.Background(), modelCmdTimeout)
		defer cancel()
		resp, err := client.ModelSwitch(ctx, &protocol.ModelSwitch{
			SessionID: sessionID,
			Model:     target,
		})
		if err != nil {
			return ModelErrMsg{Command: "model", Err: err}
		}
		return ModelSwitchedMsg{Response: resp}
	}
}

// formatModelList produces the banner text shown when
// ModelListLoadedMsg fires. Compact one-line-per-model format keeps
// the banner scannable even for larger catalogs. Marks the
// current override with "(current)".
func formatModelList(resp *protocol.ModelListResponse) string {
	if resp == nil || len(resp.Models) == 0 {
		return "No models registered."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Models (%d):", len(resp.Models))
	for _, m := range resp.Models {
		marker := ""
		if m.Name == resp.CurrentOverride {
			marker = " (current)"
		}
		fmt.Fprintf(&b, "\n  %s [%s]%s — %s", m.Name, m.Provider, marker, m.Description)
	}
	return b.String()
}

// formatModelSwitched produces the banner text for a successful
// model.switch, preferring the server-supplied Reason when present.
func formatModelSwitched(resp *protocol.ModelSwitchResponse) string {
	if resp == nil {
		return "Model switch completed."
	}
	if !resp.OverrideSet {
		msg := "Model override cleared — routing policy restored."
		if resp.Reason != "" {
			msg = resp.Reason
		}
		return msg
	}
	msg := fmt.Sprintf("Model override set: %s", resp.Model)
	if resp.Reason != "" {
		msg += " — " + resp.Reason
	}
	return msg
}

// nonEmpty returns s when non-empty, otherwise alt.
func nonEmpty(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}
