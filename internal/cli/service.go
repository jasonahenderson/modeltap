package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/service"
	"github.com/spf13/cobra"
)

// Default number of log lines to display.
const defaultLogLines = 50

func newServiceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage modeltap as a background service",
		Long: `Manage the modeltap proxy as a platform-native background service.

On macOS, this creates a launchd user agent that starts automatically at login.
On Linux, this creates a systemd user service that starts automatically.

Use "modeltap service install" to set up the service and
"modeltap service uninstall" to remove it.`,
		Example: `  # Install the proxy as a background service
  modeltap service install

  # Check whether the service is running
  modeltap service status

  # View recent service logs
  modeltap service logs

  # Remove the background service
  modeltap service uninstall`,
	}

	cmd.AddCommand(newServiceInstallCommand())
	cmd.AddCommand(newServiceUninstallCommand())
	cmd.AddCommand(newServiceStatusCommand())
	cmd.AddCommand(newServiceLogsCommand())

	return cmd
}

func newServiceInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install modeltap as a background service",
		Long: `Install the modeltap proxy as a platform-native background service.

On macOS, this writes a launchd plist to ~/Library/LaunchAgents/ and loads it.
On Linux, this writes a systemd unit to ~/.config/systemd/user/ and enables it.

The service is configured to start automatically at login and restart on failure.
The installed service uses your current modeltap binary and configuration file.
After installation, no terminal window is required -- the proxy runs persistently.`,
		Example: `  # Install as a background service
  modeltap service install

  # Verify it is running
  modeltap service status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			platform := service.DetectPlatform()
			if platform == service.PlatformUnsupported {
				return fmt.Errorf("service management is not supported on this platform (%s)", platform)
			}

			// Resolve the binary path.
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolving executable path: %w", err)
			}
			binaryPath, err := filepath.EvalSymlinks(executable)
			if err != nil {
				return fmt.Errorf("resolving symlinks for executable: %w", err)
			}

			configPath := config.DefaultConfigPath()

			cfg := service.Config{
				BinaryPath: binaryPath,
				ConfigPath: configPath,
			}

			if err := service.Install(platform, cfg); err != nil {
				return fmt.Errorf("installing service: %w", err)
			}

			servicePath, _ := service.ServiceFilePath(platform)
			fmt.Fprintf(cmd.OutOrStdout(), "Service installed successfully.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Service file: %s\n", servicePath)
			fmt.Fprintf(cmd.OutOrStdout(), "  Binary:       %s\n", binaryPath)
			fmt.Fprintf(cmd.OutOrStdout(), "  Config:       %s\n", configPath)
			fmt.Fprintf(cmd.OutOrStdout(), "\nThe modeltap proxy is now running as a background service.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Use \"modeltap service uninstall\" to remove it.\n")

			return nil
		},
	}
}

func newServiceUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the modeltap background service",
		Long: `Uninstall the modeltap background service.

This stops the running service, removes the service definition file, and
unregisters it from the platform's service manager. Your configuration
and captured data are not affected.`,
		Example: `  # Remove the background service
  modeltap service uninstall

  # Verify it has been removed
  modeltap service status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			platform := service.DetectPlatform()
			if platform == service.PlatformUnsupported {
				return fmt.Errorf("service management is not supported on this platform (%s)", platform)
			}

			if err := service.Uninstall(platform); err != nil {
				return fmt.Errorf("uninstalling service: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Service uninstalled successfully.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "The modeltap background service has been stopped and removed.\n")

			return nil
		},
	}
}

func newServiceStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the status of the modeltap background service",
		Long: `Show whether the modeltap background service is installed and running.

Displays the installation state, running status, and process ID (if running).
Use this to verify the service is healthy after installation or to confirm
it has been removed after uninstallation.`,
		Example: `  # Check service status
  modeltap service status

  # Typical output when running:
  #   Service: installed
  #   Status:  running
  #   PID:     12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			platform := service.DetectPlatform()
			if platform == service.PlatformUnsupported {
				return fmt.Errorf("service management is not supported on this platform (%s)", platform)
			}

			status, err := service.Status(platform)
			if err != nil {
				return fmt.Errorf("checking service status: %w", err)
			}

			if status.Installed {
				fmt.Fprintf(cmd.OutOrStdout(), "Service: installed\n")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Service: not installed\n")
			}

			if status.Running {
				fmt.Fprintf(cmd.OutOrStdout(), "Status:  running\n")
				fmt.Fprintf(cmd.OutOrStdout(), "PID:     %d\n", status.PID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Status:  stopped\n")
			}

			return nil
		},
	}
}

func newServiceLogsCommand() *cobra.Command {
	var lines int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show recent logs from the modeltap background service",
		Long: `Display recent log output from the modeltap background service.

On macOS, reads from the log file at ~/.modeltap/modeltap.log
(or the legacy ~/.config/modeltap/modeltap.log for installs predating PATCH-0006).
On Linux, reads from journalctl for the modeltap user service.

Use the --lines (-n) flag to control how many lines are displayed.`,
		Example: `  # Show last 50 lines (default)
  modeltap service logs

  # Show last 100 lines
  modeltap service logs --lines 100
  modeltap service logs -n 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			platform := service.DetectPlatform()
			if platform == service.PlatformUnsupported {
				return fmt.Errorf("service management is not supported on this platform (%s)", platform)
			}

			output, err := service.Logs(platform, lines)
			if err != nil {
				return fmt.Errorf("retrieving service logs: %w", err)
			}

			fmt.Fprint(cmd.OutOrStdout(), output)
			if output != "" && output[len(output)-1] != '\n' {
				fmt.Fprintln(cmd.OutOrStdout())
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", defaultLogLines, "number of log lines to display")

	return cmd
}
