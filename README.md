# DevCade

Arcade games that run in your terminal, for the minutes spent waiting on a
build, a test run or an AI response.

DevCade is a non-commercial terminal arcade collection for developers. Open
another terminal, run `devcade`, pick a game and play without leaving the
terminal: no browser, no graphical window, and no language runtime needed for
the distributed binaries. It runs on Windows, macOS and Linux.

<table>
  <tr>
    <td><strong>Snake</strong><br><img src="docs/screenshots/snake.png" alt="Snake running in an 80 by 24 terminal" width="480"></td>
    <td><strong>Block Drop</strong><br><img src="docs/screenshots/blockdrop.png" alt="Block Drop with a falling piece, stack and next-piece preview" width="480"></td>
  </tr>
  <tr>
    <td><strong>Maze Chase</strong><br><img src="docs/screenshots/mazechase.png" alt="Maze Chase with pellets, four chasers and the player" width="480"></td>
    <td><strong>Blast Grid</strong><br><img src="docs/screenshots/blastgrid.png" alt="Blast Grid with bombs, crates and three bots" width="480"></td>
  </tr>
</table>

Captured from the running Linux binary through an 80×24 pseudo-terminal;
these are actual game frames, not mockups. See [capture details](docs/screenshots/README.md).

[Release downloads](https://github.com/cagridursun/devcade/releases) ·
[Installation](docs/install.md) · [Game rules](docs/games.md)

## Status: v1 release candidate

**All four v1 games are playable.** Running `devcade` opens the arcade menu:

| ID | Game | What you do |
| --- | --- | --- |
| `snake` | Snake | Steer a growing snake to food without hitting walls or yourself |
| `blockdrop` | Block Drop | Rotate and drop falling pieces; clear full rows |
| `mazechase` | Maze Chase | Clear the maze of pellets, dodge four chasers, power up to eat them |
| `blastgrid` | Blast Grid | Bomb crates and outlast three bots in a fixed arena |

Exact rules, scoring and timing for each game are in [docs/games.md](docs/games.md).
The menu also offers the **terminal diagnostic** (`D`, or `devcade --diagnostic`):
a developer tool in which an `@` moves around a box to check input, timing,
resize and terminal restoration.

| Area | Status |
| --- | --- |
| Code: four games, menu, CLI, terminal core | Merged into main, with automated tests (see [CI](.github/workflows/ci.yml)) |
| Release tooling (M7) | Ready: reproducible archives, `SHA256SUMS`, installer scripts, Homebrew formula and Scoop manifest generators ([docs/releasing.md](docs/releasing.md)) |
| Distribution | **Not published.** No GitHub release, tap or bucket exists yet, and the repository is private. See [docs/install.md](docs/install.md) |
| Real-terminal acceptance | **Partial.** See the [terminal checklist](docs/terminal-checklist.md) |
| License | MIT; dependency notices included in binary archives |

`brew install devcade` and `scoop install devcade` do **not** work yet.

## Install

Until a release is published, build from source (next section). Once the
owner publishes a release, [docs/install.md](docs/install.md) describes:
- downloading an archive directly and checking it against `SHA256SUMS`;
- the checksum-verifying user-local installers (`install.sh`, `install.ps1`);
- the planned Homebrew tap and Scoop bucket.

Release binaries are not signed or notarized; that document explains what
macOS Gatekeeper and Windows SmartScreen will show.

## Run, build and test

Requires Go **1.26** or newer (`go.mod` declares `go 1.26.0`; CI uses the
latest 1.26.x) and an interactive terminal of at least **80 × 24**.

```sh
go run ./cmd/devcade                  # open the arcade menu
go run ./cmd/devcade snake            # start a game directly: snake, blockdrop, mazechase, blastgrid
go run ./cmd/devcade list             # list the games (no terminal needed)
go run ./cmd/devcade --diagnostic     # start the terminal diagnostic directly
go run ./cmd/devcade --help
go run ./cmd/devcade --version
```

`--help`, `--version` and `list` never open the fullscreen view or touch the
console, so they work in pipes and scripts. Unknown IDs, extra arguments and
combinations such as `--diagnostic snake` are usage errors (exit status 2),
reported before the terminal is touched.

Build a binary (cgo is not required):

```sh
CGO_ENABLED=0 go build -o bin/devcade ./cmd/devcade        # macOS / Linux
```

```powershell
$env:CGO_ENABLED = "0"; go build -o bin\devcade.exe ./cmd/devcade   # Windows PowerShell
```

Release archives for all six targets:

```sh
go run ./tools/release -version 1.0.0-rc.1 -out dist/release
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

## Controls

Letters work in upper or lower case. Keys act immediately; you never need to
hold a key or press Enter to send it.

| Screen | Keys |
| --- | --- |
| Menu | `Up`/`Down` or `W`/`S` select (wraps), `Enter` play, `D` terminal diagnostic, `Q`/`Esc`/`Ctrl+C` quit |
| Every game | `Space` pause/resume, `Enter` play again after game over or a win, `Q`/`Esc` back to the menu, `Ctrl+C` quit DevCade |
| Snake | arrows/WASD turn (up to two quick turns are queued) |
| Block Drop | `Left`/`Right` (`A`/`D`) move, `Up` (`W`) rotate clockwise, `Z` rotate counterclockwise, `Down` (`S`) soft drop, `Enter` hard drop |
| Maze Chase | arrows/WASD steer; a turn waits until the passage opens |
| Blast Grid | arrows/WASD move one cell, `Z` place a bomb |
| Terminal diagnostic | arrows/WASD change direction, `Space` pause |

A game started directly (`devcade snake`, `devcade --diagnostic`) has no menu
to return to, so `Q`/`Esc` quit there.

## Behavior shared by every screen

- **One terminal session.** Moving between the menu and a game never
  restarts the screen; it is restored once, when DevCade exits.
- **Fresh runs.** Each launch, and each Enter on an end screen, starts a new
  unpaused run. Leaving a game discards it. Nothing is saved: scores are for
  the current run only.
- **Pause** freezes the game under a `PAUSED` banner. On a game over or win
  screen Space does nothing, so Enter always restarts.
- **Resize.** Boards have a fixed size, so a larger window only re-centers
  them. Below 80×24 a game freezes (it isn't reset) and a warning shows the
  required and current sizes; `Q`, `Esc` and `Ctrl+C` still work. A pause you
  started stays on across resizes.
- **No time catch-up.** Time spent paused, too small, in the menu or on an
  end screen is never replayed into a game, and a stall such as a laptop
  sleep advances a game by at most 100 ms.
- **Exit.** Quitting, Ctrl+C, SIGINT and SIGTERM (console close/logoff/shutdown
  on Windows), a terminal input error, and a crash inside a game all restore
  the original screen, cursor and input mode before any message is printed.
  Exit status: 0 for a normal quit, 1 for an error, 2 for a usage error, 130
  for SIGINT, 143 for SIGTERM.
- `devcade` refuses to start when stdin or stdout is redirected.
- On Windows, `devcade` first checks that the console can process VT escape
  sequences (briefly enabling the flag on `CONOUT$`, then restoring the
  original mode). Consoles that can't (older than Windows 10 1809) get a clear
  error naming Windows Terminal or a newer Windows.
- Restoration cannot be guaranteed if the process is force-killed (SIGKILL,
  Task Manager "End task"), the machine loses power, or the terminal window is
  destroyed abruptly. If a terminal is ever left in a bad state, run `reset`
  (macOS/Linux) or open a new tab.

## Architecture

```
cmd/devcade/              CLI: commands, flags, signals, exit codes, error reporting
internal/arcade/          Built-in game catalog; menu and menu/game navigation
internal/engine/          Game, Canvas, Finisher and key contracts; pause, resize and timing policy
internal/terminal/        tcell adapter: screen lifecycle, event reader, key mapping, canvas
internal/games/snake/     Snake
internal/games/blockdrop/ Block Drop
internal/games/mazechase/ Maze Chase
internal/games/blastgrid/ Blast Grid
internal/games/probe/     The terminal diagnostic, written as a game
tools/release/            Release builder: archives, checksums, Homebrew/Scoop manifests
packaging/                Installer scripts, manifest templates, license notice
docs/                     Game rules, install, releasing, checklist, v1 checkpoint log
```

Dependency direction: `cmd → arcade, terminal → engine ← games`. Games import
only `engine` and the standard library. They never import the terminal
backend or the arcade, read stdin, write stdout, emit escape sequences, handle
signals, start goroutines or touch files or the network.

**Game contract** (`engine.Game`): `MinimumSize`, `Start` (called once, the
first time the screen is big enough; resets the run), `Resize` (layout only),
`HandleInput` (normalized keys), `Update(dt)` (gameplay time, capped at
100 ms and never sent while paused or undersized) and `Render(Canvas)`. Games
draw printable ASCII only, two terminal columns per board cell, and never the
last row (the arcade's navigation footer). A game that can end implements
`engine.Finisher` (`Finished() bool`); while it reports true the engine
ignores the pause key, and Enter reaches the game to restart it.

**Input.** The terminal adapter turns each key press into an `engine.Event`:
a normalized `Key` (`Up`, `Down`, `Left`, `Right`, `Pause`, `Select`, `Back`,
`Exit`, `Action`) plus the lower-case letter typed, if any. Space is always
the engine's pause, Enter is `Select`, Z is `Action`, Q/Esc are `Back`
("leave this screen") and Ctrl+C is `Exit` ("leave DevCade"). Games only
receive `Key` values. The menu also reads the letter, which is how `D` opens
the diagnostic even though `d` means "right" in a game.

**Adding a game.** Write a package under `internal/games/<id>` that
implements the contract with a `New() engine.Game` factory, add an entry with
that factory to `arcade.Builtin()`, and give the generic integration test in
`internal/terminal/games_test.go` a recipe that ends a run. The menu,
`devcade list` and `devcade <id>` pick it up from the catalog; the terminal
layer does not change. The catalog validates IDs (unique, lower-case) and
never builds a game except on an actual launch.

**Concurrency.** One loop goroutine owns all navigation, engine and game state
and does all rendering. A reader goroutine forwards backend events through a
64-event buffer; when it is full, backpressure reaches tcell's own bounded
queue, so floods of input cannot grow memory. The reader keeps draining
tcell's queue until `Fini` returns, because tcell's goroutines post with a
blocking send and `Fini` waits for them.

**Dependency choice.** [tcell](https://github.com/gdamore/tcell) v2.13.10
handles terminal input, colors, the alternate screen and diff-based rendering
on Windows (VT console APIs) and POSIX terminals (terminfo), all in pure Go.
Each frame redraws tcell's logical buffer; `Show` writes only changed cells,
and a full `Sync` repaint happens only after a resize. `golang.org/x/term` is
used for the interactive-terminal check.

## Compatibility

| Target | Native tests | Cross-build (CGO_ENABLED=0) | Interactive terminal |
| --- | --- | --- | --- |
| Windows amd64 | Local (Windows 11) + CI | Yes | Owner-reported successful gameplay of all four games (2026-10-04); detailed terminal/checklist data not recorded |
| Windows arm64 | — | Yes | Untested |
| macOS arm64 (Apple Silicon) | CI (`macos-latest`) | Yes | Pending |
| macOS amd64 | — | Yes | Untested |
| Linux amd64 | CI (`ubuntu-latest`) | Yes | All four games: PTY startup, gameplay input, normal quit and TTY restoration verified (2026-10-04). Human emulator acceptance pending |
| Linux arm64 | — | Yes | Untested |

Automated tests run every game through the real terminal loop on tcell's
simulated screen. A PTY smoke test is not the same as testing by hand in a
terminal emulator, and a successful cross-build only shows the code compiles
for that target. Git Bash's default mintty window is not a Windows console:
use Windows Terminal, or run `winpty devcade` there.

## Roadmap

| Milestone | Goal | Status |
| --- | --- | --- |
| M0/M1 | Bootstrap and terminal core | Merged; manual compatibility checks tracked |
| M2 | Arcade menu, catalog and built-in selection | Merged |
| M3 | Snake | Merged |
| M4 | Block Drop | Merged |
| M5 | Maze Chase | Merged |
| M6 | Blast Grid | Merged |
| M7 | Packaging, distribution and installers | Archives, installers and gated publishing workflow ready; public release pending |
| M8 | Four-game v1.0 | Release candidate; manual acceptance and publication pending |

See [CHANGELOG.md](CHANGELOG.md) and the [v1 checkpoint log](docs/v1-checkpoint.md).
Accounts, multiplayer, online leaderboards, analytics and plugins are out of
scope for v1.

## License

MIT — see [LICENSE](LICENSE). DevCade is a free hobby project; MIT permits
commercial use as well. Dependencies retain their own licenses. Binary
archives include the complete MIT license and third-party license texts in
`LICENSE-NOTICE.txt`; see [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt).
