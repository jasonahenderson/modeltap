package harnesshost

// Adapter wraps a harnessshell.Model and bridges it to a Runtime
// implementation. It is itself a tea.Model; host programs embed Adapter
// directly and the rest of the bridging stays inside.
//
// Stage D-1 wires the action-consumer side: ActionMsg envelopes from
// the shell are dispatched to Runtime methods via tea.Cmd, and the
// runtime's response (or error) is projected into the corresponding
// HostEvent and routed back to the shell on the next Update tick.
//
// Subsequent commits add:
//   - event projection from runtime tea.Msg types (TurnSubmittedMsg,
//     StreamTokenMsg, etc.) into HostEvents (Stage D-2)
//   - mid-stream pause buffering per WU-099 §"Mid-stream pause"
//     (Stage D-3)
//   - production wiring against internal/harness/app_conn.ConnSurface
//     (Stage D-4)

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// Adapter is the modeltap-specific Bubble Tea model that drives
// harnessshell.Model from a Runtime. Host programs construct an Adapter
// via New, then run it as their tea.Program model.
type Adapter struct {
	shell    harnessshell.Model
	runtime  Runtime
	resolver AttachmentResolver
	ctx      func() context.Context

	// correlation tables — used by event projection (Stage D-2) to
	// map runtime turn/run IDs back to shell submission IDs and vice
	// versa. Populated on SubmissionAcceptedEvent dispatch.
	submissionToRun map[string]string
	runToSubmission map[string]string
	attachedRunID   string

	// pendingPermissions tracks active permission requests by ID per
	// WU-099 §"Mid-stream Pause". While the set is non-empty, the
	// adapter buffers RunDeltaEvent forwarding to the shell so the
	// active stream visually pauses for the user. The set drains as
	// PermissionResolvedEvents arrive; when it empties, the adapter
	// replays buffered deltas in arrival order and resumes live
	// forwarding.
	pendingPermissions  map[string]struct{}
	pauseBuffer         []harnessshell.RunDeltaEvent
	detachedTranscripts map[string][]harnessshell.RunDeltaEvent

	// tokenAttachments correlates shell-emitted token IDs to the
	// resolved Attachment so LoadPreviewAction can populate
	// PreviewRequest.Path. Populated on dispatchSubmit (from
	// Submission.Tokens). Per Codex #3 in the v0.2.2 Phase 2 review.
	tokenAttachments map[string]Attachment
}

// AttachmentResolver translates a shell InputToken into an Attachment
// for the runtime. The default resolver, used when no Option overrides
// it, performs a passthrough projection that copies fields verbatim;
// production implementations resolve relative paths, read paste
// payloads through the SummarizePaste hook, or apply host-specific
// attachment policy.
type AttachmentResolver func(ctx context.Context, runtime Runtime, tok harnessshell.InputToken) (Attachment, error)

// Option configures Adapter at construction time. Options are intended
// for production wiring concerns (resolver, context source); they do
// NOT include callback hooks or state mutation accessors.
type Option func(*Adapter)

// WithAttachmentResolver overrides the default passthrough resolver.
func WithAttachmentResolver(r AttachmentResolver) Option {
	return func(a *Adapter) {
		if r != nil {
			a.resolver = r
		}
	}
}

// WithContextSource overrides the default context.Background source.
// Production hosts inject their request-scoped context here.
func WithContextSource(ctx func() context.Context) Option {
	return func(a *Adapter) {
		if ctx != nil {
			a.ctx = ctx
		}
	}
}

// New constructs an Adapter wrapping the given shell model and runtime
// implementation. Options apply in order.
func New(shell harnessshell.Model, runtime Runtime, opts ...Option) Adapter {
	a := Adapter{
		shell:               shell,
		runtime:             runtime,
		resolver:            defaultAttachmentResolver,
		ctx:                 context.Background,
		submissionToRun:     map[string]string{},
		runToSubmission:     map[string]string{},
		pendingPermissions:  map[string]struct{}{},
		detachedTranscripts: map[string][]harnessshell.RunDeltaEvent{},
		tokenAttachments:    map[string]Attachment{},
	}
	for _, opt := range opts {
		opt(&a)
	}
	return a
}

