// Package brickbreaker implements a fixed-board, ten-level arcade game.
package brickbreaker

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/gameui"
)

const (
	Cols       = 60
	Rows       = 18
	Levels     = 10
	paddleY    = Rows - 2
	fixedStep  = time.Second / 120
	effectTime = 10 * time.Second
)

type ball struct{ x, y, dx, dy float64 }
type brick struct {
	x, y, hp, value int
	steel           bool
}
type drop struct {
	x, y float64
	kind int
}

const (
	wide = iota
	multi
	slow
	life
	piercing
	double
	bonusCount
)

var bonusGlyph = [bonusCount]rune{'W', 'M', 'S', 'L', 'P', 'X'}

// Game keeps simulation coordinates independent of terminal size.
type Game struct {
	rng                        *rand.Rand
	balls                      []ball
	bricks                     []brick
	drops                      []drop
	paddle                     float64
	effects                    [bonusCount]time.Duration
	score, level, lives, combo int
	stats                      engine.RunStats
	elapsed, serve, played     time.Duration
	ended, won                 bool
}

func New() engine.Game                      { return newGame(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))) }
func newGame(rng *rand.Rand) *Game          { g := &Game{rng: rng}; g.reset(); return g }
func (g *Game) MinimumSize() (int, int)     { return 80, 24 }
func (g *Game) Start(w, h int)              { g.reset() }
func (g *Game) Resize(w, h int)             {}
func (g *Game) Finished() bool              { return g.ended }
func (g *Game) Score() int                  { return g.score }
func (g *Game) Statistics() engine.RunStats { s := g.stats; s.Score = g.score; return s }
func (g *Game) reset() {
	g.score = 0
	g.level = 1
	g.lives = 3
	g.combo = 0
	g.stats = engine.RunStats{}
	g.elapsed = 0
	g.played = 0
	g.ended = false
	g.won = false
	g.loadLevel()
}
func (g *Game) loadLevel() {
	g.bricks = nil
	// Ten deterministic patterns: openings, stripes, checkerboards and wings.
	for row := 0; row < min(6, 2+(g.level+1)/2); row++ {
		for col := 0; col < 12; col++ {
			if g.level > 2 && (col+row+g.level)%7 == 0 {
				continue
			}
			hp := 1
			if g.level >= 3 && (col+row)%3 == 0 {
				hp = 2
			}
			if g.level >= 5 && (col+2*row)%4 == 0 {
				hp = 3
			}
			steel := g.level >= 6 && row == 0 && (col == 3 || col == 8)
			value := 10
			if hp == 2 {
				value = 25
			}
			if hp == 3 {
				value = 50
			}
			g.bricks = append(g.bricks, brick{x: col * 5, y: row + 2, hp: hp, value: value, steel: steel})
		}
	}
	g.resetServe()
}
func (g *Game) resetServe() {
	g.paddle = Cols / 2
	g.balls = nil
	g.drops = nil
	g.effects = [bonusCount]time.Duration{}
	g.combo = 0
	g.serve = 1500 * time.Millisecond
}
func (g *Game) paddleWidth() float64 {
	if g.effects[wide] > 0 {
		return 13
	}
	return 9
}
func (g *Game) launch() {
	g.serve = 0
	g.balls = []ball{{x: g.paddle, y: paddleY - 0.5, dx: 0.55, dy: -math.Sqrt(1 - 0.55*0.55)}}
}
func (g *Game) HandleInput(k engine.Key) {
	if g.ended {
		if k == engine.KeySelect {
			g.reset()
		}
		return
	}
	switch k {
	case engine.KeyLeft:
		g.paddle -= 3
	case engine.KeyRight:
		g.paddle += 3
	case engine.KeySelect, engine.KeyAction:
		if g.serve > 0 {
			g.launch()
		}
	}
	g.clampPaddle()
}
func (g *Game) clampPaddle() {
	half := g.paddleWidth() / 2
	g.paddle = math.Max(half, math.Min(Cols-half, g.paddle))
}
func (g *Game) speed() float64 {
	s := 8 + float64(g.level-1)*0.65 + math.Min(3, float64(g.stats.BricksDestroyed)*0.015)
	if g.effects[slow] > 0 {
		s *= 0.65
	}
	return s
}
func (g *Game) Update(dt time.Duration) {
	if g.ended || dt <= 0 {
		return
	}
	g.elapsed += dt
	for g.elapsed >= fixedStep && !g.ended {
		g.elapsed -= fixedStep
		g.step()
	}
}
func (g *Game) step() {
	g.played += fixedStep
	g.stats.PlayTimeMS = g.played.Milliseconds()
	for i := range g.effects {
		g.effects[i] = max(0, g.effects[i]-fixedStep)
	}
	g.clampPaddle()
	if g.serve > 0 {
		g.serve -= fixedStep
		if g.serve <= 0 {
			g.launch()
		}
		return
	}
	// At maximum speed each substep travels less than 0.4 columns / 0.2 rows,
	// preventing tunnelling through one-row bricks and the paddle.
	move := g.speed() * fixedStep.Seconds()
	alive := g.balls[:0]
	for _, b := range g.balls {
		old := b
		b.x += b.dx * move * 2
		b.y += b.dy * move
		if b.x < 0 {
			b.x = -b.x
			b.dx = math.Abs(b.dx)
		}
		if b.x >= Cols {
			b.x = 2*Cols - b.x - 0.001
			b.dx = -math.Abs(b.dx)
		}
		if b.y < 0 {
			b.y = -b.y
			b.dy = math.Abs(b.dy)
		}
		if b.dy > 0 && old.y < float64(paddleY) && b.y >= float64(paddleY) && math.Abs(b.x-g.paddle) <= g.paddleWidth()/2 {
			offset := (b.x - g.paddle) / (g.paddleWidth() / 2)
			b.dx = offset * 0.85
			if math.Abs(b.dx) < 0.18 {
				if b.dx < 0 {
					b.dx = -0.18
				} else {
					b.dx = 0.18
				}
			}
			b.dy = -math.Sqrt(1 - b.dx*b.dx)
			b.y = 2*float64(paddleY) - b.y
			g.combo = 0
		}
		for i := range g.bricks {
			r := &g.bricks[i]
			if r.hp == 0 || b.x < float64(r.x) || b.x >= float64(r.x+4) || b.y < float64(r.y) || b.y >= float64(r.y+1) {
				continue
			}
			if !r.steel {
				r.hp--
				if g.effects[piercing] > 0 {
					r.hp = 0
				}
				if r.hp == 0 {
					g.destroy(*r)
				}
			}
			if r.steel || g.effects[piercing] == 0 {
				if old.x < float64(r.x) || old.x >= float64(r.x+4) {
					b.dx = -b.dx
					b.x = old.x
				} else {
					b.dy = -b.dy
					b.y = old.y
				}
			}
			break
		}
		if b.y >= Rows {
			g.stats.BallsLost++
			continue
		}
		alive = append(alive, b)
	}
	g.balls = alive
	remaining := false
	for _, r := range g.bricks {
		if !r.steel && r.hp > 0 {
			remaining = true
			break
		}
	}
	if !remaining {
		g.stats.LevelsCleared++
		g.score += g.level * 500
		if g.level == Levels {
			g.ended = true
			g.won = true
			g.score += g.lives * 250
			return
		}
		g.level++
		g.loadLevel()
		return
	}
	if len(g.balls) == 0 {
		g.lives--
		if g.lives == 0 {
			g.ended = true
			return
		}
		g.resetServe()
		return
	}
	kept := g.drops[:0]
	for _, d := range g.drops {
		oldY := d.y
		d.y += 4 * fixedStep.Seconds()
		if oldY < float64(paddleY) && d.y >= float64(paddleY) && math.Abs(d.x-g.paddle) <= g.paddleWidth()/2 {
			g.apply(d.kind)
			continue
		}
		if d.y < Rows {
			kept = append(kept, d)
		}
	}
	g.drops = kept
}
func (g *Game) destroy(r brick) {
	g.stats.BricksDestroyed++
	g.combo++
	g.stats.HighestCombo = max(g.stats.HighestCombo, g.combo)
	// Exact integer ratios: 1, 1.2, 1.5, 2, 3; capped after ten bricks.
	multiplier := 10
	switch {
	case g.combo >= 10:
		multiplier = 30
	case g.combo >= 7:
		multiplier = 20
	case g.combo >= 4:
		multiplier = 15
	case g.combo >= 2:
		multiplier = 12
	}
	points := r.value * multiplier / 10
	if g.effects[double] > 0 {
		points *= 2
	}
	g.score += points
	if g.rng.IntN(5) == 0 {
		g.drops = append(g.drops, drop{x: float64(r.x + 2), y: float64(r.y), kind: g.rng.IntN(bonusCount)})
	}
}
func (g *Game) apply(kind int) {
	switch kind {
	case wide, slow, piercing, double:
		g.effects[kind] = effectTime
		g.clampPaddle()
	case life:
		g.lives = min(5, g.lives+1)
	case multi:
		if len(g.balls) == 0 {
			return
		}
		source := g.balls[0]
		for _, dx := range []float64{-0.65, 0.65} {
			if len(g.balls) >= 5 {
				break
			}
			g.balls = append(g.balls, ball{x: source.x, y: source.y, dx: dx, dy: -math.Sqrt(1 - dx*dx)})
		}
	}
}
func (g *Game) Render(c engine.Canvas) {
	w, h := c.Size()
	const frameW = 78
	const frameH = Rows + 2
	x0 := max(0, (w-frameW)/2)
	y0 := max(0, (h-1-(2+frameH+1))/2)

	c.Text(x0, y0, engine.Format(c, "> BRICK BREAKER   Score %05d   Level %d/10   Lives %d   Combo %d",
		g.score, g.level, g.lives, g.combo), engine.Accent)
	c.Text(x0, y0+1, "Move: arrows/A/D   Launch: Enter/Z   Pause: Space", engine.Muted)

	fx, fy := x0, y0+2
	gameui.Box(c, fx, fy, frameW, frameH, engine.Border)
	gameui.DotGrid(c, fx, fy, frameW, frameH, 2)

	// Physics stay on the original 60-column board, but rendering expands that
	// board to the complete frame. The visible side walls now match the exact
	// collision boundaries, so the ball never appears to bounce in mid-air.
	innerX := fx + 1
	innerW := frameW - 2
	by := fy + 1
	mapX := func(v float64) int {
		v = math.Max(0, math.Min(Cols, v))
		return innerX + int(math.Round(v*float64(innerW-1)/float64(Cols)))
	}

	for _, r := range g.bricks {
		if r.hp == 0 {
			continue
		}
		left := mapX(float64(r.x))
		right := max(left+1, mapX(float64(r.x+4)))
		glyph := '█'
		color := engine.Accent
		if r.steel {
			glyph = '▓'
			color = engine.Border
		}
		for x := left; x < right; x++ {
			c.Cell(x, by+r.y, glyph, color)
		}
	}
	for _, d := range g.drops {
		c.Cell(mapX(d.x), by+int(d.y), bonusGlyph[d.kind], engine.Warning)
	}
	for _, b := range g.balls {
		c.Cell(mapX(b.x), by+int(b.y), '●', engine.Player)
	}

	half := g.paddleWidth() / 2
	leftPaddle := mapX(g.paddle - half)
	rightPaddle := max(leftPaddle+1, mapX(g.paddle+half))
	for x := leftPaddle; x <= rightPaddle; x++ {
		c.Cell(x, by+paddleY, '█', engine.Player)
	}

	if g.serve > 0 {
		c.Cell(mapX(g.paddle), by+paddleY-1, '●', engine.Player)
		c.Text(fx+18, by+11, "W wide   M multi   S slow   L life   P pierce   X 2x", engine.Muted)
		c.Text(fx+24, by+13, "Enter/Z: launch   auto-launch in 1.5s", engine.Warning)
	}

	c.Text(x0, y0+22, engine.Format(c, "Bricks %d   Cleared %d   Best combo %d   W M S L P X: bonuses",
		g.stats.BricksDestroyed, g.stats.LevelsCleared, g.stats.HighestCombo), engine.Muted)

	if g.ended {
		title := "GAME OVER"
		if g.won {
			title = "ALL LEVELS COMPLETE"
		}
		lines := []string{"", title, engine.Format(c, "Final score %d", g.score), "Enter: play again", ""}
		left := fx + (frameW-42)/2
		for i, s := range lines {
			c.Text(left, by+6+i, fmt.Sprintf(" %-40s ", engine.Format(c, s)), engine.Warning)
		}
	} else {
		effects := ""
		for _, i := range []int{wide, slow, piercing, double} {
			if g.effects[i] > 0 {
				effects += fmt.Sprintf("%c:%ds ", bonusGlyph[i], int(math.Ceil(g.effects[i].Seconds())))
			}
		}
		if effects != "" {
			c.Text(fx+2, by+Rows-1, effects, engine.Warning)
		}
	}
}
