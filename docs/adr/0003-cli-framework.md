---
status: accepted
date: 2026-03-03
decision-makers: Jason Henderson
---

# ADR-0003: CLI Framework and Approach

## Context and Problem Statement

Modeltap is a command-line tool with multiple subcommands — starting the proxy, querying logs, exporting data, and managing configuration. The approach to building the CLI determines the user experience (help output, shell completions, flag ergonomics), the contributor experience (how easy it is to add new commands), and the long-term maintenance burden. We need to decide whether to use a framework and, if so, which one.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – Subcommand support (5):** Modeltap needs distinct commands (`start`, `logs`, `show`, `export`, `config`). The framework must handle subcommand routing, per-command flags, and help generation cleanly.
* **D2 – Flag and argument parsing (5):** Each command has distinct flags (`--port`, `--capture-mode`, `--since`, `--format`). Parsing must support types, defaults, validation, and both long/short forms.
* **D3 – Minimal dependency footprint (4):** Fewer transitive dependencies mean fewer supply chain risks, smaller binary, and less upgrade churn. Important for an open source tool people embed in their workflow.
* **D4 – Contributor familiarity (4):** Contributors should be able to add a new command or flag without studying framework internals. A well-known framework reduces the onboarding curve.
* **D5 – Help and documentation generation (3):** Auto-generated `--help` output, usage strings, and man page support reduce maintenance burden and improve UX.
* **D6 – Shell completion support (3):** Tab completion for commands and flags improves power-user experience. Matters for a tool that developers use frequently.
* **D7 – Testability (3):** Commands should be unit-testable without spawning processes. The framework should support programmatic invocation and output capture.
* **D8 – Binary size impact (2):** The framework adds to the compiled binary. Matters marginally — a few MB difference is noticeable but not decisive.

## Considered Options

* Cobra
* urfave/cli (v2)
* Kong
* stdlib only (`flag` package)

## Decision Outcome

Chosen option: **Cobra**, because it achieves the highest weighted score (125) with a clear margin over all alternatives. Cobra dominates on every high-weight driver — subcommand support, flag parsing, contributor familiarity — and provides best-in-class shell completions and help generation. Its main weakness is dependency footprint (D3: 2), which is a real but manageable tradeoff accepted by kubectl, GitHub CLI, Docker CLI, and Hugo.

### Scoring Matrix

Scale: 1 (poor) → 5 (excellent). Weighted total = sum of (weight × score).

| Driver                              | Weight | Cobra | urfave/cli | Kong | stdlib only |
|-------------------------------------|--------|-------|------------|------|-------------|
| D1: Subcommand support              | 5      | 5     | 4          | 4    | 2           |
| D2: Flag and argument parsing       | 5      | 5     | 4          | 5    | 2           |
| D3: Minimal dependency footprint    | 4      | 2     | 3          | 4    | 5           |
| D4: Contributor familiarity         | 4      | 5     | 3          | 2    | 4           |
| D5: Help and documentation generation | 3    | 5     | 4          | 4    | 1           |
| D6: Shell completion support        | 3      | 5     | 3          | 3    | 1           |
| D7: Testability                     | 3      | 4     | 3          | 5    | 3           |
| D8: Binary size impact              | 2      | 2     | 3          | 4    | 5           |
| **Weighted Total**                  |        | **125** | **105** | **111** | **82**   |

### Scoring Justification

#### Cobra (125)

* **D1 (5):** Best-in-class subcommand support. Nested subcommands, command groups, aliases, and deprecation notices. Commands like `modeltap logs --since 1h` and `modeltap config set capture-mode full` are trivial to wire up.
* **D2 (5):** Built on `pflag` — POSIX-compliant flags with short/long forms, typed values, defaults, required flags, and mutually exclusive flag groups. Mature and battle-tested.
* **D3 (2):** Heaviest option. Pulls in `pflag`, `viper` (optional but commonly used), and their transitive dependency trees. Adds roughly 2–3 MB to the binary and a meaningful dependency surface.
* **D4 (5):** De facto standard for Go CLIs. Contributors to Go open source projects almost certainly know Cobra. kubectl, `gh`, Docker, and Hugo all use it — it is the framework people reach for first.
* **D5 (5):** Auto-generates help text, usage strings, man pages, and Markdown documentation. Customizable templates. Best-in-class for a CLI tool that needs polished help output.
* **D6 (5):** Built-in generation for Bash, Zsh, Fish, and PowerShell completions. One function call per shell. No other Go CLI library matches this breadth.
* **D7 (4):** Commands can be executed programmatically via `cmd.Execute()`. Output can be captured by redirecting stdout. Slightly awkward to test in isolation because commands are configured via init functions by convention, but workable with careful structuring.
* **D8 (2):** Largest binary size impact due to dependency tree. Adds roughly 2–3 MB compared to stdlib-only.

