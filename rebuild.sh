#!/usr/bin/env bash
set -euo pipefail

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$BASE_DIR"

NEW_BIN="${BASE_DIR}/.tmp/modeltap"

normalize_tty() {
    local t="${1:-}"
    [ -z "$t" ] && return
    t=$(printf '%s' "$t" | tr -d ' ')
    if [[ "$t" != /dev/* ]] && [ "$t" != "notatty" ]; then
        t="/dev/$t"
    fi
    printf '%s' "$t"
}

CURRENT_TTY=$(normalize_tty "$(tty 2>/dev/null || echo "")")

# Discover running modeltap processes and their TTYs.
declare -a ttys=()
declare -a cmds=()

pids=$(pgrep -x modeltap 2>/dev/null || true)
for pid in $pids; do
    [ -z "$pid" ] && continue
    kill -0 "$pid" 2>/dev/null || continue

    # Get the process's own TTY first.
    tty=$(normalize_tty "$(ps -p "$pid" -o tty= 2>/dev/null || true)")
    # If the process itself shows no TTY (e.g. double-forked), try its parent.
    if [ -z "$tty" ] || [ "$tty" = "/dev/?" ] || [ "$tty" = "/dev/??" ]; then
        ppid=$(ps -p "$pid" -o ppid= 2>/dev/null | tr -d ' ' || true)
        [ -n "$ppid" ] && tty=$(normalize_tty "$(ps -p "$ppid" -o tty= 2>/dev/null || true)")
    fi
    [ -z "$tty" ] && continue
    [ "$tty" = "/dev/?" ] && continue
    [ "$tty" = "/dev/??" ] && continue

    # Skip the terminal running this rebuild script.
    [ "$tty" = "$CURRENT_TTY" ] && continue

    cmd=$(ps -p "$pid" -o command= 2>/dev/null | sed 's/^[[:space:]]*//' || true)
    [ -z "$cmd" ] && continue

    # Build the replacement command (swap old binary path for new one).
    new_cmd=$(printf '%s' "$cmd" | sed "s|[^[:space:]]*modeltap|${NEW_BIN}|")

    # Deduplicate by TTY.
    found=false
    if [ ${#ttys[@]:-0} -gt 0 ]; then
        for t in "${ttys[@]}"; do
            [ "$t" = "$tty" ] && { found=true; break; }
        done
    fi
    [ "$found" = "true" ] && continue

    ttys+=("$tty")
    cmds+=("$new_cmd")
done

echo "Found ${#ttys[@]} running modeltap terminal(s)"

# Stop them.
if [ -n "$pids" ]; then
    echo "Stopping modeltap..."
    for pid in $pids; do
        [ -z "$pid" ] && continue
        kill -0 "$pid" 2>/dev/null || continue
        kill "$pid" 2>/dev/null || true
    done

    # Wait up to 5s for graceful exit.
    alive=1
    for ((i=0; i<50; i++)); do
        alive=0
        for pid in $pids; do
            [ -z "$pid" ] && continue
            kill -0 "$pid" 2>/dev/null && { alive=1; break; }
        done
        [ "$alive" -eq 0 ] && break
        sleep 0.1
    done

    if [ "$alive" -ne 0 ]; then
        echo "Force-killing..."
        for pid in $pids; do
            [ -z "$pid" ] && continue
            kill -0 "$pid" 2>/dev/null && kill -9 "$pid" 2>/dev/null || true
        done
        sleep 0.3
    fi
fi

# Rebuild.
echo "Rebuilding..."
make build

# Restart in discovered terminals.
if [ ${#ttys[@]} -gt 0 ]; then
    echo "Restarting in original terminal(s)..."
    for i in "${!ttys[@]}"; do
        tty="${ttys[$i]}"
        cmd="${cmds[$i]}"

        # Give the shell prompt a moment to return after the process died.
        sleep 0.5

        if printf '%s\n' "$cmd" > "$tty" 2>/dev/null; then
            echo "  Restarted in $tty"
        else
            echo "  Could not write to $tty. Run manually:" >&2
            echo "    $cmd" >&2
        fi
    done
else
    echo "No existing modeltap terminals found."
    echo ""
    echo "To start fresh, open two VS Code: integrated terminals and run:"
    echo "  cd \"$BASE_DIR\" && ./.tmp/modeltap start"
    echo "  cd \"$BASE_DIR\" && ./.tmp/modeltap shell"
    echo ""
    echo "Or use VS Code: Tasks: Shift+Cmd+P -> Tasks: Run Task -> 'modeltap server' / 'modeltap shell'"
    echo "Or use the single-terminal option: 'modeltap start'"
    echo ""
    echo "Server output (single-terminal mode) is logged to: .tmp/modeltap-server.log"
fi

echo "Done."
