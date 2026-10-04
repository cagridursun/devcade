package spaceshooter

import (
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/ui"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"
)

func seeded() *Game { return NewWithSource(rand.NewPCG(7, 19)) }
func arena() *Game {
	g := seeded()
	g.prep = 0
	g.enemies = []enemy{{id: g.id(), kind: boss, hp: 32, x: 3, y: 1, ox: 3, oy: 1, dir: 1}}
	return g
}
func TestMovementCooldownPreparationAndFire(t *testing.T) {
	g := seeded()
	g.HandleInput(engine.KeyLeft)
	g.HandleInput(engine.KeyLeft)
	if g.x != Cols/2-1 {
		t.Fatal("repeat not ignored")
	}
	g.Update(80 * time.Millisecond)
	g.HandleInput(engine.KeyUp)
	if g.y != Rows-3 {
		t.Fatal("movement")
	}
	for range 40 {
		g.Update(80 * time.Millisecond)
		g.HandleInput(engine.KeyUp)
		g.HandleInput(engine.KeyLeft)
	}
	if g.y < Rows-4 || g.x < 0 {
		t.Fatal("boundary")
	}
	h := seeded()
	h.Update(Preparation - Step)
	if len(h.enemies) != 0 || len(h.bullets) != 0 {
		t.Fatal("preparation active")
	}
	h.Update(Step)
	if len(h.enemies) != 6 {
		t.Fatal("wave one")
	}
	h.Update(FireInterval - Step)
	if len(h.bullets) != 0 {
		t.Fatal("early fire")
	}
	h.Update(Step)
	if len(h.bullets) != 1 {
		t.Fatal("fire cadence")
	}
	h.rapid = EffectDuration
	h.Update(RapidInterval)
	if len(h.bullets) != 2 {
		t.Fatal("rapid cadence")
	}
}
func TestSweptCollisionAndProjectileContact(t *testing.T) {
	if !swept(.5, 10, .5, 0, 0, 4, 0, 5, 1, 1) {
		t.Fatal("moving target tunneling")
	}
	if swept(2, 10, 2, 0, 0, 4, 0, 5, 1, 1) {
		t.Fatal("miss")
	}
	g := arena()
	g.enemies = []enemy{{id: 1, kind: gunner, hp: 2, x: 4, y: 4, ox: 4, oy: 4, dir: 1}}
	g.shoot(4.5, 6, 0, -200, false)
	g.step()
	if g.enemies[0].hp != 1 || len(g.bullets) != 0 {
		t.Fatal("traversed hit", g.enemies, g.bullets)
	}
	g = arena()
	g.shoot(20, 6, 0, -100, false)
	g.shoot(20, 3, 0, 100, true)
	g.step()
	if len(g.bullets) != 0 {
		t.Fatal("crossing projectiles survive")
	}
	g = arena()
	g.enemies[0].x, g.enemies[0].ox = 10, 10
	g.shoot(9, 1.5, 400, 0, false)
	g.step()
	if g.enemies[0].hp != 31 {
		t.Fatal("boss hit more than once")
	}
}
func TestDamageShieldAndFatalPrecedence(t *testing.T) {
	g := arena()
	g.shield = true
	for range 3 {
		g.shoot(float64(g.x)+.5, float64(g.y)-.1, 0, 20, true)
	}
	g.step()
	if g.lives != 3 || g.shield || g.protection != ShieldProtection {
		t.Fatal("shield/coalescing")
	}
	g.damage()
	if g.lives != 3 {
		t.Fatal("protection")
	}
	g.protection = 0
	g.damage()
	if g.lives != 2 || g.protection != Protection {
		t.Fatal("life")
	}
	g = arena()
	g.lives = 1
	g.enemies = []enemy{{id: 1, kind: scout, hp: 1, x: 10, y: 4, ox: 10, oy: 4, dir: 1}}
	g.shoot(10.5, 5.1, 0, -20, false)
	g.shoot(float64(g.x)+.5, float64(g.y)-.1, 0, 20, true)
	g.step()
	if !g.ended || g.score != 10 || g.wave != 1 {
		t.Fatal("fatal clear precedence", g.score, g.wave, g.lives)
	}
}
func TestWaveCompositionBossClearAndScore(t *testing.T) {
	for _, wave := range []int{1, 2, 3, 4, 5, 10, 15, 30, 1000000} {
		g := seeded()
		g.wave = wave
		g.Update(Preparation)
		if wave%5 == 0 {
			if len(g.enemies) != 1 || g.enemies[0].kind != boss || g.enemies[0].hp != 12+4*min(5, wave/5-1) {
				t.Fatal("boss", wave)
			}
			continue
		}
		if len(g.enemies) != min(18, 6+2*(wave-1)) {
			t.Fatal("count", wave)
		}
		positions := map[[2]int]bool{}
		for _, e := range g.enemies {
			p := [2]int{e.x, e.y}
			if positions[p] || e.x < 0 || e.x >= Cols || e.y < 0 || e.y >= Rows/2 {
				t.Fatal("layout")
			}
			positions[p] = true
			if wave == 1 && e.kind != scout {
				t.Fatal("intro")
			}
		}
	}
	g := arena()
	g.wave = 5
	g.enemies[0].hp = 3
	g.charges = 1
	g.HandleInput(engine.KeyAction)
	g.HandleInput(engine.KeyAction)
	g.step()
	if g.wave != 6 || g.score != 600 || g.charges != 1 || g.prep != Preparation {
		t.Fatal("boss special clear", g)
	}
	g.step()
	if g.score != 600 {
		t.Fatal("double bonus")
	}
	g = arena()
	g.wave = 30
	g.score = MaxScore - 2
	g.addScore(500)
	if g.score != MaxScore {
		t.Fatal("saturation")
	}
	if g.interval(ScoutInterval) != arenaAtWave(15).interval(ScoutInterval) {
		t.Fatal("difficulty not bounded")
	}
}
func arenaAtWave(w int) *Game { g := arena(); g.wave = w; return g }
func TestEscapeSpecialAndCaps(t *testing.T) {
	g := arena()
	g.enemies = []enemy{{id: 1, kind: scout, hp: 1, x: 3, y: Rows, ox: 3, oy: Rows, dir: 1}, {id: 2, kind: scout, hp: 1, x: 6, y: Rows, ox: 6, oy: Rows, dir: 1}}
	g.step()
	if g.lives != 2 || g.score != 100 || g.wave != 2 {
		t.Fatal("escape must not award kill points", g)
	}
	g = arena()
	for range MaxProjectiles + 20 {
		g.shoot(2, 2, 0, 1, true)
	}
	if len(g.bullets) != MaxProjectiles {
		t.Fatal("bullet cap")
	}
	g.charges = 2
	g.HandleInput(engine.KeyAction)
	g.step()
	g.HandleInput(engine.KeyAction)
	g.step()
	if g.charges != 1 || len(g.bullets) != 0 || g.enemies[0].hp != 29 {
		t.Fatal("special cooldown")
	}
	g.Update(SpecialCooldown)
	g.HandleInput(engine.KeyAction)
	g.step()
	if g.charges != 0 {
		t.Fatal("special recharge timer")
	}
	g.pickups = make([]pickup, MaxPickups)
	for range 100 {
		g.kill(&enemy{kind: scout, hp: 0})
	}
	if len(g.pickups) > MaxPickups {
		t.Fatal("pickup cap")
	}
}
func TestPickupsRefreshExpireAndResizeReset(t *testing.T) {
	g := arena()
	g.rapid = time.Second
	g.pickups = []pickup{{x: float64(g.x) + .5, y: float64(g.y) + .5}}
	g.step()
	if g.rapid != EffectDuration || len(g.pickups) != 0 {
		t.Fatal("rapid refresh")
	}
	g.pickups = []pickup{{x: float64(g.x) + .5, y: float64(g.y) + .5, shield: true}}
	g.step()
	if !g.shield {
		t.Fatal("shield")
	}
	g.pickups = []pickup{{x: 0, y: 1, age: EffectDuration - Step}, {x: 0, y: Rows - .01}}
	g.step()
	if len(g.pickups) != 0 {
		t.Fatal("expiry")
	}
	before := *g
	g.Resize(200, 60)
	if !reflect.DeepEqual(before, *g) {
		t.Fatal("resize mutated")
	}
	g.ended = true
	g.HandleInput(engine.KeySelect)
	if g.ended || g.score != 0 || g.wave != 1 || g.lives != 3 || g.charges != 1 || g.elapsed != 0 || g.shield || g.rapid > 0 || g.pending || len(g.enemies)+len(g.bullets)+len(g.pickups) > 0 {
		t.Fatal("unclean reset", g)
	}
	g.Update(Preparation)
	a := append([]enemy(nil), g.enemies...)
	g.ended = true
	g.HandleInput(engine.KeySelect)
	g.Update(Preparation)
	if reflect.DeepEqual(a, g.enemies) {
		t.Fatal("restart rewound RNG")
	}
}
func TestFramePartitioningTimestampedInputs(t *testing.T) {
	a, b := seeded(), seeded()
	// Every input has the same timestamp, even when it falls between fixed steps.
	for i := range 2000 {
		a.Update(73 * time.Millisecond)
		b.Update(7 * time.Millisecond)
		b.Update(19 * time.Millisecond)
		b.Update(47 * time.Millisecond)
		k := []engine.Key{engine.KeyLeft, engine.KeyUp, engine.KeyRight, engine.KeyDown, engine.KeyAction}[i%5]
		a.HandleInput(k)
		b.HandleInput(k)
		aa, bb := *a, *b
		aa.rng, bb.rng = nil, nil
		if !reflect.DeepEqual(aa, bb) {
			t.Fatalf("partition-dependent state at %d", i)
		}
	}
}
func TestLongRunsBoundedAndBossBehaviors(t *testing.T) {
	for seed := uint64(0); seed < 20; seed++ {
		g := NewWithSource(rand.NewPCG(seed, 11))
		for i := 0; i < 15000 && !g.ended; i++ {
			g.Update(100 * time.Millisecond)
			if len(g.enemies) > MaxEnemies || len(g.bullets) > MaxProjectiles || len(g.pickups) > MaxPickups {
				t.Fatal("unbounded")
			}
		}
		if !g.ended {
			t.Fatal("idle game stalled", seed)
		}
	}
	g := arenaAtWave(5)
	g.enemies[0].hp = 1000000
	aimed, spread := false, false
	for range 400 {
		g.protection = Protection
		g.step()
		for _, b := range g.bullets {
			if b.hostile && b.dx != 0 {
				aimed = true
			}
			if b.hostile && b.dx == 2 {
				spread = true
			}
		}
		if g.enemies[0].x < 0 || g.enemies[0].x+5 > Cols || g.enemies[0].y >= Rows/2 {
			t.Fatal("boss escaped")
		}
	}
	if !aimed || !spread {
		t.Fatal("boss patterns absent")
	}
}

