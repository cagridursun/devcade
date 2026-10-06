# DevCade games: rules and policies

All built-in games share these rules:
- They fit an 80×24 terminal, draw each board cell two columns wide in
  printable ASCII, and never use color alone to tell objects apart.
- Boards have a fixed logical size: resizing the window only re-centers them.
- `Space` pauses (except on an end screen) and `Enter` on a game over or win
  screen starts a fresh run. `Q`/`Esc` return to the game submenu, then the
  main menu; `Ctrl+C` quits. A direct game ID uses the same submenu.
- Time only advances while the game is visible and running. Pauses, small
  windows, the menu and end screens never add time, and a single frame
  advances at most 100 ms.
- Completed runs update a saved personal best per game; unfinished quits do not.
  Optional global sharing syncs those bests; see [settings](settings.md).
- Every game processes its timers in time order, so the outcome doesn't
  depend on how time is split into frames.

## Snake (`snake`)

| Rule | Value |
| --- | --- |
| Board | 36 × 18 cells: head `@@`, body `oo`, food `**` |
| Start | Three cells in the center heading right; score 0, level 1 |
| Food | Placed on a uniformly chosen free cell; one segment of growth and 10 points each |
| Level | 1 + foods eaten ÷ 5 (rounded down) |
| Speed | One cell every max(80 ms, 180 ms − 15 ms × (level − 1)) |
| Losing | Hitting a wall (no wraparound) or a body cell. Moving into the cell the tail leaves on the same step is allowed |
| Winning | Filling the whole board |

**Turning:** up to two turns are queued between steps and one is applied per
step. Each turn is checked against the last queued direction, so a quick
Up, Left while moving right turns up and then left, never back into the neck.
Repeats, reversals and a third turn are ignored.

## Block Drop (`blockdrop`)

| Rule | Value |
| --- | --- |
| Board | 10 columns × 20 visible rows, plus 2 hidden rows above |
| Glyphs | Falling piece `<>`, settled blocks `[]`, landing projection `::`, empty ` .` |
| Pieces | I, O, T, S, Z, J, L from a shuffled seven-piece bag (each bag has every piece once) |
| Gravity | One row every max(100 ms, 700 ms − 50 ms × (level − 1)) |
| Level | 1 + cleared lines ÷ 10 |
| Line clears | 100 / 300 / 500 / 800 points for 1 / 2 / 3 / 4 lines, × the level **before** the clear |
| Soft drop | `Down`: one row per key press, 1 point per row actually moved |
| Hard drop | `Enter`: drops to the landing row and locks at once, 2 points per row moved |

**Controls:** `Left`/`Right` move one column per press, `Up` rotates
clockwise, `Z` rotates counterclockwise.

**Rotation:** every piece has four states built by clockwise quarter turns of
its 4×4 (I), 2×2 (O) or 3×3 box. O rotation changes nothing. After a rotation
the game tries these offsets in order and keeps the first that fits:
(0,0), (−1,0), (+1,0), (−2,0), (+2,0), (0,−1), with x to the right and y down
(so the last one is one row up). If none fits, nothing changes. This is
DevCade's own kick list, not the standard "SRS" system.

**Locking:**
- A piece resting on the stack or floor locks after 400 ms.
- A successful move or rotation while resting restarts that 400 ms window, at
  most 8 times per piece. Failed moves don't count.
- Lifting off the support stops the lock timer but doesn't give resets back.
- Once the 8 resets are used up, a piece that lands again locks immediately,
  unless it landed lower than anywhere it had rested before. In that case it
  gets one more 400 ms window, still without resets. The piece can only move
  down a finite number of rows, so it can never stall forever.
- New pieces appear in the top visible rows; the hidden rows only give
  rotations headroom.

**Clears and game over:** when a piece locks, all full rows clear at once,
including rows with gaps between them, and the rows above move down. The game
ends if a block is still in a hidden row after clearing, or if the next piece
has no room to appear.

## Maze Chase (`mazechase`)

| Rule | Value |
| --- | --- |
| Maze | An original 29 × 19 map: walls `##`, pellets ` .`, power pellets `()`, player `@@` |
| Chasers | `C1`–`C4`; shown as `c1`–`c4` while vulnerable and `~1`–`~4` while harmless |
| Player speed | One cell every 140 ms |
| Chaser speed | One cell every 180 ms, or 260 ms while vulnerable |
| Pellet | 10 points |
| Power pellet | 50 points and 8 s during which chasers are vulnerable; another power pellet restarts the 8 s |
| Eating a chaser | 200 points; it returns to its spawn after 2 s and is harmless for 1 s |
| Lives | 3. A hit costs one life, puts everyone back on their spawns and gives the player 2 s of harmless time; score and eaten pellets are kept |
| Winning | Eating every pellet and power pellet |
| Losing | Losing the last life |

