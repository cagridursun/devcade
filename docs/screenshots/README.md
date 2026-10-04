# Terminal captures

These PNGs show actual output from the DevCade Linux amd64 binary, running
in an 80×24 PTY with `TERM=xterm-256color`, 24-bit color output (`TCELL_TRUECOLOR=enable`) and `NO_COLOR`
unset. Game captures use the Colorful palette; the Settings image shows
Turkish UI with Midnight. Captured on 2026-10-04 from the locally built `1.0.0-rc.1` archive,
installed using the bundled installer. Normal game
inputs confirm New game, move the player, hard-drop pieces or place bombs
before capture. The Settings session changes language and palette through
normal key input; both choices are saved to the isolated capture profile.

The ANSI output was decoded by pyte and rasterized with Pillow and the local
DejaVu Sans Mono font. Colors use a conventional dark terminal palette;
terminal emulators may use a different palette or font. Board cells, HUD,
scores and actors come from the running game, not a simulated scene or an
image generator. Each session quit normally and restored TTY attributes.
The adjacent `.txt` files preserve the captured character grid.

Screenshots document appearance; they are not full manual platform acceptance.

Space Shooter captures were made on 2026-10-05 from the local feature-branch
Linux amd64 CLI in isolated 80x24 PTYs. `spaceshooter.png` uses English and
Colorful; `spaceshooter-tr-mono.png` uses Turkish and Mono;
`spaceshooter-game-over.png` shows a completed real idle run. Both external
endpoints were explicitly disabled. Move/pause/shrink/enlarge/menu return and
TTY restoration passed; the English run also exercised game over and restart.
The captures use pyte/Pillow with DejaVu Sans Mono. Boss behavior is covered
by controlled tests, not an extended interactive human playtest. These images
do not imply Windows/macOS human acceptance.

`spaceshooter-special.png` and its text grid show the requested upward special
sweep in a Turkish/Mono 80x24 PTY on 2026-10-05. The actual client was built
from the special-attack fix, with both service endpoints disabled. PTY checks
confirmed the line moving upward, holding on pause, expiring and preserving
wave one; Ctrl+C restored the terminal attributes.
