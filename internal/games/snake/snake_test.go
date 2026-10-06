package snake

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

// countingSource counts random draws so tests can prove the RNG is untouched.
type countingSource struct {
	rand.Source
	draws int
}

func (s *countingSource) Uint64() uint64 { s.draws++; return s.Source.Uint64() }

func seeded() (*Game, *countingSource) {
	src := &countingSource{Source: rand.NewPCG(1, 2)}
	g := newGame(rand.New(src))
	g.Start(80, 24)
	return g, src
}

// fixture builds a controlled mid-game state. body is head first.
func fixture(body []point, dir, food point) (*Game, *countingSource) {
	g, src := seeded()
	g.body = append([]point(nil), body...)
	g.dir, g.food = dir, food
	return g, src
}

func (g *Game) head() point { return g.body[0] }

func TestStartState(t *testing.T) {
	g, _ := seeded()
	want := []point{{18, 9}, {17, 9}, {16, 9}}
	if fmt.Sprint(g.body) != fmt.Sprint(want) || g.dir != right {
		t.Fatalf("body=%v dir=%v", g.body, g.dir)
	}
	if g.Score() != 0 || g.Level() != 1 || g.interval() != 180*time.Millisecond || g.Finished() {
		t.Fatalf("score=%d level=%d interval=%v finished=%v", g.Score(), g.Level(), g.interval(), g.Finished())
	}
	assertFoodValid(t, g)
}

func assertFoodValid(t *testing.T, g *Game) {
	t.Helper()
	if g.food.x < 0 || g.food.x >= Cols || g.food.y < 0 || g.food.y >= Rows {
		t.Fatalf("food %v off the board", g.food)
	}
	for _, c := range g.body {
		if c == g.food {
			t.Fatalf("food %v on the snake", g.food)
		}
	}
}

func TestMovementTiming(t *testing.T) {
	g, _ := fixture([]point{{5, 5}, {4, 5}, {3, 5}}, right, point{30, 15})
	g.Update(179 * time.Millisecond)
	if g.head() != (point{5, 5}) {
		t.Fatal("moved before the interval")
	}
	g.Update(time.Millisecond)
	if g.head() != (point{6, 5}) {
		t.Fatalf("head %v after exactly one interval", g.head())
	}
	g.Update(90 * time.Millisecond) // remainder carries over
	g.Update(90 * time.Millisecond)
	if g.head() != (point{7, 5}) {
		t.Fatalf("head %v after two half intervals", g.head())
	}
	g.Update(3*180*time.Millisecond + 50*time.Millisecond) // three whole steps, 50 ms left
	if g.head() != (point{10, 5}) || g.elapsed != 50*time.Millisecond {
		t.Fatalf("head %v elapsed %v", g.head(), g.elapsed)
	}
	g.Update(0)
	g.Update(-time.Second)
	if g.head() != (point{10, 5}) || g.elapsed != 50*time.Millisecond {
		t.Fatal("nonpositive dt changed the game")
	}
}

func TestTurnQueue(t *testing.T) {
	start := []point{{10, 9}, {9, 9}, {8, 9}}
	step := 180 * time.Millisecond

	t.Run("duplicate and reverse ignored", func(t *testing.T) {
		g, _ := fixture(start, right, point{0, 0})
		g.HandleInput(engine.KeyRight)
		g.HandleInput(engine.KeyLeft)
		if g.queued != 0 {
			t.Fatalf("queued %d", g.queued)
		}
		g.Update(step)
		if g.head() != (point{11, 9}) {
			t.Fatalf("head %v", g.head())
		}
	})
	t.Run("right up left becomes up then left", func(t *testing.T) {
		g, _ := fixture(start, right, point{0, 0})
		g.HandleInput(engine.KeyUp)
		g.HandleInput(engine.KeyLeft) // valid after Up, not a reversal of Up
		g.Update(step)
		if g.head() != (point{10, 8}) {
			t.Fatalf("first step head %v, want up to 10,8", g.head())
		}
		g.Update(step)
		if g.head() != (point{9, 8}) || g.Finished() {
			t.Fatalf("second step head %v finished %v, want left to 9,8", g.head(), g.Finished())
		}
	})
	t.Run("queue is bounded and validated against last queued", func(t *testing.T) {
		g, _ := fixture(start, right, point{0, 0})
		g.HandleInput(engine.KeyUp)
		g.HandleInput(engine.KeyDown) // opposite of queued Up
		g.HandleInput(engine.KeyUp)   // duplicate of queued Up
		g.HandleInput(engine.KeyLeft)
		g.HandleInput(engine.KeyDown) // queue full
		if g.queued != 2 || g.queue[0] != up || g.queue[1] != left {
			t.Fatalf("queue %v (%d)", g.queue, g.queued)
		}
		g.Update(step)
		if g.queued != 1 {
			t.Fatal("more than one turn applied in a step")
		}
	})
	t.Run("non-direction keys ignored while playing", func(t *testing.T) {
		g, _ := fixture(start, right, point{0, 0})
		g.HandleInput(engine.KeySelect)
		g.HandleInput(engine.KeyPause)
		if g.queued != 0 || g.Finished() || g.head() != (point{10, 9}) {
			t.Fatal("non-direction key changed the game")
		}
	})
}

