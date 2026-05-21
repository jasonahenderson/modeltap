# 2026-04-28 — Session: shell quit command

## Scope

Added shell-native `/quit` and `/exit` commands as aliases for leaving
the Bubble Tea shell.

## Changes

- Added `/quit` and `/exit` handling in the shell key path so Enter
  returns `tea.Quit`.
- Documented the commands in `modeltap shell` and `shell-demo` help.
- Added focused tests for both aliases.

## Verification

- `make fmt-check`
- `make lint`
- `make all`
