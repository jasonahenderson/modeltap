# 2026-04-24 — Harness spike (scrolling surface) checkpoint

Branch: `spike/scrolling-surface-eval`
Base: `73236c4` (`SPIKE: iterate harness shell interactions`)
Continues from: `.sdlc/history/2026-04-24-session-harness-spike-scrolling-surface-handoff.md`

## Goal this session

Close out the scrolling-surface evaluation so the spike can move to the
packaging / extraction review phase. Specifically:

- Make the input caret recognizable inside the viewport-embedded composer.
- Consolidate the visual language between the input prompt and the transcript.
- Flatten the main surface so it sits on the terminal's own background.
- Add basic shell-style arrow history to the input.

## Changes

### Visual / interaction

- Input prompt changed to `▎ ` (blue, bold) as a stable visible marker for
  the composer. Reverse-video cursor rendering was unreliable when the
  textarea is captured into the viewport via `SetContent`, so the prompt
  marker is the primary cue; the cursor block is secondary.
- Transcript user messages and queued-submission rows now use the same
  `▎ ` marker (previously `> `), so "where I typed" and "where I'm typing"
  share a single visual.
- Removed per-row tinted backgrounds from `systemStyle`, `userBodyStyle`,
  `queuedBodyStyle`, and all five `event*` styles. Foreground colors and
  `Padding(0, 1)` are retained.
- Removed the page-level `pageBg` (`#101318`) from `headerBoxStyle`,
  `transcriptBoxStyle`, `inputBoxStyle`, `footerBoxStyle`, and
  `composerBoxStyle`. The `pageBg` variable was deleted. Main surface now
  inherits the terminal's default background.
- Detail windows (sidebar, overlay panels, dialog, palette) kept their own
  explicit backgrounds, so modals still read as distinct surfaces.

### Input behavior

- Added command history: `commandHistory []string`, `historyIndex int`
  (init `-1` meaning "not browsing"), `historyDraft string`.
- `submit()` pushes the trimmed content via `pushHistory`, which collapses
  consecutive duplicates.
- `KeyUp` / `KeyDown` in `focusInput` recall previous / next command when
  the input has no newline. Multi-line editing still uses arrows for cursor
  movement inside the textarea.
- Pressing a non-arrow key while browsing exits history mode and preserves
  the edited text as a fresh draft.
- Past the newest history entry, `KeyDown` restores the saved draft.

### Tests

- `TestInputArrowHistoryRecallAndRestoreDraft`
- `TestInputArrowDownWithoutBrowsingDoesNothing`
- `TestConsecutiveDuplicateSubmissionsStoredOnce`

### Documentation

- Updated the running checklist in
  `.sdlc/history/2026-04-23-session-harness-spike-shell.md`:
  - Replaced stale `User message restyle to caret + gray block` entry with
    `User message restyle to ▎ marker on flat background`.
  - Added new accepted items for scrolling surface, ruled composer,
    single-line default input, `▎` markers, flat rows, terminal-default
    main background, `alt+enter` newline, input focus preserved after
    submit, mouse scroll preserves focus, and updated seed message.
  - Removed the fixed-vs-scrolling sub-item from `Not checked off yet`
    (resolved in favor of the scrolling surface).
  - Reshuffled priority order: **Packaging / extraction review** is now
    `#1`.
  - Rewrote the Checkpoint section for today's state and pointed the
    immediate test targets at packaging review.

## Verified locally

- `go build ./...`
- `go test ./internal/harnessspike`

## Files touched this session

- `internal/harnessspike/app.go`
- `internal/harnessspike/app_test.go`
- `internal/harnessspike/styles.go`
- `.sdlc/history/2026-04-23-session-harness-spike-shell.md`

Not touched (intentionally left dirty from earlier work per previous
handoff):

- `internal/cli/harness.go`
- `internal/harness/{app_test,compact_test,context_test,input,model,sessions_test}.go`

## What's next

Per the updated priority order:

1. **Packaging / extraction review.** Can the spike shell stand alone as a
   releasable component? What seams does the harness need if it embeds it?
2. Tool / permission event rendering in transcript.
3. Stop / retry / branch controls for streaming.
4. Split-view inspector.
5. Composer scroll-away behavior in tight vertical layouts.