func TestEatingGrowsByOneAndScores(t *testing.T) {
	g, _ := fixture([]point{{5, 5}, {4, 5}, {3, 5}}, right, point{6, 5})
	g.Update(180 * time.Millisecond)
	if len(g.body) != 4 || g.Score() != 10 || g.head() != (point{6, 5}) || g.body[3] != (point{3, 5}) {
		t.Fatalf("after eating: body=%v score=%d", g.body, g.Score())
	}
	assertFoodValid(t, g)
	g.food = point{30, 15}
	g.Update(180 * time.Millisecond)
	if len(g.body) != 4 || g.Score() != 10 {
		t.Fatalf("grew without food: body=%v", g.body)
	}
}

func TestLevelAndSpeedThresholds(t *testing.T) {
	for _, tt := range []struct {
		eaten, level int
		interval     time.Duration
	}{
		{0, 1, 180 * time.Millisecond},
		{4, 1, 180 * time.Millisecond},
		{5, 2, 165 * time.Millisecond},
		{9, 2, 165 * time.Millisecond},
		{10, 3, 150 * time.Millisecond},
		{30, 7, 90 * time.Millisecond},
		{35, 8, 80 * time.Millisecond}, // 75 ms by formula, capped
		{200, 41, 80 * time.Millisecond},
	} {
		g, _ := seeded()
		g.eaten = tt.eaten
		if g.Level() != tt.level || g.interval() != tt.interval {
			t.Errorf("eaten %d: level %d interval %v, want %d %v", tt.eaten, g.Level(), g.interval(), tt.level, tt.interval)
		}
	}
}

func TestLevelUpSpeedsUpTheNextStep(t *testing.T) {
	g, _ := fixture([]point{{5, 5}, {4, 5}, {3, 5}}, right, point{6, 5})
	g.eaten = 4
	g.Update(180 * time.Millisecond) // eats the 5th food: level 2
	g.food = point{30, 15}
	g.Update(164 * time.Millisecond)
	if g.head() != (point{6, 5}) {
		t.Fatal("stepped before the new 165 ms interval")
	}
	g.Update(time.Millisecond)
	if g.head() != (point{7, 5}) {
		t.Fatalf("head %v: new interval not applied", g.head())
	}
}

func TestWallCollisionLeavesRenderableGameOver(t *testing.T) {
	for _, tc := range []struct {
		body []point
		dir  point
	}{
		{[]point{{35, 9}, {34, 9}, {33, 9}}, right},
		{[]point{{0, 9}, {1, 9}, {2, 9}}, left},
		{[]point{{5, 0}, {5, 1}, {5, 2}}, up},
		{[]point{{5, 17}, {5, 16}, {5, 15}}, down},
	} {
		g, _ := fixture(tc.body, tc.dir, point{20, 3})
		g.Update(time.Second)
		if !g.Finished() || g.state != lost || fmt.Sprint(g.body) != fmt.Sprint(tc.body) {
			t.Fatalf("%v: state %v body %v", tc.dir, g.state, g.body)
		}
		out := renderStrict(t, g, 80, 24)
		if !strings.Contains(out, "GAME OVER") || !strings.Contains(out, "Enter: play again") {
			t.Fatalf("game over screen:\n%s", out)
		}
	}
}

func TestSelfCollision(t *testing.T) {
	// Head at 5,5 moving up; turning left runs into 4,5, a body cell that
	// stays occupied (it is not the tail).
	body := []point{{5, 5}, {5, 6}, {4, 6}, {4, 5}, {4, 4}, {3, 4}}
	g, _ := fixture(body, up, point{20, 3})
	g.HandleInput(engine.KeyLeft)
	g.Update(180 * time.Millisecond)
	if g.state != lost || fmt.Sprint(g.body) != fmt.Sprint(body) {
		t.Fatalf("state %v body %v", g.state, g.body)
	}
}

func TestMovingIntoVacatingTailIsLegal(t *testing.T) {
	// A 2x2 loop: head 1,0 moving up, tail at 0,0. Moving left enters the
	// tail cell as the tail leaves it.
	body := []point{{1, 0}, {1, 1}, {0, 1}, {0, 0}}
	g, _ := fixture(body, up, point{20, 10})
	g.HandleInput(engine.KeyLeft)
	g.Update(180 * time.Millisecond)
	if g.Finished() || g.head() != (point{0, 0}) || len(g.body) != 4 || g.body[3] != (point{0, 1}) {
		t.Fatalf("state %v body %v", g.state, g.body)
	}
}

