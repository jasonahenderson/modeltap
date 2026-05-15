# Phase 2 Design Review: v0.2.1 WU-097 through WU-102

**Reviewer:** kimi-k2.6 (cloud)  
**Date:** 2026-04-26  
**Scope:** `plan.md`, `FEAT-0014`, `PATCH-0015`, and all six WU design docs (`097`–`102`)  
**Baseline:** `internal/harnessspike/app.go`, `internal/harness/app.go`, `internal/harness/app_test.go`

---

## Summary

The designs are directionally sound and preserve the accepted action/event boundary policy. However, several gaps create behavior-preservation risk during implementation, particularly around mid-stream permission pause/resume, the exact timing of assistant-row creation relative to host events, queue-state fidelity in the new type model, and a contradiction in the release dependency graph. The host adapter contract (`WU-099`) keeps shell internals out of modeltap at the import level, but it underspecifies how runtime-side stream buffering maps to the shell-visible pause/resume behavior that the spike currently implements locally.

---

## Findings

### 1. Dependency graph contradiction in plan.md
- **Severity:** blocking
- **Location:** `.sdlc/releases/v0.2.1/plan.md` table row WU-100, "Parallelizes With" column
- **What's wrong:** WU-100 lists WU-102 as parallelizable, but WU-102 has an explicit dependency on WU-100 (`Dependencies: WU-100`). A work unit cannot both depend on and run in parallel with another. The track file correctly places WU-102 on the critical path after WU-100, but the plan table contradicts this.
- **Suggested fix:** Remove WU-102 from WU-100's "Parallelizes With" column. Only WU-101 should be listed there.

