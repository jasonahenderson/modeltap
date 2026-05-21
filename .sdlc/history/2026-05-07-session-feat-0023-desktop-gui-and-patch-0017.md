# 2026-05-07 — Desktop GUI Commitment, FEAT-0008 Amendment 001, PATCH-0017, FEAT-0023

## Context

User asked a sanity-check question: would the BFF be able to serve both a
TUI and a Mac/Windows GUI for non-technical users? After unpacking the
trade-offs, the conversation converged on:

1. The harness protocol is already client-class-agnostic.
2. The real architectural difference between TUI and GUI is **multi-chat
   per client** — the GUI runs N concurrent chats, each potentially in a
   different project root, while the TUI is the N=1 special case.
3. The current BFF implementation hoists project context to the
   connection scope (`internal/bff/capabilities.go:38`), and overwrites
   it on every `session.resume` (`internal/bff/session.go:169`). The
   `ActiveSession` row already has its own `Project` field, but every
   reader goes through the connection-scoped accessor.

User committed to shipping a desktop GUI for macOS and Windows as a
second harness client.

## Decisions

- **Desktop GUI is on the roadmap.** Native Mac/Windows; non-technical
  productivity users are an explicit audience.
- **Project root remains a first-class concept.** Even non-technical
  users work out of directories; the difference is that each chat tab
  in the GUI binds to its own project.
- **PATCH-0017 lands in v0.3.5** as the BFF-side prep work, before any
  GUI implementation begins.
- **FEAT-0023 is drafted now** to lock the direction; targets v0.4.0
  contingent on a pending harness-role-embodiment ADR.
- **FEAT-0008 gets an inline forward note + a "Planned Amendments"
  section** rather than a retroactive rewrite, since FEAT-0008 already
  shipped in v0.2.0/v0.2.2.

## Actions Taken

### Documentation

- `docs/features/0008-bff-server.md`
  - Inline note in the project-context paragraph (~line 156) flagging
    Amendment 001.
  - New `## Planned Amendments` section with full Amendment 001
    description before `## Resolved Questions`.
- `docs/patches/0017-session-scoped-project-context.md` (new): proposed
  patch with file:line scope, checklist, out-of-scope, and a "Fix
  Detail" section covering the deprecation-shim sequencing,
  `session.open` vs `session.resume` reuse, and back-compat.
- `docs/features/0023-desktop-gui-client.md` (new): draft feature spec
  covering multi-chat workspace, tool execution / permissions, simple
  mode for non-technical users, distribution, success criteria, and
  open questions (most importantly the harness-role embodiment, which
  is flagged as ADR-grade).

### Indices

- `docs/features/README.md`: added FEAT-0023 row.
- `docs/patches/README.md`: added PATCH-0017 row.
- `docs/releases/README.md`: added v0.3.5 row in the release index;
  added FEAT-0023 row in the feature-to-release mapping.

### Memory

- Saved `project_desktop_gui_commitment.md` (project-type memory) and
  added an index entry to `MEMORY.md`. Captures the commitment, the
  multi-chat driver, the PATCH-0017 prep work, and the open
  harness-role-embodiment question.

## Files Created or Modified

- Created: `docs/patches/0017-session-scoped-project-context.md`
- Created: `docs/features/0023-desktop-gui-client.md`
- Created: `docs/history/2026-05-07-session-feat-0023-desktop-gui-and-patch-0017.md` (this log)
- Modified: `docs/features/0008-bff-server.md` (inline note + "Planned Amendments" section)
- Modified: `docs/features/README.md` (FEAT-0023 row)
- Modified: `docs/patches/README.md` (PATCH-0017 row)
- Modified: `docs/releases/README.md` (v0.3.5 row + FEAT-0023 mapping)
- Created: `~/.claude/.../memory/project_desktop_gui_commitment.md`
- Modified: `~/.claude/.../memory/MEMORY.md`

## What's Next / Open Items

1. **Approve PATCH-0017.** Move status `proposed` → `approved` so it
   can be sequenced into a v0.3.5 plan when v0.3.4 closes.
2. **Open the harness-role-embodiment ADR.** Until this is settled,
   FEAT-0023 cannot leave `draft`. Three live options: native (Swift +
   .NET), Go sidecar, hybrid (Wails-style Go + webview).
3. **Promote FEAT-0023 from `draft` → `proposed`** once the
   embodiment ADR has at least a clear option set.
4. **Decide v0.3.5 scope.** PATCH-0017 may not be the only thing that
   lands there; revisit when v0.3.4 nears close.
5. **Decide bundling model.** Whether the desktop installer ships its
   own BFF or requires a separate `modeltap` install — open question
   #3 in FEAT-0023.

## Notes

- No code was changed. All work was design-time documentation and the
  forward amendment note in FEAT-0008.
- v0.3.0 implementation work (current Phase 3) was not touched.
- The conversation is captured in the memory file and this log; future
  sessions resuming from `docs/releases/v0.3.0/status.md` should still
  find the active release context unchanged.
