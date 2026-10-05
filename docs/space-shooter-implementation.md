# Space Shooter — SS1–SS3 implementation report

## Scope and branch

Implemented in PR #13, integrated with main after Brick Breaker PR #12 merged
on 2026-10-05. The catalog order is Snake, Block Drop, Maze Chase, Blast Grid,
Brick Breaker, Terminal FC (coming soon) and Space Shooter. Space Shooter is
game seven in the menu and website. Six entries are playable; Terminal FC's
sixth slot is reserved until its implementation merges. No Terminal FC gameplay
was copied into this PR. Settings/profile rows follow catalog length.

No AGENTS.md was found in the available workspace/repository. No new dependencies,
production score submissions, production analytics, releases or deployments
were used. The initial implementation was verified locally; it was subsequently published
as PR #13. This report also includes the requested special-attack follow-up.

## SS1 — core

- Added `internal/games/spaceshooter`, implementing Game, Finisher and Score
  with compile-time assertions; registered menu and `devcade spaceshooter`.
- Fixed logical 36x18 arena, two terminal columns per cell, minimum 80x24.
  Responsive centering, readable ASCII actors and read-only rendering.
- Input follows normalized keys and terminal repeats: a single accepted
  movement press moves one cell; cooldown includes fractional simulation time.
  Movement remains available during a one-second preparation countdown.
- Automatic fire, Scouts, three lives, damage protection, scoring, wave
  transitions, game over and a complete reset. Resize does not change state.
- Per-game injected random source; production uses fresh PCG seeds. Restart
  clears all run state while continuing the source rather than rewinding it.

## SS2 — arcade rules

- Distinct Scouts, telegraphed alternating Divers, durable firing Gunners and
  fifth-wave bosses. Bosses stay above the player region, show numeric HP,
  alternate aimed/spread fire and receive only three damage from a special.
- Seeded 15% ordinary-kill pickups, evenly split between rapid fire and shield.
  Refresh/replacement, expiry, protection and bounded special charges.
- Relative swept collisions cover traversed projectile/target paths and
  opposing projectile crossings. Created entity IDs and stable slice order
  break equal-time ties. Opposing contacts resolve by traversal time, and
  player bullets hit the nearest traversed enemy once.
- Coalesced damage requests; fatality precedes clear bonuses. Escapes award
  no kill points. Surviving clears retain unexpired effects and grant a boss
  charge while removing old projectiles and pickups.
- Integer wave multipliers and score saturation preserve validated limits.
  Explicit enemy/projectile/pickup caps and retry-free spawn placement.

## SS3 — integration

- Added `spaceshooter` to profile validation, server/client score paths,
  terminal catalog, leaderboard website tabs/names/live fetch/snapshots,
  analytics dashboard, CLI help and CI/release smoke IDs.
- Found and removed the server's obsolete `len(Best) > 4` persistence check.
  Per-key ValidScore validation already limits accepted maps to recognized
  IDs. A fifth best now survives restart; existing scores/identities and
  datastore/profile versions remain compatible.
- Added English, Turkish, Spanish, Dutch and French strings through the
  existing localization system. Website content includes both English and
  Turkish; the build copies an actual Space Shooter PTY screenshot.
- Updated rules, README, changelog, screenshot provenance and release checks.
  Historical release/acceptance claims about four games remain historical.
- Reused shared tracking and score observers. Real completed runs are saved,
  submitted and fetched using isolated local HTTP servers. Consent tests cover
  all four score/usage sharing combinations, two completed runs, lower-score
  replay, unfinished exit and application close. Metrics tests check duration,
  restart run IDs, duplicate event protection and journal reopen.

## Controls and tuning

| Input | Action |
|---|---|
| Arrows / WASD | Move one cell in bottom four rows, 70 ms cooldown |
| Automatic | Fire every 240 ms, or 120 ms with rapid fire |
| Z | Special: clear hostile bullets; seven-cell corridor, ordinary damage 1 / boss 3 |
| Space | Shared engine pause |
| Enter | Restart from game over |
| Q / Esc | Shared menu return |
| Ctrl+C | Shared application exit |

