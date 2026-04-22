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
		Long: `View and manage modeltap configuration settings.

Configuration is stored in a YAML file and can also be overridden by
environment variables or CLI flags. Use the subcommands to inspect the
current configuration, update individual values, or locate the config
file on disk.`,
		Example: `  # Show the current configuration
  modeltap config show

  # Print the config file path
  modeltap config path

  # Set a configuration value
  modeltap config set port 9090`,
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
		Long: `Display the full resolved configuration as YAML.

Shows all settings including port, upstream URL, database path, retention
policy, provider configuration, pricing overrides, and dashboard settings.
Values reflect the merged result of defaults, config file, and environment
variables (CLI flags are not included since the proxy is not running).`,
		Example: `  # Print the current configuration
  modeltap config show`,
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
		Long: `Set a configuration value in the modeltap config file.

Accepts a dotted key path and a value. The change is written to the config
file so it persists across restarts.`,
		Example: `  # Change the proxy port
  modeltap config set port 9090

  # Set the upstream URL
  modeltap config set upstream https://api.openai.com

  # Set retention days
  modeltap config set retention_days 90`,
		Args: cobra.ExactArgs(2),
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
		Long: `Print the absolute path to the modeltap configuration file.

This is useful for scripting or for quickly locating the file to edit
it manually.`,
		Example: `  # Print the config file path
  modeltap config path

  # Open the config file in your editor
  $EDITOR $(modeltap config path)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), config.DefaultConfigPath())
			return nil
		},
	}
}
