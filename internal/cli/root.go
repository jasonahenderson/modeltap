// Package cli defines the Cobra command tree for the modeltap CLI.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCommand creates and returns the root cobra.Command for modeltap.
// The version string is injected by the caller (typically main.go).
func NewRootCommand(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "modeltap",
		Short: "AI API reverse-proxy with a reusable conversation shell",
		Long: `modeltap is a reverse proxy for AI/LLM APIs and a reusable Bubble Tea
conversation-shell component for driving those providers.

The legacy interactive harness (` + "`modeltap harness`" + `) was scrapped in v0.2.1.
The post-extraction conversation surface lives in ` + "`internal/harnessshell`" + `
and is exercised end-to-end by ` + "`modeltap shell-demo`" + ` against the fake
runtime in ` + "`internal/harnessdemo`" + `. Production provider integration via
` + "`harnesshost`" + ` ships in a follow-up release.

Subcommands control the proxy and tooling — ` + "`start`" + ` to run the proxy
server, ` + "`logs`" + ` / ` + "`show`" + ` / ` + "`export`" + ` / ` + "`metrics`" + ` to inspect captured
traffic, ` + "`dashboard`" + ` for the web UI, ` + "`status`" + ` / ` + "`service`" + ` / ` + "`config`" + ` /
` + "`completion`" + ` for administrative tasks, and ` + "`shell-demo`" + ` for the
extracted conversation shell with a fake backend.

Key capabilities:
  - Reusable conversation shell (FEAT-0014: single scrolling surface,
    composer-hosted permissions, queued follow-ups, paste-token expansion)
  - Transparent proxying with zero code changes in your application
  - Per-request token counting and cost estimation
  - Filtering and search across provider, model, status, and time range
  - Export to JSONL or CSV for downstream analysis
  - Aggregated usage metrics grouped by provider, model, day, or hour
  - Built-in web dashboard for visual exploration`,
		Version: version,
		Example: `  # Launch the conversation-shell demo (fake backend)
  modeltap shell-demo

  # Start the proxy server
  modeltap start --port 9090 --dashboard

  # View recent captured logs
  modeltap logs --limit 20

  # Export last 24 hours of logs as CSV
  modeltap export --format csv --since 24h

  # Check proxy and database status
  modeltap status`,
		SilenceUsage: true,
	}

	// Set version template so --version prints cleanly.
	rootCmd.SetVersionTemplate(fmt.Sprintf("modeltap %s\n", version))

	// Register all subcommands.
	rootCmd.AddCommand(
		newStartCommand(),
		newShellDemoCommand(),
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
