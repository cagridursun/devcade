# DevCade

Arcade games that run in your terminal, for the minutes spent waiting on a
build, a test run or an AI response.

DevCade is a non-commercial terminal arcade collection for developers. Open
another terminal, run `devcade`, pick a game and play without leaving the
terminal: no browser, no graphical window, and no language runtime needed for
the distributed binaries. The first release is planned to ship four games —
**Snake**, **Block Drop**, **Maze Chase** and **Blast Grid** — on Windows,
macOS and Linux, with more games added in later feature releases.

## Current status: M2 arcade menu

**No games are playable yet.** Running `devcade` opens the arcade menu. It
lists the four planned games, each clearly marked *Coming soon* with the
milestone that will deliver it:

| ID | Game | Status |
| --- | --- | --- |
| `snake` | Snake | Coming soon (M3) |
| `blockdrop` | Block Drop | Coming soon (M4) |
| `mazechase` | Maze Chase | Coming soon (M5) |
| `blastgrid` | Blast Grid | Coming soon (M6) |

The only runnable activity is the **terminal diagnostic**, a developer tool
from M1 in which an `@` moves around a box to check input, timing, resize and
terminal restoration. Open it from the menu with `D`, or directly with
`devcade --diagnostic`.

| Area | Status |
| --- | --- |
| M0/M1 terminal core | Merged ([PR #1](https://github.com/cagridursun/devcade/pull/1)). Real-terminal compatibility checks are still tracked in the [terminal checklist](docs/terminal-checklist.md) |
| M2 implementation and automated checks | Complete (see [CI](.github/workflows/ci.yml)) |
| M2 real-terminal acceptance | **Pending.** See the checklist |
| Installers (Homebrew, Windows, Linux packages) | Not started (M7). `brew install devcade` does **not** exist yet |

## Requirements

- Go **1.26** or newer to build from source. `go.mod` declares `go 1.26.0`;
  CI uses the latest 1.26.x patch release.
- An interactive terminal of at least **80 columns × 24 rows**.

## Run, build and test

From the repository root:

```sh
go run ./cmd/devcade                  # open the arcade menu
go run ./cmd/devcade list             # list games and availability (no terminal needed)
go run ./cmd/devcade --diagnostic     # start the terminal diagnostic directly
go run ./cmd/devcade snake            # start a game by ID; exits with status 2 while it is coming soon
go run ./cmd/devcade --help
go run ./cmd/devcade --version
```

`--help`, `--version` and `list` never open the fullscreen view or touch the
console, so they work in pipes and scripts. A coming-soon or unknown game ID,
extra arguments, and combinations such as `--diagnostic snake` are usage
errors (exit status 2) and are reported before the terminal is touched.

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

Letters work in upper or lower case. The footer always shows the keys that
work on the current screen.

**Menu**

| Key | Action |
| --- | --- |
| `Up` / `Down`, `W` / `S` | Move the selection (wraps at the ends) |
| `Enter` | Play the selected game. For a coming-soon game it shows a short "not playable yet" note instead and stays in the menu |
| `D` | Open the terminal diagnostic |
| `Q`, `Esc`, `Ctrl+C` | Quit DevCade |

**Terminal diagnostic**

| Key | Action |
| --- | --- |
| Arrow keys, `W` `A` `S` `D` | Change direction |
| `Space` | Pause / resume |
| `Q`, `Esc` | Back to the menu, which keeps your selection. With `--diagnostic` these quit instead, since there is no menu |
| `Ctrl+C` | Quit DevCade from anywhere |

- Menu and activities share one terminal session. Opening the diagnostic or
  going back to the menu does not restart the screen; it is restored once,
  when DevCade exits.
- Each launch starts a fresh, unpaused activity. Leaving one discards it;
  there is no saved or resumable session. Time spent in the menu is never
  replayed into a game.
- The menu also needs 80×24. When the window is smaller it shows the size
  warning and ignores navigation keys (so the selection can't change
  unseen), but `Q`, `Esc` and `Ctrl+C` still work. The selection is kept
  across resizes.
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
- On Windows, `devcade` first checks that the console can process VT escape
  sequences. It turns the flag on briefly on `CONOUT$` and then restores the
  original mode. If the console can't (older than Windows 10 1809), devcade
  exits with an error naming Windows Terminal or a newer Windows, instead of
  printing raw escape codes. tcell's legacy console backend could also hang
  on this failure.
- Restoration cannot be guaranteed if the process is force-killed (SIGKILL,
  Task Manager "End task"), the machine loses power, or the terminal window is
  destroyed abruptly. If a terminal is ever left in a bad state, run `reset`
  (macOS/Linux) or open a new tab.

## Architecture

```
cmd/devcade/          CLI: commands, flags, signals, exit codes, error reporting
internal/arcade/      Built-in game catalog; menu and menu/activity navigation
internal/engine/      Game and Canvas contracts; pause, resize and frame-timing policy
internal/terminal/    tcell adapter: screen lifecycle, event reader, key mapping, canvas
internal/games/probe/ The terminal diagnostic (moving '@'), written as a game
docs/                 Manual terminal acceptance checklist
```

Dependency direction: `cmd → arcade, terminal → engine ← games`. Games import
only `engine`. They never import the terminal backend or the arcade, read
stdin, write stdout, emit escape sequences, handle signals or run their own
loop.

**Shared game contract** (`engine.Game`): `MinimumSize`, `Start` (called once,
the first time the screen is big enough), `Resize`, `HandleInput` (normalized
keys), `Update(dt)` (elapsed game time, capped), and `Render(Canvas)`. The
engine is the only caller, from one goroutine. Canvas cells are printable ASCII
only. Any other rune is drawn as `?` so that one rune always takes one cell.
Colors are decorative only.

**Input.** The terminal adapter turns each key press into an `engine.Event`:
a normalized `Key` (`Up`, `Down`, `Left`, `Right`, `Pause`, `Select`, `Back`,
`Exit`) plus the lower-case letter typed, if any. `Back` (Q/Esc) means "leave
this screen" and `Exit` (Ctrl+C) means "leave DevCade". Games only receive the
`Key` values. The menu also reads the letter, which is how `D` opens the
diagnostic even though `d` means "right" in a game.

**Catalog and navigation.** `arcade.Catalog` is the single, ordered list of
built-in games. The menu, `devcade list` and `devcade <id>` all read it.
Construction rejects empty, non-lower-case or duplicate IDs, and an entry is
either playable (it has a factory) or planned (it names a milestone), never
both. Factories run only on an actual launch, each launch builds a fresh game,
and listing or drawing the menu never builds one. There is no runtime
discovery or plugin loading: a later milestone adds a game by giving its
catalog entry a factory. `arcade.App` has two states, the menu and one active
engine. It forwards time, input and resizes only to the active engine. The
terminal loop drives either an `arcade.App` (the menu) or a single engine
(`--diagnostic`, or a game started by ID) through the same small `Program`
interface.

**Concurrency.** One loop goroutine owns all navigation, engine and game state
and does all rendering. A reader goroutine forwards backend events through a 64-event
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
| Windows amd64 | Local (Windows 11) + CI | Yes | M1 only: owner-reported diagnostic startup and direction input in Windows Terminal. M2 menu pending |
| Windows arm64 | — | Yes | Untested |
| macOS arm64 (Apple Silicon) | CI (`macos-latest`) | Yes | Pending |
| macOS amd64 | — | Yes | Untested |
| Linux amd64 | CI (`ubuntu-latest`) | Yes | M1 only: PTY smoke test reported in the M2 brief (diagnostic startup, direction input, pause, undersized resize, quit, restored TTY attributes). M2 menu pending |
| Linux arm64 | — | Yes | Untested |

A PTY smoke test drives the program through a pseudo-terminal. It is not the
same as testing by hand in a terminal emulator. The unit tests use tcell's
simulated screen. A successful cross-build only
shows the code compiles for that target, not that it works in a real terminal.
Git Bash's default mintty window is not a Windows console. Use Windows
Terminal, or run `winpty devcade` there.

## Roadmap

| Milestone | Goal |
| --- | --- |
| M0/M1 | Bootstrap and terminal core: merged; remaining manual compatibility checks tracked |
| M2 | Arcade menu, catalog and built-in selection plumbing |
| M3 | Snake |
| M4 | Block Drop |
| M5 | Maze Chase |
| M6 | Blast Grid |
| M7 | Packaging/distribution/installers |
| M8 | Four-game v1.0 |

Homebrew, Windows and Linux installation routes are M7 work and do not exist
yet. Accounts, multiplayer, online leaderboards, analytics and plugins are out
of scope for v1.

## License

The license has not been chosen yet. Until the owner picks one, no license is
granted.
