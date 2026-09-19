# mtrx

[![CI](https://github.com/snhsish/mtrx/actions/workflows/ci.yml/badge.svg)](https://github.com/snhsish/mtrx/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/snhsish/mtrx)](https://github.com/snhsish/mtrx/releases)
[![License](https://img.shields.io/github/license/snhsish/mtrx)](LICENSE)

Local-first, open-source telemetry and analytics for AI coding agents.

mtrx collects usage data from the AI agents you already run (Claude Code, Codex,
OpenCode, Cursor, and more), stores it in a local database, and serves a dashboard
to explore token usage, cost estimates, and model breakdowns. Nothing leaves your
machine. No accounts, no cloud, no uploaded session data.

<img width="1144" height="979" alt="mtrx dashboard preview" src="https://github.com/user-attachments/assets/a468e95a-aede-45cd-aaf3-8c3616b14e65" />

## Contents

- [Install](#install)
- [Why](#why)
- [Features](#features)
- [Supported agents](#supported-agents)
- [Quick start](#quick-start)
- [CLI](#cli)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Privacy](#privacy)
- [Limitations](#limitations)
- [Development](#development)
- [Building from source](#building-from-source)
- [Releases](#releases)
- [License](#license)

## Install

No Go required. Download a prebuilt binary from the
[releases page](https://github.com/snhsish/mtrx/releases) or install with:

Linux / macOS:

```
curl -fsSL https://raw.githubusercontent.com/snhsish/mtrx/main/scripts/install.sh | sh
```

Windows (PowerShell):

```
irm https://raw.githubusercontent.com/snhsish/mtrx/main/scripts/install.ps1 | iex
```

The scripts detect your OS and CPU type and put `mtrx` in
`/usr/local/bin` (or `~/.local/bin` if that is not writable) on Unix, and in
`%LocalAppData%\mtrx\bin` on Windows. If that folder is not on your `PATH`,
add it so you can run `mtrx` from anywhere.

Pin a version instead of latest:

```
curl -fsSL https://raw.githubusercontent.com/snhsish/mtrx/main/scripts/install.sh | MTRX_VERSION=vX.Y.Z sh
```

```
$env:MTRX_VERSION = "vX.Y.Z"; irm https://raw.githubusercontent.com/snhsish/mtrx/main/scripts/install.ps1 | iex
```

Manual download: each release has `mtrx-<os>-<arch>.tar.gz` files for Unix,
`mtrx-windows-amd64.zip` for Windows, and `sha256sums.txt`. Check the files,
then install:

```
sha256sum -c sha256sums.txt
tar -xzf mtrx-linux-amd64.tar.gz
./mtrx-linux-amd64 version
```

Want to build it yourself? See [Building from source](#building-from-source).

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

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, tests, and how to send a
patch.

## Building from source

You need Go 1.26 or newer. No C compiler needed. The SQLite driver is pure
Go, so a plain `go build` works on Linux, macOS, and Windows.

Build a binary in the repo root:

```
git clone https://github.com/snhsish/mtrx
cd mtrx
go build -o mtrx ./cmd/mtrx
./mtrx version
```

Or install it to your Go bin dir (keep `$(go env GOPATH)/bin` on your
`PATH`):

```
go install github.com/snhsish/mtrx/cmd/mtrx@latest
```

Common make targets:

```
make build     # build ./mtrx
make test      # run all tests
make lint      # go vet
make cross     # build all platforms into dist/ with checksums
```

`make cross` builds Linux and macOS (amd64 and arm64) plus Windows (amd64),
packs them as `tar.gz` and `zip` files, and writes `dist/sha256sums.txt`. The
binary version comes from the git tag. The dashboard is embedded in the
binary, so the single file is all you need to run.

## Releases

Releases are automated: every push to `main` computes the next version from
conventional commits since the last tag (`feat:` → minor, `fix:` → patch,
`!` or `BREAKING CHANGE` → major; first release is `v0.1.0`), builds static
binaries for Linux, macOS (amd64/arm64), and Windows (amd64) via `make cross`,
and publishes them with checksums to the
[releases page](https://github.com/snhsish/mtrx/releases). Pushing a `vX.Y.Z`
tag releases that exact version instead.

## License

Apache License 2.0. See LICENSE.