| Parameter | Value |
|---|---|
| Simulation step | 20 ms, fractional remainder retained |
| Preparation | 1 second per wave |
| Lives / protection | 3 lives; 1.5 s after life loss |
| Shield | One charge; 500 ms protection after absorption |
| Rapid fire / pickup lifetime | 8 seconds; refresh, no speed stacking |
| Special inventory / cooldown | Start 1, boss clear +1, cap 2; 750 ms minimum |
| Special damage / visual | Seven-cell corridor centered on ship; full-width upward sweep for 600 ms |
| Scout / boss movement | 400 ms base |
| Diver movement / start | 300 ms base; 1.2 s then 400 ms telegraph |
| Gunner movement | 600 ms base |
| Gunner / boss firing timer | 1.8 s / 1.4 s base; 400 ms telegraph |
| Difficulty | `base - min(14,wave-1)*base/30`; capped at wave 15 |
| Bullet speeds | Player 10 cells/s; hostile 6; spread x velocity -2/0/+2 |
| Pickup fall speed | 3 cells/s |
| Entity caps | 18 enemies, 128 total bullets, 24 pickups |
| Numeric caps | Score 1,000,000,000; wave 1,000,000 |

Firing timers accumulate during telegraphing. An enemy fires when its warning
expires; a new warning requires the interval threshold and no active warning.
Scouts/Gunners descend at edges and every tenth movement. Spawns permute
18 spaced upper-arena slots; modulo-three composition introduces Divers in
wave 2 and Gunners in wave 3. Full caps discard newly requested entities.
Life loss removes nearby hostile bullets/bodies within three cells of the old
or reset player position, without awarding kill points for removed bodies.

Per-tick precedence:

1. Expire timers; move/spawn enemies and automatic bullets.
2. Consume a queued special; clear hostile bullets, apply corridor damage once
   (ordinary 1, boss 3) and start the 600 ms visual sweep.
3. Move bullets; resolve swept opposing bullet contacts, then enemy hits.
4. Request player damage for fire/body/escape contacts; collect pickups.
5. Apply at most one damage event; cull entities.
6. Finish on zero lives; otherwise evaluate clear and enter next preparation.

Kills: Scout 10, Diver 20, Gunner 30, boss 500. Clear: 100. Both use
`min(5, 1+(wave-1)/5)`, integer division, before wave increment. A fatal tick
keeps earned kills but gives no clear bonus. Survival alone scores nothing.
The bounded wave counter remains at its maximum on extremely long runs; that
maximum is a boss wave. IDs preserve creation order throughout normal play.

## Verification results

All executed successfully:

- `gofmt -l .` (no output) and `git diff --check`.
- `go vet ./...`.
- `go test -timeout 60s ./...`.
- `go test -race -timeout 120s ./...`.
- Native builds of `./cmd/devcade` and `./cmd/devcade-leaderboard`.
- `node --test tools/pages/*.test.mjs`: 19 tests passed.
- `node tools/pages/build.mjs --offline`: all five boards built as unavailable,
  intentionally without production requests or invented scores.
- `DEVCADE_RELEASE_E2E=1 go test -count=1 -timeout 300s -run EndToEnd -v ./tools/release`:
  full CGO-free six-target release-builder dry run passed. It verifies archives,
  manifests, checksums and native installed binary behavior; cross-target binaries
  were compiled, not executed on their respective operating systems.

Game tests cover exact cooldown, preparation/fire cadence, boundaries, swept
hits, projectile crossings, boss single-hit behavior, shield/coalescing,
protection, fatal clear precedence, composition/HP/scoring, escapes, caps,
refresh/expiry, RNG continuity/read-only rendering, equivalent timestamped
inputs with differently partitioned frames, and engine suspension. Twenty
seeded idle runs terminate without leaking/stalling. A separate protected
steering driver clears ordinary/boss targets by automatic fire through capped
wave/difficulty values, isolating reachability from driver survival.

## PTY and visual verification

Actual Linux amd64 CLI runs in isolated 80x24 PTYs, with score/usage endpoints
explicitly disabled, verified:

