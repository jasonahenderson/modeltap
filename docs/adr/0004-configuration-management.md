---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# Configuration Management Approach

## Context and Problem Statement

Modeltap needs a configuration system that supports persistent settings (port, capture mode, database path, upstream URL), environment variable overrides for CI/container environments, and per-invocation CLI flag overrides. The configuration approach must integrate cleanly with Cobra (ADR-0003) and provide a predictable, debuggable precedence model. The choice affects how users configure the tool, how contributors add new config values, and how much dependency weight we add to the binary.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Precedence layering (5):** Modeltap must support a clear, predictable override order: CLI flags → environment variables → config file → defaults. Users set defaults in a config file, override per-environment with env vars, and override per-invocation with CLI flags. Getting this wrong causes confusing behavior.
* **D2 – Environment variable support (5):** Critical for containerized and CI environments where config files do not exist. `MODELTAP_PORT`, `MODELTAP_CAPTURE_MODE`, etc. must work without a config file present.
* **D3 – Config file support (4):** Users need persistent settings (`~/.config/modeltap/config.yaml`). The tool runs as a background proxy — reconfiguring via flags every time is unacceptable.
* **D4 – Cobra integration (4):** ADR-0003 chose Cobra. The config solution must bind cleanly to Cobra flags without glue code or awkward workarounds.
* **D5 – Simplicity and debuggability (3):** When config does not behave as expected, users need to understand where a value came from. Magic merging across sources makes debugging hard.
* **D6 – Dependency footprint (3):** Consistent with ADR-0003's concern about Cobra's dependency tree. Adding another heavy dependency compounds the issue.
* **D7 – Contributor familiarity (2):** Contributors adding a new config value should be able to do so without deep framework knowledge. Less critical than for the CLI framework since config changes are less frequent.

## Considered Options

* Viper
* Viper (minimal, no remote config)
* koanf
* Custom (stdlib + YAML parser)

## Decision Outcome

Chosen option: **Viper (minimal, no remote config)**, because it achieves the highest weighted score (115) and provides seamless Cobra integration that no other option matches. The `BindPFlag` API means every config value is a single line of wiring between Cobra flags and the configuration layer.

The margin over koanf is narrow (3 points). The decisive factor beyond the score is Cobra integration (D4). ADR-0003 committed modeltap to Cobra, and Viper is its purpose-built companion. koanf can achieve the same result but requires an adapter package and more manual wiring — friction that compounds as config keys grow. Given we are already paying Cobra's dependency cost, adding Viper's incremental cost (minus remote config weight) is a smaller marginal price than the ergonomic penalty of using a non-native config library with Cobra.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                            | Weight | Viper | Viper (minimal) | koanf | Custom (stdlib + YAML) |
|-----------------------------------|--------|-------|-----------------|-------|------------------------|
| D1: Precedence layering           | 5      | 5     | 5               | 5     | 3                      |
| D2: Environment variable support  | 5      | 5     | 5               | 5     | 4                      |
| D3: Config file support           | 4      | 5     | 5               | 5     | 4                      |
| D4: Cobra integration             | 4      | 5     | 5               | 3     | 2                      |
| D5: Simplicity and debuggability  | 3      | 2     | 3               | 4     | 5                      |
| D6: Dependency footprint          | 3      | 1     | 2               | 4     | 5                      |
| D7: Contributor familiarity       | 2      | 5     | 5               | 2     | 3                      |
| **Weighted Total**                |        | **109** | **115**       | **112** | **97**               |

### Scoring Justification

#### Viper (109)

* **D1 (5):** Automatic precedence layering is Viper's core feature — flags override env vars override config file override defaults. One-line binding per value. No manual merging logic.
* **D2 (5):** `viper.AutomaticEnv()` with a configurable prefix (`MODELTAP_`) binds all config keys to env vars automatically. Supports custom env key mapping.
* **D3 (5):** Native support for YAML, TOML, JSON, HCL, and INI. File watching for live reload. Handles `~/.config/modeltap/config.yaml` out of the box.
* **D4 (5):** Purpose-built to integrate with Cobra. `viper.BindPFlag()` and `viper.BindPFlags()` wire Cobra flags directly to Viper config keys with one line per flag.
* **D5 (2):** Viper's magic is also its weakness. Values come from multiple sources with implicit merging. `viper.Get("port")` does not tell you whether the value came from a flag, env var, config file, or default. Debugging "why is this value wrong?" requires understanding Viper's internal precedence chain.
* **D6 (1):** Heaviest option. Viper pulls in `fsnotify`, `mapstructure`, `pflag`, `cast`, encoding libraries, and optionally remote config dependencies (Consul, etcd). Significant transitive tree on top of Cobra's already-heavy dependencies.
* **D7 (5):** Most Go developers who have used Cobra have also used Viper. Standard pairing. Adding a new config value is well-documented.

#### Viper minimal (115)

* **D1 (5):** Same automatic precedence layering as full Viper.
* **D2 (5):** Same `AutomaticEnv()` support.
* **D3 (5):** Same config file support.
* **D4 (5):** Same Cobra integration.
* **D5 (3):** Same implicit merging challenge as full Viper, but excluding remote config sources removes one layer of confusion. Slightly more predictable since values can only come from flags, env, file, or defaults.
* **D6 (2):** Dropping remote config (Consul, etcd) removes some transitive dependencies, but Viper's core still pulls in `mapstructure`, `fsnotify`, `cast`, and encoding libraries. Lighter than full Viper but still heavier than koanf or custom.
* **D7 (5):** Same contributor familiarity as full Viper — the API is identical.

