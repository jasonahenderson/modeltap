---
patch: "PATCH-0023"
title: "Dispatch host-native slash commands as RunHostCommandAction"
status: "approved"
date: "2026-05-08"
related:
  - "FEAT-0014 (conversation shell)"
  - "PATCH-0018 (host-info events)"
  - "docs/releases/v0.3.0/retrospective.md (Finding F8)"
branch: "patch/0023-shell-host-command-dispatch"
---

# PATCH-0023: Dispatch host-native slash commands as RunHostCommandAction

## Problem

When the user types a host-native slash command in the production
shell (e.g. `/models`, `/sessions`, `/runs`, `/context`, `/history`,
`/mcp`, `/run`, `/attach`, `/detach`, `/cancel`, `/retry`,
`/continue`, `/fork`, `/model <name>`, `/plan`, `/build`, `/auto`,
`/compact`) the shell submits the literal text as a turn user
message instead of dispatching it as a host command. Anthropic
receives `{"role":"user","content":[{"type":"text","text":"/models"}]}`
and the model answers a question about `/models` as a string instead
of the user seeing the model catalog.

`internal/harnessshell/queue.go:18` documents the gap:

> Per WU-100 §"Definite scope rule for the reusable package", /clear
> is shell-native; host-native commands cross via
> [RunHostCommandAction] (added in a later commit).

That later commit never landed. The shell only intercepts `/quit`,
`/exit` (in `model.go` before `emitSubmitOnEnter` runs) and `/clear`
(inside `emitSubmitOnEnter`). Every other slash command falls
through to `beginSubmission` → `SubmitTurnAction`.

Recorded as Finding F8 in `docs/releases/v0.3.0/retrospective.md`.

## Scope

1. **Add a leading-slash check** in
   `internal/harnessshell/queue.go` `emitSubmitOnEnter`, after the
   existing `/clear` shell-native handling. Any non-empty input
   that starts with `/` and has no attached input tokens emits
   `RunHostCommandAction` instead of `SubmitTurnAction`. The
   shell does not pre-validate the command name; the host's
   `DispatchCommand` returns an "Unknown command" status for
   anything it does not recognize.

2. **Parse name and args** from the slash-prefixed input.
   `/models` → `name="models"`, `args=""`. `/model qwen3.5:35b`
   → `name="model"`, `args="qwen3.5:35b"`. `Raw` carries the
   original string verbatim for host-side use if needed.

3. **Reset composer state** on dispatch (input cleared, tokens
   cleared, status set to `"Running /<name>"`) so the buffer
   does not leak the command into a subsequent submission.

4. **Do not enqueue while streaming.** Place the host-command
   dispatch after the existing streaming/queue handling so that a
   host command typed mid-stream is queued and released after the
   active turn completes — matching the existing `/clear`
   semantics. Future work may revisit (host commands are
   independent BFF RPCs and could safely dispatch immediately).

5. **Tests**:
   - `internal/harnessshell/events_test.go` (or queue_test.go) —
     verify `/models` emits `RunHostCommandAction` with the right
     `Invocation.Name` and that no `SubmitTurnAction` is produced.
   - Verify that `/clear` still works locally (no regression).
   - Verify that `/quit` / `/exit` still trigger `tea.Quit` (not
     touched).

## Out of Scope

- **Host-command pre-validation.** The shell does not maintain
  a list of valid host commands; the host owns that vocabulary.
- **Host-command dispatch with attached input tokens.** A user
  who types `/models <paste-token>` falls through to
  `beginSubmission`; the host-command path requires
  `len(s.inputTokens) == 0`. Tokens-with-commands is undefined
  behavior in v0.3.0.
- **Immediate dispatch during streaming.** Deferred — keeps the
  existing `/clear` queuing semantics for now.
- **`/<unknown>` discoverability.** "Unknown command: /<name>"
  emits via `HostStatusEvent` from the host runtime (already
  wired); not visually distinct from other status events. UX
  refinement deferred.

## Checklist

- [ ] `emitSubmitOnEnter` dispatches host-native slash commands
  via `RunHostCommandAction`
- [ ] `CommandInvocation.Name` / `Args` / `Raw` set correctly
- [ ] Composer state reset on dispatch
- [ ] Tests added for `/models` dispatch and `/clear` no-regression
- [ ] `go test ./...` passes
- [ ] Smoke verification: `/models` in the production shell renders
  the catalog in the transcript (via the PATCH-0018 HostInfoEvent
  path) instead of submitting as a turn
- [ ] `docs/patches/README.md` index updated
- [ ] `docs/releases/v0.3.0/retrospective.md` Finding F8 status
  updated to "Fixed in PATCH-0023"
