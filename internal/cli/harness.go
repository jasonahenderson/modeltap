package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harness/theme"
	"github.com/jasonahenderson/modeltap/internal/harness/tools"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/spf13/cobra"
)

// registerBuiltinTools wires the 13 in-tree tools into registry so
// the ToolDispatcher can resolve tool.call notifications from the
// BFF. Read-only tools (Read/Glob/Grep/Git-read) are auto-allowed
// by the permission layer; Write/Edit/Bash/WebFetch/WebSearch
// prompt first-use under PermDefault.
func registerBuiltinTools(registry *tools.Registry, projectRoot string, tracker *tools.FileTracker) {
	registry.Register(tools.NewReadTool(projectRoot, tracker))
	registry.Register(tools.NewWriteTool(projectRoot, tracker))
	registry.Register(tools.NewEditTool(projectRoot, tracker))
	registry.Register(tools.NewBashTool(projectRoot))
	registry.Register(tools.NewGitTool(projectRoot))
	registry.Register(tools.NewGlobTool(projectRoot))
	registry.Register(tools.NewGrepTool(projectRoot))
	registry.Register(tools.NewWebFetchTool())
	// WebSearch is skipped in the default bundle because it needs an
	// API key from config; wiring it here without one would register
	// a tool that always errors. Users enable it explicitly once MCP
	// / config surfaces grow an api_key input.
}

// toolActivityObserver adapts the deferredSender into the
// ToolActivityObserver interface. Every tool start / end fires a
// ToolActivityMsg onto the tea.Program so the viewport can render
// inline "⚙ tool…" / "✓ outcome" entries. The dispatcher lives on
// a goroutine outside the Update loop, so routing through sender
// (tea.Program.Send) is the correct pattern.
type toolActivityObserver struct {
	sender *deferredSender
}

func newToolActivityObserver(sender *deferredSender) *toolActivityObserver {
	return &toolActivityObserver{sender: sender}
}

func (o *toolActivityObserver) OnToolStart(call protocol.ToolCall, summary string) {
	o.sender.Send(harness.ToolActivityMsg{
		Phase:      harness.ToolActivityStart,
		ToolName:   call.Tool,
		ToolCallID: call.ToolCallID,
		Summary:    summary,
	})
}

func (o *toolActivityObserver) OnToolEnd(call protocol.ToolCall, status, output string, elapsed time.Duration) {
	o.sender.Send(harness.ToolActivityMsg{
		Phase:      harness.ToolActivityEnd,
		ToolName:   call.Tool,
		ToolCallID: call.ToolCallID,
		Status:     status,
		Summary:    output,
		Duration:   elapsed,
	})
}

// toolResultSender adapts *ConnectionManager to the narrow
// harness.ToolResultSender interface. Reads the live *ProtocolClient
// on every call so the dispatcher survives reconnects.
type toolResultSender struct {
	cm *harness.ConnectionManager
}

func newToolResultSender(cm *harness.ConnectionManager) *toolResultSender {
	return &toolResultSender{cm: cm}
}

func (t *toolResultSender) SendToolResult(ctx context.Context, result *protocol.ToolResult) error {
	client := t.cm.Client()
	if client == nil {
		return fmt.Errorf("tool result dropped: no live BFF client")
	}
	return client.SendToolResult(ctx, result)
}

// deferredSender is a ProgramSender the CLI uses to break the
// chicken-and-egg between ConnectionManager (needs a sender at
// construction) and tea.Program (needs the App to pre-contain the
// manager-backed ConnSurface). The CLI creates the sender first, hands
// it to the manager, constructs the App with a conn backed by the
// manager, then starts the tea.Program and atomically stores it on
// the sender. Any Send calls that race during that window drop the
// message (safe — the event bridge simply no-ops until the program
// exists).
type deferredSender struct {
	program atomic.Pointer[tea.Program]
}

func (d *deferredSender) Send(msg tea.Msg) {
	p := d.program.Load()
	if p == nil {
		return
	}
	p.Send(msg)
}

// harnessFlags captures the flag values shared between the
// `modeltap harness` subcommand and the root-level default run
// (WU-089 + "default subcommand launches harness" per design D2
// of track-integration). Keeping one flag struct means the root
// and the subcommand stay in lockstep without copy-paste.
type harnessFlags struct {
	socketPath  string
	resumeID    string
	project     string
	modelName   string
	noAutoStart bool
}

// bindHarnessFlags registers the harness CLI flags on a cobra
// command. Called for both the harness subcommand and the root
// command so `modeltap --model X` works identically to
// `modeltap harness --model X`.
func bindHarnessFlags(cmd *cobra.Command, f *harnessFlags) {
	cmd.Flags().StringVar(&f.socketPath, "socket", "", "override the BFF socket path (defaults to config.bff.socket_path)")
	cmd.Flags().StringVar(&f.resumeID, "resume", "", "resume the given session id at startup")
	cmd.Flags().StringVar(&f.project, "project", "", "project directory (defaults to $PWD)")
	cmd.Flags().StringVar(&f.modelName, "model", "", "initial model override for the session")
	cmd.Flags().BoolVar(&f.noAutoStart, "no-auto-start", false, "do not auto-start modeltap start when the socket is absent")
}

