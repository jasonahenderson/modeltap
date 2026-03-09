# modeltap Usage Guide

## Overview

modeltap is a transparent reverse proxy for AI/ML API clients. It sits between your application and AI provider APIs (such as Anthropic and OpenAI), capturing every request and response that passes through. It stores this data locally in SQLite and provides tools for browsing logs, exporting data, tracking usage metrics (tokens, latency, estimated cost), and viewing everything in a web dashboard.

Key capabilities:

- **Transparent proxying** -- point your AI client at modeltap instead of the provider's API; your API keys and request format remain unchanged.
- **Automatic provider detection** -- modeltap identifies whether traffic is destined for Anthropic or OpenAI based on headers and request paths.
- **Token and cost tracking** -- input/output tokens are extracted from responses, and costs are estimated using configurable per-model pricing.
- **Streaming support** -- SSE streaming responses are captured and reassembled for full token accounting.
- **Web dashboard** -- a built-in UI for browsing logs, viewing metrics, and checking proxy status.

## Installation

### Building from source

Prerequisites: Go 1.22 or later.

```sh
git clone https://github.com/jasonahenderson/modeltap.git
cd modeltap
make build
```

The compiled binary is placed at `./bin/modeltap`.

To verify the build:

```sh
./bin/modeltap --version
```

You can copy the binary to a location on your `$PATH` for convenience:

```sh
cp ./bin/modeltap /usr/local/bin/
```

## Quick Start

1. **Start the proxy** with a default upstream of `https://api.anthropic.com`:

   ```sh
   modeltap start
   ```

   This starts the proxy on port 8080. You can specify a different upstream:

   ```sh
   modeltap start --upstream https://api.openai.com
   ```

2. **Point your AI client** to `http://localhost:8080` instead of the provider's API URL. For example, with the Anthropic Python SDK:

   ```python
   import anthropic
   client = anthropic.Anthropic(base_url="http://localhost:8080")
   ```

   Or with the OpenAI Python SDK:

   ```python
   from openai import OpenAI
   client = OpenAI(base_url="http://localhost:8080/v1")
   ```

3. **View captured logs**:

   ```sh
   modeltap logs
   ```

4. **Check proxy status**:

   ```sh
   modeltap status
   ```

5. **Open the web dashboard** (requires `--dashboard` flag on start):

   ```sh
   modeltap start --dashboard
   modeltap dashboard
   ```

## Configuration

### Config file location

The config file is stored at:

```
~/.config/modeltap/config.yaml
```

You can confirm this with:

```sh
modeltap config path
```

If the file does not exist, modeltap uses built-in defaults. You do not need to create it unless you want to customize settings.

### Available settings

| Key | Default | Description |
|-----|---------|-------------|
| `port` | `8080` | Port the proxy listens on |
| `upstream` | `https://api.anthropic.com` | Default upstream API URL |
| `db_path` | `~/.config/modeltap/modeltap.db` | Path to the SQLite database |
| `retention_days` | `30` | Number of days to retain captured data |
| `max_body_size` | `10MB` | Maximum request/response body size to store |
| `dashboard.enabled` | `false` | Whether the dashboard starts with the proxy |
| `dashboard.port` | `8081` | Port the dashboard listens on |
| `dashboard.bind` | `127.0.0.1` | Address the dashboard binds to |

### Example config file

```yaml
port: 8080
upstream: https://api.anthropic.com
db_path: ~/.config/modeltap/modeltap.db
retention_days: 30
max_body_size: 10MB

dashboard:
  enabled: true
  port: 8081
  bind: 127.0.0.1

providers:
  anthropic:
    upstream: https://api.anthropic.com
  openai:
    upstream: https://api.openai.com
```

### Config commands

```sh
# Show all resolved configuration (defaults + file + env)
modeltap config show

# Set a configuration value (writes to config file)
modeltap config set <key> <value>

# Show the config file path
modeltap config path
```

### Environment variable overrides

Every config key can be overridden with an environment variable using the `MODELTAP_` prefix. Nested keys use underscores in place of dots.

| Config key | Environment variable |
|------------|---------------------|
| `port` | `MODELTAP_PORT` |
| `upstream` | `MODELTAP_UPSTREAM` |
| `db_path` | `MODELTAP_DB_PATH` |
| `retention_days` | `MODELTAP_RETENTION_DAYS` |
| `dashboard.enabled` | `MODELTAP_DASHBOARD_ENABLED` |
| `dashboard.port` | `MODELTAP_DASHBOARD_PORT` |
| `dashboard.bind` | `MODELTAP_DASHBOARD_BIND` |

