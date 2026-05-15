# Pre-Review: Bubbletea Scaffold Bundle (WU-068 through WU-072)

**Reviewer:** Claude subagent (pre-review lint)
**Date:** 2026-04-16
**Design doc:** `.sdlc/history/2026-04-16-design-bubbletea-scaffold-068-069-070-071-072.md`
**Checked against:**
- `.sdlc/features/0009-terminal-harness.md` (FEAT-0009 spec)
- `internal/protocol/messages.go` and `internal/protocol/protocol.go` (protocol types)
- `.sdlc/releases/v0.2.0/track-b-terminal-harness.md` (WU descriptions)

---

## Findings

### Blocking

#### B1. `AppState.Mode` is `string`, protocol defines `protocol.Mode` (typed string)

The design declares `Mode string` in `AppState` (line 83 of the design doc) with raw string values `"plan"`, `"build"`, `"auto"`. The protocol package defines `type Mode string` with constants `ModePlan`, `ModeBuild`, `ModeAuto` in `internal/protocol/protocol.go`. The design lists `internal/protocol/messages.go` as a consumed dependency for the `Mode` type, but then does not use it -- it redeclares Mode as a plain string.

This creates two problems:
1. No compile-time validation that Mode values match the protocol constants.
2. If the protocol Mode type gains a `Valid()` guard (it already has one), the harness bypasses it.

**Fix:** `AppState.Mode` should be `protocol.Mode`, and all assignments should use `protocol.ModePlan`, `protocol.ModeBuild`, `protocol.ModeAuto`.

#### B2. `ConnState.State` uses undocumented string values; no shared type

The design uses raw strings `"ready"`, `"degraded"`, `"reconnecting"`, `"failed"` for `ConnStateInfo.State`. The protocol package has no corresponding type or constants for connection states. The FEAT-0009 spec defines a 9-state lifecycle (starting, authenticating, registering, ready, degraded, reconnecting, draining, failed, closed -- see FEAT-0008 reference in Connection UX). The design only covers 4 of these states in the connection indicator.

While the full 9-state machine is WU-074 scope, the `ConnStateInfo` struct defined here in WU-068 must accommodate all states or it becomes a breaking change when WU-074 lands.

**Fix:** Either import connection states from a shared location, define the full set of state constants in the design, or explicitly document that WU-074 will extend `ConnStateInfo.State` and which states are deferred.

#### B3. Status bar missing mode bracket rendering per FEAT-0009

FEAT-0009 specifies the mode indicator appears in brackets in the status bar:
```
[plan] claude-opus-4-6 | 47% ctx | $0.42 | timer 3.2s
[build] claude-opus-4-6 | 47% ctx | $0.42 | timer 3.2s
```

The design's `StatusBar` (D3) has a `Mode lipgloss.Style` field and the test `TestStatusBar_ModeDisplay` says "plan/build/auto render correctly," but there is no rendering code shown for the mode display. Meanwhile the `connectionIndicator()` function is fully specified. The status bar `View()` method is declared but not shown.

This is blocking because the status bar is the primary surface for mode visibility (FEAT-0009 success criterion 8), and the design omits the rendering logic. Compare: connection indicator has full pseudocode; mode indicator has none.

**Fix:** Add a `modeDisplay()` method showing the bracket-wrapped rendering `[plan]`/`[build]`/`[auto]` consistent with FEAT-0009.

---

### Attention

#### A1. Status bar connection indicator is leftmost in FEAT-0009, position not specified in design

FEAT-0009 explicitly places the connection indicator as the "leftmost element":
```
[bullet] [build] claude-opus-4-6 | 47% ctx | $0.42 | timer 3.2s
```

The design defines `connectionIndicator()`, `contextDisplay()`, and `timerDisplay()` as separate methods but never specifies their layout order in the status bar. The `View()` method is declared but not implemented in the design. Implementation could accidentally reorder elements.

**Fix:** Document the left-to-right order: connection indicator, mode, model, context, cost, timer. This matches FEAT-0009 and the WU-069 description.

#### A2. `DisplayMessage.Role` uses raw strings; FEAT-0009 defines tool_call/tool_result as distinct message types

The design defines `Role string` with values `"user"`, `"assistant"`, `"tool_call"`, `"tool_result"`, `"system"`. These are not defined as constants anywhere. While the protocol notification types (WU-040, out of scope) would be the canonical source, defining string constants for display roles within the harness package prevents typos and enables exhaustive switches.

**Fix:** Define `const` values for the five role strings in `messages.go` or `model.go`.

#### A3. `SubmitMsg.Attachments` is `[]string` but FEAT-0009 attachment wire format has 5 fields

The design's `SubmitMsg` has `Attachments []string` (just file paths). FEAT-0009 specifies attachments carry `raw`, `content`, `content_type`, and `transform` fields. The protocol's `Attachment` struct has all five fields.

