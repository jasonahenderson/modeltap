# modeltap devops tooling

This directory contains repo-tracked helpers for local development setup. Files
under `.vscode/` are intentionally ignored, so copy or merge the templates here
into your local workspace configuration.

## VS Code tasks

To install the local task set:

```sh
mkdir -p .vscode
cp devops/vscode/tasks.json .vscode/tasks.json
```

If you already have `.vscode/tasks.json`, merge the tasks from
`devops/vscode/tasks.json` instead of overwriting your local file.

The task template provides:

- `Build modeltap` - builds `./.tmp/modeltap` with `make build`.
- `modeltap server` - runs `./.tmp/modeltap start` in a dedicated terminal.
- `modeltap shell` - runs `./.tmp/modeltap shell` in a dedicated terminal.
- `modeltap start` - builds, starts the server in the background, then opens the
  shell in one terminal.
- `Rebuild & Restart` - runs `./rebuild.sh`, rebuilding and restarting detected
  modeltap terminals when possible.
- `modeltap stop all` - stops all local `modeltap` processes.

Run tasks from VS Code with `Tasks: Run Task`.

## Rebuild workflow

The tracked `rebuild.sh` script expects the normal repo build output:

```sh
make build
./.tmp/modeltap start
./.tmp/modeltap shell
```

When `modeltap` is already running, `./rebuild.sh` stops running processes,
rebuilds, and attempts to restart commands in the original terminals.

## Reset local dev data

There is no migration for polluted local development rows from older
PATCH-0041 testing. For a clean local smoke, stop modeltap and delete the
configured SQLite database, then restart.

```sh
# From VS Code, run: Tasks: Run Task -> modeltap stop all

# Fresh installs default to:
rm -f ~/.modeltap/modeltap.db

# Legacy installs may still use:
rm -f ~/.config/modeltap/modeltap.db
```

If your config overrides the database path, delete that file instead:

```sh
rg 'db_path' ~/.modeltap/config.yaml ~/.config/modeltap/config.yaml
```

`MODELTAP_DB_PATH` or a CLI/config override takes precedence over the defaults.