type canvas struct {
	t    *testing.T
	rows [24][80]rune
}

func (c *canvas) Size() (int, int) { return 80, 24 }
func (c *canvas) Cell(x, y int, r rune, _ engine.Color) {
	if x < 0 || y < 0 || x >= 80 || y >= 24 {
		c.t.Fatalf("out of bounds %d,%d", x, y)
	}
	if engine.Printable(r) != r {
		c.t.Fatal("wide glyph")
	}
	c.rows[y][x] = r
}
func (c *canvas) Text(x, y int, s string, color engine.Color) {
	for _, r := range s {
		c.Cell(x, y, r, color)
		x++
	}
}
func TestRenderAllLocalesReadOnly(t *testing.T) {
	for _, lang := range ui.Languages {
		for _, ended := range []bool{false, true} {
			g := arenaAtWave(MaxWave)
			g.score = MaxScore
			g.ended = ended
			g.shield = true
			before := *g
			c := &canvas{t: t}
			g.Render(ui.Canvas{Canvas: c, Language: lang})
			if !reflect.DeepEqual(before, *g) {
				t.Fatal("render mutated state")
			}
			if strings.Contains(ui.Translate(lang, "Move: arrows/WASD  Auto-fire  Z: special  Pause: Space"), "Move:") && lang != "en" {
				t.Fatal("missing translation", lang)
			}
		}
	}
}
func TestEngineSuspensionFreezesEntireGame(t *testing.T) {
	g := seeded()
	e := engine.New(g)
	e.Resize(80, 24)
	e.Advance(100 * time.Millisecond)
	before := *g
	e.Input(engine.Event{Key: engine.KeyPause})
	e.Advance(time.Second)
	after := *g
	if !reflect.DeepEqual(before, after) {
		t.Fatal("pause")
	}
	e.Resize(40, 12)
	e.Input(engine.Event{Key: engine.KeyPause})
	e.Advance(time.Second)
	e.Resize(80, 24)
	if !reflect.DeepEqual(before, *g) {
		t.Fatal("undersize")
	}
}

