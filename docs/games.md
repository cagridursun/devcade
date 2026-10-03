# DevCade games: rules and policies

All four games share these rules:
- They fit an 80×24 terminal, draw each board cell two columns wide in
  printable ASCII, and never use color alone to tell objects apart.
- Boards have a fixed logical size: resizing the window only re-centers them.
- `Space` pauses (except on an end screen) and `Enter` on a game over or win
  screen starts a fresh run. `Q`/`Esc` return to the menu (or quit when the
  game was started directly) and `Ctrl+C` quits.
- Time only advances while the game is visible and running. Pauses, small
  windows, the menu and end screens never add time, and a single frame
  advances at most 100 ms.
- Scores are for the current run only; nothing is saved.
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
