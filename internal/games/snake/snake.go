// Package snake is DevCade's Snake: steer a growing snake around a fixed
// 36x18 board, eat food to score and speed up, and avoid walls and yourself.
package snake

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

// Board and rules.
const (
	Cols = 36
	Rows = 18

	startLength   = 3
	baseInterval  = 180 * time.Millisecond
	intervalStep  = 15 * time.Millisecond
	minInterval   = 80 * time.Millisecond
	pointsPerFood = 10
	foodsPerLevel = 5
	maxQueued     = 2 // pending turns between movement steps
)

type point struct{ x, y int }

func (p point) add(q point) point { return point{p.x + q.x, p.y + q.y} }
func (p point) opposite(q point) bool {
	return p.x == -q.x && p.y == -q.y
}

var (
	up    = point{0, -1}
	down  = point{0, 1}
	left  = point{-1, 0}
	right = point{1, 0}
)

type state uint8

const (
	playing state = iota
	lost
	won // the snake fills the whole board
)

// Game implements engine.Game and engine.Finisher. The engine owns pause and
// undersized suspension; Game only sees time and input while it may advance.
type Game struct {
	rng *rand.Rand

	body    []point // head first
	dir     point   // committed direction of the last step
	queue   [maxQueued]point
	queued  int
	food    point
	eaten   int
	elapsed time.Duration // movement time not yet spent on a step
	state   state

	width, height int // screen size, for layout only
}

// New returns a ready-to-play game with its own random source.
func New() engine.Game {
	return newGame(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
}

func newGame(rng *rand.Rand) *Game {
	g := &Game{rng: rng}
	g.reset()
	return g
}

// reset starts a fresh run: centered three-cell snake heading right, no
// score, no pending turns, no leftover movement time.
func (g *Game) reset() {
	head := point{Cols / 2, Rows / 2}
	g.body = g.body[:0]
	for i := range startLength {
		g.body = append(g.body, point{head.x - i, head.y})
	}
	g.dir = right
	g.queued = 0
	g.eaten = 0
	g.elapsed = 0
	g.state = playing
	g.spawnFood()
}

func (g *Game) MinimumSize() (int, int) { return 80, 24 }

func (g *Game) Start(width, height int) {
	g.width, g.height = width, height
	g.reset()
}

// Resize changes only the screen layout; the board is fixed.
func (g *Game) Resize(width, height int) { g.width, g.height = width, height }

// Finished reports whether the run has ended (lost or board completed).
func (g *Game) Finished() bool { return g.state != playing }

// Score is 10 points per food eaten in the current run.
func (g *Game) Score() int { return g.eaten * pointsPerFood }

// Level rises by one every five foods.
func (g *Game) Level() int { return 1 + g.eaten/foodsPerLevel }

// interval is the time per movement step at the current level.
func (g *Game) interval() time.Duration {
	return max(minInterval, baseInterval-intervalStep*time.Duration(g.Level()-1))
}

// HandleInput queues direction changes while playing and restarts a finished
// run on KeySelect.
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
	default:
		return
	}
	// Validate against the last direction the snake will be moving in, so
	// Right -> Up -> Left between two steps is Up then Left, never a reversal.
	last := g.dir
	if g.queued > 0 {
		last = g.queue[g.queued-1]
	}
	if g.queued == maxQueued || d == last || d.opposite(last) {
		return
	}
	g.queue[g.queued] = d
	g.queued++
}

// Update spends elapsed time on whole movement steps, keeping the remainder.
// The interval is re-read every step, so a level-up speeds up the next step.
func (g *Game) Update(dt time.Duration) {
	if g.state != playing || dt <= 0 {
		return
	}
	g.elapsed += dt
	for g.state == playing && g.elapsed >= g.interval() {
		g.elapsed -= g.interval()
		g.step()
	}
}

