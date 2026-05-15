# Integration Track

> [!NOTE]
> Historical terminology: this artifact uses the former `BFF` name. The live architecture renamed that component to the `runtime server` in ADR-0016 (`.sdlc/adr/0016-runtime-server-and-client-surfaces.md`); live source now uses `internal/runtime` and the `runtime` config namespace.

**Release:** v0.2.0
**WU Range:** WU-088 through WU-090, WU-094, WU-095 (5 work units)
**Depends on:** Track A (WU-067) and Track B (WU-087) complete

## WU-088: End-to-End — Harness → BFF → Mock Provider

**Size:** Large | **Dependencies:** WU-067, WU-087 | **Parallelizes with:** WU-089

NEW `internal/integration/harness_bff_test.go` — full stack integration tests. Real BFF server + real harness client (headless Bubbletea or direct protocol client) + mock provider (httptest).

Tests:
- Connect harness to BFF, register capabilities
- Submit turn, receive streamed response through full stack
- Tool call round-trip (Read, Edit, Bash at minimum)
- Session persistence: disconnect, reconnect, resume with context intact
- Model switch mid-session: verify format translation works
- Compaction flow: trigger, review plan, apply
- Multi-model branching: parallel review, progressive completion
- Cost tracking: verify accuracy within 5% of mock provider's token counts
- Diagnostic propagation: provider error → BFF diagnostic → harness rendering

**Done:** Full stack tests pass. Protocol contract verified between real implementations. Cost accuracy verified. Session persistence verified. Connection recovery verified.

---

## WU-089: CLI and Harness Launch Integration

**Size:** Medium | **Dependencies:** WU-067, WU-087 | **Parallelizes with:** WU-088

Updates to `internal/cli/root.go`:
- `modeltap` (no subcommand) launches the harness
- `--resume <session-id>` flag
- `--project <path>` flag
- `--model <name>` flag
- Harness auto-starts local server if not running (solo profile)
- `modeltap serve` starts server only (no harness)
- Existing subcommands (`logs`, `metrics`, `export`, `service`, `config`, `status`, `show`, `completion`, `dashboard`) remain unchanged

**Done:** `modeltap` launches harness. Flags work. Auto-start works. Existing subcommands unchanged. Help updated.

---

## WU-090: Documentation and Config Schema Updates

**Size:** Medium | **Dependencies:** WU-088, WU-089

Updates:
- `docs/usage-guide.md` — harness usage, BFF server config, session management, tool descriptions, model config, routing policy, MCP server config
- Config schema documentation for all new config keys (server, providers, models, routing, context, sessions, harness, mcp)
- `modeltap --help` updates for new commands and flags
- `.sdlc/releases/v0.2.0/changelog.md` — what shipped

**Done:** Usage guide covers all new features. Config documented. Help accurate. Changelog written.

---

## WU-094: Security Review Suite

**Size:** Large | **Dependencies:** WU-067, WU-087 | **Parallelizes with:** WU-088, WU-089, WU-095

Formal OWASP-style security review pass per the Security Reviewer role in `docs/agents.md`. This WU is **both** a test-writing exercise and a formal review; deliverables include tests colocated with the code under review and a published review document.

Scope — what must be reviewed and tested:

- **Tool framework (WU-075) and tools (WU-076-079):**
  - Path traversal tests for Read, Write, Edit, Glob (`../` escape, absolute paths outside project root, symlink escape, `NUL` bytes, long paths)
  - Command injection tests for Bash and Git tools (shell metacharacters, env-var injection, `LD_PRELOAD`-style attacks)
  - Dangerous command detection coverage: every pattern claimed by WU-078 asserted with positive and negative cases
  - Permission-enforcer bypass: autonomous mode still honors absolute deny-lists; `accept-edits` cannot escalate to Bash auto-approval
  - WebFetch/WebSearch SSRF: blocked scheme/host list (`file://`, `localhost`, RFC1918) asserted
- **Storage (WU-045, WU-091):**
  - SQL-injection review of every query in `internal/storage/sqlite.go` — only parameterized statements allowed (lint-style test)
  - Session-lock bypass attempts: concurrent `session.resume`, forged `lock_owner`, clock-skew around the 40s grace window
  - `command_history.list` scope-check: `user_id` always applied; project scope cannot leak across users
