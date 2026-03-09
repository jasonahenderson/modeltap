package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jasonahenderson/modeltap/internal/config"
	"github.com/jasonahenderson/modeltap/internal/service"
	"github.com/spf13/cobra"
)

func newServiceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage modeltap as a background service",
		Long: `Manage the modeltap proxy as a platform-native background service.

On macOS, this creates a launchd user agent that starts automatically at login.
On Linux, this creates a systemd user service that starts automatically.

Use "modeltap service install" to set up the service and
"modeltap service uninstall" to remove it.`,
	}

	cmd.AddCommand(newServiceInstallCommand())
	cmd.AddCommand(newServiceUninstallCommand())

	return cmd
}

func newServiceInstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install modeltap as a background service",
		Long: `Install the modeltap proxy as a platform-native background service.

On macOS, this writes a launchd plist to ~/Library/LaunchAgents/ and loads it.
On Linux, this writes a systemd unit to ~/.config/systemd/user/ and enables it.

The service is configured to start automatically at login and restart on failure.`,
		Example: `  # Install as a background service
  modeltap service install`,
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
unregisters it from the platform's service manager.`,
		Example: `  # Remove the background service
  modeltap service uninstall`,
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
