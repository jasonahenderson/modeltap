package cli

import (
	"fmt"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/spf13/cobra"
)

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reverse proxy",
		Long:  `Start the modeltap reverse proxy server that intercepts and logs AI API traffic.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, v, err := config.LoadWithViper("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			// Bind flags to viper so flags > env > file > defaults.
			if cmd.Flags().Changed("port") {
				v.Set("port", cmd.Flags().Lookup("port").Value.String())
			}
			if cmd.Flags().Changed("upstream") {
				v.Set("upstream", cmd.Flags().Lookup("upstream").Value.String())
			}

			// Re-unmarshal after flag binding.
			if cmd.Flags().Changed("port") || cmd.Flags().Changed("upstream") {
				cfg, err = config.UnmarshalFrom(v)
				if err != nil {
					return fmt.Errorf("applying flag overrides: %w", err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "start: not implemented yet (port=%d, upstream=%s)\n", cfg.Port, cfg.Upstream)
			return nil
		},
	}

	cmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	cmd.Flags().StringP("upstream", "u", "https://api.anthropic.com", "Upstream API URL")

	return cmd
}
