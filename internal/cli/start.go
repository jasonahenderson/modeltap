package cli

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/dashboard"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/proxy"
	"github.com/jasonahenderson/modeltap/internal/storage"
	"github.com/spf13/cobra"
)

func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the reverse proxy",
		Long: `Start the modeltap reverse proxy server that intercepts and logs AI API traffic.

The proxy listens on the configured port (default 8080) and forwards all
requests to the upstream AI API provider. Every request/response pair is
captured and stored in the local SQLite database for later inspection.

Configuration is resolved in priority order: CLI flags > environment
variables > config file > built-in defaults. The proxy handles graceful
shutdown on SIGINT/SIGTERM.

Optionally enable the built-in web dashboard with --dashboard to get a
browser-based view of captured traffic while the proxy is running.`,
		Example: `  # Start with default settings (port 8080, upstream https://api.anthropic.com)
  modeltap start

  # Listen on a custom port
  modeltap start --port 9090

  # Proxy to OpenAI instead of Anthropic
  modeltap start --upstream https://api.openai.com

  # Start with the web dashboard enabled
  modeltap start --dashboard

  # Start with dashboard on a custom port
  modeltap start --dashboard --dashboard-port 3000`,
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
			if cmd.Flags().Changed("dashboard") {
				v.Set("dashboard.enabled", true)
			}
			if cmd.Flags().Changed("dashboard-port") {
				v.Set("dashboard.port", cmd.Flags().Lookup("dashboard-port").Value.String())
			}

			// Re-unmarshal after flag binding.
			needsRemarshal := cmd.Flags().Changed("port") || cmd.Flags().Changed("upstream") ||
				cmd.Flags().Changed("dashboard") || cmd.Flags().Changed("dashboard-port")
			if needsRemarshal {
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

			// Build provider-to-upstream mapping from config.
			providerUpstreams := make(map[string]string)
			for name, pcfg := range cfg.Providers {
				if pcfg.Upstream != "" {
					providerUpstreams[name] = pcfg.Upstream
				}
			}

			// Build pricing table from config (defaults + user overrides).
			pricing := config.NewPricingTableFromConfig(cfg.Pricing)

			srv, err := proxy.NewServer(proxy.ServerConfig{
				Port:              cfg.Port,
				UpstreamURL:       cfg.Upstream,
				Store:             store,
				Registry:          registry,
				ProviderUpstreams: providerUpstreams,
				Pricing:           pricing,
			})
			if err != nil {
				return fmt.Errorf("creating proxy server: %w", err)
			}

			// Set up signal handling for graceful shutdown.
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// Start dashboard if enabled.
			if cfg.Dashboard.Enabled {
				handler := dashboard.NewAPIHandler(store, cfg)
				dashCtx, dashCancel := context.WithCancel(ctx)
				defer dashCancel()
				go func() {
					if err := handler.ListenAndServe(dashCtx); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "dashboard error: %v\n", err)
					}
				}()
				dashBind := cfg.Dashboard.Bind
				if dashBind == "" {
					dashBind = "127.0.0.1"
				}
				dashPort := cfg.Dashboard.Port
				if dashPort == 0 {
					dashPort = 8081
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dashboard: http://%s:%d\n", dashBind, dashPort)
			}

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
	cmd.Flags().Bool("dashboard", false, "Enable the web dashboard")
	cmd.Flags().Int("dashboard-port", 8081, "Port for the web dashboard")

	return cmd
}
