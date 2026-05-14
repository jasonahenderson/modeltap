[![CI](https://github.com/jasonahenderson/modeltap/actions/workflows/ci.yml/badge.svg)](https://github.com/jasonahenderson/modeltap/actions)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**A local-first integrated environment for AI coding tools.**

```
┌─────────┐      ┌─────────┐      ┌────────────────┐
│  Client │─────>│modeltap │─────>│Anthropic/OpenAI│
│(Claude, │<─────│(capture │<─────│   providers    │
│ Codex)  │      │ + log)  │      └────────────────┘
└─────────┘      └────┬────┘
                     │
                     ▼
               ┌───────────┐
               │  SQLite   │
               └───────────┘
```

> You keep your API keys. You keep your data. You keep the history.

modeltap started as a transparent reverse proxy that captures every request and response between an AI client and providers like Anthropic and OpenAI — giving you a complete, queryable record of your own traffic in local SQLite. It is growing into a broader substrate: a JSON-RPC backend, a terminal harness, a built-in tool framework, and a knowledge layer, all running on your machine and all under Apache 2.0.

---

## Quick start

Prerequisites: Go 1.22 or later.

```sh
git clone https://github.com/jasonahenderson/modeltap.git
cd modeltap
make build
./bin/modeltap --version
```

From here, two paths depending on what you want:

### Run the terminal harness

`modeltap` with no subcommand launches the interactive Bubbletea harness — the v0.2.0 headline. It auto-starts a local runtime server over a unix socket and drops you into a chat UI with slash commands, 13 built-in tools, session history, and MCP support.

```sh
export ANTHROPIC_API_KEY=sk-ant-...   # or OPENAI_API_KEY, or both
./bin/modeltap                         # launch the harness
./bin/modeltap --resume <session-id>   # pick up a prior session
```

Inside the harness: `/help` for slash commands, `/sessions` to browse history, `/models` for the model catalog, `/mcp status` for MCP servers. `Ctrl+C` to exit.

> The harness theme system is ported from [OpenCode](https://github.com/sst/opencode) (MIT) — see [`NOTICE`](NOTICE) for attribution.

### Capture traffic from your own AI tools

Run modeltap as a transparent reverse proxy in front of your existing clients. Every request and response — including reassembled SSE streams — lands in local SQLite with tokens, latency, and cost attached.

```sh
./bin/modeltap start --dashboard
# in another shell, point a client at it:
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_API_KEY=sk-ant-...   # your existing key, unchanged
claude                                # or any Anthropic-compatible client
```

For OpenAI clients, use `OPENAI_BASE_URL=http://localhost:8080/v1` with `OPENAI_API_KEY`.

### Browse captured traffic

```sh
./bin/modeltap logs
./bin/modeltap show <id>
./bin/modeltap metrics --group-by day
open http://127.0.0.1:8081            # dashboard (if --dashboard was passed)
```

Full install, configuration, and client-integration details — including Claude Code, Codex, and OpenCode — are in [`docs/usage-guide.md`](docs/usage-guide.md).

---

## Why modeltap

| Capability | What it means |
|------------|---------------|
| **Local-first** | Everything — captured traffic, session history, metrics — lives on your disk in a single SQLite database. No cloud account, no telemetry, no vendor lock. |
| **Provider-agnostic routing** | One proxy address fronts both Anthropic and OpenAI traffic. Point any client that honors `ANTHROPIC_BASE_URL` or `OPENAI_BASE_URL` at modeltap and it just works. |
| **Real cost and token accounting** | Per-request input/output tokens, latency, and estimated cost — extracted from live responses, including reassembled SSE streams. |
| **Full record, not samples** | Captures every request and response in full, with retention-based pruning ([ADR-0005](docs/adr/0005-retention-based-pruning.md)). You can always go back and read the bytes. |
| **Built to grow** | v0.2.0 adds a JSON-RPC runtime server and a Bubbletea terminal harness with a tool framework and MCP client. A knowledge layer (sqlite-vec), skills, and agent teams are on the roadmap. |

---

## What's in this repo

| Path | Purpose |
|------|---------|
| `cmd/modeltap/` | CLI entrypoint |
| `internal/` | Proxy, storage, providers, harness, runtime server, dashboard |
| `pkg/` | Public packages |
| `docs/adr/` | Architecture Decision Records — accepted ones drive the codebase |
| `docs/features/` | Feature specs — accepted specs drive work units |
| `docs/patches/` | Implementation-scoped patches (fixes, plumbing, small additions) |
| `docs/releases/` | Per-release plan, status, changelog, and track files |
| `docs/history/` | Session logs — continuity across working sessions |
| `docs/usage-guide.md` | User-facing install, config, and command reference |

---

## Status

modeltap is pre-1.0. Interfaces may shift between minor versions.

- **v0.1** — shipped. Reverse proxy, capture, metrics, dashboard, service management, Anthropic + OpenAI adapters.
- **v0.2.0** — in development on branch `exploration/integrated-harness`. Adds a JSON-RPC runtime server, a Bubbletea terminal harness, a 13-tool built-in framework, an MCP client, and an Ollama adapter. See [`docs/releases/v0.2.0/`](docs/releases/v0.2.0/) for plan, status, and changelog.
- **Direction** — enterprise auth and multi-user ([FEAT-0010](docs/features/0010-enterprise-auth.md)), knowledge integration ([FEAT-0011](docs/features/0011-knowledge-integration.md)), skills ([FEAT-0012](docs/features/0012-skills-and-agent-teams.md)), agent teams ([FEAT-0013](docs/features/0013-agent-teams.md)). Proposed, not yet accepted.

---

## Contributing

Contributions are welcome — bug reports, small fixes, whole features. Start with [`CONTRIBUTING.md`](CONTRIBUTING.md) for the workflow and [`GOVERNANCE.md`](GOVERNANCE.md) for how decisions get made.

A few things to know up front:

- **DCO sign-off is required.** Use `git commit -s` on every commit.
- **ADR-driven.** Non-trivial changes that touch architecture reference an ADR; if your change conflicts with an accepted ADR, open an issue before writing code.
- **Patches vs features vs ADRs.** Small implementation-scoped work goes through [`docs/patches/`](docs/patches/README.md). Behavior-scoped new capabilities go through [`docs/features/`](docs/features/README.md). Architectural choices go through [`docs/adr/`](docs/adr/README.md). The README for each directory explains when to use which.
- **Contributor tiers** are graduated — contributor → committer → maintainer → BDFL. See `GOVERNANCE.md`.

## Fork and build on it

modeltap is [Apache-2.0 licensed](LICENSE) (per [ADR-0010](docs/adr/0010-open-source-license.md)). Fork it, remix it, ship something on top of it. If what you're building could be upstream, open a PR — the graduated tier system is there to take new contributors seriously.

The integrated-harness direction in particular is a good substrate to fork against: the runtime server protocol, the tool framework, and the session schema are all reusable pieces if you want your own AI environment with different choices.

## License

[Apache License 2.0](LICENSE).
