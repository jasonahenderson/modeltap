package harnessshell

// Conversation-surface rendering for the reusable shell. Per WU-100
// §"Definite scope rule for the reusable package", this file renders only
// the FEAT-0014 conversation chrome:
//
//   - transcript and queued-row rendering
//   - composer rendering (including permission controls hosted in the composer)
//   - shell-local preview dialog for paste-token payloads
//   - token rendering (paste and file/reference)
//   - shell-local status/footer presentation
//
// Out-of-scope chrome (sidebar, command palette, agent list/detail, session
// explorer, model catalog) stays in the spike wrapper or in a future
// modeltap top-level harness package.
//
// Stage B introduces a value-type [RenderInput] and a top-level [Render]
// entry point. The spike projects its existing `App` state into a
// [RenderInput] immediately before calling [Render], which keeps the spike's
// `App` struct from leaking into this package while the action/event
// cutover (Stage C) is still pending. The bridge is a temporary surface and
// will be replaced by `Model.View()` once Stage C lands shell-owned state.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderMessageRole identifies the role used for a [RenderMessage]. These
// constants mirror the spike's role-string vocabulary so the bridge can
// project spike state without re-deriving role semantics.
const (
	RenderRoleSystem    = "system"
	RenderRoleUser      = "user"
	RenderRoleAssistant = "assistant"
	RenderRoleEvent     = "event"
	// RenderRoleHostInfo is a host-supplied informational row appended via
	// [HostInfoEvent] (slash-command output). Distinct from RenderRoleSystem
	// so the renderer can apply different styling.
	RenderRoleHostInfo = "host_info"
)

// RenderMessage is the per-row data the [Render] entry point consumes. It is
// the minimum projection of the spike's `message` struct that the
// conversation-surface renderer needs.
//
// Stage B treats this as a temporary bridge type. Once Stage C moves
// transcript state into shell-owned [TranscriptItem] storage, callers will
// no longer construct [RenderMessage] directly.
type RenderMessage struct {
	Role       string
	Content    string
	Streaming  bool
	Tokens     []RenderToken
	Expanded   map[int]bool
	Entries    []string
	EventState string
}

// RenderToken is the per-token data the renderer consumes for transcript
// token rendering. It mirrors the spike's `inputToken` shape for the bridge.
type RenderToken struct {
	ID      string
	Kind    string
	Label   string
	Payload string
}

// RenderQueued is one queued submission row the renderer should emit
// outside the committed transcript list.
type RenderQueued struct {
	Content string
	Tokens  []RenderToken
	Entries []string
}

// RenderPendingPermission carries the composer-hosted permission state the
// renderer needs to draw the active permission controls.
type RenderPendingPermission struct {
	ToolLabel       string
	ToolTarget      string
	ToolSummary     string
	SelectedAction  int
	SessionPolicyOn bool
	PendingTotal    int
	ActiveIndex     int
}

// RenderTranscriptRef points at a token in [RenderInput.Messages]; the
// renderer uses it to apply the active-token highlight in transcript token
// rows.
type RenderTranscriptRef struct {
	MessageIndex int
	TokenIndex   int
}

// RenderFocus mirrors the spike's focus zone for chrome decisions made by
// the renderer (e.g. transcript-focus-only footer hints).
type RenderFocus int

const (
	// RenderFocusInput indicates composer focus.
	RenderFocusInput RenderFocus = iota
	// RenderFocusTranscript indicates transcript focus.
	RenderFocusTranscript
	// RenderFocusSidebar indicates sidebar focus (host-owned chrome, but the
	// renderer still needs the value to choose footer hint variants).
	RenderFocusSidebar
)

