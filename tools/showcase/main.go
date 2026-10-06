// Showcase frames are rendered by the real DevCade game implementations.
package main

import (
	"fmt"
	"html"
	"os"
	"strings"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/blastgrid"
	"github.com/cagridursun/devcade/internal/games/blockdrop"
	"github.com/cagridursun/devcade/internal/games/brickbreaker"
	"github.com/cagridursun/devcade/internal/games/mazechase"
	"github.com/cagridursun/devcade/internal/games/snake"
	"github.com/cagridursun/devcade/internal/games/spaceshooter"
	"github.com/cagridursun/devcade/internal/games/terminalfc"
)

type cell struct {
	r rune
	c engine.Color
}

type canvas struct {
	w, h  int
	cells []cell
}

func newCanvas(w, h int) *canvas   { return &canvas{w: w, h: h, cells: make([]cell, w*h)} }
func (c *canvas) Size() (int, int) { return c.w, c.h }
func (c *canvas) Cell(x, y int, glyph rune, color engine.Color) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	c.cells[y*c.w+x] = cell{r: engine.Printable(glyph), c: color}
}
func (c *canvas) Text(x, y int, s string, color engine.Color) {
	for _, r := range s {
		c.Cell(x, y, r, color)
		x++
	}
}
func (c *canvas) reset() { clear(c.cells) }

type frame struct {
	game  string
	lines []string
}

func render(g engine.Game, c *canvas, game string) frame {
	c.reset()
	g.Render(c)
	lines := make([]string, c.h)
	for y := 0; y < c.h; y++ {
		var b strings.Builder
		last := -1
		for x := 0; x < c.w; x++ {
			r := c.cells[y*c.w+x].r
			if r == 0 {
				r = ' '
			} else {
				last = x
			}
			b.WriteRune(r)
		}
		s := b.String()
		if last < 0 {
			s = ""
		} else {
			s = strings.TrimRight(s, " ")
		}
		lines[y] = s
	}
	return frame{game: game, lines: lines}
}

func simulate(name string, g engine.Game, before func(engine.Game), step func(engine.Game, int), pre time.Duration) []frame {
	const w, h = 80, 24
	g.Start(w, h)
	if before != nil {
		before(g)
	}
	if pre > 0 {
		g.Update(pre)
	}
	c := newCanvas(w, h)
	out := make([]frame, 0, 10)
	for i := 0; i < 10; i++ {
		if step != nil {
			step(g, i)
		}
		g.Update(220 * time.Millisecond)
		out = append(out, render(g, c, name))
	}
	return out
}

func esc(s string) string { return html.EscapeString(s) }

