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
- User message restyle to caret + gray block
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
  - first fake transcript event shapes are present
  - approval / denial flow is still missing

Not checked off yet:

- Composer anchoring review
  - fixed bottom composer vs single scrolling surface
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

1. Composer anchoring review
2. Tool / permission event rendering in transcript
3. Stop / retry / branch controls for streaming
4. Split-view inspector
5. Packaging / extraction review

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
- The current layout still uses a fixed bottom composer, and the next design
  review should test a single scrolling surface with the composer at the end.

Immediate test targets after this checkpoint:

- Compare the current anchored-composer layout against a single scrolling
  surface with the composer at the bottom of the conversation.
- Decide whether transcript tool / permission events belong inline before
  adding approval and denial interactions.
