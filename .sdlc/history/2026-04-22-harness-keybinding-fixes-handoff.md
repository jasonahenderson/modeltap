# Session Handoff: Harness Keybinding and UX Fixes

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Date
2026-04-22

## What Was Completed

### 1. Default Submit Key
**Problem:** `SubmitKeyCtrlEnter` (bound to `"ctrl+@"`) was unreachable on macOS. Pressing Enter inserted a newline, but there was no way to send a message.

**Fix:** Changed default to `SubmitKeyEnter` in `internal/harness/keys.go:31`.

### 2. Tab Toggles Mode
**Problem:** Only `Ctrl+P` toggled plan/build mode. Tab did nothing.

**Fix:** Added `"tab"` as alias in `KeyMap.ToggleMode` binding in `internal/harness/keys.go:57-58`.

### 3. Newline Shortcuts
**Problem:** With Enter as submit, there was no way to insert a newline. `Alt+Enter` (Option+Enter on Mac) wasn't recognized by bubbletea. `Ctrl+J` is the universal linefeed key.

**Fix:** Added `isNewlineShortcut(msg tea.KeyMsg)` helper in `internal/harness/app.go:851-860`. Checked **before** the submit-key match so it always inserts newline:

```go
// isNewlineShortcut reports whether a key event should insert a newline
// in the input area rather than trigger the submit key binding.
func isNewlineShortcut(msg tea.KeyMsg) bool {
    return msg.Type == tea.KeyEnter && msg.Alt || msg.Type == tea.KeyCtrlJ
}
```

Evaluated at `app.go:195`. Uses `InputArea.InsertNewline()` which calls `textarea.InsertRune('\n')`.

### 4. `/help` Command
**Problem:** `/help` fell through to "Unknown command."

**Fix:** Added `"help"` case in `handleCommand` (`internal/harness/app.go:710-731`) with full command list.

### 5. Terminal Response Filtering (Partial)
**Problem:** On startup, random ASCII characters appear at the first line of the TUI.

