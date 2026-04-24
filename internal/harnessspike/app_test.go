package harnessspike

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewSeedsMessages(t *testing.T) {
	app := New()
	if len(app.messages) < 2 {
		t.Fatalf("expected seeded transcript, got %d messages", len(app.messages))
	}
}

func TestSubmitStartsStreaming(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("why do this")

	cmd := app.submit()
	if cmd == nil {
		t.Fatal("expected streaming cmd")
	}
	if !app.streaming {
		t.Fatal("expected streaming true")
	}
	if got := app.messages[len(app.messages)-1]; got.role != "assistant" || !got.streaming {
		t.Fatalf("expected streaming assistant placeholder, got %#v", got)
	}
}

func TestDemoUsesSlowStreamDelay(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/demo")

	_ = app.submit()
	if app.streamDelay != 120*time.Millisecond {
		t.Fatalf("expected demo stream delay 120ms, got %v", app.streamDelay)
	}
}

func TestClearCommandResetsTranscript(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/clear")

	_ = app.submit()
	if len(app.messages) < 2 {
		t.Fatalf("expected reseeded transcript, got %d messages", len(app.messages))
	}
}

func TestUpdateHandlesStreamTick(t *testing.T) {
	app := New()
	app.messages = []message{{role: "assistant", content: "", streaming: true}}
	app.streaming = true

	model, _ := app.Update(streamTickMsg{part: "hello ", done: false})
	next := model.(App)
	if !strings.Contains(next.messages[0].content, "hello") {
		t.Fatalf("expected content to grow, got %q", next.messages[0].content)
	}
}

func TestSubmitWhileStreamingQueuesFollowUp(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.streaming = true
	app.messages = append(app.messages, message{role: "assistant", content: "working", streaming: true})
	app.input.SetValue("queued question")

	cmd := app.submit()
	if cmd != nil {
		t.Fatal("expected no immediate cmd when queuing")
	}
	if len(app.queuedSubmissions) != 1 {
		t.Fatalf("expected 1 queued submission, got %d", len(app.queuedSubmissions))
	}
	if app.queuedSubmissions[0].content != "queued question" {
		t.Fatalf("expected queued content preserved, got %q", app.queuedSubmissions[0].content)
	}
	if !strings.Contains(app.transcript.View(), "queued") || !strings.Contains(app.transcript.View(), "queued question") {
		t.Fatal("expected queued submission to remain visible in transcript")
	}
}

func TestDoneTickReleasesQueuedSubmission(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.streaming = true
	app.messages = []message{{role: "assistant", content: "", streaming: true}}
	app.queuedSubmissions = []queuedSubmission{{content: "queued question"}, {content: "queued question 2"}}

	model, cmd := app.Update(streamTickMsg{part: "done", done: true})
	next := model.(App)

	if cmd == nil {
		t.Fatal("expected follow-up cmd after releasing queued submission")
	}
	if len(next.queuedSubmissions) != 0 {
		t.Fatalf("expected queue drained, got %d", len(next.queuedSubmissions))
	}
	if len(next.pendingSubmissions) != 1 {
		t.Fatalf("expected one pending submission remaining, got %d", len(next.pendingSubmissions))
	}
	if len(next.messages) < 3 {
		t.Fatalf("expected queued submission appended, got %d messages", len(next.messages))
	}
	if got := next.messages[len(next.messages)-2].content; got != "queued question" {
		t.Fatalf("expected queued question submitted, got %q", got)
	}
	if !next.streaming {
		t.Fatal("expected streaming restarted for queued submission")
	}
}

func TestEscStopsActiveStream(t *testing.T) {
	app := New()
	app.streaming = true
	app.streamQueue = []string{"later"}
	app.messages = []message{{role: "assistant", content: "partial", streaming: true}}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	next := model.(App)

	if next.streaming {
		t.Fatal("expected streaming stopped")
	}
	if len(next.streamQueue) != 0 {
		t.Fatalf("expected stream queue cleared, got %d parts", len(next.streamQueue))
	}
	if next.messages[0].streaming {
		t.Fatal("expected assistant message marked not streaming")
	}
	if next.status != "Stream stopped" {
		t.Fatalf("expected stream stopped status, got %q", next.status)
	}
}

