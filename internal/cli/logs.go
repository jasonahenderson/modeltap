package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "List captured request/response logs",
		Long:  `List captured request/response log entries with optional filtering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "logs: not implemented yet")
			return nil
		},
	}

	return cmd
}
