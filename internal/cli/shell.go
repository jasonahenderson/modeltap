package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/harness"
	"github.com/jasonahenderson/modeltap/internal/harnesshost"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/spf13/cobra"
)

// shellFlags captures the production shell's CLI flags. Defaults
// follow the table in the WU-105 design (production-wiring §"Flag
// defaults"): --socket → viper bff.socket_path → built-in default;
// --project → cwd; --model → viper default_model; --resume → empty.
type shellFlags struct {
	socketPath string
	resumeID   string
	project    string
	modelName  string
}

func bindShellFlags(cmd *cobra.Command, f *shellFlags) {
	cmd.Flags().StringVar(&f.socketPath, "socket", "", "override the BFF socket path (defaults to config.bff.socket_path)")
	cmd.Flags().StringVar(&f.resumeID, "resume", "", "resume the given session id at startup")
	cmd.Flags().StringVar(&f.project, "project", "", "project directory (defaults to $PWD)")
	cmd.Flags().StringVar(&f.modelName, "model", "", "initial model override for the session")
}

// newShellCommand registers the `modeltap shell` subcommand. It is
// the production conversation-shell entrypoint that wraps
// harnessshell.Model + harnesshost.Adapter + harnesshost.ProductionRuntime
// and runs as a tea.Program against a real BFF.
//
// Replaces the legacy `modeltap harness` command (deleted in v0.2.1
// when the legacy TUI App was scrapped).
func newShellCommand() *cobra.Command {
	var flags shellFlags

	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Launch the modeltap conversation shell against the BFF",
		Long: `Start the modeltap conversation shell — a Bubble Tea TUI built on
the reusable internal/harnessshell component plus the modeltap host
adapter (internal/harnesshost). Connects to the BFF over a local
unix socket (auto-starting the BFF when the socket is absent) and
drives sessions, tools, attachments, and routing commands.

Slash commands:
  /exit, /quit            exit the shell
  /clear                  clear the transcript (shell-local)
  /plan, /build, /auto    switch execution mode
  /model                  show current model
  /model <name>           switch model
  /models                 list available models
  /session                show / list sessions
  /session resume <id>    resume a session
  /session clear          clear the current session's context
  /session fork           fork the current session
  /sessions               list sessions
  /context                show current context window breakdown
  /compact                request session compaction (stubbed in v0.2.2)
  /history                show command history (stubbed in v0.2.2)
  /mcp                    MCP server status (stubbed in v0.2.2)`,
		Example: `  # Launch the shell against the default socket
  modeltap shell

  # Resume a specific session
  modeltap shell --resume 7d9f...

  # Override the initial model
  modeltap shell --model claude-opus-4-7

  # Point at a specific project directory
  modeltap shell --project /abs/path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShell(cmd, &flags)
		},
	}

	bindShellFlags(cmd, &flags)
	return cmd
}

// runShell composes config → ProductionRuntime → Adapter → tea.Program
// and blocks until the program exits.
//
// AttachProgram + Start ordering (per Phase 2 review Kimi #6):
// tea.NewProgram(adapter) → runtime.AttachProgram(p) → go runtime.Start
// → p.Run(). AttachProgram runs synchronously before any background
// connection work spawns, so early ConnStateMsg / projection events
// reach the deferredSender after the program reference is set.
func runShell(cmd *cobra.Command, flags *shellFlags) error {
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
		AutoStart:    true,
		ServerBinary: serverBinary,
		ServerArgs:   []string{"start"},
		Registration: &protocol.CapabilitiesRegister{
			ProtocolVersion: "1",
			HarnessVersion:  cmd.Root().Version,
			HarnessPlatform: "terminal",
			Project:         protocol.ProjectContext{Root: effectiveProject},
		},
	}

	runtime, err := harnesshost.NewProductionRuntime(harnesshost.ProductionRuntimeConfig{
		ConnConfig:   connCfg,
		ProjectRoot:  effectiveProject,
		Registration: connCfg.Registration,
	})
	if err != nil {
		return fmt.Errorf("constructing runtime: %w", err)
	}
	defer runtime.Close()

	label := flags.modelName
	if label == "" {
		// Fall back to a friendly placeholder; the runtime updates the
		// label on first /model command or on RunStartedEvent.
		label = "modeltap"
	}

	shell := harnessshell.New(
		harnessshell.WithTitle("modeltap"),
		harnessshell.WithLabel(label),
		harnessshell.WithPlaceholder("Type a message and press Enter."),
	)
	adapter := harnesshost.New(shell, runtime)

	p := tea.NewProgram(adapter, tea.WithAltScreen(), tea.WithMouseAllMotion())

	// Ordering rule: AttachProgram BEFORE Start.
	runtime.AttachProgram(p)

	go func() { _ = runtime.Start(context.Background()) }()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("shell exited: %w", err)
	}
	return nil
}
