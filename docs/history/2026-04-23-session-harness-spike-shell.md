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
- Structured transcript items after submit
- Transcript interaction model for submitted tokens
  - select interactive transcript items
  - toggle inline expansion for submitted paste
  - open file preview from transcript
- Queued follow-up messages during active streaming
  - submit while busy
  - auto-release after current run completes

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

Not checked off yet:

- Inline tool / permission events in transcript
- Split-view inspector
- Stream interruption / stop / retry / branch
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

1. Tool / permission event rendering in transcript
2. Stop / retry / branch controls for streaming
3. Split-view inspector
4. Slash command UX
5. History recall semantics for tokenized input

## Checkpoint

Current checkpoint state before the next test pass:

- `/demo` is now a deliberate slow-stream mode for queue/interruption testing.
- Slash commands no longer misclassify as dropped file paths.
- Submitted paste tokens expand inline in the transcript by default.
- Queued follow-up messages remain visible in the transcript while waiting.
- A queue-drain change is in progress so queued items clear from the visible
  queue surface before the next run starts.
- `Esc` stop handling for active streaming is in progress and needs manual UX
  verification in the terminal.

Immediate test targets after this checkpoint:

- Confirm `Esc` stops an active `/demo` stream in a way that feels correct.
- Confirm queued follow-ups visibly clear from the queue surface before the
  next run resumes.
