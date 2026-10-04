# DevCade v1 checkpoint log

Coordinator log for finishing the four-game v1 release candidate (M4–M8).
Update this file as work lands; it is the source of truth if context is lost.

## Baseline

- `main` = `cf33b34` (merge of PR #3). M0/M1 (#1), M2 (#2) and M3 Snake (#3)
  are merged.
- Integration branch: `feat/v1-arcade-suite`, created from `cf33b34`.
- Contract freeze commit: the first commit on `feat/v1-arcade-suite` (adds
  `engine.KeyAction`). Every agent worktree starts from it.
- Toolchain: Go 1.26.8 (`go.mod`: `go 1.26.0`), tcell v2.13.10. No dependency
  changes are planned; games use the standard library only.

## Frozen shared contract (games)

Games live in `internal/games/<id>` and import only
`github.com/cagridursun/devcade/internal/engine` plus the standard library.
They never read stdin, write stdout, use tcell, emit escape sequences, handle
signals, start goroutines or timers, or touch files or the network.

Each game package exports `func New() engine.Game`, which returns a fresh game
with its own random source. Tests build games through an unexported
constructor that takes an injected `*rand.Rand` (math/rand/v2).

A game implements `engine.Game` and `engine.Finisher`:

| Method | Contract |
| --- | --- |
| `MinimumSize()` | `80, 24` |
| `Start(w, h)` | Called once, when the screen first fits. Fully resets the run |
| `Resize(w, h)` | Layout only. Logical boards never change |
| `HandleInput(k)` | Normalized keys only (below). While finished, only `KeySelect` (restart) has an effect |
| `Update(dt)` | Gameplay time, already capped at 100 ms by the engine and never sent while paused, undersized or in the menu. `dt <= 0` and finished runs are no-ops. Keep remainders and process deadlines in time order |
| `Render(c)` | Printable ASCII only, two terminal columns per logical cell, fits 80x24, centered in `w x (h-1)`. **Never writes the last row** (`arcade.App` draws the navigation footer there) |
| `Finished()` | True on game over or win. The engine then ignores Space, so Enter always reaches the game |

Normalized keys (`engine.Key`) and their meaning:

| Key | Physical | Meaning |
| --- | --- | --- |
| `KeyUp/Down/Left/Right` | arrows, W/A/S/D (either case) | directions. Block Drop: Up = rotate clockwise, Down = soft drop |
| `KeyAction` | Z (either case) | primary action. Block Drop: rotate counterclockwise. Blast Grid: place bomb |
| `KeySelect` | Enter | Block Drop: hard drop while playing. All games: restart when finished |
| `KeyPause` | Space | handled by the engine (never reaches games) |
| `KeyBack` | Q, Esc | handled by arcade/engine: back to menu, or quit when launched directly |
| `KeyExit` | Ctrl+C | handled by arcade/engine: quit from anywhere |

The menu keeps `D` as the diagnostic shortcut. Inside games `d` is just Right.
Restart (Enter on the end screen) resets all state, including elapsed time,
timers, queues and RNG-driven state, without recreating the terminal session.

End screens: keep the board visible and show a box with the result (GAME OVER
/ YOU WIN), the final score and `Enter: play again`. The HUD controls line ends
with `Pause: Space   Leave: Q / Esc   Exit: Ctrl+C` (shortened if needed),
matching Snake.

Reference implementation: `internal/games/snake` (model, rendering, strict
canvas tests, engine integration tests).

## Ownership

| Owner | Paths | Branch / worktree |
| --- | --- | --- |
| Coordinator | `internal/engine`, `internal/terminal`, `internal/arcade`, `cmd/devcade`, `go.mod`/`go.sum`, `README.md`, `.github/workflows/ci.yml`, `docs/terminal-checklist.md`, this file, `CHANGELOG.md` | `feat/v1-arcade-suite` |
| Block Drop agent | `internal/games/blockdrop/**` | own worktree branch |
| Maze Chase agent | `internal/games/mazechase/**` | own worktree branch |
| Blast Grid agent | `internal/games/blastgrid/**` | own worktree branch |
| Packaging agent | `tools/release/**`, `packaging/**`, `.github/workflows/release.yml`, `docs/install.md`, `docs/releasing.md` | own worktree branch |
| Review agent | read-only | none |

Snake (M3) is merged and owned by no other session: coordinator audit only.

## Status

| Item | Status |
| --- | --- |
| Contract freeze (`KeyAction`) | Done (`80ea9b4`) |
| M3 Snake audit | Done: rules, Finisher, KeyAction ignored, fresh factory per launch. No changes needed. Owner reported Snake works in their terminal (environment not recorded) |
| M4 Block Drop | Done by the Block Drop agent (`feat/v1-blockdrop` `5f48f0d`), merged. Review fixes (`f75decc`: distinct `<>` piece glyph, depth-based lock after exhausted resets) merged |
| M5 Maze Chase | Done by the Maze Chase agent (`feat/v1-mazechase` `cccf352`), merged |
| M6 Blast Grid | Done by the Blast Grid agent (`feat/v1-blastgrid` `259f9fb`), merged. Review fix (`41aa00e`: safe bots never route through pending blasts) merged |
| M7 Packaging | Done by the packaging agent (`feat/v1-packaging` `1897769`), merged. Nothing published |
| Catalog / CLI / docs integration | Done: all four games registered (`80a1002`), help, README, CHANGELOG, docs/games.md, CI release dry run |
| Generic per-game loop test | Done (`internal/terminal/games_test.go`): every playable game through menu, pause/resize, end screen, restart, back, Ctrl+C, direct launch |
| Review agent pass | Done on `80a1002`: 2 defects (Block Drop color-only active piece; Blast Grid bots stepping into pending blasts), 2 low (Block Drop zero lock delay after exhausted budget; stale CLI comment). All four resolved: CLI comment by the coordinator, the others by the owning agents, re-verified after merge |
| M8 verification | Automated: complete on `0c3e4bf` (Windows local; CI pending at push). Release candidate `1.0.0-rc.1` built locally from `0c3e4bf` into `dist/release` (gitignored, not uploaded); `SHA256SUMS` verified; Windows amd64 binary smoke-tested. Manual terminal acceptance: pending (no usable interactive terminal in this session) |

Note: every agent worktree was created at `cf33b34` rather than the freeze
commit; each agent reset its branch to `80ea9b4` before starting, so all
branches share the frozen baseline.

## Decisions log

- `KeyAction` is appended after `KeyExit`, so existing key values are unchanged.
- Snake ignores `KeyAction` (its `HandleInput` only acts on directions).
- Block Drop lock rule after the 8-reset budget is used: a fresh 400 ms only
  when landing lower than any previous resting row; otherwise lock at once.
- Blast Grid scoring attribution: earliest-placed covering bomb.
- Maze Chase uses no randomness; its constructor takes no RNG.
- CI's Ubuntu job runs the release builder as a dry run (nothing uploaded).

## Release preparation (2026-10-04)

- PR #4 merged; CI synchronization fix `1165c6d` is included on main.
- Owner reports successful Windows gameplay of all four games.
- Four 80×24 PTY captures added to the README; normal quit and TTY restoration
  verified for every game on Linux amd64.
- Restart timing fixed: the first frame spanning a finished run is discarded.
- MIT license added; project and dependency license texts bundled in archives.
- Release workflow prepares artifacts on PRs and verifies actual packaged
  native binaries/installers on three OSes. Publication is explicit, main-only
  and requires public repository visibility.
- Versioned RC release notes prepared in `docs/releases/1.0.0-rc.1.md`.

Remaining: merge release preparation after green checks, make repo public,
run the publishing workflow, verify anonymous public downloads, collect
remaining human macOS/Linux acceptance. Homebrew/Scoop repositories and
signing are deferred; cross-builds are not manual platform acceptance.

## Settings and global rankings follow-up (2026-10-04)

PR #5 now also includes five languages, three palettes, username/guest
onboarding, New game/Leaderboard submenus, atomic local bests and an anonymous
HTTP global score server/client. The server has native tests, persistent
snapshots, max-only updates, public own-rank lookup and single-writer OS locks.
Docker/Caddy deployment files and a release endpoint input are included.
Publication now requires a deployed healthy public HTTPS score endpoint.
The community service is live at https://devcade.cinesdigital.com on Google
Compute Engine using Docker/Caddy and the persistent score volume. The owner
verified Windows public HTTPS access and a real Snake submission, then restarted
the server and confirmed the same score remained. Source builds and official
release packages now default to that endpoint; sharing remains opt-in.
See docs/settings.md and docs/leaderboard.md. Other games/platforms retain their
automated coverage; the earlier four-game Windows gameplay report predates
this follow-up.