// serpentine returns every board cell in a single snake-able path.
func serpentine() []point {
	var path []point
	for y := range Rows {
		for i := range Cols {
			x := i
			if y%2 == 1 {
				x = Cols - 1 - i
			}
			path = append(path, point{x, y})
		}
	}
	return path
}

func reversed(ps []point) []point {
	out := make([]point, len(ps))
	for i, p := range ps {
		out[len(ps)-1-i] = p
	}
	return out
}

func TestFoodPlacement(t *testing.T) {
	g, src := seeded()
	for range 500 {
		g.spawnFood()
		assertFoodValid(t, g)
	}
	if src.draws == 0 {
		t.Fatal("food placement did not use the injected source")
	}

	// Only one free cell left: food must go exactly there.
	path := serpentine()
	g.body = reversed(path[:len(path)-1])
	g.spawnFood()
	if g.food != path[len(path)-1] || g.Finished() {
		t.Fatalf("food %v, want the only free cell %v", g.food, path[len(path)-1])
	}
}

func TestFillingTheBoardCompletesTheRun(t *testing.T) {
	path := serpentine()
	last, prev := path[len(path)-1], path[len(path)-2]
	g, src := fixture(reversed(path[:len(path)-1]), point{last.x - prev.x, last.y - prev.y}, last)
	g.eaten = len(g.body) - startLength
	draws := src.draws
	g.Update(time.Second)
	if g.state != won || !g.Finished() || len(g.body) != Cols*Rows || src.draws != draws {
		t.Fatalf("state %v length %d draws %d->%d", g.state, len(g.body), draws, src.draws)
	}
	out := renderStrict(t, g, 80, 24)
	if !strings.Contains(out, "BOARD COMPLETE") || !strings.Contains(out, "Enter: play again") {
		t.Fatalf("win screen:\n%s", out)
	}
}

func TestNothingAdvancesAfterTheRunEnds(t *testing.T) {
	g, src := fixture([]point{{35, 9}, {34, 9}, {33, 9}}, right, point{20, 3})
	g.Update(180 * time.Millisecond)
	body, food, draws, elapsed := fmt.Sprint(g.body), g.food, src.draws, g.elapsed
	g.Update(time.Minute)
	for _, k := range []engine.Key{engine.KeyUp, engine.KeyLeft, engine.KeyDown, engine.KeyPause} {
		g.HandleInput(k)
	}
	if fmt.Sprint(g.body) != body || g.food != food || src.draws != draws || g.elapsed != elapsed || g.queued != 0 {
		t.Fatal("finished run changed")
	}
}

func TestEnterRestartsAFreshRun(t *testing.T) {
	g, _ := fixture([]point{{35, 9}, {34, 9}, {33, 9}}, right, point{20, 3})
	g.eaten = 7
	g.HandleInput(engine.KeyUp)
	g.Update(250 * time.Millisecond) // one step up to 35,8, 70 ms left over
	g.HandleInput(engine.KeyRight)
	g.Update(time.Second) // right into the wall
	if !g.Finished() {
		t.Fatal("setup: run did not end")
	}
	g.HandleInput(engine.KeySelect)
	want := []point{{18, 9}, {17, 9}, {16, 9}}
	if g.Finished() || g.Score() != 0 || g.Level() != 1 || g.queued != 0 || g.elapsed != 0 ||
		g.dir != right || fmt.Sprint(g.body) != fmt.Sprint(want) {
		t.Fatalf("after restart: state=%v score=%d queued=%d elapsed=%v dir=%v body=%v",
			g.state, g.Score(), g.queued, g.elapsed, g.dir, g.body)
	}
	assertFoodValid(t, g)
	g.Update(179 * time.Millisecond)
	if g.head() != (point{18, 9}) {
		t.Fatal("restart kept leftover movement time")
	}
}

// strictCanvas fails on out-of-bounds, non-ASCII or footer-row writes and
// records the text grid.
type strictCanvas struct {
	t    *testing.T
	w, h int
	rows [][]rune
}

func newStrict(t *testing.T, w, h int) *strictCanvas {
	c := &strictCanvas{t: t, w: w, h: h, rows: make([][]rune, h)}
	for y := range c.rows {
		c.rows[y] = []rune(strings.Repeat(" ", w))
	}
	return c
}

