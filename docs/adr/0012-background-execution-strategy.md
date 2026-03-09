---
status: accepted
date: 2026-03-08
decision-makers: jasonahenderson
---

# Background Execution Strategy for Proxy Server

## Context and Problem Statement

The modeltap proxy currently runs only in the foreground via `modeltap start`, blocking the terminal until the user sends Ctrl+C. For real-world usage — where the proxy sits in the request path for AI API calls — users need the proxy to run persistently in the background with reliable lifecycle management. A decision is needed on how to provide background execution without reimplementing OS-level process management.

## Decision Drivers

Drivers are weighted 1–5, where 5 = critical.

* **D1 – User experience / setup friction (5):** modeltap targets individual developers proxying their AI API calls. If backgrounding is hard to set up, they won't use it. This is the primary adoption gate.
* **D2 – Reliability / auto-restart (4):** A proxy sitting in the request path for AI API calls must stay up. Silent crashes mean lost requests and broken workflows.
* **D3 – Implementation complexity (4):** Every line of process management code is a maintenance liability. Go's fork semantics are notoriously tricky. Scope creep here trades off against feature work.
* **D4 – Cross-platform support (3):** macOS (launchd) and Linux (systemd) are primary targets. Windows is a stretch goal. The solution should work on at least the two primary platforms without heroics.
* **D5 – Log management (3):** Long-running background processes need log rotation and accessibility. Poor log handling leads to disk exhaustion or invisible errors.
* **D6 – Lifecycle management (3):** Users need a clear way to check if the proxy is running, stop it, and restart it. Ad-hoc PID tracking is error-prone.
* **D7 – Consistency with Go CLI conventions (2):** Go CLIs (kubectl, docker, hugo) typically run foreground and delegate backgrounding to the OS. Deviating from convention surprises users.

## Considered Options

* **Option 1: No change (foreground only)** — Keep `modeltap start` as foreground-only. Document shell techniques (`&`, `nohup`) in the usage guide.
* **Option 2: Built-in daemon mode (`--daemon`)** — Add a `--daemon` flag that forks the process, writes a PID file, and returns. Add `modeltap stop` to kill the PID.
* **Option 3: Service subcommand generator** — Add `modeltap service install/uninstall/status` that generates and manages a platform-native service definition (launchd plist on macOS, systemd unit on Linux). Proxy itself stays foreground.
* **Option 4: Hybrid foreground + simple PID wrapper** — Add a lightweight `modeltap start --background` that spawns a detached child process with PID file and log file redirection, without full daemon semantics.

## Decision Outcome

Chosen option: **Option 3: Service subcommand generator**, because it leverages battle-tested OS service managers for auto-restart, log rotation, and lifecycle management while keeping modeltap's own codebase simple. The one-time setup cost (`modeltap service install`) is justified by the permanent reliability and operational quality it provides.

### Scoring Matrix

Scale: 1 (poor) to 5 (excellent). Weighted total = sum of (weight x score).

| Driver | Weight | 1. Foreground only | 2. Built-in daemon | 3. Service generator | 4. PID wrapper |
|--------|--------|--------------------|--------------------|----------------------|----------------|
| D1: UX / setup friction | 5 | 2 | 4 | 3 | 4 |
| D2: Reliability | 4 | 2 | 2 | 5 | 2 |
| D3: Implementation complexity | 4 | 5 | 1 | 3 | 3 |
| D4: Cross-platform | 3 | 5 | 3 | 3 | 4 |
| D5: Log management | 3 | 2 | 2 | 5 | 3 |
| D6: Lifecycle management | 3 | 1 | 4 | 5 | 3 |
| D7: Go CLI conventions | 2 | 5 | 2 | 4 | 3 |
| **Weighted Total** | | **72** | **65** | **93** | **77** |

### Scoring Justification

#### Option 1: Foreground only (72)

* **D1 (2):** Requires users to know `nohup`, `&`, `disown` — fine for experienced devs, but a gap for anyone expecting `modeltap start` to "just work" in the background.
* **D2 (2):** No auto-restart. If it crashes overnight, it stays down until the user notices.
* **D3 (5):** Zero implementation work. Nothing to build, nothing to maintain.
* **D4 (5):** Shell backgrounding works identically everywhere.
* **D5 (2):** Logs go to terminal, nohup.out, or /dev/null. No rotation, no structured management.
* **D6 (1):** No built-in way to check if the proxy is running or stop it cleanly. Users grep for PIDs.
* **D7 (5):** This is exactly how most Go CLIs work. kubectl, hugo, caddy (without systemd) all run foreground.

