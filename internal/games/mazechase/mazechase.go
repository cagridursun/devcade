// Package mazechase is DevCade's Maze Chase: clear an original fixed maze of
// pellets while four chasers with distinct, deterministic policies hunt you.
// Power pellets make the chasers vulnerable for a while.
//
// # Simulation order
//
// The game keeps an absolute gameplay clock. Every deadline (player step,
// chaser steps, chaser respawns, power expiry, grace expiries) is an absolute
// time, and Update(dt) processes the deadlines that fall within dt strictly in
// time order, one batch per distinct timestamp. Results therefore never
// depend on how elapsed time is split into frames. Within one batch at time t
// the order is:
//
//  1. Timers: removed chasers whose respawn time is t reappear at their spawn
//     with a one-second harmless grace. Power and grace windows are half-open
//     ([start, end)), so at their end time they are already over.
//  2. Player step (if due): apply the buffered direction when its neighbor is
//     open, otherwise keep the current direction; walls stop the player. The
//     pellet or power pellet on the new cell is collected, and a power pellet
//     starts (or refreshes) vulnerability immediately, before contacts.
//  3. Chaser steps (if due), in chaser order 1-4. Each chaser schedules its
//     next step 180 ms later, or 260 ms while vulnerability is active.
//  4. Contacts, once for the whole batch. A chaser touches the player when it
//     shares the player's cell, or when the two swapped cells in this batch
//     (crossed the same edge in opposite directions). Contacts are ignored
//     during the player's respawn grace and for chasers in their own grace.
//     While vulnerable every touching chaser is eaten (+200 each); otherwise
//     any number of touching chasers costs exactly one life.
//  5. Win check: the level is won when no collectible remains and the run is
//     still playing after contacts. So if the final pellet and a lethal
//     contact coincide, the life is lost first: with no lives left the run is
//     lost, otherwise the (respawned) player wins.
//
// Losing a life keeps score and collected pellets, puts every actor back on
// its spawn with no direction, cancels vulnerability and gives the player two
// seconds of harmless grace.
package mazechase

import (
	"fmt"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/gameui"
)

// Rules.
const (
	playerStep     = 140 * time.Millisecond
	chaserStep     = 180 * time.Millisecond
	vulnerableStep = 260 * time.Millisecond
	powerDuration  = 8 * time.Second
	eatenRespawn   = 2 * time.Second // removed time of an eaten chaser
	respawnGrace   = 1 * time.Second // harmless time after an eaten chaser returns
	lifeLossGrace  = 2 * time.Second // harmless time after the player loses a life

	startLives   = 3
	pelletPoints = 10
	powerPoints  = 50
	chaserPoints = 200

	aheadCells  = 4 // chaser 2 aims this many cells ahead of the player
	shyDistance = 8 // chaser 4 chases only while farther than this
)

type point struct{ x, y int }

func (p point) add(q point) point       { return point{p.x + q.x, p.y + q.y} }
func (p point) scale(n int) point       { return point{p.x * n, p.y * n} }
func (p point) neg() point              { return point{-p.x, -p.y} }
func (p point) String() string          { return fmt.Sprintf("%d,%d", p.x, p.y) }
func (p point) isZero() bool            { return p == point{} }
func open(p point) bool                 { return maze.open(p) }
func distances(p point) [Rows][Cols]int { return maze.distances(p) }

var (
	up    = point{0, -1}
	down  = point{0, 1}
	left  = point{-1, 0}
	right = point{1, 0}

	// dirOrder is the tie-break order for every movement decision.
	dirOrder = [4]point{up, left, down, right}
)

// Chaser policies and their fixed targets.
var (
	// patrolRoute is chaser 3's waypoint loop (the four power pellet corners).
	patrolRoute = [4]point{{1, 1}, {27, 1}, {27, 17}, {1, 17}}
	// shyCorner is where chaser 4 retreats when close to the player.
	shyCorner = point{1, 17}
)

type state uint8

const (
	playing state = iota
	lost
	won
)

