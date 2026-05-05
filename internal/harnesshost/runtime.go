// Package harnesshost is the modeltap-specific glue between the reusable
// internal/harnessshell conversation component and the modeltap runtime
// services (connection manager, protocol client, tool dispatcher,
// attachment loader, permission enforcer).
//
// Per WU-099 §"Runtime Interface", the adapter depends on a narrow,
// modeltap-internal Runtime contract — not on the full ConnProtocolClient
// from internal/harness/app_conn.go. The interface deliberately exposes
// only what the FEAT-0014 boundary requires.
//
// This file declares the Runtime contract and the request/response types
// that cross between the adapter and the runtime implementation.
package harnesshost

import (
	"context"
	"time"

	"github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// Runtime is the modeltap-internal contract harnesshost depends on.
// Concrete implementations wrap the harness ConnSurface, the tool
// dispatcher, the context manager, and the permission enforcer; tests
// supply in-memory fakes.
//
// Per WU-099 the interface intentionally does NOT include PauseRun /
// ResumeRun. Mid-stream permission pause is implemented inside the
// adapter as a stream-delta buffering concern (Stage D-3 wires it up).
type Runtime interface {
	// SubmitTurn submits a turn to the runtime. The returned
	// SubmitAccepted carries the runtime-assigned RunID; the adapter
	// uses it to correlate later RunDelta/RunCompleted/etc. messages
	// back to the originating shell submission.
	SubmitTurn(ctx context.Context, req SubmitRequest) (SubmitAccepted, error)

	// InterruptRun asks the runtime to stop the named run. The runtime
	// is expected to surface a terminal lifecycle event (handled by
	// the event-projection side of the adapter); InterruptRun itself
	// returns once the request has been delivered, not once the run
	// has actually stopped.
	InterruptRun(ctx context.Context, runID string) error

	// DispatchCommand routes a host-native slash command (e.g. /model,
	// /session, /context) to the appropriate runtime service. The
	// adapter is responsible for surfacing the result back to the
	// shell as a transcript event or HostStatusEvent.
	DispatchCommand(ctx context.Context, cmd HostCommand) error

	// ResolvePermission applies the user's decision to a pending
	// permission request. The (ctx, requestID, decision) shape matches
	// ResolvePermissionAction.RequestID from WU-098 because the shell
	// allows multiple pending permissions to coexist; the request
	// identity is required for the host to apply the decision to the
	// correct request.
	ResolvePermission(ctx context.Context, requestID string, decision harnessshell.PermissionDecision) error

	// LoadPreview asks the runtime to fetch the data for a preview
	// target — typically a file path requested by the shell when the
	// user opens an inline file/reference token.
	LoadPreview(ctx context.Context, req PreviewRequest) (harnessshell.PreviewPayload, error)

	// SummarizePaste compacts a large pasted blob into a one-line
	// summary the shell renders inside the paste-token chip. The
	// shell already knows how to summarize paste payloads locally;
	// this hook is for host-aware summarization (e.g. language
	// detection, code-block heuristics) and is allowed to be a
	// passthrough.
	SummarizePaste(ctx context.Context, raw string) (string, error)
}

// SubmitRequest is the payload the adapter sends to Runtime.SubmitTurn.
// It mirrors harnessshell.Submission but carries adapter-resolved
// attachment data (host has already validated paths, optionally read
// content, and produced protocol-ready attachment descriptors).
type SubmitRequest struct {
	// SubmissionID is the shell-generated submission identifier. The
	// adapter records it in the correlation table so later runtime
	// messages can be projected back to the originating submission.
	SubmissionID string

	// Text is the merged user-visible turn text.
	Text string

	// Entries is the per-entry slice that produced Text (used for
	// queue-merged submissions where several user turns concatenated
	// into one runtime turn).
	Entries []string

	// Attachments is the runtime-resolved attachment descriptor list.
	// The adapter resolves shell InputTokens into Attachments before
	// calling SubmitTurn; the shell never sees Attachment internals.
	Attachments []Attachment

	// Source is the SubmissionSource from the shell (direct or
	// queue_release). Preserved in correlation metadata so runtime
	// telemetry can distinguish queue-released submissions.
	Source harnessshell.SubmissionSource

	// RequestedAt is the shell-stamped submission time.
	RequestedAt time.Time
}

// SubmitAccepted is the runtime's acknowledgement of a submitted turn.
// The adapter projects it into a harnessshell.SubmissionAcceptedEvent
// for the shell to consume.
type SubmitAccepted struct {
	// RunID is the runtime-assigned identifier the adapter uses to
	// correlate later RunDelta/RunCompleted/etc. messages back to the
	// originating shell submission.
	RunID string

	// Label is the optional model/agent label the runtime chose for
	// this run (e.g. the resolved model name). Non-empty values
	// become the chrome-visible label via RunStartedEvent.Label.
	Label string
}

// Attachment is the runtime-facing representation of a shell
// InputToken. The adapter resolves tokens into Attachments before
// calling SubmitTurn; the runtime then attaches them to the protocol
// turn payload according to its own attachment policy.
type Attachment struct {
	// TokenID matches the shell's InputToken.ID; the adapter uses it
	// to thread shell selection state through the runtime turn.
	TokenID string

	// Kind is the shell TokenKind ("paste" or "file") preserved so
	// the runtime can apply kind-specific handling.
	Kind harnessshell.TokenKind

	// Label is the human-readable label the shell rendered for the
	// token (e.g. "image-1 (foo.png)"); the runtime may surface it
	// in tool/result payloads.
	Label string

	// Path is set for file/reference tokens. Empty for paste tokens.
	Path string

	// Payload is set for paste tokens — the raw text the shell
	// captured. Empty for file/reference tokens (the runtime is
	// expected to read the file via Path).
	Payload string
}

// HostCommand is the parsed shape of a host-native slash command
// crossing into the runtime. It mirrors harnessshell.CommandInvocation
// but is named so the adapter API stays self-explanatory.
type HostCommand struct {
	Name string
	Args string
	Raw  string
}

// PreviewRequest carries a resolved preview target into the runtime.
// The adapter is responsible for resolving the shell's PreviewTarget
// (which only knows TokenID + source) into the concrete path or
// reference the runtime needs.
type PreviewRequest struct {
	// TokenID is the shell InputToken.ID being previewed.
	TokenID string

	// Path is the resolved file path for file/reference tokens. Empty
	// for paste tokens (the shell handles those locally and never
	// emits LoadPreviewAction for them).
	Path string

	// Source is "composer" or "transcript", preserved for runtime
	// telemetry.
	Source string
}
