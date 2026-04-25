package harnessspike

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewStartsWithEmptyTranscript(t *testing.T) {
	app := New()
	if len(app.messages) != 0 {
		t.Fatalf("expected empty transcript on startup, got %d messages", len(app.messages))
	}
	if app.focus != focusInput {
		t.Fatalf("expected input focus by default, got %v", app.focus)
	}
	if app.sidebarOpen {
		t.Fatal("expected sidebar closed by default")
	}
	if app.input.ShowLineNumbers {
		t.Fatal("expected input line numbers disabled")
	}
	if app.input.Height() != 1 {
		t.Fatalf("expected single-line input by default, got %d", app.input.Height())
	}
	if !app.transcript.MouseWheelEnabled {
		t.Fatal("expected transcript mouse wheel enabled")
	}
}

func TestComposerRendersInsideTranscriptSurface(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()

	if !strings.Contains(app.transcript.View(), app.input.Placeholder) {
		t.Fatalf("expected composer input inside transcript surface, got %q", app.transcript.View())
	}
	if strings.Contains(app.transcript.View(), "compose") {
		t.Fatalf("expected composer label removed, got %q", app.transcript.View())
	}
	if app.input.Height() != 1 {
		t.Fatalf("expected single-line input after layout, got %d", app.input.Height())
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
	if app.focus != focusInput {
		t.Fatalf("expected input focus preserved after submit, got %v", app.focus)
	}
	if got := app.messages[len(app.messages)-1]; got.role != "assistant" || !got.streaming {
		t.Fatalf("expected streaming assistant placeholder, got %#v", got)
	}
}

func TestAltEnterInsertsNewlineInInput(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("first line")

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	next := model.(App)

	if got := next.input.Value(); got != "first line\n" {
		t.Fatalf("expected alt+enter to insert newline, got %q", got)
	}
}

func TestMouseScrollDoesNotStealInputFocus(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()

	model, _ := app.Update(tea.MouseMsg{})
	next := model.(App)

	if next.focus != focusInput {
		t.Fatalf("expected input focus preserved after mouse scroll/update, got %v", next.focus)
	}
}

func TestDemoUsesSlowStreamDelay(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/demo")

	_ = app.submit()
	if app.streamDelay != 500*time.Millisecond {
		t.Fatalf("expected demo stream delay 500ms, got %v", app.streamDelay)
	}
	if len(app.streamQueue) < 100 {
		t.Fatalf("expected long demo stream queue, got %d chunks", len(app.streamQueue))
	}
}

func TestClearCommandResetsTranscript(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.messages = append(app.messages, message{role: "user", content: "before clear"})
	app.input.SetValue("/clear")

	_ = app.submit()
	if len(app.messages) != 0 {
		t.Fatalf("expected empty transcript after clear, got %d messages", len(app.messages))
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
	if len(next.pendingSubmissions) != 0 {
		t.Fatalf("expected pending queue drained, got %d", len(next.pendingSubmissions))
	}
	if len(next.messages) < 3 {
		t.Fatalf("expected queued submission appended, got %d messages", len(next.messages))
	}
	if got := next.messages[len(next.messages)-2].content; got != "queued question\n\nqueued question 2" {
		t.Fatalf("expected merged queued content, got %q", got)
	}
	if !next.streaming {
		t.Fatal("expected streaming restarted for queued submission")
	}
}

func TestSubmitAfterStopWithBacklogPreservesQueueOrder(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.queuedSubmissions = []queuedSubmission{{content: "first queued"}, {content: "second queued"}}
	app.input.SetValue("newest message")

	cmd := app.submit()
	if cmd == nil {
		t.Fatal("expected queued release to start")
	}
	if len(app.queuedSubmissions) != 0 {
		t.Fatalf("expected visible queue drained into pending, got %d queued", len(app.queuedSubmissions))
	}
	if len(app.pendingSubmissions) != 0 {
		t.Fatalf("expected pending queue drained into merged submission, got %d", len(app.pendingSubmissions))
	}
	if got := app.messages[len(app.messages)-2].content; got != "first queued\n\nsecond queued\n\nnewest message" {
		t.Fatalf("expected merged queued messages in FIFO order, got %q", got)
	}
	if len(app.messages[len(app.messages)-2].entries) != 3 {
		t.Fatalf("expected 3 separate merged entries, got %d", len(app.messages[len(app.messages)-2].entries))
	}
}

func TestEmptySubmitReleasesQueuedWorkWhenIdle(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.queuedSubmissions = []queuedSubmission{{content: "first queued"}, {content: "second queued"}}

	cmd := app.submit()
	if cmd == nil {
		t.Fatal("expected empty submit to release queued work when idle")
	}
	if len(app.queuedSubmissions) != 0 {
		t.Fatalf("expected queued submissions drained, got %d", len(app.queuedSubmissions))
	}
	if len(app.pendingSubmissions) != 0 {
		t.Fatalf("expected pending submissions drained, got %d", len(app.pendingSubmissions))
	}
	if len(app.messages) < 2 {
		t.Fatalf("expected released queued submission appended, got %d messages", len(app.messages))
	}
	if got := app.messages[len(app.messages)-2].content; got != "first queued\n\nsecond queued" {
		t.Fatalf("expected merged queued content, got %q", got)
	}
	if !app.streaming {
		t.Fatal("expected streaming restarted for released queued work")
	}
}

func TestToolDemoSessionRendersEvents(t *testing.T) {
	app := New()
	app.seedWithSession("Tool Demo")
	app.width = 120
	app.height = 40
	app.layout()

	if !strings.Contains(app.transcript.View(), "Permission required") {
		t.Fatalf("expected permission event in transcript, got %q", app.transcript.View())
	}
}

func TestEscStopsActiveStream(t *testing.T) {
	app := New()
	app.streaming = true
	app.streamQueue = []string{"later"}
	app.messages = []message{{role: "assistant", content: "partial", streaming: true}}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	next := model.(App)

	if !next.interruptArmed {
		t.Fatal("expected first escape to arm interrupt")
	}
	if next.streaming == false {
		t.Fatal("expected streaming to continue after first escape")
	}
	if next.status != "Press Esc again to interrupt" {
		t.Fatalf("expected interrupt warning, got %q", next.status)
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyEsc})
	next = model.(App)

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

func TestStreamPulseAdvancesWorkingIndicator(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.streaming = true
	app.messages = []message{{role: "assistant", content: "partial", streaming: true}}
	app.refreshTranscript()

	model, cmd := app.Update(streamPulseMsg{})
	next := model.(App)

	if cmd == nil {
		t.Fatal("expected next pulse cmd")
	}
	if next.streamPulse != 1 {
		t.Fatalf("expected pulse to advance, got %d", next.streamPulse)
	}
	if !strings.Contains(next.transcript.View(), "working.") {
		t.Fatalf("expected working indicator in transcript, got %q", next.transcript.View())
	}
}

func TestTabCyclesFocus(t *testing.T) {
	app := New()
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	next := model.(App)
	if next.focus != focusTranscript {
		t.Fatalf("expected transcript focus, got %v", next.focus)
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyTab})
	next = model.(App)
	if next.focus != focusInput {
		t.Fatalf("expected input focus when sidebar is closed, got %v", next.focus)
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
	if len(next.messages) < 1 {
		t.Fatalf("expected transcript untouched before choice, got %d messages", len(next.messages))
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next = model.(App)
	if next.dialog != nil {
		t.Fatal("expected dialog cleared after confirm")
	}
	if len(next.messages) != 0 {
		t.Fatalf("expected empty transcript after clear, got %d messages", len(next.messages))
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
	if app.sidebarOpen {
		t.Fatal("expected sidebar closed by default")
	}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	next := model.(App)
	if !next.sidebarOpen {
		t.Fatal("expected sidebar open after ctrl+b")
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	next = model.(App)
	if next.sidebarOpen {
		t.Fatal("expected sidebar closed after second ctrl+b")
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
	app.sidebarOpen = true
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

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	next := model.(App)

	if next.selectedTranscriptRef != 1 {
		t.Fatalf("expected second transcript ref selected, got %d", next.selectedTranscriptRef)
	}
}

func TestRefreshTranscriptPreservesScrollOffsetWhenNotAtBottom(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 20
	app.layout()
	app.focus = focusTranscript
	var msgs []message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, message{role: "assistant", content: "line"})
	}
	app.messages = msgs
	app.refreshTranscript()
	app.transcript.SetYOffset(3)

	app.messages = append(app.messages, message{role: "assistant", content: "new line"})
	app.refreshTranscript()

	if app.transcript.YOffset != 3 {
		t.Fatalf("expected scroll offset preserved, got %d", app.transcript.YOffset)
	}
}

func TestRefreshTranscriptPreservesScrollOffsetWithInputFocus(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 20
	app.layout()
	app.focus = focusInput
	var msgs []message
	for i := 0; i < 20; i++ {
		msgs = append(msgs, message{role: "assistant", content: "line"})
	}
	app.messages = msgs
	app.refreshTranscript()
	app.transcript.SetYOffset(3)

	app.input.SetValue("typing while scrolled up")
	app.refreshTranscript()

	if app.transcript.YOffset != 3 {
		t.Fatalf("expected scroll offset preserved with input focus, got %d", app.transcript.YOffset)
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

func TestInputArrowHistoryRecallAndRestoreDraft(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()

	app.input.SetValue("first")
	_ = app.submit()
	app.input.SetValue("second")
	_ = app.submit()

	app.input.SetValue("draft")

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyUp})
	next := model.(App)
	if got := next.input.Value(); got != "second" {
		t.Fatalf("expected most-recent history on up, got %q", got)
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyUp})
	next = model.(App)
	if got := next.input.Value(); got != "first" {
		t.Fatalf("expected older history on second up, got %q", got)
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyUp})
	next = model.(App)
	if got := next.input.Value(); got != "first" {
		t.Fatalf("expected to stay at oldest history, got %q", got)
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next = model.(App)
	if got := next.input.Value(); got != "second" {
		t.Fatalf("expected forward history on down, got %q", got)
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next = model.(App)
	if got := next.input.Value(); got != "draft" {
		t.Fatalf("expected draft restored past newest, got %q", got)
	}

	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyDown})
	next = model.(App)
	if got := next.input.Value(); got != "draft" {
		t.Fatalf("expected draft to remain when down pressed without browsing, got %q", got)
	}
}

func TestInputArrowDownWithoutBrowsingDoesNothing(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("typed")

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	next := model.(App)

	if got := next.input.Value(); got != "typed" {
		t.Fatalf("expected input untouched when not browsing, got %q", got)
	}
}

func TestConsecutiveDuplicateSubmissionsStoredOnce(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()

	app.input.SetValue("same")
	_ = app.submit()
	app.input.SetValue("same")
	_ = app.submit()

	if got := len(app.commandHistory); got != 1 {
		t.Fatalf("expected consecutive duplicates collapsed, got %d entries", got)
	}
}

func TestPermCommandTriggersPendingPermission(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/perm")

	_ = app.submit()

	if app.currentPendingPermission() == nil {
		t.Fatal("expected pending permission after /perm")
	}
	if app.streaming {
		t.Fatal("expected no stream while awaiting permission")
	}
	last := app.messages[len(app.messages)-1]
	if last.role != "event" || last.eventState != "permission" {
		t.Fatalf("expected last message to be a permission event, got %+v", last)
	}
	if !strings.Contains(app.transcript.View(), "Permission required") {
		t.Fatal("expected transcript to retain the permission request")
	}
	if !strings.Contains(app.transcript.View(), "Approve once") || !strings.Contains(app.transcript.View(), "Allow for session") || !strings.Contains(app.transcript.View(), "Deny") {
		t.Fatal("expected composer action list for permission controls")
	}
	if !strings.Contains(app.transcript.View(), "workspace/README.md") || !strings.Contains(app.transcript.View(), "Read a workspace file") {
		t.Fatal("expected composer to show permission target details")
	}
}

func TestPermGrantContinuesWithToolAndStream(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/perm")
	_ = app.submit()

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	next := model.(App)

	if next.currentPendingPermission() != nil {
		t.Fatal("expected pending permission cleared after grant")
	}
	if cmd == nil {
		t.Fatal("expected stream cmd after grant")
	}
	if !next.streaming {
		t.Fatal("expected streaming after grant")
	}
	// Walk messages after the user message to find granted + running + done + assistant.
	var grantedFound, runningFound, doneFound, assistantStreaming bool
	for _, m := range next.messages {
		if m.role == "event" && m.eventState == "granted" {
			grantedFound = true
		}
		if m.role == "event" && m.eventState == "running" {
			runningFound = true
		}
		if m.role == "event" && m.eventState == "done" {
			doneFound = true
		}
		if m.role == "assistant" && m.streaming {
			assistantStreaming = true
		}
	}
	if !grantedFound || !runningFound || !doneFound || !assistantStreaming {
		t.Fatalf("expected granted+running+done+streaming assistant, got granted=%v running=%v done=%v streaming=%v", grantedFound, runningFound, doneFound, assistantStreaming)
	}
}

func TestPermDenyShortCircuitsWithoutStream(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/perm")
	_ = app.submit()

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	next := model.(App)

	if next.currentPendingPermission() != nil {
		t.Fatal("expected pending permission cleared after deny")
	}
	if cmd != nil {
		t.Fatal("expected no stream cmd after deny")
	}
	if next.streaming {
		t.Fatal("expected no streaming after deny")
	}
	var deniedFound, hasRunningOrDone bool
	for _, m := range next.messages {
		if m.role == "event" && m.eventState == "denied" {
			deniedFound = true
		}
		if m.role == "event" && (m.eventState == "running" || m.eventState == "done") {
			hasRunningOrDone = true
		}
	}
	if !deniedFound {
		t.Fatal("expected a denied event")
	}
	if hasRunningOrDone {
		t.Fatal("expected no tool running/done events after deny")
	}
	if next.messages[len(next.messages)-1].role != "assistant" {
		t.Fatal("expected a trailing assistant message after deny")
	}
}

func TestYKeyDoesNotTriggerGrantWhenInputNonEmpty(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/perm")
	_ = app.submit()
	app.input.SetValue("yes please")

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	next := model.(App)

	if next.currentPendingPermission() == nil {
		t.Fatal("expected permission still pending when user is typing")
	}
}

func TestInputAreaCanApprovePermissionWithEnter(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/perm")
	_ = app.submit()

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := model.(App)
	if next.currentPendingPermission() != nil {
		t.Fatal("expected pending permission cleared after composer approval")
	}
	if cmd == nil || !next.streaming {
		t.Fatal("expected composer approval to start streaming")
	}
}

func TestInputAreaCanMoveAcrossPermissionActions(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/perm")
	_ = app.submit()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := model.(App)
	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyRight})
	next = model.(App)
	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyLeft})
	next = model.(App)

	if next.currentPendingPermission() == nil {
		t.Fatal("expected permission to remain pending while moving action selection")
	}
	if next.currentPendingPermission().selectedAction != 1 {
		t.Fatalf("expected selected action index 1, got %d", next.currentPendingPermission().selectedAction)
	}
}

