---
feature: FEAT-0004
title: Service Management
status: accepted
date: 2026-03-08
adr-constraints:
  - ADR-0003: Cobra CLI framework
  - ADR-0004: Viper configuration management
  - ADR-0012: Background execution strategy
---

# FEAT-0004: Service Management

## Problem

The modeltap proxy runs only in the foreground via `modeltap start`, blocking the terminal until Ctrl+C. For real-world usage — where the proxy sits in the request path for AI API calls — users need the proxy to run persistently in the background with auto-restart on crash, log management, and clean lifecycle control. Building custom daemon logic in Go is error-prone and reimplements what OS service managers already do well.

## Solution

A `modeltap service` subcommand that generates and installs platform-native service definitions — launchd plist on macOS and systemd unit on Linux. The proxy itself stays foreground; the OS service manager handles backgrounding, auto-restart, and log management.

## Key Capabilities

### Service Installation
- `modeltap service install` generates and installs a platform-native service definition
- Detects the current platform (macOS vs Linux) and generates the appropriate format
- Uses the current binary path and resolved configuration
- Starts the service immediately after installation

### Service Lifecycle
- `modeltap service uninstall` stops the service and removes the service definition
- `modeltap service status` shows whether the service is running, its PID, and uptime
- `modeltap service logs` shows recent service logs (journalctl on Linux, log show on macOS)

### Platform Support
- **macOS:** launchd plist in `~/Library/LaunchAgents/` (user-level, no sudo required)
- **Linux:** systemd user unit in `~/.config/systemd/user/` (user-level, no sudo required)
- **Windows:** Deferred to future work

### Service Configuration
- Service definition references the modeltap config file, so `modeltap config set` changes take effect on service restart
- Environment variables can be passed through the service definition
- Service restarts automatically on crash (launchd `KeepAlive`, systemd `Restart=on-failure`)

## CLI Integration

```
modeltap service install     # Install and start the service
modeltap service uninstall   # Stop and remove the service
modeltap service status      # Show service status
modeltap service logs        # Show recent service logs
```

## Configuration

No new config keys required — the service definition uses the existing modeltap configuration. The service subcommand reads the binary path and config path at install time and embeds them in the service definition.

## Relationship to ADRs

- Implements ADR-0012 (Background Execution Strategy)
- Uses ADR-0003 (Cobra) for CLI commands
- Uses ADR-0004 (Viper) for configuration path resolution
