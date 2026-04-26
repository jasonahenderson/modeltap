# Session Log — Harness Focus, Submit Key, and /model Session Fixes

Date: 2026-04-24
Branch: exploration/integrated-harness

## What Changed

### 1. Default focus on transcript
- `internal/harness/model.go:134` — `NewAppState()` now starts with `Focus: ViewportFocus` instead of `InputFocus`.
- Typing any printable rune still auto-switches focus to input (existing behavior in `maybeSwitchFocus`).
- Arrow keys immediately scroll conversation history on launch.

### 2. Default submit key changed to Ctrl+Enter
- `internal/cli/harness.go:256` — Default `effectiveSubmitKey` is now `SubmitKeyCtrlEnter` instead of `SubmitKeyEnter`.
- This removes the ambiguity where Enter was both submit and newline.
- `internal/harness/input.go:49` — Placeholder updated to `"Type a message... (Ctrl+Enter to send, /help for commands, @file to attach)"`.

### 3. `/model` command works before first turn
- `internal/harness/model.go:134` — `NewAppState()` generates a default `SessionID` via `uuid.NewString()` so the harness always has a session to send.
- `internal/bff/routing.go:169` — `handleModelSwitch()` auto-creates a session if the requested one doesn't exist, matching `handleTurnSubmit` behavior.

### 4. Test updates
- `internal/harness/app_test.go:20` — Expects `ViewportFocus` instead of `InputFocus`.
- `internal/harness/sessions_test.go`, `compact_test.go`, `context_test.go` — Explicitly clear `SessionID` to simulate "no active session" state.
- `internal/bff/routing_test.go:213` — Updated `TestHandleModelSwitch_SessionNotFound` to expect auto-created session behavior.

### 5. Viewport disappearance investigation (inconclusive)
- Reviewed `viewport.go`, `app.go`, `connection.go`, `streaming.go` — the message accumulation and rendering logic appears correct.
- Could not reproduce a code path that drops assistant responses from `Messages`.
- Possible environmental causes: terminal alt-screen rendering, `userScrolled` flag stuck, or empty provider responses.

## Commands Verified

```bash
go build ./...
go test ./internal/harness/... ./internal/bff/...
```

Both pass (pre-existing CLI and integration test failures unrelated).