func TestAllowForSessionPersistsWithoutAutoAnsweringLaterPermRequests(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/perm")
	_ = app.submit()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := model.(App)
	model, cmd := next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next = model.(App)

	if cmd == nil || !next.streaming {
		t.Fatal("expected allow-for-session to begin streaming")
	}
	if !next.sessionAllowedTools["Read workspace/README.md"] {
		t.Fatal("expected session policy recorded for tool")
	}

	next.stopStreaming()
	next.input.SetValue("/perm")
	cmd = next.submit()

	if cmd != nil {
		t.Fatal("expected repeated /perm to remain interactive, not auto-continue")
	}
	if next.currentPendingPermission() == nil {
		t.Fatal("expected repeated /perm to surface a fresh pending permission")
	}
	if !strings.Contains(next.transcript.View(), "session policy active for this tool") {
		t.Fatal("expected repeated /perm to show the persisted session-policy hint")
	}
}

func TestInputAreaCanDeny(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/perm")
	_ = app.submit()

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	next := model.(App)
	model, _ = next.Update(tea.KeyMsg{Type: tea.KeyRight})
	next = model.(App)

	model, cmd := next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next = model.(App)
	if cmd != nil {
		t.Fatal("expected no stream when denying")
	}
	if next.currentPendingPermission() != nil {
		t.Fatal("expected pending permission cleared after deny")
	}
	if got := next.messages[len(next.messages)-1].content; !strings.Contains(got, "Read request denied") {
		t.Fatalf("expected deny assistant note, got %q", got)
	}
}

