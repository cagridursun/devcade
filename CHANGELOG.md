# Changelog

## 1.0.0-rc.1 (release candidate, not published)

First four-game release candidate.

### Games
- **Snake**: fixed 36×18 board, two-turn input queue, speed rises every five
  foods, wins on a full board.
- **Block Drop**: 10×20 board with two hidden spawn rows, seven-piece bag,
  clockwise and counterclockwise rotation with a small documented wall-kick
  list, 400 ms lock delay with an eight-reset budget, soft and hard drop
  scoring, next-piece preview and landing projection.
- **Maze Chase**: an original 29×19 maze, four chasers with distinct
  deterministic policies, power pellets that make chasers vulnerable for eight
  seconds, three lives.
- **Blast Grid**: a 17×13 arena against three bots that plan escapes before
  bombing, chain reactions resolved at one timestamp, deterministic scoring
  attribution.

### Arcade
- All four games are listed as available in the menu, `devcade list` and
  `devcade <id>`.
- `Z` is a new normalized action key: Block Drop rotates counterclockwise,
  Blast Grid places a bomb. Space stays pause everywhere and Enter restarts
  every end screen.

### Release preparation
- MIT project license and complete dependency license texts in every archive.
- Four real terminal captures at the top of the README.
- Restart discards the first frame spanning a finished run, preventing old
  elapsed time from being delivered to the new run.
- Release workflow verifies packaged binaries and installation on Linux,
  macOS and Windows before optional publication.

### Release tooling
- `go run ./tools/release -version <v>` builds reproducible `CGO_ENABLED=0`
  archives for Windows, macOS and Linux (amd64 and arm64), writes
  `SHA256SUMS`, and generates a Homebrew formula and a Scoop manifest filled
  from the real checksums.
- Checksum-verifying user-local installers `install.sh` and `install.ps1`.
- A manual `release` GitHub workflow that builds and uploads the artifacts
  without publishing anything.

### Not done yet
- No GitHub release, tag, Homebrew tap or Scoop bucket has been published, and
  the repository is private.
- Binaries are not signed or notarized.
- The project is licensed under MIT.
- Real-terminal acceptance is incomplete; see `docs/terminal-checklist.md`.

## Earlier milestones
- M3 Snake ([#3](https://github.com/cagridursun/devcade/pull/3)).
- M2 arcade menu and catalog ([#2](https://github.com/cagridursun/devcade/pull/2)).
- M0/M1 project bootstrap and terminal core ([#1](https://github.com/cagridursun/devcade/pull/1)).