type chaser struct {
	pos, dir   point
	next       time.Duration // time of the next step while present
	removed    bool          // eaten and waiting to respawn
	respawnAt  time.Duration
	graceUntil time.Duration // harmless before this time
	waypoint   int           // index into patrolRoute (chaser 3)
}

// Game implements engine.Game and engine.Finisher. The engine owns pause and
// undersized suspension; Game only sees time and input while it may advance.
type Game struct {
	items     [Rows][Cols]item
	remaining int // pellets plus power pellets left

	player     point
	dir        point // current direction, zero when standing still
	want       point // buffered desired direction, zero when none
	playerNext time.Duration

	chasers [numChasers]chaser

	now        time.Duration // gameplay clock
	powerUntil time.Duration // chasers are vulnerable while now < powerUntil
	graceUntil time.Duration // player contacts are ignored while now < graceUntil

	score, lives int
	state        state

	width, height int // screen size, for layout only
}

// New returns a ready-to-play game. Maze Chase is fully deterministic and
// needs no random source.
func New() engine.Game { return newGame() }

func newGame() *Game {
	g := &Game{}
	g.reset()
	return g
}

// reset starts a fresh run: full maze, three lives, actors on their spawns,
// clock at zero.
func (g *Game) reset() {
	*g = Game{width: g.width, height: g.height}
	g.items = maze.items
	g.remaining = maze.pellets + maze.powers
	g.lives = startLives
	g.resetActors()
}

// resetActors puts every actor on its spawn with no direction and schedules
// first steps relative to the current time.
func (g *Game) resetActors() {
	g.player, g.dir, g.want = maze.player, point{}, point{}
	g.playerNext = g.now + playerStep
	for i := range g.chasers {
		g.chasers[i] = chaser{pos: maze.chasers[i], next: g.now + chaserStep}
	}
}

func (g *Game) MinimumSize() (int, int) { return 80, 24 }

func (g *Game) Start(width, height int) {
	g.width, g.height = width, height
	g.reset()
}

// Resize changes only the screen layout; the maze is fixed.
func (g *Game) Resize(width, height int) { g.width, g.height = width, height }

// Finished reports whether the run has ended (all lives lost or maze cleared).
func (g *Game) Finished() bool { return g.state != playing }

// Score exposes the run's points to personal-best and leaderboard tracking.
func (g *Game) Score() int { return g.score }

func (g *Game) vulnerable() bool { return g.now < g.powerUntil }

func (g *Game) chaserInterval() time.Duration {
	if g.vulnerable() {
		return vulnerableStep
	}
	return chaserStep
}

// HandleInput buffers one desired direction while playing (a newer press
// replaces it) and restarts a finished run on KeySelect.
func (g *Game) HandleInput(k engine.Key) {
	if g.state != playing {
		if k == engine.KeySelect {
			g.reset()
		}
		return
	}
	switch k {
	case engine.KeyUp:
		g.want = up
	case engine.KeyDown:
		g.want = down
	case engine.KeyLeft:
		g.want = left
	case engine.KeyRight:
		g.want = right
	}
}

// Update processes every deadline up to now+dt in time order.
func (g *Game) Update(dt time.Duration) {
	if g.state != playing || dt <= 0 {
		return
	}
	end := g.now + dt
	for g.state == playing {
		t := g.nextEvent()
		if t > end {
			break
		}
		g.advance(t)
	}
	if g.state == playing {
		g.now = end
	}
}

// nextEvent returns the earliest pending deadline after now.
func (g *Game) nextEvent() time.Duration {
	t := g.playerNext
	consider := func(d time.Duration) {
		if d > g.now && d < t {
			t = d
		}
	}
	for i := range g.chasers {
		c := &g.chasers[i]
		if c.removed {
			consider(c.respawnAt)
		} else {
			consider(c.next)
		}
		consider(c.graceUntil)
	}
	consider(g.powerUntil)
	consider(g.graceUntil)
	return t
}

