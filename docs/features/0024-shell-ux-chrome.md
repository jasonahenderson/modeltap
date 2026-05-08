---
feature: FEAT-0024
title: Shell UX Chrome — sidebar, command palette, slash autocomplete, agents view
status: draft
date: 2026-05-08
related:
  - FEAT-0014 (Harness Conversation Shell)
  - PATCH-0015 (Harness Shell Component API)
  - PATCH-0023 (Dispatch host-native slash commands)
  - PATCH-0027 (Strip misleading footer hints)
  - WU-100 §"Definite scope rule for the reusable package"
supersedes: none
---

# FEAT-0024: Shell UX Chrome — sidebar, command palette, slash autocomplete, agents view

## Problem

Three productivity surfaces existed in the original `internal/harnessspike` and worked end-to-end during the v0.2.x spike phase:

- **Sidebar** (Ctrl+B): a left-rail listing recent sessions, the current model, and quick actions, focusable for keyboard navigation.
- **Command palette** (Ctrl+K): a centered overlay with type-to-filter command search across sessions, actions, views, and toggles.
- **Agents view** (Ctrl+T): an overlay showing background agents and their streamed status output.

WU-100 (`8f2d392`, 2026-04-27) extracted the conversation shell into `internal/harnessshell` (reusable component) + `internal/harnesshost` (modeltap-specific adapter) + `internal/harnessdemo` (fake runtime). WU-100's commit message explicitly **dropped these three surfaces** from the reusable package per its "Definite scope rule for the reusable package":

> "Spike-only chrome (sidebar, command palette, agent overlays, background-agent state, presets) is dropped from the post-extraction architecture. These were spike experiments outside the FEAT-0014 conversation-shell contract; they may resurface in a future modeltap top-level harness package but are not WU-100 work."

The "may resurface in a future modeltap top-level harness package" clause was never executed. As a result:

- Users who relied on the spike's keyboard-first navigation lost it on the v0.2.1 → v0.3.0 transition.
- The reusable package's footer hint string still advertised `Ctrl+B sidebar  Ctrl+T agents  Ctrl+K palette` until PATCH-0027 stripped it (recorded as Finding F9 in `docs/releases/v0.3.0/retrospective.md`).
- Slash-command discoverability is poor — `PATCH-0023` made slash commands dispatch correctly, but the user has no way to see what commands exist short of reading source.

This feature re-homes the spike chrome in the host (`internal/harnesshost`) where WU-100 said it should live, plus adds an inline slash-command autocomplete that the spike did not have.

## Personas

- **Power TUI user** (primary). Wants keyboard-first navigation, quick session switching, and command discoverability without leaving the composer. Lost productivity when the spike features were dropped.
- **First-time modeltap user**. Cannot discover the slash-command vocabulary without source-diving. Needs an inline autocomplete that surfaces the catalog when typing `/`.
- **Agent-team operator** (FEAT-0013). Runs multiple background agents and needs a glanceable status panel; the agents overlay was the spike's solution.

## Stories

1. **Open the sidebar with Ctrl+B; close it with Ctrl+B.** Sidebar lists current session, recently-resumed sessions, current model, and a few quick actions (clear transcript, replay intro). Keyboard-navigable when focused; focus cycles via Tab.
2. **Open the command palette with Ctrl+K.** Type to filter; arrow keys / j/k to navigate; Enter to run; Esc to dismiss. Catalog is the union of host slash commands and palette-only commands (e.g., "View: Background Agents", "Toggle: Sidebar").
3. **See an inline slash-command autocomplete dropdown when I type `/` in the composer.** As I keep typing, the list filters by prefix and substring match. Up/Down/Tab navigates, Enter inserts the command (with a space) for me to add args, Esc dismisses.
4. **Open the agents view with Ctrl+T.** Lists the active background agents (FEAT-0017) with their status and the latest streamed line of output. Selecting one drills into a per-agent detail with full streamed text. Esc returns to the conversation.
5. **No surface lies.** Footer hint, in-overlay help text, and `--help` examples reflect only the surfaces that exist in the running build.

## Architecture decisions to make

### A. Where do these features live?

- The reusable `internal/harnessshell` is intentionally minimal per WU-100 §"Definite scope rule". Adding sidebar/palette/agents back into it would relitigate WU-100.
- The host (`internal/harnesshost` + `internal/cli/shell.go`) is where WU-100 said they should land.
- **Decision:** implement in the host. The `harnesshell` reusable component grows two new surfaces (a way for the host to inject pre-composer content above the transcript, and a way for the host to inject footer hint text) but does not own the chrome itself.