// RenderInput is the value-type bridge the spike uses to drive the new
// renderer in Stage B. Every field is shell-relevant; the spike fills it in
// from its `App` state immediately before calling [Render].
//
// Stage C will replace this struct with shell-owned state on [Model].
type RenderInput struct {
	// Width is the transcript inner width (already padding-adjusted).
	Width int

	// Title is the host-fed product or shell name shown in the empty-state
	// welcome block.
	Title string

	// ModelLabel is the host-fed assistant label (e.g. "fake-kimi-spike").
	ModelLabel string

	// Messages is the committed transcript list in render order.
	Messages []RenderMessage

	// Queued is the visible follow-up queue, rendered outside the committed
	// transcript list.
	Queued []RenderQueued

	// InputView is the rendered composer textarea string; the renderer
	// embeds it inside the composer block so the textarea remains
	// host-owned during Stage B.
	InputView string

	// InputTokens are the currently-selected composer tokens.
	InputTokens []RenderToken

	// SelectedToken is the index of the currently active composer token
	// inside [InputTokens].
	SelectedToken int

	// TranscriptRefs is the accumulated transcript-token reference list,
	// derived during render. Callers should pass an empty slice and read
	// it back from [RenderResult.TranscriptRefs].
	TranscriptRefs []RenderTranscriptRef

	// SelectedTranscriptRef is the active transcript-token index.
	SelectedTranscriptRef int

	// Focus is the current focus zone.
	Focus RenderFocus

	// Streaming is true while a host run is producing deltas.
	Streaming bool

	// StreamPulse is the working-indicator pulse counter (0..3).
	StreamPulse int

	// InterruptArmed is true after the first Esc has armed an interrupt.
	InterruptArmed bool

	// PendingPermission, when non-nil, describes the active composer-hosted
	// permission request.
	PendingPermission *RenderPendingPermission

	// PermissionComposerActive is true when the composer-hosted permission
	// action row should highlight its active button. Per FEAT-0014 the
	// spike rule is "composer focused AND composer buffer empty"; the
	// renderer takes the boolean precomputed by the host so it does not
	// need access to the textarea buffer state.
	PermissionComposerActive bool

	// QueuedCount is the size of the visible queue (used in the footer
	// hint). It mirrors len(Queued) but is passed explicitly so the
	// bridge does not depend on slice-length identity.
	QueuedCount int

	// AgentCount is the number of background agents the host wants
	// surfaced in the footer hint. The shell does not own agent state, but
	// FEAT-0014 lets the host show the count in the shell footer.
	AgentCount int

	// Status is the shell's chrome status line text — short single-line
	// content set by the host (via [HostStatusEvent]) or by shell-internal
	// transitions ("Submitted", "Done", "Streaming response"). Empty
	// collapses cleanly: the chrome row is omitted entirely. Multi-line
	// command output should not flow through this field; use a transcript
	// row via [HostInfoEvent] instead.
	Status string

	// StatusKind tags [Status] semantically (ready, streaming, error,
	// permission-pending, interrupt-armed) so the renderer can drive
	// styling without parsing the display string.
	StatusKind StatusKind
}

// RenderResult is the renderer's output. The string in [Content] should be
// piped into the host's transcript viewport. [TranscriptRefs] are the
// transcript-token references discovered during render so the host can
// drive selection state.
type RenderResult struct {
	Content        string
	TranscriptRefs []RenderTranscriptRef
}

// Render produces the conversation surface (transcript + queued rows +
// composer block) as a single string. It is the Stage B entry point used by
// the spike wrapper; Stage C replaces this with `Model.View()` once
// shell-owned state lands on [Model].
//
// The renderer is intentionally pure: given the same [RenderInput] it
// produces the same [RenderResult]. It does not touch the textarea or the
// viewport; the spike retains ownership of those Bubble Tea sub-models
// during Stage B.
func Render(in RenderInput) RenderResult {
	var b strings.Builder
	refs := in.TranscriptRefs[:0]
	contentWidth := in.Width - 2
	if contentWidth < 10 {
		contentWidth = 10
	}
	if len(in.Messages) == 0 && len(in.Queued) == 0 {
		b.WriteString(renderWelcomeBlock(in, contentWidth))
	}
	for i, msg := range in.Messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		switch msg.Role {
		case RenderRoleSystem:
			b.WriteString(systemStyle.Width(contentWidth).Render(msg.Content))
		case RenderRoleUser:
			refs = renderUserRow(&b, in, i, msg, refs, contentWidth)
		case RenderRoleEvent:
			b.WriteString(renderEventRow(msg, contentWidth))
		case RenderRoleAssistant:
			renderAssistantRow(&b, in, msg, contentWidth)
		case RenderRoleHostInfo:
			b.WriteString(renderHostInfoRow(msg, contentWidth))
		}
	}
	for _, queued := range in.Queued {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(renderQueuedRow(queued))
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	if status := renderChromeStatus(in, contentWidth); status != "" {
		b.WriteString(status)
		b.WriteString("\n\n")
	}
	b.WriteString(renderComposerSurface(in))
	return RenderResult{
		Content:        b.String(),
		TranscriptRefs: refs,
	}
}

