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
		Short: "AI API reverse-proxy that captures and analyzes LLM traffic",
		Long: `modeltap is a lightweight reverse-proxy for AI/LLM API providers such as
Anthropic and OpenAI. It sits between your application and the upstream API,
transparently capturing every request/response pair. Captured traffic is
stored locally in SQLite and can be viewed, filtered, exported, and analyzed
through the CLI or a built-in web dashboard.

Key capabilities:
  - Transparent proxying with zero code changes in your application
  - Per-request token counting and cost estimation
  - Filtering and search across provider, model, status, and time range
  - Export to JSONL or CSV for downstream analysis
  - Aggregated usage metrics grouped by provider, model, day, or hour
  - Built-in web dashboard for visual exploration`,
		Version: version,
		Example: `  # Start the proxy on the default port (8080)
  modeltap start

  # Start with a custom port and enable the dashboard
  modeltap start --port 9090 --dashboard

  # View recent captured logs
  modeltap logs --limit 20

  # Show full detail for a specific request
  modeltap show abc12345

  # Export last 24 hours of logs as CSV
  modeltap export --format csv --since 24h

  # View usage metrics grouped by model
  modeltap metrics --group-by model

  # Check proxy and database status
  modeltap status`,
	}

	// Set version template so --version prints cleanly.
	rootCmd.SetVersionTemplate(fmt.Sprintf("modeltap %s\n", version))

	// Register all subcommands.
	rootCmd.AddCommand(
		newStartCommand(),
		newHarnessCommand(),
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
