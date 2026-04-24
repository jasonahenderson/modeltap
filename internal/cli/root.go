// Package cli defines the Cobra command tree for the modeltap CLI.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCommand creates and returns the root cobra.Command for modeltap.
// The version string is injected by the caller (typically main.go).
func NewRootCommand(version string) *cobra.Command {
	// rootFlags are the harness flags the root command accepts so
	// `modeltap --model X` works the same as `modeltap harness --model X`.
	var rootFlags harnessFlags

	rootCmd := &cobra.Command{
		Use:   "modeltap",
		Short: "AI API reverse-proxy and interactive terminal harness",
		Long: `modeltap is both a reverse proxy for AI/LLM APIs and an interactive
Bubbletea-based terminal harness for driving those providers.

Run with no subcommand to launch the interactive harness (the default),
which auto-starts a local BFF server when needed and connects over a
unix socket. Subcommands control the proxy directly — ` + "`start`" + ` to
run the proxy server, ` + "`logs`" + ` / ` + "`show`" + ` / ` + "`export`" + ` / ` + "`metrics`" + ` to
inspect captured traffic, ` + "`dashboard`" + ` for the web UI, ` + "`status`" + ` /
` + "`service`" + ` / ` + "`config`" + ` / ` + "`completion`" + ` for administrative tasks.

Key capabilities:
  - Interactive harness (session-aware, tool-enabled, MCP-ready)
  - Transparent proxying with zero code changes in your application
  - Per-request token counting and cost estimation
  - Filtering and search across provider, model, status, and time range
  - Export to JSONL or CSV for downstream analysis
  - Aggregated usage metrics grouped by provider, model, day, or hour
  - Built-in web dashboard for visual exploration`,
		Version: version,
		Example: `  # Launch the interactive harness (default)
  modeltap

  # Resume a specific session in the harness
  modeltap --resume 7d9f…

  # Start the proxy server explicitly
  modeltap start --port 9090 --dashboard

  # View recent captured logs
  modeltap logs --limit 20

  # Export last 24 hours of logs as CSV
  modeltap export --format csv --since 24h

  # Check proxy and database status
  modeltap status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// No subcommand → launch the harness. Unknown subcommands
			// are still caught by Cobra's own handling; this RunE only
			// fires for bare `modeltap` invocations.
			return runHarness(cmd, &rootFlags)
		},
		SilenceUsage: true,
	}

	bindHarnessFlags(rootCmd, &rootFlags)

	// Set version template so --version prints cleanly.
	rootCmd.SetVersionTemplate(fmt.Sprintf("modeltap %s\n", version))

	// Register all subcommands.
	rootCmd.AddCommand(
		newStartCommand(),
		newHarnessCommand(),
		newHarnessSpikeCommand(),
		newLogsCommand(),
		newShowCommand(),
		newExportCommand(),
		newConfigCommand(),
		newStatusCommand(),
		newMetricsCommand(),
		newDashboardCommand(),
		newCompletionCommand(),
		newServiceCommand(),
	)

	return rootCmd
}
