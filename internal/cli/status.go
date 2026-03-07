package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show proxy and database status",
		Long:  `Display the current status of the modeltap proxy server and database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "status: not implemented yet")
			return nil
		},
	}

	return cmd
}
