---
patch: "PATCH-0038"
title: "Add /help command listing the host slash-command surface"
status: "proposed"
date: "2026-05-12"
related:
  - "PATCH-0023 (host-command dispatch)"
  - ".sdlc/releases/v0.3.0/retrospective.md (Finding F22)"
branch: "patch/0038-help-command"
---

# PATCH-0038: Add /help command listing the host slash-command surface

## Problem

The production shell has no `/help` command. The host's slash-command
surface (modes, model, sessions, context, run/runs, attach/detach/
cancel, retry/continue/fork, plus shell-native /clear /select /quit
/exit) is undocumented from inside the binary — users have to read
source code or guess. Surfaced when the user typed `/sessions` to
discover what subcommands exist and there was no in-shell way to
find out.

Recorded as Finding F22 in `.sdlc/releases/v0.3.0/retrospective.md`.

## Scope

1. **Register `/help` as a host command** in
   `internal/harnesshost/production_runtime.go` (`DispatchCommand`
   switch). Argument-less invocation lists the full surface; future
   `/help <name>` per-command detail is out of scope for this
   patch (FEAT-0024).

2. **Print the command list as a HostInfoEvent** so it lands in the
   transcript per the PATCH-0018 host-info pattern. Format:

   ```
   Available commands:

     modes:     /plan  /build  /auto
     model:     /model <name>  /models
     session:   /sessions [list|resume <id>|clear|fork]
     context:   /context  /compact
     history:   /history
     mcp:       /mcp
     runs:      /run [<id>]  /runs  /jobs
     lifecycle: /attach <id>  /detach  /cancel  /retry  /continue  /fork
     shell:     /clear  /select  /help  /quit  /exit
   ```

3. **Test** that DispatchCommand with `{Name: "help"}` emits a
   HostInfoEvent containing each command name as a substring.

## Out of Scope

- **`/help <command>`** per-command argument detail. The host doesn't
  currently expose a structured help map; adding one is FEAT-0024
  scope (consolidated with the structured-status surface and the
  command discovery work).
- **Sidebar / command palette / autocomplete.** Larger UX surfaces
  tracked under FEAT-0024.
- **Updating `/sessions clear` / `/clear` semantics** — that is
  PATCH-0039's territory; the help text written here pre-emptively
  reflects PATCH-0039's `/sessions clear` subcommand naming.

## Checklist

- [ ] `case "help"` added to `DispatchCommand` switch
- [ ] `handleHelpCommand` emits the formatted HostInfoEvent
- [ ] Unit test asserts each top-level command name appears in the
  emitted text
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] `.sdlc/patches/README.md` index updated
- [ ] `.sdlc/releases/v0.3.0/retrospective.md` F22 entry references
  this patch as fix