func TestTabCyclesFocus(t *testing.T) {
	app := New()
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := model.(App)
	if next.focus != focusTranscript {
		t.Fatalf("expected transcript focus, got %v", next.focus)
	}
}

func TestSidebarArrowNavigation(t *testing.T) {
	app := New()
	app.focus = focusSidebar

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := model.(App)
	if next.sidebarIndex != 1 {
		t.Fatalf("expected sidebar index 1, got %d", next.sidebarIndex)
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyUp})
	next = model.(App)
	if next.sidebarIndex != 0 {
		t.Fatalf("expected sidebar index 0, got %d", next.sidebarIndex)
	}
}

func TestSidebarNavigatesIntoSecondSection(t *testing.T) {
	app := New()
	app.focus = focusSidebar

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := model.(App)
	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next = model.(App)
	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next = model.(App)

	if next.sidebarItems[next.sidebarIndex].section != "Model" {
		t.Fatalf("expected to reach Model section, got %s", next.sidebarItems[next.sidebarIndex].section)
	}
}

func TestSidebarActionClearResetsTranscript(t *testing.T) {
	app := New()
	app.focus = focusSidebar
	app.messages = append(app.messages, message{role: "user", content: "extra"})
	app.sidebarIndex = 4 // Clear Transcript

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := model.(App)
	if next.dialog == nil {
		t.Fatal("expected choice dialog, got nil")
	}
	if len(next.messages) < 3 {
		t.Fatalf("expected transcript untouched before choice, got %d messages", len(next.messages))
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next = model.(App)
	if next.dialog != nil {
		t.Fatal("expected dialog cleared after confirm")
	}
	if len(next.messages) < 2 {
		t.Fatalf("expected reseeded transcript after clear, got %d messages", len(next.messages))
	}
}

func TestSidebarActionClearCancelKeepsTranscript(t *testing.T) {
	app := New()
	app.focus = focusSidebar
	app.messages = append(app.messages, message{role: "user", content: "extra"})
	app.sidebarIndex = 4 // Clear Transcript

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := model.(App)
	if next.dialog == nil {
		t.Fatal("expected choice dialog, got nil")
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next = model.(App)
	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next = model.(App)
	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next = model.(App)
	if next.dialog != nil {
		t.Fatal("expected dialog cleared after cancel choice")
	}
	if got := next.messages[len(next.messages)-1].content; got != "extra" {
		t.Fatalf("expected transcript preserved after cancel, got %q", got)
	}
}

func TestSidebarActionClearEmptyLeavesTranscriptEmpty(t *testing.T) {
	app := New()
	app.focus = focusSidebar
	app.messages = append(app.messages, message{role: "user", content: "extra"})
	app.sidebarIndex = 4 // Clear Transcript

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := model.(App)
	if next.dialog == nil {
		t.Fatal("expected choice dialog, got nil")
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next = model.(App)
	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next = model.(App)
	if next.dialog != nil {
		t.Fatal("expected dialog cleared after clear-empty choice")
	}
	if len(next.messages) != 0 {
		t.Fatalf("expected empty transcript, got %d messages", len(next.messages))
	}
}

func TestCtrlBTogglesSidebar(t *testing.T) {
	app := New()
	if !app.sidebarOpen {
		t.Fatal("expected sidebar open by default")
	}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	next := model.(App)
	if next.sidebarOpen {
		t.Fatal("expected sidebar closed after ctrl+b")
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	next = model.(App)
	if !next.sidebarOpen {
		t.Fatal("expected sidebar open after second ctrl+b")
	}
}

func TestLargePasteCreatesCompactToken(t *testing.T) {
	app := New()
	app.focus = focusInput

	large := strings.Repeat("line\n", 40)
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(large)})
	next := model.(App)

	if len(next.inputTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(next.inputTokens))
	}
	if next.inputTokens[0].kind != "paste" {
		t.Fatalf("expected paste token, got %q", next.inputTokens[0].kind)
	}
	if next.input.Value() != "" {
		t.Fatalf("expected textarea reset, got %q", next.input.Value())
	}
}

func TestDroppedPathCreatesFileToken(t *testing.T) {
	app := New()
	app.focus = focusInput

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/example.png")})
	next := model.(App)

	if len(next.inputTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(next.inputTokens))
	}
	if next.inputTokens[0].kind != "file" {
		t.Fatalf("expected file token, got %q", next.inputTokens[0].kind)
	}
	if !strings.Contains(next.inputTokens[0].label, "image-1") {
		t.Fatalf("expected image token label, got %q", next.inputTokens[0].label)
	}
}

