# 2026-04-16 — Design: Integration Track Bundle (WU-088 + WU-089 + WU-090 + WU-094 + WU-095)

## Scope

- **WU-088** — End-to-end tests: harness → BFF → mock provider (`internal/integration/`)
- **WU-089** — CLI and harness launch integration (`internal/cli/`)
- **WU-090** — Documentation and config schema updates
- **WU-094** — Security review suite
- **WU-095** — Performance benchmarks and budgets

**Out of scope:** All Track A and Track B WUs (must be complete before this track begins).

## Design Decisions

### D1. End-to-end tests (WU-088)

```go
// internal/integration/harness_bff_test.go

// Full stack: real BFF server + real harness client (headless) + mock provider (httptest).
```

The E2E tests validate the complete protocol contract between the real BFF and real harness implementations, with only the upstream provider mocked.

#### D1.1. Test infrastructure

```go
// testStack creates a full BFF + harness stack with a mock provider.
func testStack(t *testing.T) (*bff.Server, *harness.App, *httptest.Server, func())
```

Headless harness: `harness.App` can be driven programmatically via `tea.Cmd` injection without a real terminal. The Bubbletea `tea.Program` runs with `tea.WithOutput(io.Discard)`.

#### D1.2. Test matrix

| Test | Coverage |
|------|----------|
| `TestE2E_ConnectAndRegister` | Harness connects to BFF, registers capabilities |
| `TestE2E_TurnStreaming` | Submit turn → streamed response through full stack |
| `TestE2E_ToolRoundTrip` | tool.call → harness executes → tool.result → BFF resumes |
| `TestE2E_SessionPersistence` | Disconnect, reconnect, resume with context intact |
| `TestE2E_ModelSwitch` | model.switch → format translation verified |
| `TestE2E_Compaction` | Trigger → review plan → apply → context reduced |
| `TestE2E_MultiModel` | Parallel review → progressive completion |
| `TestE2E_CostAccuracy` | Cost within 5% of mock provider's token counts |
| `TestE2E_DiagnosticPropagation` | Provider error → BFF diagnostic → harness rendering |
| `TestE2E_ConnectionRecovery` | Kill BFF → harness reconnects → session.sync |

### D2. CLI and harness launch integration (WU-089)

Updates to `internal/cli/root.go`:

```go
// modeltap (no subcommand) → launch harness
// modeltap serve → start server only
// modeltap --resume <id> → launch harness, resume session
// modeltap --project <path> → launch harness with project context
// modeltap --model <name> → launch harness with model override
```

```go
func rootCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use: "modeltap",
        RunE: func(cmd *cobra.Command, args []string) error {
            // No subcommand → launch harness
            return launchHarness(cmd)
        },
    }
    cmd.Flags().String("resume", "", "Resume session by ID")
    cmd.Flags().String("project", "", "Project root path")
    cmd.Flags().String("model", "", "Initial model override")
    return cmd
}
```

Auto-start: if BFF is not running and profile is "solo", harness auto-starts the server (via ConnectionManager WU-074 auto-start logic).

Existing subcommands (`logs`, `metrics`, `export`, `service`, `config`, `status`, `show`, `completion`, `dashboard`) remain unchanged.

### D3. Documentation and config schema (WU-090)

Deliverables:
- `docs/usage-guide.md` — harness usage, BFF server config, session management, tool descriptions, model config, routing policy, MCP server config
- Config schema documentation for all new config keys:
  - `server.*` (socket, TLS, timeouts)
  - `providers[]` (endpoints)
  - `models[]` (manual overrides)
  - `routing.*` (dot-path policy)
  - `context.*` (pressure thresholds, compact model)
  - `sessions.*` (lock timing, retention)
  - `harness.*` (submit key, paste threshold, permission level)
  - `mcp[]` (MCP server config)
- `modeltap --help` updates for new commands and flags
- `docs/releases/v0.2.0/changelog.md` — what shipped

### D4. Security review suite (WU-094)

Per track-integration.md, this is both a test-writing exercise and a formal review. The design here specifies the test scope; the formal review document is produced during Phase 3 execution.

#### D4.1. Test files