**Root Cause:** Terminal responses to startup queries (OSC 11 background color, cursor position reports) leak into the textarea as `tea.KeyRunes` keystrokes:
- `]11;rgb:1919/1a1a/1b1b` (background color response)
- `[1;1R` (cursor position report)
- `alt+\` (another terminal response)

**Attempted Fix:** Added filtering at `app.go:154-164` that swallows `KeyRunes` whose first rune is `[`, `]`, `\`, or `\u001b` (ESC):

```go
if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
    switch msg.Runes[0] {
    case '[', ']', '\\', '\u001b':
        return a, nil
    }
}
```

**Status:** Ctrl+J and Option+Enter verified working. `/help` prints above the command area (expected behavior — it's a BannerMsg). **Terminal responses may still leak through** if they don't start with those characters, or if the filter is in the wrong location in the message pipeline.

### 6. Config Plumbing
Added `HarnessConfig` struct to `internal/config/config.go:92-101` with `submit_key` field, and wired it through `internal/cli/harness.go:252-265`.

### 7. BFF Session Auto-Creation
**Problem:** First turn sent empty `SessionID` and BFF rejected it with "session_id is required".

**Fix:** In `internal/bff/turn.go:77-82`, auto-mint a UUID when SessionID is empty:

```go
if submit.SessionID == "" {
    submit.SessionID = uuid.NewString()
}
```

### 8. BFF Subprocess Redirection
**Problem:** Server startup logs leaked into harness TUI.

**Fix:** Set `cmd.Stdout = nil`, `cmd.Stderr = nil` in `internal/harness/connection.go:534-538` so exec uses `/dev/null` implicitly.

## Remaining Issues

### Terminal Response Characters Still Appearing
Despite the filter at `app.go:154-164`, terminal response sequences may still appear. Possible reasons:

1. **Filter location:** The filter is inside `case tea.KeyMsg` in `App.Update`, but terminal responses may arrive as a bulk paste event (a single `KeyMsg` with `Paste: true` containing multiple response sequences) rather than individual keystrokes.

2. **Response format variation:** Some terminals may send responses with different prefixes (e.g., `CSI` sequences starting with `0x9b` or `ESC[`). The filter only catches `[`, `]`, `\`, and `ESC`.

3. **Timing:** The responses may arrive before the TUI is fully initialized, bypassing the Update handler entirely.

### Help Banner Placement
The `/help` output is rendered as a `BannerMsg` which appears in the banner area (above the input, below the viewport). This is the design (FEAT-0009). If you want it in the conversation viewport instead, change `/help` to append a `DisplayMessage` with `RoleSystem` to `AppState.Messages`.

## Files Changed
- `internal/harness/keys.go` — default submit key, Tab alias
- `internal/harness/app.go` — newline shortcuts, terminal response filter, `/help` command
- `internal/harness/input.go` — `InsertNewline()` helper
- `internal/harness/connection.go` — subprocess stdout/stderr redirection
- `internal/harness/model.go` — removed uuid import (reverted)
- `internal/harness/debug.go` — new (remove before commit)
- `internal/cli/harness.go` — config plumbing for submit_key
- `internal/config/config.go` — HarnessConfig struct
- `internal/bff/turn.go` — auto-create session on empty SessionID
- `internal/bff/turn_test.go` — updated test for auto-session

## References

### Crush Design Reference
From studying `github.com/charmbracelet/crush`:

- **Default submit:** `Enter`
- **Newline:** `Shift+Enter` (on terminals with keyboard disambiguation) or `Ctrl+J` (universal)
- **Terminal disambiguation:** Crush uses `tea.KeyboardEnhancementsMsg` to detect terminals that support `Shift+Enter`. They dynamically update help text. We don't implement this yet.
- **No configurable submit_key:** Crush hardcodes these bindings.
- **Backslash escape:** If last char is `\`, Enter removes it and inserts newline instead of sending. We didn't implement this.

### Bubbletea Key Types
- `tea.KeyEnter` = CR (carriage return, `\r`)
- `tea.KeyCtrlJ` = LF (line feed, `\n`)
- `tea.KeyAlt` is a boolean on `tea.Key` struct, NOT a separate key type
- `key.Matches()` compares `msg.String()` against binding keys. `ctrl+j` does NOT match the Submit binding which is `"enter"`.

## Recommended Next Steps

1. **Test terminal response filtering:** Run the debug build, capture `/tmp/modeltap_debug.log`, verify the filter actually catches the sequences. If not, the fix may need to be in the Bubbletea initialization (disable terminal queries) or in the textarea component itself.

2. **Add `Shift+Enter` support:** On terminals with `kitty keyboard protocol` or `iTerm2`, Shift+Enter comes through as a distinct key type. Add it alongside `Ctrl+J` in `isNewlineShortcut`.

3. **Implement backslash escape:** Add the Crush-style escape where trailing `\` + Enter removes the backslash and inserts newline.

4. **Consider BFF Session Resume on Connect:** Instead of auto-creating a session on the first turn, the harness could call `SessionResume` with the generated UUID immediately after capabilities registration. This would be cleaner architecturally.

5. **Remove debug.go:** The `internal/harness/debug.go` file and `logDebug` calls should be removed before commit. They're instrumentation only.

## Commit Commands (when ready)

```bash
git add internal/harness/keys.go internal/harness/app.go internal/harness/input.go \
    internal/harness/connection.go internal/cli/harness.go \
    internal/config/config.go internal/bff/turn.go internal/bff/turn_test.go
git reset internal/harness/debug.go  # remove debug file
git restore internal/harness/compact_test.go internal/harness/context_test.go \
    internal/harness/sessions_test.go  # revert test changes if not needed
git commit -s -m "PATCH-0011: fix harness submit key, newline shortcuts, /help, terminal responses"
```

(End of handoff)