- **Protocol / transport (WU-046-049):**
  - Malformed NDJSON frames do not crash the server (property/fuzz-style tests)
  - Oversized request caps declared, enforced, and tested
  - TLS trust-config: self-signed rejection in default mode, explicit pin in pinned mode
  - Unix socket file-permission mode: 0600 default, group-writable only when configured
  - Sequencing: no handler executes before `ready` state; `capabilities.register` cannot be replayed to reset tools
- **Credentials and config (WU-057):**
  - API keys never appear in logs, error messages, diagnostic payloads, or protocol events
  - Provider endpoint config files: 0644 (world-readable) warns; 0600 preferred
- **Capture store (ADR-0005, inherited from v0.1):**
  - Captured bodies redact declared secret-bearing headers (`Authorization`, `x-api-key`) on ingress and in any `session.details` / log export paths

Deliverables:
- Security test files colocated with reviewed code (e.g., `internal/harness/tools/bash_security_test.go`, `internal/storage/sqlite_security_test.go`, `internal/bff/session_security_test.go`)
- `.sdlc/history/<date>-security-feat-0008-0009.md` — formal review doc listing each scope item, test references, and pass/fail with severity for any findings
- `.sdlc/patches/` entries for findings that require follow-up beyond in-scope fixes

**Done:** Every scope item has tests asserting the stated property. Review document published. High/critical findings fixed in-WU; medium/low findings ticketed as patches. `go test -race ./...` stays green.

---

## WU-095: Performance Benchmarks and Budgets

**Size:** Medium | **Dependencies:** WU-067, WU-087 | **Parallelizes with:** WU-088, WU-089, WU-094

Satisfies FEAT-0008 ("zero added latency" claim for streaming) and FEAT-0009 (interactive TUI responsiveness) with concrete, measured budgets rather than subjective evaluation.

Implements `internal/bff/bench_test.go`, `internal/harness/bench_test.go`, `internal/protocol/bench_test.go`, `internal/provider/bench_test.go`, and `internal/storage/bench_test.go` using `testing.B`.

Budget table (enforced by tests — bench fails if exceeded):

| Scenario | Benchmark location | Budget |
|----------|--------------------|--------|
| JSON-RPC transport: 10k small frames on one conn | `internal/bff/bench_test.go` | ≥ 50k msg/s |
| Streaming relay: SSE chunk → `token.delta` added latency | `internal/bff/bench_test.go` | < 1ms p50, < 5ms p99 |
| Canonical → provider format (Anthropic, 10-turn) | `internal/provider/bench_test.go` | < 500µs |
| Canonical → provider format (OpenAI, 10-turn) | `internal/provider/bench_test.go` | < 500µs |
| Session store: `ListSessions` with 10k rows, filtered | `internal/storage/bench_test.go` | < 50ms |
| Compaction plan generation: 64k-token history | `internal/bff/bench_test.go` | < 250ms |
| Glamour markdown render: 10KB streaming debounced | `internal/harness/bench_test.go` | < 16ms per redraw |
| Viewport redraw: 1000-line history on keystroke | `internal/harness/bench_test.go` | < 8ms |
| TUI cold start → first frame | `internal/harness/bench_test.go` | < 150ms |
| Protocol round-trip: marshal+unmarshal `token.delta` | `internal/protocol/bench_test.go` | < 50µs |

Deliverables:
- Benches asserted via a thin helper that fails when budget is exceeded; still runnable with `go test -bench` for profiling
- `.sdlc/releases/v0.2.0/perf-budgets.md` — canonical budget table with links to enforcing benchmarks and the reference machine spec
- Makefile target `make bench` running all benches with consistent flags (`-benchtime=2s -count=3`, race disabled)

Reference machine: M-series Mac; spec recorded in perf-budgets.md. CI runners are not the reference; CI may run with looser budgets (documented) to avoid flaky failures from shared-tenant hardware.

**Done:** Every budget has a passing benchmark asserting it on the reference machine. `make bench` green. Any budget that cannot be met is renegotiated in perf-budgets.md with written rationale — not silently dropped.
