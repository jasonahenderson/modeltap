package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export logs to JSONL or CSV",
		Long:  `Export captured request/response logs to JSONL or CSV format.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "export: not implemented yet")
			return nil
		},
	}

	return cmd
}
