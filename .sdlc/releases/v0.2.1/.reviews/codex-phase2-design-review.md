# Codex Phase 2 Design Review: v0.2.1

Review scope:

- `.sdlc/releases/v0.2.1/plan.md`
- `.sdlc/releases/v0.2.1/track-a-harness-shell-componentization.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-refactor-plan-097.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-extraction-implementation-100.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-docs-embedding-101.md`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-parity-regression-102.md`

Reference contracts:

- `.sdlc/features/0014-harness-conversation-shell.md`
- `.sdlc/patches/0015-harness-shell-component-api.md`

## Findings

### 1. WU-102 is incorrectly listed as parallelizable with WU-100

Severity: significant

Location:

- `.sdlc/releases/v0.2.1/plan.md` — work-unit table
- `.sdlc/releases/v0.2.1/track-a-harness-shell-componentization.md` — WU-100 / WU-102 dependency summary

What's wrong:

WU-100 is marked as parallelizing with WU-102, but WU-102 depends on WU-100 and
explicitly runs after extraction. This can authorize dependency-illegal Phase 3
execution.

Suggested fix:

Remove WU-102 from WU-100's "Parallelizes With"; state WU-102 can only start
after WU-100 exits and only parallelizes with WU-101.

### 2. Host permission resolution drops request identity

Severity: blocking

Location:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md` — Minimal Host Interface

What's wrong:

`Runtime.ResolvePermission(ctx, decision)` drops the `RequestID` emitted by
WU-098's `ResolvePermissionAction`. With multiple pending permissions, the host
cannot safely apply the decision to the intended request, violating FEAT-0014's
multi-pending permission behavior.

Suggested fix:

Change the runtime-facing adapter method to include request identity, for
example `ResolvePermission(ctx, requestID, decision)` or
`ResolvePermission(ctx, PermissionResolution)`.

### 3. WU-098 omits explicit submission accepted/failed host events

Severity: significant

Location:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md` — Host Event Contract

What's wrong:

The required inbound host events omit submission accepted/submission failed,
while WU-100 requires "submission accepted / started" intake. Without an
explicit submit acceptance/failure event, the shell cannot deterministically
handle pending submitted rows, host rejection, or queue-release failure.

Suggested fix:

Add `SubmissionAcceptedEvent` and `SubmissionFailedEvent` keyed by
`SubmissionID`, and define their effects on pending/queued state.

### 4. WU-099 depends on an ambiguous sibling checkout

Severity: significant

Location:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md` — Context

What's wrong:

The host-adapter design depends on a "sibling checkout" at the same path as this
repo. That makes the integration inventory non-reproducible and ambiguous for
Phase 3 implementers.

Suggested fix:

Replace the path reference with a concrete branch/commit/worktree path, or copy
the required integration inventory into the release/design doc so
implementation does not depend on an unstated local checkout.

### 5. WU-100 may pull modeltap app chrome into the reusable shell

Severity: significant

Location:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-extraction-implementation-100.md` — Extraction Sequence / Step 2

What's wrong:

WU-100 allows sidebar, palette, agent overlay, and footer rendering into
`internal/harnessshell` if considered shell-owned, but FEAT-0014 marks broader
session-tree/multi-agent orchestration as non-goals and WU-099 says
status/sidebar/session explorer surfaces stay outside the extracted shell. This
risks pulling modeltap app chrome and internals into the reusable component.

Suggested fix:

Constrain `internal/harnessshell` to the conversation surface plus only generic
shell-local presentation; require sidebar/palette/agent surfaces to stay
top-level or pass through a generic host-fed data boundary.

### 6. WU-102 does not explicitly test streaming-output wrapping

Severity: advisory

Location:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-parity-regression-102.md` — Required Parity Coverage Areas / Transcript and composer surface

What's wrong:

WU-102 maps FEAT-0014 success criterion 2 to generic wrapping, but does not
explicitly require wrapping of streaming assistant output, which FEAT-0014 calls
out directly.

Suggested fix:

Add a required assertion that incremental `RunDeltaEvent` output wraps within
viewport width during active streaming.

### 7. WU-102 misses the exact post-interrupt queue-release success criterion

Severity: advisory

Location:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-parity-regression-102.md` — Required Parity Coverage Areas / Queue behavior

What's wrong:

WU-102 covers "empty Enter releases queued work" and "interrupted runs do not
auto-release," but not the exact FEAT-0014 criterion: queue during stream,
interrupt, then idle empty `Enter` releases the queued work.

Suggested fix:

Add one end-to-end queue parity test for that full sequence.

### 8. Host command action naming is inconsistent across WU-098 and WU-099

Severity: advisory

Location:

- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-shell-component-api-098.md` — Action Contract / `RunCommandAction`
- `.sdlc/releases/v0.2.1/designs/2026-04-25-design-host-adapter-integration-099.md` — Command Routing Split

What's wrong:

WU-098 names the host command action `RunCommandAction`, while WU-099 uses
`RunHostCommandAction` and also references `RunShellCommandAction`. The concept
is clear, but the naming mismatch invites implementation churn.

Suggested fix:

Standardize on one exported host-native action name and remove
`RunShellCommandAction` unless it is a real internal event type.

## Disposition

Processed 2026-04-26 by the agent team during Phase 2 closure. Severity is
restated against the project review schema; see `.sdlc/adr/.reviews/README.md`.

| Finding | Severity | Disposition | Rationale |
| --- | --- | --- | --- |
| 1 | significant | accepted | Removed WU-102 from WU-100's "Parallelizes With" in plan.md and track-a. Same defect as Kimi #1. |
| 2 | blocking | accepted | Changed `Runtime.ResolvePermission` to take `(ctx, requestID, decision)` so request identity survives the boundary. Tracks Kimi #2 mid-stream pause work. |
| 3 | significant | accepted | Added `SubmissionAcceptedEvent` and `SubmissionFailedEvent` to WU-098's host-event contract; WU-099 already enumerated the lifecycle informally. |
| 4 | significant | accepted | Replaced the `/Users/...` sibling-checkout path in WU-099 with a branch-relative reference; the spike branch now contains the harness line. |
| 5 | significant | accepted | Replaced WU-100 Step 2's "only if" conditional with a definitive scope rule (sidebar/palette/agent overlays stay out of `internal/harnessshell`). Tracks Kimi #19. |
| 6 | advisory | accepted | Added an explicit streaming-wrap parity assertion to WU-102. |
| 7 | advisory | accepted | Added the queue-during-stream → interrupt → idle empty Enter end-to-end parity test to WU-102. |
| 8 | advisory | accepted | Standardized on `RunHostCommandAction` in WU-098 and removed the misleading `RunShellCommandAction` heading from WU-099. |
