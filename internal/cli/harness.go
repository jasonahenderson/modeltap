package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harness/tools"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/spf13/cobra"
)

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

// newHarnessCommand wires the terminal harness into the modeltap CLI
// (WU-089). It composes the Bubbletea App (Bundle 5 + Bundle 7 tools
// + Bundle 13 overlays) with a live ConnectionManager that speaks
// JSON-RPC to the BFF, auto-starting `modeltap start` when the
// configured socket is absent so the harness works from a cold boot.
func newHarnessCommand() *cobra.Command {
	var (
		socketPath string
		resumeID   string
		project    string
		modelName  string
		noAutoStart bool
	)

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
			cfg, _, err := config.LoadWithViper("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			effectiveSocket := socketPath
			if effectiveSocket == "" {
				effectiveSocket = cfg.BFF.SocketPath
			}
			if effectiveSocket == "" {
				effectiveSocket = config.DefaultBFFSocketPath()
			}

			effectiveProject := project
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
				AutoStart:    !noAutoStart,
				ServerBinary: serverBinary,
				ServerArgs:   []string{"start"},
				Registration: &protocol.CapabilitiesRegister{
					ProtocolVersion: "1",
					HarnessVersion:  cmd.Root().Version,
					HarnessPlatform: "terminal",
					Project:         protocol.ProjectContext{Root: effectiveProject},
				},
			}

			// Build shared tool framework pieces so the harness can
			// resolve @file attachments and (future) run local tools.
			tracker := tools.NewFileTracker()

			// Deferred sender lets the manager exist before the program,
			// satisfying the NewConnectionManager(config, sender) API
			// while the program is still being constructed around an
			// App that already knows its ConnSurface.
			sender := &deferredSender{}
			cm := harness.NewConnectionManager(connCfg, sender)
			defer cm.Disconnect()

			app := harness.NewApp(harness.AppOptions{
				Conn:     harness.WrapConnectionManager(cm),
				Attacher: harness.NewContextManager(effectiveProject, tracker),
			})
			if resumeID != "" {
				app.State().SessionID = resumeID
			}
			if modelName != "" {
				app.State().ModelName = modelName
				app.State().ModelOverride = true
			}

			program := tea.NewProgram(app, tea.WithAltScreen())
			sender.program.Store(program)

			// Trigger the lifecycle asynchronously so the UI renders
			// immediately; ConnStateMsg events drive the banner.
			go func() { _ = cm.ConnectSync(context.Background()) }()

			if _, err := program.Run(); err != nil {
				return fmt.Errorf("harness exited: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&socketPath, "socket", "", "override the BFF socket path (defaults to config.bff.socket_path)")
	cmd.Flags().StringVar(&resumeID, "resume", "", "resume the given session id at startup")
	cmd.Flags().StringVar(&project, "project", "", "project directory (defaults to $PWD)")
	cmd.Flags().StringVar(&modelName, "model", "", "initial model override for the session")
	cmd.Flags().BoolVar(&noAutoStart, "no-auto-start", false, "do not auto-start modeltap start when the socket is absent")

	return cmd
}