#### koanf (112)

* **D1 (5):** Explicit provider-based precedence — you load sources in order and later sources override earlier ones. `koanf.Load(file.Provider(...))` then `koanf.Load(env.Provider(...))` then flag binding. Clear, explicit, predictable.
* **D2 (5):** `env.Provider` with a configurable prefix. Same functional outcome as Viper's `AutomaticEnv()`.
* **D3 (5):** YAML, TOML, JSON support via pluggable parsers. Load from file path, byte slice, or reader. Clean API.
* **D4 (3):** No built-in Cobra integration. Cobra flags can be bound via `posflag.Provider(cmd.Flags())`, but it is a separate adapter package and requires slightly more wiring than Viper's `BindPFlag`. Works but not seamless.
* **D5 (4):** Precedence is explicit in load order — you can see exactly which source overrides which by reading the initialization code. Easier to debug than Viper's implicit merging.
* **D6 (4):** Modular design — you only import the providers you use. No `fsnotify`, no `mapstructure`, no encoding libraries unless you opt in. Significantly lighter than Viper.
* **D7 (2):** Growing but still much less known than Viper. Contributors will likely not have used it. Documentation is good but the ecosystem of examples and blog posts is thinner.

#### Custom / stdlib + YAML (97)

* **D1 (3):** Precedence must be hand-coded. A `loadConfig()` function that reads YAML, checks `os.Getenv()` overrides, then applies flag values. Straightforward but error-prone to maintain as config keys grow. Every new config value needs manual wiring in multiple places.
* **D2 (4):** `os.Getenv("MODELTAP_PORT")` works fine. No automatic binding — each env var must be explicitly read and mapped. Functional but tedious.
* **D3 (4):** `gopkg.in/yaml.v3` handles YAML parsing cleanly. Unmarshal into a Go struct. No file watching, no multi-format support — but modeltap only needs YAML.
* **D4 (2):** No integration — Cobra flags and config struct exist independently. You write manual code to merge them: `if cmd.Flags().Changed("port") { cfg.Port = flagPort }`. This wiring scales poorly.
* **D5 (5):** Maximum transparency. Every config value's source is explicit in the code. No framework magic. If something is wrong, you read the `loadConfig()` function and trace the value.
* **D6 (5):** Only dependency is `gopkg.in/yaml.v3` — a single, stable, widely-used library. Minimal supply chain surface.
* **D7 (3):** No framework to learn, but contributors must understand the project's custom config loading pattern. Adding a new config value means touching multiple places (struct, YAML, env var mapping, flag binding).

### Consequences

* Good, because Viper's `BindPFlag` API provides one-line wiring between every Cobra flag and its config equivalent, keeping the configuration layer DRY.
* Good, because `AutomaticEnv()` with a `MODELTAP_` prefix means every config key is automatically available as an environment variable with no additional code.
* Good, because native YAML config file support handles `~/.config/modeltap/config.yaml` without custom parsing logic.
* Good, because the Cobra + Viper pairing is the most documented and widely-understood configuration stack in the Go CLI ecosystem.
* Neutral, because excluding remote config (Consul, etcd) reduces the dependency surface while still leaving Viper's core dependencies (mapstructure, fsnotify, cast).
* Bad, because Viper's implicit precedence merging makes it harder to debug "where did this config value come from?" compared to koanf's explicit load ordering.
* Bad, because Viper adds meaningful dependency weight on top of Cobra's already-heavy tree, compounding the supply chain surface area concern from ADR-0003.
* Bad, because Viper uses global state by default (`viper.Get()` operates on a package-level instance), though this can be mitigated by using `viper.New()` for an instance-scoped configuration.

### Confirmation

The decision will be confirmed by:

1. Successfully implementing config loading from `~/.config/modeltap/config.yaml` with Viper.
2. Verifying that the precedence chain works correctly: CLI flags override env vars override config file override defaults.
3. Demonstrating that `MODELTAP_PORT=9090 modeltap start` correctly overrides the config file's port setting.
4. Confirming that adding a new config value requires only: (a) adding the flag to the Cobra command, (b) one `viper.BindPFlag()` call, and (c) optionally a default in `config.yaml`.

If Viper's dependency weight becomes a concern (e.g., vulnerability in a transitive dependency), koanf is the first alternative to evaluate given its close second-place score (112) and lighter dependency tree.

## More Information

The decision aligns with the weighted scoring matrix. No override was necessary — Viper (minimal) leads on weighted total, and the qualitative analysis of Cobra integration reinforces the quantitative result.

The configuration file will follow XDG conventions:

```yaml
# ~/.config/modeltap/config.yaml
port: 8080
capture_mode: full
db_path: ~/.config/modeltap/modeltap.db
upstream: https://api.anthropic.com
max_body_size: 10MB
retention_days: 30
```

Environment variable mapping follows the pattern `MODELTAP_<UPPERCASE_KEY>`:

```
MODELTAP_PORT=9090
MODELTAP_CAPTURE_MODE=summary
MODELTAP_DB_PATH=/var/lib/modeltap/modeltap.db
MODELTAP_UPSTREAM=https://api.anthropic.com
```

Note: Viper's global state concern will be mitigated by using `viper.New()` to create an instance-scoped config, avoiding package-level mutable state.
