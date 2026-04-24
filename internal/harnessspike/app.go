package harnessspike

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focusZone int

const (
	focusSidebar focusZone = iota
	focusTranscript
	focusInput
)

type message struct {
	role       string
	content    string
	streaming  bool
	tokens     []inputToken
	expanded   map[int]bool
	entries    []string
	eventState string
}

type inputToken struct {
	id      string
	kind    string
	label   string
	payload string
}

type sidebarItemKind int

const (
	sidebarItemSession sidebarItemKind = iota
	sidebarItemModel
	sidebarItemAction
)

type sidebarItem struct {
	section string
	label   string
	kind    sidebarItemKind
	value   string
}

type streamTickMsg struct {
	part string
	done bool
}

type streamPulseMsg struct{}

type choiceOption struct {
	label string
	value string
}

type choiceDialog struct {
	title   string
	prompt  string
	options []choiceOption
	index   int
}

type previewDialog struct {
	title   string
	content string
}

type backgroundAgent struct {
	id      string
	name    string
	status  string
	summary string
	stream  []string
}

type agentListDialog struct {
	index int
}

type agentDetailDialog struct {
	agentID string
}

type commandPalette struct {
	query string
	index int
}

type paletteCommand struct {
	label  string
	value  string
	kind   string
	filter string
}

type transcriptRef struct {
	messageIndex int
	tokenIndex   int
}

type queuedSubmission struct {
	content string
	tokens  []inputToken
	entries []string
}

type App struct {
	width  int
	height int

	focus focusZone

	input      textarea.Model
	transcript viewport.Model

	modelName string
	status    string
	messages  []message

	streamQueue    []string
	streaming      bool
	streamDelay    time.Duration
	streamPulse    int
	interruptArmed bool

	sidebarItems []sidebarItem
	sidebarIndex int

	currentSession string
	dialog         *choiceDialog
	preview        *previewDialog
	agentList      *agentListDialog
	agentDetail    *agentDetailDialog
	palette        *commandPalette
	sidebarOpen    bool

	inputTokens           []inputToken
	selectedToken         int
	agents                []backgroundAgent
	transcriptRefs        []transcriptRef
	selectedTranscriptRef int
	queuedSubmissions     []queuedSubmission
	pendingSubmissions    []queuedSubmission

	commandHistory []string
	historyIndex   int
	historyDraft   string
}

func New() App {
	ta := textarea.New()
	ta.Placeholder = "Ask something. Enter sends. Alt+Enter or Ctrl+J inserts a newline. /clear wipes the transcript."
	ta.Prompt = "▎ "
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetHeight(1)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6F86A3"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6F86A3"))
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF")).Bold(true)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#4A5668"))
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5F7FB"))
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF"))
	ta.Focus()

	app := App{
		focus:          focusInput,
		input:          ta,
		transcript:     viewport.New(0, 0),
		modelName:      "fake-kimi-spike",
		status:         "Ready",
		currentSession: "Spike Session",
		sidebarOpen:    false,
		streamDelay:    35 * time.Millisecond,
		historyIndex:   -1,
		sidebarItems: []sidebarItem{
			{section: "Session", label: "Spike Session", kind: sidebarItemSession, value: "spike-session"},
			{section: "Session", label: "Reference Layout", kind: sidebarItemSession, value: "reference-layout"},
			{section: "Session", label: "Dummy Stream", kind: sidebarItemSession, value: "dummy-stream"},
			{section: "Model", label: "fake-kimi-spike", kind: sidebarItemModel, value: "fake-kimi-spike"},
			{section: "Actions", label: "Clear Transcript", kind: sidebarItemAction, value: "clear"},
			{section: "Actions", label: "Replay Intro", kind: sidebarItemAction, value: "replay"},
			{section: "Actions", label: "Replay Tool Demo", kind: sidebarItemAction, value: "tool-demo"},
		},
		agents: []backgroundAgent{
			{
				id:      "agent-1",
				name:    "Patch planning",
				status:  "running",
				summary: "Summarizing input-model edge cases and likely follow-up work.",
				stream: []string{
					"Scanning current spike behavior.",
					"Comparing paste, token, and modal flows.",
					"Drafting cleanup notes for the next implementation pass.",
				},
			},
			{
				id:      "agent-2",
				name:    "Reference review",
				status:  "running",
				summary: "Collecting examples for transcript chrome and background-task affordances.",
				stream: []string{
					"Reviewing transcript hierarchy patterns.",
					"Checking how secondary task streams are surfaced.",
					"Preparing concise recommendations for the footer model.",
				},
			},
		},
	}
	app.transcript.MouseWheelEnabled = true
	app.transcript.MouseWheelDelta = 3
	app.seed()
	return app
}