// renderChromeStatus returns the chrome status line for the current
// shell state, or "" when there's nothing to show. Empty Status
// collapses cleanly so the surface above the composer doesn't reserve
// a blank line.
func renderChromeStatus(in RenderInput, width int) string {
	if in.Status == "" {
		return ""
	}
	style := chromeStatusReadyStyle
	switch in.StatusKind {
	case StatusStreaming:
		style = chromeStatusStreamingStyle
	case StatusError:
		style = chromeStatusErrorStyle
	case StatusPermissionPending:
		style = chromeStatusPermissionStyle
	case StatusInterruptArmed:
		style = chromeStatusInterruptStyle
	}
	return style.Width(width).Render(in.Status)
}

// renderWelcomeBlock renders the shell's compact empty-state identity mark.
func renderWelcomeBlock(in RenderInput, width int) string {
	title := in.Title
	if title == "" {
		title = "modeltap"
	}
	subtitle := "Conversation shell"
	if in.ModelLabel != "" {
		subtitle = "Conversation shell  |  " + in.ModelLabel
	}
	logomark := splashMarkStyle.Render("mt")
	wordmark := lipgloss.JoinVertical(lipgloss.Left,
		splashTitleStyle.Render(title),
		splashSubtitleStyle.Render(subtitle),
	)
	return splashBoxStyle.Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Top, logomark, "  ", wordmark))
}

// renderUserRow renders a user message, including any inline tokens. It
// also accumulates transcript-token refs for selection.
func renderUserRow(b *strings.Builder, in RenderInput, msgIndex int, msg RenderMessage, refs []RenderTranscriptRef, width int) []RenderTranscriptRef {
	var userBlock strings.Builder
	if len(msg.Entries) > 0 {
		for idx, entry := range msg.Entries {
			if idx > 0 {
				userBlock.WriteString("\n\n")
			}
			userBlock.WriteString("▎ " + entry)
		}
	} else if msg.Content != "" {
		userBlock.WriteString("▎ " + msg.Content)
	}
	for tokenIndex, tok := range msg.Tokens {
		if userBlock.Len() > 0 {
			userBlock.WriteString("\n")
		}
		if userBlock.Len() > 0 {
			userBlock.WriteString("\n")
		}
		ref := RenderTranscriptRef{MessageIndex: msgIndex, TokenIndex: tokenIndex}
		refs = append(refs, ref)
		selected := len(refs)-1 == in.SelectedTranscriptRef && in.Focus == RenderFocusTranscript
		userBlock.WriteString(renderTranscriptToken(msg, tokenIndex, tok, selected))
	}
	b.WriteString(userBodyStyle.Width(width).Render(userBlock.String()))
	return refs
}

// renderAssistantRow renders the assistant label (with streaming dot) and
// body content for a transcript row.
func renderAssistantRow(b *strings.Builder, in RenderInput, msg RenderMessage, width int) {
	label := fmt.Sprintf("%s  %s", in.ModelLabel, statusDot(msg.Streaming, in.StreamPulse, in.InterruptArmed))
	b.WriteString(assistantLabelStyle.Width(width).Render(label))
	b.WriteString("\n")
	b.WriteString(assistantBodyStyle.Width(width).Render(msg.Content))
}

// renderHostInfoRow renders a host-supplied informational row (slash-command
// output) with a dim style distinct from assistant content.
func renderHostInfoRow(msg RenderMessage, width int) string {
	return hostInfoStyle.Width(width).Render(msg.Content)
}

// renderEventRow renders a non-conversational event row (permission,
// tool-result, etc.) with a status-driven style.
func renderEventRow(msg RenderMessage, width int) string {
	style := eventInfoStyle
	switch msg.EventState {
	case "requested":
		style = eventRequestedStyle
	case "permission":
		style = eventPermissionStyle
	case "running":
		style = eventRunningStyle
	case "done":
		style = eventDoneStyle
	case "granted":
		style = eventGrantedStyle
	case "denied":
		style = eventDeniedStyle
	}
	return style.Width(width).Render(msg.Content)
}

