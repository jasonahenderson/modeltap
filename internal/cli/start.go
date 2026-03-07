package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reverse proxy",
		Long:  `Start the modeltap reverse proxy server that intercepts and logs AI API traffic.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "start: not implemented yet")
			return nil
		},
	}

	return cmd
}