func (a App) Init() tea.Cmd { return textarea.Blink }

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.layout()
		return a, nil

	case tea.MouseMsg:
		var cmd tea.Cmd
		a.transcript, cmd = a.transcript.Update(msg)
		return a, cmd

	case tea.KeyMsg:
		if a.dialog != nil {
			return a, a.handleDialogKey(msg)
		}
		if a.preview != nil {
			return a, a.handlePreviewKey(msg)
		}
		if a.palette != nil {
			return a, a.handlePaletteKey(msg)
		}
		if a.agentDetail != nil {
			return a, a.handleAgentDetailKey(msg)
		}
		if a.agentList != nil {
			return a, a.handleAgentListKey(msg)
		}

		switch {
		case msg.Type == tea.KeyCtrlC:
			return a, tea.Quit
		case msg.Type == tea.KeyEsc && a.streaming:
			if a.interruptArmed {
				a.stopStreaming()
			} else {
				a.interruptArmed = true
				a.status = "Press Esc again to interrupt"
				a.refreshTranscript()
			}
			return a, nil
		case msg.Type == tea.KeyTab:
			a.focus = nextFocus(a.focus, a.sidebarOpen)
			if a.focus == focusInput {
				a.input.Focus()
			} else {
				a.input.Blur()
			}
			return a, nil
		case msg.Type == tea.KeyEnter && msg.Alt && a.focus == focusInput:
			a.input.InsertRune('\n')
			a.syncInputHeight()
			a.refreshTranscript()
			return a, nil
		case msg.Type == tea.KeyEnter && a.focus == focusInput:
			cmd := a.submit()
			return a, cmd
		case msg.Type == tea.KeyCtrlJ && a.focus == focusInput:
			a.input.InsertRune('\n')
			a.syncInputHeight()
			a.refreshTranscript()
			return a, nil
		case msg.Type == tea.KeyUp && a.focus == focusInput && !strings.Contains(a.input.Value(), "\n"):
			a.recallPreviousCommand()
			return a, nil
		case msg.Type == tea.KeyDown && a.focus == focusInput && !strings.Contains(a.input.Value(), "\n"):
			a.recallNextCommand()
			return a, nil
		case strings.ToLower(msg.String()) == "ctrl+b":
			a.sidebarOpen = !a.sidebarOpen
			if !a.sidebarOpen && a.focus == focusSidebar {
				a.focus = focusTranscript
			}
			if a.sidebarOpen {
				a.status = "Sidebar opened"
			} else {
				a.status = "Sidebar closed"
			}
			a.layout()
			return a, nil
		case strings.ToLower(msg.String()) == "ctrl+o":
			if a.focus == focusInput && len(a.inputTokens) > 0 {
				a.openSelectedTokenPreview()
				return a, nil
			}
		case msg.Type == tea.KeyCtrlK:
			a.openPalette()
			return a, nil
		case msg.Type == tea.KeyCtrlT:
			a.openAgents()
			return a, nil
		case strings.ToLower(msg.String()) == "ctrl+n":
			if a.focus == focusInput && len(a.inputTokens) > 0 {
				a.moveTokenSelection(1)
				return a, nil
			}
		case strings.ToLower(msg.String()) == "ctrl+p":
			if a.focus == focusInput && len(a.inputTokens) > 0 {
				a.moveTokenSelection(-1)
				return a, nil
			}
		}

		switch a.focus {
		case focusSidebar:
			switch msg.Type {
			case tea.KeyUp:
				a.moveSidebar(-1)
				return a, nil
			case tea.KeyDown:
				a.moveSidebar(1)
				return a, nil
			}
			switch msg.String() {
			case "k":
				a.moveSidebar(-1)
				return a, nil
			case "j":
				a.moveSidebar(1)
				return a, nil
			case "enter":
				a.activateSidebar()
				return a, nil
			}
		case focusTranscript:
			switch msg.Type {
			case tea.KeyEnter:
				if len(a.transcriptRefs) > 0 {
					a.openSelectedTranscriptRef()
					return a, nil
				}
			}
			switch msg.String() {
			case "k":
				if len(a.transcriptRefs) > 0 {
					a.moveTranscriptRef(-1)
					return a, nil
				}
			case "j":
				if len(a.transcriptRefs) > 0 {
					a.moveTranscriptRef(1)
					return a, nil
				}
			}
			var cmd tea.Cmd
			a.transcript, cmd = a.transcript.Update(msg)
			return a, cmd
		case focusInput:
			prev := a.input.Value()
			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			if a.input.Value() != prev {
				a.historyIndex = -1
			}
			a.handleInputMutation(prev, a.input.Value())
			a.syncInputHeight()
			a.refreshTranscript()
			return a, cmd
		default:
			return a, nil
		}

	case streamTickMsg:
		if len(a.messages) == 0 || !a.streaming {
			return a, nil
		}
		last := &a.messages[len(a.messages)-1]
		last.content += msg.part
		last.streaming = !msg.done
		a.status = "Streaming fake response"
		if msg.done {
			a.streaming = false
			a.interruptArmed = false
			if len(a.queuedSubmissions) == 0 && len(a.pendingSubmissions) == 0 {
				a.status = "Ready"
			}
		}
		a.refreshTranscript()
		if msg.done {
			if len(a.queuedSubmissions) > 0 || len(a.pendingSubmissions) > 0 {
				cmd := a.releaseQueuedSubmission()
				return a, cmd
			}
			return a, nil
		}
		return a, a.nextStreamCmd()

	case streamPulseMsg:
		if !a.streaming {
			return a, nil
		}
		a.streamPulse = (a.streamPulse + 1) % 4
		a.refreshTranscript()
		return a, a.nextPulseCmd()
	}

	return a, nil
}

func (a App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Loading spike harness..."
	}

	sidebar := a.renderSidebar()
	main := a.renderMain()
	content := main
	if a.sidebarOpen {
		content = lipgloss.JoinHorizontal(lipgloss.Top, main, sidebar)
	}
	if a.agentList != nil || a.agentDetail != nil || a.palette != nil {
		return a.renderOverlay()
	}
	if a.dialog == nil && a.preview == nil && a.agentList == nil && a.agentDetail == nil && a.palette == nil {
		return content
	}
	dimmed := modalScrimStyle.Render(content)
	overlay := a.renderOverlay()
	x := max((a.width-lipgloss.Width(overlay))/2, 0)
	y := max((a.height-lipgloss.Height(overlay))/2, 0)
	return overlayString(dimmed, overlay, x, y)
}

func (a *App) seed() {
	a.messages = sessionPreset(a.currentSession)
	a.refreshTranscript()
}

func sessionPreset(name string) []message {
	switch name {
	case "Tool Demo":
		return []message{
			{role: "system", content: "Tool demo mode. Use this preset to judge how inline tool and permission events read inside the transcript."},
			{role: "user", content: "Summarize the README and tell me what changed in the spike."},
			{role: "event", content: "Read README.md", eventState: "requested"},
			{role: "event", content: "Permission required to read workspace file", eventState: "permission"},
			{role: "event", content: "Read README.md", eventState: "running"},
			{role: "event", content: "Read README.md", eventState: "done"},
			{role: "assistant", content: "The spike is evaluating a replacement harness shell with better transcript structure, queue handling, and interrupt behavior."},
		}
	case "Reference Layout":
		return []message{
			{
				role: "system",
				content: "Layout reference mode. Use this preset to judge spacing, hierarchy, and scanning speed " +
					"without any streaming noise.",
			},
			{
				role:    "assistant",
				content: "The sidebar should feel like navigation. The main panel should feel like work. This preset is tuned for reading, not interaction.",
			},
			{
				role:    "user",
				content: "Show me what a stable transcript looks like.",
			},
			{
				role:    "assistant",
				content: "It should have clear grouping, durable headings, enough whitespace to scan, and no jitter when messages update.",
			},
		}
	case "Dummy Stream":
		return []message{
			{
				role: "system",
				content: "Dummy stream mode. Use this preset to test how the transcript behaves while tokens arrive, " +
					"and whether the shell still feels coherent under motion.",
			},
			{
				role:    "assistant",
				content: "Ask any question and the fake backend will stream a reply. Watch how the viewport, header, and input hold together.",
			},
		}
	default:
		return []message{
			{
				role: "system",
				content: "Spike shell only. No real backend, no tools, no sessions persistence. " +
					"This is for evaluating layout, focus, and transcript behavior.",
			},
			{
				role:    "assistant",
				content: "Try /demo for a long stream, queue a follow-up while it runs, or open Tool Demo to judge inline event rendering.",
			},
		}
	}
}