// Init starts the inner shell. Adapter does not own any timers or
// background work of its own at construction time.
func (a Adapter) Init() tea.Cmd {
	return a.shell.Init()
}

// Update routes Bubble Tea messages between the shell and the
// runtime. ActionMsg envelopes are intercepted and dispatched to the
// runtime via dispatchAction; HostEvents and all other messages flow
// straight through to the shell.
func (a Adapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case harnessshell.ActionMsg:
		return a, a.dispatchAction(m.Action)
	case submissionAcceptedAdapterMsg:
		// Record correlation so later runtime-message projection can
		// map runtime RunIDs back to shell submissions, then forward
		// as the typed shell event through the pause-aware helper.
		a.submissionToRun[m.SubmissionID] = m.RunID
		a.runToSubmission[m.RunID] = m.SubmissionID
		a.attachedRunID = m.RunID
		var cmd tea.Cmd
		a, cmd = a.forwardEvent(harnessshell.SubmissionAcceptedEvent{
			SubmissionID: m.SubmissionID,
			RunID:        m.RunID,
		})
		if m.Label != "" {
			// RunStartedEvent carries the label; without a real
			// runtime stream message the adapter synthesizes a
			// minimal one here so chrome label updates do not
			// require Stage D-2 to be present.
			started := harnessshell.RunStartedEvent{
				SubmissionID: m.SubmissionID,
				RunID:        m.RunID,
				Label:        m.Label,
			}
			startedCmd := func() tea.Msg { return started }
			if cmd == nil {
				cmd = startedCmd
			} else {
				cmd = tea.Batch(cmd, startedCmd)
			}
		}
		return a, cmd
	}

	if evt := projectRuntimeMessage(msg); evt != nil {
		var cmd tea.Cmd
		a, cmd = a.forwardEvent(evt)
		return a, cmd
	}

	// Direct HostEvent injection (e.g., synthetic RunStartedEvent
	// produced by the submissionAccepted Cmd, or events sent by
	// integration tests) also flows through the pause-aware path.
	if evt, ok := msg.(harnessshell.HostEvent); ok {
		var cmd tea.Cmd
		a, cmd = a.forwardEvent(evt)
		return a, cmd
	}

	var cmd tea.Cmd
	var inner tea.Model
	inner, cmd = a.shell.Update(msg)
	a.shell = inner.(harnessshell.Model)
	return a, cmd
}

