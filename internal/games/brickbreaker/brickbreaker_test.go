package brickbreaker

import (
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

func fixture() *Game { return newGame(rand.New(rand.NewPCG(1, 2))) }
func TestTenLevelsAndSteelNeverBlocksCompletion(t *testing.T) {
	g := fixture()
	for level := 1; level <= Levels; level++ {
		if g.level != level || g.serve <= 0 {
			t.Fatal("level setup", g.level)
		}
		count := 0
		for i := range g.bricks {
			r := &g.bricks[i]
			if !r.steel {
				count++
				r.hp = 0
			}
		}
		if count == 0 {
			t.Fatal("no breakable bricks")
		}
		g.launch()
		g.step()
	}
	if !g.won || !g.Finished() || g.stats.LevelsCleared != 10 || g.Score() != 27500+750 {
		t.Fatal(g.Statistics(), g.Score())
	}
	g.Update(time.Second)
	if g.Score() != 28250 {
		t.Fatal("win bonus repeated")
	}
	g.HandleInput(engine.KeySelect)
	if g.Score() != 0 || g.level != 1 || g.lives != 3 || g.Finished() || g.Statistics().LevelsCleared != 0 {
		t.Fatal("restart")
	}
}
func TestDurabilityCollisionAndPiercing(t *testing.T) {
	for _, pierce := range []bool{false, true} {
		g := fixture()
		g.serve = 0
		g.bricks = []brick{{x: 10, y: 3, hp: 3, value: 50}, {x: 40, y: 3, hp: 1, value: 10}}
		if pierce {
			g.effects[piercing] = time.Second
		}
		g.balls = []ball{{x: 12, y: 4.01, dx: 0, dy: -1}}
		g.step()
		if pierce {
			if g.bricks[0].hp != 0 || g.balls[0].dy >= 0 || g.stats.BricksDestroyed != 1 {
				t.Fatal("piercing", g.bricks, g.balls)
			}
		} else {
			if g.bricks[0].hp != 2 || g.balls[0].dy <= 0 || g.Score() != 0 {
				t.Fatal("durability", g.bricks, g.balls)
			}
		}
	}
	g := fixture()
	g.serve = 0
	g.bricks = []brick{{x: 10, y: 3, hp: 1, steel: true}, {x: 40, y: 3, hp: 1, value: 10}}
	g.effects[piercing] = time.Second
	g.balls = []ball{{x: 12, y: 4.01, dy: -1}}
	g.step()
	if g.bricks[0].hp != 1 || g.balls[0].dy <= 0 || g.Score() != 0 {
		t.Fatal("steel must reflect piercing balls")
	}
}
func TestWallsPaddleAndSideCollision(t *testing.T) {
	cases := []struct {
		name string
		b    ball
		axis string
	}{
		{"left", ball{x: 0.01, y: 10, dx: -1}, "right"},
		{"right", ball{x: Cols - 0.01, y: 10, dx: 1}, "left"},
		{"ceiling", ball{x: 25, y: 0.01, dy: -1}, "down"},
		{"paddle", ball{x: 30, y: paddleY - 0.01, dy: 1}, "up"},
		{"brick side", ball{x: 9.99, y: 3.5, dx: 1}, "left"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			g := fixture()
			g.serve = 0
			g.combo = 7
			g.bricks = []brick{{x: 10, y: 3, hp: 3, value: 50}}
			g.balls = []ball{tt.b}
			g.step()
			b := g.balls[0]
			if (tt.axis == "right" && b.dx <= 0) || (tt.axis == "left" && b.dx >= 0) || (tt.axis == "up" && b.dy >= 0) || (tt.axis == "down" && b.dy <= 0) {
				t.Fatal(b)
			}
			if tt.name == "paddle" && (g.combo != 0 || math.Abs(b.dx) < 0.18) {
				t.Fatal("anti-stall and combo reset", b, g.combo)
			}
		})
	}
}
func TestMultiballLosesLifeOnlyAfterLastBall(t *testing.T) {
	g := fixture()
	g.launch()
	g.apply(multi)
	if len(g.balls) != 3 {
		t.Fatal(g.balls)
	}
	g.apply(multi)
	g.apply(multi)
	if len(g.balls) != 5 {
		t.Fatal("ball cap")
	}
	g.balls = []ball{{x: 1, y: Rows - 0.01, dy: 1}, {x: 20, y: 10, dy: -1}}
	g.step()
	if g.lives != 3 || len(g.balls) != 1 || g.stats.BallsLost != 1 {
		t.Fatal("single loss", g.Statistics())
	}
	for want := 2; want >= 0; want-- {
		g.serve = 0
		g.balls = []ball{{x: 1, y: Rows - 0.01, dy: 1}}
		g.step()
		if g.lives != want {
			t.Fatal(g.lives, want)
		}
	}
	if !g.Finished() || g.won {
		t.Fatal("loss")
	}
}
func TestEveryBonusCatchExpiryAndLifeCap(t *testing.T) {
	for kind := 0; kind < bonusCount; kind++ {
		t.Run(string(bonusGlyph[kind]), func(t *testing.T) {
			g := fixture()
			g.launch()
			base := g.speed()
			g.drops = []drop{{x: g.paddle, y: paddleY - 0.01, kind: kind}}
			g.step()
			if len(g.drops) != 0 {
				t.Fatal("drop not caught")
			}
			switch kind {
			case multi:
				if len(g.balls) != 3 {
					t.Fatal("multiball")
				}
			case life:
				if g.lives != 4 {
					t.Fatal("extra life")
				}
				g.apply(life)
				g.apply(life)
				if g.lives != 5 {
					t.Fatal("life cap")
				}
			default:
				if g.effects[kind] != effectTime {
					t.Fatal("timed bonus")
				}
			}
			if kind == slow && g.speed() >= base {
				t.Fatal("slow bonus")
			}
			if kind == wide {
				if g.paddleWidth() != 13 {
					t.Fatal("wide")
				}
				g.paddle = 0
				g.clampPaddle()
				if g.paddle != 6.5 {
					t.Fatal("edge clamp")
				}
			}
			if kind != multi && kind != life {
				g.effects[kind] = fixedStep
				g.step()
				if g.effects[kind] != 0 {
					t.Fatal("expiry")
				}
			}
		})
	}
	g := fixture()
	g.launch()
	g.drops = []drop{{x: 1, y: paddleY - 0.01, kind: life}}
	g.step()
	if g.lives != 3 {
		t.Fatal("missed bonus collected")
	}
}
func TestComboScoringAndDouble(t *testing.T) {
	g := fixture()
	for range 10 {
		g.destroy(brick{value: 10})
	}
	if g.Score() != 169 || g.stats.HighestCombo != 10 {
		t.Fatal(g.Score(), g.Statistics())
	}
	g.effects[double] = time.Second
	g.destroy(brick{value: 50})
	if g.Score() != 469 {
		t.Fatal(g.Score())
	}
}
func TestTimingResizeAndProgressiveSpeed(t *testing.T) {
	a, b := fixture(), fixture()
	a.launch()
	b.launch()
	a.Update(100 * time.Millisecond)
	for range 10 {
		b.Update(10 * time.Millisecond)
	}
	if !reflect.DeepEqual(a.balls, b.balls) || a.Statistics() != b.Statistics() {
		t.Fatal("frame subdivision changes physics")
	}
	before := a.Statistics()
	balls := append([]ball(nil), a.balls...)
	a.Resize(160, 50)
	a.Update(0)
	a.Update(-time.Second)
	if before != a.Statistics() || !reflect.DeepEqual(balls, a.balls) {
		t.Fatal("resize or nonpositive dt advances gameplay")
	}
	if a.stats.PlayTimeMS < 99 || a.stats.PlayTimeMS > 100 {
		t.Fatal(a.stats.PlayTimeMS)
	}
	base := a.speed()
	a.level = 10
	if a.speed() <= base {
		t.Fatal("difficulty must increase")
	}
	a.effects[wide] = time.Second
	a.resetServe()
	if a.effects[wide] != 0 {
		t.Fatal("effects leak across life loss")
	}
}
