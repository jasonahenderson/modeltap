package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDashboardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Open web dashboard in browser",
		Long:  `Open the modeltap web dashboard in the default browser.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "dashboard: not implemented yet")
			return nil
		},
	}

	return cmd
}