#### Option 2: Built-in daemon mode (65)

* **D1 (4):** `modeltap start --daemon` is intuitive. `modeltap stop` is clean. Low friction.
* **D2 (2):** No auto-restart unless we build a supervisor loop, which compounds complexity. PID files go stale on hard crashes.
* **D3 (1):** Go's `os.StartProcess` fork is unreliable for daemonization (goroutine scheduling, file descriptor inheritance). PID file lifecycle is full of edge cases (stale files, race conditions). This is a known tar pit.
* **D4 (3):** Works on macOS/Linux but PID file paths and signal handling differ. Windows has no SIGTERM — needs a completely different approach.
* **D5 (2):** Must implement log file rotation or redirect to a file and hope the user manages it. Easy to get wrong.
* **D6 (4):** `modeltap stop` reading a PID file is straightforward when it works. Stale PID files cause confusion.
* **D7 (2):** Most Go CLIs avoid this pattern deliberately. Docker and Caddy moved away from built-in daemon modes.

#### Option 3: Service generator (93)

* **D1 (3):** Requires a two-step setup (`modeltap service install`, then the service starts automatically). More upfront friction than `--daemon`, but it's a one-time operation and the result is better.
* **D2 (5):** systemd `Restart=on-failure` and launchd `KeepAlive` provide battle-tested auto-restart. This is what service managers exist for.
* **D3 (3):** Template generation is straightforward. Must support two platforms (systemd + launchd), but each is a small template + a few exec calls. No tricky fork/PID logic.
* **D4 (3):** macOS and Linux covered well. Windows would need a separate approach (NSSM or Windows Service wrapper), but could be deferred.
* **D5 (5):** journald (Linux) and syslog/ASL (macOS) handle log rotation, storage, and querying automatically. Zero custom code.
* **D6 (5):** `systemctl status modeltap` / `launchctl list | grep modeltap` or wrapped via `modeltap service status`. Full lifecycle for free.
* **D7 (4):** Caddy uses this exact pattern (`caddy install-service`). It's the modern Go CLI approach for long-running servers.

#### Option 4: PID wrapper (77)

* **D1 (4):** `modeltap start --background` is intuitive and single-step.
* **D2 (2):** No auto-restart. Slightly better than raw nohup (PID is tracked), but a crash still means the proxy is down.
* **D3 (3):** Simpler than full daemon mode — just `os.StartProcess` with detach + PID file + log redirect. Still has stale PID edge cases but avoids signal forwarding complexity.
* **D4 (4):** Spawn-and-detach works on macOS/Linux. Windows would need `CREATE_NEW_PROCESS_GROUP` but is feasible.
* **D5 (3):** Can redirect stdout/stderr to a log file. No rotation, but at least logs are captured somewhere predictable.
* **D6 (3):** PID file enables basic `modeltap stop` and status check. Not as robust as a service manager but functional.
* **D7 (3):** Less conventional than foreground-only or service generator, but not unusual for dev tools.

### Consequences

* Good, because auto-restart and crash recovery are handled by proven OS infrastructure with zero custom code.
* Good, because log management (rotation, querying, storage) is delegated to journald/syslog for free.
* Good, because the proxy codebase stays simple — no fork logic, PID files, or signal forwarding.
* Good, because `modeltap service install` is a one-time operation that produces a permanently reliable setup.
* Neutral, because the foreground `modeltap start` mode is preserved for development and ad-hoc usage.
* Bad, because initial setup requires one extra step compared to a `--daemon` flag.
* Bad, because Windows support requires a separate implementation path (deferred to future work).

### Confirmation

* Verify on both macOS (launchd) and Linux (systemd) that `modeltap service install` creates a working service that auto-restarts on crash.
* Verify `modeltap service uninstall` cleanly removes all artifacts.
* If users frequently report confusion about the two-step setup, consider adding a post-install hint to `modeltap start` output suggesting `modeltap service install` for persistent operation.

## More Information

The decision aligns with the weighted scoring matrix — Option 3 scored highest at 93, with a 16-point margin over the next closest option. This pattern is precedented by Caddy (`caddy install-service`), Prometheus Node Exporter, and other Go server CLIs. The foreground mode remains the default for development, testing, and environments where the user prefers manual process management.
