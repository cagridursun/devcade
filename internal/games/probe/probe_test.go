package probe

import (
	"strings"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

func started(w, h int) *Probe {
	p := New()
	p.Start(w, h)
	return p
}

func TestStartsCenteredMovingRight(t *testing.T) {
	p := started(80, 24)
	if x, y := p.Position(); x != 39 || y != 12 {
		t.Fatalf("start = %d,%d, want 39,12", x, y)
	}
	if dx, dy := p.Direction(); dx != 1 || dy != 0 {
		t.Fatalf("direction = %d,%d", dx, dy)
	}
}

func TestMovesOneCellPerStepAndKeepsRemainder(t *testing.T) {
	p := started(80, 24)
	p.Update(Step - time.Millisecond)
	if x, _ := p.Position(); x != 39 {
		t.Fatalf("moved early to x=%d", x)
	}
	p.Update(time.Millisecond)
	p.Update(Step / 2)
	p.Update(Step / 2)
	if x, _ := p.Position(); x != 41 {
		t.Fatalf("x = %d after two steps, want 41", x)
	}
}

func TestDirectionInput(t *testing.T) {
	p := started(80, 24)
	for _, tc := range []struct {
		key    engine.Key
		dx, dy int
	}{
		{engine.KeyUp, 0, -1},
		{engine.KeyLeft, -1, 0},
		{engine.KeyDown, 0, 1},
		{engine.KeyRight, 1, 0},
	} {
		x, y := p.Position()
		p.HandleInput(tc.key)
		p.Update(Step)
		nx, ny := p.Position()
		if nx-x != tc.dx || ny-y != tc.dy {
			t.Errorf("%v moved by %d,%d", tc.key, nx-x, ny-y)
		}
	}
}

func TestBouncesInsideArena(t *testing.T) {
	p := started(80, 24)
	for range 500 {
		p.Update(Step)
		x, y := p.Position()
		if x < 1 || x > 78 || y < 4 || y > 21 {
			t.Fatalf("left arena at %d,%d", x, y)
		}
	}
	if dx, _ := p.Direction(); dx == 0 {
		t.Fatal("lost horizontal movement")
	}
	p.HandleInput(engine.KeyUp)
	p.Update(20 * Step)
	if _, y := p.Position(); y != 16 { // 8 steps up to row 4, bounce, 12 down
		t.Fatalf("y = %d after vertical bounce, want 16", y)
	}
}

func TestResizeClampsIntoArena(t *testing.T) {
	p := started(200, 60)
	p.Update(80 * Step) // x = 99+80 = 179
	p.Resize(80, 24)
	if x, y := p.Position(); x != 78 || y != 21 {
		t.Fatalf("clamped to %d,%d, want 78,21", x, y)
	}
}

func TestTinyArenaIsSafe(t *testing.T) {
	for _, size := range [][2]int{{0, 0}, {1, 1}, {3, 7}, {2, 30}} {
		p := started(size[0], size[1])
		p.Update(10 * Step)
		p.Resize(size[1], size[0])
		p.Update(10 * Step)
		p.Render(&strictCanvas{t: t, w: size[0], h: size[1], clip: true})
	}
}

func TestRenderStaysInBoundsAndShowsHeader(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {81, 25}, {200, 60}} {
		c := &strictCanvas{t: t, w: size[0], h: size[1]}
		p := started(size[0], size[1])
		p.Render(c)
		if !strings.Contains(c.text, "DEVCADE") || !strings.Contains(c.text, "Space") || c.ats != 1 {
			t.Errorf("%v: header or single '@' missing", size)
		}
	}
}

// strictCanvas fails the test on non-ASCII writes and, unless clip is set,
// on out-of-bounds writes.
type strictCanvas struct {
	t    *testing.T
	w, h int
	clip bool
	text string
	ats  int
}

func (c *strictCanvas) Size() (int, int) { return c.w, c.h }
func (c *strictCanvas) Cell(x, y int, r rune, _ engine.Color) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		if c.clip {
			return
		}
		c.t.Errorf("cell %d,%d outside %dx%d", x, y, c.w, c.h)
	}
	if engine.Printable(r) != r {
		c.t.Errorf("non-ASCII glyph %q", r)
	}
	if r == '@' {
		c.ats++
	}
}
func (c *strictCanvas) Text(x, y int, s string, color engine.Color) {
	c.text += s + "\n"
	for _, r := range s {
		if x < c.w { // text may be clipped on the right; Cell checks the rest
			c.Cell(x, y, r, color)
		}
		x++
	}
}