// newHarnessCommand wires the terminal harness into the modeltap CLI
// (WU-089). It composes the Bubbletea App (Bundle 5 + Bundle 7 tools
// + Bundle 13 overlays) with a live ConnectionManager that speaks
// JSON-RPC to the BFF, auto-starting `modeltap start` when the
// configured socket is absent so the harness works from a cold boot.
func newHarnessCommand() *cobra.Command {
	var flags harnessFlags

	cmd := &cobra.Command{
		Use:   "harness",
		Short: "Launch the interactive modeltap harness",
		Long: `Start the modeltap terminal harness — a Bubbletea-powered TUI that
connects to the BFF over a local socket and drives sessions, tools,
attachments, and routing commands.

By default the harness auto-starts the BFF server (modeltap start)
if the configured socket is absent, waits for it to accept
connections, and then launches the TUI.

Slash commands available in-harness:
  /status                         show connection state
  /reconnect                      force reconnect
  /plan /build /auto              switch execution mode (Ctrl+P toggles)
  /history <user|project|session> change command-history scope
  /model                          show current model
  /models                         list catalog
  /model <name|auto>              switch session override
  /session                        show current session
  /sessions (or /session list)    list sessions
  /session resume <id>            resume
  /session fork                   fork current session
  /session clear                  clear context
  /context                        show context breakdown
  /mcp [status]                   MCP server state
  /mcp reconnect <name>           force-retry an MCP server`,
		Example: `  # Launch the harness against the default socket
  modeltap harness

  # Resume a specific session
  modeltap harness --resume 7d9f…

  # Override the initial model
  modeltap harness --model claude-opus-4-7

  # Point at a specific project directory (affects @file / tool scopes)
  modeltap harness --project /abs/path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHarness(cmd, &flags)
		},
	}

	bindHarnessFlags(cmd, &flags)
	return cmd
}

// runHarness is the shared entry point used by both the harness
// subcommand and the root default. It composes config → conn →
// app → tea.Program and blocks until the program exits.
func runHarness(cmd *cobra.Command, flags *harnessFlags) error {
	cfg, _, err := config.LoadWithViper("")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	effectiveSocket := flags.socketPath
	if effectiveSocket == "" {
		effectiveSocket = cfg.BFF.SocketPath
	}
	if effectiveSocket == "" {
		effectiveSocket = config.DefaultBFFSocketPath()
	}

	effectiveProject := flags.project
	if effectiveProject == "" {
		effectiveProject, _ = os.Getwd()
	}
	effectiveProject, _ = filepath.Abs(effectiveProject)

	serverBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving modeltap binary: %w", err)
	}

	connCfg := harness.ConnectionConfig{
		SocketPath:   effectiveSocket,
		AutoStart:    !flags.noAutoStart,
		ServerBinary: serverBinary,
		ServerArgs:   []string{"start"},
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  cmd.Root().Version,
			HarnessPlatform: "terminal",
			Project:         protocol.ProjectContext{Root: effectiveProject},
		},
	}

	// Build the shared tool framework pieces so the harness can
	// resolve @file attachments and execute tool.call notifications
	// from the server. Read-only auto-allowed by default; writes
	// and exec prompt first-use (PermDefault).
	tracker := tools.NewFileTracker()
	registry := tools.NewRegistry()
	registerBuiltinTools(registry, effectiveProject, tracker)
	permissions := tools.NewPermissionEnforcer(tools.PermDefault)
	executor := tools.NewExecutor(registry, permissions)

	// Deferred sender lets the manager exist before the program,
	// satisfying the NewConnectionManager(config, sender) API
	// while the program is still being constructed around an
	// App that already knows its ConnSurface.
	sender := &deferredSender{}
	cm := harness.NewConnectionManager(connCfg, sender)
	defer cm.Disconnect()

	// Resolve submit key from flags (none yet) → config → default.
	effectiveSubmitKey := cfg.Harness.SubmitKey
	if effectiveSubmitKey == "" {
		effectiveSubmitKey = harness.SubmitKeyEnter
	}
	// Validate; warn and fall back to enter on unknown.
	switch effectiveSubmitKey {
	case harness.SubmitKeyEnter, harness.SubmitKeyCtrlEnter, harness.SubmitKeyEscEnter:
		// ok
	default:
		fmt.Fprintf(os.Stderr, "Warning: unknown harness.submit_key %q, defaulting to enter\n", effectiveSubmitKey)
		effectiveSubmitKey = harness.SubmitKeyEnter
	}

	app := harness.NewApp(harness.AppOptions{
		SubmitKey: effectiveSubmitKey,
		Conn:      harness.WrapConnectionManager(cm),
		Attacher:  harness.NewContextManager(effectiveProject, tracker),
	})

	// Initialize the dynamic system theme from terminal background
	// detection and propagate it to all UI components.
	theme.InitSystemTheme()
	app.SetTheme(theme.CurrentTheme())

	// Plan-mode interception is a policy decorator on the
	// dispatcher: when the App's mode == plan, mutating tool.calls
	// get queued onto the PlanAccumulator and a synthetic
	// tool.result comes back instead of executing.
	dispatcher := harness.NewToolDispatcher(
		executor,
		newToolResultSender(cm),
		app.Plan(),
		app.State(),
	)
	dispatcher.SetObserver(newToolActivityObserver(sender))
	cm.SetToolDispatcher(dispatcher)
	if flags.resumeID != "" {
		app.State().SessionID = flags.resumeID
	}
	if flags.modelName != "" {
		app.State().ModelName = flags.modelName
		app.State().ModelOverride = true
	}

	program := tea.NewProgram(app, tea.WithAltScreen(), tea.WithFilter(harness.TerminalResponseFilter))
	sender.program.Store(program)

	// Trigger the lifecycle asynchronously so the UI renders
	// immediately; ConnStateMsg events drive the banner.
	go func() { _ = cm.ConnectSync(context.Background()) }()

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("harness exited: %w", err)
	}
	return nil
}
