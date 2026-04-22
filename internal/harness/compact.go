package harness

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// CompactPlanLoadedMsg fires when /compact fetches a plan from the
// BFF. The App seeds the CompactHandler and renders a banner with
// the top-level apply/cancel prompt.
type CompactPlanLoadedMsg struct {
	Plan *protocol.CompactPlan
}

// CompactAppliedMsg fires after a successful compact.apply round-
// trip. Banner surfaces freed tokens + a short narrative.
type CompactAppliedMsg struct {
	Response *protocol.CompactApplyResponse
}

// CompactErrMsg carries a failure from either session.compact or
// compact.apply. The App renders it as a transient banner.
type CompactErrMsg struct {
	Command string
	Err     error
}

// compactHandlerState is the CompactHandler's internal state machine.
type compactHandlerState int

const (
	compactIdle compactHandlerState = iota
	compactAwaitingChoice
	compactApplying
)

// CompactHandler is the harness-side modal overlay driving the
// /compact flow. Invoked after the App receives CompactPlanLoadedMsg;
// captures keystrokes [a]pply / [c]ancel and dispatches the result.
//
// v1 renders only the top-level two-choice prompt ([a]pply all, using
// the server's suggested actions; [c]ancel). Per-category select
// mode ([s]elect → k/s/d/p per category) is a follow-up — the wire
// surface supports arbitrary action maps today, so the follow-up is
// purely UI.
type CompactHandler struct {
	mu    sync.Mutex
	state compactHandlerState
	plan  *protocol.CompactPlan
}

// NewCompactHandler returns an inactive handler.
func NewCompactHandler() *CompactHandler { return &CompactHandler{} }

// Active reports whether a compact decision is pending.
func (h *CompactHandler) Active() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state != compactIdle
}

// Stage captures a plan and transitions to awaiting-choice. Returns
// a banner cmd describing the plan + the apply/cancel prompt.
func (h *CompactHandler) Stage(plan *protocol.CompactPlan) tea.Cmd {
	h.mu.Lock()
	h.state = compactAwaitingChoice
	h.plan = plan
	h.mu.Unlock()

	banner := formatCompactPlan(plan)
	return func() tea.Msg { return BannerMsg{Text: banner, Duration: 0} }
}

// HandleKey processes one keystroke while the handler is active.
// Returns a cmd that either dispatches compact.apply with the
// server's suggested actions ([a]), or clears the overlay ([c]/Esc).
// Other keys are swallowed so stray input doesn't escape into the
// input area while the user decides.
func (h *CompactHandler) HandleKey(k tea.KeyMsg) tea.Cmd {
	h.mu.Lock()
	if h.state != compactAwaitingChoice {
		h.mu.Unlock()
		return nil
	}
	plan := h.plan

	choice := rune(0)
	if k.Type == tea.KeyRunes && len(k.Runes) == 1 {
		choice = k.Runes[0]
	}
	if k.Type == tea.KeyEsc {
		choice = 'c'
	}

	switch choice {
	case 'a', 'A':
		h.state = compactApplying
		h.mu.Unlock()
		actions := suggestedActionsMap(plan)
		return func() tea.Msg { return compactApplyRequestMsg{actions: actions} }
	case 'c', 'C':
		h.state = compactIdle
		h.plan = nil
		h.mu.Unlock()
		return tea.Batch(
			func() tea.Msg { return BannerClearMsg{} },
			func() tea.Msg {
				return BannerMsg{Text: "Compaction cancelled.", Duration: 3 * time.Second}
			},
		)
	}
	h.mu.Unlock()
	return nil
}

// Complete clears the handler after an apply finishes (success or
// failure). The App calls this when CompactAppliedMsg / CompactErrMsg
// with Command="compact apply" arrives.
func (h *CompactHandler) Complete() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = compactIdle
	h.plan = nil
}