Precedence order (highest to lowest): CLI flags > environment variables > config file > defaults.

## CLI Commands Reference

### `modeltap start`

Start the reverse proxy server.

```sh
modeltap start [flags]
```

**Flags:**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | `8080` | Port to listen on |
| `--upstream` | `-u` | `https://api.anthropic.com` | Upstream API URL |
| `--dashboard` | | `false` | Enable the web dashboard |
| `--dashboard-port` | | `8081` | Port for the web dashboard |

**Example:**

```sh
modeltap start -p 9090 -u https://api.openai.com --dashboard
```

The proxy runs in the foreground and shuts down gracefully on SIGINT or SIGTERM.

### `modeltap logs`

List captured request/response logs.

```sh
modeltap logs [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | `50` | Maximum number of results to return |
| `--provider` | | Filter by provider name (e.g., `anthropic`, `openai`) |
| `--model` | | Filter by model name (e.g., `claude-sonnet-4-20250514`) |
| `--since` | | Show requests after this time (duration like `24h`, `7d`, or RFC3339 timestamp) |
| `--until` | | Show requests before this time |
| `--status` | | Filter by HTTP response status code |

**Output columns:** ID, TIMESTAMP, PROVIDER, MODEL, STATUS, IN TOKENS, OUT TOKENS, COST, LATENCY

**Examples:**

```sh
# Last 50 requests (default)
modeltap logs

# Last 10 Anthropic requests
modeltap logs --provider anthropic --limit 10

# Requests from the last 24 hours
modeltap logs --since 24h