// advance runs one batch at time t in the order documented on the package.
func (g *Game) advance(t time.Duration) {
	g.now = t

	// 1. Timers.
	for i := range g.chasers {
		c := &g.chasers[i]
		if c.removed && c.respawnAt <= t {
			*c = chaser{pos: maze.chasers[i], next: t + g.chaserInterval(), graceUntil: t + respawnGrace}
		}
	}

	prevPlayer := g.player
	var prev [numChasers]point
	for i := range g.chasers {
		prev[i] = g.chasers[i].pos
	}

	// 2. Player.
	if g.playerNext <= t {
		g.stepPlayer()
		g.playerNext = t + playerStep
	}

	// 3. Chasers.
	for i := range g.chasers {
		c := &g.chasers[i]
		if !c.removed && c.next <= t {
			g.stepChaser(i)
			c.next = t + g.chaserInterval()
		}
	}

	// 4. Contacts.
	g.resolveContacts(prevPlayer, prev)

	// 5. Win.
	if g.state == playing && g.remaining == 0 {
		g.state = won
	}
}

func (g *Game) stepPlayer() {
	if !g.want.isZero() && open(g.player.add(g.want)) {
		g.dir, g.want = g.want, point{}
	}
	if g.dir.isZero() {
		return
	}
	next := g.player.add(g.dir)
	if !open(next) {
		return // walls stop the player; the direction is kept
	}
	g.player = next
	switch g.items[next.y][next.x] {
	case pellet:
		g.score += pelletPoints
	case power:
		g.score += powerPoints
		g.powerUntil = g.now + powerDuration // refreshes an active timer
	default:
		return
	}
	g.items[next.y][next.x] = none
	g.remaining--
}

// target returns chaser i's current goal cell. Every result is open floor:
//
//   - chaser 1: the player's cell (direct chase);
//   - chaser 2: aheadCells cells ahead of the player along its direction
//     (the player's cell when standing still), snapped to the nearest open
//     cell;
//   - chaser 3: the current patrol waypoint;
//   - chaser 4: the player while more than shyDistance steps away, otherwise
//     its corner (distance-dependent chase/patrol).
func (g *Game) target(i int) point {
	switch i {
	case 0:
		return g.player
	case 1:
		return maze.nearestOpen(g.player.add(g.dir.scale(aheadCells)))
	case 2:
		return patrolRoute[g.chasers[i].waypoint]
	default:
		c := g.chasers[i].pos
		if distances(g.player)[c.y][c.x] > shyDistance {
			return g.player
		}
		return shyCorner
	}
}

// stepChaser moves chaser i one cell. Candidates are the open neighbors
// except the reverse of its current direction (reversing only when nothing
// else is open). Normally it takes the candidate with the smallest walking
// distance to its target; while vulnerable it flees, taking the candidate
// with the largest walking distance from the player. Ties go to the first
// direction in dirOrder: up, left, down, right.
func (g *Game) stepChaser(i int) {
	c := &g.chasers[i]
	if i == 2 && c.pos == patrolRoute[c.waypoint] {
		c.waypoint = (c.waypoint + 1) % len(patrolRoute)
	}
	flee := g.vulnerable()
	var field [Rows][Cols]int
	if flee {
		field = distances(g.player)
	} else {
		field = distances(g.target(i))
	}
	best, bestD := point{}, 0
	for _, d := range dirOrder {
		n := c.pos.add(d)
		if !open(n) || (!c.dir.isZero() && d == c.dir.neg()) {
			continue
		}
		v := field[n.y][n.x]
		if best.isZero() || (flee && v > bestD) || (!flee && v < bestD) {
			best, bestD = d, v
		}
	}
	if best.isZero() {
		best = c.dir.neg() // dead end: turn back
	}
	if best.isZero() || !open(c.pos.add(best)) {
		return
	}
	c.dir = best
	c.pos = c.pos.add(best)
}

