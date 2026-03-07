package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMetricsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show usage metrics",
		Long:  `Display aggregated usage metrics for captured API traffic.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "metrics: not implemented yet")
			return nil
		},
	}

	cmd.AddCommand(newMetricsRebuildCommand())

	return cmd
}

func newMetricsRebuildCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild metrics from stored logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "metrics rebuild: not implemented yet")
			return nil
		},
	}
}