**Steering:** the last direction pressed is remembered and taken as soon as
that passage is open. Until then the player keeps going, and a wall simply
stops the player.

**Chasers** recompute the shortest walking distance to their target every
step and never turn back unless stuck:
- `C1` targets the player.
- `C2` targets four cells ahead of the player.
- `C3` patrols the four corners.
- `C4` chases when more than 8 steps away, otherwise heads to the bottom-left
  corner.
- While vulnerable, all chasers flee from the player.
- Ties are broken in the order up, left, down, right.

**Order within one moment:** timers expire first, then the player moves (a
power pellet takes effect immediately), then the chasers move, then contacts
are checked once. A contact is either sharing a cell or swapping cells, so
nobody passes through anyone. One moment can cost at most one life. If the
last pellet is eaten in the same moment as a fatal hit, the hit counts first:
it ends the run on the last life, otherwise you still win.

## Blast Grid (`blastgrid`)

| Rule | Value |
| --- | --- |
| Arena | 17 × 13: walls `##`, crates `[]`, bombs `()`, flames `**`, player `@@`, bots `B1`–`B3` |
| Movement | One cell per key press, then a 120 ms cooldown during which presses are ignored |
| Bombs | `Z` places one at your cell; one active bomb per player or bot; 2 s fuse |
| Blast | 3 cells in each direction for 500 ms. Stops before walls; destroys and stops at the first crate; sets off any bomb it reaches |
| Score | 10 per crate and 100 per bot destroyed by your blasts |
| Winning | All three bots gone while you are alive |
| Losing | Being caught by any flame, including your own, and including the moment the last bot dies |

**Bomb cells:** nobody can walk onto a bomb. The player or bot that placed
it may stay on it and step off once, but can't come back while it exists.

**Chain reactions** resolve in the same moment: each bomb explodes once and
its owner gets the bomb back once. All blast lines are traced against the
arena as it was at the start of that moment, so a crate destroyed by one
blast never lets another blast reach farther. When blasts overlap, each
destroyed crate or bot is credited to the earliest-placed bomb that reached
it, and scores only once.

**Bots** decide every 180 ms:
- They predict every pending and chained blast.
- If in danger, they walk the shortest safe route out.
- They bomb an opponent or crates only when they have already found an escape
  route that beats the fuse.
- Otherwise they head for useful safe spots without stepping into a blast
  that is about to happen.
- Ties are broken with the run's seeded randomness, so a run is reproducible.
- Bots also treat each other as opponents.

**Arena:** the layout keeps every floor cell connected once crates are gone,
and every spawn can escape its own bomb. Tests check both.

## Brick Breaker (`brickbreaker`)

Move the paddle with Left/Right or A/D. Enter or Z launches the ball;
serves also launch automatically after 1.5 seconds. Space pauses, Q/Esc
returns to the menu, and Enter restarts after a win or loss. The fixed
60×18 board fits the standard 80×24 terminal and survives resizing.

A run has ten progressively harder layouts and starts with three lives.
Bricks display their remaining durability (`[1]`, `[2]`, `[3]`); `####`
are steel obstacles that reflect even piercing balls. Only breakable bricks
must be cleared. Ball speed increases with the level and destroyed bricks;
paddle impact position changes its angle, with a minimum horizontal component
to avoid vertical stalls. Losing the last active ball costs one life.

Destroyed bricks have a 20% chance to drop one of six equally likely bonuses:

| Glyph | Bonus | Effect |
|---|---|---|
| W | Wide Paddle | Paddle grows from 9 to 13 columns for 10 seconds |
| M | Multi Ball | Adds two balls, up to five active balls |
| S | Slow Ball | All balls run at 65% speed for 10 seconds |
| L | Extra Life | Adds one life, up to five |
| P | Piercing Ball | Destroys breakable bricks without reflecting for 10 seconds |
| X | Score Multiplier | Doubles brick points for 10 seconds |

Collect drops by catching them with the paddle. Repeated timed bonuses refresh
their duration; timers advance only during gameplay. Losing a life or advancing
a level clears bonuses and drops. Losing one ball during multiball preserves
both the remaining balls and active effects.

