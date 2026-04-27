package harnessshell

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
)

// FocusZone identifies which interactive surface currently has input focus.
type FocusZone int

const (
	// FocusSidebar focuses the (optional) sidebar list.
	FocusSidebar FocusZone = iota
	// FocusTranscript focuses the transcript scroll surface.
	FocusTranscript
	// FocusInput focuses the composer textarea.
	FocusInput
)

// Role identifies the speaker associated with a transcript item.
type Role string

const (
	// RoleUser identifies a transcript row authored by the user.
	RoleUser Role = "user"
	// RoleAssistant identifies an assistant streaming or completed row.
	RoleAssistant Role = "assistant"
	// RoleSystem identifies a system-originated row.
	RoleSystem Role = "system"
	// RoleEvent identifies a non-conversational event row (e.g. permission).
	RoleEvent Role = "event"
)

// TranscriptItemKind enumerates the major transcript row varieties.
type TranscriptItemKind int

const (
	// TranscriptItemKindMessage is a user/assistant/system conversational row.
	TranscriptItemKindMessage TranscriptItemKind = iota
	// TranscriptItemKindEvent is a permission or other event row.
	TranscriptItemKindEvent
	// TranscriptItemKindQueued is a queued submission row rendered outside the
	// committed transcript list.
	TranscriptItemKindQueued
)

// EventState carries the transcript-side state for an event row, e.g. the
// active state of a permission request as it transitions from requested to
// resolved.
type EventState struct {
	// Status is a free-form textual marker (e.g. "requested", "granted",
	// "denied"). The shell uses it to pick the row's display style.
	Status string
	// RequestID, when non-empty, links the row to a [PermissionRequest].
	RequestID string
}

// TranscriptItem is the exported transcript-row model defined by WU-098.
// Implementations may use a denser internal form, but every host-observable
// transcript state must round-trip through these fields.
type TranscriptItem struct {
	ID        string
	Kind      TranscriptItemKind
	Role      Role
	Text      string
	Tokens    []InputToken
	Expanded  map[string]bool
	Event     *EventState
	Streaming bool
	Entries   []string
}

// QueuedSubmission is a follow-up that the user submitted while a run was
// active. Queued submissions remain FIFO and merge per WU-098 queue
// invariants.
type QueuedSubmission struct {
	ID      string
	Text    string
	Tokens  []InputToken
	Entries []string
}

// PendingPermission is the shell-side mirror of an active
// [PermissionRequest]. It tracks transcript correlation and the user's
// in-progress action selection in the composer.
type PendingPermission struct {
	Request        PermissionRequest
	TranscriptID   string
	SelectedAction int
}

// TranscriptRef points at a specific token in a transcript row, used for
// selection and preview-target resolution.
type TranscriptRef struct {
	MessageIndex int
	TokenIndex   int
}

// ChoiceOption is a labelled value rendered inside a [ChoiceDialog].
type ChoiceOption struct {
	Label string
	Value string
}

// ChoiceDialog is a shell-local overlay for picking from a small option set.
type ChoiceDialog struct {
	Title   string
	Prompt  string
	Options []ChoiceOption
	Index   int
}

// PreviewDialog is the shell-local preview surface; for paste tokens the
// shell synthesizes [Content] locally, for file tokens [Content] arrives via
// [PreviewLoadedEvent].
type PreviewDialog struct {
	Title   string
	Content string
}

// CommandPaletteState holds the in-flight palette query and selection index.
type CommandPaletteState struct {
	Query string
	Index int
}

// SidebarItemKind enumerates the high-level sidebar row varieties.
type SidebarItemKind int

const (
	// SidebarItemSession identifies a session-list row.
	SidebarItemSession SidebarItemKind = iota
	// SidebarItemModel identifies a model-list row.
	SidebarItemModel
	// SidebarItemAction identifies an action row (e.g. Clear Transcript).
	SidebarItemAction
)

// SidebarItem is one row in the optional sidebar surface.
type SidebarItem struct {
	Section string
	Label   string
	Kind    SidebarItemKind
	Value   string
}

// state holds the shell-owned interaction state. It is private; the public
// API surface is [Model] plus the action/event types in types.go.
//
// The fields here mirror the spike's App state per WU-098 §"Ownership Split /
// Shell-owned state". Stage A introduces the struct; subsequent stages wire
// it into Update/View.
type state struct {
	width  int
	height int

	focus FocusZone

	input      textarea.Model
	transcript viewport.Model

	// Title and label are host-fed presentation hints (e.g. session name and
	// model label). They live in shell-owned state because the shell owns
	// rendering chrome.
	title string
	label string

	// status is the shell's current footer/status text.
	status string
	// statusKind tracks the structured kind that drives chrome decisions.
	statusKind StatusKind

	// transcriptItems is the canonical transcript list (committed messages
	// and event rows). Queued submissions live separately so they render
	// outside the committed list per the WU-098 transcript model.
	transcriptItems []TranscriptItem

	// queuedSubmissions is the visible follow-up queue rendered in the
	// transcript per FEAT-0014.
	queuedSubmissions []QueuedSubmission
	// pendingSubmissions is a transient merge buffer holding submissions that
	// have been promoted out of queuedSubmissions but not yet emitted as
	// SubmitTurnAction. Per WU-098 queue invariants, this buffer is shell-owned
	// and never crosses the action/event boundary.
	pendingSubmissions []QueuedSubmission

	inputTokens   []InputToken
	selectedToken int

	transcriptRefs        []TranscriptRef
	selectedTranscriptRef int

	commandHistory []string
	historyIndex   int
	historyDraft   string

	pendingPermissions    []PendingPermission
	activePermissionIndex int

	interruptArmed bool

	sidebarOpen  bool
	sidebarItems []SidebarItem
	sidebarIndex int

	dialog      *ChoiceDialog
	preview     *PreviewDialog
	palette     *CommandPaletteState
	agentList   *agentListState
	agentDetail *agentDetailState

	// pendingActions is the outbound action queue drained by Update on each
	// tick to forward typed actions to the host program.
	pendingActions []Action
}

// agentListState and agentDetailState are placeholders for spike-only overlays
// that may or may not survive into the reusable shell. They are kept private
// during Stage A to mirror the spike's App fields without committing them to
// the public API.
type agentListState struct {
	Index int
}

type agentDetailState struct {
	AgentID string
}