| Package | File | Scope |
|---------|------|-------|
| `internal/harness/tools/` | `bash_security_test.go` | Command injection, env-var injection, LD_PRELOAD attacks |
| `internal/harness/tools/` | `read_security_test.go` | Path traversal (../, symlink escape, NUL bytes, long paths) |
| `internal/harness/tools/` | `write_security_test.go` | Path traversal for Write/Edit |
| `internal/harness/tools/` | `web_security_test.go` | SSRF: blocked schemes/hosts (file://, localhost, RFC1918) |
| `internal/harness/tools/` | `permission_security_test.go` | Permission escalation: autonomous can't bypass absolute deny; accept-edits can't auto-approve bash |
| `internal/storage/` | `sqlite_security_test.go` | SQL injection (parameterized-only lint), session lock bypass, scope leaks |
| `internal/bff/` | `session_security_test.go` | Concurrent session.resume, forged lock_owner, clock-skew grace window |
| `internal/bff/` | `transport_security_test.go` | Malformed NDJSON (fuzz), oversize frame enforcement, sequencing (no handler before ready) |
| `internal/bff/` | `credential_security_test.go` | API keys never in logs/errors/events/diagnostics |

#### D4.2. Specific test cases

```go
// Path traversal tests
func TestSecurity_Read_PathTraversal(t *testing.T) {
    // Attempt: ../../../etc/passwd → rejected
    // Attempt: absolute path outside project root → rejected
    // Attempt: symlink pointing outside project root → rejected
    // Attempt: NUL byte in path → rejected
}

// Command injection tests
func TestSecurity_Bash_Injection(t *testing.T) {
    // Attempt: ; rm -rf / → flagged as dangerous
    // Attempt: $(evil_command) → flagged
    // Attempt: LD_PRELOAD=/evil.so cmd → flagged
}

// SSRF tests
func TestSecurity_WebFetch_SSRF(t *testing.T) {
    // Attempt: file:///etc/passwd → blocked
    // Attempt: http://localhost:8080 → blocked
    // Attempt: http://169.254.169.254 → blocked (AWS metadata)
    // Attempt: http://10.0.0.1 → blocked (RFC1918)
}

// SQL injection lint
func TestSecurity_Storage_ParameterizedOnly(t *testing.T) {
    // Scan all .go files in internal/storage/ for raw SQL string concatenation
    // Assert: every query uses ? placeholders, never fmt.Sprintf for values
}
```

#### D4.3. Deliverables

- Security test files colocated with reviewed code
- `docs/history/<date>-security-feat-0008-0009.md` — formal review document
- `docs/patches/` entries for findings requiring follow-up

### D5. Performance benchmarks (WU-095)

Per track-integration.md, budgets enforced by benchmark tests.

#### D5.1. Benchmark files

```go
// internal/bff/bench_test.go
func BenchmarkTransport_10kFrames(b *testing.B)        // ≥ 50k msg/s
func BenchmarkStreamRelay_TokenDelta(b *testing.B)      // < 1ms p50, < 5ms p99
func BenchmarkCompactPlan_64kTokens(b *testing.B)       // < 250ms

// internal/provider/bench_test.go
func BenchmarkFormatMessages_Anthropic_10Turn(b *testing.B) // < 500µs
func BenchmarkFormatMessages_OpenAI_10Turn(b *testing.B)    // < 500µs

// internal/storage/bench_test.go
func BenchmarkListSessions_10kRows(b *testing.B)            // < 50ms

// internal/harness/bench_test.go
func BenchmarkMarkdownRender_10KB(b *testing.B)              // < 16ms per redraw
func BenchmarkViewportRedraw_1000Lines(b *testing.B)         // < 8ms
func BenchmarkColdStart(b *testing.B)                        // < 150ms

// internal/protocol/bench_test.go
func BenchmarkTokenDelta_MarshalUnmarshal(b *testing.B)      // < 50µs
```

#### D5.2. Budget enforcement

```go
// benchBudget fails the benchmark if the measured value exceeds the budget.
func benchBudget(b *testing.B, measured, budget time.Duration, metric string) {
    if measured > budget {
        b.Fatalf("%s: %v exceeds budget %v", metric, measured, budget)
    }
}
```

#### D5.3. Deliverables

- Benchmark files in each package
- `docs/releases/v0.2.0/perf-budgets.md` — canonical budget table with reference machine spec
- Makefile target: `make bench` running `-benchtime=2s -count=3`, race disabled

## Key Files

| Action | Path | WU |
|--------|------|----|
| NEW | `internal/integration/harness_bff_test.go` | 088 |
| MODIFY | `internal/cli/root.go` | 089 |
| NEW | `docs/usage-guide.md` | 090 |
| NEW | `docs/releases/v0.2.0/changelog.md` | 090 |
| NEW | `internal/harness/tools/*_security_test.go` | 094 |
| NEW | `internal/storage/sqlite_security_test.go` | 094 |
| NEW | `internal/bff/*_security_test.go` | 094 |
| NEW | `internal/bff/bench_test.go` | 095 |
| NEW | `internal/harness/bench_test.go` | 095 |
| NEW | `internal/provider/bench_test.go` | 095 |
| NEW | `internal/storage/bench_test.go` | 095 |
| NEW | `internal/protocol/bench_test.go` | 095 |
| NEW | `docs/releases/v0.2.0/perf-budgets.md` | 095 |
