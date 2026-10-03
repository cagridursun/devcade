# M1 real-terminal acceptance checklist

**Status: pending.** Automated checks use tcell's simulated screen, and
cross-builds only prove the code compiles. M1 is accepted only after the
surfaces below have been tested by hand.

A *shell* (PowerShell, cmd, bash, zsh) is not a *terminal emulator* (Windows
Terminal, conhost, Terminal.app, iTerm2, GNOME Terminal). Record both, because
input and rendering behavior comes from the terminal.

## Surfaces

| Platform | Terminal emulator | Shell | OS / version | Terminal version | Result | Limitations / notes |
| --- | --- | --- | --- | --- | --- | --- |
| Windows | Windows Terminal | PowerShell | | | Pending | |
| Windows | Windows Terminal | Not recorded | Not recorded | Not recorded | **Partial (owner-reported, 2026-10-04)** | Startup, immediate input and arrow/WASD direction control work. Pause, resize, quit and restoration not yet reported |
| Windows | Windows Terminal | cmd | | | Pending | |
| Windows | Legacy console (conhost.exe, Windows 10 1809+ with VT support) | PowerShell or cmd | | | Pending | Best-effort target; consoles without VT support are not supported |
| macOS | Terminal.app | zsh | | | Pending | |
| macOS | iTerm2 | zsh | | | Pending | |
| macOS | Apple Silicon (`darwin/arm64` binary), either terminal | zsh | | | Pending | |
| Linux | GNOME Terminal (or equivalent: name it) | bash | | | Pending | |
| Linux | tmux inside any terminal (record both) | bash | | | Pending | Check `TERM` inside tmux |

Consoles that cannot enable VT output processing (Windows before 10 version
1809) are rejected at startup with an error that names Windows Terminal or a
newer Windows. The console mode must be unchanged afterwards. This rejection
is covered by automated tests only, because no such console was available.

Not supported: Git Bash's default mintty window (not a Windows console; use
`winpty devcade` or Windows Terminal), and running with stdin or stdout
redirected.

## Steps for each surface

Build or run from source (`go run ./cmd/devcade`), or use the binary built for
that platform. Fill some scrollback first (for example run `ls` or `dir` a few
times) so restoration can be checked.

1. **Startup.** At 80×24 or larger: an alternate screen opens, the cursor is
   hidden, and you see the `DEVCADE` header, a controls line, a status line, a
   bordered box and one `@`.
2. **Movement.** The `@` moves right about 8 cells per second and bounces off
   the walls without flicker, scrolling or leftover characters.
3. **Immediate input.** Arrows and `w` `a` `s` `d` (and `W` `A` `S` `D` with Caps
   Lock or Shift) change direction straight away, with no Enter. Holding a key
   (auto-repeat) causes no lag and no growing delay.
4. **Pause.** Space shows `PAUSED` and freezes the `@` (the status line
   position doesn't change). Wait ~5 s. Space again resumes from the same cell
   with no jump.
5. **Shrink.** Make the window smaller than 80×24. A "window too small"
   message shows the needed and current sizes (clipped if very small). The `@`
   doesn't advance. `q` still quits (also try this at a very small size, then
   start again).
6. **Enlarge.** Make it larger than 80×24. The layout redraws cleanly to the
   new size, the `@` is inside the box, and play resumes without a jump.
7. **Pause across resize.** Pause, shrink, enlarge: the game must still be
   paused.
8. **Exit.** Quit separately with `q`, `Q`, `Esc` and `Ctrl+C`. Each time: the
   previous screen contents and scrollback come back, the cursor is visible,
   typed characters echo, and line editing (backspace, arrows, history) works.
   The exit status is 0.
9. **Non-TTY.** `devcade --version` and `devcade --help` print without opening
   the fullscreen view. `devcade < /dev/null` (PowerShell: `"" | devcade`)
   fails straight away with "interactive terminal is required" and status 1.
   `devcade nope` prints a usage error with status 2.
10. **Signals (macOS/Linux).** From another terminal run `kill -TERM <pid>`.
    The terminal must be restored and the exit status must be 143. With
    `kill -INT <pid>`, the status must be 130.
11. **Windows console close.** Closing the tab ends the process. There is no
    screen left to inspect, so just note anything unusual.

## Known limits

Restoration cannot be guaranteed after SIGKILL, Task Manager "End task", power
loss or a terminal destroyed underneath the process. If a terminal is left
in a broken state, run `reset` (POSIX) or open a new tab.

## Results log

Add one row per run to the surface table above, and record any failures here
with steps to reproduce.