func (a *App) seedWithSession(name string) {
	a.currentSession = name
	a.messages = sessionPreset(name)
	a.refreshTranscript()
}

func (a *App) syncInputHeight() {
	lines := strings.Count(a.input.Value(), "\n") + 1
	if lines < 1 {
		lines = 1
	}
	a.input.SetHeight(lines)
}

func (a *App) pushHistory(content string) {
	a.historyIndex = -1
	a.historyDraft = ""
	if content == "" {
		return
	}
	if n := len(a.commandHistory); n > 0 && a.commandHistory[n-1] == content {
		return
	}
	a.commandHistory = append(a.commandHistory, content)
}

func (a *App) recallPreviousCommand() {
	if len(a.commandHistory) == 0 {
		return
	}
	if a.historyIndex == -1 {
		a.historyDraft = a.input.Value()
		a.historyIndex = len(a.commandHistory) - 1
	} else if a.historyIndex > 0 {
		a.historyIndex--
	} else {
		return
	}
	a.input.SetValue(a.commandHistory[a.historyIndex])
	a.input.CursorEnd()
	a.syncInputHeight()
	a.refreshTranscript()
}

func (a *App) recallNextCommand() {
	if a.historyIndex == -1 {
		return
	}
	if a.historyIndex < len(a.commandHistory)-1 {
		a.historyIndex++
		a.input.SetValue(a.commandHistory[a.historyIndex])
	} else {
		a.historyIndex = -1
		a.input.SetValue(a.historyDraft)
		a.historyDraft = ""
	}
	a.input.CursorEnd()
	a.syncInputHeight()
	a.refreshTranscript()
}

func (a *App) layout() {
	sidebarWidth := 0
	if a.sidebarOpen {
		sidebarWidth = clamp(a.width/4, 24, 32)
	}
	mainWidth := a.width - sidebarWidth
	if mainWidth < 24 {
		mainWidth = 24
	}
	headerHeight := 3
	bodyHeight := a.height - headerHeight
	if bodyHeight < 6 {
		bodyHeight = 6
	}
	a.input.SetWidth(max(mainWidth-4, 20))
	a.syncInputHeight()
	a.transcript.Width = max(mainWidth-4, 20)
	a.transcript.Height = bodyHeight
	a.refreshTranscript()
}

func (a *App) submit() tea.Cmd {
	content := strings.TrimSpace(a.input.Value())
	if content == "" && len(a.inputTokens) == 0 {
		return nil
	}
	a.pushHistory(content)
	a.focus = focusInput
	a.input.Focus()
	if a.streaming || len(a.queuedSubmissions) > 0 || len(a.pendingSubmissions) > 0 {
		a.enqueueSubmission(content, a.inputTokens)
		a.input.Reset()
		a.inputTokens = nil
		a.selectedToken = 0
		a.refreshTranscript()
		if !a.streaming {
			return a.releaseQueuedSubmission()
		}
		return nil
	}
	if content == "/clear" {
		a.messages = nil
		a.seed()
		a.input.Reset()
		a.status = "Transcript cleared"
		a.refreshTranscript()
		return nil
	}
	var submittedTokens []inputToken
	if len(a.inputTokens) > 0 {
		submittedTokens = append(submittedTokens, a.inputTokens...)
	}
	return a.beginSubmission(content, nil, submittedTokens)
}

func (a *App) beginSubmission(content string, entries []string, submittedTokens []inputToken) tea.Cmd {
	a.focus = focusInput
	a.input.Focus()
	userContent := strings.TrimSpace(content)
	if len(submittedTokens) > 0 {
		var refs []string
		for _, tok := range submittedTokens {
			refs = append(refs, tok.label)
		}
		if userContent != "" {
			userContent += "\n\n"
		}
		userContent += strings.Join(refs, "  ")
	}
	expanded := map[int]bool{}
	for i, tok := range submittedTokens {
		if tok.kind == "paste" {
			expanded[i] = true
		}
	}
	a.messages = append(a.messages, message{role: "user", content: strings.TrimSpace(content), entries: entries, tokens: submittedTokens, expanded: expanded})
	a.messages = append(a.messages, message{role: "assistant", content: "", streaming: true})
	a.input.Reset()
	a.status = "Preparing fake response"
	a.streamQueue = splitForStreaming(fakeReply(userContent))
	a.streamDelay = streamDelayForPrompt(userContent)
	a.streamPulse = 0
	a.interruptArmed = false
	a.streaming = true
	a.refreshTranscript()
	return tea.Batch(a.nextStreamCmd(), a.nextPulseCmd())
}

func (a *App) nextStreamCmd() tea.Cmd {
	if len(a.streamQueue) == 0 {
		return func() tea.Msg { return streamTickMsg{done: true} }
	}
	part := a.streamQueue[0]
	a.streamQueue = a.streamQueue[1:]
	done := len(a.streamQueue) == 0
	return tea.Tick(a.streamDelay, func(time.Time) tea.Msg {
		return streamTickMsg{part: part, done: done}
	})
}

func (a *App) nextPulseCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return streamPulseMsg{}
	})
}

func (a *App) refreshTranscript() {
	followTail := a.transcript.AtBottom()
	offset := a.transcript.YOffset
	if a.focus == focusInput || a.transcript.TotalLineCount() == 0 {
		followTail = true
	}
	var b strings.Builder
	a.transcriptRefs = nil
	refCount := 0
	for i, msg := range a.messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		switch msg.role {
		case "system":
			b.WriteString(systemStyle.Render(msg.content))
		case "user":
			var userBlock strings.Builder
			if len(msg.entries) > 0 {
				for idx, entry := range msg.entries {
					if idx > 0 {
						userBlock.WriteString("\n\n")
					}
					userBlock.WriteString("▎ " + entry)
				}
			} else if msg.content != "" {
				userBlock.WriteString("▎ " + msg.content)
			}
			for tokenIndex, tok := range msg.tokens {
				if userBlock.Len() > 0 {
					userBlock.WriteString("\n")
				}
				if userBlock.Len() > 0 {
					userBlock.WriteString("\n")
				}
				ref := transcriptRef{messageIndex: i, tokenIndex: tokenIndex}
				a.transcriptRefs = append(a.transcriptRefs, ref)
				selected := refCount == a.selectedTranscriptRef && a.focus == focusTranscript
				refCount++
				userBlock.WriteString(a.renderTranscriptToken(msg, tokenIndex, tok, selected))
			}
			b.WriteString(userBodyStyle.Render(userBlock.String()))
		case "event":
			b.WriteString(renderEventMessage(msg))
		case "assistant":
			label := fmt.Sprintf("%s  %s", a.modelName, statusDot(msg.streaming, a.streamPulse, a.interruptArmed))
			b.WriteString(assistantLabelStyle.Render(label))
			b.WriteString("\n")
			b.WriteString(assistantBodyStyle.Render(msg.content))
		}
	}
	for _, queued := range a.queuedSubmissions {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(a.renderQueuedSubmission(queued))
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString(a.renderComposerSurface())
	if len(a.transcriptRefs) == 0 {
		a.selectedTranscriptRef = 0
	} else if a.selectedTranscriptRef >= len(a.transcriptRefs) {
		a.selectedTranscriptRef = len(a.transcriptRefs) - 1
	}
	a.transcript.SetContent(b.String())
	if followTail {
		a.transcript.GotoBottom()
	} else {
		a.transcript.SetYOffset(offset)
	}
}

