---
patch: "PATCH-0042"
title: "Render tool activity as durable transcript rows"
status: "done"
date: "2026-05-22"
related:
  - "PATCH-0041 (session details command)"
  - "FEAT-0024 (shell UX chrome)"
branch: "patch/0042-tool-call-transcript-rows"
---

# PATCH-0042: Render tool activity as durable transcript rows

## Problem

Runtime tool activity currently projects into transient shell status text. That
makes adjacent calls visually blur together and disappear from the transcript,
especially during sessions with repeated reads, greps, edits, shell commands,
or agent/tool loops.

OpenCode avoids this by treating tool calls as ordered message parts and
rendering each call as an inline or block row. Modeltap does not need the full
part model in this patch, but it should persist a compact row per observed tool
activity so users can distinguish calls in the output.

## Scope

1. Add a shell host event for tool activity.
2. Project `harness.ToolActivityMsg` to that event instead of status-only text.
3. Append or update transcript event rows keyed by tool call identity.
4. Render tool rows with clear outcome glyphs and stable one-line summaries:
   - running: `⚙ <tool> — <summary>`
   - success: `✓ <tool> — <summary>`
   - error: `✗ <tool> — <summary>`
   - rejected: `⊘ <tool> — <summary>`
5. Keep the footer/status signal for active tool activity so existing chrome
   remains informative.
6. Add focused tests for projection, state updates, and rendering.

## Out of Scope

- Full OpenCode-style typed message parts.
- Per-tool rich block renderers for shell output, diffs, write snapshots, or
  subagent cards.
- Collapsible long tool output.
- Run transcript/event replay changes.
- FEAT-0024 command palette, sidebar, or agents overlay work.

## Checklist

- [x] Patch index updated
- [x] Tool activity host event added
- [x] Projection emits durable tool activity event
- [x] Transcript rows append/update by tool call id
- [x] Tool rows render with outcome glyphs and summary text
- [x] Existing status footer remains useful during active tool activity
- [x] Tests added or updated
- [x] `go test ./internal/harnessshell ./internal/harnesshost` passes
- [x] `go test ./...`, `go build ./...`, and `go vet ./...` pass

## Fix Detail

The narrow implementation mirrors OpenCode's presentation model without
adopting its full message-part architecture. The runtime already emits
`ToolActivityMsg`; the host adapter should preserve that event as a shell
`ToolActivityEvent`. The shell then owns a durable transcript event row keyed by
tool call id where available, falling back to a stable key from tool name and
summary.

This makes every call visible in transcript order now, while leaving richer
inline/block per-tool rendering for FEAT-0024.
