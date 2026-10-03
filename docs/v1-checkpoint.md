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
| Contract freeze (`KeyAction`) | Done |
| M3 Snake audit | Done: rules, Finisher, KeyAction ignored, fresh factory per launch. No changes needed. Owner reported Snake works in their terminal (environment not recorded) |
| M4 Block Drop | Delegated |
| M5 Maze Chase | Delegated |
| M6 Blast Grid | Delegated |
| M7 Packaging | Delegated |
| Catalog / CLI / docs integration | Pending |
| Review agent pass | Pending |
| M8 verification | Pending |

## Decisions log

- `KeyAction` is appended after `KeyExit`, so existing key values are unchanged.
- Snake ignores `KeyAction` (its `HandleInput` only acts on directions).