func writeSVG(path string, frames []frame) error {
	const frameMS = 180
	total := len(frames) * frameMS
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="900" height="500" viewBox="0 0 900 500">
`)
	b.WriteString(`<rect width="900" height="500" rx="18" fill="#0b100d"/><rect x="18" y="18" width="864" height="464" rx="14" fill="#111814" stroke="#304437" stroke-width="2"/>`)
	b.WriteString(`<text x="38" y="45" fill="#829187" font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" font-size="13">actual DevCade Game.Render output · 80 × 24</text>`)
	for i, f := range frames {
		begin := i * frameMS
		b.WriteString(fmt.Sprintf(`<g opacity="0"><set attributeName="opacity" to="1" begin="%dms" dur="%dms" repeatCount="indefinite"/>`, begin, total))
		b.WriteString(fmt.Sprintf(`<text x="38" y="70" fill="#5bdc8c" font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" font-size="14">%s</text>`, esc(f.game)))
		b.WriteString(`<text x="38" y="92" fill="#e2eae4" font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" font-size="11.2" xml:space="preserve">`)
		for y, line := range f.lines {
			b.WriteString(fmt.Sprintf(`<tspan x="38" y="%d">%s</tspan>`, 92+y*15, esc(line)))
		}
		b.WriteString(`</text></g>`)
	}
	b.WriteString(`<text x="38" y="468" fill="#829187" font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" font-size="12">Snake · Block Drop · Maze Chase · Blast Grid · Brick Breaker · Terminal FC · Space Shooter</text></svg>`)
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func main() {
	var frames []frame
	frames = append(frames, simulate("Snake", snake.New(), nil, func(g engine.Game, i int) {
		switch i {
		case 2:
			g.HandleInput(engine.KeyUp)
		case 4:
			g.HandleInput(engine.KeyLeft)
		case 6:
			g.HandleInput(engine.KeyDown)
		case 8:
			g.HandleInput(engine.KeyRight)
		}
	}, 0)...)
	frames = append(frames, simulate("Block Drop", blockdrop.New(), nil, func(g engine.Game, i int) {
		switch i {
		case 1:
			g.HandleInput(engine.KeyRight)
		case 2:
			g.HandleInput(engine.KeyAction)
		case 4:
			g.HandleInput(engine.KeySelect)
		case 6:
			g.HandleInput(engine.KeyLeft)
		case 8:
			g.HandleInput(engine.KeySelect)
		}
	}, 0)...)
	frames = append(frames, simulate("Maze Chase", mazechase.New(), func(g engine.Game) { g.HandleInput(engine.KeyLeft) }, func(g engine.Game, i int) {
		switch i {
		case 2:
			g.HandleInput(engine.KeyUp)
		case 5:
			g.HandleInput(engine.KeyRight)
		case 8:
			g.HandleInput(engine.KeyDown)
		}
	}, 300*time.Millisecond)...)
	frames = append(frames, simulate("Blast Grid", blastgrid.New(), func(g engine.Game) { g.HandleInput(engine.KeyAction) }, func(g engine.Game, i int) {
		switch i {
		case 0:
			g.HandleInput(engine.KeyRight)
		case 2:
			g.HandleInput(engine.KeyDown)
		case 4:
			g.HandleInput(engine.KeyDown)
		case 6:
			g.HandleInput(engine.KeyRight)
		case 8:
			g.HandleInput(engine.KeyAction)
		}
	}, 100*time.Millisecond)...)
	frames = append(frames, simulate("Brick Breaker", brickbreaker.New(), func(g engine.Game) { g.HandleInput(engine.KeySelect) }, func(g engine.Game, i int) {
		if i%4 < 2 {
			g.HandleInput(engine.KeyLeft)
		} else {
			g.HandleInput(engine.KeyRight)
		}
	}, 200*time.Millisecond)...)
	frames = append(frames, simulate("Terminal FC", terminalfc.New(), func(g engine.Game) { g.HandleInput(engine.KeySelect) }, func(g engine.Game, i int) {
		switch i {
		case 1, 2:
			g.HandleInput(engine.KeyRight)
		case 3:
			g.HandleInput(engine.KeySecondary)
		case 5:
			g.HandleInput(engine.KeySelect)
		case 7:
			g.HandleInput(engine.KeyTertiary)
		case 8:
			g.HandleInput(engine.KeyUp)
		}
	}, 1200*time.Millisecond)...)
	frames = append(frames, simulate("Space Shooter", spaceshooter.New(), nil, func(g engine.Game, i int) {
		switch i {
		case 1:
			g.HandleInput(engine.KeyLeft)
		case 3:
			g.HandleInput(engine.KeyRight)
		case 5:
			g.HandleInput(engine.KeyAction)
		case 7:
			g.HandleInput(engine.KeyUp)
		case 9:
			g.HandleInput(engine.KeyRight)
		}
	}, 1200*time.Millisecond)...)

	if err := os.MkdirAll("dist/showcase", 0755); err != nil {
		panic(err)
	}
	if err := writeSVG("dist/showcase/devcade-seven-games.svg", frames); err != nil {
		panic(err)
	}
	fmt.Printf("captured %d renderer-backed gameplay frames\n", len(frames))

	seen := map[string]bool{}
	for _, f := range frames {
		if seen[f.game] {
			continue
		}
		seen[f.game] = true
		fmt.Printf("\n===== %s / actual 80x24 Game.Render =====\n", f.game)
		for _, line := range f.lines {
			fmt.Println(line)
		}
	}
}
