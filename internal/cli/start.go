package cli

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/proxy"
	"github.com/jasonahenderson/modeltap/internal/storage"
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

			// Create storage.
			store, err := storage.NewSQLiteStore(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("creating store: %w", err)
			}
			defer store.Close()

			// Create provider registry with known providers.
			registry := provider.NewRegistry()
			registry.Register(provider.NewAnthropicProvider())
			registry.Register(provider.NewOpenAIProvider())

			srv, err := proxy.NewServer(proxy.ServerConfig{
				Port:        cfg.Port,
				UpstreamURL: cfg.Upstream,
				Store:       store,
				Registry:    registry,
			})
			if err != nil {
				return fmt.Errorf("creating proxy server: %w", err)
			}

			// Set up signal handling for graceful shutdown.
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			fmt.Fprintf(cmd.OutOrStdout(), "modeltap proxy listening on :%d -> %s\n", srv.Port(), srv.UpstreamURL())

			// Start server in a goroutine.
			errCh := make(chan error, 1)
			go func() {
				if err := srv.Start(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
				close(errCh)
			}()

			// Wait for signal or server error.
			select {
			case <-ctx.Done():
				stop()
				fmt.Fprintf(cmd.OutOrStdout(), "\nshutting down gracefully...\n")
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := srv.Shutdown(shutdownCtx); err != nil {
					return fmt.Errorf("shutdown: %w", err)
				}
				return nil
			case err := <-errCh:
				if err != nil {
					return fmt.Errorf("server error: %w", err)
				}
				return nil
			}
		},
	}

	cmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	cmd.Flags().StringP("upstream", "u", "https://api.anthropic.com", "Upstream API URL")

	return cmd
}
