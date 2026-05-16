# v0.1.0 Changelog

**Status:** Shipped (2026-03-08)
**Work Units:** WU-001 through WU-038 (38 total)
**Features:** FEAT-0003 (Web Dashboard), FEAT-0004 (Service Management)

## What Shipped

### Proxy Core
- Reverse proxy with SSE stream capture (WU-013, WU-014, WU-015)
- Multi-provider routing with automatic detection (WU-016)
- Full request/response capture with retention-based pruning (WU-007, WU-008)
- Provider adapters: Anthropic and OpenAI (WU-010, WU-011, WU-012)
- Cost estimation with pricing table (WU-019)
- Metrics aggregation tables — hourly and daily (WU-017)

### CLI
- Cobra command structure: start, logs, show, export, config, status, metrics (WU-005, WU-006, WU-009, WU-018, WU-020, WU-021, WU-022)
- Shell completions for bash, zsh, fish, powershell (WU-032)
- CLI help system (WU-033)

### Web Dashboard (FEAT-0003)
- REST API endpoints: /api/logs, /api/metrics, /api/status (WU-025)
- Log viewer with filtering and pagination (WU-026)
- Metrics display with provider/model breakdown (WU-027)
- Status page (WU-028)
- CLI integration and config (WU-029)
- Dashboard help page (WU-034)

### Service Management (FEAT-0004)
- Platform-native service templates: launchd (macOS), systemd (Linux) (WU-035)
- Service install/uninstall commands (WU-036)
- Service status and logs commands (WU-037)

### Infrastructure
- Go module and project scaffolding (WU-001)
- Build system with Makefile and goreleaser (WU-002)
- GitHub Actions CI pipeline (WU-003)
- Open source files: Apache 2.0 license, CONTRIBUTING.md, GOVERNANCE.md (WU-004)

### Quality
- End-to-end integration tests (WU-023)
- Security review: proxy (WU-024) and dashboard (WU-030)
- User documentation and usage guide (WU-031, WU-038)

## Architecture Decisions (ADR-0001 through ADR-0012)

| ADR | Decision |
|-----|----------|
| 0001 | Go as primary language |
| 0002 | SQLite via modernc.org/sqlite, WAL mode |
| 0003 | Cobra for CLI |
| 0004 | Viper, minimal usage, non-global instances |
| 0005 | Always full capture, retention-based pruning |
| 0006 | Provider adapter interface, Anthropic + OpenAI |
| 0007 | Pre-computed aggregation tables |
| 0008 | sqlite-vec for knowledge layer (optional) |
| 0009 | MCP stdio transport for knowledge access |
| 0010 | Apache 2.0 license |
| 0011 | BDFL with contributor tiers |
| 0012 | Platform-native service managers |

## Session Logs

Work unit session logs are in `.sdlc/history/2026-03-06-*` through `.sdlc/history/2026-03-08-*`.