Normal, reinforced and heavy bricks award 10, 25 and 50 base points when
destroyed. Successive destructions before a paddle bounce earn combo ratios
of 1× (one brick), 1.2× (2–3), 1.5× (4–6), 2× (7–9) and 3× (10+).
Points use integer arithmetic, rounding down per brick. Steel and partial hits
award no points. Each cleared level adds `level × 500`; completing all ten
adds `remaining lives × 250` once. No time bonus or endless mode is included.

Personal bests and global boards rank by score, using the existing opt-in
sharing path. Opt-in run metrics also record score, bricks destroyed, levels
cleared, highest combo, balls lost and active play time. Balls lost counts
individual balls; lives are consumed only when no active balls remain.

Deployment: update the leaderboard service together with the client before
publishing a release. Older servers reject the new `brickbreaker` game ID and
new optional run summary. Local play and personal bests remain available.

## Space Shooter (`spaceshooter`)

Space Shooter is game seven. Brick Breaker is game five and Terminal FC is
game six. There are seven playable games.

- Arrows/WASD move one cell within the bottom four rows; movement accepts at
  most one press per 70 ms of active time. Fire is automatic every 240 ms.
- Z uses a special charge, clearing hostile bullets globally and hitting a seven-cell-wide
  vertical corridor centered on the ship: one damage to ordinary enemies,
  three to a boss whose hitbox overlaps the corridor. One initial charge; a boss clear grants one, capped at two.
  Activations are separated by at least 750 ms. A full-width line sweeps
  upward across the arena for 600 ms, with `==` marking the damaging corridor
  and `--` outside it. Damage occurs once at activation; the sweep is visual,
  follows gameplay time and freezes during suspension. Space pauses; Enter restarts
  after game over; Q/Esc return; Ctrl+C exits.
- Three lives; losing one resets position and removes hostile threats within
  three cells of the previous/reset position. Protection lasts 1.5 s, shown
  as `{}`. A shield absorbs one damage event and protects for 500 ms.
- Waves begin with a one-second countdown; movement remains enabled. Ordinary
  waves contain `min(18, 6 + 2*(wave-1))` enemies in a seeded permutation of
  eighteen spaced slots. Index modulo three chooses Scout (0), Diver (1,
  introduced in wave 2), Gunner (2, introduced in wave 3); earlier unavailable
  types are Scouts. Scout `><`: 1 HP, 10 points; Diver `VV`: 1 HP, 20; Gunner
  `[]`: 2 HP, 30. Enemies escaping the bottom request damage and give no points.
- Every fifth wave has only a five-cell boss: HP `12 + 4*min(5, wave/5-1)`,
  500 kill points. It sweeps the upper arena, alternating aimed and three-way
  spread shots; a projectile hits it only once. `!!`/`V!`/`[!` and boss
  warning glyphs telegraph attacks for 400 ms. Diver starts after 1.2 s plus
  telegraph, then alternates lateral steps while descending.
- Base movement intervals: Scout/boss 400 ms, Diver 300 ms, Gunner 600 ms.
  Scouts/Gunners descend at edges and every tenth move. Gunner firing timer
  1.8 s, boss 1.4 s, plus telegraph. Intervals scale by
  `base - min(14,wave-1)*base/30`, capped at wave 15. Player bullets move
  10 cells/s; hostile bullets 6 cells/s (spread adds +/-2 lateral cells/s);
  pickups fall 3 cells/s.
- Ordinary kills have a seeded 15% pickup chance, equally split: RF gives
  120 ms firing for eight seconds, refreshed on collection; SH gives one
  shield charge, replaced on collection. Pickups expire after eight seconds
  or leaving the arena. Bosses give no random drops.
- Kill points and the 100-point clear bonus use `min(5, 1+(wave-1)/5)`.
  A clear scores once using the completed wave before incrementing it. Final
  life loss takes precedence over wave clear: kill points remain, no bonus.
  Score saturates at 1,000,000,000; wave number saturates at 1,000,000.
- Logical arena: 36x18 cells, two terminal columns per cell, minimum 80x24.
  Simulation: 20 ms fixed step with retained fractional time. Rendering uses
  no RNG and resize only changes placement. Per-tick order: expire timers;
  move/spawn; special attack; swept opposing bullet contact; swept enemy hits;
  player contact/escape requests; collect pickups; coalesced player damage;
  fatality; surviving clear. Sweeps use relative movement; stable creation IDs
  break equal-time ties within each collision phase; a projectile hits the
  nearest traversed enemy once. Opposing contacts use traversal-time order.