func (a App) renderComposerSurface() string {
	var b strings.Builder
	b.WriteString(a.renderInputSurface())
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(a.renderFooter(a.transcript.Width))
	return composerBoxStyle.Render(b.String())
}

func (a App) renderTranscriptToken(msg message, tokenIndex int, tok inputToken, selected bool) string {
	style := transcriptTokenStyle
	if selected {
		style = transcriptTokenActiveStyle
	}
	var lines []string
	lines = append(lines, style.Render(tok.label))
	switch tok.kind {
	case "paste":
		if msg.expanded[tokenIndex] {
			lines = append(lines, transcriptMetaStyle.Render(tok.payload))
		} else {
			lines = append(lines, transcriptMetaStyle.Render(summarizePasteToken(tok.payload)))
		}
	case "file":
		lines = append(lines, transcriptMetaStyle.Render(tok.payload))
	}
	return transcriptTokenBlockStyle.Render(strings.Join(lines, "\n"))
}

func (a App) renderQueuedSubmission(queued queuedSubmission) string {
	var block strings.Builder
	block.WriteString(queuedLabelStyle.Render("queued"))
	if len(queued.entries) > 0 {
		for idx, entry := range queued.entries {
			block.WriteString("\n")
			if idx > 0 {
				block.WriteString("\n")
			}
			block.WriteString("▎ " + entry)
		}
	} else if queued.content != "" {
		block.WriteString("\n")
		block.WriteString("▎ " + queued.content)
	}
	for _, tok := range queued.tokens {
		if block.Len() > 0 {
			block.WriteString("\n\n")
		}
		block.WriteString(transcriptTokenBlockStyle.Render(strings.Join([]string{
			transcriptTokenStyle.Render(tok.label),
			transcriptMetaStyle.Render(summarizeQueuedToken(tok)),
		}, "\n")))
	}
	return queuedBodyStyle.Render(block.String())
}

func (a App) renderSidebar() string {
	if !a.sidebarOpen {
		return ""
	}
	width := clamp(a.width/4, 24, 32)
	var b strings.Builder
	b.WriteString(sidebarTitleStyle.Width(width - 4).Render("modeltap spike"))
	b.WriteString("\n")
	b.WriteString(sidebarMutedStyle.Width(width - 4).Render("Crush-inspired shell evaluation"))
	b.WriteString("\n\n")
	currentSection := ""
	for i, item := range a.sidebarItems {
		if item.section != currentSection {
			if currentSection != "" {
				b.WriteString("\n")
			}
			currentSection = item.section
			b.WriteString(sidebarMetaStyle.Render(strings.ToUpper(currentSection)))
			b.WriteString("\n")
		}
		style := sidebarItemStyle
		prefix := "  "
		if item.kind == sidebarItemModel {
			style = sidebarValueStyle.Padding(0, 1)
		}
		if i == a.sidebarIndex {
			style = sidebarItemActiveStyle
			prefix = "• "
			if a.focus == focusSidebar {
				style = sidebarItemFocusedStyle
				prefix = "› "
			}
		}
		b.WriteString(style.Width(width - 4).Render(prefix + item.label))
		b.WriteString("\n")
	}
	b.WriteString("\n\n")
	b.WriteString(sidebarMetaStyle.Render("KEYS"))
	b.WriteString("\n")
	keys := []string{
		"Ctrl+B  toggle sidebar",
		"Tab  cycle focus",
		"Enter  send",
		"Ctrl+J  newline",
		"Ctrl+C  quit",
	}
	for _, k := range keys {
		b.WriteString(sidebarMutedStyle.Width(width - 4).Render(k))
		b.WriteString("\n")
	}
	return sidebarBoxStyle.Width(width).Height(max(a.height, 12)).Render(b.String())
}