# Failed requests only
modeltap logs --status 500
```

### `modeltap show <id>`

Show the full detail of a captured request/response pair.

```sh
modeltap show <id>
```

The `<id>` argument is required. You can use the truncated 8-character ID shown in `logs` output or the full ID.

The output includes:
- Request metadata (provider, model, status, latency, tokens, cost)
- Full request (method, URL, headers, body)
- Full response (status, headers, body)

### `modeltap export`

Export captured logs to JSONL or CSV format. Output is written to stdout.

```sh
modeltap export [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `jsonl` | Output format: `jsonl` or `csv` |
| `--since` | | Filter requests after this time |
| `--until` | | Filter requests before this time |

**Examples:**

```sh
# Export all logs as JSONL
modeltap export > logs.jsonl

# Export last 7 days as CSV
modeltap export --format csv --since 7d > logs.csv
```

**JSONL fields:** `id`, `timestamp`, `provider`, `model`, `status`, `input_tokens`, `output_tokens`, `latency_ms`, `cost`

**CSV columns:** `id`, `timestamp`, `provider`, `model`, `status`, `input_tokens`, `output_tokens`, `latency_ms`, `cost`

### `modeltap metrics`

Display aggregated usage metrics for captured API traffic.

```sh
modeltap metrics [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | `30d` | Filter metrics after this time |
| `--until` | | Filter metrics before this time |
| `--group-by` | | Group output by: `provider`, `model`, `day`, or `hour` |
| `--format` | `table` | Output format: `table`, `json`, or `csv` |

**Output columns:** PERIOD, PROVIDER, MODEL, REQUESTS, INPUT TOKENS, OUTPUT TOKENS, COST, AVG LATENCY, ERRORS

**Examples:**

```sh
# Summary for the last 30 days (default)
modeltap metrics

# Daily breakdown
modeltap metrics --group-by day

# Hourly breakdown for the last 24 hours
modeltap metrics --group-by hour --since 24h

# Metrics grouped by provider, as JSON
modeltap metrics --group-by provider --format json
```

#### `modeltap metrics rebuild`

Rebuild the metrics aggregation tables from the raw stored logs. Useful if metrics appear inconsistent.

```sh
modeltap metrics rebuild
```

### `modeltap status`

Show the current proxy and database status.

```sh
modeltap status
```

Displays:
- **Proxy** -- configured port and upstream URL
- **Database** -- path and record count
- **Retention** -- retention period in days
- **Providers** -- registered provider adapters

### `modeltap config`

Manage configuration settings.

```sh
modeltap config <subcommand>
```

**Subcommands:**

| Subcommand | Description |
|------------|-------------|
| `show` | Show the current resolved configuration as YAML |
| `set <key> <value>` | Set a configuration value |
| `path` | Print the config file path |

### `modeltap dashboard`

Open the web dashboard in the default browser.

```sh
modeltap dashboard
```

This command reads the dashboard bind address and port from configuration and attempts to open `http://<bind>:<port>` in your default browser.

Note: The dashboard must be running (started via `modeltap start --dashboard` or by setting `dashboard.enabled: true` in your config file).

### `modeltap completion`

Generate shell completion scripts.

```sh
modeltap completion <shell>
```

**Subcommands:** `bash`, `zsh`, `fish`, `powershell`

**Installation examples:**

```sh
# Bash (Linux)
modeltap completion bash > /etc/bash_completion.d/modeltap

# Bash (macOS with Homebrew)
modeltap completion bash > $(brew --prefix)/etc/bash_completion.d/modeltap

# Zsh
modeltap completion zsh > "${fpath[1]}/_modeltap"

# Fish
modeltap completion fish > ~/.config/fish/completions/modeltap.fish

# PowerShell
modeltap completion powershell > modeltap.ps1
```

To load completions in the current session without installing permanently:

```sh
# Bash
source <(modeltap completion bash)

# Zsh
source <(modeltap completion zsh)

# Fish
modeltap completion fish | source
```

## Multi-Provider Support

modeltap automatically detects which AI provider a request is targeting and applies the correct parser for extracting metadata (model name, token counts, cost).

### Automatic provider detection

**Anthropic** is detected when any of these conditions are true:
- The request host is `api.anthropic.com`
- The request includes an `anthropic-version` header
- The request path contains `/v1/messages` and includes an `x-api-key` header

**OpenAI** is detected when any of these conditions are true:
- The request host is `api.openai.com`
- The request path contains `/v1/chat/completions` (and no `Anthropic-Version` header is present)

### Provider-specific upstream routing

You can configure different upstream URLs for each provider, allowing modeltap to route traffic to the correct API endpoint regardless of which provider your client is targeting:

```yaml
# ~/.config/modeltap/config.yaml
providers:
  anthropic:
    upstream: https://api.anthropic.com
  openai:
    upstream: https://api.openai.com
```

When provider-specific upstreams are configured, modeltap routes detected Anthropic traffic to the Anthropic upstream and detected OpenAI traffic to the OpenAI upstream, even though the client always sends requests to the single modeltap proxy address.

## Web Dashboard

The web dashboard provides a browser-based interface for viewing captured data.

### Enabling the dashboard

The dashboard is disabled by default. Enable it in one of three ways:

1. **CLI flag:** `modeltap start --dashboard`
2. **Config file:** set `dashboard.enabled: true`
3. **Environment variable:** `MODELTAP_DASHBOARD_ENABLED=true`

### Accessing the dashboard

Once the proxy is running with the dashboard enabled, open your browser to:

```
http://127.0.0.1:8081
```

Or use the CLI shortcut to open it automatically:

```sh
modeltap dashboard
```

The bind address and port are configurable via `dashboard.bind` and `dashboard.port`.

### Dashboard pages

- **Log viewer** -- browse and inspect captured request/response pairs
- **Metrics** -- view aggregated usage statistics (tokens, cost, latency) with time-range filtering
- **Status** -- check proxy configuration and database state

## Supported Providers

### Anthropic

- **API:** Messages API (`/v1/messages`)
- **Models:** Claude model family (claude-sonnet-4-20250514, claude-haiku-4-20250414, etc.)
- **Authentication:** `x-api-key` header
- **Streaming:** SSE streams are captured and reassembled (message_start, content_block_delta, message_delta events)

### OpenAI

- **API:** Chat Completions API (`/v1/chat/completions`)
- **Models:** GPT model family (gpt-4o, gpt-4o-mini, etc.)
- **Authentication:** `Authorization: Bearer <key>` header
- **Streaming:** SSE streams are captured and reassembled (chunked data with `[DONE]` terminator)

## Time Flag Format

Several commands accept `--since` and `--until` flags for time-based filtering. These accept two formats:

- **Duration shorthand** -- a number followed by a unit: `s` (seconds), `m` (minutes), `h` (hours), `d` (days). Examples: `24h`, `7d`, `30m`. The value is interpreted as "that much time ago from now."
- **RFC3339 timestamp** -- a full timestamp such as `2026-03-01T00:00:00Z`.