// renderTranscriptToken renders a single inline transcript token, expanding
// paste tokens that the user has marked expanded and summarizing the rest.
func renderTranscriptToken(msg RenderMessage, tokenIndex int, tok RenderToken, selected bool) string {
	style := transcriptTokenStyle
	if selected {
		style = transcriptTokenActiveStyle
	}
	var lines []string
	lines = append(lines, style.Render(tok.Label))
	switch tok.Kind {
	case "paste":
		if msg.Expanded[tokenIndex] {
			lines = append(lines, transcriptMetaStyle.Render(tok.Payload))
		} else {
			lines = append(lines, transcriptMetaStyle.Render(SummarizePasteToken(tok.Payload)))
		}
	case "file":
		lines = append(lines, transcriptMetaStyle.Render(tok.Payload))
	}
	return transcriptTokenBlockStyle.Render(strings.Join(lines, "\n"))
}

// renderQueuedRow renders one queued follow-up entry outside the committed
// transcript list.
func renderQueuedRow(queued RenderQueued) string {
	var block strings.Builder
	block.WriteString(queuedLabelStyle.Render("queued"))
	if len(queued.Entries) > 0 {
		for idx, entry := range queued.Entries {
			block.WriteString("\n")
			if idx > 0 {
				block.WriteString("\n")
			}
			block.WriteString("▎ " + entry)
		}
	} else if queued.Content != "" {
		block.WriteString("\n")
		block.WriteString("▎ " + queued.Content)
	}
	for _, tok := range queued.Tokens {
		if block.Len() > 0 {
			block.WriteString("\n\n")
		}
		block.WriteString(transcriptTokenBlockStyle.Render(strings.Join([]string{
			transcriptTokenStyle.Render(tok.Label),
			transcriptMetaStyle.Render(SummarizeQueuedToken(tok)),
		}, "\n")))
	}
	return queuedBodyStyle.Render(block.String())
}

// renderComposerSurface renders the composer block: pending permission
// details (when active), composer tokens, the textarea view, and the
// shell-local footer. The composer is tail-mounted on the transcript
// surface per FEAT-0014.
func renderComposerSurface(in RenderInput) string {
	var b strings.Builder
	b.WriteString(renderInputSurface(in))
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(renderFooter(in, in.Width))
	return composerBoxStyle.Render(b.String())
}

// renderInputSurface renders the composer's interior: any active permission
// controls, composer tokens, and the textarea view passed in from the host.
func renderInputSurface(in RenderInput) string {
	var b strings.Builder
	if in.PendingPermission != nil {
		b.WriteString(permissionPromptStyle.Render("Permission required"))
		b.WriteString("\n")
		b.WriteString(renderPermissionDetails(in.PendingPermission))
		b.WriteString("\n")
		b.WriteString(renderPermissionActions(in.PendingPermission, in.PermissionComposerActive))
		b.WriteString("\n")
		meta := "Left/Right select  Enter apply"
		if in.PendingPermission.PendingTotal > 1 {
			meta += "  Up/Down change request"
		}
		b.WriteString(permissionMetaStyle.Render(meta))
		b.WriteString("\n\n")
	}
	if len(in.InputTokens) > 0 {
		for i, tok := range in.InputTokens {
			style := tokenStyle
			if i == in.SelectedToken {
				style = tokenActiveStyle
			}
			b.WriteString(style.Render(tok.Label))
			b.WriteString(" ")
		}
		b.WriteString("\n")
		b.WriteString(tokenHintStyle.Render("Ctrl+P/Ctrl+N select token • Ctrl+O preview token"))
		b.WriteString("\n")
	}
	b.WriteString(in.InputView)
	return b.String()
}

// renderPermissionActions renders the three composer-hosted permission
// action buttons. The selected button is highlighted only when the
// composer has focus.
func renderPermissionActions(p *RenderPendingPermission, composerFocused bool) string {
	if p == nil {
		return ""
	}
	actions := []string{
		renderPermissionAction("Approve once", composerFocused && p.SelectedAction == 0),
		renderPermissionAction("Allow for session", composerFocused && p.SelectedAction == 1),
		renderPermissionAction("Deny", composerFocused && p.SelectedAction == 2),
	}
	return permissionActionsStyle.Render(strings.Join(actions, " "))
}

// renderPermissionAction renders one permission action button.
func renderPermissionAction(label string, active bool) string {
	if active {
		return permissionActionActiveStyle.Render(label)
	}
	return permissionActionStyle.Render(label)
}

