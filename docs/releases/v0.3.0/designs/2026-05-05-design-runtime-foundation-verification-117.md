# 2026-05-05 - Design: Runtime Foundation Verification and Docs (WU-117)

## Scope

This design covers WU-117:

- tests that prove WU-111 through WU-116 work together
- user documentation for run commands
- developer documentation for run storage/protocol contracts

## Test Layers

### Storage

Package: `internal/storage`

Tests:

- fresh DB migrates to schema version 3
- v2 DB migrates to v3 without losing sessions/turns
- run tables and indexes exist
- idempotent run creation returns or rejects duplicate keys deterministically
- event append is contiguous per run
- checkpoint and lifecycle state update are transactional

### Protocol

Package: `internal/protocol`

Tests:

- JSON fixtures for all `run.*` methods and essential events
- event payloads require `run_id` and `seq`
- `turn.submit` fixture remains backward-compatible
- optional `run_id` in `TurnSubmitResponse` omits cleanly for old clients

### BFF

Package: `internal/bff`

Tests:

- `turn.submit` creates run before dispatch
- provider dispatch failure marks run failed
- normal provider completion marks run completed with usage totals
- `turn.cancel` and `run.cancel` cancel the same active run
- `run.list` scopes by user/project/session
- `run.attach` serializes competing attach requests
- `run.events` returns full replay or summary fidelity
- executor disconnect produces `waiting_user`
- permission request produces `waiting_permission`

### Harness Host and Shell

Packages: `internal/harnesshost`, `internal/harnessshell`

Tests:

- run protocol events project into existing shell host events
- active `/run` renders summary without changing shell-native commands
- `/runs` and `/jobs` render compact run rows
- detached run deltas do not mutate foreground transcript
- attach replay projects selected run transcript
- reconnect resumes from last observed sequence

### Integration

Package: `internal/integration`

Use a test BFF server plus production runtime where practical.

Scenarios:

- foreground chat becomes a completed run
- cancel during stream leaves retained checkpoint
- detach, stream progress, list, reattach
- reconnect with full replay
- reconnect with forced summary-only replay

## Required Regression Assertions

- Every model/harness turn can be represented as a run.
- `workflow_type` persists with default `implementation`.
- `waiting_permission` and `waiting_user` are not collapsed.
- Detached transcripts do not merge into foreground transcript.
- Cost/token/model metadata is present on terminal run summaries.
- Checkpoint records include reserved extension keys for downstream releases.

## Documentation

Update:

- `docs/usage-guide.md`: user-facing `/run`, `/runs`/`/jobs`, attach, detach,
  cancel, retry, continue, fork basics
- `docs/guides/harness-shell-embedding.md`: host projection expectations for
  run events
- `docs/releases/v0.3.0/changelog.md`: final shipped behavior when Phase 3
  completes

Add developer notes in package comments or README snippets only where needed:

- `internal/protocol`: run method/event naming
- `internal/storage`: run schema transaction rules
- `internal/bff`: lifecycle owner and compatibility with `turn.submit`

## Done Criteria

WU-117 is complete when:

1. all tests above pass under `go test ./...`
2. targeted tests cover storage, protocol, BFF lifecycle, harness projection,
   reconnect, and detached transcript behavior
3. docs explain commands and compatibility limits
4. final release status links the implemented tests and docs
