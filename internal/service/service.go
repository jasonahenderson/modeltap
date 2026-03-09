// Package service generates platform-native service definitions for running
// the modeltap proxy as a background service. It supports macOS (launchd)
// and Linux (systemd) user-level services.
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Platform represents a supported operating system for service management.
type Platform string

const (
	// PlatformMacOS represents macOS with launchd.
	PlatformMacOS Platform = "darwin"
	// PlatformLinux represents Linux with systemd.
	PlatformLinux Platform = "linux"
	// PlatformUnsupported represents an unsupported platform.
	PlatformUnsupported Platform = "unsupported"
)

// Config holds the parameters needed to generate a service definition.
type Config struct {
	// BinaryPath is the absolute path to the modeltap binary.
	BinaryPath string
	// ConfigPath is the absolute path to the modeltap config file.
	ConfigPath string
	// ServiceName is the service identifier. Defaults to "com.modeltap.proxy"
	// for macOS and "modeltap" for Linux.
	ServiceName string
}

// DetectPlatform returns the current platform based on runtime.GOOS.
func DetectPlatform() Platform {
	switch runtime.GOOS {
	case "darwin":
		return PlatformMacOS
	case "linux":
		return PlatformLinux
	default:
		return PlatformUnsupported
	}
}

// defaultServiceName returns the platform-appropriate default service name.
func defaultServiceName(platform Platform) string {
	switch platform {
	case PlatformMacOS:
		return "com.modeltap.proxy"
	case PlatformLinux:
		return "modeltap"
	default:
		return ""
	}
}

// GenerateServiceFile returns the service definition content as a string for
// the given platform and configuration.
func GenerateServiceFile(platform Platform, cfg Config) (string, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = defaultServiceName(platform)
	}

	switch platform {
	case PlatformMacOS:
		return generateLaunchdPlist(cfg)
	case PlatformLinux:
		return generateSystemdUnit(cfg)
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}
}

// ServiceFilePath returns the filesystem path where the service file should
// be written for the given platform.
func ServiceFilePath(platform Platform) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}

	switch platform {
	case PlatformMacOS:
		return filepath.Join(home, "Library", "LaunchAgents", "com.modeltap.proxy.plist"), nil
	case PlatformLinux:
		return filepath.Join(home, ".config", "systemd", "user", "modeltap.service"), nil
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}
}
