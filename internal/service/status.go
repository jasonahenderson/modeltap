package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ServiceStatus represents the current state of the modeltap service.
type ServiceStatus struct {
	// Installed indicates whether the service definition file exists.
	Installed bool
	// Running indicates whether the service process is currently active.
	Running bool
	// PID is the process ID of the running service, or 0 if not running.
	PID int
}

// Status checks whether the modeltap service is installed and running on the
// given platform.
func Status(platform Platform) (*ServiceStatus, error) {
	switch platform {
	case PlatformMacOS:
		return statusLaunchd()
	case PlatformLinux:
		return statusSystemd()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// statusLaunchd checks launchd service status on macOS.
func statusLaunchd() (*ServiceStatus, error) {
	status := &ServiceStatus{}

	// Check if the plist file exists.
	path, err := ServiceFilePath(PlatformMacOS)
	if err != nil {
		return nil, fmt.Errorf("resolving service file path: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		status.Installed = true
	}

	// Check if the service is loaded by querying launchctl list.
	cmd := exec.Command("launchctl", "list", "com.modeltap.proxy")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// Service is not loaded.
		return status, nil
	}

	// Parse launchctl list output. The output format is:
	//   "PID"	Status	Label
	// where PID is "-" if the process is not running.
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "com.modeltap.proxy" {
			if fields[0] != "-" {
				pid, err := strconv.Atoi(fields[0])
				if err == nil {
					status.Running = true
					status.PID = pid
				}
			}
			break
		}
	}

	// If we got here without finding the label line, the service is loaded
	// but we check for a PID key-value pair (single-service output format).
	if !status.Running {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "\"PID\"") || strings.HasPrefix(trimmed, "PID") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 {
					pid, err := strconv.Atoi(strings.TrimSpace(strings.Trim(parts[1], ";\"")))
					if err == nil && pid > 0 {
						status.Running = true
						status.PID = pid
					}
				}
			}
		}
	}

	return status, nil
}

// statusSystemd checks systemd user service status on Linux.
func statusSystemd() (*ServiceStatus, error) {
	status := &ServiceStatus{}

	// Check if the unit file exists.
	path, err := ServiceFilePath(PlatformLinux)
	if err != nil {
		return nil, fmt.Errorf("resolving service file path: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		status.Installed = true
	}

	// Check if the service is active.
	cmd := exec.Command("systemctl", "--user", "is-active", "modeltap")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err == nil {
		if strings.TrimSpace(stdout.String()) == "active" {
			status.Running = true
		}
	}

	// Get the PID if running.
	if status.Running {
		cmd = exec.Command("systemctl", "--user", "show", "modeltap", "--property=MainPID")
		stdout.Reset()
		cmd.Stdout = &stdout
		if err := cmd.Run(); err == nil {
			// Output format: MainPID=12345
			output := strings.TrimSpace(stdout.String())
			if strings.HasPrefix(output, "MainPID=") {
				pid, err := strconv.Atoi(strings.TrimPrefix(output, "MainPID="))
				if err == nil && pid > 0 {
					status.PID = pid
				}
			}
		}
	}

	return status, nil
}

// LogFilePath returns the path to the modeltap log file on macOS.
// Prefers the new canonical location (~/.modeltap/modeltap.log or
// $XDG_DATA_HOME/modeltap/modeltap.log) but falls back to the v0.1-era
// ~/.config/modeltap/modeltap.log if that file already exists and the
// new one does not — mirrors the PATCH-0006 config resolver so a
// legacy install keeps finding its logs.
func LogFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	newPath := filepath.Join(dataDir(home), "modeltap.log")
	if _, err := os.Stat(newPath); err == nil {
		return newPath, nil
	}
	legacy := filepath.Join(home, ".config", "modeltap", "modeltap.log")
	if _, err := os.Stat(legacy); err == nil {
		return legacy, nil
	}
	return newPath, nil
}

// dataDir returns the directory that holds modeltap's runtime data
// for log resolution. Kept local to this package to avoid an import
// cycle with internal/config.
func dataDir(home string) string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "modeltap")
	}
	return filepath.Join(home, ".modeltap")
}

// Logs retrieves the last N lines of service log output for the given platform.
func Logs(platform Platform, lines int) (string, error) {
	switch platform {
	case PlatformMacOS:
		return logsMacOS(lines)
	case PlatformLinux:
		return logsLinux(lines)
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}
}

// logsMacOS reads the last N lines from the modeltap log file.
func logsMacOS(n int) (string, error) {
	logPath, err := LogFilePath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("log file not found: %s (is the service installed?)", logPath)
		}
		return "", fmt.Errorf("reading log file: %w", err)
	}

	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return "", nil
	}

	allLines := strings.Split(content, "\n")
	if n >= len(allLines) {
		return content, nil
	}

	return strings.Join(allLines[len(allLines)-n:], "\n"), nil
}

// logsLinux retrieves logs from journalctl for the modeltap user service.
func logsLinux(n int) (string, error) {
	cmd := exec.Command("journalctl", "--user", "-u", "modeltap", "--no-pager", "-n", strconv.Itoa(n))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("journalctl: %s: %w", stderr.String(), err)
	}
	return stdout.String(), nil
}