// step moves the snake one cell: apply one queued turn, find the next head,
// decide whether it eats, check collisions against the cells that stay
// occupied, then commit the move and score.
func (g *Game) step() {
	if g.queued > 0 {
		g.dir = g.queue[0]
		g.queue[0] = g.queue[1]
		g.queued--
	}
	next := g.body[0].add(g.dir)
	if next.x < 0 || next.x >= Cols || next.y < 0 || next.y >= Rows {
		g.state = lost
		return
	}
	grow := next == g.food
	occupied := g.body
	if !grow {
		occupied = g.body[:len(g.body)-1] // the tail moves away this step
	}
	for _, c := range occupied {
		if c == next {
			g.state = lost
			return
		}
	}
	if grow {
		g.body = append(g.body, point{})
	}
	copy(g.body[1:], g.body[:len(g.body)-1])
	g.body[0] = next
	if grow {
		g.eaten++
		g.spawnFood()
	}
}

// spawnFood places food on a uniformly chosen free cell, or completes the
// run when the snake fills the board.
func (g *Game) spawnFood() {
	var occupied [Cols * Rows]bool
	for _, c := range g.body {
		occupied[c.y*Cols+c.x] = true
	}
	free := make([]point, 0, Cols*Rows-len(g.body))
	for i, taken := range occupied {
		if !taken {
			free = append(free, point{i % Cols, i / Cols})
		}
	}
	if len(free) == 0 {
		g.state = won
		return
	}
	g.food = free[g.rng.IntN(len(free))]
}

// Layout: two HUD rows above a bordered board of two terminal columns per
// cell, centered in the screen minus the last row (the navigation footer).
const (
	boardW = Cols*2 + 2
	boardH = Rows + 2
	hudH   = 2
)

func (g *Game) Render(c engine.Canvas) {
	w, h := c.Size()
	x0 := max(0, (w-boardW)/2)
	y0 := max(0, (h-1-(hudH+boardH))/2)

	status := "PLAYING"
	switch g.state {
	case lost:
		status = "GAME OVER"
	case won:
		status = "BOARD COMPLETE"
	}
	c.Text(x0, y0, engine.Format(c, "SNAKE   Score %-5d Level %-3d Length %-4d %s", g.Score(), g.Level(), len(g.body), engine.Format(c, status)), engine.Accent)
	c.Text(x0, y0+1, "Move: arrows / WASD   Pause: Space   Leave: Q / Esc   Exit: Ctrl+C", engine.Default)

	bx, by := x0, y0+hudH
	for x := 1; x < boardW-1; x++ {
		c.Cell(bx+x, by, '-', engine.Default)
		c.Cell(bx+x, by+boardH-1, '-', engine.Default)
	}
	for y := 0; y < boardH; y++ {
		glyph := '|'
		if y == 0 || y == boardH-1 {
			glyph = '+'
		}
		c.Cell(bx, by+y, glyph, engine.Default)
		c.Cell(bx+boardW-1, by+y, glyph, engine.Default)
	}
	cell := func(p point, s string, color engine.Color) {
		c.Text(bx+1+p.x*2, by+1+p.y, s, color)
	}
	if g.state == playing {
		cell(g.food, "**", engine.Warning)
	}
	for i := len(g.body) - 1; i >= 1; i-- {
		cell(g.body[i], "oo", engine.Player)
	}
	cell(g.body[0], "@@", engine.Player)

	if g.state != playing {
		title := "GAME OVER"
		if g.state == won {
			title = "BOARD COMPLETE - YOU WIN"
		}
		// A blank-padded box centered on the board, above the footer row.
		box := []string{"", title, engine.Format(c, "Final score %d   Level %d", g.Score(), g.Level()), "Enter: play again", ""}
		top := by + (boardH-len(box))/2
		for i, line := range box {
			c.Text(bx+(boardW-32)/2, top+i, "  "+center(engine.Format(c, line), 28)+"  ", engine.Warning)
		}
	}
}

func center(s string, width int) string {
	pad := max(0, width-len([]rune(s)))
	return fmt.Sprintf("%*s%s%*s", pad/2, "", s, pad-pad/2, "")
}
