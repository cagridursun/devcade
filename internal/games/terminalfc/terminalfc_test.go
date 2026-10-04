package terminalfc

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

func seeded(seed uint64) *Game {
	return newGame(rand.New(rand.NewPCG(seed, seed+1)))
}

func makeLive(g *Game) {
	g.phase = phaseLive
	g.phaseLeft = 0
	g.liveLeft = matchTime
	g.accum = 0
	g.now = 0
	g.nextThink = botInterval
}

func TestMovementCooldownAndCarriedBall(t *testing.T) {
	g := seeded(1)
	makeLive(g)
	g.active = 1
	g.ball.owner = 1
	g.ball.mode = ballCarried
	g.ball.pos = g.players[1].pos
	start := g.players[1].pos

	g.HandleInput(engine.KeyRight)
	if g.players[1].pos.x != start.x+1 || g.ball.pos != g.players[1].pos {
		t.Fatalf("first move = %+v ball=%+v", g.players[1].pos, g.ball.pos)
	}
	g.HandleInput(engine.KeyRight)
	if g.players[1].pos.x != start.x+1 {
		t.Fatal("movement cooldown accepted a second immediate press")
	}
	g.Update(moveCooldown)
	g.HandleInput(engine.KeyRight)
	if g.players[1].pos.x != start.x+2 {
		t.Fatal("movement did not resume after cooldown")
	}
}

func TestPassReleasesIndependentBallAndKickerGrace(t *testing.T) {
	g := seeded(2)
	makeLive(g)
	g.active = 1
	g.players[1].pos = vec{10, 9}
	g.players[1].facing = vec{1, 0}
	g.players[2].pos = vec{16, 9}
	for i := 5; i < 10; i++ {
		g.players[i].pos = vec{30, float64(i)}
	}
	g.ball = ballState{pos: g.players[1].pos, owner: 1, lastTouch: 1, releasedBy: noPlayer, mode: ballCarried}

	g.HandleInput(engine.KeyAction)
	if g.ball.owner != noPlayer || g.ball.mode != ballPass || math.Abs(g.ball.vel.len()-passSpeed) > 1e-9 {
		t.Fatalf("pass did not release a moving ball: %+v", g.ball)
	}
	if g.acquire(1) {
		t.Fatal("kicker reclaimed the ball inside grace period")
	}
	g.now = g.ball.reclaimAfter
	if !g.acquire(1) {
		t.Fatal("kicker could not reclaim after grace period")
	}
}

func TestSweptGoalAndOwnGoal(t *testing.T) {
	for _, tt := range []struct {
		name       string
		start      vec
		vel        vec
		lastTouch  int
		home, away int
	}{
		{"home shot", vec{35.2, 9}, vec{20, 0}, 1, 1, 0},
		{"home own goal", vec{0.8, 9}, vec{-20, 0}, 1, 0, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := seeded(3)
			makeLive(g)
			for i := range g.players {
				g.players[i].pos = vec{18, 1 + float64(i%3)}
			}
			g.ball = ballState{pos: tt.start, vel: tt.vel, owner: noPlayer, lastTouch: tt.lastTouch, releasedBy: tt.lastTouch, mode: ballShot}
			g.Update(100 * time.Millisecond)
			if g.homeGoals != tt.home || g.awayGoals != tt.away || g.phase != phaseGoal {
				t.Fatalf("score %d-%d phase=%v", g.homeGoals, g.awayGoals, g.phase)
			}
		})
	}
}

func TestFinalWhistlePreventsLateGoal(t *testing.T) {
	g := seeded(4)
	makeLive(g)
	g.liveLeft = fixedStep
	for i := range g.players {
		g.players[i].pos = vec{18, 1 + float64(i%3)}
	}
	g.ball = ballState{pos: vec{35, 9}, vel: vec{20, 0}, owner: noPlayer, lastTouch: 1, releasedBy: 1, mode: ballShot}
	g.Update(100 * time.Millisecond)
	if !g.Finished() || g.homeGoals != 0 || g.awayGoals != 0 {
		t.Fatalf("late goal or unfinished match: finished=%v score=%d-%d", g.Finished(), g.homeGoals, g.awayGoals)
	}
}

func TestArcadeScoreFormulaAndReset(t *testing.T) {
	tests := []struct {
		home, away, want int
	}{
		{0, 1, 0},
		{0, 0, 500},
		{3, 1, 1200},
		{10, 0, 1750},
		{20, 0, 1750},
	}
	for _, tt := range tests {
		g := seeded(uint64(10 + tt.home))
		makeLive(g)
		g.homeGoals, g.awayGoals = tt.home, tt.away
		g.finish()
		if got := g.Score(); got != tt.want {
			t.Fatalf("%d-%d score=%d want=%d", tt.home, tt.away, got, tt.want)
		}
		g.HandleInput(engine.KeySelect)
		if g.Finished() || g.Score() != 0 || g.homeGoals != 0 || g.awayGoals != 0 || g.phase != phaseKickoff {
			t.Fatal("Enter did not reset the entire match")
		}
	}
}

func TestFramePartitioningKeepsState(t *testing.T) {
	a, b := seeded(21), seeded(21)
	makeLive(a)
	makeLive(b)
	a.ball = ballState{pos: vec{18, 9}, vel: vec{6, 2}, owner: noPlayer, lastTouch: noPlayer, releasedBy: noPlayer, mode: ballFree}
	b.ball = a.ball

	a.Update(240 * time.Millisecond)
	for range 12 {
		b.Update(20 * time.Millisecond)
	}
	if a.phase != b.phase || a.liveLeft != b.liveLeft || dist(a.ball.pos, b.ball.pos) > 1e-9 || dist(a.ball.vel, b.ball.vel) > 1e-9 {
		t.Fatalf("partition changed state: A=%+v B=%+v", a.ball, b.ball)
	}
	for i := range a.players {
		if dist(a.players[i].pos, b.players[i].pos) > 1e-9 {
			t.Fatalf("player %d differs: %+v %+v", i, a.players[i].pos, b.players[i].pos)
		}
	}
}

func TestSeededMatchesFinishWithinBound(t *testing.T) {
	totalHome, totalAway := 0, 0
	for seed := uint64(1); seed <= 8; seed++ {
		g := seeded(seed)
		for step := 0; step < 4000 && !g.Finished(); step++ {
			g.Update(100 * time.Millisecond)
		}
		if !g.Finished() {
			t.Fatalf("seed %d did not finish", seed)
		}
		totalHome += g.homeGoals
		totalAway += g.awayGoals
		if g.Score() < 0 || g.Score() > 1750 {
			t.Fatalf("seed %d invalid score %d", seed, g.Score())
		}
		for i, p := range g.players {
			if math.IsNaN(p.pos.x) || math.IsNaN(p.pos.y) || math.IsInf(p.pos.x, 0) || math.IsInf(p.pos.y, 0) {
				t.Fatalf("seed %d player %d invalid position %+v", seed, i, p.pos)
			}
		}
		if math.IsNaN(g.ball.pos.x) || math.IsNaN(g.ball.pos.y) {
			t.Fatalf("seed %d invalid ball %+v", seed, g.ball.pos)
		}
	}
	t.Logf("8 seeded matches completed; aggregate goals home=%d away=%d", totalHome, totalAway)
}