// compactApplyRequestMsg is the internal signal the handler emits
// when the user approves; the App sees it and issues the RPC.
type compactApplyRequestMsg struct {
	actions map[string]string
}

// suggestedActionsMap copies the plan's suggested_action per category
// into the map shape compact.apply expects.
func suggestedActionsMap(plan *protocol.CompactPlan) map[string]string {
	if plan == nil {
		return nil
	}
	out := make(map[string]string, len(plan.Categories))
	for _, c := range plan.Categories {
		out[c.Name] = c.SuggestedAction
	}
	return out
}

// formatCompactPlan renders the plan-review banner: headline + per-
// category rows (tokens, suggested action, preview) + estimated
// freed + context pct before/after + the apply/cancel prompt.
func formatCompactPlan(plan *protocol.CompactPlan) string {
	if plan == nil || len(plan.Categories) == 0 {
		return "Nothing to compact (session too small or no candidate categories)."
	}
	var b strings.Builder
	fmt.Fprintf(&b,
		"Compact plan — estimated free %d tokens (%.0f%% → %.0f%%):",
		plan.EstimatedTokensFreed,
		plan.ContextPctBefore*100, plan.ContextPctAfter*100,
	)
	for _, c := range plan.Categories {
		fmt.Fprintf(&b, "\n  %-20s  %6d tok  →  %s",
			c.Name, c.TokenCount, c.SuggestedAction)
		if c.SummaryPreview != "" {
			fmt.Fprintf(&b, "\n      %s", c.SummaryPreview)
		}
	}
	b.WriteString("\n[a]pply all  [c]ancel")
	return b.String()
}

// handleCompactCommand drives /compact from the App's command router.
// Without a session id there's nothing to compact — short-circuits
// to a usage banner. Otherwise dispatches session.compact and stages
// the plan on CompactPlanLoadedMsg via the App Update handler.
func (a *App) handleCompactCommand(_ SubmitMsg) tea.Cmd {
	conn := a.conn
	sessionID := a.state.SessionID
	if sessionID == "" {
		return func() tea.Msg {
			return BannerMsg{Text: "/compact needs an active session", Duration: 4 * time.Second}
		}
	}
	if conn == nil {
		return func() tea.Msg {
			return CompactErrMsg{Command: "compact", Err: errNoConnection}
		}
	}
	return func() tea.Msg {
		client := conn.Client()
		if client == nil {
			return CompactErrMsg{Command: "compact", Err: errNotConnected}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		plan, err := client.SessionCompact(ctx, sessionID)
		if err != nil {
			return CompactErrMsg{Command: "compact", Err: err}
		}
		return CompactPlanLoadedMsg{Plan: plan}
	}
}

// dispatchCompactApply fires compact.apply with the provided action
// map. Emits CompactAppliedMsg on success, CompactErrMsg on failure;
// either way the App calls compact.Complete() in the follow-up.
func (a *App) dispatchCompactApply(actions map[string]string) tea.Cmd {
	conn := a.conn
	sessionID := a.state.SessionID
	return func() tea.Msg {
		if conn == nil {
			return CompactErrMsg{Command: "compact apply", Err: errNoConnection}
		}
		client := conn.Client()
		if client == nil {
			return CompactErrMsg{Command: "compact apply", Err: errNotConnected}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resp, err := client.CompactApply(ctx, sessionID, actions)
		if err != nil {
			return CompactErrMsg{Command: "compact apply", Err: err}
		}
		return CompactAppliedMsg{Response: resp}
	}
}

// formatCompactApplied composes the post-apply banner.
func formatCompactApplied(resp *protocol.CompactApplyResponse) string {
	if resp == nil {
		return "Compact applied."
	}
	if !resp.Applied {
		return "Compact: " + resp.Summary
	}
	return fmt.Sprintf(
		"Compacted — freed %d tokens (context now %.0f%%). %s",
		resp.TokensFreed, resp.ContextPctAfter*100, resp.Summary,
	)
}
