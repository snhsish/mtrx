# Contributing to mtrx

Thanks for helping. This guide covers setup, style, tests, and how releases
work.

## Setup

You need Go 1.26 or newer. No C compiler needed.

```
git clone https://github.com/snhsish/mtrx
cd mtrx
go build -o mtrx ./cmd/mtrx
go test ./...
```

Useful commands:

```
make build     # build ./mtrx
make test      # run all tests
make lint      # go vet
make cross     # build all platforms into dist/ with checksums
```

## How the code is laid out

- `agents/<name>/` collectors that read one agent's local files and return
  normalized events. All collectors follow `internal/agents/adapter.go`.
- `internal/collectors/` runs collectors on a timer and writes events to SQLite.
- `internal/database/` opens and migrates the SQLite file.
- `internal/analytics/` answers metric queries against the database.
- `internal/server/` serves the JSON API and the embedded dashboard.
- `cmd/mtrx/` is the CLI. The dashboard files are embedded, so the build
  gives one binary with nothing else to install.

## Commits

Commit messages set the next release version, so use this shape:

```
feat: add Cursor token support
fix: handle empty session files
docs: explain retention config
```

Rules:

- Start with `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, or `chore:`.
- `feat:` bumps the minor version. `fix:` and the rest bump the patch version.
- For a breaking change, add `!` (`feat(api)!: ...`) or a
  `BREAKING CHANGE:` line. That bumps the major version.
- Keep the first line short (under 70 chars). Past commits in
  `git log --oneline` show the style.

Every push to `main` can publish a release, so each commit should build and
pass tests on its own.

## Branches and pull requests

- Make one branch per change, cut from `main` (`feat/...`, `fix/...`,
  `docs/...`, `test/...`).
- Keep the change small. One concern per pull request.
- Push the branch and open a pull request against `main`.
- CI runs `gofmt`, `go vet`, `go build`, and `go test ./...`. All four must
  pass.
- Ask for a review if you are unsure. Small is better than perfect.

## Tests

Tests use only the standard library. Run them with `go test ./...`.

When you change behavior, add or update a test next to the code:

- New collector: add a fixture based test like
  `agents/opencode/opencode_test.go`. Write a small fake transcript file,
  run `Collect`, and check the events (type, tokens, session, skips).
- New query: seed a temp SQLite DB and check the numbers, like
  `internal/analytics/analytics_test.go`.
- New endpoint: use `httptest` against `server.New`, like
  `internal/server/server_test.go`.

Checklist before you push:

```
gofmt -l .        # must print nothing
go vet ./...
go test ./...
```

## Code style

- Run `gofmt` on every file you touch.
- No new third party deps without a good reason. Say why in the pull request.
- Follow the patterns already in the file you edit (naming, error strings,
  struct shapes).
- Keep functions small. Return errors with context (`fmt.Errorf("...: %w", err)`).
- Never log or store secrets. Agent files can hold credentials, so collectors
  must skip them (see the skip lists in the existing adapters).
- collectors must never crash on bad input. Skip bad lines, keep going.

## Releases

You do not cut releases by hand. Pushing to `main` triggers
`.github/workflows/release.yml`, which picks the next `vX.Y.Z` from the
commit messages, builds binaries for Linux, macOS, and Windows, and publishes
them to the releases page with checksums. Pushing a `vX.Y.Z` tag releases
that exact version. The full rules are in the workflow file and in
`README.md` under Releases.

## Reporting issues

Open an issue with what you ran, what you expected, and what happened.
Include `mtrx doctor` output and the log file when you can
(`~/.local/state/mtrx/mtrx.log`).