func (c *strictCanvas) Size() (int, int) { return c.w, c.h }
func (c *strictCanvas) Cell(x, y int, r rune, _ engine.Color) {
	switch {
	case x < 0 || y < 0 || x >= c.w || y >= c.h:
		c.t.Errorf("cell %d,%d outside %dx%d", x, y, c.w, c.h)
		return
	case y == c.h-1:
		c.t.Errorf("wrote %q to the footer row at %d,%d", r, x, y)
	case engine.Printable(r) != r:
		c.t.Errorf("non-ASCII glyph %q", r)
	}
	c.rows[y][x] = r
}
func (c *strictCanvas) Text(x, y int, s string, color engine.Color) {
	for _, r := range s {
		c.Cell(x, y, r, color)
		x++
	}
}
func (c *strictCanvas) String() string {
	lines := make([]string, len(c.rows))
	for i, r := range c.rows {
		lines[i] = string(r)
	}
	return strings.Join(lines, "\n")
}

func renderStrict(t *testing.T, g *Game, w, h int) string {
	t.Helper()
	c := newStrict(t, w, h)
	g.Render(c)
	return c.String()
}

func TestRenderFitsAndShowsOneHeadAndFood(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {81, 25}, {120, 40}} {
		g, _ := seeded()
		g.Resize(size[0], size[1])
		out := renderStrict(t, g, size[0], size[1])
		if strings.Count(out, "▣▣") != 1 || strings.Count(out, "✱ ") != 1 || strings.Count(out, "██") != 2 {
			t.Errorf("%v: want one head, one food, two body cells:\n%s", size, out)
		}
		for _, want := range []string{"> SNAKE", "Score: 0000", "Level: 1", "PLAYING", "Pause: Space", "Q / Esc: menu"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: HUD lacks %q", size, want)
			}
		}
	}
}

func TestResizeChangesLayoutOnly(t *testing.T) {
	g, _ := seeded()
	e := engine.New(g)
	e.Resize(80, 24)
	e.Advance(time.Second) // dropped first frame
	e.Advance(100 * time.Millisecond)
	e.Advance(100 * time.Millisecond) // one step
	body, food := fmt.Sprint(g.body), g.food
	at80 := renderStrict(t, g, 80, 24)

	e.Resize(30, 8) // undersized: suspended
	e.Advance(time.Minute)
	e.Input(engine.Event{Key: engine.KeyUp}) // invisible: not buffered
	e.Resize(120, 40)
	e.Advance(time.Minute) // dropped: spans the undersized period
	if fmt.Sprint(g.body) != body || g.food != food || g.queued != 0 || g.Finished() {
		t.Fatalf("resize changed the board: body %v food %v queued %d", g.body, g.food, g.queued)
	}
	at120 := renderStrict(t, g, 120, 40)
	if headCol(at80) == headCol(at120) {
		t.Fatal("board not re-centered on the larger screen")
	}
}

func headCol(screen string) [2]int {
	for y, line := range strings.Split(screen, "\n") {
		if x := strings.Index(line, "▣▣"); x >= 0 {
			return [2]int{x, y}
		}
	}
	return [2]int{-1, -1}
}

func TestEngineIntegrationPauseAndRestart(t *testing.T) {
	g, src := seeded()
	e := engine.New(g)
	e.Resize(80, 24) // Start resets the run; set up the board afterwards
	g.body, g.dir, g.food = []point{{30, 9}, {29, 9}, {28, 9}}, right, point{2, 2}
	e.Advance(time.Second) // dropped first frame
	draws := src.draws

	e.Input(engine.Event{Key: engine.KeyPause})
	e.Input(engine.Event{Key: engine.KeyUp}) // not buffered while paused
	e.Resize(40, 10)
	e.Resize(80, 24)
	for range 50 {
		e.Advance(100 * time.Millisecond)
	}
	if !e.Paused() || g.head() != (point{30, 9}) || g.queued != 0 || src.draws != draws {
		t.Fatalf("paused game advanced: paused=%v head=%v queued=%d draws=%d", e.Paused(), g.head(), g.queued, src.draws)
	}
	e.Input(engine.Event{Key: engine.KeyPause})
	e.Advance(time.Minute) // dropped: spans the pause
	if g.head() != (point{30, 9}) {
		t.Fatal("pause time replayed")
	}
	for range 20 {
		e.Advance(100 * time.Millisecond) // 2 s: six steps reach the wall at x=35
	}
	if !g.Finished() {
		t.Fatalf("run should have ended at the wall, head %v", g.head())
	}

	// On the end screen Space does nothing, so Enter still restarts.
	e.Input(engine.Event{Key: engine.KeyPause})
	if e.Paused() {
		t.Fatal("end screen was paused")
	}
	e.Input(engine.Event{Key: engine.KeySelect})
	if g.Finished() || g.Score() != 0 || g.head() != (point{18, 9}) || e.Paused() {
		t.Fatalf("restart failed: finished=%v head=%v", g.Finished(), g.head())
	}
	e.Input(engine.Event{Key: engine.KeyPause})
	if !e.Paused() {
		t.Fatal("pause does not work after restart")
	}
}
