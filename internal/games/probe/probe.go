// Package probe is the M1 terminal diagnostic: a single '@' that moves on a
// fixed tick so input, timing, resize and rendering can be checked by eye.
// It is not an arcade game.
package probe

import (
	"fmt"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

const (
	// Step is the time the '@' takes to move one cell.
	Step = 120 * time.Millisecond

	MinWidth  = 80
	MinHeight = 24

	headerRows = 3 // title, controls, status
)

// Probe implements engine.Game.
type Probe struct {
	width, height int
	x, y          int
	dx, dy        int
	elapsed       time.Duration
}

func New() *Probe { return &Probe{} }

func (p *Probe) MinimumSize() (int, int) { return MinWidth, MinHeight }

// Start centers the '@' and sets it moving right.
func (p *Probe) Start(width, height int) {
	p.width, p.height = width, height
	left, top, right, bottom := p.arena()
	p.x, p.y = (left+right)/2, (top+bottom)/2
	p.dx, p.dy = 1, 0
	p.elapsed = 0
}

// Resize keeps the '@' inside the new arena.
func (p *Probe) Resize(width, height int) {
	p.width, p.height = width, height
	left, top, right, bottom := p.arena()
	p.x = clamp(p.x, left, right)
	p.y = clamp(p.y, top, bottom)
}

func (p *Probe) HandleInput(key engine.Key) {
	switch key {
	case engine.KeyUp:
		p.dx, p.dy = 0, -1
	case engine.KeyDown:
		p.dx, p.dy = 0, 1
	case engine.KeyLeft:
		p.dx, p.dy = -1, 0
	case engine.KeyRight:
		p.dx, p.dy = 1, 0
	}
}

// Update moves one cell per Step of accumulated time, bouncing off the walls.
func (p *Probe) Update(dt time.Duration) {
	left, top, right, bottom := p.arena()
	if left > right || top > bottom {
		return
	}
	p.elapsed += dt
	for p.elapsed >= Step {
		p.elapsed -= Step
		p.x, p.dx = bounce(p.x, p.dx, left, right)
		p.y, p.dy = bounce(p.y, p.dy, top, bottom)
	}
}

// Position returns the current cell of the '@'.
func (p *Probe) Position() (x, y int) { return p.x, p.y }

// Direction returns the current movement vector.
func (p *Probe) Direction() (dx, dy int) { return p.dx, p.dy }

func (p *Probe) Render(c engine.Canvas) {
	w, h := c.Size()
	c.Text(1, 0, "DEVCADE >_  terminal core diagnostic (M1)", engine.Accent)
	c.Text(1, 1, "Move: arrows / WASD   Pause: Space   Leave: Q / Esc   Exit: Ctrl+C", engine.Default)
	c.Text(1, 2, fmt.Sprintf("Position %2d,%-2d  Heading %-5s  Screen %dx%d", p.x, p.y, p.heading(), w, h), engine.Default)

	// Border around the arena: rows headerRows and h-2, columns 0 and w-1.
	top, bottom := headerRows, h-2
	for x := 0; x < w; x++ {
		c.Cell(x, top, '-', engine.Default)
		c.Cell(x, bottom, '-', engine.Default)
	}
	for y := top; y <= bottom; y++ {
		glyph := '|'
		if y == top || y == bottom {
			glyph = '+'
		}
		c.Cell(0, y, glyph, engine.Default)
		c.Cell(w-1, y, glyph, engine.Default)
	}
	if left, top, right, bottom := p.arena(); left <= right && top <= bottom {
		c.Cell(p.x, p.y, '@', engine.Player)
	}
	c.Text(1, h-1, "Terminal diagnostic: checks input, timing, resize and terminal restore.", engine.Default)
}

// arena returns the inclusive interior bounds of the play area. The bounds
// are empty (left > right or top > bottom) when the screen is too small.
func (p *Probe) arena() (left, top, right, bottom int) {
	return 1, headerRows + 1, p.width - 2, p.height - 3
}

func (p *Probe) heading() string {
	switch {
	case p.dx > 0:
		return "right"
	case p.dx < 0:
		return "left"
	case p.dy > 0:
		return "down"
	case p.dy < 0:
		return "up"
	}
	return "none"
}

// bounce advances pos by d, reversing d at either bound.
func bounce(pos, d, lo, hi int) (int, int) {
	next := pos + d
	if next < lo || next > hi {
		d = -d
		next = pos + d
		if next < lo || next > hi {
			return clamp(pos, lo, hi), d
		}
	}
	return next, d
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return max(lo, min(v, hi))
}
