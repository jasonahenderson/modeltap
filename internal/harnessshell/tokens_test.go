package harnessshell

import (
	"strings"
	"testing"
)

// WU-102 Layer 1 parity tests for token capture and preview behavior.
// Maps to FEAT-0014 §"submitted artifact model" and the WU-102
// §"Required Parity Coverage Areas / Token and preview behavior"
// assertions.

func TestLargePasteCompactsIntoToken(t *testing.T) {
	// Simulating a large paste: handleInputMutation captures input
	// deltas >=120 chars as paste tokens and clears the buffer.
	m := newWithFixedClock()
	prev := m.state.input.Value()
	long := strings.Repeat("xy ", 80) // 240 chars
	m.state.input.SetValue(long)
	m.state.handleInputMutation(prev, m.state.input.Value())

	if len(m.state.inputTokens) != 1 {
		t.Fatalf("expected 1 paste token, got %d", len(m.state.inputTokens))
	}
	if m.state.inputTokens[0].Kind != TokenKindPaste {
		t.Fatalf("token kind = %v, want TokenKindPaste", m.state.inputTokens[0].Kind)
	}
	if m.state.input.Value() != "" {
		t.Fatalf("composer should clear after paste capture; got %q", m.state.input.Value())
	}
}

func TestSmallInputDoesNotCompactIntoToken(t *testing.T) {
	// Below the 120-char threshold the input is left in the buffer.
	m := newWithFixedClock()
	prev := ""
	m.state.input.SetValue("hello world")
	m.state.handleInputMutation(prev, m.state.input.Value())
	if len(m.state.inputTokens) != 0 {
		t.Fatalf("short input should not compact; got %d tokens", len(m.state.inputTokens))
	}
}

func TestDroppedPathCapturedAsFileToken(t *testing.T) {
	// A single absolute path arriving via the textarea is captured as
	// a file token (drag-and-drop scenario).
	m := newWithFixedClock()
	m.state.input.SetValue("/Users/jason/notes/foo.md")
	m.state.handleInputMutation("", m.state.input.Value())

	if len(m.state.inputTokens) != 1 {
		t.Fatalf("expected 1 file token, got %d", len(m.state.inputTokens))
	}
	if m.state.inputTokens[0].Kind != TokenKindFile {
		t.Fatalf("token kind = %v, want TokenKindFile", m.state.inputTokens[0].Kind)
	}
	if m.state.inputTokens[0].Payload != "/Users/jason/notes/foo.md" {
		t.Fatalf("payload = %q, want path", m.state.inputTokens[0].Payload)
	}
	if m.state.input.Value() != "" {
		t.Fatalf("composer should clear after path capture")
	}
}

func TestSlashCommandNotClassifiedAsFileToken(t *testing.T) {
	// "/clear" looks like a path prefix but should not be captured as
	// a file token; it stays in the buffer for shell-native handling.
	m := newWithFixedClock()
	m.state.input.SetValue("/clear")
	m.state.handleInputMutation("", m.state.input.Value())
	if len(m.state.inputTokens) != 0 {
		t.Fatalf("slash command must not become a file token; got %d", len(m.state.inputTokens))
	}
	if m.state.input.Value() != "/clear" {
		t.Fatalf("/clear should remain in buffer; got %q", m.state.input.Value())
	}
}

func TestSubmittedPasteTokenStartsExpanded(t *testing.T) {
	// FEAT-0014: "submitted paste tokens render inline and start
	// expanded in the transcript". beginSubmission populates the
	// transcript user row's Expanded map for paste-kind tokens.
	m := newWithFixedClock()
	m.state.input.SetValue("look at this paste")
	m.state.inputTokens = []InputToken{
		{ID: "paste-1", Kind: TokenKindPaste, Label: "paste-1", Payload: "long content"},
	}
	m, _ = drainActions(t, m, enterKey())

	if len(m.state.transcriptItems) < 1 {
		t.Fatalf("expected user row in transcript")
	}
	user := m.state.transcriptItems[0]
	if user.Role != RoleUser {
		t.Fatalf("first row role = %v, want RoleUser", user.Role)
	}
	if !user.Expanded["paste-1"] {
		t.Fatalf("paste-1 should be expanded in submitted user row; got %+v", user.Expanded)
	}
}

func TestTranscriptEnterTogglesPasteTokenExpansion(t *testing.T) {
	// FEAT-0014: "transcript Enter toggles paste-token expansion
	// inline". After submit, focus the transcript and press Enter on
	// the paste-token ref to flip Expanded.
	m := newWithFixedClock()
	m.state.input.SetValue("look at this paste")
	m.state.inputTokens = []InputToken{
		{ID: "paste-1", Kind: TokenKindPaste, Label: "paste-1", Payload: "long content"},
	}
	m, _ = drainActions(t, m, enterKey())

	// transcriptRefs are populated by refresh inside Update (post
	// WU-107 refactor). Refresh already ran from the Enter dispatch
	// above; we just need a selection.
	m.state.focus = FocusTranscript
	if len(m.state.transcriptRefs) == 0 {
		t.Fatalf("expected transcriptRefs populated by refresh after submit")
	}
	m.state.selectedTranscriptRef = 0

	// Initially expanded (set by submit).
	if !m.state.transcriptItems[0].Expanded["paste-1"] {
		t.Fatalf("paste-1 should start expanded after submit")
	}
	// Enter toggles it off.
	m.state.activateSelectedTranscriptRef()
	if m.state.transcriptItems[0].Expanded["paste-1"] {
		t.Fatalf("Enter should collapse paste-1; still expanded")
	}
	// Enter again toggles back on.
	m.state.activateSelectedTranscriptRef()
	if !m.state.transcriptItems[0].Expanded["paste-1"] {
		t.Fatalf("second Enter should re-expand paste-1")
	}
}

func TestTranscriptEnterFileTokenEmitsLoadPreview(t *testing.T) {
	// File tokens via transcript Enter emit LoadPreviewAction with
	// Source="transcript" + MessageIndex/TokenIndex so the host can
	// resolve the source token unambiguously.
	m := newWithFixedClock()
	m.state.transcriptItems = []TranscriptItem{
		{
			ID:    "msg-user-sub-1",
			Kind:  TranscriptItemKindMessage,
			Role:  RoleUser,
			Text:  "look at this file",
			Tokens: []InputToken{
				{ID: "file-1", Kind: TokenKindFile, Label: "file-1 (foo.txt)", Payload: "/abs/foo.txt"},
			},
		},
	}
	m.state.transcriptRefs = []TranscriptRef{{MessageIndex: 0, TokenIndex: 0}}
	m.state.selectedTranscriptRef = 0

	m.state.activateSelectedTranscriptRef()
	if len(m.state.pendingActions) != 1 {
		t.Fatalf("expected 1 LoadPreviewAction, got %d", len(m.state.pendingActions))
	}
	a, ok := m.state.pendingActions[0].(LoadPreviewAction)
	if !ok {
		t.Fatalf("action[0] = %T, want LoadPreviewAction", m.state.pendingActions[0])
	}
	if a.Target.TokenID != "file-1" || a.Target.Source != "transcript" {
		t.Fatalf("target = %+v, want transcript/file-1", a.Target)
	}
}
