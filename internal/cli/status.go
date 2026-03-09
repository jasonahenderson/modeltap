package cli

import (
	"context"
	"fmt"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/provider"
	"github.com/jasonahenderson/modeltap/internal/storage"
	"github.com/spf13/cobra"
)

// statusStore is a package-level variable that allows injecting a Store for
// the status command. In production this is set via SetStatusStore before
// command execution; in tests it is set directly.
var statusStore storage.Store

// statusConfig is a package-level variable that allows injecting a Config for
// the status command. In production this may be set via SetStatusConfig; if
// nil the command loads config from disk.
var statusConfig *config.Config

// statusRegistry is a package-level variable for the provider registry.
// If nil, a default registry with anthropic and openai is used.
var statusRegistry *provider.Registry

// SetStatusStore sets the store used by the status command.
func SetStatusStore(s storage.Store) {
	statusStore = s
}

// SetStatusConfig sets the config used by the status command.
func SetStatusConfig(cfg *config.Config) {
	statusConfig = cfg
}

// SetStatusRegistry sets the provider registry used by the status command.
func SetStatusRegistry(r *provider.Registry) {
	statusRegistry = r
}

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show proxy and database status",
		Long: `Display the current status of the modeltap proxy server and database.

Shows the configured proxy port and upstream URL, the database file path
and total record count, the retention policy, and the list of registered
API providers. This is useful for verifying your configuration before
starting the proxy or diagnosing connectivity issues.

Tip: For persistent background execution that survives restarts and
starts automatically at login, use "modeltap service install".`,
		Example: `  # Show current status
  modeltap status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// Load config.
			cfg := statusConfig
			if cfg == nil {
				var err error
				cfg, err = config.Load("")
				if err != nil {
					return fmt.Errorf("loading config: %w", err)
				}
			}

			// Proxy section.
			fmt.Fprintln(w, "Proxy")
			fmt.Fprintf(w, "  Port:     %d\n", cfg.Port)
			fmt.Fprintf(w, "  Upstream: %s\n", cfg.Upstream)
			fmt.Fprintln(w)

			// Database section.
			fmt.Fprintln(w, "Database")
			fmt.Fprintf(w, "  Path:    %s\n", cfg.DBPath)
			if statusStore != nil {
				count, err := statusStore.CountRequests(context.Background(), storage.ListFilter{})
				if err != nil {
					return fmt.Errorf("counting records: %w", err)
				}
				fmt.Fprintf(w, "  Records: %s\n", formatCount(count))
			} else {
				fmt.Fprintf(w, "  Records: %s\n", "N/A")
			}
			fmt.Fprintln(w)

			// Retention section.
			fmt.Fprintln(w, "Retention")
			fmt.Fprintf(w, "  Days: %d\n", cfg.RetentionDays)
			fmt.Fprintln(w)

			// Providers section.
			fmt.Fprintln(w, "Providers")
			reg := statusRegistry
			if reg != nil {
				for _, p := range reg.All() {
					fmt.Fprintf(w, "  - %s\n", p.Name())
				}
			} else {
				// Default known providers.
				fmt.Fprintln(w, "  - anthropic")
				fmt.Fprintln(w, "  - openai")
			}

			return nil
		},
	}

	return cmd
}

// formatCount formats an integer with comma separators for readability.
func formatCount(n int64) string {
	if n < 0 {
		return fmt.Sprintf("-%s", formatCount(-n))
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	// Insert commas from the right.
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
