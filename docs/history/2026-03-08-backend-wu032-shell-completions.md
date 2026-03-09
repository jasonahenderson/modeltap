# WU-032: Shell Completion Generation

**Date:** 2026-03-08
**Role:** Backend Implementer, Documentation Specialist
**Status:** Complete

## Summary

Refactored the `modeltap completion` command from a single command with
argument-based shell selection to proper subcommands (`bash`, `zsh`, `fish`,
`powershell`). Each subcommand generates a shell-specific completion script
using Cobra's built-in completion generation and writes it to stdout.

## Changes

### `internal/cli/completion.go`
- Replaced single-command switch-case approach with four subcommands
- Each subcommand (`bash`, `zsh`, `fish`, `powershell`) has its own `cobra.Command`
- Uses `cmd.OutOrStdout()` instead of `os.Stdout` for testability
- Added detailed long descriptions with installation instructions for each shell:
  - Bash: source directly or install to `/etc/bash_completion.d/` or Homebrew prefix
  - Zsh: source directly or install to `fpath`
  - Fish: pipe to `source` or install to `~/.config/fish/completions/`
  - PowerShell: pipe to `Invoke-Expression` or add to profile

### `internal/cli/completion_test.go` (new)
- 5 tests covering:
  - `TestCompletionBash` - verifies bash completion output contains expected markers
  - `TestCompletionZsh` - verifies zsh completion output contains expected markers
  - `TestCompletionFish` - verifies fish completion output contains expected markers
  - `TestCompletionPowershell` - verifies powershell completion output contains expected markers
  - `TestCompletionHelpText` - verifies help text lists all four shells

### `internal/cli/root_test.go`
- Added `--help` test cases for the four new completion subcommands

## Verification

- `go build ./...` succeeds
- `go test ./...` all packages pass