### 2. Mid-stream permission pause mechanism is unspecified in host adapter
- **Severity:** significant
- **Location:** `WU-099` §Permission Integration Points; `WU-099` §Minimal Host Interface `Runtime` interface
- **What's wrong:** `FEAT-0014` requires that "a permission request that appears during streaming pauses the active stream immediately." The spike implements this in `pauseStreamingForPermission()`, which saves the remaining `streamQueue` into `pausedResponse` and stops local streaming ticks. `WU-099`'s `Runtime` interface has `SubmitTurn`, `InterruptRun`, `DispatchCommand`, `ResolvePermission`, `LoadPreview`, and `SummarizePaste` — but no `PauseRun`, `ResumeRun`, or stream-buffering surface. The design says "the host must pause it before or alongside this event" (`WU-098`), yet `WU-099` does not define how the adapter or runtime performs that pause, nor how buffered deltas are replayed after resolution. This will surface as an implementation surprise in `WU-100`.
- **Suggested fix:** Add an explicit stream-pause/resume or delta-buffering contract to the `Runtime` interface (or to the adapter's internal responsibility list). Specify whether the server/runtime pauses naturally at the tool boundary, or whether the adapter must explicitly buffer `StreamTokenMsg` deltas while a permission is pending and replay them after `PermissionResolvedEvent`.

### 3. Assistant row creation timing is ambiguous vs. spike behavior
- **Severity:** significant
- **Location:** `WU-098` §`RunStartedEvent`; `WU-098` §`SubmitTurnAction`
- **What's wrong:** In the spike, `beginSubmission()` appends the user message **and** immediately appends an assistant message with `streaming: true` before any fake stream ticks arrive. The user never sees a state where the user row exists but the assistant row does not. In `WU-098`'s event contract, `SubmitTurnAction` is emitted by the shell, and `RunStartedEvent` "transitions the shell from submitted/queued state into active streaming state." This implies the shell waits for the host to acknowledge before creating the assistant row. If the host has non-zero latency, the UX changes from the spike. The design does not explicitly state whether the shell optimistically creates the assistant row on submit (preserving spike behavior) or waits for `RunStartedEvent`.
- **Suggested fix:** State explicitly in `WU-098` that the shell creates the assistant transcript row optimistically when emitting `SubmitTurnAction`, and that `RunStartedEvent` serves only for run-ID correlation and host-side failure signaling. Alternatively, require that the host adapter send `RunStartedEvent` synchronously/immediately upon receiving `SubmitTurnAction` so no visible latency is introduced.

### 4. `pendingSubmissions` merge buffer not modeled in shell contract
- **Severity:** significant
- **Location:** `WU-098` §Queue invariants; `WU-100` §Step 5
- **What's wrong:** The spike uses both `queuedSubmissions` (visible queue) and `pendingSubmissions` (a transient buffer used during merge in `releaseQueuedSubmission()`). `WU-098` models queued work but never mentions a pending/merge buffer. `WU-100` says "preserve the distinction between: visible queued submissions, pending released submissions" but does not show how the new shell state encodes this. If `WU-100` flattens this into a single queue slice, the merge-order semantics could drift, especially when multiple queued items are released together.
- **Suggested fix:** Add `pendingSubmissions` (or equivalent internal merge state) to the shell-owned state inventory in `WU-098`, and verify in `WU-102` that multi-item queue merge preserves FIFO per the spike.

### 5. `HostStatusEvent` is too coarse for FEAT-0014 status behaviors
- **Severity:** significant
- **Location:** `WU-098` §`HostStatusEvent`
- **What's wrong:** `WU-098` defines `HostStatusEvent` as a single `Status string`. The spike displays rich, behavior-specific status text: "Streaming fake response", "Permission required (session policy active)", "Press Esc again to interrupt", etc. A single opaque string means the shell cannot make rendering decisions based on status type (e.g., whether to show a pulsing dot, whether to arm interrupt styling). This either forces all status semantics into the host adapter (which then knows too much about shell rendering) or forces the shell to parse status strings.
- **Suggested fix:** Replace the single string with a structured status event: `type HostStatusEvent struct { Status string; Kind StatusKind }` where `StatusKind` enumerates at least `ready`, `streaming`, `interrupt_armed`, `permission_pending`, `error`. This preserves the host's freedom to supply display text while giving the shell enough signal to drive chrome behavior.

### 6. Rendering cutover before action/event cutover is optimistic
- **Severity:** significant
- **Location:** `WU-100` §Extraction Sequence Step 2 (Stage B)
- **What's wrong:** Step 2 proposes moving "pure shell rendering and layout" into `internal/harnessshell` before replacing runtime coupling with actions/events (Step 3). The spike's `refreshTranscript()` is tightly coupled to spike-local state: `a.messages`, `a.queuedSubmissions`, `a.pendingPermissions`, `a.inputTokens`, etc. Moving rendering to a new package while keeping that state in the old spike package requires either (a) exposing all that state through accessors, or (b) duplicating the state model. Either approach is high-risk and undermines the goal of incremental extraction. `WU-100`'s "intermediate compatibility stages" acknowledge this but don't provide a concrete bridge strategy.
- **Suggested fix:** Reverse the order: move state and types first (Step 1), then action/event cutover (Step 3), then rendering cutover (Step 2). Rendering should follow the state model, not lead it. If the current order is retained, add an explicit Stage B½ where the spike translates its old state into the new shell state struct before calling the new renderer.

### 7. Style/theme dependency boundary is unaddressed
- **Severity:** significant
- **Location:** `WU-100` §New reusable package; `WU-098` §Proposed Package Layout
- **What's wrong:** The spike imports `github.com/charmbracelet/lipgloss` directly and defines local styles. The later harness line has `internal/harness/theme`. If `internal/harnessshell` imports `internal/harness/theme`, it gains a modeltap-specific dependency that violates PATCH-0015's separation requirement and future repository promotability. Neither `WU-098` nor `WU-100` states whether the shell package defines its own theme-neutral styles, accepts a theme interface, or avoids theming entirely.
- **Suggested fix:** State explicitly in `WU-098` that the reusable shell package must not import `internal/harness/theme` or any modeltap-specific style constants. Define a minimal style-config surface (e.g., `type StyleConfig struct { Prompt lipgloss.Style ... }` or simple color hex strings) passed via `Option`, or require that the shell package owns its own neutral style definitions.

### 8. Post-permission resolution text (`grantText`/`denyText`) origin undefined
- **Severity:** significant
- **Location:** `WU-098` §`PermissionResolvedEvent`; `WU-099` §Permission Integration Points
- **What's wrong:** The spike's `pendingPermission` struct carries `grantText` and `denyText`, which become the assistant response after resolution. `WU-098`'s `PermissionResolvedEvent` has a `Message string`, but neither design specifies who produces that message. In a real modeltap runtime, the tool result or server response would provide the text. But WU-099 does not say how the adapter maps a runtime tool result into `PermissionResolvedEvent.Message`, nor what happens if the runtime provides structured data instead of a string. If the adapter must synthesize fallback text, that logic should be documented.
- **Suggested fix:** In `WU-099`, add a sentence: "The adapter constructs `PermissionResolvedEvent.Message` from the runtime tool result payload; if the payload is empty or structured, the adapter falls back to a generic granted/denied message." In `WU-098`, note that `Message` may be host-synthesized and is the sole text appended to the assistant row on resolution.

### 9. `/perm` command listed as FEAT-0014 behavior but is spike-only
- **Severity:** significant
- **Location:** `FEAT-0014` §CLI / UI Integration; `WU-099` §Host-native commands
- **What's wrong:** `FEAT-0014` lists `/perm` under "In-shell behaviors covered by this feature." But `/perm` is a spike-only demo command that triggers fake permission requests. `WU-099`'s host-native command inventory (drawn from the later harness line) does not include `/perm`. This means the real modeltap host adapter will not support `/perm`, making the FEAT-0014 success criterion "run `/perm`" impossible to satisfy after extraction. The success criterion and feature spec should not depend on a demo-only command.
- **Suggested fix:** In `FEAT-0014`, replace `/perm` references with a host-agnostic description: "A permission request arrives from the runtime/tool layer and surfaces in the composer." Update the test criterion to use a fake-host injection or adapter-level test hook rather than a slash command.

### 10. WU-100 compatibility test strategy is underspecified
- **Severity:** significant
- **Location:** `WU-100` §Intermediate Compatibility Steps
- **What's wrong:** `WU-100` says "At each stage, the code must compile and the still-relevant parity tests must continue passing." The spike's `app_test.go` tests the monolithic `App` struct. As types move to `internal/harnessshell`, the old tests will break because they reference moved types (`queuedSubmission`, `inputToken`, etc.). The design does not explain how to keep the old test surface alive during migration — whether through temporary type aliases, a compatibility shim, or by immediately rewriting tests.
- **Suggested fix:** Add an explicit test-migration rule: either (a) keep temporary type aliases in `internal/harnessspike` that forward to `internal/harnessshell` types during Stages A–C, or (b) accept that `app_test.go` will not compile during extraction and require that `WU-102`'s new tests in `internal/harnessshell` are passing before any old tests are deleted.

### 11. Empty-Enter queue release action contract is contradictory
- **Severity:** significant
- **Location:** `WU-098` §Shell-native commands; `WU-098` §`SubmitTurnAction`
- **What's wrong:** `WU-098` classifies "empty `Enter` queue release while idle" as a **shell-native** command (local, does not cross boundary). But `SubmitTurnAction` is defined as "Emitted when the user submits immediately or queued work is released." These statements conflict: if queue release is shell-native, it should not emit a host-bound action. If it emits `SubmitTurnAction`, it is not purely shell-native. The host adapter needs to know whether a submission was direct or queue-released (`Source` field), but the shell's ownership of the release decision vs. the host's ownership of the actual turn submission is blurred.
- **Suggested fix:** Clarify the split: the shell **owns the decision** of when to release queued work (empty Enter while idle triggers release logic), but the shell **emits `SubmitTurnAction` with `Source = queue_release`** so the host performs the actual submission. The command is "shell-native" in the sense that the trigger condition is evaluated locally, but the effect crosses the boundary. Document this nuance in `WU-098` §Command Boundary.

### 12. `PermissionDecision` type never defined in WU-098
- **Severity:** advisory
- **Location:** `WU-098` §`ResolvePermissionAction`; `WU-098` §Permission invariants
- **What's wrong:** `ResolvePermissionAction` references `PermissionDecision`, and the invariant section lists `approve_once`, `approve_session`, `deny`. But no Go type definition for `PermissionDecision` appears in the design. WU-100 will have to invent it.
- **Suggested fix:** Add a minimal type definition to `WU-098`: `type PermissionDecision int` with `DecisionApproveOnce`, `DecisionApproveSession`, `DecisionDeny` constants, or a string type with documented valid values.

### 13. Action emission Bubble Tea mechanism is unspecified
- **Severity:** advisory
- **Location:** `WU-098` §Boundary Model / Shell emits
- **What's wrong:** The design says the shell "emits typed outbound actions by appending them to an internal action queue and returning a Bubble Tea command that forwards them to the host program." It does not define the actual command or message type the host should expect. For example, is it `func() tea.Msg { return ShellActionMsg{...} }`? Does the host register a custom message type and pattern-match on it in its own `Update` loop? Without this, `WU-099` cannot precisely define the adapter's intake path.
- **Suggested fix:** Define the concrete action-forwarding shape, e.g., a `type ActionMsg struct { Action Action }` that the shell returns as a `tea.Msg` from its `Update` method, or document that this is intentionally left to `WU-100`.

### 14. WU-099 and WU-100 file lists diverge
- **Severity:** advisory
- **Location:** `WU-099` §Proposed Package Layout; `WU-100` §New modeltap host package
- **What's wrong:** `WU-099` lists `projection.go` for event mapping. `WU-100` lists `runtime_events.go` for the same responsibility. The host adapter's expected file set is inconsistent across designs.
- **Suggested fix:** Reconcile to one name in both documents, or add a note that `WU-100` may rename files.

### 15. FEAT-0014 uses "callbacks" wording that contradicts PATCH-0015
- **Severity:** advisory
- **Location:** `FEAT-0014` §API Expectations
- **What's wrong:** The API Expectations bullet says the component should expose "explicit messages/callbacks for shell actions." PATCH-0015 §8 explicitly forbids callback-shaped contracts. WU-098 correctly inherits the no-callback policy, but FEAT-0014's own wording could confuse an implementer.
- **Suggested fix:** Change "messages/callbacks" to "typed action and event messages" in `FEAT-0014`.

### 16. PATCH-0015 checklist is entirely unchecked despite approved status
- **Severity:** advisory
- **Location:** `PATCH-0015` §Checklist
- **What's wrong:** All 13 checklist items are `[ ]`. The patch is marked "approved" and dated 2026-04-24. An approved patch with an entirely unchecked checklist suggests either the checklist is aspirational (in which case it should be labeled "Implementation Checklist" or moved to the release plan) or it was never updated after approval.
- **Suggested fix:** Either check off items that are already satisfied by the designs (e.g., "Define shell-owned responsibilities", "Define host-owned responsibilities"), or relabel the section as "Implementation Checklist" to indicate it tracks Phase 3 work.

### 17. WU-102 lacks detail on mid-stream permission verification approach
- **Severity:** advisory
- **Location:** `WU-102` §Permission behavior; `WU-102` §Layer 1 tests
- **What's wrong:** FEAT-0014 success criterion 7 requires testing that "mid-stream permissions pause the active stream immediately and resume or end cleanly." `WU-102` lists "mid-stream permission pause and resume behavior" as a parity target but does not specify how to exercise this in unit tests. Because the pause trigger now originates from the host side (via `PermissionRequestedEvent`), a pure shell unit test cannot trigger mid-stream pause without a fake host. The design mentions "test fakes for focused shell tests" as a valid host but does not say the fake host must support mid-stream permission injection.
- **Suggested fix:** Add a requirement in `WU-102` that the test fake host supports injecting `PermissionRequestedEvent` while a `RunDeltaEvent` stream is active, so that Layer 1 tests can verify the shell's pause/render/resume state transitions.

### 18. `isHostEvent()` private methods limit external test package flexibility
- **Severity:** advisory
- **Location:** `WU-098` §Boundary Model
- **What's wrong:** The sealed interface `type HostEvent interface { isHostEvent() }` prevents external test packages (e.g., `package harnessshell_test`) from implementing custom host events. While in-package tests can satisfy the interface, the design enforces a specific test package layout without stating it.
- **Suggested fix:** Either export the marker method (`IsHostEvent()`) or add a design note: "Tests that need custom host events must be in `package harnessshell` (not `package harnessshell_test`) or use the exported concrete event types only."

### 19. WU-100 conditional retention of sidebar/palette surfaces creates ambiguity
- **Severity:** advisory
- **Location:** `WU-100` §Step 2
- **What's wrong:** Step 2 says "keep sidebar, palette, preview, agent overlay, and footer rendering in the shell package only if they are part of the shell-owned interaction contract." `FEAT-0014` explicitly narrows scope to the conversation shell; sidebar, palette, and agent overlays are spike/demo chrome outside the contract. The conditional "only if" invites an implementer to argue they belong in the reusable package.
- **Suggested fix:** Replace the conditional with a definitive rule: "Sidebar, palette, agent list, command palette, and background-agent surfaces are spike/demo-only and must stay out of `internal/harnessshell`. Only transcript, composer, queued-work rendering, token surfaces, permission composer chrome, and preview dialog (for shell-owned paste tokens) may enter the reusable package."

### 20. WU-101 doc paths not explicitly named
- **Severity:** advisory
- **Location:** `WU-101` §Documentation Set / Embedding guide
- **What's wrong:** `WU-101` says docs should become "a developer-facing markdown doc under repo docs" but does not name a file path. Given project conventions (`.sdlc/features/`, `.sdlc/patches/`, `.sdlc/releases/`), the final doc location should be explicit.
- **Suggested fix:** Specify a path such as `docs/guides/harness-shell-embedding.md` or `internal/harnessshell/README.md`, or at minimum require that `WU-101` name the final paths in its acceptance criteria.

---

## Cross-Cutting Concerns

### Host adapter keeps shell internals out of modeltap
`WU-099` succeeds at the import level: `internal/harnessshell` must not import modeltap runtime packages, and the adapter is the only package that knows both sides. **However**, the host adapter contract is thin on runtime behavior specifics (stream pause, `/perm` command absence, post-permission message sourcing). This means modeltap runtime semantics could leak back into the shell during `WU-100` as implementation workarounds unless the adapter contract is tightened.

### Extraction sequence dependency-legality
The seven-step sequence in `WU-100` is mostly dependency-legal, but Stage B (rendering before action/event) has a hidden dependency on state shape that the design does not resolve. Reordering to state → actions → rendering would be safer.

### WU-102 parity coverage vs. FEAT-0014 success criteria
| Criterion | WU-102 Coverage | Gap |
|---|---|---|
| 1. Single scrolling surface + tail composer | Layer 1 (transcript/composer surface) | None |
| 2. Content wraps in narrow terminal | Layer 1 (transcript lines wrap) | `/demo` command is spike-only; test needs fake host |
| 3. Manual scroll preserved | Layer 1 (scroll preservation) | None |
| 4. Queue FIFO + empty Enter release | Layer 1 (queue behavior) | None |
| 5. Non-modal composer permission flow | Layer 1 (permission behavior) | None |
| 6. Multi-pending Up/Down | Layer 1 (permission navigation) | None |
| 7. Mid-stream pause/resume | Layer 1 (mid-stream permission) | Verification approach underspecified; needs fake host injection |
| 8. Paste inline + file compact preview | Layer 1 (token behavior) + Layer 2 (preview routing) | Test split across layers not explicitly mapped |

**Recommendation:** Before Phase 3, add a fake-host capability matrix to `WU-102` that lists which host behaviors the test fake must support: turn submission, stream deltas, stream completion, permission injection mid-stream, preview responses, and host status updates.

---

## Verdict

The designs are **conditionally ready for Phase 3** after disposition of the blocking finding (#1) and the four significant behavior-preservation gaps (#2 mid-stream pause, #3 assistant row timing, #4 pending submissions, #5 status event structure). Findings #6 and #7 (extraction ordering and theme boundary) are significant but can be addressed during implementation if the behavior contract is first tightened. Advisory findings should be dispositioned at the implementer's discretion.

---

## Disposition

Processed 2026-04-26 by the agent team during Phase 2 closure. Severity is
restated against the project review schema; see `docs/adr/.reviews/README.md`.

| Finding | Severity | Disposition | Rationale |
| --- | --- | --- | --- |
| 1 | blocking | accepted | Removed WU-102 from WU-100's "Parallelizes With" in plan.md and track-a. Duplicate of Codex #1. |
| 2 | significant | accepted | Adopted adapter-level stream buffering during pending permissions (matches the spike's `pauseStreamingForPermission`). Documented in WU-099 Permission Integration Points and a new pause/resume responsibility for the adapter. |
| 3 | significant | accepted | WU-098 now states explicitly that the shell appends the assistant transcript row optimistically when emitting `SubmitTurnAction`; `RunStartedEvent` is for run-ID correlation and host-side failure signaling only. |
| 4 | significant | accepted | Added `pendingSubmissions` to WU-098's shell-owned state inventory and queue invariants; WU-100 Step 5 now references it explicitly. |
| 5 | significant | accepted | Replaced single-string `HostStatusEvent` with `Status string + Kind StatusKind` and added a `StatusKind` enum (ready, streaming, interrupt_armed, permission_pending, error). |
| 6 | significant | accepted | Added a Stage A→B bridge note to WU-100 covering type translation helpers; rendering cutover now explicitly depends on the new state types from Stage A. |
| 7 | significant | accepted | WU-098 and WU-100 now state explicitly that `internal/harnessshell` must not import `internal/harness/theme` or any modeltap-specific style constants; the package owns its own neutral styles. |
| 8 | significant | accepted | WU-099 specifies that the adapter constructs `PermissionResolvedEvent.Message` from the runtime tool result payload, with a generic granted/denied fallback for empty/structured payloads. WU-098 notes `Message` is host-synthesized. |
| 9 | significant | accepted | FEAT-0014 `/perm` and `/demo` references in success criteria 5 and 7 replaced with host-agnostic descriptions. CLI/UI integration list updated. |
| 10 | significant | accepted | WU-100 and WU-102 now require that spike `app_test.go` becomes a migration checklist; new shell-package tests must pass before any old test is deleted. No type aliases — accept that during cutover the spike test file will not compile. |
| 11 | significant | accepted | WU-098 Command Boundary now clarifies: shell owns the trigger condition (empty Enter while idle); the effect crosses the boundary as `SubmitTurnAction{Source = queue_release}`. |
| 12 | advisory | accepted | Added `PermissionDecision` string-typed enum with `DecisionApproveOnce`, `DecisionApproveSession`, `DecisionDeny` constants in WU-098. |
| 13 | advisory | deferred | Concrete Bubble Tea action message shape (e.g., `ActionMsg`) intentionally left to WU-100 implementation, as the design itself notes is acceptable. WU-098 marked the deferral explicitly. |
| 14 | advisory | accepted | Standardized on `runtime_events.go` in both WU-099 and WU-100. |
| 15 | advisory | accepted | FEAT-0014 API Expectations now reads "typed action and event messages." |
| 16 | advisory | accepted | PATCH-0015 "## Checklist" relabeled "## Implementation Checklist" with a note that it tracks Phase 3 work. |
| 17 | advisory | accepted | WU-102 now requires the test fake host to support injecting `PermissionRequestedEvent` mid-stream and lists a fake-host capability matrix. |
| 18 | advisory | accepted | WU-098 adds a design note that tests needing custom host events must live in `package harnessshell` (not the external test package) or use exported concrete event types. |
| 19 | advisory | accepted | Same fix as Codex #5 — WU-100 Step 2 conditional replaced with definitive scope rule. |
| 20 | advisory | accepted | WU-101 now names explicit doc paths: package READMEs at `internal/harnessshell/README.md` and `internal/harnesshost/README.md`; embedding guide at `docs/guides/harness-shell-embedding.md`. |