While WU-082 (File Context Management) is responsible for the full attachment pipeline, the `SubmitMsg` type defined here in WU-068 must be compatible. If WU-082 has to change `SubmitMsg.Attachments` from `[]string` to `[]protocol.Attachment`, that is a breaking interface change to a type other WUs may already depend on.

**Fix:** Either type `Attachments` as `[]protocol.Attachment` now, or explicitly document in the "Interfaces Exported" section that WU-082 will change this field's type.

#### A4. Ctrl+P toggle cycles plan<->build only; FEAT-0009 confirms this but design omits auto from toggle

FEAT-0009: "Ctrl+P toggles between plan and build (the two most common modes)." The design's `KeyMap` has `ToggleMode key.Binding // Ctrl+P` but the test `TestApp_ModeToggle` says "Ctrl+P toggles plan <-> build" without specifying what happens if the current mode is `auto`. If the user is in auto mode and presses Ctrl+P, the behavior is undefined.

**Fix:** Specify that Ctrl+P from auto mode goes to build (the default), or to plan, and add a test case for it.

#### A5. No `ModeToggleMsg` or mode-change Bubbletea message defined

The design defines many `Msg` types (Submit, StreamToken, ConnState, ModelUpdate, etc.) but no message for mode changes. Ctrl+P is listed in the `KeyMap` but the resulting state change has no corresponding Bubbletea message. If mode changes need to propagate to downstream WUs (WU-080, the status bar), a message type is needed.

**Fix:** Add a `ModeChangeMsg` to the message types, or document that mode changes are handled by direct `AppState.Mode` mutation in the Update loop (and explain why that is safe for the status bar's reactive rendering).

#### A6. Design claims dependency on `protocol/messages.go` for Mode and method constants but uses neither

The "Dependencies Consumed" section states: "`internal/protocol/messages.go` (WU-039): `Mode` type, method constants (for command routing)." However:
- `Mode` is defined in `protocol.go`, not `messages.go`.
- No method constants from the protocol package are referenced anywhere in the design.
- The design redefines Mode as a plain string.

This is misleading for downstream implementers.

**Fix:** Correct the dependency reference to `protocol.go` for the `Mode` type. Remove the "method constants" claim if no protocol methods are used by these five WUs (they are UI-only, no server calls).

#### A7. Missing banner rendering in FEAT-0009 transient states

FEAT-0009 specifies transient banners for connection establishment:
```
Starting local server...
Authenticating (OIDC)...
Registering tools (14 built-in + 3 MCP)...
```

The design defines `BannerMsg` and `BannerClearMsg` types and mentions `bannerLines()` in layout calculation, but does not specify where banners render (above input? below viewport?) or how the banner area interacts with the three-zone layout.

**Fix:** Specify banner placement. FEAT-0009 implies banners are above the status bar and below the viewport. The layout comment says "0-2 lines for transient banners" but the visual position relative to input/viewport is not stated.

---

### Nit

#### N1. `AppOptions.InitialMode` is `string`, should be `protocol.Mode`

Same issue as B1 but for the options struct. Consistent typing prevents misconfiguration at construction time.

#### N2. `PasteDetectedMsg` vs `PastDetectedMsg` typo in comment

Line 479 of the design: `type PasteDetectedMsg struct` -- this is correct, but the function doc at line 478 says "Returns a PastDetectedMsg" (missing 'e'). Minor inconsistency.

#### N3. Test table for WU-068 missing Ctrl+P from auto mode case

`TestApp_ModeToggle` says "Ctrl+P toggles plan <-> build" but does not list a test for starting in auto mode. Related to A4.

#### N4. `formatTokens` function signature shown but not specified

`formatTokens(n int) string` with examples `1234 -> "1.2K"`, `12345 -> "12K"`, `123456 -> "123K"` but the exact rounding/formatting rules are not specified. For example, is 999 rendered as `"999"` or `"1.0K"`? Is 1000 `"1.0K"` or `"1K"`?

#### N5. Glamour `WithWordWrap` vs Glamour `WithWrap`

The design uses `glamour.WithWordWrap(width)`. Verify the actual Glamour API -- some versions use `glamour.WithWordWrap()` and others use `glamour.WithWrap()`. This is implementation-time verification, not a design issue per se, but worth noting.

#### N6. `healPartialMarkdown` does not account for HTML blocks

The streaming tolerance logic (D6.1) handles fenced code blocks, inline code, and bold/italic, but does not mention raw HTML blocks (`<details>`, `<summary>`, etc.) which models sometimes emit. Low priority for initial implementation but worth a TODO.

---

## Summary

| Severity | Count |
|----------|-------|
| Blocking | 3 |
| Attention | 7 |
| Nit | 6 |

**Overall assessment:** The design is well-structured and covers the five WUs comprehensively. The main issues are type-safety gaps where the design uses raw strings instead of the already-defined protocol types (B1, A3, A6), incomplete connection state coverage (B2), and a missing mode rendering specification in the status bar (B3). None of these require a fundamental redesign -- they are tightening passes that align the design with the existing protocol types and FEAT-0009 spec text.