// renderPermissionDetails renders the permission summary block: tool label,
// target, summary text, optional pending counter, and session-policy hint.
func renderPermissionDetails(p *RenderPendingPermission) string {
	if p == nil {
		return ""
	}
	lines := []string{
		permissionLabelStyle.Render(p.ToolLabel),
		permissionMetaStyle.Render("target  " + p.ToolTarget),
		permissionMetaStyle.Render(p.ToolSummary),
	}
	if p.PendingTotal > 1 {
		lines = append(lines, permissionMetaStyle.Render(fmt.Sprintf("pending  %d of %d", p.ActiveIndex+1, p.PendingTotal)))
	}
	if p.SessionPolicyOn {
		lines = append(lines, permissionGrantedMetaStyle.Render("session policy active for this tool"))
	}
	return permissionDetailsStyle.Render(strings.Join(lines, "\n"))
}

// renderFooter renders the composer's footer hint line. Per
// PATCH-0027, the right-hand hint advertises only keybindings that
// the current shell actually wires. Sidebar / palette / agent
// surfaces (Ctrl+B / Ctrl+K / Ctrl+T in the spike) live behind
// FEAT-0024 and are not wired in v0.3.x; they will reintroduce a
// host-supplied footer hint when they ship.
//
// The agent-count label only appears when the host explicitly
// supplies a non-zero AgentCount, so a default-zero RenderInput
// no longer prints "0 background agents running".
func renderFooter(in RenderInput, width int) string {
	var labelParts []string

	switch {
	case in.AgentCount == 1:
		labelParts = append(labelParts, "1 background agent running")
	case in.AgentCount > 1:
		labelParts = append(labelParts, fmt.Sprintf("%d background agents running", in.AgentCount))
	}
	if in.Focus == RenderFocusTranscript {
		labelParts = append(labelParts, "scroll: wheel/arrows  items: j/k  open: Enter")
	}
	if in.QueuedCount > 0 {
		labelParts = append(labelParts, fmt.Sprintf("%d queued", in.QueuedCount))
	}

	hint := "Tab focus  Enter submit  Ctrl+J newline"
	left := footerStatusStyle.Render(strings.Join(labelParts, "  |  "))
	right := footerHintStyle.Render(hint)
	space := width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if space < 1 {
		space = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, left, strings.Repeat(" ", space), right)
}

// RenderPreview renders the shell-local paste-token preview dialog. Per
// WU-100 §"Definite scope rule for the reusable package", file/reference
// preview rendering that needs host-side data still flows through this
// dialog; only paste-payload synthesis is fully shell-local.
func RenderPreview(title, content string) string {
	var b strings.Builder
	b.WriteString(dialogTitleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(dialogDividerStyle.Render(strings.Repeat("─", 42)))
	b.WriteString("\n")
	b.WriteString(previewBodyStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(dialogHintStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			keycapStyle.Render("Esc"),
			" ",
			dialogHintStyle.Render("close"),
		),
	))
	return dialogBoxStyle.Render(b.String())
}

// SummarizePasteToken collapses a paste-token payload into a compact
// summary line for transcript and queued-row rendering.
func SummarizePasteToken(payload string) string {
	lines := strings.Split(strings.TrimSpace(payload), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "empty paste"
	}
	summaryLines := minInt(len(lines), 3)
	preview := strings.Join(lines[:summaryLines], " / ")
	if len(lines) > summaryLines {
		return fmt.Sprintf("%d lines: %s ...", len(lines), preview)
	}
	return fmt.Sprintf("%d lines: %s", len(lines), preview)
}

// SummarizeQueuedToken returns the per-token summary line used in queued
// follow-up rendering.
func SummarizeQueuedToken(tok RenderToken) string {
	switch tok.Kind {
	case "paste":
		return SummarizePasteToken(tok.Payload)
	case "file":
		return tok.Payload
	default:
		return tok.Label
	}
}

// statusDot renders the assistant streaming indicator. While streaming, it
// pulses and switches to interrupt-armed text after the first Esc; when
// idle it shows "done".
func statusDot(streaming bool, pulse int, interruptArmed bool) string {
	if streaming {
		if interruptArmed {
			return "press Esc again to interrupt"
		}
		return "working" + strings.Repeat(".", pulse)
	}
	return "done"
}

// minInt is a shell-local int min helper. The reusable package keeps its
// own copy to avoid pulling in modeltap-specific util packages.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
