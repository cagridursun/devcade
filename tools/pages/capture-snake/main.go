// capture-snake records frames rendered by the real Snake game, using a
// simple path-finding driver. It does not register players or submit scores.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/snake"
)

type point struct{ x, y int }
type canvas struct {
	text  [24][80]rune
	color [24][80]engine.Color
}

func (c *canvas) Size() (int, int) { return 80, 24 }
func (c *canvas) Cell(x, y int, r rune, color engine.Color) {
	if x >= 0 && x < 80 && y >= 0 && y < 24 {
		c.text[y][x] = engine.Printable(r)
		c.color[y][x] = color
	}
}
func (c *canvas) Text(x, y int, text string, color engine.Color) {
	for _, r := range text {
		c.Cell(x, y, r, color)
		x++
	}
}
func render(g engine.Game) *canvas {
	c := &canvas{}
	for y := range 24 {
		for x := range 80 {
			c.text[y][x] = ' '
		}
	}
	g.Render(c)
	c.Text(0, 23, "Q / Esc: back to game menu    Ctrl+C: quit DevCade", engine.Accent)
	return c
}

type frame struct {
	Text     []string `json:"text"`
	Colors   []string `json:"colors"`
	Duration int      `json:"duration"`
}

func record(c *canvas, duration int) frame {
	f := frame{Duration: duration}
	for y := range 24 {
		f.Text = append(f.Text, string(c.text[y][:]))
		colors := make([]byte, 80)
		for x := range 80 {
			colors[x] = '0' + byte(c.color[y][x])
		}
		f.Colors = append(f.Colors, string(colors))
	}
	return f
}

var moves = []point{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
var keys = []engine.Key{engine.KeyRight, engine.KeyDown, engine.KeyLeft, engine.KeyUp}

func steer(c *canvas, direction point) (engine.Key, point, bool) {
	head, food := point{-1, -1}, point{-1, -1}
	occupied := map[point]bool{}
	// The 74-column board is centred at terminal column 3, row 2.
	for y := range snake.Rows {
		for x := range snake.Cols {
			p := point{x, y}
			r := c.text[3+y][4+x*2]
			if r == '@' {
				head = p
			}
			if r == '*' {
				food = p
			}
			if r == '@' || r == 'o' {
				occupied[p] = true
			}
		}
	}
	if head.x < 0 || food.x < 0 {
		return engine.KeyNone, direction, false
	}
	queue := []point{head}
	first := map[point]int{head: -1}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for i, d := range moves {
			if p == head && d.x == -direction.x && d.y == -direction.y {
				continue
			}
			n := point{p.x + d.x, p.y + d.y}
			if n.x < 0 || n.x >= snake.Cols || n.y < 0 || n.y >= snake.Rows || occupied[n] {
				continue
			}
			if _, seen := first[n]; seen {
				continue
			}
			first[n] = first[p]
			if p == head {
				first[n] = i
			}
			if n == food {
				index := first[n]
				return keys[index], moves[index], true
			}
			queue = append(queue, n)
		}
	}
	return engine.KeyNone, direction, false
}
func main() {
	// Retry only the local random initial board; choose an interesting, safe
	// 24-second clip containing movement, turns, eating and visible growth.
	for attempt := 0; attempt < 100; attempt++ {
		g := snake.New()
		g.Start(80, 24)
		direction := point{1, 0}
		frames := []frame{}
		duration := 0
		for duration < 24000 && !g.(engine.Finisher).Finished() {
			c := render(g)
			score := g.(interface{ Score() int }).Score()
			interval := max(80, 180-15*(score/50))
			frames = append(frames, record(c, interval))
			duration += interval
			key, next, ok := steer(c, direction)
			if !ok {
				break
			}
			g.HandleInput(key)
			direction = next
			g.Update(time.Duration(interval) * time.Millisecond)
		}
		if duration < 24000 || g.(engine.Finisher).Finished() || g.(interface{ Score() int }).Score() < 40 {
			continue
		}
		out := struct {
			Version int     `json:"version"`
			Cols    int     `json:"cols"`
			Rows    int     `json:"rows"`
			Frames  []frame `json:"frames"`
		}{1, 80, 24, frames}
		if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
			panic(err)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "could not record a safe Snake demo")
	os.Exit(1)
}
