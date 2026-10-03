# DevCade

Arcade games that run in your terminal, for the minutes spent waiting on a
build, a test run or an AI response.

DevCade is a non-commercial terminal arcade collection for developers. Open
another terminal, run `devcade`, pick a game and play without leaving the
terminal: no browser, no graphical window, and no language runtime needed for
the distributed binaries. The first release is planned to ship four games —
**Snake**, **Block Drop**, **Maze Chase** and **Blast Grid** — on Windows,
macOS and Linux, with more games added in later feature releases.

## Current status: M0 + M1

**No games are playable yet.** This milestone delivers the project bootstrap
(M0) and the cross-platform terminal core (M1). Running `devcade` starts a
fullscreen diagnostic: an `@` moves around a box so you can check input,
timing, resize and terminal restoration.

| Area | Status |
| --- | --- |
| Implementation and automated checks | Complete (see [CI](.github/workflows/ci.yml)) |
| Real-terminal acceptance | **Pending.** See the [terminal checklist](docs/terminal-checklist.md) |
| Installers (Homebrew, Windows, Linux packages) | Not started (M7). `brew install devcade` does **not** exist yet |

## Requirements

- Go **1.26** or newer to build from source. `go.mod` declares `go 1.26.0`;
  CI uses the latest 1.26.x patch release.
- An interactive terminal of at least **80 columns × 24 rows**.

## Run, build and test

From the repository root:

```sh
go run ./cmd/devcade              # start the diagnostic
go run ./cmd/devcade --help
go run ./cmd/devcade --version
```

Build a binary (cgo is not required):

```sh
CGO_ENABLED=0 go build -o bin/devcade ./cmd/devcade        # macOS / Linux
```

```powershell
$env:CGO_ENABLED = "0"; go build -o bin\devcade.exe ./cmd/devcade   # Windows PowerShell
```

Checks (the same ones CI runs):

```sh
go mod verify
go mod tidy -diff          # fails if go.mod/go.sum are out of date; edits nothing
gofmt -l .                 # must print nothing
go vet ./...
go test -timeout 60s ./...
go test -race -timeout 120s ./...   # needs cgo and a C compiler (gcc/clang)
```

Dependencies are locked in `go.mod` and `go.sum`, which are both committed.
CI fails rather than repairing them.

## Controls and behavior

| Key | Action |
| --- | --- |
| Arrow keys, `W` `A` `S` `D` (upper or lower case) | Change direction |
| `Space` | Pause / resume |
| `Q`, `Esc`, `Ctrl+C` | Quit |

- Keys act immediately; you don't press Enter.
- The `@` moves one cell every 120 ms and bounces off the walls. The screen
  updates about 30 times a second.
- **Pause** freezes the game and shows a `PAUSED` banner. Paused time is never
  replayed afterwards.
- **Resize:** below 80×24 the game is suspended (not reset) and a message
  gives the required and current sizes, clipped to whatever fits. Quit keys
  still work. When the window is large enough again the layout is recomputed,
  the `@` is kept inside the box, and play resumes without catching up on the
  time spent too small. A pause you started stays on across resizes.
- After a stall such as a laptop sleep, at most 100 ms of game time is applied,
  so nothing jumps across the board.
- **Exit:** quitting, Ctrl+C, SIGINT and SIGTERM (console close/logoff/shutdown
  on Windows), a terminal input error, and a crash inside a game all restore
  the original screen, cursor and input mode before any message is printed.
  Exit status: 0 for a normal quit, 1 for an error, 2 for a usage error, 130
  for SIGINT, 143 for SIGTERM.
- `devcade` refuses to start when stdin or stdout is redirected, and prints
  an error instead of taking over the terminal.
- Restoration cannot be guaranteed if the process is force-killed (SIGKILL,
  Task Manager "End task"), the machine loses power, or the terminal window is
  destroyed abruptly. If a terminal is ever left in a bad state, run `reset`
  (macOS/Linux) or open a new tab.

## Architecture

```
cmd/devcade/          CLI: flags, --help/--version, signals, exit codes, error reporting
internal/engine/      Game and Canvas contracts; pause, resize and frame-timing policy
internal/terminal/    tcell adapter: screen lifecycle, event reader, key mapping, canvas
internal/games/probe/ The M1 diagnostic (moving '@'), written as a game
docs/                 Manual terminal acceptance checklist
```

Dependency direction: `cmd → terminal → engine ← games`. Games import only
`engine`. They never import the terminal backend, read stdin, write stdout,
emit escape sequences, handle signals or run their own loop.

**Shared game contract** (`engine.Game`): `MinimumSize`, `Start` (called once,
the first time the screen is big enough), `Resize`, `HandleInput` (normalized
keys), `Update(dt)` (elapsed game time, capped), and `Render(Canvas)`. The
engine is the only caller, from one goroutine. Canvas cells are printable ASCII
only. Any other rune is drawn as `?` so that one rune always takes one cell.
Colors are decorative only.

**Concurrency.** One loop goroutine owns all engine and game state and does
all rendering. A reader goroutine forwards backend events through a 64-event
buffer. When the buffer is full the reader blocks, and the backpressure reaches
tcell's own bounded queue, so floods of input cannot grow memory. The reader
keeps draining tcell's queue until `Fini` returns. tcell's internal goroutines
post events with a blocking send and `Fini` waits for them, so if nothing
drained the queue, quitting during an input flood could hang.

**Dependency choice.** [tcell](https://github.com/gdamore/tcell) v2.13.10
handles terminal input, colors, the alternate screen and diff-based rendering
on Windows (VT console APIs) and POSIX terminals (terminfo), all in pure Go.
DevCade does not add a second frame buffer: each frame redraws tcell's logical
buffer, `Show` writes only the cells that changed, and a full `Sync` repaint
happens only after a resize. `golang.org/x/term` is used for the
interactive-terminal check.

## Compatibility

| Target | Native tests | Cross-build (CGO_ENABLED=0) | Interactive terminal |
| --- | --- | --- | --- |
| Windows amd64 | Local (Windows 11) + CI | Yes | Pending |
| Windows arm64 | — | Yes | Untested |
| macOS arm64 (Apple Silicon) | CI (`macos-latest`) | Yes | Pending |
| macOS amd64 | — | Yes | Untested |
| Linux amd64 | CI (`ubuntu-latest`) | Yes | Pending |
| Linux arm64 | — | Yes | Untested |

The unit tests use tcell's simulated screen. A successful cross-build only
shows the code compiles for that target, not that it works in a real terminal.
Git Bash's default mintty window is not a Windows console. Use Windows
Terminal, or run `winpty devcade` there.

## Roadmap

| Milestone | Goal |
| --- | --- |
| M0 | Project bootstrap |
| M1 | Cross-platform terminal core and diagnostic |
| M2 | Arcade menu and built-in game selection |
| M3 | Snake |
| M4 | Block Drop |
| M5 | Maze Chase |
| M6 | Blast Grid |
| M7 | Packaging, binary distribution and installers |
| M8 | Four-game v1.0 release |

Homebrew, Windows and Linux installation routes are M7 work and do not exist
yet. Accounts, multiplayer, online leaderboards, analytics and plugins are out
of scope for v1.

## License

The license has not been chosen yet. Until the owner picks one, no license is
granted.