func (a App) renderMain() string {
	sidebarWidth := clamp(a.width/4, 24, 32)
	if !a.sidebarOpen {
		sidebarWidth = 0
	}
	mainWidth := a.width - sidebarWidth
	header := headerBoxStyle.Width(mainWidth).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			headerTitleStyle.Render("Harness Spike"),
			headerSubtitleStyle.Render(a.currentSession+"  |  "+a.modelName+"  |  "+a.status+"  |  focus: "+a.focus.String()+"  |  sidebar: "+sidebarState(a.sidebarOpen)),
		),
	)
	body := transcriptBoxStyle.Width(mainWidth).Height(max(a.transcript.Height+2, 8)).Render(a.transcript.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (f focusZone) String() string {
	switch f {
	case focusSidebar:
		return "sidebar"
	case focusTranscript:
		return "transcript"
	case focusInput:
		return "input"
	default:
		return "unknown"
	}
}

func nextFocus(current focusZone, sidebarOpen bool) focusZone {
	switch current {
	case focusInput:
		return focusTranscript
	case focusTranscript:
		if !sidebarOpen {
			return focusInput
		}
		return focusSidebar
	default:
		return focusInput
	}
}

func (a *App) moveSidebar(delta int) {
	if len(a.sidebarItems) == 0 {
		return
	}
	a.sidebarIndex += delta
	if a.sidebarIndex < 0 {
		a.sidebarIndex = 0
	}
	if a.sidebarIndex >= len(a.sidebarItems) {
		a.sidebarIndex = len(a.sidebarItems) - 1
	}
	item := a.sidebarItems[a.sidebarIndex]
	a.status = "Sidebar: " + item.section + " / " + item.label
}

func (a *App) activateSidebar() {
	if len(a.sidebarItems) == 0 {
		return
	}
	item := a.sidebarItems[a.sidebarIndex]
	switch item.kind {
	case sidebarItemSession:
		a.seedWithSession(item.label)
		a.status = "Selected session: " + item.label
	case sidebarItemModel:
		a.modelName = item.label
		a.status = "Model set to " + item.label
	case sidebarItemAction:
		switch item.value {
		case "clear":
			a.dialog = &choiceDialog{
				title:  "Clear Transcript",
				prompt: "Choose how to handle the current transcript.",
				options: []choiceOption{
					{label: "Clear and reseed intro", value: "clear-reseed"},
					{label: "Clear and leave empty", value: "clear-empty"},
					{label: "Cancel", value: "cancel"},
				},
			}
			a.status = "Awaiting choice"
		case "replay":
			a.seed()
			a.status = "Intro replayed"
		case "tool-demo":
			a.seedWithSession("Tool Demo")
			a.status = "Tool demo replayed"
		default:
			a.status = "Selected action: " + item.label
		}
	}
	a.refreshTranscript()
}

func (a *App) handleDialogKey(msg tea.KeyMsg) tea.Cmd {
	if a.dialog == nil {
		return nil
	}
	switch msg.Type {
	case tea.KeyUp:
		if a.dialog.index > 0 {
			a.dialog.index--
		}
		return nil
	case tea.KeyDown:
		if a.dialog.index < len(a.dialog.options)-1 {
			a.dialog.index++
		}
		return nil
	case tea.KeyEsc:
		a.dialog = nil
		a.status = "Cancelled"
		return nil
	case tea.KeyEnter:
		a.applyChoice(a.dialog.options[a.dialog.index].value)
		return nil
	}
	switch strings.ToLower(msg.String()) {
	case "k":
		if a.dialog.index > 0 {
			a.dialog.index--
		}
	case "j":
		if a.dialog.index < len(a.dialog.options)-1 {
			a.dialog.index++
		}
	}
	return nil
}

func (a *App) applyChoice(choice string) {
	a.dialog = nil
	switch choice {
	case "cancel":
		a.status = "Cancelled"
		return
	case "clear-reseed":
		a.seed()
		a.status = "Transcript cleared"
	case "clear-empty":
		a.messages = nil
		a.refreshTranscript()
		a.status = "Transcript emptied"
	}
}

func sidebarState(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}

func (a App) renderOverlay() string {
	if a.agentDetail != nil {
		return a.renderAgentDetail()
	}
	if a.agentList != nil {
		return a.renderAgentList()
	}
	if a.palette != nil {
		return a.renderPalette()
	}
	if a.preview != nil {
		return a.renderPreview()
	}
	return a.renderDialog()
}

func (a App) renderDialog() string {
	if a.dialog == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(dialogTitleStyle.Render(a.dialog.title))
	b.WriteString("\n")
	b.WriteString(dialogPromptStyle.Render(a.dialog.prompt))
	b.WriteString("\n\n")
	for i, opt := range a.dialog.options {
		style := dialogOptionStyle
		prefix := "  "
		if i == a.dialog.index {
			style = dialogOptionActiveStyle
			prefix = "› "
		}
		b.WriteString(style.Render(prefix + opt.label))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(dialogDividerStyle.Render(strings.Repeat("─", 42)))
	b.WriteString("\n")
	b.WriteString(dialogHintStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			keycapStyle.Render("↑/↓"),
			" ",
			dialogHintStyle.Render("choose"),
			"   ",
			keycapStyle.Render("Enter"),
			" ",
			dialogHintStyle.Render("confirm"),
			"   ",
			keycapStyle.Render("Esc"),
			" ",
			dialogHintStyle.Render("cancel"),
		),
	))
	return dialogBoxStyle.Render(b.String())
}

func (a App) renderPreview() string {
	if a.preview == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(dialogTitleStyle.Render(a.preview.title))
	b.WriteString("\n")
	b.WriteString(dialogDividerStyle.Render(strings.Repeat("─", 42)))
	b.WriteString("\n")
	b.WriteString(previewBodyStyle.Render(a.preview.content))
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

func (a App) renderInputSurface() string {
	var b strings.Builder
	if len(a.inputTokens) > 0 {
		for i, tok := range a.inputTokens {
			style := tokenStyle
			if i == a.selectedToken {
				style = tokenActiveStyle
			}
			b.WriteString(style.Render(tok.label))
			b.WriteString(" ")
		}
		b.WriteString("\n")
		b.WriteString(tokenHintStyle.Render("Ctrl+P/Ctrl+N select token • Ctrl+O preview token"))
		b.WriteString("\n")
	}
	b.WriteString(a.input.View())
	return b.String()
}

func (a App) renderFooter(width int) string {
	count := len(a.agents)
	label := fmt.Sprintf("%d background agents running", count)
	hint := "Ctrl+B sidebar  Ctrl+T agents  Ctrl+K palette"
	if count == 1 {
		label = "1 background agent running"
	}
	if a.focus == focusTranscript {
		label += "  |  scroll: wheel/arrows  items: j/k  open: Enter"
	}
	if len(a.queuedSubmissions) > 0 {
		label += fmt.Sprintf("  |  %d queued", len(a.queuedSubmissions))
	}
	left := footerStatusStyle.Render(label)
	right := footerHintStyle.Render(hint)
	space := width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if space < 1 {
		space = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, left, strings.Repeat(" ", space), right)
}

func (a App) renderAgentList() string {
	width := max(a.width-4, 40)
	var b strings.Builder
	b.WriteString(overlayHeaderStyle.Render("Background Agents"))
	b.WriteString("\n")
	b.WriteString(overlaySubheadStyle.Render("Select an agent stream to inspect."))
	b.WriteString("\n\n")
	b.WriteString(overlaySectionStyle.Render("RUNNING"))
	b.WriteString("\n")
	for i, agent := range a.agents {
		style := overlayItemStyle
		prefix := "  "
		if a.agentList != nil && i == a.agentList.index {
			style = overlayItemActiveStyle
			prefix = "› "
		}
		line := fmt.Sprintf("%s%s  [%s]", prefix, agent.name, agent.status)
		b.WriteString(style.Width(max(width-8, 20)).Render(line))
		b.WriteString("\n")
		b.WriteString(dialogHintStyle.Render("  " + agent.summary))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(dialogDividerStyle.Render(strings.Repeat("─", max(width-4, 20))))
	b.WriteString("\n")
	b.WriteString(overlayFooterStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			keycapStyle.Render("↑/↓"),
			" ",
			dialogHintStyle.Render("choose"),
			"   ",
			keycapStyle.Render("Enter"),
			" ",
			dialogHintStyle.Render("open"),
			"   ",
			keycapStyle.Render("Esc"),
			" ",
			dialogHintStyle.Render("close"),
		),
	))
	return overlayPanelStyle.Width(width).Height(max(a.height-2, 12)).Render(b.String())
}

