# 2026-04-23 — Harness spike shell

Started a replacement-harness spike on `spike/crush-shell-eval`.

What landed:

- Added a new `modeltap harness-spike` command.
- Created an isolated `internal/harnessspike/` package instead of editing the
  existing `internal/harness/` implementation.
- Built a minimal Bubble Tea shell with:
  - sidebar
  - transcript viewport
  - multiline input
  - fake streaming assistant replies
  - `/clear` reset command

Purpose:

- Evaluate shell layout, focus behavior, transcript rendering, and streaming
  feel without involving the current BFF or harness codepath.
- Keep the existing harness implementation available in parallel for reference.

Deliberately out of scope for this spike slice:

- real provider calls
- BFF integration
- tool execution
- MCP
- persistent sessions

## Master checklist

Checked off enough to evaluate:

- Isolated replacement shell under `internal/harnessspike/`
- Right-side sidebar
- Sidebar open/close with main-area expansion
- Navigable multi-section sidebar
- Action confirmation flow
- Real overlay rendering
- Multi-option modal instead of hardcoded yes/no
- Polished modal styling
- User message restyle to `▎` marker on flat background
- Large paste compaction into tokens
- Terminal-style file-drop/path tokenization
- Token preview before submit
- Background-agent status surface
- Background-agent list/detail overlays
- Direct-open behavior when only one agent exists
- Command palette
- Session switching through palette
- Sidebar toggle through palette
- Sidebar closed by default
- Transcript mouse/touchpad scrolling without explicit focus change
- Structured transcript items after submit
- Transcript interaction model for submitted tokens
  - select interactive transcript items
  - toggle inline expansion for submitted paste
  - open file preview from transcript
- Queued follow-up messages during active streaming
  - submit while busy
  - auto-release after current run completes
  - preserve FIFO order after interrupt and resumed processing
  - drain backlog into one merged resumed turn
- Single scrolling surface with composer at transcript tail
- Composer as top/bottom ruled section (no tinted slab)
- Single-line default input height
- `▎` blue prompt marker on input line
- `▎` marker on user messages and queued items in transcript
- Transcript rows on flat background (no per-row tints)
- Main surface uses terminal-default background
- `alt+enter` inserts newline in input
- Input focus preserved after submit
- Mouse/touchpad scroll preserves input focus
- Startup seeded assistant message replaced with actionable test guidance

Partially checked off:

- Drag/drop of files
  - tokenization works
  - attachment pipeline does not exist yet
- Large paste
  - compaction works
  - transcript/history representation is improved with inline expansion
  - still no richer inspector or recall flow
- Background agents
  - navigation pattern works
  - no real task model yet
- Queued follow-up messages
  - no edit/cancel/reorder controls yet
  - no explicit interrupt-vs-queue choice yet
- Stream interruption / stop / retry / branch
  - stop is implemented with a two-step Esc interrupt
  - retry / branch are still missing
- Inline tool / permission event rendering
  - UI shape validated:
    - inline placement decision locked in (events render in the transcript)
    - `/perm` demo command drives a live request → permission → grant/deny
      flow for evaluation
    - inline `y` / `n` keybindings grant or deny the active permission
    - `granted` and `denied` event states have distinct styling
    - grant extends the trace with `running` / `done` events and streams the
      assistant reply; deny short-circuits with a trailing assistant note
  - UI work still open in the spike:
    - replace instructional inline hint text with a composer-mounted action
      list while keeping the permission request itself in the transcript
    - define the initial action set and ordering (`approve once`, `always
      allow for session`, `deny` unless testing suggests a different shape)
    - make permission actions keyboard-selectable from the input/composer area
      with `Left` / `Right` and `Enter`; treat raw `y` / `n` as optional
      shortcuts, not the primary UI
    - evaluate whether mouse cursor selection/click is worth supporting in the
      spike surface after keyboard selection feels right
    - "always allow" affordance and placement in the inline UI
    - tool parameter / target display clarity
    - multiple pending permissions as a UI/interaction problem
    - permissions that originate mid-stream as a UI interruption problem
  - Packaging / productionization refactor is tracked separately under
    "Required before merging back to `main`"; do not block UI iteration on
    replacing the current demo/scaffolding path yet

Not checked off yet:

- Composer anchoring review
  - Claude-style composer scroll-away behavior in tight vertical layouts
- Packaging / extraction review
  - can the spike shell stand alone as a releasable component
  - can the harness consume it as an embedded package without tight coupling
  - what seams are required if it remains in-repo
- Split-view inspector
- Slash command UX with suggestions
- History recall semantics for tokenized input
- Empty/loading/error states
- Session list depth
  - recent/running/pinned/unread patterns
- Transient vs durable side surfaces
  - what should be overlay vs docked panel
- Selection model beyond input/sidebar
  - transcript artifacts, message-local actions, etc.

## Current priority order

1. Tool / permission event rendering in transcript *(partially finished — see "Partially checked off" for the open items)*
2. Stop / retry / branch controls for streaming
3. Split-view inspector
4. Composer scroll-away behavior in tight vertical layouts

## Required before merging back to `main`

- **Packaging / extraction review.** Blocks the merge back. Must confirm:
  - the spike shell can stand alone as a releasable component;
  - the harness can consume it as an embedded package without tight
    coupling;
  - what seams are required if it remains in-repo.
- **Packaging plan for inline permissions.** Keep iterating on the current
  UI-first inline permission flow during the spike, but treat the present
  `/perm` + singleton pending-permission path as temporary scaffolding.
  Packaging must convert it into a production permission model without
  changing the inline transcript placement that the spike is validating.
  Required packaging-phase refactor targets:
  - replace the single pending-permission pointer with a real collection
    keyed by stable request ID
  - move permission origination to the actual tool/runtime boundary instead
    of the `/perm` demo path
  - separate transcript display events from permission-control state
  - define a structured permission contract (`request_id`, tool/action/target,
    parameters, scope, status, created/run origin metadata)
  - replace implicit `y` / `n` against "the current permission" with explicit
    approve/deny actions against a selected request
  - support multiple pending permissions and permissions that originate
    mid-stream
  - add scoped approval (`once`, `session`, and any later project/workspace
    policy if needed)
  - persist enough permission state that redraw/reflow does not lose context

## Checkpoint

Current checkpoint state before the next test pass:

- `/demo` is now a deliberate slow-stream mode for queue/interruption testing.
- Slash commands no longer misclassify as dropped file paths.
- The sidebar now starts closed and footer hints carry the open/toggle affordance.
- Submitted paste tokens expand inline in the transcript by default.
- Queued follow-up messages remain visible in the transcript while waiting.
- Queue draining now merges waiting follow-ups into one resumed turn.
- `Esc` stop handling is implemented with a two-step interrupt flow.
- Mouse/touchpad transcript scrolling is enabled and refreshes preserve scroll
  position instead of forcing the view back to the bottom.
- The layout now uses a single scrolling surface with the composer rendered at
  the end of the transcript content. Fixed-bottom anchoring has been set aside
  in favor of the scrolling-surface model.
- Composer reads as a top/bottom ruled section; per-row transcript tints are
  removed and the main surface uses the terminal-default background.
- `▎` is the shared visual marker for the input prompt and for user / queued
  entries in the transcript.
- `alt+enter` inserts a newline in the input; input focus is preserved after
  submit; mouse scroll no longer steals focus.
- `ctrl+k` / `ctrl+t` bindings are handled via explicit Bubble Tea key types
  instead of stringly-typed matching.
- Composer input now has shell-style `Up` / `Down` command history on a
  single-line buffer; multi-line editing still uses arrows for cursor motion.
- Inline tool / permission events are live: `/perm` triggers a request →
  permission event, and `y` / `n` grant or deny while the input is empty.
  Granted runs append `running` / `done` and stream a reply; denied runs
  short-circuit with a trailing assistant note.
- Default session starts with an empty transcript. `/clear` on the default
  session also leaves the transcript empty.

Immediate test targets after this checkpoint:

- Interactive validation of the `/perm` flow (trigger, grant, deny, typing
  does not grant, empty-transcript startup reads as expected).
- Begin priority #2: stop / retry / branch controls for streaming.
- Return to the remaining UI-side open items on the partial tool / permission
  track (composer action-list design, keyboard cursor interaction, scope
  affordance, parameter display, multi-pending interaction, mid-stream
  interruption handling) as they come up, not as a blocking set. Leave the
  production model refactor to the packaging phase.
