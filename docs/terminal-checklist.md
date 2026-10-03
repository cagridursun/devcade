# Real-terminal acceptance checklist

**Status: pending.** Automated checks use tcell's simulated screen, and
cross-builds only prove the code compiles. The M1 terminal core (merged), the
M2 arcade menu and M3 Snake are accepted only after the surfaces below have
been tested by hand. Earlier results are kept in their own column and do not
count toward later milestones.

A *shell* (PowerShell, cmd, bash, zsh) is not a *terminal emulator* (Windows
Terminal, conhost, Terminal.app, iTerm2, GNOME Terminal). Record both, because
input and rendering behavior comes from the terminal. A PTY smoke test drives
DevCade through a pseudo-terminal, which is useful evidence, but it is not a
terminal emulator test.

## Surfaces

| Platform | Terminal emulator | Shell | OS / version | Terminal version | M1 diagnostic (section B) | M2 menu (section A) | M3 Snake (section D) | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Windows | Windows Terminal | PowerShell | | | Pending | Pending | Pending | |
| Windows | Windows Terminal | Not recorded | Not recorded | Not recorded | **Partial (owner-reported, 2026-10-04, before M2):** startup, immediate input and arrow/WASD direction control work | Pending | Pending | Pause, resize, quit and restoration not yet reported. M1 evidence only; not an M2 or M3 test |
| Windows | Windows Terminal | cmd | | | Pending | Pending | Pending | |
| Windows | Legacy console (conhost.exe, Windows 10 1809+ with VT support) | PowerShell or cmd | | | Pending | Pending | Pending | Best-effort target; consoles without VT support are rejected |
| macOS | Terminal.app | zsh | | | Pending | Pending | Pending | |
| macOS | iTerm2 | zsh | | | Pending | Pending | Pending | |
| macOS | Apple Silicon (`darwin/arm64` binary), either terminal | zsh | | | Pending | Pending | Pending | |
| Linux | GNOME Terminal (or equivalent: name it) | bash | | | Pending | Pending | Pending | |
| Linux | tmux inside any terminal (record both) | bash | | | Pending | Pending | Pending | Check `TERM` inside tmux |
| Not recorded | Owner's terminal (not recorded) | Not recorded | Not recorded | Not recorded | n/a | n/a | **Passed (owner-reported, M3 review):** "works without problems" | Owner's own play test of Snake. Environment and individual steps not recorded |
| Linux amd64 | PTY smoke test (not an emulator) | n/a | Not recorded | n/a | **Passed (reported before M2):** startup, direction input, pause, undersized resize, quit while undersized, restored TTY attributes | Pending | Pending | Covers the M1 diagnostic only |

Consoles that cannot enable VT output processing (Windows before 10 version
1809) are rejected at startup with an error that names Windows Terminal or a
newer Windows. The console mode must be unchanged afterwards. This rejection
is covered by automated tests only, because no such console was available.

Not supported: Git Bash's default mintty window (not a Windows console; use
`winpty devcade` or Windows Terminal), and running with stdin or stdout
redirected.

## Before each surface

Build or run from source, or use the binary built for that platform. Fill some
scrollback first (for example run `ls` or `dir` a few times) so restoration can
be checked.

## A. Arcade menu (M2): `devcade`

1. **Startup.** At 80×24 or larger, `devcade` opens the menu (not the
   diagnostic) in the alternate screen with the cursor hidden. It shows the
   title, Snake / Block Drop / Maze Chase / Blast Grid in that order, with
   Snake `Available` and the others `Coming soon (M4)`…`(M6)`, a `>` marker
   on Snake, Snake's description, a `D  Terminal diagnostic` line and a
   footer.
2. **Footer.** On Snake the footer says `Enter: play`. On a coming-soon game
   it says `Enter: details`, never `Enter: play`.
3. **Selection.** `Down`/`Up`, `s`/`w` and `S`/`W` move the marker
   immediately. It wraps from the last game to the first and back. The
   description follows the marker. Holding a key causes no growing lag.
4. **Unavailable entry.** `Enter` on Block Drop, Maze Chase or Blast Grid keeps the menu open and shows
   "<Game> is not playable yet: it is planned for M<n>." Moving the selection
   clears the note.
5. **Diagnostic from the menu.** `D` (and `d`) opens the diagnostic with a
   footer `Q / Esc: back to menu   Ctrl+C: quit DevCade`. Run steps B2–B4.
   Then `Q` returns to the menu with the same game selected. Repeat with
   `Esc`. Pause the diagnostic, go back, open it again: it starts fresh and
   unpaused. The screen must not flash back to the shell between menu and
   diagnostic.
