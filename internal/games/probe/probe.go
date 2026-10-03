// Package probe provides the M1 terminal diagnostic, not an arcade game.
package probe

import (
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

const step = 120 * time.Millisecond

type Probe struct {
	x, y int
	dx, dy int
	width, height int
	elapsed time.Duration
	initialized bool
}

func New() *Probe { return &Probe{dx: 1} }

func (p *Probe) MinimumSize() (int, int) { return 80, 24 }

func (p *Probe) Resize(width, height int) {
	p.width, p.height = width, height
	if !p.initialized {
		p.Reset()
	}
	p.x = max(1, min(p.x, width-2))
	p.y = max(4, min(p.y, height-3))
}

func (p *Probe) Reset() {
	p.x, p.y = p.width/2, p.height/2
	p.dx, p.dy = 1, 0
	p.elapsed = 0
	p.initialized = true
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

func (p *Probe) Update(dt time.Duration) {
	p.elapsed += dt
	for p.elapsed >= step {
		p.elapsed -= step
		nx, ny := p.x+p.dx, p.y+p.dy
		if nx < 1 || nx > p.width-2 || ny < 4 || ny > p.height-3 {
			p.dx, p.dy = -p.dx, -p.dy
			nx, ny = p.x+p.dx, p.y+p.dy
		}
		p.x, p.y = nx, ny
	}
}

func (p *Probe) Render(c engine.Canvas) {
	w, h := c.Size()
	c.Text(2, 0, "D E V C A D E  |  M1 TERMINAL PROBE", engine.Cyan)
	c.Text(2, 1, "Arrows / WASD: direction | Space: pause | Q / Esc: quit", engine.Default)
	for x := 0; x < w; x++ {
		c.Cell(x, 3, '-', engine.Default)
		c.Cell(x, h-2, '-', engine.Default)
	}
	for y := 3; y < h-1; y++ {
		c.Cell(0, y, '|', engine.Default)
		c.Cell(w-1, y, '|', engine.Default)
	}
	c.Cell(p.x, p.y, '@', engine.Green)
	c.Text(2, h-1, "Core checkpoint: input, ticking, resize, rendering and cleanup", engine.Default)
}
