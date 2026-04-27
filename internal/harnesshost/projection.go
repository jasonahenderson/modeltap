package harnesshost

// Runtime message → HostEvent projection. Per WU-099 §"Runtime Event →
// Shell Event Mapping", the adapter is the only package in the repo
// that imports both internal/harnessshell and internal/harness, and it
// uses that privileged position to translate modeltap-internal runtime
// notifications into the typed shell events the conversation surface
// understands.
//
// Stage D-2 wires the projection cases for stream lifecycle (delta /
// complete), tool activity, branch events (flattened into the single-
// transcript model per FEAT-0014), permission prompts, and the chrome
// status updates (connection / model / context / cost). Mid-stream
// pause buffering of RunDeltaEvent lands in Stage D-3.

import (
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
)

// projectRuntimeMessage maps a runtime tea.Msg into the corresponding
// shell HostEvent (or nil when the message has no shell-bound
// projection — e.g., paste-handler messages that the host App owns).
// The adapter calls this from Update's runtime-message branch and
// forwards the result to the inner shell.
func projectRuntimeMessage(msg interface{}) harnessshell.HostEvent {
	switch m := msg.(type) {
	case harness.StreamTokenMsg:
		return harnessshell.RunDeltaEvent{
			RunID: m.TurnID,
			Delta: m.Delta,
		}
	case harness.StreamCompleteMsg:
		return harnessshell.RunCompletedEvent{
			RunID: m.TurnID,
		}
	case harness.TurnSubmittedMsg:
		if m.Err != nil {
			return harnessshell.SubmissionFailedEvent{
				SubmissionID: m.TurnID,
				Message:      m.Err.Error(),
			}
		}
		return harnessshell.SubmissionAcceptedEvent{
			SubmissionID: m.TurnID,
			RunID:        m.TurnID,
		}
	case harness.StatusUpdateMsg:
		return harnessshell.HostStatusEvent{
			Status: m.Message,
			Kind:   harnessshell.StatusStreaming,
		}
	case harness.BranchStartedMsg:
		return harnessshell.RunStartedEvent{
			SubmissionID: m.TurnID,
			RunID:        branchRunID(m.TurnID, m.BranchID),
			Label:        m.Model,
		}
	case harness.BranchCompleteMsg:
		return harnessshell.RunCompletedEvent{
			RunID: branchRunID(m.TurnID, m.BranchID),
		}
	case harness.BranchErrorMsg:
		return harnessshell.RunFailedEvent{
			RunID:   branchRunID(m.TurnID, m.BranchID),
			Message: m.Error,
		}
	case harness.ToolActivityMsg:
		return projectToolActivity(m)
	case harness.PermissionPromptMsg:
		return harnessshell.PermissionRequestedEvent{
			Request: harnessshell.PermissionRequest{
				ID:        m.ToolCallID,
				ToolLabel: m.ToolName,
				Target:    permissionTargetFromInput(m.Input),
				Summary:   m.Description,
			},
		}
	case harness.ConnStateMsg:
		return harnessshell.HostStatusEvent{
			Status: connStateStatus(m.Info),
			Kind:   connStateKind(m.Info),
		}
	case harness.ModelUpdateMsg:
		return harnessshell.HostStatusEvent{
			Status: "Model: " + m.Name,
			Kind:   harnessshell.StatusReady,
		}
	case harness.ContextUpdateMsg:
		return harnessshell.HostStatusEvent{
			Status: contextUpdateStatus(m),
			Kind:   harnessshell.StatusReady,
		}
	case harness.CostUpdateMsg:
		return harnessshell.HostStatusEvent{
			Status: costUpdateStatus(m),
			Kind:   harnessshell.StatusReady,
		}
	}
	return nil
}

// branchRunID composes a stable run identifier for a multi-model branch.
// The shell sees the branch as a distinct run because the FEAT-0014
// single-transcript model flattens branches into per-model assistant
// rows.
func branchRunID(turnID, branchID string) string {
	if branchID == "" {
		return turnID
	}
	return turnID + ":" + branchID
}

// projectToolActivity projects a runtime ToolActivityMsg into a
// HostStatusEvent describing the current tool state. Per FEAT-0014 the
// transcript event row chrome for tool calls is the host's
// responsibility (the shell only renders permission events as
// transcript rows); a HostStatusEvent surfaces the activity in the
// status footer without spawning a transcript row.
func projectToolActivity(m harness.ToolActivityMsg) harnessshell.HostEvent {
	prefix := "⚙ "
	if m.Phase == harness.ToolActivityEnd {
		switch m.Status {
		case "success":
			prefix = "✓ "
		case "error":
			prefix = "✗ "
		case "rejected":
			prefix = "⊘ "
		default:
			prefix = "• "
		}
	}
	status := prefix + m.ToolName
	if m.Summary != "" {
		status += " — " + m.Summary
	}
	return harnessshell.HostStatusEvent{
		Status: status,
		Kind:   harnessshell.StatusStreaming,
	}
}

// permissionTargetFromInput extracts a one-line target description from
// a permission prompt's Input JSON. The input shape is tool-specific so
// the projection is intentionally conservative: it returns the JSON's
// raw string when short, otherwise the tool name. WU-099 calls out
// adapter-side enrichment as the right place for tool-specific target
// extraction.
func permissionTargetFromInput(input []byte) string {
	const maxLen = 80
	if len(input) == 0 {
		return ""
	}
	s := string(input)
	if len(s) > maxLen {
		s = s[:maxLen] + "…"
	}
	return s
}

// connStateStatus and connStateKind map the modeltap connection-state
// info into the shell's status string + StatusKind. The shell does not
// import internal/harness so this projection lives here.
func connStateStatus(info harness.ConnStateInfo) string {
	if info.State == "" {
		return "Connection state changed"
	}
	if info.Detail != "" {
		return "Connection: " + info.State + " (" + info.Detail + ")"
	}
	return "Connection: " + info.State
}

func connStateKind(info harness.ConnStateInfo) harnessshell.StatusKind {
	switch info.State {
	case "connected":
		return harnessshell.StatusReady
	case "connecting", "reconnecting":
		return harnessshell.StatusStreaming
	case "disconnected", "error":
		return harnessshell.StatusError
	}
	return harnessshell.StatusReady
}

// contextUpdateStatus formats a ContextUpdateMsg into a single status
// line. The shell drives chrome decisions from Kind, so the format here
// is purely cosmetic.
func contextUpdateStatus(m harness.ContextUpdateMsg) string {
	if m.Max == 0 {
		return "Context updated"
	}
	return formatContextUpdate(m.Pct, m.Used, m.Max)
}

// costUpdateStatus formats a CostUpdateMsg into a single status line.
func costUpdateStatus(m harness.CostUpdateMsg) string {
	return formatCostUpdate(m.Total)
}
