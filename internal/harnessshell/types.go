// Package harnessshell is the reusable conversation-shell component extracted
// from the harness spike per FEAT-0014 and PATCH-0015. The package owns
// transcript rendering, composer rendering, queue/permission UI state, and
// shell-local interaction rules. All external effects cross the API boundary
// as typed [Action] values; the host returns typed [HostEvent] values.
//
// This file holds the exported action, event, and supporting data types that
// constitute the closed-typed API boundary defined in WU-098.
package harnessshell

import "time"

// Action is the closed-typed marker satisfied by every outbound shell action.
// External packages cannot define new actions because [Action.isAction] is
// unexported; this is intentional so the action surface stays serializable
// and replayable.
type Action interface {
	isAction()
}

// HostEvent is the closed-typed marker satisfied by every inbound host event.
// External packages cannot define new events because [HostEvent.isHostEvent]
// is unexported; hosts emit only the concrete event types declared in this
// package.
type HostEvent interface {
	isHostEvent()
}

// SubmissionSource describes whether a submission was triggered by a direct
// user submit or by an idle empty-Enter queue release.
type SubmissionSource string

const (
	// SubmissionSourceDirect indicates a normal user submission from the composer.
	SubmissionSourceDirect SubmissionSource = "direct"
	// SubmissionSourceQueueRelease indicates a submission produced by releasing
	// queued work after a run completed or via empty-Enter while idle.
	SubmissionSourceQueueRelease SubmissionSource = "queue_release"
)

// TokenKind enumerates the token varieties the shell knows how to render.
type TokenKind string

const (
	// TokenKindPaste is a compacted paste token whose payload is shell-owned.
	TokenKindPaste TokenKind = "paste"
	// TokenKindFile is a file/reference token whose preview the host loads.
	TokenKindFile TokenKind = "file"
)

// InputToken is a stable, exported representation of a paste or file token in
// the composer or transcript. Token identity must remain stable across
// submission, transcript rendering, preview requests, and queue merges.
type InputToken struct {
	ID      string
	Kind    TokenKind
	Label   string
	Payload string
}

// Submission is the outbound payload that crosses the boundary on
// [SubmitTurnAction]. It contains the merged user-visible entries, the
// normalized merged text, the submitted tokens, and a stable identifier the
// host uses to correlate run lifecycle events.
type Submission struct {
	ID          string
	Entries     []string
	Text        string
	Tokens      []InputToken
	Source      SubmissionSource
	RequestedAt time.Time
}

// PermissionDecision is the closed-string set of decisions the composer can
// emit when applying a permission action.
type PermissionDecision string

const (
	// DecisionApproveOnce approves the request for the current invocation only.
	DecisionApproveOnce PermissionDecision = "approve_once"
	// DecisionApproveSession approves the request and remembers the policy for
	// the current session per host policy state.
	DecisionApproveSession PermissionDecision = "approve_session"
	// DecisionDeny rejects the request.
	DecisionDeny PermissionDecision = "deny"
)

// PermissionOutcome is the host-reported outcome for a resolved permission
// request. It mirrors [PermissionDecision] but also includes terminal
// rejection states the host may originate without a decision.
type PermissionOutcome string

const (
	// OutcomeApprovedOnce indicates the host accepted approve-once.
	OutcomeApprovedOnce PermissionOutcome = "approved_once"
	// OutcomeApprovedSession indicates the host accepted approve-session.
	OutcomeApprovedSession PermissionOutcome = "approved_session"
	// OutcomeDenied indicates the host treated the request as denied.
	OutcomeDenied PermissionOutcome = "denied"
)

// SessionPolicyState describes any remembered policy the host wants the
// composer to surface alongside a pending permission request.
type SessionPolicyState struct {
	// SessionApproved is true when the tool has been previously approved for
	// the current session.
	SessionApproved bool
	// Note is host-supplied display text describing the remembered policy.
	Note string
}

// PermissionRequest is the host-fed payload describing a permission the user
// must resolve through the composer.
type PermissionRequest struct {
	ID                 string
	ToolLabel          string
	Target             string
	Summary            string
	SessionPolicyState SessionPolicyState
}

// PreviewTarget identifies the token the shell wants previewed and which
// surface the request originated from.
type PreviewTarget struct {
	// TokenID is the InputToken.ID requested for preview.
	TokenID string
	// Source identifies the originating surface, e.g. "composer" or "transcript".
	Source string
	// MessageIndex is the transcript row index when Source identifies a
	// transcript token; ignored for composer-source previews.
	MessageIndex int
	// TokenIndex is the per-row token index for transcript-source previews.
	TokenIndex int
}

// PreviewPayload is the host-supplied data the shell renders in the preview
// surface for a non-paste token target.
type PreviewPayload struct {
	Title    string
	Content  string
	Metadata map[string]string
}

// CommandInvocation describes a host-native slash command crossing the
// boundary via [RunHostCommandAction].
type CommandInvocation struct {
	// Name is the parsed command name (e.g. "/model" without the slash).
	Name string
	// Args is the raw remainder of the slash-command line.
	Args string
	// Raw is the original command line as the user typed it.
	Raw string
}

// StopReason describes why a run stopped when the host emits
// [RunStoppedEvent].
type StopReason string

const (
	// StopReasonInterrupt indicates an explicit user-driven interrupt.
	StopReasonInterrupt StopReason = "interrupt"
	// StopReasonHost indicates the host stopped the run on its own.
	StopReasonHost StopReason = "host"
)

// StatusKind lets the shell make chrome decisions without parsing display
// status text.
type StatusKind string

