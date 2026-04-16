# Integration Track

**Release:** v0.2.0
**WU Range:** WU-088 through WU-090 (3 work units)
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
- `docs/releases/v0.2.0/changelog.md` — what shipped

**Done:** Usage guide covers all new features. Config documented. Help accurate. Changelog written.
