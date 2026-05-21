---
status: superseded
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0004: Configuration Management Approach

## Status

**Superseded** — this ADR originally chose Viper (minimal, no remote config) for configuration management. After review, the decision was reversed: modeltap will use stdlib + `gopkg.in/yaml.v3` instead.

## Decision

Use Go stdlib (`os.Getenv`) and `gopkg.in/yaml.v3` for configuration. No configuration framework.

Modeltap has a small number of config keys (~6). The precedence logic (CLI flags → environment variables → config file → defaults) is straightforward to implement directly:

1. **Config file:** `~/.config/modeltap/config.yaml`, parsed with `gopkg.in/yaml.v3` into a Go struct.
2. **Environment variables:** `MODELTAP_<KEY>` via `os.Getenv()`, checked explicitly per key.
3. **CLI flags:** Cobra flags (ADR-0003), checked via `cmd.Flags().Changed()`.
4. **Defaults:** Hardcoded in the config struct.

This approach is ~100 lines of code, fully debuggable (every value's source is traceable), and adds only one dependency (`gopkg.in/yaml.v3`).

### Why not Viper

- Viper's implicit precedence merging makes it hard to debug where a config value came from.
- Viper adds significant dependency weight (`mapstructure`, `fsnotify`, `cast`) on top of Cobra's already-heavy tree.
- Viper's global state default (`viper.Get()`) is a footgun that requires discipline to avoid.
- For ~6 config keys, the framework overhead is not justified.

### Configuration file

```yaml
# ~/.config/modeltap/config.yaml
port: 8080
capture_mode: full
db_path: ~/.config/modeltap/modeltap.db
upstream: https://api.anthropic.com
retention_days: 30
```

### Environment variable mapping

```
MODELTAP_PORT=9090
MODELTAP_CAPTURE_MODE=summary
MODELTAP_DB_PATH=/var/lib/modeltap/modeltap.db
MODELTAP_UPSTREAM=https://api.anthropic.com
```