- Launch through the game menu, movement, shared pause and unchanged paused frame.
- Shrink to 40x12, size warning, enlarge to 80x24, retained pause, resume.
- English/colorful: game over after an actual idle run, Enter clean restart,
  Q game-menu return, Ctrl+C normal exit and restored TTY attributes.
- Turkish/mono: translated HUD/help/status, movement/pause/resize/menu return,
  normal exit and restored TTY attributes.

PNG captures and text grids live in `docs/screenshots`; the English gameplay
capture was visually inspected. Automated terminal harnesses additionally
exercise menu/direct launch, game over, restart and resize for every available
game. All locale renders are checked at 80x24, including maximum score/wave
and the boss HUD. Boss behavior and progression are verified by controlled
headless scenarios; an extended interactive human boss playtest was not done.
Windows/macOS human terminal playability also remains unverified for this game.
These limitations should remain explicit in platform acceptance notes.

## PR-ready text and rollout

**Title:** feat: add Space Shooter with deterministic waves, bosses and shared integration

**Description:**

Add Space Shooter to the terminal menu and CLI with automatic fire, three enemy
types, fifth-wave bosses, power-ups and a limited special attack. Fixed-step
simulation and swept collision handling keep timing, damage and scores stable
across frame partitions, pause and resize. Completed runs use existing personal
best persistence, opt-in leaderboard submission and separate usage consent.

Register the new game throughout profile/server validation, localized UI,
analytics dashboard, website boards and platform smoke checks. Remove the
obsolete four-best server restart limit while preserving datastore identities
and versions. Brick Breaker is preserved from main; Terminal FC is a planned sixth entry.

Validation: full Go tests/vet/race, website tests/offline build, six-platform
CGO-free release dry run and real English/colorful and Turkish/mono PTY checks.
Bosses are covered by deterministic/reachability tests; extended human boss
and Windows/macOS playtests remain outstanding.

**Rollout order (not performed):** Deploy the compatible leaderboard/analytics
backend first; preserve datastore/profile files and identities. Then publish
the client and website. An older backend rejects `spaceshooter` even if the
new local game works. When Terminal FC merges, replace its planned entry with its factory and retain
all existing IDs in validators and smoke checks.

## Special-attack follow-up (PR #13)

Normal enemies have 1-2 HP, so the original three-damage global special
instantly cleared an ordinary wave. At the user's request, enemy damage now
affects only a seven-cell vertical corridor centered on the ship. Ordinary
enemies take one damage; bosses take three if any hitbox cell overlaps it.
Hostile bullets are still cleared across the arena. One-charge/cooldown/scoring
behavior remains shared with the previous implementation.

A full-width ASCII line moves from the bottom to the top in 600 ms. The corridor
uses `==`, the remainder `--`; actors draw over the line for readability, and
all five locales show a translated SPECIAL ATTACK status. The visual consumes
no randomness, produces no additional damage, stays anchored to the activation
column and freezes on pause/undersize. It expires even during the next wave's
preparation, and restart removes it.

Regression tests cover near/far targets, both corridor edges, boss hitbox
overlap, Gunner survival, one-time points, first-wave preservation across 100
seeds, the moving line's lifetime, read-only rendering, boundary layout,
suspension and reset.

Follow-up verification passed: `go vet ./...`, full Go tests, full race tests,
native client build, clean gofmt/diff checks, and a real Turkish/Mono PTY
special activation. The latter observed the sweep at arena rows 16 and 9,
verified pause freeze, expiry, wave-one preservation and TTY restoration.
The captured midpoint is `docs/screenshots/spaceshooter-special.png`.

## Integration and game-seven ordering

Merged main after Brick Breaker PR #12, preserving both games and the
Brick Breaker metrics/statistics, translations, website card and smoke checks.
Reserved catalog slot six for Terminal FC as coming soon; Space Shooter now
occupies slot seven. The website displays 05/06/07 accordingly, with only six
playable games included in score fetching and smoke launches. No release
workflow dispatch, tag or release publication is part of this integration.