func TestPermCommandCanQueueMultiplePendingPermissions(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()

	app.input.SetValue("/perm")
	_ = app.submit()
	app.input.SetValue("/perm")
	_ = app.submit()

	if got := len(app.pendingPermissions); got != 2 {
		t.Fatalf("expected 2 pending permissions, got %d", got)
	}
	if !strings.Contains(app.transcript.View(), "pending  2 of 2") {
		t.Fatal("expected composer to show active pending permission position")
	}

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyUp})
	next := model.(App)
	if next.activePermissionIndex != 0 {
		t.Fatalf("expected to move to first pending permission, got index %d", next.activePermissionIndex)
	}
	if !strings.Contains(next.transcript.View(), "pending  1 of 2") {
		t.Fatal("expected composer to update pending permission position after navigation")
	}
}

func TestPermDuringStreamingPausesAndWaitsForApproval(t *testing.T) {
	app := New()
	app.width = 120
	app.height = 40
	app.layout()
	app.input.SetValue("/demo")
	cmd := app.submit()
	if cmd == nil || !app.streaming {
		t.Fatal("expected /demo to begin streaming")
	}

	app.input.SetValue("/perm")
	cmd = app.submit()

	if cmd != nil {
		t.Fatal("expected /perm during streaming to pause for approval, not stream immediately")
	}
	if app.streaming {
		t.Fatal("expected streaming paused while permission is pending")
	}
	if app.pausedResponse == nil || app.pausedResponse.remaining == "" {
		t.Fatal("expected paused response state captured for later resume")
	}
	if app.currentPendingPermission() == nil {
		t.Fatal("expected pending permission after mid-stream /perm")
	}

	model, resume := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	next := model.(App)
	if resume == nil || !next.streaming {
		t.Fatal("expected approval to resume streaming after mid-stream pause")
	}
}
