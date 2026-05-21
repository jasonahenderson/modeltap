---
patch: "PATCH-0041"
title: "Wire /sessions show to session.details and recent session runs"
status: "done"
date: "2026-05-20"
related:
  - "PATCH-0039 (session semantics redefine)"
  - "FEAT-0017 (durable runs and background agents)"
  - "FEAT-0024 (shell UX chrome)"
branch: "patch/0041-session-details-command"
---

# PATCH-0041: Wire /sessions show to session.details and recent session runs

## Problem

`/sessions` now gives a useful list of session IDs, but the production shell has
no way to inspect one of those sessions before resuming it. The runtime already
implements `session.details`, and the protocol client already exposes
`SessionDetails`, but `internal/harnesshost/production_runtime.go` only routes
`/sessions` to list/resume/clear/fork/current.

This leaves users with an awkward gap: they can see that a prior session exists,
but cannot answer "what happened in it?" or "which runs/agents belong to it?"
from inside the TUI.

## Scope

1. **Add `/sessions show [session-id]` and `/sessions details [session-id]`.**
   Both commands call `session.details`. `/session show` and
   `/session details` work through the existing alias path.

2. **Default to the active session when no ID is supplied.** If no active
   session exists, return a clear status error instead of calling the runtime
   with an empty ID.

3. **Render a compact `HostInfoEvent` detail view.** Include:
   - session ID, summary, created/last-active timestamps
   - model/model override, context percentage, total cost
   - turn list with sequence, compacted marker, model, cost, and summary
   - files touched and files modified
   - server events

4. **Append recent runs for that session.** Call `run.list` with the target
   `session_id` and a small limit, then append a `Runs:` section showing run ID,
   status/stage, attachment state, input-required/stuck markers, and title.
   This gives background agents/durable runs a first-class place in the session
   overview without flattening run transcripts into the foreground session
   transcript.

5. **Keep drill-down commands run-native.** The detail view should point users
   at existing run commands for deeper inspection:
   - `/run <run-id>` for read-only run details
   - `/attach <run-id>` for active/detached run streams
   - `/runs` for current-session run list

6. **Update help text.** Mention `/sessions show [id]` or `/sessions details
   [id]` in the session command row.

7. **Preserve turn sequencing across session switches.** `/sessions resume`,
   bootstrap resume, and `/clear` must seed/reset the host's next
   `turn.submit` sequence so newly submitted turns continue the selected
   session instead of making the history appear stuck at the first persisted
   turn/run.

## Out of Scope

- **Transcript rehydration on session resume.** This patch inspects persisted
  turns; it does not replay them into the visible conversation when switching
  sessions.
- **Interactive session browser/sidebar.** Sidebar session navigation,
  command-palette session search, and richer keyboard drill-down belong in
  FEAT-0024.
- **Agents overlay.** Live multi-agent status UI belongs in FEAT-0024 once the
  FEAT-0017 run surface is ready enough to feed it.
- **Full run transcript rendering inside `/sessions show`.** Run transcript
  and event history remain under `/run <id>` / `/attach <id>` so foreground
  session history and background-agent history stay separate.
- **TUI-only clear keybinding.** `Cmd+K` is usually terminal-owned on macOS, so
  modeltap should implement `Ctrl+L` as the reliable in-TUI "clear visible
  transcript, keep current session" binding, with optional `/cls`. That should
  land either as a small follow-up patch or inside FEAT-0024 alongside keyboard
  chrome. It must not change `/clear`, which PATCH-0039 defines as "new
  session."
- **`/history` semantics.** `/history` remains reserved for command-history or
  a later explicit design. This patch avoids overloading it with session
  transcript browsing.
- **Session delete/prune cleanup tooling.** PATCH-0040 owns
  `/sessions delete <id>` and `/sessions prune`.

## Checklist

- [x] `handleSessionCommand` accepts `show` and `details` subcommands
- [x] New host helper resolves omitted session ID to `r.mode.SessionID()`
- [x] Host helper calls `session.details` and formats a compact detail view
- [x] Host helper calls `run.list` for the target session and appends recent
  run summaries when available
- [x] `/help` session row includes the new detail command
- [x] Tests:
  - `/sessions show <id>` calls `session.details` for that ID
  - `/sessions show` uses the active session ID
  - missing ID with no active session returns a status error
  - detail formatter includes turns, files, events, and recent runs
  - resumed and cleared sessions seed the next submit sequence correctly
- [x] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [x] `.sdlc/patches/README.md` index updated

## Fix Detail

Implementation should stay in the host layer:

- `internal/harnesshost/production_runtime.go`
  - extend `handleSessionCommand`
  - add `handleSessionDetails(ctx, id string)`
  - add a small formatter, mirroring `formatRunDetails` / `handleRunsCommand`
- `internal/harness/client.go` already has `SessionDetails`; host code can use
  `CallInto` directly as it does for the current run/session commands.
- `internal/protocol/sessions.go` already defines `SessionDetail` and
  `TurnSummary`.

The run section is deliberately a summary only. The user should drill into each
run with `/run <run-id>` or `/attach <run-id>` because a session can contain
multiple foreground and background runs, and those run transcripts are separate
durable streams under FEAT-0017.

Session switching also needs to preserve the runtime/user-turn sequence
contract. `session.resume` now returns `next_sequence`, computed from the
restored conversation's user-turn sequence. The host uses that value when
adopting a resumed session, and resets to sequence 1 when `/clear` creates a new
session. A plain `turn.submit` acknowledgement still records the returned
session ID without resetting sequence, so the first-turn auto-create path keeps
working.