### B. How does the palette discover host commands?

- Hardcode the catalog in `internal/harnesshost/palette_commands.go` to start. Each entry has `name`, `args-template`, `kind` (host-command / shell-action / view-toggle), `description`.
- Future: BFF could expose a `commands.list` RPC that returns server-defined commands.

### C. How do new chrome surfaces compose with the existing transcript renderer?

- Sidebar: new `Sidebar *RenderSidebar` field on `RenderInput`. When non-nil, the renderer reserves left-side width and calls a sidebar render path. The host populates it via a new shell option / projection.
- Palette: full-screen overlay. Renderer either composes via `overlayString` (the spike's approach) or the host renders the overlay outside `Render()`. **Recommended:** overlay support inside `harnessshell.Render()` so paste-preview, palette, and agents overlay all use the same composition primitive.
- Agents view: same overlay primitive as palette.
- Footer hint extension: new `FooterHint string` field on `RenderInput` for host-supplied hint text. Reusable component owns nothing about Ctrl+B / Ctrl+K / Ctrl+T.

### D. Inline slash-command autocomplete

- This is **not** in the spike — net-new. UX baseline is Codex / Claude Code:
  - Triggered when the composer buffer starts with `/` and the cursor is on the first line.
  - Renders a dropdown directly above (or below) the composer with `(name, description)` rows.
  - Filtering is prefix+substring match against `name` and `description`.
  - Up/Down/Tab navigate, Enter completes with a trailing space, Esc closes.
- Implementation: shell-owned dropdown that consumes a host-supplied catalog (same source as palette). Lives in `harnessshell` because it sits inside the composer; the catalog comes through the host injection seam.

### E. Background-agent state plumbing

- Needs FEAT-0017's "background agents" surface to be live.
- For v0.3.x, agents view can show a "no agents yet" placeholder until FEAT-0017 lands.

## Work units (proposed)

- **WU-NNN-A: harnessshell injection seams.** Add `Sidebar *RenderSidebar`, `Overlay *RenderOverlay`, `FooterHint string`, `CommandCatalog []PaletteCommand` to `RenderInput`. Render path threads them. No host-side change.
- **WU-NNN-B: host sidebar.** Implement sidebar in `harnesshost`: state, content, toggle action, focus integration. Wire Ctrl+B in `cli/shell.go` or via a new shell option that intercepts the keypress before the textarea sees it.
- **WU-NNN-C: host command palette.** Implement palette in `harnesshost`: catalog, query state, filter, render via the overlay seam, run-command dispatch (host commands route through existing `RunHostCommandAction`; palette-only actions like "Toggle Sidebar" route locally). Wire Ctrl+K.
- **WU-NNN-D: inline slash autocomplete.** Shell-owned dropdown component that shows when the composer starts with `/`. Consumes the host's catalog via the new injection seam. Tab/Enter/Esc behavior.
- **WU-NNN-E: agents view.** Overlay listing background agents from FEAT-0017's runtime state. Read-only for v0.3.x (no agent control). Wire Ctrl+T.
- **WU-NNN-F: footer hint extension.** Host populates `FooterHint` to advertise its keybindings. Reusable component shows it verbatim.

## Success criteria

- [ ] Ctrl+B opens/closes a working sidebar with at least: current session, recent sessions, current model, "Clear transcript" action.
- [ ] Ctrl+K opens a command palette listing every dispatchable host slash command plus a few palette-only actions; type to filter, Enter to run.
- [ ] Typing `/` in the composer surfaces an inline autocomplete dropdown filtered by what's typed; Tab/Enter completes.
- [ ] Ctrl+T opens an agents view (placeholder OK for v0.3.x; live for FEAT-0017+).
- [ ] No on-screen surface advertises a binding or feature that doesn't work.
- [ ] All existing tests pass; the new chrome surfaces have unit tests and a host-side test that exercises the keypath through to the dispatched action.

## Source-of-truth from the spike

The spike at git ref `8f2d392~1:internal/harnessspike/app.go` is the authoritative reference for the working spike chrome. Recover with:

```sh
git show 8f2d392~1:internal/harnessspike/app.go > /tmp/spike-app.go
git show 8f2d392~1:internal/harnessspike/styles.go > /tmp/spike-styles.go
```

Key sections (line numbers in the spike file):

### E1. Palette state types (lines 96–106)

```go
type commandPalette struct {
    query string
    index int
}

type paletteCommand struct {
    label  string
    value  string
    kind   string
    filter string
}
```

### E2. Sidebar items + state (lines 152–162, 207–215)

```go
type App struct {
    ...
    sidebarItems []sidebarItem
    sidebarIndex int
    palette      *commandPalette
    sidebarOpen  bool
    ...
}

// Initial seed
sidebarItems: []sidebarItem{
    {section: "Session", label: "Spike Session", kind: sidebarItemSession, value: "spike-session"},
    {section: "Session", label: "Reference Layout", kind: sidebarItemSession, value: "reference-layout"},
    {section: "Session", label: "Dummy Stream", kind: sidebarItemSession, value: "dummy-stream"},
    {section: "Model", label: "fake-kimi-spike", kind: sidebarItemModel, value: "fake-kimi-spike"},
    {section: "Actions", label: "Clear Transcript", kind: sidebarItemAction, value: "clear"},
    {section: "Actions", label: "Replay Intro", kind: sidebarItemAction, value: "replay"},
    {section: "Actions", label: "Replay Tool Demo", kind: sidebarItemAction, value: "tool-demo"},
},
```

### E3. Ctrl+B / Ctrl+K / Ctrl+T keybinding handlers (lines 342–363)

```go
case strings.ToLower(msg.String()) == "ctrl+b":
    a.sidebarOpen = !a.sidebarOpen
    if !a.sidebarOpen && a.focus == focusSidebar {
        a.focus = focusTranscript
    }
    if a.sidebarOpen {
        a.status = "Sidebar opened"
    } else {
        a.status = "Sidebar closed"
    }
    a.layout()
    return a, nil
case msg.Type == tea.KeyCtrlK:
    a.openPalette()
    return a, nil
case msg.Type == tea.KeyCtrlT:
    a.openAgents()
    return a, nil
```

### E4. Palette open + key handler (lines 1386–1438)

```go
func (a *App) openPalette() {
    a.palette = &commandPalette{}
    a.status = "Command palette open"
}

func (a *App) handlePaletteKey(msg tea.KeyMsg) tea.Cmd {
    if a.palette == nil {
        return nil
    }
    switch msg.Type {
    case tea.KeyEsc:
        a.palette = nil
        a.status = "Command palette closed"
        return nil
    case tea.KeyUp:
        if a.palette.index > 0 {
            a.palette.index--
        }
        return nil
    case tea.KeyDown:
        if a.palette.index < len(a.filteredCommands())-1 {
            a.palette.index++
        }
        return nil
    case tea.KeyBackspace:
        if len(a.palette.query) > 0 {
            a.palette.query = a.palette.query[:len(a.palette.query)-1]
            a.palette.index = 0
        }
        return nil
    case tea.KeyEnter:
        commands := a.filteredCommands()
        if len(commands) == 0 {
            return nil
        }
        a.runPaletteCommand(commands[a.palette.index])
        return nil
    case tea.KeyRunes:
        a.palette.query += msg.String()
        a.palette.index = 0
        return nil
    }
    return nil
}
```

### E5. Palette filter (lines 1441–1471)

```go
func (a *App) filteredCommands() []paletteCommand {
    var commands []paletteCommand
    commands = append(commands,
        paletteCommand{label: "Session: Spike Session",   value: "Spike Session",   kind: "session", filter: "session spike"},
        paletteCommand{label: "Session: Reference Layout",value: "Reference Layout",kind: "session", filter: "session reference layout"},
        paletteCommand{label: "Session: Dummy Stream",    value: "Dummy Stream",    kind: "session", filter: "session dummy stream"},
        paletteCommand{label: "Session: Tool Demo",       value: "Tool Demo",       kind: "session", filter: "session tool demo permission events"},
        paletteCommand{label: "Action: Clear Transcript", value: "clear",           kind: "action",  filter: "action clear transcript"},
        paletteCommand{label: "Action: Replay Intro",     value: "replay",          kind: "action",  filter: "action replay intro"},
        paletteCommand{label: "View: Background Agents",  value: "agents",          kind: "view",    filter: "view agents background"},
        paletteCommand{label: "Toggle: Sidebar",          value: "sidebar",         kind: "toggle",  filter: "toggle sidebar"},
    )
    query := strings.ToLower(strings.TrimSpace(a.palette.query))
    if query == "" {
        return commands
    }
    var out []paletteCommand
    for _, cmd := range commands {
        if strings.Contains(strings.ToLower(cmd.label), query) || strings.Contains(cmd.filter, query) {
            out = append(out, cmd)
        }
    }
    return out
}
```

### E6. Sidebar render (lines 962–1011)

```go
func (a App) renderSidebar() string {
    if !a.sidebarOpen {
        return ""
    }
    width := clamp(a.width/4, 24, 32)
    var b strings.Builder
    b.WriteString(sidebarTitleStyle.Width(width - 4).Render("modeltap"))
    b.WriteString("\n\n")
    currentSection := ""
    for i, item := range a.sidebarItems {
        if item.section != currentSection {
            if currentSection != "" {
                b.WriteString("\n")
            }
            currentSection = item.section
            b.WriteString(sidebarMetaStyle.Render(strings.ToUpper(currentSection)))
            b.WriteString("\n")
        }
        style := sidebarItemStyle
        prefix := "  "
        if item.kind == sidebarItemModel {
            style = sidebarValueStyle.Padding(0, 1)
        }
        if i == a.sidebarIndex {
            style = sidebarItemActiveStyle
            prefix = "• "
            if a.focus == focusSidebar {
                style = sidebarItemFocusedStyle
                prefix = "› "
            }
        }
        b.WriteString(style.Width(width - 4).Render(prefix + item.label))
        b.WriteString("\n")
    }
    return sidebarBoxStyle.Width(width).Height(max(a.height, 12)).Render(b.String())
}
```

### E7. Palette render (lines 1313–1360)

```go
func (a App) renderPalette() string {
    width := clamp(a.width-12, 56, 88)
    commands := a.filteredCommands()
    var b strings.Builder
    b.WriteString(dialogTitleStyle.Render("Command Palette"))
    b.WriteString("\n")
    b.WriteString(paletteQueryStyle.Render("> " + a.palette.query))
    b.WriteString("\n\n")
    if len(commands) == 0 {
        b.WriteString(dialogHintStyle.Render("No matching commands"))
    } else {
        for i, cmd := range commands {
            style := dialogOptionStyle
            prefix := "  "
            if i == a.palette.index {
                style = dialogOptionActiveStyle
                prefix = "› "
            }
            b.WriteString(style.Width(width - 6).Render(prefix + cmd.label))
            b.WriteString("\n")
        }
    }
    b.WriteString("\n")
    b.WriteString(dialogDividerStyle.Render(strings.Repeat("─", max(width-4, 20))))
    b.WriteString("\n")
    b.WriteString(dialogHintStyle.Render(/* type / Enter / Esc footer */))
    return dialogBoxStyle.Width(width).Render(b.String())
}
```

### E8. Overlay composition primitive (lines 1372+)

The spike's `overlayString` renders the palette (and other dialogs) by rune-replacing into the base layout. Reused as the proposed shared overlay primitive in WU-NNN-A.

```go
func overlayString(base, overlay string, x, y int) string {
    baseLines := strings.Split(base, "\n")
    overlayLines := strings.Split(overlay, "\n")
    for row, line := range overlayLines {
        targetRow := y + row
        if targetRow < 0 || targetRow >= len(baseLines) {
            continue
        }
        baseRunes := []rune(baseLines[targetRow])
        overlayRunes := []rune(line)
        if x > len(baseRunes) {
            baseRunes = append(baseRunes, []rune(strings.Repeat(" ", x-len(baseRunes)))...)
        }
        // ...rune-by-rune replace...
    }
    return strings.Join(baseLines, "\n")
}
```

The full implementation lives at lines 1364–1385 of the spike file; recovery via `git show 8f2d392~1:internal/harnessspike/app.go`.

## Out of Scope

- **Restoring the spike file directly.** The post-extraction architecture has a different module shape; we re-implement against the current architecture using the spike as design reference, not as code to revert.
- **Background-agent control.** Read-only agents view for v0.3.x. Cancel/retry/inspect lands with FEAT-0017.
- **Mouse-driven palette / sidebar interaction.** Keyboard-first; mouse comes later if needed.
- **Theme / color customization.** Use the existing `harnessshell` styles or extend them; theming is out of scope.
- **Server-driven catalog.** Static host catalog for v0.3.x; `commands.list` RPC is a follow-up if needed.

## Sequencing

This feature is **not v0.3.0**. PATCH-0027 strips the misleading footer hint so v0.3.0 ships honest, and FEAT-0024 is the canonical place to track the real implementation.

Likely v0.3.x slot: **v0.3.1 or v0.3.2** depending on the rest of the v0.3.0 close-out and the F4 sub-items (10a stale-daemon detection, 10d `modeltap status` probe, 10e single-terminal mode, 10f binary-mismatch warning) that also slipped from v0.3.0.
