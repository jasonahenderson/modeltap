package cli

import (
	"fmt"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `View and manage modeltap configuration settings.`,
	}

	cmd.AddCommand(
		newConfigShowCommand(),
		newConfigSetCommand(),
		newConfigPathCommand(),
	)

	return cmd
}

func newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			yamlStr, err := cfg.YAML()
			if err != nil {
				return fmt.Errorf("formatting config: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), yamlStr)
			return nil
		},
	}
}

func newConfigSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "config set %s=%s: not implemented yet\n", args[0], args[1])
			return nil
		},
	}
}

func newConfigPathCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), config.DefaultConfigPath())
			return nil
		},
	}
}
