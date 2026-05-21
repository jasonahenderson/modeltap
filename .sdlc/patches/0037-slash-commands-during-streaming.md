---
patch: "PATCH-0037"
title: "Dispatch slash commands before queue check so /cancel works during streaming"
status: "proposed"
date: "2026-05-12"
related:
  - "PATCH-0023 (host-command dispatch)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F20)"
branch: "patch/0037-slash-commands-during-streaming"
---

# PATCH-0037: Dispatch slash commands before queue check

## Problem

`/cancel <run-id>` (and `/run`, `/runs`, `/detach`, `/sessions clear`,
any other host slash command) typed while a run is streaming gets
**enqueued as user content** instead of dispatched as a host command.
The user sees a queued submission marker in the transcript and the
in-flight run continues to completion — there is no working
cancellation path other than Esc Esc.

Surfaced during smoke step 11 ("Count slowly from 1 to 200…") when
the user typed `/cancel <run-id>` and saw it queued rather than
acted on.

Root cause: `internal/harnessshell/queue.go:108-181`
(`emitSubmitOnEnter`) tests `s.streaming` at line 141 **before** the
slash-command dispatch at line 170. The streaming-queue branch
matches any non-empty input including slash commands, so it short-
circuits and enqueues `/cancel` as a user message.

Recorded as Finding F20 in `.sdlc/releases/v0.3.0/retrospective.md`.

## Scope

1. **Reorder `emitSubmitOnEnter`** so the slash-command-prefix branch
   runs **before** the streaming/queue branch. Empty Enter (queue
   release, permission resolve) keeps its current position; `/clear`
   shell-native remains shell-side.

2. **Test slash-during-streaming.** New case in
   `internal/harnessshell/queue_test.go` (or `events_test.go`) that
   sets `s.streaming = true`, types `/cancel run-x`, and asserts
   the shell emits `RunHostCommandAction{Name: "cancel"}` instead
   of enqueuing.

3. **Test that non-slash content still queues during streaming.**
   Regression guard: typing `"hello"` while streaming must still
   enqueue, not dispatch.

## Out of Scope

- **Mid-stream `/clear` behavior.** That depends on PATCH-0039's
  redefinition; handled there.
- **Permission-pending blocking.** A pending permission can still
  intercept Enter at the permission-resolve branch (line 126); that
  ordering is unchanged.
- **Queued-submission cancellation.** Cancelling an *already*-queued
  user message is a separate concern (queue-management UX, FEAT-0024).

## Checklist

- [ ] Slash-prefix branch precedes streaming-queue branch in
  `emitSubmitOnEnter`
- [ ] Test: `/cancel run-x` during streaming dispatches
  `RunHostCommandAction`, not `SubmitTurnAction`
- [ ] Regression test: plain text during streaming still enqueues
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` F20 entry references
  this patch as fix

## Fix Detail

The reorder in `emitSubmitOnEnter`:

```go
// Empty-Enter handling (queue release, permission resolve) stays at top.
if content == "" && len(s.inputTokens) == 0 { ... }

s.pushHistory(content)

// PATCH-0037: slash commands dispatch immediately regardless of
// streaming state, so /cancel, /run, /detach, etc. can take effect
// in-flight. /clear is the shell-native exception below.
if content == shellNativeClearCommand && len(s.inputTokens) == 0 {
    // existing /clear shell-native handling
    return true
}
if strings.HasPrefix(content, "/") && len(content) > 1 && len(s.inputTokens) == 0 {
    s.dispatchHostCommand(content)
    return true
}

// Non-slash content: queue while streaming, otherwise begin.
if s.streaming || len(s.queuedSubmissions) > 0 || len(s.pendingSubmissions) > 0 {
    s.enqueueSubmission(content, s.inputTokens)
    ...
    return true
}
s.beginSubmission(content, ...)
return true
```
