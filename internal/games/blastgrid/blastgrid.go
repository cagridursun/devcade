// Package blastgrid is DevCade's Blast Grid: a solo bomb arena on a fixed
// 17x13 grid. Place bombs to break crates and catch the three bots in the
// blast, without getting caught yourself.
//
// All timing is gameplay time delivered through Update. Fuses, flames, the
// player's movement cooldown and bot decisions are absolute deadlines on one
// clock and are processed in time order, so how elapsed time is split into
// frames never changes the outcome.
package blastgrid

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/gameui"
)

// Arena and rules.
const (
	Cols = 17
	Rows = 13

	bombRadius   = 3
	fuseTime     = 2 * time.Second
	flameTime    = 500 * time.Millisecond
	moveCooldown = 120 * time.Millisecond // player: one cell per press
	botInterval  = 180 * time.Millisecond // bots: one decision per tick

	crateScore = 10
	botScore   = 100

	numBots   = 3
	numActors = 1 + numBots
	player    = 0 // actor index; bots are 1..numBots
	noActor   = -1
)

// layout is the authored arena. '#' wall, 'x' crate, '.' floor, 'P' player
// spawn, '1'-'3' bot spawns. Every corner spawn keeps an L of floor plus two
// side pockets (for example 3,2 and 2,3 for the top-left corner) that are off
// the spawn's blast lines, so a bomb dropped on the spawn can always be
// escaped. Pillars sit on every even/even cell, so every non-wall cell is
// connected once crates are cleared. TestArenaIsValid checks all of this.
var layout = [Rows]string{
	"#################",
	"#P..xxxxxxxxx..1#",
	"#.#.#x#x#x#x#.#.#",
	"#..xxx.xxx.xxx..#",
	"#x#x#x#.#.#x#x#x#",
	"#xxx.xxxxxxx.xxx#",
	"#x#.#x#.#.#x#.#x#",
	"#xxx.xxxxxxx.xxx#",
	"#x#x#x#.#.#x#x#x#",
	"#..xxx.xxx.xxx..#",
	"#.#.#x#x#x#x#.#.#",
	"#2..xxxxxxxxx..3#",
	"#################",
}

type point struct{ x, y int }

func (p point) add(q point) point { return point{p.x + q.x, p.y + q.y} }

func inside(p point) bool { return p.x >= 0 && p.x < Cols && p.y >= 0 && p.y < Rows }

var (
	up    = point{0, -1}
	down  = point{0, 1}
	left  = point{-1, 0}
	right = point{1, 0}
	dirs  = [4]point{up, right, down, left}
)

type tile uint8

const (
	floor tile = iota
	wall
	crate
)

type actor struct {
	pos   point
	alive bool
}

// bomb is one placed bomb. holder is the actor still standing on the bomb
// since placing it (noActor once it has stepped off). Bomb cells are never
// enterable, so the placer may stay and leave once but can never come back
// while the bomb exists.
type bomb struct {
	pos       point
	owner     int
	seq       int // placement order: lower seq wins blast attribution
	explodeAt time.Duration
	holder    int
}

type state uint8

const (
	playing state = iota
	lost
	won
)

// Game implements engine.Game and engine.Finisher. The engine owns pause and
// undersized suspension; Game only sees time and input while it may advance.
type Game struct {
	seed *rand.Rand // injected source; seeds one fresh bot RNG per run
	rng  *rand.Rand // this run's bot RNG

	grid       [Rows][Cols]tile
	actors     [numActors]actor
	bombs      []*bomb // placement order
	nextSeq    int
	flameUntil [Rows][Cols]time.Duration // a cell burns while now < flameUntil
	flameOwner [Rows][Cols]int           // owner of the credited bomb
	now        time.Duration             // gameplay clock of this run
	nextMove   time.Duration             // player movement cooldown deadline
	nextThink  time.Duration             // next bot decision tick
	score      int
	state      state

	width, height int // screen size, for layout only
}

