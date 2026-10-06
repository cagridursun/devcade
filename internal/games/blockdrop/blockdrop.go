// Package blockdrop is DevCade's Block Drop: steer falling four-cell pieces
// on a 10x20 board, complete rows to clear them, and keep the stack from
// reaching the top.
//
// Coordinates use x to the right and y downwards. The board has two hidden
// spawn rows (y 0 and 1) above the twenty visible rows (y 2..21); hidden rows
// take part in every collision check and in the top-out rules.
package blockdrop

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/gameui"
)

// Board and rules.
const (
	Cols        = 10
	VisibleRows = 20
	HiddenRows  = 2
	Rows        = HiddenRows + VisibleRows // logical rows, hidden ones first

	baseInterval = 700 * time.Millisecond
	intervalStep = 50 * time.Millisecond
	minInterval  = 100 * time.Millisecond
	lockDelay    = 400 * time.Millisecond
	maxResets    = 8 // lock-delay resets per piece
	linesPerLvl  = 10
)

// lineScores is indexed by the number of rows cleared at once and is
// multiplied by the level in effect before the clear.
var lineScores = [5]int{0, 100, 300, 500, 800}

type point struct{ x, y int }

// kind identifies a piece shape; none marks an empty board cell.
type kind uint8

const (
	none kind = iota
	pieceI
	pieceO
	pieceT
	pieceS
	pieceZ
	pieceJ
	pieceL
	kinds // number of kinds including none
)

// base is rotation state 0 of every piece inside its n x n box.
//
//	I (4x4)   O (2x2)   T (3x3)   S (3x3)   Z (3x3)   J (3x3)   L (3x3)
//	....      []        .[].      .[][]     [][].     []..      ..[]
//	[][][][]  [][]      [][][]    [][].     .[][]     [][][]    [][][]
//	....
//	....
//
// Rotation states 1, 2 and 3 are 90, 180 and 270 degrees clockwise. One
// clockwise quarter turn maps a box cell (x, y) to (n-1-y, x), so T, S, Z,
// J and L turn about their box center (1, 1) and I about (1.5, 1.5). O's
// 2x2 box maps onto itself, and Block Drop treats O rotation as a no-op.
// The states match the familiar orientations; the kicks below are DevCade's
// own small policy, not a guideline rotation system.
var base = [kinds]struct {
	n     int
	cells [4]point
}{
	pieceI: {4, [4]point{{0, 1}, {1, 1}, {2, 1}, {3, 1}}},
	pieceO: {2, [4]point{{0, 0}, {1, 0}, {0, 1}, {1, 1}}},
	pieceT: {3, [4]point{{1, 0}, {0, 1}, {1, 1}, {2, 1}}},
	pieceS: {3, [4]point{{1, 0}, {2, 0}, {0, 1}, {1, 1}}},
	pieceZ: {3, [4]point{{0, 0}, {1, 0}, {1, 1}, {2, 1}}},
	pieceJ: {3, [4]point{{0, 0}, {0, 1}, {1, 1}, {2, 1}}},
	pieceL: {3, [4]point{{2, 0}, {0, 1}, {1, 1}, {2, 1}}},
}

// shapes[k][r] lists the box cells of kind k in rotation state r.
var shapes [kinds][4][4]point

func init() {
	for k := pieceI; k < kinds; k++ {
		cells, n := base[k].cells, base[k].n
		for r := range 4 {
			shapes[k][r] = cells
			for i, p := range cells {
				cells[i] = point{n - 1 - p.y, p.x}
			}
		}
	}
}

// kicks are the offsets tried, in order, after a rotation: stay, one column
// left, one right, two left, two right, then one row up (x right, y down).
var kicks = [...]point{{0, 0}, {-1, 0}, {1, 0}, {-2, 0}, {2, 0}, {0, -1}}

// piece is a shape in a rotation state whose box's top-left corner is at
// board position (x, y).
type piece struct {
	kind kind
	rot  int
	x, y int
}

// spawnPiece places k in state 0 with its box at column 3 (O: column 4) and
// its top cells on y = 2, the top visible row, so a new piece is always
// fully visible; the hidden rows above are headroom for rotations and the
// upward kick. Spawned cells (x, y) are:
//
//	I (3..6,2)   O (4..5,2..3)   T (4,2)(3..5,3)   S (4..5,2)(3..4,3)
//	Z (3..4,2)(4..5,3)   J (3,2)(3..5,3)   L (5,2)(3..5,3)
func spawnPiece(k kind) piece {
	switch k {
	case pieceI:
		return piece{kind: k, x: 3, y: 1} // I's state-0 cells sit on box row 1
	case pieceO:
		return piece{kind: k, x: 4, y: 2}
	}
	return piece{kind: k, x: 3, y: 2}
}