const (
	// StatusReady is the idle steady state.
	StatusReady StatusKind = "ready"
	// StatusStreaming indicates an active streaming run.
	StatusStreaming StatusKind = "streaming"
	// StatusInterruptArmed indicates the first Esc has armed an interrupt.
	StatusInterruptArmed StatusKind = "interrupt_armed"
	// StatusPermissionPending indicates one or more permissions await user input.
	StatusPermissionPending StatusKind = "permission_pending"
	// StatusError indicates a host-reported error state.
	StatusError StatusKind = "error"
)

// SubmitTurnAction is emitted when the user submits the composer or queued
// work is released. The host correlates lifecycle events back via
// [Submission.ID].
type SubmitTurnAction struct {
	Submission Submission
}

func (SubmitTurnAction) isAction() {}

// InterruptRunAction is emitted on the second Esc when an active run is armed
// for interrupt.
type InterruptRunAction struct {
	RunID string
}

func (InterruptRunAction) isAction() {}

// ResolvePermissionAction is emitted when the composer applies the user's
// chosen permission action.
type ResolvePermissionAction struct {
	RequestID string
	Decision  PermissionDecision
}

func (ResolvePermissionAction) isAction() {}

// LoadPreviewAction is emitted when the user requests a preview for a token
// the shell does not fully own (typically a file/reference token).
type LoadPreviewAction struct {
	Target PreviewTarget
}

func (LoadPreviewAction) isAction() {}

// RunHostCommandAction is emitted for host-native slash commands. Shell-native
// commands such as /clear are dispatched locally and never cross the boundary.
type RunHostCommandAction struct {
	Invocation CommandInvocation
}

func (RunHostCommandAction) isAction() {}

// SubmissionAcceptedEvent confirms the host has accepted a submission and
// assigned a RunID. The shell uses it to correlate the optimistically rendered
// assistant row with the host run.
type SubmissionAcceptedEvent struct {
	SubmissionID string
	RunID        string
}

func (SubmissionAcceptedEvent) isHostEvent() {}

// SubmissionFailedEvent indicates the host could not accept a submission. The
// shell removes the placeholder assistant row and surfaces failure text.
type SubmissionFailedEvent struct {
	SubmissionID string
	Message      string
}

func (SubmissionFailedEvent) isHostEvent() {}

// RunStartedEvent transitions the shell from submitted/queued state into
// active streaming. It does not create the assistant transcript row; that row
// was already inserted optimistically when [SubmitTurnAction] was emitted.
type RunStartedEvent struct {
	SubmissionID string
	RunID        string
	Label        string
}

func (RunStartedEvent) isHostEvent() {}

// RunDeltaEvent appends inline assistant output to the active transcript row.
type RunDeltaEvent struct {
	RunID string
	Delta string
}

func (RunDeltaEvent) isHostEvent() {}

// RunCompletedEvent marks the active assistant row complete and triggers
// queue auto-release per FEAT-0014 invariants.
type RunCompletedEvent struct {
	RunID string
}

func (RunCompletedEvent) isHostEvent() {}

// RunStoppedEvent reports an explicit interrupt or host-side stop. Queue
// state remains queued.
type RunStoppedEvent struct {
	RunID   string
	Reason  StopReason
	Message string
}

func (RunStoppedEvent) isHostEvent() {}

// RunFailedEvent marks the run terminal and surfaces failure text; the shell
// does not invent retry semantics.
type RunFailedEvent struct {
	RunID   string
	Message string
}

func (RunFailedEvent) isHostEvent() {}

// PermissionRequestedEvent appends a durable transcript event row and
// activates composer permission controls.
type PermissionRequestedEvent struct {
	Request PermissionRequest
}

func (PermissionRequestedEvent) isHostEvent() {}

// PermissionResolvedEvent updates the transcript event row to granted or
// denied and clears the active composer controls for that request.
type PermissionResolvedEvent struct {
	RequestID string
	Outcome   PermissionOutcome
	Message   string
}

func (PermissionResolvedEvent) isHostEvent() {}

// PreviewLoadedEvent supplies preview data for a requested target. The shell
// owns rendering of the preview surface; the host owns the data fetch.
type PreviewLoadedEvent struct {
	Target  PreviewTarget
	Preview PreviewPayload
}

func (PreviewLoadedEvent) isHostEvent() {}

// HostStatusEvent carries host-supplied display text and a structured
// [StatusKind] so the shell can drive chrome (pulsing dot, interrupt-armed
// styling, permission-pending highlight) without parsing the display string.
//
// Use HostStatusEvent for short single-line chrome state ("Submitted",
// "Done", "Mode: build", "Resumed session: <id>"). For multi-line
// command output that must persist in the transcript, use
// [HostInfoEvent] instead.
type HostStatusEvent struct {
	Status string
	Kind   StatusKind
}

func (HostStatusEvent) isHostEvent() {}

// HostInfoEvent appends a host-supplied informational row to the
// transcript. Used for slash-command output (/models, /sessions,
// /runs, /context, /history) where the result is multi-line and must
// persist in the visible transcript rather than flash through the
// chrome status line. Distinct from [HostStatusEvent], which carries
// short single-line chrome state.
type HostInfoEvent struct {
	Text string
}

func (HostInfoEvent) isHostEvent() {}

// ActionMsg is the [tea.Msg] envelope used to forward outbound shell actions
// to the host program. Per WU-098 §"Concrete forwarding shape", the exact
// envelope shape was deferred to WU-100. The reusable shell uses a single
// envelope so adding a new [Action] type does not require host loops to grow
// new outermost cases; hosts pattern-match `ActionMsg` once and dispatch on
// the concrete `Action` type at the host-adapter layer.
type ActionMsg struct {
	Action Action
}
