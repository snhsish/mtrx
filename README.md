# mtrx

Local-first, open-source telemetry and analytics for AI coding agents.

mtrx collects usage data from the AI agents you already run (Claude Code, Codex,
OpenCode, Cursor, and more), stores it in a local database, and serves a dashboard
to explore token usage, cost estimates, and model breakdowns. Nothing leaves your
machine. No accounts, no cloud, no uploaded session data.

```
go build -o mtrx ./cmd/mtrx
./mtrx              # starts the server and opens http://localhost:6767
```

## Why

Most agent dashboards are hosted services that require an account and upload your
session data to a third party. mtrx is the opposite: it runs entirely on your
machine, reads the transcripts your agents already write to disk, and gives you
the same visibility locally and privately.

## Features

- Multi-agent collection: one process tails each agent's local data and normalizes
  it into a single event stream.
- Web dashboard: filter by agent and model, view cumulative token usage, cost
  breakdown, and per-model tables sortable by price or tokens.
- CLI-first: every metric is also available from the command line.
- Local SQLite storage: no external services, no network calls.
- Cross-platform: Linux, macOS, and Windows.

## Supported agents

| Agent        | Status                                                        |
| ------------ | ------------------------------------------------------------- |
| OpenCode     | Full local telemetry (tokens, models, sessions)              |
| Codex        | Full local telemetry (sessions + state_5.sqlite)              |
| Claude Code  | Transcripts under ~/.claude/projects                          |
| Cursor       | Sessions only. Cursor does not store token or cost data locally (see Limitations) |
| aider        | Detected, collector planned                                  |
| gemini       | Detected, collector planned                                  |

Collectors run in the background every few seconds after the server starts. There
is nothing to configure per agent.

## Install

From source:

```
git clone https://github.com/snhsish/mtrx
cd mtrx
go build -o mtrx ./cmd/mtrx
```

With go install:

```
go install mtrx/cmd/mtrx@latest
```

Prebuilt binaries for Linux, macOS, and Windows are on the releases page.

## Quick start

```
mtrx                 # starts the daemon and opens the dashboard
mtrx start --foreground
```

Open http://localhost:6767. Stop with `mtrx stop`.

## CLI

```
mtrx [start]                 # start the dashboard (daemonized by default)
mtrx start --foreground      # run in the current terminal
mtrx stop                    # stop a running daemon
mtrx status                  # show whether the server is running
mtrx open                    # open the dashboard in your browser
mtrx agents list             # list all known agents
mtrx agents detect           # list agents detected on this machine
mtrx sessions                # list collected sessions
mtrx events [flags]          # query raw events
    --agent <name>           #   filter by harness/agent
    --session <id>           #   filter by session
    --type <type>            #   filter by event type
    --json                   #   output as JSON
mtrx import <path>           # import events from a JSONL/JSON file
mtrx export [path]           # export events
    --format jsonl|json|csv
mtrx database stats          # database size, row counts, free disk
mtrx database vacuum         # compact the SQLite database
mtrx database backup <path>  # copy the database to <path>
mtrx database verify         # integrity-check the database
mtrx doctor                  # diagnose config, paths, and agent detection
mtrx version                 # print version and platform
```

## Configuration

mtrx works with zero configuration. An optional config file is read from
`$XDG_CONFIG_HOME/mtrx/config.yaml` or `~/.config/mtrx/config.yaml`.

```
server:
  host: localhost   # bind address (default: localhost, not exposed to the network)
  port: 6767        # dashboard port
storage:
  path: ~/.local/share/mtrx   # where mtrx.db and mtrx.pid live
collectors:
  enabled: true     # set false to stop background collection
retention:
  enabled: false    # set true to auto-prune old events
  days: 90          # keep this many days when retention is enabled
```

Data locations:

| Path                              | Purpose                   |
| --------------------------------- | ------------------------- |
| ~/.local/share/mtrx/mtrx.db       | SQLite telemetry database |
| ~/.local/share/mtrx/mtrx.pid      | PID file for the daemon   |
| ~/.local/state/mtrx/mtrx.log      | Server log                |
| ~/.config/mtrx/config.yaml        | Optional config           |

On Windows these resolve under your user profile, e.g.
`%USERPROFILE%\.local\share\mtrx\mtrx.db`.

## Architecture

```
agent stores -> adapters -> normalized events -> SQLite -> analytics -> HTTP API -> dashboard
(~/.claude,    (per-agent               (local)               (query)    (JSON)     (web UI)
 ~/.codex,      readers)
 ~/.opencode)
```

- Adapters (`internal/agents/*`) read one agent's local format and emit normalized
  `events.Event` records via `ID()`, `Name()`, `Detect()`, `Sources()`, `Collect()`,
  and `Watch()`.
- Analytics (`internal/analytics`) answers queries against the database.
- Server (`internal/server`) exposes a JSON API and serves the embedded dashboard.
- Database (`internal/database`) is a thin SQLite layer (pure-Go driver, no CGO).

## Privacy

mtrx never makes outbound network connections except to serve the local dashboard on
localhost. Your agent transcripts are read from disk and stored only in the local
SQLite file. No analytics, no phoning home, no account.

## Limitations

- Cursor token and cost data is unavailable by design. Cursor stores conversation
  bubbles locally with token counts hardcoded to zero and no cost field. That data is
  tracked server-side by Anysphere, not written to disk. mtrx can report Cursor
  session counts and model names where present, but not token or cost metrics.
- Cost is estimated, not billed. None of the agents persist actual cost, so mtrx
  shows token counts only unless you supply a pricing table.
- aider and gemini adapters are registered for detection but their collectors are not
  implemented yet.

## Development

```
make build     # build ./mtrx
make test      # run tests
make lint      # go vet + gofmt check
make cross     # build linux/darwin/windows binaries into dist/
```

Requires Go 1.26+.

## License

Apache License 2.0. See LICENSE.
