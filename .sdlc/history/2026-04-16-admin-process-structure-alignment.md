# 2026-04-16 — ADMIN: Process Structure Alignment Start

Started the first pass of aligning `modeltap`'s repo-process structure with the
more explicit process layering used in `keyproxy/alpha`.

Created:

- `.agents/process.md`
- `.agents/contracts/base.md`
- `.agents/contracts/agent-team.md`
- `.sdlc/history/2026-04-16-admin-process-adoption-checklist.md`

Intent of this pass:

- establish a canonical tool-agnostic process file
- establish a minimal contract structure under `.agents/`
- reduce drift between `AGENTS.md`, `CLAUDE.md`, and `docs/agents.md`
- prepare a separate checklist doc for remaining adoption work

Follow-up work:

- repoint top-level instruction files to `.agents/*`
- decide whether `docs/agents.md` remains a human-friendly overview or becomes a
  thin pointer
- decide whether to add templates now or in a later `ADMIN:` pass