func (a App) renderAgentDetail() string {
	agent := a.currentAgent()
	if agent == nil {
		return ""
	}
	width := max(a.width-4, 40)
	var b strings.Builder
	b.WriteString(overlayHeaderStyle.Render(agent.name))
	b.WriteString("\n")
	b.WriteString(overlaySubheadStyle.Render("Status: " + agent.status))
	b.WriteString("\n")
	b.WriteString(dialogDividerStyle.Render(strings.Repeat("─", max(width-4, 20))))
	b.WriteString("\n")
	b.WriteString(overlaySectionStyle.Render("STREAM"))
	b.WriteString("\n")
	for i, line := range agent.stream {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(agentStreamStyle.Render(line))
	}
	b.WriteString("\n\n")
	b.WriteString(dialogDividerStyle.Render(strings.Repeat("─", max(width-4, 20))))
	b.WriteString("\n")
	b.WriteString(overlayFooterStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			keycapStyle.Render("Esc"),
			" ",
			dialogHintStyle.Render("back"),
		),
	))
	return overlayPanelStyle.Width(width).Height(max(a.height-2, 12)).Render(b.String())
}

func (a App) renderPalette() string {
	width := clamp(a.width-12, 56, 88)
	commands := a.filteredCommands()
	var b strings.Builder
	b.WriteString(dialogTitleStyle.Render("Command Palette"))
	b.WriteString("\n")
	b.WriteString(paletteQueryStyle.Render("> " + a.palette.query))
	b.WriteString("\n\n")
	if len(commands) == 0 {
		b.WriteString(dialogHintStyle.Render("No matching commands"))
	} else {
		for i, cmd := range commands {
			style := dialogOptionStyle
			prefix := "  "
			if i == a.palette.index {
				style = dialogOptionActiveStyle
				prefix = "› "
			}
			b.WriteString(style.Width(width - 6).Render(prefix + cmd.label))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(dialogDividerStyle.Render(strings.Repeat("─", max(width-4, 20))))
	b.WriteString("\n")
	b.WriteString(dialogHintStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			keycapStyle.Render("type"),
			" ",
			dialogHintStyle.Render("filter"),
			"   ",
			keycapStyle.Render("Enter"),
			" ",
			dialogHintStyle.Render("run"),
			"   ",
			keycapStyle.Render("Esc"),
			" ",
			dialogHintStyle.Render("close"),
		),
	))
	return dialogBoxStyle.Width(width).Render(b.String())
}

func overlayString(base, overlay string, x, y int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")
	for row, line := range overlayLines {
		targetRow := y + row
		if targetRow < 0 || targetRow >= len(baseLines) {
			continue
		}
		baseRunes := []rune(baseLines[targetRow])
		overlayRunes := []rune(line)
		if x > len(baseRunes) {
			baseRunes = append(baseRunes, []rune(strings.Repeat(" ", x-len(baseRunes)))...)
		}
		for col, r := range overlayRunes {
			targetCol := x + col
			if targetCol < 0 {
				continue
			}
			if targetCol >= len(baseRunes) {
				baseRunes = append(baseRunes, []rune(strings.Repeat(" ", targetCol-len(baseRunes)+1))...)
			}
			baseRunes[targetCol] = r
		}
		baseLines[targetRow] = string(baseRunes)
	}
	return strings.Join(baseLines, "\n")
}

func (a *App) openPalette() {
	a.palette = &commandPalette{}
	a.status = "Command palette open"
}

func (a *App) handlePaletteKey(msg tea.KeyMsg) tea.Cmd {
	if a.palette == nil {
		return nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		a.palette = nil
		a.status = "Command palette closed"
		return nil
	case tea.KeyUp:
		if a.palette.index > 0 {
			a.palette.index--
		}
		return nil
	case tea.KeyDown:
		if a.palette.index < len(a.filteredCommands())-1 {
			a.palette.index++
		}
		return nil
	case tea.KeyBackspace:
		if len(a.palette.query) > 0 {
			a.palette.query = a.palette.query[:len(a.palette.query)-1]
			a.palette.index = 0
		}
		return nil
	case tea.KeyEnter:
		commands := a.filteredCommands()
		if len(commands) == 0 {
			return nil
		}
		a.runPaletteCommand(commands[a.palette.index])
		return nil
	case tea.KeyRunes:
		a.palette.query += msg.String()
		a.palette.index = 0
		return nil
	}
	switch strings.ToLower(msg.String()) {
	case "j":
		if a.palette.index < len(a.filteredCommands())-1 {
			a.palette.index++
		}
	case "k":
		if a.palette.index > 0 {
			a.palette.index--
		}
	}
	return nil
}

func (a *App) filteredCommands() []paletteCommand {
	var commands []paletteCommand
	commands = append(commands,
		paletteCommand{label: "Session: Spike Session", value: "Spike Session", kind: "session", filter: "session spike"},
		paletteCommand{label: "Session: Reference Layout", value: "Reference Layout", kind: "session", filter: "session reference layout"},
		paletteCommand{label: "Session: Dummy Stream", value: "Dummy Stream", kind: "session", filter: "session dummy stream"},
		paletteCommand{label: "Session: Tool Demo", value: "Tool Demo", kind: "session", filter: "session tool demo permission events"},
		paletteCommand{label: "Action: Clear Transcript", value: "clear", kind: "action", filter: "action clear transcript"},
		paletteCommand{label: "Action: Replay Intro", value: "replay", kind: "action", filter: "action replay intro"},
		paletteCommand{label: "View: Background Agents", value: "agents", kind: "view", filter: "view agents background"},
		paletteCommand{label: "Toggle: Sidebar", value: "sidebar", kind: "toggle", filter: "toggle sidebar"},
	)
	query := strings.ToLower(strings.TrimSpace(a.palette.query))
	if query == "" {
		return commands
	}
	var out []paletteCommand
	for _, cmd := range commands {
		if strings.Contains(strings.ToLower(cmd.label), query) || strings.Contains(cmd.filter, query) {
			out = append(out, cmd)
		}
	}
	return out
}

