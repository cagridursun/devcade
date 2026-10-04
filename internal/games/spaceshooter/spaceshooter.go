// Package spaceshooter implements a fixed-step, backend-independent arcade shooter.
package spaceshooter

import (
	"math"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

const (
	Cols              = 36
	Rows              = 18
	Step              = 20 * time.Millisecond
	MoveCooldown      = 70 * time.Millisecond
	FireInterval      = 240 * time.Millisecond
	RapidInterval     = 120 * time.Millisecond
	Preparation       = time.Second
	Protection        = 1500 * time.Millisecond
	ShieldProtection  = 500 * time.Millisecond
	EffectDuration    = 8 * time.Second
	SpecialCooldown   = 750 * time.Millisecond
	SpecialRadius     = 3 // Seven-cell vertical corridor centered on the ship.
	ShockDuration     = 600 * time.Millisecond
	ScoutInterval     = 400 * time.Millisecond
	DiverInterval     = 300 * time.Millisecond
	GunnerInterval    = 600 * time.Millisecond
	EnemyFireInterval = 1800 * time.Millisecond
	BossFireInterval  = 1400 * time.Millisecond
	Telegraph         = 400 * time.Millisecond
	MaxEnemies        = 18
	MaxProjectiles    = 128
	MaxPickups        = 24
	MaxScore          = 1000000000
	MaxWave           = 1000000
)
const (
	scout = iota
	diver
	gunner
	boss
)

type enemy struct {
	id                                 uint64
	kind, hp, x, y, ox, oy, dir, moves int
	clock, fire, warning               time.Duration
	aimX, aimY                         int
	spread                             bool
}
type projectile struct {
	id                   uint64
	x, y, ox, oy, dx, dy float64
	hostile, dead        bool
}
type pickup struct {
	x, y   float64
	shield bool
	age    time.Duration
}
type Game struct {
	rng                                                       *rand.Rand
	enemies                                                   []enemy
	bullets                                                   []projectile
	pickups                                                   []pickup
	nextID                                                    uint64
	shockX                                                    int
	shock                                                     time.Duration
	x, y, oldX, oldY                                          int
	score, wave, lives, charges                               int
	elapsed, prep, movement, fire, protection, rapid, special time.Duration
	shield, pending, ended                                    bool
}

var _ engine.Game = (*Game)(nil)
var _ engine.Finisher = (*Game)(nil)
var _ interface{ Score() int } = (*Game)(nil)

func New() engine.Game { return NewWithSource(rand.NewPCG(rand.Uint64(), rand.Uint64())) }

// NewWithSource permits reproducible simulations without global randomness.
func NewWithSource(source rand.Source) *Game { g := &Game{rng: rand.New(source)}; g.reset(); return g }
func (g *Game) MinimumSize() (int, int)      { return 80, 24 }
func (g *Game) Start(w, h int)               { g.reset() }
func (g *Game) Resize(w, h int)              {}
func (g *Game) Finished() bool               { return g.ended }
func (g *Game) Score() int                   { return g.score }
func (g *Game) reset() {
	rng := g.rng
	*g = Game{rng: rng, wave: 1, lives: 3, charges: 1, x: Cols / 2, y: Rows - 2, prep: Preparation}
	g.oldX, g.oldY = g.x, g.y
}
func (g *Game) id() uint64          { g.nextID++; return g.nextID }
func (g *Game) multiplier() int     { return min(5, 1+(g.wave-1)/5) }
func (g *Game) addScore(points int) { g.score = min(MaxScore, g.score+points*g.multiplier()) }
func (g *Game) HandleInput(k engine.Key) {
	if g.ended {
		if k == engine.KeySelect {
			g.reset()
		}
		return
	}
	if k == engine.KeyAction {
		if g.prep == 0 && g.special == 0 && g.charges > 0 && !g.pending {
			g.pending = true
		}
		return
	}
	if g.movement > g.elapsed {
		return
	}
	switch k {
	case engine.KeyLeft:
		g.x = max(0, g.x-1)
	case engine.KeyRight:
		g.x = min(Cols-1, g.x+1)
	case engine.KeyUp:
		g.y = max(Rows-4, g.y-1)
	case engine.KeyDown:
		g.y = min(Rows-1, g.y+1)
	default:
		return
	}
	g.movement = MoveCooldown + g.elapsed
}
func (g *Game) Update(dt time.Duration) {
	if g.ended || dt <= 0 {
		return
	}
	g.elapsed += dt
	for g.elapsed >= Step && !g.ended {
		g.elapsed -= Step
		g.step()
	}
	if g.ended {
		g.elapsed = 0
	}
}
func (g *Game) interval(base time.Duration) time.Duration {
	return base - time.Duration(min(14, g.wave-1))*base/30
}
func (g *Game) spawnWave() {
	g.fire = 0
	if g.wave%5 == 0 {
		g.enemies = []enemy{{id: g.id(), kind: boss, hp: 12 + 4*min(5, g.wave/5-1), x: (Cols - 5) / 2, y: 2, ox: (Cols - 5) / 2, oy: 2, dir: 1}}
		return
	}
	n := min(MaxEnemies, 6+2*(g.wave-1))
	// A seeded permutation of 18 spaced slots prevents overlaps and retry loops.
	slots := g.rng.Perm(MaxEnemies)
	for i := 0; i < n; i++ {
		kind := scout
		if g.wave >= 3 && i%3 == 2 {
			kind = gunner
		} else if g.wave >= 2 && i%3 == 1 {
			kind = diver
		}
		x, y := 3+(slots[i]%6)*5, 1+(slots[i]/6)*2
		hp := 1
		if kind == gunner {
			hp = 2
		}
		g.enemies = append(g.enemies, enemy{id: g.id(), kind: kind, hp: hp, x: x, y: y, ox: x, oy: y, dir: 1, fire: time.Duration(i%6) * 100 * time.Millisecond})
	}
}
func width(e enemy) int {
	if e.kind == boss {
		return 5
	}
	return 1
}
func (g *Game) shoot(x, y, dx, dy float64, hostile bool) {
	if len(g.bullets) >= MaxProjectiles {
		return
	}
	g.bullets = append(g.bullets, projectile{id: g.id(), x: x, y: y, ox: x, oy: y, dx: dx, dy: dy, hostile: hostile})
}
func (g *Game) moveEnemy(e *enemy) {
	e.ox, e.oy = e.x, e.y
	e.clock += Step
	e.fire += Step
	if e.warning > 0 {
		e.warning = max(0, e.warning-Step)
		if e.warning == 0 {
			switch e.kind {
			case diver:
				e.moves = 1
				e.clock = 0
			case gunner:
				g.shoot(float64(e.x)+.5, float64(e.y)+1, 0, 6, true)
			case boss:
				if e.spread {
					for _, dx := range []float64{-2, 0, 2} {
						g.shoot(float64(e.x)+2.5, float64(e.y)+1, dx, 6, true)
					}
				} else {
					dx, dy := float64(e.aimX)+.5-(float64(e.x)+2.5), float64(e.aimY)+.5-(float64(e.y)+1)
					norm := math.Hypot(dx, dy)
					g.shoot(float64(e.x)+2.5, float64(e.y)+1, 6*dx/norm, 6*dy/norm, true)
				}
				e.spread = !e.spread
			}
		}
	}
	switch e.kind {
	case scout, gunner, boss:
		base := ScoutInterval
		if e.kind == gunner {
			base = GunnerInterval
		}
		if e.clock >= g.interval(base) {
			e.clock -= g.interval(base)
			nx := e.x + e.dir
			if nx < 0 || nx+width(*e) > Cols {
				e.dir = -e.dir
				if e.kind != boss {
					e.y++
				}
			} else {
				e.x = nx
			}
			e.moves++
			if e.kind != boss && e.moves%10 == 0 {
				e.y++
			}
		}
	case diver:
		if e.moves == 0 && e.clock >= 1200*time.Millisecond && e.warning == 0 {
			e.warning = Telegraph
		}
		if e.moves > 0 && e.clock >= g.interval(DiverInterval) {
			e.clock -= g.interval(DiverInterval)
			e.y++
			e.x = min(Cols-1, max(0, e.x+e.dir))
			e.dir = -e.dir
			e.moves++
		}
	}
	if (e.kind == gunner || e.kind == boss) && e.warning == 0 {
		base := EnemyFireInterval
		if e.kind == boss {
			base = BossFireInterval
		}
		if e.fire >= g.interval(base) {
			e.fire = 0
			e.warning = Telegraph
			e.aimX, e.aimY = g.x, g.y
		}
	}
}

// swept tests a moving point against a moving rectangular hitbox in relative
// coordinates. Inclusive entry permits contact on cell edges; stable slice/ID
// order breaks simultaneous ties, and each projectile is consumed once.
func sweepTime(ax, ay, bx, by, tx, ty, ux, uy, w, h float64) (float64, bool) {
	x, y := ax-tx, ay-ty
	dx, dy := (bx-ax)-(ux-tx), (by-ay)-(uy-ty)
	lo, hi := 0.0, 1.0
	for _, v := range [][3]float64{{x, dx, w}, {y, dy, h}} {
		if math.Abs(v[1]) < 1e-12 {
			if v[0] < 0 || v[0] > v[2] {
				return 0, false
			}
			continue
		}
		a, b := -v[0]/v[1], (v[2]-v[0])/v[1]
		if a > b {
			a, b = b, a
		}
		lo = math.Max(lo, a)
		hi = math.Min(hi, b)
		if lo > hi {
			return 0, false
		}
	}
	return lo, true
}
func swept(ax, ay, bx, by, tx, ty, ux, uy, w, h float64) bool {
	_, ok := sweepTime(ax, ay, bx, by, tx, ty, ux, uy, w, h)
	return ok
}

func (g *Game) kill(e *enemy) {
	points := 10
	switch e.kind {
	case diver:
		points = 20
	case gunner:
		points = 30
	case boss:
		points = 500
	}
	g.addScore(points)
	if e.kind != boss && g.rng.IntN(100) < 15 {
		isShield := g.rng.IntN(2) == 1
		if len(g.pickups) < MaxPickups {
			g.pickups = append(g.pickups, pickup{x: float64(e.x) + .5, y: float64(e.y) + .5, shield: isShield})
		}
	}
}
func (g *Game) damage() {
	if g.protection > 0 {
		return
	}
	if g.shield {
		g.shield = false
		g.protection = ShieldProtection
		return
	}
	g.lives--
	g.x, g.y = Cols/2, Rows-2
	g.protection = Protection
	// Remove hostile bullets and bodies within three cells of both old and reset positions.
	kept := g.bullets[:0]
	for _, b := range g.bullets {
		if b.hostile && (math.Hypot(b.x-float64(g.x), b.y-float64(g.y)) < 3 || math.Hypot(b.x-float64(g.oldX), b.y-float64(g.oldY)) < 3) {
			continue
		}
		kept = append(kept, b)
	}
	g.bullets = kept
	for i := range g.enemies {
		e := &g.enemies[i]
		if e.kind != boss && (math.Hypot(float64(e.x-g.x), float64(e.y-g.y)) < 3 || math.Hypot(float64(e.x-g.oldX), float64(e.y-g.oldY)) < 3) {
			e.hp = 0
		}
	}
}
func (g *Game) step() {
	// Stable order: timers; movement/spawn; special/projectile contacts and
	// kills; pickups and coalesced player damage; fatality; surviving clear.
	for _, timer := range []*time.Duration{&g.movement, &g.protection, &g.rapid, &g.special, &g.shock} {
		*timer = max(0, *timer-Step)
	}
	if g.prep > 0 {
		g.prep = max(0, g.prep-Step)
		if g.prep == 0 {
			g.spawnWave()
		}
		g.oldX, g.oldY = g.x, g.y
		return
	}
	for i := range g.enemies {
		g.moveEnemy(&g.enemies[i])
	}
	g.fire += Step
	interval := FireInterval
	if g.rapid > 0 {
		interval = RapidInterval
	}
	if g.fire >= interval {
		g.fire -= interval
		g.shoot(float64(g.x)+.5, float64(g.y), 0, -10, false)
	}
	if g.pending {
		g.pending = false
		g.charges--
		g.special = SpecialCooldown
		g.shock, g.shockX = ShockDuration, g.x
		for i := range g.bullets {
			if g.bullets[i].hostile {
				g.bullets[i].dead = true
			}
		}
		for i := range g.enemies {
			e := &g.enemies[i]
			if e.hp > 0 && e.x <= g.shockX+SpecialRadius && e.x+width(*e)-1 >= g.shockX-SpecialRadius {
				damage := 1
				if e.kind == boss {
					damage = 3
				}
				e.hp -= damage
				if e.hp <= 0 {
					g.kill(e)
				}
			}
		}
	}
	for i := range g.bullets {
		b := &g.bullets[i]
		b.ox, b.oy = b.x, b.y
		b.x += b.dx * Step.Seconds()
		b.y += b.dy * Step.Seconds()
	}
	// Resolve opposing contacts in traversal-time order, then by stable IDs.
	type contact struct {
		a, b int
		at   float64
	}
	contacts := []contact{}
	for i, a := range g.bullets {
		if a.dead {
			continue
		}
		for j := i + 1; j < len(g.bullets); j++ {
			b := g.bullets[j]
			if b.dead || a.hostile == b.hostile {
				continue
			}
			if at, ok := sweepTime(a.ox, a.oy, a.x, a.y, b.ox-.12, b.oy-.12, b.x-.12, b.y-.12, .24, .24); ok {
				contacts = append(contacts, contact{i, j, at})
			}
		}
	}
	sort.Slice(contacts, func(i, j int) bool {
		a, b := contacts[i], contacts[j]
		if a.at != b.at {
			return a.at < b.at
		}
		if g.bullets[a.a].id != g.bullets[b.a].id {
			return g.bullets[a.a].id < g.bullets[b.a].id
		}
		return g.bullets[a.b].id < g.bullets[b.b].id
	})
	for _, hit := range contacts {
		a, b := &g.bullets[hit.a], &g.bullets[hit.b]
		if !a.dead && !b.dead {
			a.dead, b.dead = true, true
		}
	}
	for i := range g.bullets {
		b := &g.bullets[i]
		if b.dead || b.hostile {
			continue
		}
		target, best := -1, 2.0
		for j, e := range g.enemies {
			if e.hp <= 0 {
				continue
			}
			at, ok := sweepTime(b.ox, b.oy, b.x, b.y, float64(e.ox), float64(e.oy), float64(e.x), float64(e.y), float64(width(e)), 1)
			if ok && (at < best || (at == best && (target < 0 || e.id < g.enemies[target].id))) {
				target, best = j, at
			}
		}
		if target >= 0 {
			b.dead = true
			e := &g.enemies[target]
			e.hp--
			if e.hp == 0 {
				g.kill(e)
			}
		}
	}

	hurt := false
	for i := range g.bullets {
		b := &g.bullets[i]
		if !b.dead && b.hostile && swept(b.ox, b.oy, b.x, b.y, float64(g.oldX), float64(g.oldY), float64(g.x), float64(g.y), 1, 1) {
			b.dead = true
			hurt = true
		}
	}
	for i := range g.enemies {
		e := &g.enemies[i]
		if e.hp <= 0 {
			continue
		}
		if e.kind != boss && e.y >= Rows {
			e.hp = 0
			hurt = true
			continue
		}
		if swept(float64(g.oldX)+.5, float64(g.oldY)+.5, float64(g.x)+.5, float64(g.y)+.5, float64(e.ox)-.5, float64(e.oy)-.5, float64(e.x)-.5, float64(e.y)-.5, float64(width(*e))+1, 2) {
			hurt = true
		}
	}
	kept := g.pickups[:0]
	for _, p := range g.pickups {
		p.age += Step
		old := p.y
		p.y += 3 * Step.Seconds()
		if p.age >= EffectDuration || p.y >= Rows {
			continue
		}
		if swept(p.x, old, p.x, p.y, float64(g.oldX), float64(g.oldY), float64(g.x), float64(g.y), 1, 1) {
			if p.shield {
				g.shield = true
			} else {
				g.rapid = EffectDuration
			}
			continue
		}
		kept = append(kept, p)
	}
	g.pickups = kept
	if hurt {
		g.damage()
	}
	alive := g.enemies[:0]
	for _, e := range g.enemies {
		if e.hp > 0 {
			alive = append(alive, e)
		}
	}
	g.enemies = alive
	bullets := g.bullets[:0]
	for _, b := range g.bullets {
		if !b.dead && b.x >= 0 && b.x < Cols && b.y >= 0 && b.y < Rows {
			bullets = append(bullets, b)
		}
	}
	g.bullets = bullets
	if g.lives == 0 {
		g.ended = true
	} else if len(g.enemies) == 0 {
		g.addScore(100)
		if g.wave%5 == 0 {
			g.charges = min(2, g.charges+1)
		}
		g.wave = min(MaxWave, g.wave+1)
		g.prep = Preparation
		g.bullets = nil
		g.pickups = nil
		g.fire = 0
	}
	g.oldX, g.oldY = g.x, g.y
}
func (g *Game) Render(c engine.Canvas) {
	w, h := c.Size()
	x0, y0 := max(0, (w-74)/2), max(0, (h-23)/2)
	text := func(x, y int, s string, args ...any) { c.Text(x, y, engine.Format(c, s, args...), engine.Default) }
	text(x0, y0, "SPACE SHOOTER  Score %d Wave %d Lives %d Z %d", g.score, g.wave, g.lives, g.charges)
	text(x0, y0+1, "Move: arrows/WASD  Auto-fire  Z: special  Pause: Space")
	bx, by := x0, y0+2
	for x := 0; x < 74; x++ {
		c.Cell(bx+x, by, '-', engine.Default)
		c.Cell(bx+x, by+19, '-', engine.Default)
	}
	for y := 0; y < 20; y++ {
		c.Cell(bx, by+y, '|', engine.Default)
		c.Cell(bx+73, by+y, '|', engine.Default)
	}
	cell := func(x, y int, s string, color engine.Color) {
		if x >= 0 && x < Cols && y >= 0 && y < Rows {
			c.Text(bx+1+x*2, by+1+y, s, color)
		}
	}
	// Render the full-width sweep behind actors. The stronger corridor marks
	// where enemy damage was applied at activation; rendering has no side effects.
	if g.shock > 0 {
		row := Rows - 1 - int((ShockDuration-g.shock)*time.Duration(Rows)/ShockDuration)
		for x := 0; x < Cols; x++ {
			glyph := "--"
			color := engine.Default
			if x >= g.shockX-SpecialRadius && x <= g.shockX+SpecialRadius {
				glyph = "=="
				color = engine.Warning
			}
			cell(x, row, glyph, color)
		}
	}
	for _, e := range g.enemies {
		glyph := []string{"><", "VV", "[]", "<<[==]>>  "}[e.kind]
		if e.warning > 0 {
			glyph = []string{"!!", "V!", "[!", "!!"}[e.kind]
			if e.kind == boss {
				glyph = "!![==]!!  "
			}
		}
		cell(e.x, e.y, glyph, engine.Accent)
	}
	for _, b := range g.bullets {
		s := "| "
		if b.hostile {
			s = ". "
		}
		cell(int(b.x), int(b.y), s, engine.Warning)
	}
	for _, p := range g.pickups {
		s := "RF"
		if p.shield {
			s = "SH"
		}
		cell(int(p.x), int(p.y), s, engine.Accent)
	}
	player := "A^"
	if g.protection > 0 {
		player = "{}"
	} else if g.shield {
		player = "[A"
	}
	cell(g.x, g.y, player, engine.Player)
	if g.prep > 0 {
		text(bx+20, by+9, "Wave %d ready: %d", g.wave, int((g.prep+time.Second-1)/time.Second))
	}
	status := "PLAYING"
	if g.rapid > 0 {
		status = "Rapid fire"
	}
	if g.shield {
		status = "Shield"
	}
	if g.protection > 0 {
		status = "Protected"
	}
	if g.shock > 0 {
		status = "SPECIAL ATTACK"
	}
	if g.ended {
		status = "GAME OVER"
	}
	text(x0, y0+22, status)
	for _, e := range g.enemies {
		if e.kind == boss {
			text(x0+28, y0+22, "Boss HP %d", e.hp)
		}
	}
	if g.ended {
		for i, s := range []string{"GAME OVER", engine.Format(c, "Final score %d", g.score), engine.Format(c, "Highest wave %d", g.wave), "Enter: play again"} {
			if i == 0 || i == 3 {
				s = engine.Format(c, s)
			}
			c.Text(bx+20, by+7+i, "                                        ", engine.Warning)
			c.Text(bx+20, by+7+i, s, engine.Warning)
		}
	}
}