func (g *Game) resolveContacts(prevPlayer point, prev [numChasers]point) {
	if g.now < g.graceUntil {
		return
	}
	vulnerable, lethal := g.vulnerable(), false
	moved := g.player != prevPlayer
	for i := range g.chasers {
		c := &g.chasers[i]
		if c.removed || g.now < c.graceUntil {
			continue
		}
		same := c.pos == g.player
		swapped := moved && c.pos == prevPlayer && prev[i] == g.player
		if !same && !swapped {
			continue
		}
		if vulnerable {
			g.score += chaserPoints
			*c = chaser{pos: maze.chasers[i], removed: true, respawnAt: g.now + eatenRespawn}
		} else {
			lethal = true
		}
	}
	if lethal {
		g.loseLife()
	}
}

func (g *Game) loseLife() {
	g.lives--
	if g.lives <= 0 {
		g.lives = 0
		g.state = lost
		return
	}
	g.powerUntil = 0
	g.graceUntil = g.now + lifeLossGrace
	g.resetActors()
}

// Layout: HUD (2 rows), maze (Rows rows, two columns per cell) and a legend
// row, centered in the screen minus the last row (the navigation footer).
const (
	blockW = 78
	boardW = Cols * 2
	hudH   = 2
	blockH = hudH + Rows + 1
)

func (g *Game) Render(c engine.Canvas) {
	w, h := c.Size()
	const frameW = 78
	const frameH = Rows + 2
	x0 := max(0, (w-frameW)/2)
	y0 := max(0, (h-1-(2+frameH))/2)

	status := "PLAYING"
	switch g.state {
	case lost:
		status = "GAME OVER"
	case won:
		status = "YOU WIN"
	}
	powerText := "Power  -- "
	if g.vulnerable() {
		powerText = engine.Format(c, "Power %4.1fs", (g.powerUntil-g.now).Seconds())
	}

	c.Text(x0, y0, engine.Format(c, "> MAZE CHASE   Score %d   Lives %d   %s   Left %d   %s",
		g.score, g.lives, powerText, g.remaining, engine.Format(c, status)), engine.Accent)
	c.Text(x0, y0+1, "Move: arrows / WASD   Pause: Space   Leave: Q / Esc   Exit: Ctrl+C", engine.Muted)

	fx, fy := x0, y0+2
	gameui.Box(c, fx, fy, frameW, frameH, engine.Border)
	gameui.DotGrid(c, fx, fy, frameW, frameH, 2)

	bx, by := fx+(frameW-boardW)/2, fy+1
	cell := func(p point, s string, color engine.Color) {
		c.Text(bx+p.x*2, by+p.y, s, color)
	}
	for y := range Rows {
		for x := range Cols {
			p := point{x, y}
			switch {
			case maze.wall[y][x]:
				cell(p, "##", engine.Border)
			case g.items[y][x] == pellet:
				cell(p, " .", engine.Muted)
			case g.items[y][x] == power:
				cell(p, "()", engine.Warning)
			}
		}
	}
	for i := range g.chasers {
		ch := &g.chasers[i]
		if ch.removed {
			continue
		}
		digit := string(rune('1' + i))
		switch {
		case g.now < ch.graceUntil:
			cell(ch.pos, "~"+digit, engine.Muted)
		case g.vulnerable():
			cell(ch.pos, "c"+digit, engine.Accent)
		default:
			cell(ch.pos, "C"+digit, engine.Danger)
		}
	}
	cell(g.player, "@@", engine.Player)

	legend := "@@ you   C1-C4 chasers   c vulnerable   ~ harmless   () power"
	c.Text(fx+2, fy+frameH-1, legend, engine.Muted)

	if g.state != playing {
		title := "GAME OVER"
		if g.state == won {
			title = "MAZE CLEARED - YOU WIN"
		}
		box := []string{"", title, engine.Format(c, "Final score %d", g.score), "Enter: play again", ""}
		top := by + (Rows-len(box))/2
		for i, line := range box {
			c.Text(bx+(boardW-32)/2, top+i, "  "+center(engine.Format(c, line), 28)+"  ", engine.Warning)
		}
	}
}

func center(s string, width int) string {
	pad := max(0, width-len([]rune(s)))
	return fmt.Sprintf("%*s%s%*s", pad/2, "", s, pad-pad/2, "")
}
