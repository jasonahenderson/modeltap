# v0.3.0 — Status

**Current phase:** Phase 2 — Ready for user review; Phase 3 blocked

**Branch:** `release/v0.3.0`  
**Scope:** Run runtime foundation for FEAT-0016 and the first slice of
FEAT-0017

Phase 1 opened on 2026-05-05 by explicit `ADMIN:` commit and completed after all
WU-108 through WU-117 designs were drafted. Phase 2 opened on 2026-05-05 by
explicit `ADMIN:` commit. Phase 2 design findings recorded so far:

- `docs/releases/v0.3.0/.reviews/codex-design-review.md` — 5 findings (F1–F5)
  accepted and applied to the designs.
- `docs/releases/v0.3.0/.reviews/claude-design-review.md` — 10 net-new findings
  (F6–F15), pending disposition. The largest is the FEAT-0015 observability axis
  (trace IDs, heartbeats, stage deadlines, fsync) which the v0.3.0 designs do
  not cover and the plan does not explicitly defer.

The complete release design index for user review is
`docs/releases/v0.3.0/designs/README.md`.

Because FEAT-0015, FEAT-0016, and FEAT-0017 remain draft specs, the release
remains design-against-draft work only. Phase 3 remains blocked until those
feature contracts, the FEAT-0017 foundation slice, and WU-108 run-runtime ADR
are accepted.

## Work Units

| WU | Title | Size | State | Design |
|---|---|---|---|---|
| 108 | Run runtime ADR | M | designed | [designs/2026-05-05-design-run-runtime-adr-108.md](designs/2026-05-05-design-run-runtime-adr-108.md), [ADR-0015](../../adr/0015-run-runtime.md) |
| 109 | Run schema, storage, and migration design | M | designed | [designs/2026-05-05-design-run-storage-109.md](designs/2026-05-05-design-run-storage-109.md) |
| 110 | Run protocol methods and event taxonomy | M | designed | [designs/2026-05-05-design-run-protocol-110.md](designs/2026-05-05-design-run-protocol-110.md) |
| 111 | BFF run registry and lifecycle store | L | designed | [designs/2026-05-05-design-bff-run-runtime-111-113.md](designs/2026-05-05-design-bff-run-runtime-111-113.md) |
| 112 | `turn.submit` to foreground-run integration | L | designed | [designs/2026-05-05-design-bff-run-runtime-111-113.md](designs/2026-05-05-design-bff-run-runtime-111-113.md) |
| 113 | Pipeline stage/status emission and checkpoint metadata | M | designed | [designs/2026-05-05-design-bff-run-runtime-111-113.md](designs/2026-05-05-design-bff-run-runtime-111-113.md) |
| 114 | Harness run projection and active `/run` surface | M | designed | [designs/2026-05-05-design-harness-run-surface-114-116.md](designs/2026-05-05-design-harness-run-surface-114-116.md) |
| 115 | Run list, attach/detach/cancel/retry/continue/fork commands | L | designed | [designs/2026-05-05-design-harness-run-surface-114-116.md](designs/2026-05-05-design-harness-run-surface-114-116.md) |
| 116 | Reconnect/resume behavior for active and detached runs | M | designed | [designs/2026-05-05-design-harness-run-surface-114-116.md](designs/2026-05-05-design-harness-run-surface-114-116.md) |
| 117 | Runtime foundation tests and docs | M | designed | [designs/2026-05-05-design-runtime-foundation-verification-117.md](designs/2026-05-05-design-runtime-foundation-verification-117.md) |

## Gates

- Phase 1 started by explicit `ADMIN:` release-open commit on 2026-05-05 with a
  design-against-draft exception for FEAT-0015/0016/0017 foundation scope.
- v0.3.0 Phase 1 design may depend on the committed BFF/harness contracts in
  `internal/protocol`, `internal/bff`, `internal/harness`,
  `internal/harnesshost`, `internal/harnessshell`, `internal/cli`, and
  `internal/storage` at the `release/v0.3.0` branch point.
- Phase 1 closes only after every WU has a design artifact.
- Phase 2 design review is recorded and findings are dispositioned.
- Phase 3 begins only after the explicit Phase 2 -> Phase 3 `ADMIN:` commit,
  accepted FEAT-0015/0016/0017 foundation scope, accepted run-runtime ADR, and
  reachable v0.2.x prerequisite surfaces.

## Open Items

- Disposition the 10 findings from `claude-design-review.md` (F6–F15). The four
  significant ones are F6 (observability axis), F7 (`run.create` /
  `queued` reachability), F8 (`run.blocked` event naming), and F9 (fsync /
  N-1 checkpoint compatibility).
- Accept the run-runtime ADR before Phase 3.
- Accept FEAT-0015, FEAT-0016, and the FEAT-0017 foundation slice before Phase
  3.
- Confirm v0.2.0, v0.2.1, and v0.2.2 prerequisite surfaces remain reachable
  from the v0.3.0 implementation branch before Phase 3.