// forwardEvent routes a HostEvent into the inner shell with the WU-099
// mid-stream pause buffering layered on top:
//
//   - PermissionRequestedEvent registers the request in pending
//     permissions; subsequent RunDeltaEvents buffer until resolution
//   - RunDeltaEvent is buffered (not forwarded) while any permission
//     is pending; the shell visually pauses without needing a
//     PauseRun method on Runtime per WU-099 §"Mid-stream Pause"
//   - PermissionResolvedEvent decrements pending; when the set
//     empties, the resolve is forwarded first and then any buffered
//     deltas replay in arrival order before live forwarding resumes
//
// All other events forward straight through to shell.Update.
func (a Adapter) forwardEvent(evt harnessshell.HostEvent) (Adapter, tea.Cmd) {
	switch e := evt.(type) {
	case harnessshell.SubmissionAcceptedEvent:
		if e.RunID != "" {
			a.attachedRunID = e.RunID
		}
	case harnessshell.RunStartedEvent:
		if e.RunID != "" {
			a.attachedRunID = e.RunID
		}
		if e.RunID != "" && len(a.detachedTranscripts[e.RunID]) > 0 {
			inner, cmd := a.shell.Update(evt)
			a.shell = inner.(harnessshell.Model)
			cmds := []tea.Cmd{}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			for _, d := range a.detachedTranscripts[e.RunID] {
				inner, replayCmd := a.shell.Update(d)
				a.shell = inner.(harnessshell.Model)
				if replayCmd != nil {
					cmds = append(cmds, replayCmd)
				}
			}
			delete(a.detachedTranscripts, e.RunID)
			if len(cmds) == 0 {
				return a, nil
			}
			return a, tea.Batch(cmds...)
		}
	case harnessshell.PermissionRequestedEvent:
		if e.Request.ID != "" {
			a.pendingPermissions[e.Request.ID] = struct{}{}
		}
	case harnessshell.PermissionResolvedEvent:
		delete(a.pendingPermissions, e.RequestID)
		if len(a.pendingPermissions) == 0 && len(a.pauseBuffer) > 0 {
			// Forward the resolve event first so the shell flips
			// the transcript event row and clears composer
			// permission UI before the replayed deltas land.
			inner, cmd := a.shell.Update(evt)
			a.shell = inner.(harnessshell.Model)
			cmds := []tea.Cmd{}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			for _, d := range a.pauseBuffer {
				inner, replayCmd := a.shell.Update(d)
				a.shell = inner.(harnessshell.Model)
				if replayCmd != nil {
					cmds = append(cmds, replayCmd)
				}
			}
			a.pauseBuffer = nil
			if len(cmds) == 0 {
				return a, nil
			}
			return a, tea.Batch(cmds...)
		}
	case harnessshell.RunDeltaEvent:
		if len(a.pendingPermissions) > 0 {
			a.pauseBuffer = append(a.pauseBuffer, e)
			return a, nil
		}
		if e.RunID != "" && a.attachedRunID != "" && e.RunID != a.attachedRunID {
			a.detachedTranscripts[e.RunID] = append(a.detachedTranscripts[e.RunID], e)
			return a, nil
		}
		if e.RunID != "" && a.attachedRunID == "" {
			a.detachedTranscripts[e.RunID] = append(a.detachedTranscripts[e.RunID], e)
			return a, nil
		}
	case harnessshell.RunCompletedEvent:
		if e.RunID == a.attachedRunID {
			a.attachedRunID = ""
		}
	case harnessshell.RunStoppedEvent:
		if e.RunID == a.attachedRunID {
			a.attachedRunID = ""
		}
	case harnessshell.RunFailedEvent:
		if e.RunID == a.attachedRunID {
			a.attachedRunID = ""
		}
	}
	inner, cmd := a.shell.Update(evt)
	a.shell = inner.(harnessshell.Model)
	return a, cmd
}

// View renders through the inner shell. The adapter does not add chrome.
func (a Adapter) View() string {
	return a.shell.View()
}

// dispatchAction maps a typed shell Action onto a Runtime call wrapped
// in a tea.Cmd. The Cmd's resulting tea.Msg is the corresponding
// HostEvent (success path) or a failure HostEvent (error path); the
// next Update tick delivers it back into the shell.
func (a Adapter) dispatchAction(action harnessshell.Action) tea.Cmd {
	switch act := action.(type) {
	case harnessshell.SubmitTurnAction:
		return a.dispatchSubmit(act)
	case harnessshell.InterruptRunAction:
		return a.dispatchInterrupt(act)
	case harnessshell.ResolvePermissionAction:
		return a.dispatchResolvePermission(act)
	case harnessshell.LoadPreviewAction:
		return a.dispatchLoadPreview(act)
	case harnessshell.RunHostCommandAction:
		return a.dispatchHostCommand(act)
	}
	return nil
}

func (a Adapter) dispatchSubmit(act harnessshell.SubmitTurnAction) tea.Cmd {
	sub := act.Submission
	ctx := a.ctx()
	attachments := make([]Attachment, 0, len(sub.Tokens))
	for _, tok := range sub.Tokens {
		att, err := a.resolver(ctx, a.runtime, tok)
		if err != nil {
			msg := err.Error()
			return func() tea.Msg {
				return harnessshell.SubmissionFailedEvent{
					SubmissionID: sub.ID,
					Message:      msg,
				}
			}
		}
		attachments = append(attachments, att)
		// Record the resolved attachment by tokenID so a later
		// LoadPreviewAction can populate PreviewRequest.Path.
		if att.TokenID != "" {
			a.tokenAttachments[att.TokenID] = att
		}
	}
	req := SubmitRequest{
		SubmissionID: sub.ID,
		Text:         sub.Text,
		Entries:      sub.Entries,
		Attachments:  attachments,
		Source:       sub.Source,
		RequestedAt:  sub.RequestedAt,
	}
	runtime := a.runtime
	return func() tea.Msg {
		accepted, err := runtime.SubmitTurn(ctx, req)
		if err != nil {
			return harnessshell.SubmissionFailedEvent{
				SubmissionID: sub.ID,
				Message:      err.Error(),
			}
		}
		return submissionAcceptedAdapterMsg{
			SubmissionID: sub.ID,
			RunID:        accepted.RunID,
			Label:        accepted.Label,
		}
	}
}