func (a *App) runPaletteCommand(cmd paletteCommand) {
	a.palette = nil
	switch cmd.kind {
	case "session":
		a.seedWithSession(cmd.value)
		a.status = "Selected session: " + cmd.value
	case "action":
		switch cmd.value {
		case "clear":
			a.dialog = &choiceDialog{
				title:  "Clear Transcript",
				prompt: "Choose how to handle the current transcript.",
				options: []choiceOption{
					{label: "Clear and reseed intro", value: "clear-reseed"},
					{label: "Clear and leave empty", value: "clear-empty"},
					{label: "Cancel", value: "cancel"},
				},
			}
			a.status = "Awaiting choice"
		case "replay":
			a.seed()
			a.status = "Intro replayed"
		}
	case "view":
		if cmd.value == "agents" {
			a.openAgents()
		}
	case "toggle":
		if cmd.value == "sidebar" {
			a.sidebarOpen = !a.sidebarOpen
			if !a.sidebarOpen && a.focus == focusSidebar {
				a.focus = focusTranscript
			}
			a.layout()
			if a.sidebarOpen {
				a.status = "Sidebar opened"
			} else {
				a.status = "Sidebar closed"
			}
		}
	}
	a.refreshTranscript()
}

func (a *App) openAgents() {
	if len(a.agents) == 0 {
		a.status = "No background agents"
		return
	}
	if len(a.agents) == 1 {
		a.agentDetail = &agentDetailDialog{agentID: a.agents[0].id}
		a.agentList = nil
		a.status = "Viewing background agent"
		return
	}
	a.agentList = &agentListDialog{}
	a.agentDetail = nil
	a.status = "Viewing background agents"
}

func (a *App) handleAgentListKey(msg tea.KeyMsg) tea.Cmd {
	if a.agentList == nil {
		return nil
	}
	switch msg.Type {
	case tea.KeyUp:
		if a.agentList.index > 0 {
			a.agentList.index--
		}
	case tea.KeyDown:
		if a.agentList.index < len(a.agents)-1 {
			a.agentList.index++
		}
	case tea.KeyEsc:
		a.agentList = nil
		a.status = "Closed background agents"
	case tea.KeyEnter:
		a.agentDetail = &agentDetailDialog{agentID: a.agents[a.agentList.index].id}
		a.status = "Viewing " + a.agents[a.agentList.index].name
	}
	switch strings.ToLower(msg.String()) {
	case "k":
		if a.agentList.index > 0 {
			a.agentList.index--
		}
	case "j":
		if a.agentList.index < len(a.agents)-1 {
			a.agentList.index++
		}
	}
	return nil
}

func (a *App) handleAgentDetailKey(msg tea.KeyMsg) tea.Cmd {
	if a.agentDetail == nil {
		return nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		if len(a.agents) > 1 {
			idx := a.agentIndexByID(a.agentDetail.agentID)
			a.agentList = &agentListDialog{index: idx}
		}
		a.agentDetail = nil
		a.status = "Closed background agent"
	}
	return nil
}

func (a App) currentAgent() *backgroundAgent {
	if a.agentDetail == nil {
		return nil
	}
	idx := a.agentIndexByID(a.agentDetail.agentID)
	if idx < 0 {
		return nil
	}
	return &a.agents[idx]
}

func (a App) agentIndexByID(id string) int {
	for i, agent := range a.agents {
		if agent.id == id {
			return i
		}
	}
	return -1
}

func (a *App) handleInputMutation(prev, current string) {
	if current == prev {
		return
	}
	if normalizedPath, ok := normalizeDroppedPath(current); ok {
		a.addToken("file", normalizedPath)
		a.input.Reset()
		a.status = "Dropped path captured as file entry"
		return
	}
	delta := len(current) - len(prev)
	if delta >= 120 {
		trimmed := strings.TrimSpace(current)
		if trimmed != "" {
			a.addToken("paste", trimmed)
			a.input.Reset()
			a.status = "Large paste captured as compact entry"
			return
		}
	}
}

func (a *App) addToken(kind, payload string) {
	n := len(a.inputTokens) + 1
	label := kind + "-" + fmt.Sprintf("%d", n)
	if kind == "file" {
		label = detectFileLabel(payload, n)
	}
	a.inputTokens = append(a.inputTokens, inputToken{
		id:      fmt.Sprintf("%s-%d", kind, n),
		kind:    kind,
		label:   label,
		payload: payload,
	})
	a.selectedToken = len(a.inputTokens) - 1
}

func normalizeDroppedPath(v string) (string, bool) {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return "", false
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) != 1 {
		return "", false
	}
	path := strings.TrimSpace(lines[0])
	if len(path) >= 2 {
		if (path[0] == '"' && path[len(path)-1] == '"') || (path[0] == '\'' && path[len(path)-1] == '\'') {
			path = path[1 : len(path)-1]
		}
	}
	path = strings.ReplaceAll(path, "\\ ", " ")
	path = strings.ReplaceAll(path, "\\(", "(")
	path = strings.ReplaceAll(path, "\\)", ")")
	path = strings.ReplaceAll(path, "\\[", "[")
	path = strings.ReplaceAll(path, "\\]", "]")
	if looksLikeDroppedPath(path) {
		return path, true
	}
	return "", false
}

func looksLikeDroppedPath(path string) bool {
	if path == "" || path == "/" || strings.HasPrefix(path, "//") {
		return false
	}
	if strings.HasPrefix(path, "~/") {
		return len(path) > 2
	}
	if !strings.HasPrefix(path, "/") {
		return false
	}
	rest := path[1:]
	if rest == "" {
		return false
	}
	if !strings.Contains(rest, "/") {
		return false
	}
	return true
}

func detectFileLabel(path string, n int) string {
	name := path
	if idx := strings.LastIndexAny(path, "/\\"); idx >= 0 && idx+1 < len(path) {
		name = path[idx+1:]
	}
	if name == "" {
		return fmt.Sprintf("file-%d", n)
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".gif"), strings.HasSuffix(lower, ".webp"):
		return fmt.Sprintf("image-%d (%s)", n, name)
	default:
		return fmt.Sprintf("file-%d (%s)", n, name)
	}
}

func (a *App) moveTokenSelection(delta int) {
	if len(a.inputTokens) == 0 {
		return
	}
	a.selectedToken += delta
	if a.selectedToken < 0 {
		a.selectedToken = 0
	}
	if a.selectedToken >= len(a.inputTokens) {
		a.selectedToken = len(a.inputTokens) - 1
	}
	a.status = "Selected " + a.inputTokens[a.selectedToken].label
}

func (a *App) openSelectedTokenPreview() {
	if len(a.inputTokens) == 0 {
		return
	}
	tok := a.inputTokens[a.selectedToken]
	a.preview = &previewDialog{
		title:   tok.label,
		content: tok.payload,
	}
	a.status = "Previewing " + tok.label
}

func (a *App) enqueueSubmission(content string, tokens []inputToken) {
	var queuedTokens []inputToken
	if len(tokens) > 0 {
		queuedTokens = append(queuedTokens, tokens...)
	}
	entries := []string{content}
	if strings.TrimSpace(content) == "" {
		entries = nil
	}
	a.queuedSubmissions = append(a.queuedSubmissions, queuedSubmission{
		content: content,
		entries: entries,
		tokens:  queuedTokens,
	})
	a.status = fmt.Sprintf("Queued follow-up message (%d waiting)", len(a.queuedSubmissions))
}

