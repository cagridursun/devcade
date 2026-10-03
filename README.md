# DevCade

Install once. Play arcade games without leaving your terminal.

DevCade is a non-commercial terminal arcade collection for developers waiting
for builds, tests or agent responses. Planned first games: Snake, Block Drop,
Maze Chase and Blast Grid. All four must share the same terminal core.

## Current checkpoint: M0 + M1 draft

This scaffold provides a moving `@` diagnostic, not the four arcade games yet.
It has a pure-Go game contract, a 30 Hz loop, ASCII cell drawing, directional
keyboard input, pause, resize suspension, and deferred screen restoration.
The tcell backend owns terminal input, colors, alternate screen and differential
rendering; games do not write ANSI sequences or read standard input directly.

**Validation status:** this draft has not been built or tested. The preparation
environment had no Go installation and no access to the new GitHub repository.
There is no generated `go.sum` yet. Do not mark M1 complete until the commands
below, CI, and the interactive terminal checklist pass.

## Run from source

Install Go 1.24 or newer, then from the repository root:

```sh
go mod tidy
gofmt -w cmd internal
go run ./cmd/devcade
```

Minimum terminal size: 80 columns by 24 rows. The `@` moves automatically;
arrows or WASD change direction. Space pauses; Q, Escape or Ctrl+C exits.
Making the terminal too small freezes gameplay until it is enlarged. A user
pause remains active across resize.

```sh
go run ./cmd/devcade --version
go run ./cmd/devcade --help
go build -o bin/devcade ./cmd/devcade
```

On Windows, use `go build -o bin/devcade.exe ./cmd/devcade`.
Homebrew and Windows package-manager installation are future release work;
`brew install devcade` is not available yet.

## Verify

```sh
go vet ./...
go test -race -timeout 60s ./...
go build ./...
```

Commit the generated `go.sum` and formatting changes after verification. CI
runs native checks on Linux, macOS and Windows and builds amd64/arm64 targets
without CGO. Compilation and simulated screen tests do not establish real
terminal compatibility. Follow [the acceptance checklist](docs/terminal-checklist.md).

## Layout

- `cmd/devcade`: CLI entry point, signal handling and error reporting.
- `internal/engine`: backend-independent game/canvas contracts and timing policy.
- `internal/terminal`: tcell input, screen lifecycle and renderer adapter.
- `internal/games/probe`: the M1 moving-character diagnostic.
- `docs`: release checkpoints and terminal acceptance criteria.

## Roadmap

| Milestone | Deliverable |
| --- | --- |
| M0 | Go bootstrap, docs and three-OS CI |
| M1 | Verified terminal core and moving-character diagnostic |
| M2 | Arcade menu and game registry |
| M3 | Snake: score, restart, pause and collision |
| M4 | Block Drop |
| M5 | Maze Chase |
| M6 | Blast Grid |
| M7 | Six binary targets, checksums and installers |
| M8 | Four-game v1.0 release |

Later games ship as feature releases. Online services, multiplayer and accounts
are outside v1. License selection is pending; this scaffold does not choose a
license on the owner's behalf.