// New returns a ready-to-play game with its own random source.
func New() engine.Game {
	return newGame(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
}

func newGame(seed *rand.Rand) *Game {
	g := &Game{seed: seed}
	g.reset()
	return g
}

// reset rebuilds the arena, actors, bombs, flames, clocks, score and the bot
// RNG for a fresh run.
func (g *Game) reset() {
	g.rng = rand.New(rand.NewPCG(g.seed.Uint64(), g.seed.Uint64()))
	g.load(layout)
	g.bombs = nil
	g.nextSeq = 0
	g.flameUntil = [Rows][Cols]time.Duration{}
	g.flameOwner = [Rows][Cols]int{}
	g.now = 0
	g.nextMove = 0
	g.nextThink = botInterval
	g.score = 0
	g.state = playing
}

// load sets the grid and actor spawns from an arena map in layout's format.
func (g *Game) load(rows [Rows]string) {
	for y, row := range rows {
		for x, ch := range row {
			t := floor
			switch ch {
			case '#':
				t = wall
			case 'x':
				t = crate
			case 'P':
				g.actors[player] = actor{pos: point{x, y}, alive: true}
			case '1', '2', '3':
				g.actors[ch-'0'] = actor{pos: point{x, y}, alive: true}
			}
			g.grid[y][x] = t
		}
	}
}

func (g *Game) MinimumSize() (int, int) { return 80, 24 }

func (g *Game) Start(width, height int) {
	g.width, g.height = width, height
	g.reset()
}

// Resize changes only the screen layout; the arena is fixed.
func (g *Game) Resize(width, height int) { g.width, g.height = width, height }

// Finished reports whether the run has ended (won or lost).
func (g *Game) Finished() bool { return g.state != playing }

// Score is 10 per crate and 100 per bot destroyed by the player's bombs.
func (g *Game) Score() int { return g.score }

// BotsLeft is the number of living bots.
func (g *Game) BotsLeft() int {
	n := 0
	for i := 1; i < numActors; i++ {
		if g.actors[i].alive {
			n++
		}
	}
	return n
}

// HandleInput moves the player one cell per press (subject to the movement
// cooldown), places a bomb on KeyAction, and restarts a finished run on
// KeySelect.
func (g *Game) HandleInput(k engine.Key) {
	if g.state != playing {
		if k == engine.KeySelect {
			g.reset()
		}
		return
	}
	var d point
	switch k {
	case engine.KeyUp:
		d = up
	case engine.KeyDown:
		d = down
	case engine.KeyLeft:
		d = left
	case engine.KeyRight:
		d = right
	case engine.KeyAction:
		g.placeBomb(player)
		return
	default:
		return
	}
	if g.now < g.nextMove {
		return // presses during the cooldown are ignored, not queued
	}
	if g.move(player, d) {
		g.nextMove = g.now + moveCooldown
		g.checkEnd()
	}
}

// Update advances gameplay time, handling every fuse and bot tick at its
// exact timestamp. At one timestamp, explosions resolve before bots decide.
// Flames need no event: a cell burns while now < flameUntil.
func (g *Game) Update(dt time.Duration) {
	if g.state != playing || dt <= 0 {
		return
	}
	end := g.now + dt
	for g.state == playing {
		t := g.nextThink
		for _, b := range g.bombs {
			t = min(t, b.explodeAt)
		}
		if t > end {
			break
		}
		g.now = t
		g.explodeDue()
		g.checkEnd()
		if g.state == playing && g.nextThink == t {
			for i := 1; i < numActors && g.state == playing; i++ {
				if g.actors[i].alive {
					g.think(i)
					g.checkEnd()
				}
			}
			g.nextThink += botInterval
		}
	}
	if g.state == playing {
		g.now = end
	}
}

// checkEnd applies win/loss precedence: a dead player always loses, even if
// the last bot died at the same timestamp.
func (g *Game) checkEnd() {
	switch {
	case !g.actors[player].alive:
		g.state = lost
	case g.BotsLeft() == 0:
		g.state = won
	}
}

func (g *Game) bombAt(p point) *bomb {
	for _, b := range g.bombs {
		if b.pos == p {
			return b
		}
	}
	return nil
}

func (g *Game) hasBomb(i int) bool {
	for _, b := range g.bombs {
		if b.owner == i {
			return true
		}
	}
	return false
}

func (g *Game) actorAt(p point) int {
	for i, a := range g.actors {
		if a.alive && a.pos == p {
			return i
		}
	}
	return noActor
}

// walkable reports whether actor i may step onto p: floor, no bomb (extra is
// an optional hypothetical bomb) and no other living actor.
func (g *Game) walkable(p point, i int, extra *bomb) bool {
	if !inside(p) || g.grid[p.y][p.x] != floor || g.bombAt(p) != nil {
		return false
	}
	if extra != nil && extra.pos == p {
		return false
	}
	other := g.actorAt(p)
	return other == noActor || other == i
}

// placeBomb drops actor i's single bomb on its cell, if it has none active
// and the cell is free of bombs. It reports whether a bomb was placed.
func (g *Game) placeBomb(i int) bool {
	a := &g.actors[i]
	if !a.alive || g.hasBomb(i) || g.bombAt(a.pos) != nil {
		return false
	}
	g.bombs = append(g.bombs, &bomb{pos: a.pos, owner: i, seq: g.nextSeq, explodeAt: g.now + fuseTime, holder: i})
	g.nextSeq++
	return true
}

// move steps actor i one cell in direction d if the target is walkable.
// Stepping into a burning cell kills the actor, credited to the flame owner.
func (g *Game) move(i int, d point) bool {
	a := &g.actors[i]
	next := a.pos.add(d)
	if !a.alive || !g.walkable(next, i, nil) {
		return false
	}
	for _, b := range g.bombs {
		if b.holder == i {
			b.holder = noActor // left the bomb: it can never be re-entered
		}
	}
	a.pos = next
	if g.now < g.flameUntil[next.y][next.x] {
		g.kill(i, g.flameOwner[next.y][next.x])
	}
	return true
}

func (g *Game) kill(i, by int) {
	g.actors[i].alive = false
	for _, b := range g.bombs {
		if b.holder == i {
			b.holder = noActor
		}
	}
	if i != player && by == player {
		g.score += botScore
	}
}

// trace returns the cells a blast at origin covers (origin first) and the
// bomb cells its rays reach. Rays stop before walls, include the first crate
// and stop, and include the first bomb cell and stop. isBomb decides which
// cells hold bombs; crates come from the grid as it is when trace is called.
func (g *Game) trace(origin point, isBomb func(point) bool) (cells, bombs []point) {
	cells = append(cells, origin)
	for _, d := range dirs {
		p := origin
		for range bombRadius {
			p = p.add(d)
			if !inside(p) || g.grid[p.y][p.x] == wall {
				break
			}
			cells = append(cells, p)
			if g.grid[p.y][p.x] == crate {
				break
			}
			if isBomb(p) {
				bombs = append(bombs, p)
				break
			}
		}
	}
	return cells, bombs
}

// explodeDue resolves every bomb whose fuse ends now, plus the chain it
// triggers, as one simultaneous event:
//
//   - A work queue seeded with the due bombs (placement order) and a visited
//     set ensure each bomb explodes once, so capacity is restored once.
//   - All rays are traced against the arena as it was at the start of this
//     timestamp: crates are destroyed and bombs removed only after every ray
//     has been traced, so iteration order cannot make a ray travel farther.
//   - Every flamed cell is credited to the covering bomb with the lowest
//     placement seq (the earliest placed). That bomb's owner gets the crate
//     or bot score for the cell, so each crate and actor scores at most once.
func (g *Game) explodeDue() {
	visited := map[*bomb]bool{}
	var queue []*bomb
	for _, b := range g.bombs {
		if b.explodeAt <= g.now {
			queue = append(queue, b)
			visited[b] = true
		}
	}
	if len(queue) == 0 {
		return
	}
	isBomb := func(p point) bool { return g.bombAt(p) != nil }
	var credit [Rows][Cols]*bomb
	for i := 0; i < len(queue); i++ {
		b := queue[i]
		cells, hits := g.trace(b.pos, isBomb)
		for _, c := range cells {
			if cur := credit[c.y][c.x]; cur == nil || b.seq < cur.seq {
				credit[c.y][c.x] = b
			}
		}
		for _, h := range hits {
			if hb := g.bombAt(h); !visited[hb] {
				visited[hb] = true
				queue = append(queue, hb)
			}
		}
	}

	kept := make([]*bomb, 0, len(g.bombs))
	for _, b := range g.bombs {
		if !visited[b] {
			kept = append(kept, b)
		}
	}
	g.bombs = kept
	for y := range Rows {
		for x := range Cols {
			b := credit[y][x]
			if b == nil {
				continue
			}
			g.flameUntil[y][x] = g.now + flameTime
			g.flameOwner[y][x] = b.owner
			if g.grid[y][x] == crate {
				g.grid[y][x] = floor
				if b.owner == player {
					g.score += crateScore
				}
			}
		}
	}
	for i, a := range g.actors {
		if b := credit[a.pos.y][a.pos.x]; a.alive && b != nil {
			g.kill(i, b.owner)
		}
	}
}

// Layout: two HUD rows, the arena at two terminal columns per cell, and a
// legend row, centered in the screen minus the last row (the footer).
const (
	arenaW = Cols * 2
	hudH   = 2
	blockH = hudH + Rows + 1
)

func (g *Game) Render(c engine.Canvas) {
	w, h := c.Size()
	const (
		frameW = 78
		frameH = 18
	)
	x0 := max(0, (w-frameW)/2)
	y0 := max(0, (h-1-(2+frameH+1))/2)

	status := "PLAYING"
	switch g.state {
	case lost:
		status = "GAME OVER"
	case won:
		status = "YOU WIN"
	}
	bombState := "ready"
	if g.hasBomb(player) {
		bombState = "armed"
	}

	c.Text(x0, y0, engine.Format(c, "> BLAST GRID   Score %04d   Bots %d   Bomb %s   %s",
		g.score, g.BotsLeft(), engine.Format(c, bombState), engine.Format(c, status)), engine.Accent)
	c.Text(x0, y0+1, "Move: arrows / WASD   Bomb: Z   Pause: Space   Leave: Q / Esc   Exit: Ctrl+C", engine.Muted)

	fx, fy := x0, y0+2
	gameui.Box(c, fx, fy, frameW, frameH, engine.Border)
	gameui.DotGrid(c, fx, fy, frameW, frameH, 2)

	// The 17x13 simulation grid is projected across the complete arcade frame.
	// The authored outer wall is the frame itself; interior tiles are enlarged
	// into readable arcade sprites instead of the old compact ##/[] map.
	const (
		innerCols = Cols - 2
		innerRows = Rows - 2
	)
	innerX, innerY := fx+1, fy+1
	innerW, innerH := frameW-2, frameH-2
	left := func(x int) int { return innerX + (x-1)*innerW/innerCols }
	right := func(x int) int { return innerX + x*innerW/innerCols }
	top := func(y int) int { return innerY + (y-1)*innerH/innerRows }
	bottom := func(y int) int { return innerY + y*innerH/innerRows }
	centerX := func(x int) int {
		l, r := left(x), right(x)
		return l + max(0, (r-l-1)/2)
	}
	centerY := func(y int) int {
		t, b := top(y), bottom(y)
		return t + max(0, (b-t-1)/2)
	}

	drawTile := func(x, y int, glyph rune, color engine.Color) {
		l, r := left(x), right(x)
		width := min(3, max(1, r-l))
		start := l + max(0, (r-l-width)/2)
		for px := start; px < start+width; px++ {
			c.Cell(px, centerY(y), glyph, color)
		}
	}
	for y := 1; y < Rows-1; y++ {
		for x := 1; x < Cols-1; x++ {
			switch g.grid[y][x] {
			case wall:
				drawTile(x, y, '█', engine.Border)
			case crate:
				drawTile(x, y, '▓', engine.Accent)
			}
			if g.now < g.flameUntil[y][x] {
				drawTile(x, y, '✱', engine.Danger)
			}
		}
	}
	for _, b := range g.bombs {
		c.Cell(centerX(b.pos.x), centerY(b.pos.y), '●', engine.Warning)
	}
	for i, a := range g.actors {
		if !a.alive {
			continue
		}
		if i == player {
			glyph := '▲'
			if g.bombAt(a.pos) != nil {
				glyph = '▣'
			}
			c.Cell(centerX(a.pos.x), centerY(a.pos.y), glyph, engine.Player)
			continue
		}
		label := engine.Format(c, "B%d", i)
		x := centerX(a.pos.x) - len([]rune(label))/2
		c.Text(x, centerY(a.pos.y), label, engine.Danger)
	}

	c.Text(x0, y0+2+frameH, "▲ you   B1-B3 bots   ● bomb   ✱ flame   ▓ crate   █ wall", engine.Muted)

	if g.state != playing {
		title := "GAME OVER"
		if g.state == won {
			title = "YOU WIN"
		}
		box := []string{"", title, engine.Format(c, "Final score %d   Bots left %d", g.score, g.BotsLeft()), "Enter: play again", ""}
		top := fy + (frameH-len(box))/2
		left := fx + (frameW-34)/2
		for i, s := range box {
			c.Text(left, top+i, "  "+center(engine.Format(c, s), 30)+"  ", engine.Warning)
		}
	}
}

func center(s string, width int) string {
	pad := max(0, width-len([]rune(s)))
	return fmt.Sprintf("%*s%s%*s", pad/2, "", s, pad-pad/2, "")
}