#### urfave/cli v2 (105)

* **D1 (4):** Solid subcommand support with `cli.Command` structs. Slightly less ergonomic than Cobra for deeply nested commands or command groups, but handles modeltap's flat command structure fine.
* **D2 (4):** Good flag parsing with typed values and defaults. Lacks some of Cobra's advanced features (mutually exclusive groups, flag completion functions). Adequate for modeltap's needs.
* **D3 (3):** Lighter than Cobra but still brings in dependencies. Smaller transitive tree.
* **D4 (3):** Well-known but noticeably less popular than Cobra for new projects. Contributors may need to consult documentation rather than working from memory.
* **D5 (4):** Auto-generated help is clean. Supports Markdown documentation generation via community plugins. No built-in man page generation.
* **D6 (3):** Shell completion exists but is less polished than Cobra's. Bash and Zsh supported; Fish and PowerShell require more manual work.
* **D7 (3):** Testable but less ergonomic — requires constructing a full `cli.App` and invoking `Run()` with `os.Args`-style slices.
* **D8 (3):** Smaller binary impact than Cobra, larger than Kong or stdlib.

#### Kong (111)

* **D1 (4):** Subcommands defined as struct fields — clean and declarative. Handles modeltap's command structure well. Less ecosystem support for advanced patterns (command groups, deprecation).
* **D2 (5):** Struct tags drive everything — types, defaults, required, enum values, environment variable binding. Very expressive with minimal boilerplate. Arguably the cleanest flag definition syntax.
* **D3 (4):** Small dependency tree. Lighter than both Cobra and urfave/cli.
* **D4 (2):** Smallest community of the three libraries. Contributors will likely not have used it before. The struct-tag-driven approach is intuitive once learned but unfamiliar initially.
* **D5 (4):** Auto-generated help is clean and customizable. No built-in man page generation.
* **D6 (3):** Basic shell completion support. Less mature than Cobra's.
* **D7 (5):** Most testable option — commands are plain Go structs. You can construct them directly and call `Run()` methods without any framework ceremony. Excellent for unit testing.
* **D8 (4):** Small binary footprint. Minimal dependency overhead.

#### stdlib only (82)

* **D1 (2):** No built-in subcommand routing. Requires a manual `switch` statement on `os.Args[1]`. Works but is boilerplate-heavy and error-prone as commands grow.
* **D2 (2):** The `flag` package handles basic typed flags but no short forms (`-p` vs `--port`), no flag groups, no validation beyond type parsing. Each subcommand needs its own `flag.FlagSet`.
* **D3 (5):** Zero dependencies. Nothing to audit, nothing to upgrade, nothing to break.
* **D4 (4):** Every Go developer knows the `flag` package. But the manual subcommand routing code is custom per-project — contributors need to understand the project's pattern, not a standard framework.
* **D5 (1):** Minimal auto-generated help. `flag.Usage` produces a flat list of flags with no structure, no subcommand help, no usage examples. Everything must be hand-written.
* **D6 (1):** No shell completion support. Entirely DIY.
* **D7 (3):** Testable in the same way any Go function is, but no framework help for capturing output or simulating CLI invocations.
* **D8 (5):** Zero overhead. Smallest possible binary.

### Consequences

* Good, because Cobra's subcommand model maps directly to modeltap's command structure (`start`, `logs`, `show`, `export`, `config`) with minimal wiring code.
* Good, because built-in shell completion generation for Bash, Zsh, Fish, and PowerShell gives users a polished experience out of the box.
* Good, because Cobra is the de facto standard — contributors can add commands and flags without learning a new framework.
* Good, because auto-generated help output and man pages reduce documentation maintenance.
* Neutral, because Cobra's `viper` integration is available but unused — configuration is handled with stdlib + YAML (see ADR-0004).
* Bad, because Cobra's dependency tree is the largest of the four options, increasing supply chain surface area and binary size by roughly 2–3 MB.
* Bad, because Cobra's conventional use of `init()` functions and package-level variables can make command testing slightly awkward, requiring deliberate structuring to keep commands testable.

### Confirmation

The decision will be confirmed by successfully implementing the core command structure (`start`, `logs`, `show`, `export`, `config`) with Cobra, verifying that help output, flag parsing, and shell completion generation work as expected. If Cobra's dependency footprint becomes a concern (e.g., due to security vulnerabilities in transitive dependencies), Kong would be the first alternative to evaluate given its second-place finish.

## More Information

The decision aligns with the weighted scoring matrix. No override was necessary — Cobra leads on weighted total with a 14-point margin over Kong and a 20-point margin over urfave/cli. The dependency footprint concern (D3: 2) is real but is outweighed by Cobra's strengths on higher-weight drivers.

Note: Cobra's `viper` dependency is not used. Configuration is handled with stdlib + YAML (see ADR-0004).
