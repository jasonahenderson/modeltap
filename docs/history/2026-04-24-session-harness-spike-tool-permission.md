# 2026-04-24 — Harness spike: tool / permission inline approval

Branch: `spike/scrolling-surface-eval`
Base commit: `eb55263` (`SPIKE: scrolling-surface checkpoint — ▎ marker, flat surface, arrow history`)
Continues from:
- `docs/history/2026-04-24-session-harness-spike-scrolling-surface-handoff.md`
- `docs/history/2026-04-24-session-harness-spike-scrolling-surface-checkpoint.md`

## Goal this session

Move priority #1 on the spike — **tool / permission event rendering in
transcript** — to a usable state without blocking on polish items.

## Decisions reached

- **Inline placement is locked in.** Tool / permission events render in the
  transcript, not in a docked side panel. The Tool Demo session preset and
  the new `/perm` flow both validate the shape.
- **Inline `y` / `n` keybindings** on the active permission event, gated
  on an empty input so normal typing is not hijacked. Modal overlays were
  considered and rejected as heavier than needed.
- **Packaging / extraction review is a merge gate, not a normal priority.**
  Moved out of the numbered list into a dedicated "Required before
  merging back to `main`" section.
- **Default session starts empty.** The seeded system + assistant messages
  were pure spike noise; removing them gives a clean surface that matches
  what a real first-run will look like.

## Changes

### Tool / permission flow (`internal/harnessspike/app.go`)

- New `pendingPermission` struct capturing `eventIndex`, `toolLabel`,
  `grantText`, `denyText`.
- New `App.pendingPermission *pendingPermission` field.
- `/perm` slash command intercepted in `submit()` (after the queue /
  streaming check, mirroring the `/clear` pattern). Triggers
  `beginPermissionDemo` which appends the user message, a `requested`
  event, a `permission` event, and sets `pendingPermission`. It does not
  start a stream.
- `grantPermission()` flips the event to `granted`, appends
  `running` / `done` events and a streaming assistant message, returns
  the usual stream tick + pulse batch.
- `denyPermission()` flips the event to `denied` and appends a short
  non-streaming assistant message.
- `Update()` intercepts `y` / `n` (and capitalized variants) when
  `pendingPermission != nil`, `focus == focusInput`, and
  `input.Value() == ""`.
- `refreshTranscript()` appends a yellow bold hint
  (`permissionHintStyle`) after the active permission event:
  `press y to grant · n to deny`.
- `renderEventMessage()` handles the new `granted` / `denied` states.

### Styles (`internal/harnessspike/styles.go`)

- Added `eventGrantedStyle` (bold, green `#7EE787`).
- Added `eventDeniedStyle` (bold, red `#F85149`).
- Replaced the temporary inline hint styling with action-list styling for
  inline permission controls.

### Default seed removal

- `sessionPreset("Spike Session" / default)` now returns `nil`. Fresh
  app and `/clear` on the default session both yield an empty transcript.
- Named presets (`Tool Demo`, `Reference Layout`, `Dummy Stream`) are
  unchanged.

### Tests (`internal/harnessspike/app_test.go`)

- Added `TestPermCommandTriggersPendingPermission` — `/perm` appends the
  expected events, sets `pendingPermission`, and renders the inline action
  list.
- Added `TestPermGrantContinuesWithToolAndStream` — `y` clears pending,
  emits granted + running + done, starts streaming assistant.
- Added `TestPermDenyShortCircuitsWithoutStream` — `n` clears pending,
  emits denied, no running/done, trailing assistant message, no stream.
- Added `TestYKeyDoesNotTriggerGrantWhenInputNonEmpty` — typing does not
  grant.
- Added transcript-cursor coverage for permission actions:
  - activating the selected action from the composer with `Enter`
  - moving across action chips with `Left` / `Right`
  - deny-with-reason using the composer action list
- Renamed `TestNewSeedsMessages` → `TestNewStartsWithEmptyTranscript`
  (asserts zero messages at startup).
- Removed `TestDefaultSessionSeedsUsefulStartupMessage`.
- Updated `TestClearCommandResetsTranscript` and
  `TestSidebarActionClearResetsTranscript` to expect an empty transcript
  after `/clear` (after seeding a stub row manually).

### Doc updates (`docs/history/2026-04-23-session-harness-spike-shell.md`)

- "Partially checked off → Inline tool / permission event rendering":
  replaced the old stub with the actual scope landed this session plus
  the enumerated open items (scope option, multi-permission, parameter
  display, mid-stream origination, deny-with-reason).
- Split the inline permission track into:
  - validated UI shape
  - remaining UI-side interaction work
  - packaging / productionization work deferred to the merge gate
- Recorded the next UI direction:
  - replace instructional hint text with a composer-mounted action list
  - keep the permission request in the transcript as durable history
  - use the input/composer area as the primary interaction path
  - keep raw `y` / `n` only as optional shortcuts
- "Current priority order" #1 carries a `(partially finished)` marker so
  the numbering is preserved.
- "Required before merging back to `main`" section introduced earlier
  stays. Packaging / extraction review is now a merge gate.
- Checkpoint bullets extended with: arrow command history, inline
  `/perm` permission flow, empty default transcript.
- "Immediate test targets" rewritten to point at interactive validation
  of the `/perm` flow and the start of priority #2.

## Verified locally

- `go build ./...`
- `go test ./internal/harnessspike`

## Follow-up UI adjustment

After the initial `y` / `n` implementation, the spike was tightened to a
more production-credible hybrid interaction shape while still staying
UI-first:

- The permission request remains in the transcript.
- The available actions now render in the composer area instead of on the
  transcript row.
- `Left` / `Right` move across the available actions while the composer is
  empty.
- `Enter` activates the selected action from input focus.
- The current action set is:
  - `Approve once`
  - `Allow for session`
  - `Deny`
  - `Deny with reason`
- `y` / `n` still work from the empty input as fallback shortcuts, but they
  are no longer the primary UI model.

This keeps the inline placement decision intact while making the spike
worth testing as a real interaction surface instead of as shortcut text.

## Files touched this session

- `internal/harnessspike/app.go`
- `internal/harnessspike/app_test.go`
- `internal/harnessspike/styles.go`
- `docs/history/2026-04-23-session-harness-spike-shell.md`
- `docs/history/2026-04-24-session-harness-spike-tool-permission.md` (new)
- `docs/history/2026-04-24-session-harness-spike-tool-permission-handoff.md` (new)

Not touched (intentionally left dirty from earlier work per previous
handoffs):

- `internal/cli/harness.go`
- `internal/harness/{app_test,compact_test,context_test,input,model,sessions_test}.go`

## What's partial / what's next

Priority #1 is **partially finished**. The flow is usable and tested, but
the following live items remain on the "Partially checked off → Inline
tool / permission event rendering" entry:

- "Always allow for this session" scope option.
- Multiple simultaneous / queued permissions.
- Tool parameter / target display on permission events (what is being
  read, written, executed).
- Permissions that originate mid-stream instead of from the `/perm`
  slash command.
- Deny-with-reason input.

Next session should start on **priority #2** (stop / retry / branch
controls for streaming) and return to the partial items opportunistically.
