---
patch: "PATCH-0011"
title: "Harness UX Polish — OpenCode Theme Port, Borders, Sensible Keybindings"
status: "done"
date: "2026-04-21"
related:
  - "FEAT-0009 (terminal harness)"
  - "ADR-0013 (Bubbletea)"
  - "ADR-0010 (Apache-2.0 license compatibility)"
  - "PATCH-0003 (app ↔ conn-mgr wiring)"
branch: "exploration/integrated-harness"
---

# PATCH-0011: Harness UX Polish — OpenCode Theme Port, Borders, Sensible Keybindings

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

## Problem

The v0.2.0 harness ships a Bubbletea TUI with three issues that make it unusable for real users, and a baseline visual style that is well below the bar for a tool others will be asked to use.

1. **Default submit key is unreachable.** `internal/harness/keys.go:29-50` defaults to `SubmitKeyCtrlEnter`, bound to the keycode `"ctrl+@"` (the sequence bubbletea receives when terminals translate Ctrl+Enter). On a stock Mac keyboard in Terminal.app or iTerm2 there is no practical keystroke that produces Ctrl+@, so pressing Enter drops a newline into the textarea and there is no chord that sends the message. The CLI at `internal/cli/harness.go:252` never passes `AppOptions.SubmitKey`, so every user lands on this unreachable default. FEAT-0009:620 documents `harness.submit_key` as a config key, but the config path is not plumbed.
2. **Mode toggle is discoverable only by Ctrl+P.** `keys.go:57-60` binds `ToggleMode` only to `ctrl+p`. Tab — the industry-standard mode-switch key in comparable TUIs — does nothing. FEAT-0009 success criterion #9 names Ctrl+P explicitly; Tab must be added as an alias, not as a replacement.
3. **No theme system, minimal visual chrome.** `statusbar.go` and `viewport.go` use raw `lipgloss.Color("10")` / `"11"` / `"12"` / `"13"` ANSI indices directly with no palette abstraction, no adaptive light/dark behavior, no border chrome around the viewport or input, no prompt glyph, no streaming spinner, and no themed markdown rendering. The result is serviceable as a scaffold but visually below the bar for a tool intended for real users.

All three surfaced in the same session: the user could not execute `/help` (issue 1), could not `Tab` between build and plan mode (issue 2), and reported "the terminal UI is awful looking — I was expecting a look like OpenCode" (issue 3).

## Scope

This patch does three things: ports the OpenCode theme system under MIT attribution, wires it into the existing harness components, and fixes the keybinding defaults.

### A. Keybindings (the unblock)

1. **Flip default `submit_key` to `enter`** in `internal/harness/keys.go`. Shift+Enter / Alt+Enter retain their textarea behaviour (insert newline).
2. **Plumb `harness.submit_key` config → `AppOptions.SubmitKey`** in `internal/cli/harness.go:252`. Config values `enter | ctrl+enter | esc-enter` per FEAT-0009:620 are respected; unknown values log a warning and fall back to `enter`.
3. **Add `"tab"` to `KeyMap.ToggleMode`** alongside `"ctrl+p"`. Since the global keymap is checked in `app.go:175` before routing to the focused child, `tab` will not leak into textarea input.

### B. Theme system port (from sst/opencode, MIT)

Port the following files from `github.com/sst/opencode` at commit `f68374a^1` (the last commit before the Go+Bubbletea TUI was deleted in favor of OpenTUI on 2025-11-02). Source license: MIT. Destination license: Apache-2.0. MIT→Apache-2.0 is compatible; MIT requires the copyright notice and permission notice be preserved.

- `packages/tui/internal/theme/theme.go` → `internal/harness/theme/theme.go`
- `packages/tui/internal/theme/manager.go` → `internal/harness/theme/manager.go`
- `packages/tui/internal/theme/system.go` → `internal/harness/theme/system.go`
- `packages/tui/internal/theme/loader.go` → `internal/harness/theme/loader.go`
- `packages/tui/internal/theme/loader_test.go` → `internal/harness/theme/loader_test.go`
- `packages/tui/internal/theme/themes/*.json` (24 files) → `internal/harness/theme/themes/*.json`