func (a *App) releaseQueuedSubmission() tea.Cmd {
	if len(a.pendingSubmissions) == 0 && len(a.queuedSubmissions) > 0 {
		a.pendingSubmissions = append(a.pendingSubmissions, a.queuedSubmissions...)
		a.queuedSubmissions = nil
	}
	if len(a.pendingSubmissions) == 0 {
		return nil
	}
	next := mergeQueuedSubmissions(a.pendingSubmissions)
	a.pendingSubmissions = nil
	a.status = "Releasing merged queued follow-up"
	a.refreshTranscript()
	return a.beginSubmission(next.content, next.entries, next.tokens)
}

func mergeQueuedSubmissions(items []queuedSubmission) queuedSubmission {
	var merged queuedSubmission
	if len(items) == 0 {
		return merged
	}
	var parts []string
	for _, item := range items {
		if strings.TrimSpace(item.content) != "" {
			parts = append(parts, strings.TrimSpace(item.content))
		}
		if len(item.entries) > 0 {
			merged.entries = append(merged.entries, item.entries...)
		} else if strings.TrimSpace(item.content) != "" {
			merged.entries = append(merged.entries, strings.TrimSpace(item.content))
		}
		if len(item.tokens) > 0 {
			merged.tokens = append(merged.tokens, item.tokens...)
		}
	}
	merged.content = strings.Join(parts, "\n\n")
	return merged
}

func (a *App) stopStreaming() {
	a.streaming = false
	a.streamQueue = nil
	a.interruptArmed = false
	if len(a.messages) > 0 {
		last := &a.messages[len(a.messages)-1]
		if last.role == "assistant" && last.streaming {
			last.streaming = false
			if strings.TrimSpace(last.content) == "" {
				last.content = "[stopped]"
			}
		}
	}
	a.status = "Stream stopped"
	a.refreshTranscript()
}

func summarizePasteToken(payload string) string {
	lines := strings.Split(strings.TrimSpace(payload), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "empty paste"
	}
	summaryLines := min(len(lines), 3)
	preview := strings.Join(lines[:summaryLines], " / ")
	if len(lines) > summaryLines {
		return fmt.Sprintf("%d lines: %s ...", len(lines), preview)
	}
	return fmt.Sprintf("%d lines: %s", len(lines), preview)
}

func summarizeQueuedToken(tok inputToken) string {
	switch tok.kind {
	case "paste":
		return summarizePasteToken(tok.payload)
	case "file":
		return tok.payload
	default:
		return tok.label
	}
}

func renderEventMessage(msg message) string {
	style := eventInfoStyle
	switch msg.eventState {
	case "requested":
		style = eventRequestedStyle
	case "permission":
		style = eventPermissionStyle
	case "running":
		style = eventRunningStyle
	case "done":
		style = eventDoneStyle
	}
	return style.Render(msg.content)
}

func (a *App) handlePreviewKey(msg tea.KeyMsg) tea.Cmd {
	if a.preview == nil {
		return nil
	}
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		a.preview = nil
		a.status = "Preview closed"
	}
	return nil
}

func (a *App) moveTranscriptRef(delta int) {
	if len(a.transcriptRefs) == 0 {
		return
	}
	a.selectedTranscriptRef += delta
	if a.selectedTranscriptRef < 0 {
		a.selectedTranscriptRef = 0
	}
	if a.selectedTranscriptRef >= len(a.transcriptRefs) {
		a.selectedTranscriptRef = len(a.transcriptRefs) - 1
	}
	ref := a.transcriptRefs[a.selectedTranscriptRef]
	tok := a.messages[ref.messageIndex].tokens[ref.tokenIndex]
	a.status = "Transcript item: " + tok.label
	a.refreshTranscript()
}

func (a *App) openSelectedTranscriptRef() {
	if len(a.transcriptRefs) == 0 {
		return
	}
	ref := a.transcriptRefs[a.selectedTranscriptRef]
	msg := &a.messages[ref.messageIndex]
	tok := msg.tokens[ref.tokenIndex]
	if tok.kind == "paste" {
		if msg.expanded == nil {
			msg.expanded = map[int]bool{}
		}
		msg.expanded[ref.tokenIndex] = !msg.expanded[ref.tokenIndex]
		if msg.expanded[ref.tokenIndex] {
			a.status = "Expanded " + tok.label
		} else {
			a.status = "Collapsed " + tok.label
		}
		a.refreshTranscript()
		return
	}
	a.preview = &previewDialog{
		title:   tok.label,
		content: tok.payload,
	}
	a.status = "Previewing " + tok.label
}

func fakeReply(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lower, "/demo"):
		return demoReply()
	case strings.Contains(lower, "why"):
		return "This spike exists to prove the shell before wiring the backend. If the shell feels wrong with fake data, real integrations will only make it worse."
	default:
		return "Fake response for: " + trimmed + "\n\nThis is intentionally dumb, but it should feel responsive:\n- immediate echo\n- progressive stream\n- stable transcript\n- predictable focus"
	}
}

func streamDelayForPrompt(prompt string) time.Duration {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(prompt)), "/demo") {
		return 500 * time.Millisecond
	}
	return 35 * time.Millisecond
}

func demoReply() string {
	segments := []string{
		"Demo stream mode engaged for long-run testing.",
		"Watch the working indicator pulse while tokens continue to arrive.",
		"Use this mode to test queue visibility, interrupt handling, and transcript stability.",
		"Each numbered step is intentionally verbose so the stream lasts long enough to judge behavior clearly.",
	}
	var parts []string
	for i := 1; i <= 12; i++ {
		parts = append(parts, fmt.Sprintf("Step %02d.", i))
		for _, segment := range segments {
			parts = append(parts, segment)
		}
	}
	parts = append(parts, "Demo stream complete. The queue should now release the next waiting message.")
	return strings.Join(parts, " ")
}

func splitForStreaming(s string) []string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return []string{""}
	}
	out := make([]string, 0, len(parts))
	for i, part := range parts {
		if i == len(parts)-1 {
			out = append(out, part)
		} else {
			out = append(out, part+" ")
		}
	}
	return out
}

func statusDot(streaming bool, pulse int, interruptArmed bool) string {
	if streaming {
		if interruptArmed {
			return "press Esc again to interrupt"
		}
		return "working" + strings.Repeat(".", pulse)
	}
	return "done"
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
