# M1 acceptance checklist

Status: NOT VERIFIED. Record OS, terminal name/version, result, and failures for
each run. Shell names alone are not terminal emulators; on Windows explicitly
test PowerShell and cmd hosted in Windows Terminal and the legacy console.

| OS | Required manual surfaces | Result |
| --- | --- | --- |
| Windows | Windows Terminal + PowerShell; Windows Terminal + cmd; legacy console | Pending |
| macOS | Terminal; iTerm2; Apple Silicon binary | Pending |
| Linux | GNOME Terminal or equivalent; tmux | Pending |

For each surface:

1. Start at 80x24 or larger. Confirm alternate screen and hidden cursor.
2. Watch automatic movement. Confirm no flickering or scrolling.
3. Test arrows and WASD, including uppercase WASD.
4. Pause/resume using Space; state must stay frozen while paused.
5. Shrink below 80x24; confirm warning, no gameplay update, and working Q.
6. Enlarge again; confirm state resumes, stays within bounds, and manual pause persists.
7. Exit with Q, Escape, and Ctrl+C separately. Confirm prompt, cursor and normal
   echo/line editing return, and the previous terminal contents are restored.
8. Run --version and --help; neither should open the terminal screen.
9. Run with redirected input or output and document observed backend behavior;
   a non-interactive invocation must fail cleanly rather than hang.
10. On POSIX, send SIGTERM and verify normal cleanup.

Simulated tests cover cancellation and render panic cleanup. Restoration cannot
be guaranteed after forced process termination (SIGKILL), power loss or a closed
terminal. Inspect and test failed initialization separately before M1 acceptance.

## Draft handoff

Repository access was unavailable during preparation. This archive does not
contain or replace repository history. Integrate the files into a clean feature
branch, preserving any existing README, license or other files. Check for
AGENTS.md instructions in the actual repository before integration. Run `go mod
tidy`, `gofmt -w cmd internal`, and the checks from README, commit generated
go.sum, then open a PR. Do not overwrite an existing main branch wholesale.