func TestExactSeventyMillisecondInputCooldown(t *testing.T) {
	g := seeded()
	g.Update(10 * time.Millisecond)
	g.HandleInput(engine.KeyLeft)
	x := g.x
	g.Update(69 * time.Millisecond)
	g.HandleInput(engine.KeyLeft)
	if g.x != x {
		t.Fatal("accepted before 70ms")
	}
	g.Update(time.Millisecond)
	g.HandleInput(engine.KeyLeft)
	if g.x != x-1 {
		t.Fatal("rejected at 70ms")
	}
}
func TestEnemyBehaviorsAndRenderDoesNotConsumeRandomness(t *testing.T) {
	g := arena()
	e := enemy{kind: diver, hp: 1, x: 12, y: 2, dir: 1}
	for range 60 {
		g.moveEnemy(&e)
	}
	if e.warning != Telegraph || e.y != 2 {
		t.Fatal("diver did not telegraph")
	}
	for range 20 {
		g.moveEnemy(&e)
	}
	g.Update(time.Millisecond)
	for range 15 {
		g.moveEnemy(&e)
	}
	if e.y != 3 || e.x != 13 {
		t.Fatal("diver first step", e)
	}
	for range 15 {
		g.moveEnemy(&e)
	}
	if e.y != 4 || e.x != 12 {
		t.Fatal("diver alternating lateral step", e)
	}
	e = enemy{kind: gunner, hp: 2, x: 12, y: 2, dir: 1}
	for range 90 {
		g.moveEnemy(&e)
	}
	if e.warning != Telegraph || len(g.bullets) != 0 {
		t.Fatal("gunner warning")
	}
	for range 20 {
		g.moveEnemy(&e)
	}
	if len(g.bullets) != 1 || !g.bullets[0].hostile || g.bullets[0].dx != 0 {
		t.Fatal("gunner fire")
	}
	a, b := seeded(), seeded()
	for range 100 {
		a.Render(&canvas{t: t})
	}
	a.Update(Preparation)
	b.Update(Preparation)
	if !reflect.DeepEqual(a.enemies, b.enemies) {
		t.Fatal("render consumed RNG")
	}
}
func TestOrdinaryAndBossTargetsCanBeClearedByAutomaticFire(t *testing.T) {
	for _, wave := range []int{1, 2, 3, 4, 5, 10, 15, 30, 1000, MaxWave} {
		g := seeded()
		g.wave = wave
		g.Update(Preparation)
		cleared := false
		for range 8000 {
			// Invulnerability isolates target reachability from driver survival.
			g.protection = Protection
			e := g.enemies[0]
			target := e.x
			if e.kind == boss {
				target = min(Cols-1, max(0, e.x+2+e.dir*5))
			}
			if g.x < target {
				g.HandleInput(engine.KeyRight)
			} else if g.x > target {
				g.HandleInput(engine.KeyLeft)
			}
			g.Update(80 * time.Millisecond)
			if g.prep > 0 {
				cleared = true
				break
			}
		}
		if !cleared {
			t.Fatal("unreachable target or stalled wave", wave, g.enemies)
		}
	}
}

