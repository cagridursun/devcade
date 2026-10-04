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

Brick Breaker was captured on 2026-10-05 from the feature branch build using
the same 80×24 PTY, Colorful palette, pyte/Pillow workflow and font. It uses
an isolated local profile with score/usage sharing disabled. The session
launched through the normal game menu, exited normally and restored TTY attributes.