- Caps: 18 enemies (one on boss waves), 128 projectiles total, 24 pickups.
  Full projectile/pickup caps discard the new entity deterministically. Spawns
  use a permutation, with no retries. Clear removes projectiles and pickups
  while retaining lives, unexpired power-ups and protection.

Local bests follow the shared completed-run policy; quitting unfinished does
not submit a best. Score and usage sharing remain separate and default on; players can opt out independently in Settings.
Deploy a backend recognizing `spaceshooter` **before** publishing the new
client or website. Older servers reject the new ID. Preserve existing
profile/server data and identities; no migration or reset is required.


## Terminal FC (`terminalfc`)

Terminal FC is a three-minute 5v5 arcade football match on a fixed **36 × 18**
logical pitch. Each logical cell is two terminal columns wide. Home always
attacks right and Away always attacks left. Each side has one goalkeeper,
one defender, two midfielders and one forward. You control one Home outfield
player at a time; the other nine players are bots.

| Rule | Value |
| --- | --- |
| Live match time | 180 s |
| Simulation step | 20 ms fixed step |
| Bot decision interval | 120 ms |
| Human movement | One logical cell per accepted press; 80 ms cooldown |
| Pass / shot cooldown | 300 ms |
| Tackle cooldown | 700 ms |
| Manual player switch | 250 ms |
| Pass speed | 12 logical cells/s |
| Shot speed | 20 logical cells/s |
| Free-ball deceleration | 4 logical cells/s² |
| Kicker reclaim grace | 150 ms |
| Ownership-change protection | 250 ms |
| Goal overlay | 1.5 s |
| Kickoff / other restart | 1 s |
| Goalkeeper hold | At most 2 s |

**Controls:** arrow keys move the selected player and update facing. `A`
plays a lob pass in possession and tackles while defending. `S` plays a
ground pass in possession and presses toward the ball while defending. `D`
shoots while in possession. `W` switches to another Home outfield player.
Space uses the shared pause behavior; Q/Esc leaves the match.

**Ball and contacts:** passes and shots release one independent ball rather
than teleporting possession. Ball contacts and boundary crossings are swept
along the movement segment. The earliest event wins; an exact goal-line
contact/crossing tie goes to the boundary crossing. A goal is therefore
decided by the actual crossing point inside the six-row goal mouth. Fast
shots are deflected by outfield contacts rather than instantly controlled;
goalkeepers may control a valid contact. Stable player IDs break geometric
ties.

**Selection:** a Home outfield player receiving possession becomes selected.
When Away gains possession, the nearest Home outfield player is selected once.
`W` explicitly switches according to ball ownership and distance. The
selected player is never moved by teammate bot logic.

**Restarts:** a goal shows a short GOAL overlay, then the conceding side takes
the next kickoff. Touchline exits become throw-ins for the team opposite the
last touch. End-line exits outside the goal become either a goal kick or a
corner. Restarts use a deterministic one-second phase and an automatic short
release.

**Clock:** only live play consumes the 180-second clock. Kickoff countdowns,
goal overlays, restarts, shared pause and undersized-window suspension do not.
At full time the match freezes immediately; no contact or goal after the
deadline is accepted. Enter then creates a completely fresh match.

**Leaderboard score:** football goals are separate from the DevCade arcade
score. On a completed match:

```text
resultPoints     = 1000 win, 500 draw, 0 loss
goalPoints       = 50 × min(10, homeGoals)
differencePoints = 25 × min(10, max(0, homeGoals - awayGoals))
arcadeScore      = resultPoints + goalPoints + differencePoints
```

The valid range is **0–1750** and only the best single completed match is
stored. Leaving early records no completed result. A completed zero-point
match still follows the same personal-best and sharing policy.

**Bot policy:** both teams use the same movement/action limits. The current
policy keeps role-based home zones, chooses a primary presser or free-ball
chaser, lets carriers dribble/pass/shoot, and bounds goalkeepers to their own
area while tracking the ball and distributing within two seconds.

**Current limitations:** there is one balanced standard match. There is no
online/local two-player mode, season mode, licensed club content, offside,
fouls/cards, stamina, substitutions, charged shots or audio. The first
version deliberately favors a compact readable terminal match over a full
football simulation.