Adaptation work for each ported Go file:

- Replace `github.com/charmbracelet/lipgloss/v2` imports with the v1 import already used by modeltap (`github.com/charmbracelet/lipgloss` at the pin in `go.mod:9`).
- Replace `compat.AdaptiveColor{Dark, Light}` with `lipgloss.AdaptiveColor{Dark, Light}` (v1's equivalent type; same two string fields).
- Replace `lipgloss.Cyan` / `lipgloss.Red` / `lipgloss.Magenta` / `lipgloss.Yellow` ANSI color constants (v2 lipgloss has these as `color.Color` values) with their v1 equivalents: `lipgloss.Color("6")` / `"1"` / `"5"` / `"3"`. Centralize these in a small `ansi.go` helper so the port reads close to the original.
- Drop the image/color dependency where it only existed to satisfy `compat.AdaptiveColor`.

Port `packages/tui/internal/styles/` selectively:

- `styles/styles.go` → `internal/harness/styles/styles.go` (style builders and utilities that the theme system needs)
- `styles/background.go` → `internal/harness/styles/background.go` (background-fill helpers used by the status bar strip)
- `styles/utilities.go` → `internal/harness/styles/utilities.go` (padding / border helpers)
- **Skip** `styles/markdown.go` for now. The modeltap `MarkdownRenderer` (`internal/harness/markdown.go`) already uses Glamour with a stock style; wiring it through the theme's syntax-highlighting palette is a follow-up.
- **Skip** `packages/tui/internal/components/*`. Modeltap has its own `InputArea`, `ConversationViewport`, `StatusBar`, dialog / modal surfaces. Porting OpenCode components would duplicate rather than improve these.

### C. Apply the theme to existing components

- `internal/harness/statusbar.go` — `StatusBarStyle` fields populate from the active theme. Status bar renders as a full-width background-colored strip with `·` separators, `[●]` / `[◐]` / `[↻]` / `[✗]` connection indicator colored from theme status colors, mode indicator on an accent background, model / context / cost segments styled consistently.
- `internal/harness/viewport.go` — `ViewportStyle` populates from theme. Assistant header shows a subtle horizontal rule above the model name in muted accent. User prefix `❯ ` in accent color (replaces `> `). Tool call / tool result glyphs keep their current `⚙` / `✓` but pick up theme status colors. Viewport is wrapped in a rounded `lipgloss.Border` colored in `theme.BorderSubtle()`.
- `internal/harness/input.go` — textarea prompt becomes `❯ ` in accent color. Input area is wrapped in a rounded border colored in `theme.Border()` when focused, `theme.BorderSubtle()` when viewport has focus.
- `internal/harness/app.go` — streaming spinner. Reuse `github.com/charmbracelet/bubbles/spinner` (already transitively available via `bubbles/textarea`). Spinner ticks while `state.CallActive` is true; rendered in the status bar immediately to the left of the call-duration timer.
- `internal/harness/markdown.go` — no theme port here; follow-up patch.

### D. Terminal-background detection and theme selection

- `harness.theme` config key per FEAT-0009:624 — values `auto | dark | light | <named-theme>`. `auto` default is terminal-bg detection (uses the ported `theme/system.go`). `dark` / `light` force a palette side. A named theme (e.g. `catppuccin`, `dracula`, `nord`) selects that specific JSON theme.
- Ported `theme/system.go` uses OSC 11 to query the terminal bg; on non-supporting terminals it falls back to the `COLORFGBG` env var, then to dark as a final default. This is OpenCode's existing logic; port as-is.
- Unicode / color fallback: when `TERM` is `dumb` or does not advertise 256-color support, borders, spinner, and prompt glyph all fall back to ASCII (`+-|`, `|`, `>`). Already the pattern used by lipgloss; add a single guard in `theme.Manager` that downgrades the palette to a 16-color safe subset.

### E. Attribution (non-negotiable per MIT §1)

1. **`NOTICE` file at repo root** (new) — credits sst/opencode at commit `f68374a^1`, MIT copyright notice reproduced verbatim. Modeltap's own Apache-2.0 LICENSE remains unchanged.
2. **Per-file header** on every ported `.go` and `.json` file:
   ```
   // Portions ported from github.com/sst/opencode at commit f68374a^1 (2025-10-31).
   // Original source MIT-licensed, copyright (c) 2025 opencode. See NOTICE.
   ```
3. **README credit line** in the harness section pointing readers at `NOTICE`.

### F. Docs and tests

- Update `.sdlc/features/0009-terminal-harness.md`? **No** — FEAT-0009 already names `submit_key`, `theme`, Bubbletea, Ctrl+P, borders are unspecified. This patch implements already-specified surface area.
- Update `internal/harness/statusbar_test.go` and `viewport_test.go` — these tests already use `SetStyle(plain)` to avoid ANSI noise in golden-string assertions, so most should survive. Update any that assert exact prefix text (`> ` → `❯ `).
- `internal/harness/theme/loader_test.go` ports with the rest; it validates that all 24 theme JSONs parse cleanly.
- `internal/cli/harness_test.go` (if present) — add a case asserting `harness.submit_key` config flows to `AppOptions.SubmitKey`.
- `.sdlc/patches/README.md` — register this patch.
- `.sdlc/releases/v0.2.0/changelog.md` — append entry.
- `.sdlc/history/2026-04-21-session-harness-ux-polish.md` — session log after completion.

## Out of Scope

- **Glamour / markdown theming.** The ported `styles/markdown.go` is skipped. Wiring the theme's syntax-highlighting palette into the markdown renderer is its own follow-up patch (likely `PATCH-0012`) once the base theme system is in and wired.
- **Porting OpenCode components.** `components/textarea`, `components/toast`, `components/dialog`, `components/modal`, `components/list`, `components/diff` stay un-ported. Modeltap has its own equivalents and a component port would be rewrites, not ports.
- **Upgrading lipgloss to v2.** OpenCode used v2; modeltap uses v1. The upgrade is cross-cutting and better done as its own patch if we need v2 features later. Ported code is adapted to v1 inline.
- **Session switcher / slash-command overlay UI inspired by OpenCode.** Any overlay not already in FEAT-0009 would be new behavior and needs either a FEAT-0009 amendment or a separate feature spec.
- **Theme live-reload.** Themes load at startup only. Reloading on config change is a reasonable follow-up but not in this patch.
- **Custom user themes.** The ported loader supports `~/.config/modeltap/themes/*.json` and `<project>/.modeltap/themes/*.json` override directories. Surfacing these in documentation and validating schema errors is part of the follow-up markdown-theme patch.

## Checklist

### Keybindings
- [ ] `DefaultKeyMap("")` defaults to `SubmitKeyEnter`
- [ ] `internal/cli/harness.go` reads `harness.submit_key` from config and passes it to `NewApp(AppOptions{SubmitKey: ...})`
- [ ] Unknown `submit_key` values log a warning and fall back to `enter`
- [ ] `KeyMap.ToggleMode` binds both `ctrl+p` and `tab`
- [ ] `/help` can be executed by typing `/help` then pressing Enter on a stock Mac keyboard in Terminal.app

### Theme port
- [ ] `internal/harness/theme/` package created, ported from `f68374a^1` with v1-lipgloss adaptation
- [ ] All 24 JSON theme files ported byte-for-byte (data only, no code changes needed)
- [ ] `theme/loader_test.go` passes — all themes parse
- [ ] `theme.Manager` correctly resolves `auto | dark | light | <named>`
- [ ] `theme/system.go` terminal-bg detection compiles and runs (manual verification; no CI support for TTY tests)
- [ ] `ansi.go` helper maps v2 ANSI constants to v1 equivalents used by ported code

### Styles port
- [ ] `internal/harness/styles/` package created with `styles.go`, `background.go`, `utilities.go`

### Attribution
- [ ] `NOTICE` file at repo root with MIT copyright reproduction
- [ ] Per-file header on every ported file
- [ ] README credit line in the harness section

### Wire-up
- [ ] `StatusBar` consumes theme; renders as full-width strip with separators
- [ ] `ConversationViewport` wrapped in rounded border; user prefix `❯ ` in accent color
- [ ] `InputArea` wrapped in rounded border; prompt becomes `❯ `
- [ ] Streaming spinner visible in status bar while `CallActive`
- [ ] Terminal falls back to ASCII chrome when `TERM=dumb`

### Tests and docs
- [ ] `statusbar_test.go` / `viewport_test.go` updated for prefix / glyph changes
- [ ] New test: config `submit_key` flows to `AppOptions`
- [ ] `.sdlc/patches/README.md` index entry
- [ ] `.sdlc/releases/v0.2.0/changelog.md` entry
- [ ] Session log in `.sdlc/history/`
- [ ] `make fmt-check vet build test` all green

## Fix Detail

### Commit breakdown

Per `CLAUDE.md` Commit Policy, commit at each work-unit boundary:

1. `PATCH-0011: draft harness UX polish patch doc` (this file)
2. `PATCH-0011: default submit key to enter, plumb submit_key config`
3. `PATCH-0011: add tab alias for mode toggle`
4. `PATCH-0011: port OpenCode theme package (MIT, attributed)` — theme files, NOTICE, per-file headers
5. `PATCH-0011: port OpenCode style primitives (MIT, attributed)`
6. `PATCH-0011: wire theme into statusbar, viewport, input; add borders and ❯ prompt`
7. `PATCH-0011: streaming spinner in status bar`
8. `PATCH-0011: ASCII fallback for dumb terminals`
9. `ADMIN: session log 2026-04-21 harness UX polish`

### v2 → v1 lipgloss adaptation cheat sheet

| v2 (ported source)                  | v1 (modeltap target)           |
|-------------------------------------|--------------------------------|
| `lipgloss/v2/compat.AdaptiveColor`  | `lipgloss.AdaptiveColor`       |
| `lipgloss.Cyan`                     | `lipgloss.Color("6")`          |
| `lipgloss.Magenta`                  | `lipgloss.Color("5")`          |
| `lipgloss.Red`                      | `lipgloss.Color("1")`          |
| `lipgloss.Yellow`                   | `lipgloss.Color("3")`          |
| `lipgloss.Green`                    | `lipgloss.Color("2")`          |
| `lipgloss.Blue`                     | `lipgloss.Color("4")`          |
| `lipgloss.White`                    | `lipgloss.Color("15")`         |
| `lipgloss.BrightRed` (etc.)         | `lipgloss.Color("9")` + offset |
| `color.Color` interface (image/color) | drop — not needed outside system.go |

`system.go` is the only file that keeps `image/color` — it needs `color.Color` to parse terminal-bg OSC 11 replies. Fine, that's a stdlib import.

### Why port rather than depend

OpenCode's Go TUI is no longer a maintained module — the directory was deleted on 2025-11-02 in favor of OpenTUI. There is no way to add it as a Go module dependency, and even at the last commit it lived inside the `sst/opencode` monorepo without a standalone `go.mod` for the `tui` package (it shared the monorepo's `go.mod` at `packages/tui/go.mod`). Source port with attribution is the only path.

### Why this doesn't touch provider / BFF code

Every change is in `internal/harness/`, `internal/cli/harness.go`, and new top-level files (`NOTICE`, `README.md` credit). No protocol surface changes. No BFF endpoint changes. Providers untouched. Config schema gains two keys that FEAT-0009 already named (`harness.submit_key`, `harness.theme`); no migration needed.
