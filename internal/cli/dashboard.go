package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/spf13/cobra"
)

func newDashboardCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Open web dashboard in browser",
		Long: `Open the modeltap web dashboard in the default browser.

Reads the dashboard bind address and port from the configuration file,
constructs the URL, and attempts to launch the default browser. The
dashboard must already be running (start the proxy with --dashboard or
configure dashboard.enabled in the config file).

If the browser cannot be opened automatically, the URL is printed so
you can copy and paste it manually.`,
		Example: `  # Open the dashboard in the default browser
  modeltap dashboard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load("")
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			bind := cfg.Dashboard.Bind
			if bind == "" {
				bind = "127.0.0.1"
			}
			port := cfg.Dashboard.Port
			if port == 0 {
				port = 8081
			}

			url := fmt.Sprintf("http://%s:%d", bind, port)
			fmt.Fprintf(cmd.OutOrStdout(), "Dashboard: %s\n", url)

			// Attempt to open in default browser.
			if err := openBrowser(url); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Could not open browser automatically. Please visit the URL above.\n")
			}

			return nil
		},
	}

	return cmd
}

// openBrowser attempts to open a URL in the default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
