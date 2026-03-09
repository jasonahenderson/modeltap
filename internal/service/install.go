package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Install generates the platform-specific service definition, writes it to
// the correct location, and starts the service using the platform's service
// manager.
func Install(platform Platform, cfg Config) error {
	content, err := GenerateServiceFile(platform, cfg)
	if err != nil {
		return fmt.Errorf("generating service file: %w", err)
	}

	path, err := ServiceFilePath(platform)
	if err != nil {
		return fmt.Errorf("resolving service file path: %w", err)
	}

	// Create parent directories if needed.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", filepath.Dir(path), err)
	}

	// Write the service file.
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing service file %s: %w", path, err)
	}

	// Load/enable the service.
	switch platform {
	case PlatformMacOS:
		return installLaunchd(path)
	case PlatformLinux:
		return installSystemd()
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}
}

// Uninstall stops and removes the platform-specific service definition.
func Uninstall(platform Platform) error {
	path, err := ServiceFilePath(platform)
	if err != nil {
		return fmt.Errorf("resolving service file path: %w", err)
	}

	switch platform {
	case PlatformMacOS:
		return uninstallLaunchd(path)
	case PlatformLinux:
		return uninstallSystemd(path)
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}
}

// installLaunchd loads the plist via launchctl. It tries bootstrap first
// (modern macOS) and falls back to load -w.
func installLaunchd(plistPath string) error {
	// Try bootstrap first (macOS 10.10+).
	uid := fmt.Sprintf("gui/%d", os.Getuid())
	cmd := exec.Command("launchctl", "bootstrap", uid, plistPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fall back to load -w (legacy).
	cmd = exec.Command("launchctl", "load", "-w", plistPath)
	stderr.Reset()
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launchctl load: %s: %w", stderr.String(), err)
	}
	return nil
}

// installSystemd reloads the systemd user daemon and enables the service.
func installSystemd() error {
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	return runSystemctl("enable", "--now", "modeltap")
}

// uninstallLaunchd unloads the plist and removes the file.
func uninstallLaunchd(plistPath string) error {
	// Try bootout first (modern macOS).
	uid := fmt.Sprintf("gui/%d", os.Getuid())
	cmd := exec.Command("launchctl", "bootout", uid, plistPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Fall back to unload (legacy).
		cmd = exec.Command("launchctl", "unload", plistPath)
		stderr.Reset()
		cmd.Stderr = &stderr
		// Ignore errors from unload — the service may not be loaded.
		_ = cmd.Run()
	}

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing service file %s: %w", plistPath, err)
	}
	return nil
}

// uninstallSystemd disables the service, removes the unit file, and reloads.
func uninstallSystemd(unitPath string) error {
	// Disable and stop the service. Ignore errors — it may not be running.
	_ = runSystemctl("disable", "--now", "modeltap")

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing service file %s: %w", unitPath, err)
	}

	return runSystemctl("daemon-reload")
}

// runSystemctl runs a systemctl --user command and returns any error with
// stderr context.
func runSystemctl(args ...string) error {
	fullArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", fullArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %v: %s: %w", args, stderr.String(), err)
	}
	return nil
}