func TestSlashCommandDoesNotBecomeFileToken(t *testing.T) {
	app := New()
	app.focus = focusInput

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/demo")})
	next := model.(App)

	if len(next.inputTokens) != 0 {
		t.Fatalf("expected no file token for slash command, got %d", len(next.inputTokens))
	}
	if next.input.Value() != "/demo" {
		t.Fatalf("expected slash command to remain in input, got %q", next.input.Value())
	}
}

func TestQuotedDroppedPathCreatesFileToken(t *testing.T) {
	app := New()
	app.focus = focusInput

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\"/tmp/My File.png\"")})
	next := model.(App)

	if len(next.inputTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(next.inputTokens))
	}
	if next.inputTokens[0].payload != "/tmp/My File.png" {
		t.Fatalf("expected normalized payload, got %q", next.inputTokens[0].payload)
	}
}

func TestEscapedDroppedPathCreatesFileToken(t *testing.T) {
	app := New()
	app.focus = focusInput

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/My\\ File.png")})
	next := model.(App)

	if len(next.inputTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(next.inputTokens))
	}
	if next.inputTokens[0].payload != "/tmp/My File.png" {
		t.Fatalf("expected normalized payload, got %q", next.inputTokens[0].payload)
	}
}

func TestCtrlOPreviewsSelectedToken(t *testing.T) {
	app := New()
	app.focus = focusInput
	app.inputTokens = []inputToken{{id: "paste-1", kind: "paste", label: "paste-1", payload: "hello\nworld"}}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	next := model.(App)

	if next.preview == nil {
		t.Fatal("expected preview dialog")
	}
	if next.preview.title != "paste-1" {
		t.Fatalf("expected preview title paste-1, got %q", next.preview.title)
	}
}

func TestSidebarCloseExpandsMainArea(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	withSidebar := app.transcript.Width

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	next := model.(App)
	withoutSidebar := next.transcript.Width

	if withoutSidebar <= withSidebar {
		t.Fatalf("expected transcript width to grow when sidebar closes, got %d -> %d", withSidebar, withoutSidebar)
	}
}

func TestCtrlTOpensAgentListWhenMultipleAgents(t *testing.T) {
	app := New()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	next := model.(App)

	if next.agentList == nil {
		t.Fatal("expected agent list to open")
	}
	if next.agentDetail != nil {
		t.Fatal("expected no direct agent detail when multiple agents exist")
	}
}

func TestAgentListEnterOpensAgentDetail(t *testing.T) {
	app := New()
	app.openAgents()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := model.(App)

	if next.agentDetail == nil {
		t.Fatal("expected agent detail to open")
	}
	if next.agentDetail.agentID != "agent-1" {
		t.Fatalf("expected agent-1 detail, got %q", next.agentDetail.agentID)
	}
}

func TestCtrlTOpensAgentDetailDirectlyWhenSingleAgent(t *testing.T) {
	app := New()
	app.agents = app.agents[:1]

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	next := model.(App)

	if next.agentDetail == nil {
		t.Fatal("expected agent detail to open directly")
	}
	if next.agentList != nil {
		t.Fatal("expected no agent list when only one agent exists")
	}
}

