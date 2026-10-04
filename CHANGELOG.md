# Changelog

## Unreleased

- Limit Space Shooter special damage to a seven-cell vertical corridor
  (ordinary enemies: 1, boss: 3) and add a 600 ms upward sweep visual.

- Add Space Shooter: endless waves, three enemy types, fifth-wave bosses,
  shields, rapid fire and a limited special attack.
- Integrate completed best scores, opt-in analytics, all five UI languages,
  terminal/website leaderboards and platform smoke checks.
- Validate persisted server bests by recognized IDs instead of a fixed four-game
  map limit; preserve existing data and identities.


## 1.0.0-rc.2 (published 2026-10-04)

- Fix Maze Chase exposing no score to the shared personal-best/leaderboard path.
  Add real-game completion, persistence and submission regression coverage.
- Read public leaderboard scores on website opening and Refresh, with restricted
  read-only CORS and the newest available per-game snapshot as a fallback.
- Add a private operator analytics panel and independent, default-off game/site
  usage sharing; download counters are distinct from active installations.

## 1.0.0-rc.1 (published 2026-10-04)

First four-game release candidate.

### Settings, profile and community rankings
- Five UI languages; English is the default. Three palettes; black/white is the default.
- Username onboarding with guest play and opt-in anonymous score sharing.
- Game-specific New game / Leaderboard submenus, persistent completed-run bests.
- Shared HTTP leaderboard server/client with max-only per-game updates and own rank.
- Creator attribution/profile launcher and new-games notice in the main menu.
- Public HTTPS service deployment is a launch prerequisite; container/proxy files are included.

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

### Acceptance status
- Public release candidate, GitHub Pages website, Homebrew tap and Scoop bucket
  are published. Stable v1.0 acceptance remains tracked separately.
- Binaries are not signed or notarized.
- The project is licensed under MIT.
- Real-terminal acceptance is incomplete; see `docs/terminal-checklist.md`.

## Earlier milestones
- M3 Snake ([#3](https://github.com/cagridursun/devcade/pull/3)).
- M2 arcade menu and catalog ([#2](https://github.com/cagridursun/devcade/pull/2)).
- M0/M1 project bootstrap and terminal core ([#1](https://github.com/cagridursun/devcade/pull/1)).
