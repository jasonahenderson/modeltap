# Session Log: Terminal Response Leak — Iteration

## Date
2026-04-23

## What Was Discussed

Continued debugging the terminal response leak in the harness TUI. Despite initial fixes, terminal garbage (single `]` characters, frozen screen) kept appearing on startup or after multiple open/close cycles.

## Root Cause Analysis

The leak is caused by bubbletea's `init()` calling `lipgloss.HasDarkBackground()`, which queries the terminal via `termenv`. `termenv` sends an **OSC 11 query** and a **CPR query** simultaneously but only reads the first response. The leftover response sits in the TTY buffer and leaks into the input stream as keystrokes.

### Specific leaks identified:
1. **OSC response body** — when CPR is consumed first, OSC response `\x1b]11;rgb:1919/1a1a/1b1b\x1b\` is left behind. Bubbletea parses `\x1b]` as `Alt+]`, `\x1b\` as `Alt+\`, but the body `11;rgb:...` slips through because it starts with a digit, not a bracket.
2. **CSI parameter fragments** — when the ESC prefix is filtered, remaining fragments like `1;1R` or `0;95;0c` can appear as short `KeyRunes`.
3. **Unexported byte-slice messages** — bubbletea emits `unknownCSISequenceMsg` (unexported `[]byte` type) for unrecognized CSI sequences. These bypassed the original `tea.KeyMsg` filter.

## Decisions Made

1. **Filtering must happen in `tea.WithFilter`** — confirmed via bubbletea source (`tea.go:390`) that this runs before `Update` and catches messages that timing issues would otherwise miss.
2. **Must catch both `tea.KeyMsg` and unexported byte-slice messages** — reflect-based check on any `[]uint8` starting with `0x1b`.
3. **Empty `Runes` must not be filtered** — Enter, Backspace, Tab have empty `Runes`. A previous `onlyCSIParams` loop over empty strings returned `true` (no-op iteration), which broke all special keys.
4. **ESC-prefixed fragments as `Alt+[`, `Alt+]`, `Alt+\`** — all three must be dropped, not just `Alt+\`.

## Actions Taken

- **Replaced inline filter** in `App.Update` with `tea.WithFilter(TerminalResponseFilter)` wired in `internal/cli/harness.go`.
- **Added `terminalResponseFilter`** function in `internal/harness/app.go`:
  - Handles `tea.KeyMsg` via `isTerminalGarbage`
  - Handles unexported `[]byte` messages via reflection (check first byte == `0x1b`)
- **Added `isTerminalGarbage`** function covering:
  - `Alt+[`, `Alt+]`, `Alt+\` (ESC-prefixed fragments)
  - `\x1b`-prefixed strings
  - `]N;...` OSC responses
  - `[N;...` CSI responses (CPR, focus events, DA1/DA2, kitty keyboard)
  - Short numeric parameter fragments (`11;rgb:...`, `1;1R`, etc.) via `digits;` prefix check
- **Added unit tests** in `internal/harness/app_test.go`:
  - 21 test cases covering drop and passthrough behavior
  - Regression tests for Enter, Backspace, Tab (empty runes)
  - New cases for `Alt+[`, `Alt+]`, OSC body, unknown CSI byte slice

## Files Modified

- `internal/harness/app.go` — `TerminalResponseFilter`, `isTerminalGarbage`
- `internal/cli/harness.go` — `tea.WithFilter(harness.TerminalResponseFilter)`
- `internal/harness/app_test.go` — `TestTerminalResponseFilter` (21 cases)

## Open Items

- **Frozen terminal** — reported after multiple open/close cycles with `ctrl+c`. Needs investigation into whether bubbletea is leaving the terminal in raw mode on abrupt exit. Possible causes:
  - Double `ctrl+c` (signal handler vs. Quit key both fire)
  - The `tea.WithAltScreen()` exitAltScreen sequence not completing
  - Terminal emulator state desync
- **Terminal freeze investigation deferred** — not part of PATCH-0011 scope.