func TestAgentDetailEscReturnsToListForMultipleAgents(t *testing.T) {
	app := New()
	app.openAgents()
	app.agentDetail = &agentDetailDialog{agentID: "agent-2"}
	app.agentList = nil

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	next := model.(App)

	if next.agentDetail != nil {
		t.Fatal("expected agent detail closed")
	}
	if next.agentList == nil {
		t.Fatal("expected to return to agent list")
	}
	if next.agentList.index != 1 {
		t.Fatalf("expected list to reselect second agent, got %d", next.agentList.index)
	}
}

func TestCtrlKOpensCommandPalette(t *testing.T) {
	app := New()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	next := model.(App)

	if next.palette == nil {
		t.Fatal("expected command palette to open")
	}
}

func TestPaletteFiltersCommands(t *testing.T) {
	app := New()
	app.openPalette()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("dummy")})
	next := model.(App)
	commands := next.filteredCommands()
	if len(commands) != 1 {
		t.Fatalf("expected 1 filtered command, got %d", len(commands))
	}
	if commands[0].value != "Dummy Stream" {
		t.Fatalf("expected Dummy Stream command, got %q", commands[0].value)
	}
}

func TestPaletteCanSwitchSession(t *testing.T) {
	app := New()
	app.openPalette()
	app.palette.query = "dummy"

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := model.(App)

	if next.currentSession != "Dummy Stream" {
		t.Fatalf("expected Dummy Stream session, got %q", next.currentSession)
	}
	if next.palette != nil {
		t.Fatal("expected palette to close after running command")
	}
}

func TestSubmitPreservesStructuredTokensInTranscript(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("review this")
	app.inputTokens = []inputToken{{id: "paste-1", kind: "paste", label: "paste-1", payload: "a\nb\nc\nd"}}

	_ = app.submit()

	user := app.messages[len(app.messages)-2]
	if user.role != "user" {
		t.Fatalf("expected user message, got %q", user.role)
	}
	if len(user.tokens) != 1 {
		t.Fatalf("expected 1 structured token, got %d", len(user.tokens))
	}
	if user.content != "review this" {
		t.Fatalf("expected plain user content preserved, got %q", user.content)
	}
	if !strings.Contains(app.transcript.View(), "paste-1") {
		t.Fatal("expected transcript to render submitted token label")
	}
}

func TestTranscriptEnterOpensSubmittedTokenPreview(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.focus = focusTranscript
	app.messages = append(app.messages, message{
		role:    "user",
		content: "review this",
		tokens: []inputToken{
			{id: "paste-1", kind: "paste", label: "paste-1", payload: "hello\nworld"},
		},
		expanded: map[int]bool{},
	})
	app.refreshTranscript()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := model.(App)

	if next.preview != nil {
		t.Fatal("expected inline toggle for paste token, not preview modal")
	}
	if !next.messages[len(next.messages)-1].expanded[0] {
		t.Fatal("expected paste token to expand inline")
	}
}

func TestTranscriptCanSelectSecondSubmittedToken(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.focus = focusTranscript
	app.messages = append(app.messages, message{
		role:    "user",
		content: "review these",
		tokens: []inputToken{
			{id: "paste-1", kind: "paste", label: "paste-1", payload: "hello"},
			{id: "file-2", kind: "file", label: "file-2 (a.txt)", payload: "/tmp/a.txt"},
		},
	})
	app.refreshTranscript()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := model.(App)

	if next.selectedTranscriptRef != 1 {
		t.Fatalf("expected second transcript ref selected, got %d", next.selectedTranscriptRef)
	}
}

func TestSubmittedPasteStartsExpandedInTranscript(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.inputTokens = []inputToken{{id: "paste-1", kind: "paste", label: "paste-1", payload: "alpha\nbeta\ngamma"}}

	_ = app.submit()

	user := app.messages[len(app.messages)-2]
	if !user.expanded[0] {
		t.Fatal("expected submitted paste to start expanded")
	}
	if !strings.Contains(app.transcript.View(), "alpha") {
		t.Fatal("expected transcript to include expanded paste content")
	}
}
