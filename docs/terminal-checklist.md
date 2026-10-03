# Real-terminal acceptance checklist

**Status: pending.** Automated checks use tcell's simulated screen, and
cross-builds only prove the code compiles. The M1 terminal core (merged) and
the M2 arcade menu are accepted only after the surfaces below have been tested
by hand.

A *shell* (PowerShell, cmd, bash, zsh) is not a *terminal emulator* (Windows
Terminal, conhost, Terminal.app, iTerm2, GNOME Terminal). Record both, because
input and rendering behavior comes from the terminal. A PTY smoke test drives
DevCade through a pseudo-terminal, which is useful evidence, but it is not a
terminal emulator test.

## Surfaces

| Platform | Terminal emulator | Shell | OS / version | Terminal version | M1 diagnostic (section B) | M2 menu (section A) | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Windows | Windows Terminal | PowerShell | | | Pending | Pending | |
| Windows | Windows Terminal | Not recorded | Not recorded | Not recorded | **Partial (owner-reported, 2026-10-04, before M2):** startup, immediate input and arrow/WASD direction control work | Pending | Pause, resize, quit and restoration not yet reported. This is M1 evidence, not an M2 navigation test |
| Windows | Windows Terminal | cmd | | | Pending | Pending | |
| Windows | Legacy console (conhost.exe, Windows 10 1809+ with VT support) | PowerShell or cmd | | | Pending | Pending | Best-effort target; consoles without VT support are rejected |
| macOS | Terminal.app | zsh | | | Pending | Pending | |
| macOS | iTerm2 | zsh | | | Pending | Pending | |
| macOS | Apple Silicon (`darwin/arm64` binary), either terminal | zsh | | | Pending | Pending | |
| Linux | GNOME Terminal (or equivalent: name it) | bash | | | Pending | Pending | |
| Linux | tmux inside any terminal (record both) | bash | | | Pending | Pending | Check `TERM` inside tmux |
| Linux amd64 | PTY smoke test (not an emulator) | n/a | Not recorded | n/a | **Passed (reported before M2):** startup, direction input, pause, undersized resize, quit while undersized, restored TTY attributes | Pending | Covers the M1 diagnostic only |

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
   title, Snake / Block Drop / Maze Chase / Blast Grid in that order, each
   marked `Coming soon (M3)`…`(M6)`, a `>` marker on Snake, Snake's
   description, a `D  Terminal diagnostic` line and a footer.
2. **Footer.** For a coming-soon game the footer says `Enter: details`, never
   `Enter: play`.
3. **Selection.** `Down`/`Up`, `s`/`w` and `S`/`W` move the marker
   immediately. It wraps from the last game to the first and back. The
   description follows the marker. Holding a key causes no growing lag.
4. **Unavailable entry.** `Enter` on any game keeps the menu open and shows
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

## C. Command line (any surface)

1. `devcade --help`, `devcade --version` and `devcade list` print without
   opening the fullscreen view. They also work piped, for example
   `devcade list | more`.
2. `devcade snake` (and `blockdrop`, `mazechase`, `blastgrid`) prints
   "not available yet" and exits with status 2, without opening the screen.
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
