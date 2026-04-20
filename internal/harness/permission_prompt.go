package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harness/tools"
)

// PermissionRequestMsg is published by the permission handler's
// PromptCallback when a tool call requires user approval. The App
// stages the handler and renders a modal banner; user keystrokes
// route to PermissionHandler.HandleKey until a decision lands.
type PermissionRequestMsg struct {
	ToolName string
	Summary  string
}

// permissionRequest is the per-call handoff between the dispatcher
// goroutine (PromptCallback) and the App update loop (HandleKey).
type permissionRequest struct {
	tool    tools.Tool
	input   json.RawMessage
	summary string
	resp    chan permissionDecision
}

// permissionDecision is what HandleKey sends back to the caller.
// "always" grants approval for this session by calling
// PermissionEnforcer.Approve(toolName).
type permissionDecision struct {
	approved bool
	always   bool
}

// PermissionHandler is the modal that shows an approval prompt and
// waits for a user keystroke. Built on the same pattern as
// PasteHandler and CompactHandler — one pending request at a time,
// synchronous from the dispatcher's perspective.
//
// Wiring in runHarness:
//
//	h := NewPermissionHandler(sender, executor.Permissions())
//	executor.SetPromptCallback(h.PromptCallback)
//	app.SetPermissionHandler(h)
//
// The dispatcher goroutine calls PromptCallback, which queues a
// PermissionRequestMsg on the program sender and blocks until the
// user chooses. Between queue-and-block the App's Update reads
// the message, stages the handler, shows the banner. User presses
// y/n/a, HandleKey sends the decision back, dispatcher resumes.
type PermissionHandler struct {
	sender      ProgramSender
	permissions *tools.PermissionEnforcer

	mu      sync.Mutex
	active  bool
	pending *permissionRequest
}

// NewPermissionHandler constructs a handler that posts requests via
// sender and records "always allow" choices in permissions. Both
// are required; passing nil for either produces a handler that
// auto-denies (safer than accepting a half-wired installation).
func NewPermissionHandler(sender ProgramSender, permissions *tools.PermissionEnforcer) *PermissionHandler {
	return &PermissionHandler{sender: sender, permissions: permissions}
}

// Active reports whether a decision is pending.
func (h *PermissionHandler) Active() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.active
}

// PromptCallback satisfies tools.PromptCallback. Runs on the tool
// dispatch goroutine, so it must be safe to call from outside the
// Bubbletea update loop. Posts a PermissionRequestMsg to the App
// via the program sender and blocks on the response channel until
// HandleKey resolves the decision (or ctx cancels).
func (h *PermissionHandler) PromptCallback(ctx context.Context, tool tools.Tool, input json.RawMessage) bool {
	if h == nil || h.sender == nil {
		return false
	}
	summary := defaultPlanSummary(tool.Name(), input)
	req := &permissionRequest{
		tool:    tool,
		input:   input,
		summary: summary,
		resp:    make(chan permissionDecision, 1),
	}

	h.mu.Lock()
	if h.active {
		// Another request already pending. Deny the new one rather
		// than queue — queued prompts would let an adversarial
		// server push a stack of modal dialogs that the user has
		// to click through to reach the one they actually care
		// about. Safer to say "one at a time" and let the server
		// retry.
		h.mu.Unlock()
		return false
	}
	h.active = true
	h.pending = req
	h.mu.Unlock()

	h.sender.Send(PermissionRequestMsg{ToolName: tool.Name(), Summary: summary})

	select {
	case d := <-req.resp:
		if d.always && h.permissions != nil {
			h.permissions.Approve(tool.Name())
		}
		return d.approved
	case <-ctx.Done():
		h.clear()
		return false
	}
}

// HandleKey processes one keystroke while a decision is pending.
// Returns a tea.Cmd that clears the overlay (on y/n/a/esc) or nil
// (unrelated keys). The dispatcher goroutine unblocks via the
// response channel.
func (h *PermissionHandler) HandleKey(k tea.KeyMsg) tea.Cmd {
	h.mu.Lock()
	if !h.active || h.pending == nil {
		h.mu.Unlock()
		return nil
	}
	req := h.pending
	h.mu.Unlock()

	choice := rune(0)
	if k.Type == tea.KeyRunes && len(k.Runes) == 1 {
		choice = k.Runes[0]
	}
	if k.Type == tea.KeyEsc {
		choice = 'n'
	}
	switch choice {
	case 'y', 'Y':
		h.resolve(req, permissionDecision{approved: true})
		return func() tea.Msg { return BannerClearMsg{} }
	case 'a', 'A':
		h.resolve(req, permissionDecision{approved: true, always: true})
		return tea.Batch(
			func() tea.Msg { return BannerClearMsg{} },
			func() tea.Msg {
				return BannerMsg{
					Text:     "Always allowed " + req.tool.Name() + " for this session",
					Duration: 3 * time.Second,
				}
			},
		)
	case 'n', 'N':
		h.resolve(req, permissionDecision{approved: false})
		return tea.Batch(
			func() tea.Msg { return BannerClearMsg{} },
			func() tea.Msg {
				return BannerMsg{Text: "Denied " + req.tool.Name(), Duration: 3 * time.Second}
			},
		)
	}
	return nil
}

// resolve signals the waiting dispatcher goroutine with the decision
// and clears the handler state under the lock.
func (h *PermissionHandler) resolve(req *permissionRequest, d permissionDecision) {
	select {
	case req.resp <- d:
	default:
	}
	h.clear()
}

// clear resets the handler to idle. Called on resolve + on ctx cancel.
func (h *PermissionHandler) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = false
	h.pending = nil
}

// StageBanner returns the banner cmd shown when a PermissionRequestMsg
// arrives. Exposed so the App can call it from its Update handler.
func (h *PermissionHandler) StageBanner(msg PermissionRequestMsg) tea.Cmd {
	var b strings.Builder
	fmt.Fprintf(&b, "Approve tool call?\n  %s\n  %s\n[y]es  [n]o/Esc  [a]lways allow %s this session",
		msg.ToolName, msg.Summary, msg.ToolName)
	banner := BannerMsg{Text: b.String(), Duration: 0}
	return func() tea.Msg { return banner }
}