func TestTraversedTargetOrderAndStableTies(t *testing.T) {
	g := arena()
	g.enemies = []enemy{{id: 1, kind: gunner, hp: 2, x: 10, y: 2, ox: 10, oy: 2, dir: 1}, {id: 2, kind: gunner, hp: 2, x: 10, y: 5, ox: 10, oy: 5, dir: 1}}
	g.shoot(10.5, 7, 0, -300, false)
	g.step()
	if g.enemies[0].hp != 2 || g.enemies[1].hp != 1 {
		t.Fatal("must hit nearest traversed target", g.enemies)
	}
	g = arena()
	g.enemies = []enemy{{id: 9, kind: gunner, hp: 2, x: 10, y: 5, ox: 10, oy: 5, dir: 1}, {id: 2, kind: gunner, hp: 2, x: 10, y: 5, ox: 10, oy: 5, dir: 1}}
	g.shoot(10.5, 7, 0, -300, false)
	g.step()
	if g.enemies[0].hp != 2 || g.enemies[1].hp != 1 {
		t.Fatal("equal-time contacts must use stable IDs")
	}
	g = arena()
	g.shoot(20, 7, 0, -300, false)
	g.shoot(20, 2, 0, 0, true)
	g.shoot(20, 5, 0, 0, true)
	g.step()
	if len(g.bullets) != 1 || g.bullets[0].id != g.nextID-1 {
		t.Fatal("near contact must consume crossing projectile first", g.bullets)
	}
}
