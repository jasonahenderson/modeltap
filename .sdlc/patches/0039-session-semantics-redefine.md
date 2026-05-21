---
patch: "PATCH-0039"
title: "Redefine /clear as new-session; auto-resume most-recent session on launch; /sessions current"
status: "proposed"
date: "2026-05-12"
related:
  - "PATCH-0028 (session.create RPC + harness bootstrap)"
  - "PATCH-0029 (bootstrap race fix)"
  - "PATCH-0040 (session.delete / session.prune; follow-up)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Findings F21, F23)"
branch: "patch/0039-session-semantics-redefine"
---

# PATCH-0039: Redefine /clear; auto-resume on launch; /sessions current

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

Three intertwined session-UX problems surfaced during smoke testing:

1. **Sessions accumulate**. PATCH-0028's `session.create` on every
   `ConnStateReady` mints a fresh session at every shell launch. Over
   a debugging cycle the DB collects N orphan sessions; `/sessions
   list` returns "a bunch."

2. **`/clear` is semantically wrong**. The shell-native `/clear` wipes
   the transcript display only — the BFF's working memory is intact,
   so the model keeps responding as if it remembers prior turns. This
   conflicts with what Claude Code / OpenCode / Codex users expect:
   `/clear` should start a fresh conversation.

3. **`/runs` and `/run` disagree** (F21). `/run <id>` finds any run
   regardless of session, but `/runs` filters to the harness's current
   session id. Runs from prior shell launches live on prior session
   ids and don't appear in the current `/runs`. Surfaces as "ghost
   runs" the user cannot list.

Recorded as Findings F21, F23 in
`.sdlc/releases/v0.3.0/retrospective.md`.

## Scope

### Shell-side / Harness

1. **Redefine `/clear` as host-routed "new conversation"**. `/clear`
   no longer wipes the transcript locally; instead it dispatches a
   `RunHostCommandAction{Name: "clear"}` to the host. The host:
   - Refuses while a run is streaming (`HostStatusEvent` error
     "cannot start new conversation while a run is in flight; press
     Esc twice to cancel first")
   - Calls `session.create` to mint a fresh session id
   - Updates `r.mode.SetSessionID(newID)`; clears `ActiveRunID`
   - Emits a typed `TranscriptClearEvent` so the shell wipes its
     transcript (only on success)
   - Emits a status event "Started new conversation: <id>"

2. **Auto-resume on launch.** Replace `bootstrapSession`'s
   unconditional `session.create` with: list the user's sessions for
   the current project (`session.list`), pick the most-recent active
   one, call `session.resume`. Fall back to `session.create` only when
   the list is empty.

   The shell's transcript stays empty on resume (rehydrating the
   visible transcript from BFF turns is FEAT-0024 scope), but the BFF
   carries the prior conversation context so the model continues from
   where it left off.

3. **Welcome message.** On first `ConnStateReady` after bootstrap,
   emit a `HostInfoEvent` that names the session and how to start
   fresh:
   - On resume: `"Resumed session <id>. Type /clear to start a new
     conversation. Type /sessions list to see all sessions."`
   - On create: `"New session <id>. Type /help for commands."`

4. **`/sessions current` subcommand**. Prints the active session id
   via `HostInfoEvent`. Useful for cross-referencing with `/run` /
   `/runs` output.

### Out of scope (PATCH-0040)

- `session.delete` RPC and `/sessions delete <id>` command
- `session.prune` RPC and `/sessions prune` command
- Storage `DeleteSession`, `DeleteEmptySessions` methods

These land separately so each patch stays atomic.

### Other out of scope

- **Transcript rehydration on resume.** The shell's visible
  transcript stays empty after auto-resume; the BFF carries the
  model's working memory. Rendering prior turns into the shell on
  resume is FEAT-0024 / harness-TUI re-completion scope.
- **Mid-stream `/clear`.** Refused with a clear error. Allowing it
  would require cancelling the active run first and reasoning about
  partial state; not worth the complexity for v0.3.0.
- **Time-based auto-prune** of stale sessions on launch.
- **Welcome message persistence**: shown once on connect, not on
  every reconnect.

## Checklist

- [ ] Shell-native `/clear` becomes a slim dispatcher: emits a
  `RunHostCommandAction{Name: "clear"}` and lets the host own the
  actual reset
- [ ] New `TranscriptClearEvent` typed HostEvent; shell `applyHostEvent`
  clears `transcriptItems`, `transcriptRefs`, `selectedTranscriptRef`
  and sets a status when this arrives
- [ ] Host `handleClearCommand` rejects while streaming, otherwise
  calls `session.create`, switches active session, emits
  `TranscriptClearEvent` + status
- [ ] `bootstrapSession` calls `session.list` first; resumes most-
  recent or creates if none
- [ ] Welcome `HostInfoEvent` emitted on bootstrap (resume vs. new
  flavor)
- [ ] `/sessions current` subcommand handler
- [ ] `/help` text updated to mention `/sessions current`
- [ ] Tests:
  - shell-side: `/clear` dispatches `RunHostCommandAction{Name: "clear"}`
    instead of wiping locally; the new transcript wipe happens on
    `TranscriptClearEvent`, not on Enter
  - host-side: `handleClearCommand` rejects when active run streaming;
    success path calls session.create, updates active id
  - bootstrap: list-returns-empty → create; list-returns-one → resume
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` F21 / F23 entries
  reference this patch
