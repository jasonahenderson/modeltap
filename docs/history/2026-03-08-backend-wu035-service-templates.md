# WU-035: Service Template Generator

**Date:** 2026-03-08
**Agent:** Backend
**Status:** Complete

## Summary

Created the `internal/service/` package that generates platform-native service definitions for running the modeltap proxy as a background service. This implements the template generation layer described in ADR-0012 (Background Execution Strategy) and the service management feature doc.

## Files Created

- `internal/service/service.go` — Core package with `Platform` type, `DetectPlatform()`, `Config` struct, `GenerateServiceFile()`, and `ServiceFilePath()` functions
- `internal/service/launchd.go` — macOS launchd plist template generation using `text/template`
- `internal/service/systemd.go` — Linux systemd unit file template generation using `text/template`
- `internal/service/service_test.go` — Test suite with 6 tests covering platform detection, template generation, file paths, unsupported platform errors, and default service names

## Design Decisions

- Used `text/template` for template generation as specified
- Used `os.UserHomeDir()` for home directory resolution, consistent with `internal/config/config.go`
- Service definitions are user-level (no sudo required): `~/Library/LaunchAgents/` for macOS, `~/.config/systemd/user/` for Linux
- Default service names: `com.modeltap.proxy` (macOS), `modeltap` (Linux)
- Log paths: `~/.config/modeltap/modeltap.log` and `modeltap.err.log`, consistent with the config directory convention
- Systemd unit passes config path via `MODELTAP_CONFIG` environment variable

## Test Results

All 6 tests pass: `TestDetectPlatform`, `TestGenerateLaunchdPlist`, `TestGenerateSystemdUnit`, `TestServiceFilePath` (3 sub-tests), `TestGenerateServiceFile_UnsupportedPlatform`, `TestGenerateServiceFile_DefaultServiceName`.
