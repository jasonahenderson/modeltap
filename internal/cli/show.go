package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show detail of a captured request",
		Long:  `Show the full detail of a captured request/response pair by its ID.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "show %s: not implemented yet\n", args[0])
			return nil
		},
	}

	return cmd
}
