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
		Long: `modeltap is a reverse-proxy for AI/LLM API providers.
It captures request/response pairs, stores them locally, and provides
tools for viewing, exporting, and analyzing API usage.`,
		Version: version,
	}

	// Set version template so --version prints cleanly.
	rootCmd.SetVersionTemplate(fmt.Sprintf("modeltap %s\n", version))

	// Register all subcommands.
	rootCmd.AddCommand(
		newStartCommand(),
		newLogsCommand(),
		newShowCommand(),
		newExportCommand(),
		newConfigCommand(),
		newStatusCommand(),
		newMetricsCommand(),
		newDashboardCommand(),
		newCompletionCommand(),
	)

	return rootCmd
}
