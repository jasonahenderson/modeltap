# Session Log — Harness debug scope + build unblock

**Date:** 2026-04-22
**Branch:** exploration/integrated-harness
**Context:** Resumed from `docs/history/2026-04-21-handoff-harness-debug.md`. Goal was to debug and review the v0.2.0 Bubbletea harness. Detoured midway to unblock a broken `make` and fix a storage bug that surfaced in the test run.

## What was discussed / decided

### Harness usability review (no code landed yet)

User ran the harness and reported three issues:

1. **`/help` could not be sent** — Enter inserted a newline instead of submitting.
2. **Tab did not toggle build↔plan mode.**
3. **"The terminal UI is awful looking; I was expecting a look like OpenCode."**

Diagnosis:

1. `internal/harness/keys.go:29-50` defaults `submitKey` to `SubmitKeyCtrlEnter`, which bubbletea binds to `"ctrl+@"`. On a stock Mac keyboard in Terminal.app / iTerm2 there is no practical keystroke that produces Ctrl+@. The CLI at `internal/cli/harness.go:252` never plumbs `AppOptions.SubmitKey` from config despite FEAT-0009:620 documenting `harness.submit_key`, so every user lands on the unreachable default.
2. `keys.go:57-60` binds `ToggleMode` only to `ctrl+p`. Tab was never wired. FEAT-0009 success criterion #9 names Ctrl+P explicitly, but Tab is an industry-standard alias that must be added, not substituted.
3. `statusbar.go` and `viewport.go` use raw `lipgloss.Color("10")` / `"11"` / `"12"` / `"13"` ANSI indices with no palette abstraction, no borders, no prompt glyph, no streaming spinner. Below the bar for a tool intended for real users.

### OpenCode lift investigation

User asked if we could replicate OpenCode directly rather than describing OpenCode-inspired styling from memory. Web + GitHub research produced:

- OpenCode's TUI **was** Go + Bubbletea with a mature theme system, but was **deleted on 2025-11-02** (commit `f68374a`: *"DELETE GO BUBBLETEA CRAP HOORAY"*) in favor of OpenTUI (TypeScript).
- The **old Go + Bubbletea code at `f68374a^1`** is MIT-licensed (Apache-2.0 compatible per ADR-0010). Usable as a source port with attribution.
- Contents at that commit under `packages/tui/internal/`: `theme/` (6 Go files + 24 JSON theme files: Catppuccin, Dracula, Nord, Tokyo Night, Gruvbox, Rosé Pine, Solarized, Kanagawa, Material, Ayu, Monokai, Everforest, GitHub, Cobalt 2, Nightowl, Palenight, Mellow, Matrix, Vesper, Synthwave84, Zenburn, Aura, One Dark, OpenCode's own), `styles/` (`background.go`, `markdown.go`, `styles.go`, `utilities.go`), `components/` (chat, textarea, dialog, modal, status, toast, list, diff, qr), `layout/`, plus `app/`, `commands/`, `viewport/`, `util/`.

### Scope-negotiation walk

Three paths surfaced:

- **Medium-A (~1-2 days, 50-65% parity):** port `theme/` + `styles/` (including `markdown.go`) into modeltap under lipgloss v1 adaptation (`compat.AdaptiveColor` → v1 `lipgloss.AdaptiveColor`). No chat/textarea/layout port. This is essentially the original PATCH-0011 draft.
- **Medium-B (~10-14 days, ~85% parity):** Medium-A plus port `components/chat/`, `components/textarea/`, `components/layout/`. **Requires upgrading modeltap to bubbletea v2 / bubbles v2 / lipgloss v2** first — the chat and textarea components are hard-wired to v2 APIs (`compat.AdaptiveColor`, v2 `tea.Msg` types, custom viewport). v2 is still pre-1.0 beta.
- **Full lift (~3-4 weeks, ~95% parity):** everything including dialogs, modals, agents overlay, timeline.

Initial estimate for "medium" as ~4-5 days was **wrong** — didn't account for the v2 dependency wall. Corrected the estimate mid-conversation.

**Decision pending.** User said "medium — look and feel of opencode, but wired to our BFF," which maps to Medium-B, but the v2-upgrade reality check shifted the tradeoff. No final direction chosen.

### Build unblock

User's `make` failed at the `lint` target because `golangci-lint` was not installed on the machine. PATCH-0010's open checklist item (lint unverified) became a real blocker.

Chose: remove `lint` from the default `all:` target, keep `make lint` as an explicit target that still fails loudly when the binary is missing. Landed as **PATCH-0012**.

### Storage bug fix

Running the new `make fmt-check vet test build` surfaced a failing integration test:

```
--- FAIL: TestMetricsAggregation (3.03s)
    integration_test.go:535: timed out waiting for 3 stored requests (got 2)
ERROR failed to save captured request error="inserting request: database is locked (5) (SQLITE_BUSY)"
```

Initially flagged as "pre-existing flake." On the user's re-run the same failure surfaced (plus a follow-on `disk I/O error (1802)` / `SQLITE_IOERR_BLOCKED`), so investigated further and found the root cause at `internal/storage/sqlite.go:37`: DSN set `foreign_keys` and `journal_mode(WAL)` but **not `busy_timeout`**. Without it, `modernc.org/sqlite` returns `SQLITE_BUSY` immediately on any concurrent write contention instead of briefly waiting. **This is a production bug** — the proxy silently drops captured requests under concurrent upstream traffic. The failing test was the reproducible symptom, not the bug.

Fix: add `_pragma=busy_timeout(5000)` to the DSN. Landed as **PATCH-0013**.

## Actions taken

### Commits landed on `exploration/integrated-harness`

- `d54ce24` — **PATCH-0012: remove lint from default Makefile target**
- `50ebbfc` — **PATCH-0013: set sqlite busy_timeout on every pool connection**

Both signed-off (DCO). Both reference their canonical patch doc path in the commit body.

Note: while committing, a third commit `f3d6f8b ADMIN: document release tag policy` appeared on the branch between my last `git log` check and my own commits. Another worktree / tool landed it concurrently; not mine, left untouched, sits as parent of PATCH-0012.

### Patch docs drafted

- **PATCH-0011** — `docs/patches/0011-harness-ux-polish.md` (proposed, **uncommitted**). Initial scope: theme + styles port with v1 adaptation + keybinding fixes. **Scope is stale** after the v2-dependency reality check. Revision needed once user confirms Medium-A vs Medium-B vs full lift.
- **PATCH-0012** — `docs/patches/0012-lint-out-of-default-target.md` (proposed, committed). Rationale: opt-in-but-strict (no silent skip when `golangci-lint` is missing); `make lint` still fails loudly, `make` no longer does.
- **PATCH-0013** — `docs/patches/0013-sqlite-busy-timeout.md` (proposed, committed). Rationale: DSN pragma (connection-scoped, replayed on every `db` connection) rather than `db.Exec("PRAGMA busy_timeout")` (wrong connection). 5000ms is the Go/SQLite community convention.

### Index and changelog entries

- `docs/patches/README.md` — PATCH-0011 (index only; uncommitted), PATCH-0012, PATCH-0013 entries added.
- `docs/releases/v0.2.0/changelog.md` — PATCH-0012 and PATCH-0013 entries added.

## Files created / modified

**Committed:**
- `Makefile` — drop `lint` from `all:`
- `internal/storage/sqlite.go` — add `busy_timeout(5000)` to DSN
- `internal/storage/sqlite_test.go` — new `TestBusyTimeoutConfigured`
- `docs/patches/0012-lint-out-of-default-target.md` (new)
- `docs/patches/0013-sqlite-busy-timeout.md` (new)
- `docs/patches/README.md` — PATCH-0012 + PATCH-0013 index rows
- `docs/releases/v0.2.0/changelog.md` — PATCH-0012 + PATCH-0013 rows

**Uncommitted (intentionally — still in review / WIP):**
- `docs/patches/0011-harness-ux-polish.md` (scope revision pending)
- `docs/patches/README.md` — PATCH-0011 index row (re-added after the two commits landed)

**Untouched (pre-existing WIP on the tree from other work):**
- `README.md`, `docs/agents.md` / etc. entries in `.agents/`, `docs/features/.reviews/*`, `docs/patches/.reviews/*`, `internal/cli/harness.go`, `internal/config/config.go`, `internal/harness/keys.go`, untracked `docs/explorations/0011-upstream-feature-porting.md`, untracked `docs/history/2026-04-22-session-readme-header-modernization.md`, untracked `.claude/worktrees/`, untracked `crush.json` (IDE-opened only).

## What's next / open items

1. **PATCH-0011 scope decision** — user to pick between Medium-A (~2 days, theme-only port under v1) vs Medium-B (~2 weeks, v2 upgrade + theme + chat + textarea + layout port) vs full lift (~3-4 weeks). Draft doc needs revision after decision.
2. **Once PATCH-0011 scope is set** — the immediate keybinding unblock (Enter-submits + Tab-toggles + `submit_key` config plumbing) could land as its own small bundle inside PATCH-0011, regardless of which port scope is chosen. That alone gets `/help` working and Tab toggling — ~1 hour of work.
3. **Flip PATCH-0012 / PATCH-0013 status to `done`** after validating in the next session — separate small `ADMIN:` commits (they currently show as `proposed` in the index).
4. **`golangci-lint` install instructions** — not in PATCH-0012's scope, but worth a one-liner in the root README or CONTRIBUTING.md so contributors know `make lint` needs that binary.
5. **Broader tree WIP** (not mine) — modifications to `.agents/process.md`, `docs/agents.md`, root `README.md`, review findings files, `internal/cli/harness.go` / `internal/config/config.go` / `internal/harness/keys.go`, plus untracked `docs/explorations/0011-upstream-feature-porting.md` and a session log for 2026-04-22 README work. Outside this session's scope; flagged to the user during the commit phase and left untouched.