6. **Resize the menu.** Shrink below 80×24: the size warning appears.
   Arrow keys do nothing. Enlarge: the menu redraws cleanly with the same
   selection. `Q` quits while undersized (try a very small size too).
7. **Exit.** Quit from the menu separately with `q`, `Esc` and `Ctrl+C`, and
   once with `Ctrl+C` while the diagnostic is open from the menu. Each time:
   previous screen contents and scrollback come back, the cursor is visible,
   typing echoes, and line editing works. The exit status is 0.

## B. Terminal diagnostic (M1): `devcade --diagnostic`

1. **Startup.** At 80×24 or larger, the diagnostic opens directly: `DEVCADE`
   header, controls line, status line, bordered box and one `@`.
2. **Movement.** The `@` moves right about 8 cells per second and bounces off
   the walls without flicker, scrolling or leftover characters.
3. **Immediate input.** Arrows and `w` `a` `s` `d` (and `W` `A` `S` `D` with Caps
   Lock or Shift) change direction straight away, with no Enter.
4. **Pause.** Space shows `PAUSED` and freezes the `@`. Wait ~5 s. Space again
   resumes from the same cell with no jump.
5. **Shrink.** Below 80×24 the "window too small" message shows the needed
   and current sizes, and the `@` doesn't advance. `q` still quits.
6. **Enlarge.** The layout redraws cleanly, the `@` is inside the box, and
   play resumes without a jump.
7. **Pause across resize.** Pause, shrink, enlarge: still paused.
8. **Exit.** In this direct mode `q`, `Q`, `Esc` and `Ctrl+C` each quit
   DevCade (no menu appears). Check restoration as in A7. Exit status 0.

## D. Snake (M3): from the menu and with `devcade snake`

1. **Start.** Enter on Snake (or `devcade snake`) shows the HUD
   (`SNAKE  Score 0  Level 1  Length 3  PLAYING`), a controls line, and a
   bordered board with `@@oooo` in the center heading right and one `**`
   food. The last row holds only the menu footer, or nothing in direct mode.
2. **Movement.** The snake moves steadily about 5–6 cells per second without
   flicker, leftover glyphs or scrolling.
3. **Turning.** Arrows and WASD turn immediately. Tap two turns quickly (for
   example Up then Left while moving right): the snake turns up, then left,
   and never reverses into itself. Pressing the opposite direction alone
   does nothing.
4. **Food.** Eating `**` adds one `oo` segment and 10 points. Food never
   appears on the snake. Every 5 foods the level goes up and the snake
   speeds up.
5. **Pause.** Space shows `PAUSED` and freezes the board. Resume after ~5 s:
   no jump.
6. **Resize.** Shrink below 80×24 (warning, frozen), then enlarge: the board
   is the same size and the snake and food are where they were. Only the
   centering changes. A pause survives this.
7. **Game over.** Run into a wall (and, separately, into your body). The
   board stays visible with `GAME OVER`, the final score and
   `Enter: play again`. Space does nothing here. Enter starts a fresh run
   (score 0, length 3, centered, unpaused).
8. **Leaving.** From the menu: Q and Esc return to the menu with Snake still
   selected. Entering again starts a fresh run. With `devcade snake`: Q/Esc
   quit. Ctrl+C quits while playing, paused, too small and on the game over
   screen. Check restoration each time as in A7.

## C. Command line (any surface)

1. `devcade --help`, `devcade --version` and `devcade list` print without
   opening the fullscreen view. They also work piped, for example
   `devcade list | more`.
2. `devcade blockdrop` (and `mazechase`, `blastgrid`) prints "not available
   yet" and exits with status 2, without opening the screen. `devcade snake`
   with redirected input fails with "interactive terminal is required" and
   status 1.
3. `devcade nope`, `devcade list extra`, `devcade --diagnostic snake` and
   `devcade --version --diagnostic` print usage errors with status 2.
4. `devcade < /dev/null` (PowerShell: `"" | devcade`) and the same with
   `--diagnostic` fail straight away with "interactive terminal is required"
   and status 1.
5. **Signals (macOS/Linux).** From another terminal, `kill -TERM <pid>` while
   in the menu or diagnostic restores the terminal with exit status 143.
   `kill -INT <pid>` gives 130.
6. **Windows console close.** Closing the tab ends the process. There is no
   screen left to inspect, so just note anything unusual.

## Known limits

Restoration cannot be guaranteed after SIGKILL, Task Manager "End task", power
loss or a terminal destroyed underneath the process. If a terminal is left
in a broken state, run `reset` (POSIX) or open a new tab.

## Results log

Add one row per run to the surface table above, and record any failures here
with steps to reproduce.