func (p piece) cells() [4]point {
	cells := shapes[p.kind][p.rot]
	for i := range cells {
		cells[i].x += p.x
		cells[i].y += p.y
	}
	return cells
}

type state uint8

const (
	playing state = iota
	lost
)

// Game implements engine.Game and engine.Finisher. The engine owns pause and
// undersized suspension; Game only sees time and input while it may advance.
//
// Timing model: while the current piece is airborne, fall accumulates toward
// the next gravity step. While it is grounded (it cannot move down), gravity
// is held at zero and lockTime accumulates toward the 400 ms lock delay.
// Update walks both deadlines in time order, so how elapsed time is split
// into frames never changes the outcome.
//
// Lock delay rules, per piece (all reset on spawn):
//   - a successful move or rotation of a grounded piece restarts the lock
//     delay, at most maxResets times; failed actions change nothing;
//   - leaving support clears the grounded time but never refunds resets;
//   - the game tracks the deepest row (the piece's lowest cell) at which the
//     piece has been grounded. Once the budget is spent, a touchdown strictly
//     below that row grants a fresh 400 ms window (still without resets),
//     while a touchdown on the same or a higher row locks at once. Rows are
//     bounded, so moves and up-kicks cannot postpone the lock forever, yet a
//     piece that slides into a deep well still gets a normal lock delay.
type Game struct {
	rng *rand.Rand

	board    [Rows][Cols]kind
	cur      piece
	next     kind
	bag      [7]kind // the current seven-piece bag
	bagLeft  int     // undrawn pieces at the end of bag
	score    int
	lines    int
	fall     time.Duration // airborne time not yet spent on a gravity step
	lockTime time.Duration // grounded time toward the lock delay
	resets   int           // lock-delay resets used by the current piece
	airborne bool          // the piece was airborne at its last position change
	deepest  int           // lowest cell row at which the piece was grounded, -1 if never
	lockNow  bool          // exhausted touchdown at or above deepest: lock immediately
	state    state

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

// reset starts a fresh run: empty board, no score or lines, a new bag and
// new per-piece timers.
func (g *Game) reset() {
	g.board = [Rows][Cols]kind{}
	g.score, g.lines = 0, 0
	g.bagLeft = 0
	g.state = playing
	g.next = g.draw()
	g.spawn()
}

func (g *Game) MinimumSize() (int, int) { return 80, 24 }

func (g *Game) Start(width, height int) {
	g.width, g.height = width, height
	g.reset()
}

// Resize changes only the screen layout; the board is fixed.
func (g *Game) Resize(width, height int) { g.width, g.height = width, height }

// Finished reports whether the run has ended.
func (g *Game) Finished() bool { return g.state != playing }

// Score is the run's total: soft/hard drop rows and line clears.
func (g *Game) Score() int { return g.score }

// Lines is the number of rows cleared in the run.
func (g *Game) Lines() int { return g.lines }

// Level rises by one every ten cleared lines.
func (g *Game) Level() int { return 1 + g.lines/linesPerLvl }

// interval is the gravity step time at the current level.
func (g *Game) interval() time.Duration {
	return max(minInterval, baseInterval-intervalStep*time.Duration(g.Level()-1))
}

// draw takes the next piece from the bag, refilling it with all seven
// shapes in shuffled order when it is empty.
func (g *Game) draw() kind {
	if g.bagLeft == 0 {
		g.bag = [7]kind{pieceI, pieceO, pieceT, pieceS, pieceZ, pieceJ, pieceL}
		g.rng.Shuffle(len(g.bag), func(i, j int) { g.bag[i], g.bag[j] = g.bag[j], g.bag[i] })
		g.bagLeft = len(g.bag)
	}
	k := g.bag[len(g.bag)-g.bagLeft]
	g.bagLeft--
	return k
}

// spawn makes the preview piece current with fresh timers and reset budget.
// A spawn that overlaps the stack ends the run (block out).
func (g *Game) spawn() {
	g.cur = spawnPiece(g.next)
	g.next = g.draw()
	g.fall, g.lockTime, g.resets = 0, 0, 0
	g.airborne, g.deepest, g.lockNow = true, -1, false
	if !g.fits(g.cur) {
		g.state = lost
		return
	}
	g.settle()
}

// bottom is the row of the piece's lowest cell.
func (p piece) bottom() int {
	b := 0
	for _, c := range p.cells() {
		b = max(b, c.y)
	}
	return b
}

// settle records where the current piece stands after it moved: an airborne
// piece loses its grounded time, and a touchdown with the reset budget spent
// either opens a fresh lock window (strictly below the deepest row it was
// grounded on) or arms an immediate lock. See the Game comment.
func (g *Game) settle() {
	if !g.grounded() {
		g.lockTime = 0
		g.airborne = true
		g.lockNow = false // decided again at the next touchdown
		return
	}
	b := g.cur.bottom()
	if g.airborne {
		g.airborne = false
		if g.resets == maxResets && b <= g.deepest {
			g.lockNow = true
		}
	}
	g.deepest = max(g.deepest, b)
}

// fits reports whether p lies inside the board (hidden rows included) and
// overlaps no settled cell.
func (g *Game) fits(p piece) bool {
	for _, c := range p.cells() {
		if c.x < 0 || c.x >= Cols || c.y < 0 || c.y >= Rows || g.board[c.y][c.x] != none {
			return false
		}
	}
	return true
}

func (g *Game) grounded() bool {
	p := g.cur
	p.y++
	return !g.fits(p)
}

// landing is where the current piece would come to rest if dropped straight
// down: the landing projection and the hard-drop target.
func (g *Game) landing() piece {
	p := g.cur
	for {
		p.y++
		if !g.fits(p) {
			p.y--
			return p
		}
	}
}

// HandleInput moves, rotates and drops the current piece while playing and
// restarts a finished run on KeySelect.
func (g *Game) HandleInput(k engine.Key) {
	if g.state != playing {
		if k == engine.KeySelect {
			g.reset()
		}
		return
	}
	switch k {
	case engine.KeyLeft:
		g.shift(-1)
	case engine.KeyRight:
		g.shift(1)
	case engine.KeyUp:
		g.rotate(1)
	case engine.KeyAction:
		g.rotate(3) // three clockwise quarter turns: counterclockwise
	case engine.KeyDown:
		g.softDrop()
	case engine.KeySelect:
		g.hardDrop()
	}
}

// move makes p current if it fits and applies the lock-delay rules described
// on Game: a successful move of a grounded piece spends one reset while the
// budget lasts; failed moves change nothing.
func (g *Game) move(p piece) bool {
	if !g.fits(p) {
		return false
	}
	wasGrounded := g.grounded()
	g.cur = p
	if wasGrounded && g.resets < maxResets {
		g.resets++
		g.lockTime = 0
	}
	g.settle()
	return true
}

func (g *Game) shift(dx int) {
	p := g.cur
	p.x += dx
	g.move(p)
}

// rotate turns the piece by quarter clockwise turns and tries the kick
// offsets in order; the first legal placement wins. If none fits, nothing
// changes. O rotation is a no-op (it would leave the same cells), so it
// neither moves the piece nor spends lock-delay resets.
func (g *Game) rotate(quarters int) {
	if g.cur.kind == pieceO {
		return
	}
	for _, k := range kicks {
		p := g.cur
		p.rot = (p.rot + quarters) % 4
		p.x += k.x
		p.y += k.y
		if g.move(p) {
			return
		}
	}
}

// softDrop moves the piece down one row for one point. It also restarts the
// gravity step, so a soft drop is never followed by an immediate fall.
func (g *Game) softDrop() {
	p := g.cur
	p.y++
	if g.move(p) {
		g.score++
		g.fall = 0
	}
}

// hardDrop drops the piece to its landing row for two points per row
// actually moved and locks it immediately, bypassing the lock delay.
func (g *Game) hardDrop() {
	p := g.landing()
	g.score += 2 * (p.y - g.cur.y)
	g.cur = p
	g.lock()
}

// Update spends elapsed time on gravity steps and the lock deadline in
// chronological order, keeping the remainder. Time left over after a lock
// carries on with the next piece; nothing advances once the run ends.
//
// A deadline that falls exactly at the end of dt is processed in this call,
// including an immediate lock (zero delay) right after an exhausted
// touchdown.
func (g *Game) Update(dt time.Duration) {
	if dt <= 0 {
		return
	}
	for g.state == playing {
		if g.grounded() {
			g.fall = 0
			left := lockDelay - g.lockTime
			if g.lockNow {
				left = 0
			}
			if dt < left {
				g.lockTime += dt
				return
			}
			dt -= left
			g.lock()
			continue
		}
		g.lockTime = 0
		left := g.interval() - g.fall
		if dt < left {
			g.fall += dt
			return
		}
		dt -= left
		g.fall = 0
		g.cur.y++
		g.settle()
	}
}

// lock commits the current piece, clears every full row at once, compacts
// the rest downwards, scores the clear at the level in effect before it and
// spawns the next piece.
//
// Top-out rule: the run ends if, after clearing, any settled cell remains
// in a hidden row (lock out), or if the next piece cannot spawn (block out).
func (g *Game) lock() {
	if !g.fits(g.cur) { // unreachable: the current piece always fits
		g.state = lost
		return
	}
	for _, c := range g.cur.cells() {
		g.board[c.y][c.x] = g.cur.kind
	}
	g.fall, g.lockTime = 0, 0 // the piece is done; spawn starts the next one
	cleared := g.clearRows()
	g.score += lineScores[cleared] * g.Level()
	g.lines += cleared
	for y := range HiddenRows {
		for _, k := range g.board[y] {
			if k != none {
				g.state = lost
				return
			}
		}
	}
	g.spawn()
}

// clearRows removes all full rows simultaneously, moves the remaining rows
// down in order, and returns how many rows were removed.
func (g *Game) clearRows() int {
	dst := Rows - 1
	for y := Rows - 1; y >= 0; y-- {
		full := true
		for _, k := range g.board[y] {
			if k == none {
				full = false
				break
			}
		}
		if !full {
			g.board[dst] = g.board[y]
			dst--
		}
	}
	cleared := dst + 1
	for y := 0; y <= dst; y++ {
		g.board[y] = [Cols]kind{}
	}
	return cleared
}

// Layout: a bordered board of the twenty visible rows (two terminal columns
// per cell) with a side panel, centered in the screen minus the last row
// (the navigation footer).
const (
	boardW  = Cols*2 + 2
	boardH  = VisibleRows + 2
	panelX  = boardW + 3 // panel offset from the board's left edge
	layoutW = panelX + len(controlsLine)

	controlsLine = "Pause: Space   Leave: Q / Esc   Exit: Ctrl+C"
	legendLine   = "Piece <>   Landing ::   Stack []"
)

func (g *Game) Render(c engine.Canvas) {
	w, h := c.Size()
	const (
		frameW = 78
		cellW  = 3
		wellW  = Cols*cellW + 2
	)
	x0 := max(0, (w-frameW)/2)
	y0 := max(0, (h-1-(boardH+1))/2)

	status := "PLAYING"
	if g.state == lost {
		status = "GAME OVER"
	}
	c.Text(x0, y0, engine.Format(c, "> BLOCK DROP   Score %05d   Lines %03d   Level %02d   %s",
		g.score, g.lines, g.Level(), engine.Format(c, status)), engine.Accent)

	fx, fy := x0, y0+1
	gameui.Box(c, fx, fy, frameW, boardH, engine.Border)
	gameui.DotGrid(c, fx, fy, frameW, boardH, 2)

	// Block Drop keeps the authentic 10x20 rules, but the board is rendered as
	// a chunky arcade well rather than the old two-character ASCII panel.
	wx := fx + 10
	gameui.Box(c, wx, fy, wellW, boardH, engine.Border)

	cell := func(p point, glyph string, color engine.Color) {
		if p.y >= HiddenRows && p.y < Rows && p.x >= 0 && p.x < Cols {
			c.Text(wx+1+p.x*cellW, fy+1+p.y-HiddenRows, glyph, color)
		}
	}
	for y := HiddenRows; y < Rows; y++ {
		for x := range Cols {
			if g.board[y][x] != none {
				cell(point{x, y}, "▓▓▓", engine.Accent)
			}
		}
	}
	if g.state == playing {
		for _, p := range g.landing().cells() {
			cell(p, "░░░", engine.Muted)
		}
		for _, p := range g.cur.cells() {
			cell(p, "███", engine.Player)
		}
	}

	px := wx + wellW + 4
	c.Text(px, fy+2, engine.Format(c, status), engine.Accent)
	c.Text(px, fy+4, "NEXT", engine.Default)

	top := 4
	for _, p := range shapes[g.next][0] {
		top = min(top, p.y)
	}
	for _, p := range shapes[g.next][0] {
		c.Text(px+2+p.x*3, fy+6+p.y-top, "██", engine.Player)
	}

	for i, line := range []string{
		"Left / Right  move",
		"Up / Z        rotate",
		"Down          soft drop",
		"Enter         hard drop",
		"Pause: Space",
		"Leave: Q / Esc",
		"Exit: Ctrl+C",
	} {
		c.Text(px, fy+11+i, line, engine.Muted)
	}
	c.Text(px, fy+19, "Active █  Ghost ░  Stack ▓", engine.Muted)

	if g.state != playing {
		box := []string{"", "GAME OVER", engine.Format(c, "Final score %d", g.score),
			engine.Format(c, "Lines %d   Level %d", g.lines, g.Level()), "Enter: play again", ""}
		top := fy + (boardH-len(box))/2
		left := wx + (wellW-30)/2
		for i, line := range box {
			c.Text(left, top+i, " "+center(engine.Format(c, line), 28)+" ", engine.Warning)
		}
	}
}

func center(s string, width int) string {
	pad := max(0, width-len([]rune(s)))
	return fmt.Sprintf("%*s%s%*s", pad/2, "", s, pad-pad/2, "")
}