func (a Adapter) dispatchInterrupt(act harnessshell.InterruptRunAction) tea.Cmd {
	ctx := a.ctx()
	runtime := a.runtime
	runID := act.RunID
	return func() tea.Msg {
		if err := runtime.InterruptRun(ctx, runID); err != nil {
			return harnessshell.RunFailedEvent{
				RunID:   runID,
				Message: err.Error(),
			}
		}
		// Successful interrupt: the runtime is expected to surface
		// a terminal lifecycle event via the runtime-message
		// projection path (Stage D-2). Return nil so the next
		// Update tick is not driven by this Cmd alone.
		return nil
	}
}

func (a Adapter) dispatchResolvePermission(act harnessshell.ResolvePermissionAction) tea.Cmd {
	ctx := a.ctx()
	runtime := a.runtime
	requestID := act.RequestID
	decision := act.Decision
	return func() tea.Msg {
		if err := runtime.ResolvePermission(ctx, requestID, decision); err != nil {
			return harnessshell.PermissionResolvedEvent{
				RequestID: requestID,
				Outcome:   harnessshell.OutcomeDenied,
				Message:   err.Error(),
			}
		}
		return nil
	}
}

func (a Adapter) dispatchLoadPreview(act harnessshell.LoadPreviewAction) tea.Cmd {
	ctx := a.ctx()
	runtime := a.runtime
	target := act.Target
	req := PreviewRequest{
		TokenID: target.TokenID,
		Source:  target.Source,
	}
	// Look up the resolved attachment recorded at submit time so the
	// runtime has a path to read. Misses (token never submitted, or
	// submitted before adapter construction) leave Path empty and
	// the runtime returns a clear error.
	if att, ok := a.tokenAttachments[target.TokenID]; ok {
		req.Path = att.Path
	}
	return func() tea.Msg {
		payload, err := runtime.LoadPreview(ctx, req)
		if err != nil {
			return harnessshell.HostStatusEvent{
				Status: "Preview failed: " + err.Error(),
				Kind:   harnessshell.StatusError,
			}
		}
		return harnessshell.PreviewLoadedEvent{
			Target:  target,
			Preview: payload,
		}
	}
}

func (a Adapter) dispatchHostCommand(act harnessshell.RunHostCommandAction) tea.Cmd {
	ctx := a.ctx()
	runtime := a.runtime
	cmd := HostCommand{
		Name: act.Invocation.Name,
		Args: act.Invocation.Args,
		Raw:  act.Invocation.Raw,
	}
	return func() tea.Msg {
		if err := runtime.DispatchCommand(ctx, cmd); err != nil {
			return harnessshell.HostStatusEvent{
				Status: "Command failed: " + err.Error(),
				Kind:   harnessshell.StatusError,
			}
		}
		return nil
	}
}

// submissionAcceptedAdapterMsg is an adapter-internal message produced
// by dispatchSubmit on the success path. The next Update tick records
// the SubmissionID → RunID correlation and forwards a typed
// SubmissionAcceptedEvent into the shell.
type submissionAcceptedAdapterMsg struct {
	SubmissionID string
	RunID        string
	Label        string
}

// defaultAttachmentResolver is the passthrough resolver used when no
// Option overrides it. It copies token fields verbatim; production
// resolvers can layer path validation, content reading, or the
// SummarizePaste hook on top.
func defaultAttachmentResolver(ctx context.Context, runtime Runtime, tok harnessshell.InputToken) (Attachment, error) {
	att := Attachment{
		TokenID: tok.ID,
		Kind:    tok.Kind,
		Label:   tok.Label,
	}
	switch tok.Kind {
	case harnessshell.TokenKindFile:
		att.Path = tok.Payload
	case harnessshell.TokenKindPaste:
		att.Payload = tok.Payload
	}
	return att, nil
}
