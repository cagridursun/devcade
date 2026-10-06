package blockdrop

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

const ms = time.Millisecond

// countingSource counts random draws so tests can prove the RNG is used.
type countingSource struct {
	rand.Source
	draws int
}

func (s *countingSource) Uint64() uint64 { s.draws++; return s.Source.Uint64() }

func seededWith(a, b uint64) (*Game, *countingSource) {
	src := &countingSource{Source: rand.NewPCG(a, b)}
	g := newGame(rand.New(src))
	g.Start(80, 24)
	return g, src
}

func seeded() (*Game, *countingSource) { return seededWith(1, 2) }

// put replaces the current piece with fresh per-piece timers, as a spawn
// would. Set up the board first: put records whether the piece is grounded.
func (g *Game) put(k kind, rot, x, y int) {
	g.cur = piece{kind: k, rot: rot, x: x, y: y}
	g.fall, g.lockTime, g.resets = 0, 0, 0
	g.airborne, g.deepest, g.lockNow = true, -1, false
	g.settle()
}

// fill sets the bottom rows of the board from pictures, top to bottom:
// 'x' is a settled cell and anything else is empty.
func (g *Game) fill(rows ...string) {
	for i, row := range rows {
		y := Rows - len(rows) + i
		for x := range Cols {
			g.board[y][x] = none
			if x < len(row) && row[x] == 'x' {
				g.board[y][x] = pieceJ
			}
		}
	}
}

// rowString renders board row y as 'x' (settled) and '.' (empty).
func (g *Game) rowString(y int) string {
	var b strings.Builder
	for _, k := range g.board[y] {
		if k == none {
			b.WriteByte('.')
		} else {
			b.WriteByte('x')
		}
	}
	return b.String()
}

func (g *Game) settled() int {
	n := 0
	for y := range Rows {
		for _, k := range g.board[y] {
			if k != none {
				n++
			}
		}
	}
	return n
}

// snapshot captures every piece of gameplay state for equality checks.
func snapshot(g *Game) string {
	return fmt.Sprintf("%v|%+v|%v|%v|%d|%d|%d|%v|%v|%d|%v|%d|%v|%v",
		g.board, g.cur, g.next, g.bag, g.bagLeft, g.score, g.lines,
		g.fall, g.lockTime, g.resets, g.airborne, g.deepest, g.lockNow, g.state)
}

func TestStartState(t *testing.T) {
	g, _ := seeded()
	if g.Score() != 0 || g.Lines() != 0 || g.Level() != 1 || g.Finished() || g.settled() != 0 {
		t.Fatalf("score=%d lines=%d level=%d finished=%v", g.Score(), g.Lines(), g.Level(), g.Finished())
	}
	if g.interval() != 700*ms || g.fall != 0 || g.lockTime != 0 || g.resets != 0 {
		t.Fatalf("interval=%v fall=%v lock=%v resets=%d", g.interval(), g.fall, g.lockTime, g.resets)
	}
	if g.cur != spawnPiece(g.cur.kind) || g.bagLeft != 5 {
		t.Fatalf("cur=%+v bagLeft=%d", g.cur, g.bagLeft)
	}
}

// picture draws the box cells of a rotation state as n rows joined by '|'.
func picture(cells [4]point, n int) string {
	rows := make([][]byte, n)
	for y := range rows {
		rows[y] = []byte(strings.Repeat(".", n))
	}
	for _, p := range cells {
		rows[p.y][p.x] = '#'
	}
	parts := make([]string, n)
	for i, r := range rows {
		parts[i] = string(r)
	}
	return strings.Join(parts, "|")
}

func TestRotationTables(t *testing.T) {
	want := map[kind][4]string{
		pieceI: {"....|####|....|....", "..#.|..#.|..#.|..#.", "....|....|####|....", ".#..|.#..|.#..|.#.."},
		pieceO: {"##|##", "##|##", "##|##", "##|##"},
		pieceT: {".#.|###|...", ".#.|.##|.#.", "...|###|.#.", ".#.|##.|.#."},
		pieceS: {".##|##.|...", ".#.|.##|..#", "...|.##|##.", "#..|##.|.#."},
		pieceZ: {"##.|.##|...", "..#|.##|.#.", "...|##.|.##", ".#.|##.|#.."},
		pieceJ: {"#..|###|...", ".##|.#.|.#.", "...|###|..#", ".#.|.#.|##."},
		pieceL: {"..#|###|...", ".#.|.#.|.##", "...|###|#..", "##.|.#.|.#."},
	}
	for k, states := range want {
		for r, w := range states {
			if got := picture(shapes[k][r], base[k].n); got != w {
				t.Errorf("kind %d state %d = %s, want %s", k, r, got, w)
			}
		}
	}
}

func TestRotationThroughInput(t *testing.T) {
	for k := pieceI; k < kinds; k++ {
		g, _ := seeded()
		g.put(k, 0, 3, 8)
		start := g.cur
		for i := 1; i <= 4; i++ {
			g.HandleInput(engine.KeyUp)
			wantRot := i % 4
			if k == pieceO {
				wantRot = 0
			}
			if g.cur.rot != wantRot || g.cur.x != 3 || g.cur.y != 8 {
				t.Fatalf("kind %d after %d cw: %+v", k, i, g.cur)
			}
		}
		g.HandleInput(engine.KeyAction)
		if k == pieceO {
			if g.cur != start || g.resets != 0 {
				t.Fatalf("O rotation changed the piece: %+v resets %d", g.cur, g.resets)
			}
			continue
		}
		if g.cur.rot != 3 {
			t.Fatalf("kind %d: ccw from state 0 gave state %d", k, g.cur.rot)
		}
		g.HandleInput(engine.KeyAction)
		g.HandleInput(engine.KeyAction)
		g.HandleInput(engine.KeyAction)
		if g.cur != start {
			t.Fatalf("kind %d: four ccw turns gave %+v", k, g.cur)
		}
	}
}

func TestORotationIsANoOpEvenWhenGrounded(t *testing.T) {
	g, _ := seeded()
	g.put(pieceO, 0, 0, 20)
	g.Update(300 * ms)
	g.HandleInput(engine.KeyUp)
	g.HandleInput(engine.KeyAction)
	if g.cur != (piece{pieceO, 0, 0, 20}) || g.resets != 0 || g.lockTime != 300*ms {
		t.Fatalf("O rotation: %+v resets %d lock %v", g.cur, g.resets, g.lockTime)
	}
}

func TestSpawnPositions(t *testing.T) {
	want := map[kind]string{
		pieceI: "[{3 2} {4 2} {5 2} {6 2}]",
		pieceO: "[{4 2} {5 2} {4 3} {5 3}]",
		pieceT: "[{4 2} {3 3} {4 3} {5 3}]",
		pieceS: "[{4 2} {5 2} {3 3} {4 3}]",
		pieceZ: "[{3 2} {4 2} {4 3} {5 3}]",
		pieceJ: "[{3 2} {3 3} {4 3} {5 3}]",
		pieceL: "[{5 2} {3 3} {4 3} {5 3}]",
	}
	for k, w := range want {
		if got := sortedCells(spawnPiece(k)); got != w {
			t.Errorf("kind %d spawns at %s, want %s", k, got, w)
		}
	}
}

func TestBagContentsAndReproducibility(t *testing.T) {
	sequence := func(a, b uint64) ([]kind, int) {
		g, src := seededWith(a, b)
		seq := []kind{g.cur.kind, g.next}
		for len(seq) < 70 {
			seq = append(seq, g.draw())
		}
		return seq, src.draws
	}
	seq, draws := sequence(1, 2)
	all := []kind{pieceI, pieceO, pieceT, pieceS, pieceZ, pieceJ, pieceL}
	for i := 0; i < len(seq); i += 7 {
		bag := slices.Clone(seq[i : i+7])
		slices.Sort(bag)
		if !slices.Equal(bag, all) {
			t.Fatalf("bag %d = %v, want each shape once", i/7, seq[i:i+7])
		}
	}
	if draws == 0 {
		t.Fatal("bag did not use the injected source")
	}
	again, _ := sequence(1, 2)
	if !slices.Equal(seq, again) {
		t.Fatal("same seed gave a different sequence")
	}
	other, _ := sequence(3, 4)
	if slices.Equal(seq, other) {
		t.Fatal("different seeds gave the same 70 pieces")
	}
}

func TestWallKicks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rows  []string
		from  piece
		key   engine.Key
		want  piece
		cells string // expected cells, checked when set
	}{
		{"none needed", nil, piece{pieceT, 0, 4, 10}, engine.KeyUp, piece{pieceT, 1, 4, 10}, ""},
		{"right wall: one left", nil, piece{pieceI, 1, 7, 5}, engine.KeyUp, piece{pieceI, 2, 6, 5}, "[{6 7} {7 7} {8 7} {9 7}]"},
		{"left wall: one right", nil, piece{pieceI, 3, -1, 5}, engine.KeyUp, piece{pieceI, 0, 0, 5}, "[{0 6} {1 6} {2 6} {3 6}]"},
		{"right wall: two left", nil, piece{pieceI, 3, 8, 5}, engine.KeyUp, piece{pieceI, 0, 6, 5}, "[{6 6} {7 6} {8 6} {9 6}]"},
		{"left wall ccw: two right", nil, piece{pieceI, 1, -2, 5}, engine.KeyAction, piece{pieceI, 0, 0, 5}, ""},
		{"floor: one up", nil, piece{pieceT, 0, 4, 20}, engine.KeyUp, piece{pieceT, 1, 4, 19}, "[{5 19} {5 20} {6 20} {5 21}]"},
		{"stack: one left", []string{".....x"}, piece{pieceT, 0, 4, 19}, engine.KeyUp, piece{pieceT, 1, 3, 19}, "[{4 19} {4 20} {5 20} {4 21}]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := seeded()
			g.fill(tc.rows...)
			g.cur = tc.from
			g.HandleInput(tc.key)
			if g.cur != tc.want {
				t.Fatalf("got %+v, want %+v", g.cur, tc.want)
			}
			if tc.cells != "" && sortedCells(g.cur) != tc.cells {
				t.Fatalf("cells %v, want %s", g.cur.cells(), tc.cells)
			}
		})
	}
}

// sortedCells prints a piece's board cells ordered by row, then column.
func sortedCells(p piece) string {
	cells := p.cells()
	slices.SortFunc(cells[:], func(a, b point) int {
		if a.y != b.y {
			return a.y - b.y
		}
		return a.x - b.x
	})
	return fmt.Sprint(cells)
}

func TestFailedRotationKeepsStateAndTimers(t *testing.T) {
	// A vertical I at the bottom of a one-column well: every horizontal
	// offset and the one-row lift intersect the walls.
	g, _ := seeded()
	well := "xxxxx.xxxx"
	g.fill(well, well, well, well, well, well)
	g.put(pieceI, 1, 3, 18)
	g.Update(250 * ms)
	before := snapshot(g)
	g.HandleInput(engine.KeyUp)
	g.HandleInput(engine.KeyAction)
	g.HandleInput(engine.KeyLeft)
	g.HandleInput(engine.KeyRight)
	g.HandleInput(engine.KeyDown)
	if snapshot(g) != before || g.lockTime != 250*ms || g.resets != 0 {
		t.Fatalf("failed actions changed the game: %+v lock %v resets %d", g.cur, g.lockTime, g.resets)
	}
}

func TestBoundsAndStackCollisions(t *testing.T) {
	g, _ := seeded()
	g.put(pieceO, 0, 0, 10)
	g.HandleInput(engine.KeyLeft)
	if g.cur.x != 0 {
		t.Fatal("moved through the left wall")
	}
	g.put(pieceO, 0, 8, 10)
	g.HandleInput(engine.KeyRight)
	if g.cur.x != 8 {
		t.Fatal("moved through the right wall")
	}
	g.board[11][3] = pieceL
	g.put(pieceO, 0, 4, 10)
	g.HandleInput(engine.KeyLeft)
	if g.cur.x != 4 {
		t.Fatal("moved into a settled cell")
	}
	g.HandleInput(engine.KeyRight)
	if g.cur.x != 5 {
		t.Fatal("legal move rejected")
	}
	// Upward kicks never leave the top of the hidden rows.
	g.put(pieceI, 0, 3, -1) // cells on row 0
	if !g.fits(g.cur) || g.fits(piece{pieceI, 0, 3, -2}) || g.fits(piece{pieceO, 0, 4, 21}) {
		t.Fatal("fits accepted a placement outside the board")
	}
	score := g.Score()
	g.put(pieceO, 0, 0, 20)
	g.HandleInput(engine.KeyDown)
	if g.cur.y != 20 || g.Score() != score {
		t.Fatal("soft drop through the floor moved or scored")
	}
}

func TestSoftDropScoresRowsActuallyMoved(t *testing.T) {
	g, _ := seeded()
	g.put(pieceO, 0, 4, 17) // three rows above the floor
	g.fall = 500 * ms
	for range 5 {
		g.HandleInput(engine.KeyDown)
	}
	if g.cur.y != 20 || g.Score() != 3 || g.fall != 0 {
		t.Fatalf("y=%d score=%d fall=%v", g.cur.y, g.Score(), g.fall)
	}
}

func TestProjectionAndHardDrop(t *testing.T) {
	g, _ := seeded()
	g.fill(
		"....x.....",
		"...xxx....",
		"x.xxxx.xxx",
	)
	g.put(pieceT, 0, 3, 5) // cells (4,5) (3,6) (4,6) (5,6)
	land := g.landing()
	if land != (piece{pieceT, 0, 3, 17}) {
		t.Fatalf("landing %+v", land)
	}
	out := boardArea(t, renderStrict(t, g, 80, 24))
	if n := strings.Count(out, "░░░"); n != 4 {
		t.Fatalf("projection cells %d, want 4:\n%s", n, out)
	}
	g.HandleInput(engine.KeySelect)
	if g.Score() != 2*12 {
		t.Fatalf("hard drop score %d, want 24", g.Score())
	}
	for _, c := range land.cells() {
		if g.board[c.y][c.x] != pieceT {
			t.Fatalf("cell %v not locked", c)
		}
	}
	if g.cur != spawnPiece(g.cur.kind) || g.lockTime != 0 || g.fall != 0 {
		t.Fatalf("next piece not spawned fresh: %+v", g.cur)
	}

	// A hard drop that moves no rows scores nothing but still locks.
	g.put(pieceO, 0, 0, 19) // already resting on the settled cell at (0, 21)
	g.HandleInput(engine.KeySelect)
	if g.Score() != 24 || g.board[20][0] != pieceO {
		t.Fatalf("zero-row hard drop: score %d", g.Score())
	}
}

func TestProjectionHiddenWhenGrounded(t *testing.T) {
	g, _ := seeded()
	g.put(pieceO, 0, 4, 20)
	if out := boardArea(t, renderStrict(t, g, 80, 24)); strings.Contains(out, "░░░") {
		t.Fatalf("projection drawn under a grounded piece:\n%s", out)
	}
}

func TestLevelAndGravityIntervals(t *testing.T) {
	for _, tt := range []struct {
		lines, level int
		interval     time.Duration
	}{
		{0, 1, 700 * ms},
		{9, 1, 700 * ms},
		{10, 2, 650 * ms},
		{19, 2, 650 * ms},
		{50, 6, 450 * ms},
		{120, 13, 100 * ms},
		{130, 14, 100 * ms}, // 50 ms by formula, capped
		{999, 100, 100 * ms},
	} {
		g, _ := seeded()
		g.lines = tt.lines
		if g.Level() != tt.level || g.interval() != tt.interval {
			t.Errorf("lines %d: level %d interval %v, want %d %v", tt.lines, g.Level(), g.interval(), tt.level, tt.interval)
		}
	}
}

func TestGravityTiming(t *testing.T) {
	g, _ := seeded()
	g.put(pieceT, 0, 3, 5)
	g.Update(699 * ms)
	if g.cur.y != 5 {
		t.Fatal("fell before the interval")
	}
	g.Update(ms)
	if g.cur.y != 6 || g.fall != 0 {
		t.Fatalf("y=%d fall=%v after one interval", g.cur.y, g.fall)
	}
	g.Update(3*700*ms + 50*ms)
	if g.cur.y != 9 || g.fall != 50*ms {
		t.Fatalf("y=%d fall=%v", g.cur.y, g.fall)
	}
	g.Update(0)
	g.Update(-time.Second)
	if g.cur.y != 9 || g.fall != 50*ms {
		t.Fatal("nonpositive dt changed the game")
	}
}

func TestGravityAndLockProcessedInOrder(t *testing.T) {
	setup := func() *Game {
		g, _ := seeded()
		g.put(pieceO, 0, 4, 18) // two rows above the floor
		g.fall = 650 * ms
		return g
	}
	// Falls at 50 ms and 750 ms (now grounded), locks at 1150 ms, and the
	// last 50 ms already count toward the next piece's first gravity step.
	g := setup()
	g.Update(1200 * ms)
	if g.rowString(20) != "....xx...." || g.rowString(21) != "....xx...." {
		t.Fatalf("rows:\n%s\n%s", g.rowString(20), g.rowString(21))
	}
	if g.cur != spawnPiece(g.cur.kind) || g.fall != 50*ms || g.lockTime != 0 {
		t.Fatalf("next piece %+v fall %v lock %v", g.cur, g.fall, g.lockTime)
	}
	want := snapshot(g)

	chunkings := map[string][]time.Duration{
		"10ms":   repeat(10*ms, 120),
		"7ms":    append(repeat(7*ms, 171), 3*ms),
		"100ms":  repeat(100*ms, 12),
		"uneven": {49 * ms, 2 * ms, 699 * ms, ms, 399 * ms, ms, 49 * ms},
	}
	for name, steps := range chunkings {
		g := setup()
		for _, dt := range steps {
			g.Update(dt)
		}
		if got := snapshot(g); got != want {
			t.Errorf("%s chunking diverged:\n got %s\nwant %s", name, got, want)
		}
	}

	// Stopping just before the lock leaves the piece grounded with 399 ms.
	g = setup()
	g.Update(1149 * ms)
	if g.cur != (piece{pieceO, 0, 4, 20}) || g.lockTime != 399*ms || g.fall != 0 {
		t.Fatalf("before lock: %+v lock %v fall %v", g.cur, g.lockTime, g.fall)
	}
}

func repeat(dt time.Duration, n int) []time.Duration {
	out := make([]time.Duration, n)
	for i := range out {
		out[i] = dt
	}
	return out
}

func TestFrameChunkingNeverChangesALongRun(t *testing.T) {
	// Gravity alone stacks pieces in the middle until the run tops out.
	const total = 10 * time.Minute
	run := func(next func() time.Duration) *Game {
		g, _ := seeded()
		for spent := time.Duration(0); spent < total; {
			dt := min(next(), total-spent)
			g.Update(dt)
			spent += dt
		}
		return g
	}
	want := run(func() time.Duration { return 100 * ms })
	if !want.Finished() || want.settled() == 0 {
		t.Fatal("setup: the gravity-only run should top out")
	}
	chunks := rand.New(rand.NewPCG(7, 7))
	got := run(func() time.Duration { return time.Duration(1+chunks.IntN(100)) * ms })
	if snapshot(got) != snapshot(want) {
		t.Fatalf("random chunking diverged:\n got %s\nwant %s", snapshot(got), snapshot(want))
	}
}

func TestLockDelayResetBudget(t *testing.T) {
	g, _ := seeded()
	g.put(pieceT, 0, 4, 20) // resting on the floor
	g.Update(399 * ms)
	for i := 1; i <= maxResets; i++ {
		if g.cur.y != 20 || g.lockTime != 399*ms {
			t.Fatalf("reset %d: piece locked early or timer wrong: %+v lock %v", i, g.cur, g.lockTime)
		}
		key := engine.KeyLeft
		if i%2 == 0 {
			key = engine.KeyRight
		}
		g.HandleInput(key)
		if g.resets != i || g.lockTime != 0 {
			t.Fatalf("after move %d: resets %d lock %v", i, g.resets, g.lockTime)
		}
		g.Update(399 * ms)
	}
	// Budget spent: a further successful move does not postpone the lock.
	g.HandleInput(engine.KeyUp) // T rotates on the floor with a lift kick
	if g.resets != maxResets || g.lockTime != 399*ms || g.cur.rot != 1 {
		t.Fatalf("exhausted: resets %d lock %v piece %+v", g.resets, g.lockTime, g.cur)
	}
	locked := g.cur
	g.Update(ms)
	for _, c := range locked.cells() {
		if g.board[c.y][c.x] != pieceT {
			t.Fatalf("piece did not lock 400 ms after its last reset: %v", g.board[c.y])
		}
	}
	if g.resets != 0 || g.lockTime != 0 {
		t.Fatal("per-piece budget not reset on spawn")
	}
}

func TestFailedActionsDoNotResetLockDelay(t *testing.T) {
	g, _ := seeded()
	g.put(pieceT, 0, 0, 20) // grounded against the left wall
	g.Update(300 * ms)
	g.HandleInput(engine.KeyLeft)
	g.HandleInput(engine.KeyDown)
	if g.resets != 0 || g.lockTime != 300*ms || g.cur.x != 0 {
		t.Fatalf("failed actions reset: resets %d lock %v", g.resets, g.lockTime)
	}
	g.Update(100 * ms)
	if g.board[21][0] != pieceT {
		t.Fatal("piece did not lock at 400 ms")
	}
}

func TestLeavingSupportClearsGroundedTimeButNotTheBudget(t *testing.T) {
	g, _ := seeded()
	g.fill("xxxxxx....") // a ledge under columns 0..5
	g.put(pieceO, 0, 4, 19)
	g.Update(300 * ms)
	g.HandleInput(engine.KeyRight) // columns 5,6: still supported by column 5
	if g.resets != 1 || g.lockTime != 0 || !g.grounded() {
		t.Fatalf("grounded move: resets %d lock %v", g.resets, g.lockTime)
	}
	g.Update(300 * ms)
	g.HandleInput(engine.KeyRight) // columns 6,7: off the ledge
	if g.resets != 2 || g.lockTime != 0 || g.grounded() || g.lockNow {
		t.Fatalf("off the ledge: resets %d lock %v", g.resets, g.lockTime)
	}
	g.Update(700 * ms) // one gravity step lands it on the floor
	if g.cur.y != 20 || !g.grounded() || g.resets != 2 || g.lockTime != 0 {
		t.Fatalf("landed: %+v resets %d lock %v", g.cur, g.resets, g.lockTime)
	}
	g.Update(399 * ms)
	if g.cur.y != 20 {
		t.Fatal("locked before a full delay after landing")
	}
	g.Update(ms)
	if g.rowString(21) != "xxxxxxxx.." {
		t.Fatalf("bottom row %s", g.rowString(21))
	}
}

// exhaust spends the whole reset budget with grounded Left/Right shuffles,
// ending where it started.
func exhaust(t *testing.T, g *Game) {
	t.Helper()
	for i := range maxResets {
		key := engine.KeyLeft
		if i%2 == 1 {
			key = engine.KeyRight
		}
		g.HandleInput(key)
		g.Update(100 * ms)
	}
	if g.resets != maxResets || !g.grounded() || g.lockTime != 100*ms {
		t.Fatalf("setup: resets %d grounded %v lock %v", g.resets, g.grounded(), g.lockTime)
	}
}

func TestExhaustedTouchdownOnTheSameRowLocksAtOnce(t *testing.T) {
	// A vertical I on the floor (deepest row 21) turns flat one row up,
	// which leaves it airborne; gravity puts it back on row 21.
	g, _ := seeded()
	g.put(pieceI, 1, 2, 18)
	exhaust(t, g)
	g.HandleInput(engine.KeyUp)
	if g.cur != (piece{pieceI, 2, 2, 18}) || g.grounded() || g.resets != maxResets || g.deepest != 21 {
		t.Fatalf("after rotating: %+v grounded %v resets %d deepest %d", g.cur, g.grounded(), g.resets, g.deepest)
	}
	g.Update(699 * ms)
	if g.cur.y != 18 || g.settled() != 0 {
		t.Fatal("fell or locked early")
	}
	g.Update(ms) // lands on row 21 again and locks at the same instant
	if g.rowString(21) != "..xxxx...." || g.settled() != 4 {
		t.Fatalf("bottom row %s", g.rowString(21))
	}
}

func TestExhaustedTouchdownOnALowerRowGetsAFreshDelay(t *testing.T) {
	// An O shuffles on a ledge (deepest row 20) until the budget is spent,
	// then slides off and falls to the floor, one row lower.
	setup := func() *Game {
		g, _ := seeded()
		g.fill("xxxxxx....")
		g.put(pieceO, 0, 4, 19)
		exhaust(t, g)
		g.HandleInput(engine.KeyRight) // columns 5,6: still on the ledge, no reset
		g.HandleInput(engine.KeyRight) // columns 6,7: off the ledge
		if g.grounded() || g.resets != maxResets || g.lockTime != 0 || g.deepest != 20 {
			t.Fatalf("setup: grounded %v resets %d lock %v deepest %d", g.grounded(), g.resets, g.lockTime, g.deepest)
		}
		return g
	}
	g := setup()
	g.Update(700 * ms) // lands on the floor
	if g.cur.y != 20 || g.lockNow || g.lockTime != 0 || g.settled() != 6 || g.deepest != 21 {
		t.Fatalf("landed: %+v lockNow %v lock %v settled %d", g.cur, g.lockNow, g.lockTime, g.settled())
	}
	g.Update(200 * ms)
	g.HandleInput(engine.KeyRight) // grounded, but the budget is spent: no reset
	if g.cur.x != 7 || g.resets != maxResets || g.lockTime != 200*ms {
		t.Fatalf("move after touchdown: %+v resets %d lock %v", g.cur, g.resets, g.lockTime)
	}
	g.Update(199 * ms)
	if g.settled() != 6 {
		t.Fatal("locked before the fresh 400 ms window ended")
	}
	g.Update(ms)
	if g.rowString(21) != "xxxxxx.xx." || g.rowString(20) != ".......xx." {
		t.Fatalf("rows:\n%s\n%s", g.rowString(20), g.rowString(21))
	}

	// Without the move, frame chunking does not change when it locks.
	whole, chunked := setup(), setup()
	whole.Update(1100 * ms)
	for range 11 {
		chunked.Update(100 * ms)
	}
	if snapshot(whole) != snapshot(chunked) || whole.settled() != 10 {
		t.Fatalf("chunking diverged:\n%s\n%s", snapshot(whole), snapshot(chunked))
	}
}

func TestSimultaneousNonAdjacentClears(t *testing.T) {
	g, _ := seeded()
	g.fill(
		"x.........",
		"xxxx.xxxx.",
		"xxxx.xxxxx", // full once the I fills column 4
		".xxx.xxxxx",
		"xxxx.xxxxx", // full once the I fills column 4
	)
	g.put(pieceI, 1, 2, 5) // vertical in column 4
	g.HandleInput(engine.KeySelect)
	if g.Lines() != 2 || g.Score() != 2*13+300 {
		t.Fatalf("lines %d score %d", g.Lines(), g.Score())
	}
	for y, want := range map[int]string{
		21: ".xxxxxxxxx",
		20: "xxxxxxxxx.",
		19: "x.........",
		18: "..........",
	} {
		if got := g.rowString(y); got != want {
			t.Errorf("row %d = %s, want %s", y, got, want)
		}
	}
	if g.settled() != 19 {
		t.Fatalf("settled %d, want 19", g.settled())
	}
}

func TestClearScoresAtTheLevelBeforeTheClear(t *testing.T) {
	g, _ := seeded()
	g.lines = 9 // level 1; four more lines reach level 2
	row := "xxxx.xxxxx"
	g.fill(row, row, row, row)
	g.put(pieceI, 1, 2, 5)
	g.HandleInput(engine.KeySelect)
	if g.Lines() != 13 || g.Level() != 2 || g.Score() != 2*13+800*1 || g.settled() != 0 {
		t.Fatalf("lines %d level %d score %d settled %d", g.Lines(), g.Level(), g.Score(), g.settled())
	}
	if g.interval() != 650*ms {
		t.Fatal("level-up did not speed up gravity")
	}

	// At level 2 a single line is worth 200.
	g.fill("xxxxxx.xxx")
	g.put(pieceI, 1, 4, 18)
	g.HandleInput(engine.KeySelect)
	if g.Lines() != 14 || g.Score() != 826+200 {
		t.Fatalf("lines %d score %d", g.Lines(), g.Score())
	}
}

func TestBlockOutEndsTheRun(t *testing.T) {
	g, _ := seeded()
	g.board[2][4], g.board[3][4] = pieceJ, pieceJ // every spawn uses one of them
	g.put(pieceO, 0, 0, 20)
	g.HandleInput(engine.KeySelect)
	if !g.Finished() {
		t.Fatal("spawn into the stack did not end the run")
	}
	out := renderStrict(t, g, 80, 24)
	for _, want := range []string{"GAME OVER", "Final score 0", "Enter: play again"} {
		if !strings.Contains(out, want) {
			t.Fatalf("end screen lacks %q:\n%s", want, out)
		}
	}
}

func TestLockOutInHiddenRowsEndsTheRun(t *testing.T) {
	g, _ := seeded()
	for y := 2; y < Rows; y++ { // a tower reaching the top visible row
		g.board[y][4], g.board[y][5] = pieceJ, pieceJ
	}
	g.put(pieceO, 0, 4, 0) // e.g. lifted there by an upward kick
	if !g.fits(g.cur) {
		t.Fatal("setup: piece should fit in the hidden rows")
	}
	g.HandleInput(engine.KeySelect) // locks on rows 0-1, both hidden
	if !g.Finished() || g.board[1][4] != pieceO {
		t.Fatalf("lock in a hidden row: finished=%v", g.Finished())
	}
}

func TestTopOutIsJudgedAfterClearing(t *testing.T) {
	// A vertical I locks in column 9, rows 0..3, so two of its cells start
	// in the hidden rows. Rows 4..21 never clear (column 0 is empty).
	setup := func(row2 string) *Game {
		g, _ := seeded()
		for y := 4; y < Rows; y++ {
			for x := 1; x < Cols; x++ {
				g.board[y][x] = pieceJ
			}
		}
		for x := range Cols - 1 {
			g.board[3][x] = pieceJ
			if row2[x] == 'x' {
				g.board[2][x] = pieceJ
			}
		}
		g.put(pieceI, 1, 7, 0)
		g.HandleInput(engine.KeySelect)
		return g
	}

	// Rows 2 and 3 clear together and the hidden cells drop into them.
	g := setup("xxxxxxxxx")
	if g.Finished() || g.Lines() != 2 || g.rowString(2) != ".........x" || g.rowString(3) != ".........x" {
		t.Fatalf("finished=%v lines=%d rows 2,3: %s %s", g.Finished(), g.Lines(), g.rowString(2), g.rowString(3))
	}

	// Only row 3 clears: one I cell is still hidden after compaction.
	g = setup("xxxxxxxx.")
	if !g.Finished() || g.Lines() != 1 || g.board[1][9] != pieceI {
		t.Fatalf("finished=%v lines=%d row 1: %s", g.Finished(), g.Lines(), g.rowString(1))
	}
}

func TestNothingAdvancesAfterTheRunEnds(t *testing.T) {
	g, src := seeded()
	g.board[2][4], g.board[3][4] = pieceJ, pieceJ
	g.put(pieceO, 0, 0, 20)
	g.HandleInput(engine.KeySelect)
	before, draws := snapshot(g), src.draws
	g.Update(time.Minute)
	for _, k := range []engine.Key{engine.KeyLeft, engine.KeyRight, engine.KeyUp, engine.KeyDown, engine.KeyAction, engine.KeyPause} {
		g.HandleInput(k)
	}
	if snapshot(g) != before || src.draws != draws {
		t.Fatal("finished run changed")
	}
}

func TestEnterRestartsAFreshRun(t *testing.T) {
	g, _ := seeded()
	g.lines, g.score = 23, 4321
	g.fill("xxxx.xxxxx")
	g.put(pieceT, 0, 3, 10)
	g.Update(250 * ms)
	g.board[2][4], g.board[3][4] = pieceJ, pieceJ
	g.HandleInput(engine.KeySelect)
	if !g.Finished() {
		t.Fatal("setup: run did not end")
	}
	g.lockNow, g.deepest, g.airborne = true, 21, false
	g.resets, g.lockTime, g.fall = 5, 100*ms, 200*ms
	g.HandleInput(engine.KeySelect)
	if g.Finished() || g.Score() != 0 || g.Lines() != 0 || g.Level() != 1 || g.settled() != 0 ||
		g.fall != 0 || g.lockTime != 0 || g.resets != 0 || g.lockNow || g.deepest != -1 || !g.airborne {
		t.Fatalf("after restart: %s", snapshot(g))
	}
	// A fresh bag: current, preview and the five left form one full bag.
	if g.bagLeft != 5 || g.cur != spawnPiece(g.cur.kind) {
		t.Fatalf("bag not restarted: left %d cur %+v", g.bagLeft, g.cur)
	}
	bag := slices.Clone(g.bag[:])
	slices.Sort(bag)
	if !slices.Equal(bag, []kind{pieceI, pieceO, pieceT, pieceS, pieceZ, pieceJ, pieceL}) ||
		g.bag[0] != g.cur.kind || g.bag[1] != g.next {
		t.Fatalf("bag %v cur %d next %d", g.bag, g.cur.kind, g.next)
	}
	g.Update(699 * ms)
	if g.cur.y != spawnPiece(g.cur.kind).y {
		t.Fatal("restart kept leftover gravity time")
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

func TestRenderFitsAndShowsHUD(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {81, 25}, {120, 40}} {
		g, _ := seeded()
		g.Resize(size[0], size[1])
		g.fill("xxxxx.....")
		out := renderStrict(t, g, size[0], size[1])
		board := boardArea(t, out)
		if strings.Count(board, "▓▓▓") != 5 || strings.Count(board, "███") != 4 || strings.Count(board, "░░░") != 4 {
			t.Errorf("%v: want 5 settled cells, 4 piece cells, 4 projection cells:\n%s", size, board)
		}
		for _, want := range []string{"BLOCK DROP", "Score 00000", "Lines 000", "Level 01", "NEXT",
			"rotate", "hard drop", "Pause: Space", "Leave: Q / Esc", "Exit: Ctrl+C",
			"Active █  Ghost ░  Stack ▓"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: HUD lacks %q", size, want)
			}
		}
		if strings.Count(out, "+------------------------------+") != 2 {
			t.Errorf("%v: board border not complete:\n%s", size, out)
		}
	}
}

func TestRenderHidesCellsInHiddenRows(t *testing.T) {
	g, _ := seeded()
	g.put(pieceI, 1, 3, 0) // column 5, rows 0..3: two cells hidden
	board := boardArea(t, renderStrict(t, g, 80, 24))
	if n := strings.Count(board, "███"); n != 2 {
		t.Fatalf("%d piece glyphs, want 2 visible piece cells:\n%s", n, board)
	}
}

// boardArea returns the 20 visible board rows inside the border.
func boardArea(t *testing.T, screen string) string {
	t.Helper()
	lines := strings.Split(screen, "\n")
	at := position(screen, "+------------------------------+")
	if at[1] < 0 || at[1]+21 >= len(lines) {
		t.Fatalf("no board border:\n%s", screen)
	}
	rows := make([]string, VisibleRows)
	for i := range rows {
		line := []rune(lines[at[1]+1+i])
		rows[i] = string(line[at[0]+1 : at[0]+1+Cols*3])
	}
	return strings.Join(rows, "\n")
}

func TestGroundedPieceIsDistinctFromTheStackWithoutColor(t *testing.T) {
	// An O resting on the floor fills the gap in two otherwise full rows:
	// the projection is hidden under it, so only its glyph sets it apart.
	g, _ := seeded()
	g.fill("xxxx..xxxx", "xxxx..xxxx")
	g.put(pieceO, 0, 4, 20)
	board := strings.Split(boardArea(t, renderStrict(t, g, 80, 24)), "\n")
	want := strings.Repeat("▓▓▓", 4) + strings.Repeat("███", 2) + strings.Repeat("▓▓▓", 4)
	for _, row := range board[VisibleRows-2:] {
		if row != want {
			t.Fatalf("bottom rows render as %q, want the active piece in solid blocks", board[VisibleRows-2:])
		}
	}
	if strings.Contains(strings.Join(board, ""), "░░░") {
		t.Fatal("projection drawn under a grounded piece")
	}
}

func TestRenderEndScreenAtSeveralSizes(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}} {
		g, _ := seeded()
		g.score, g.lines = 1234, 12
		g.board[2][4], g.board[3][4] = pieceJ, pieceJ
		g.put(pieceO, 0, 0, 20)
		g.HandleInput(engine.KeySelect)
		out := renderStrict(t, g, size[0], size[1])
		for _, want := range []string{"GAME OVER", "Final score 1234", "Lines 12   Level 2", "Enter: play again"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: end screen lacks %q:\n%s", size, want, out)
			}
		}
	}
}

func position(screen, s string) [2]int {
	for y, line := range strings.Split(screen, "\n") {
		if x := strings.Index(line, s); x >= 0 {
			return [2]int{x, y}
		}
	}
	return [2]int{-1, -1}
}

func TestEnginePauseAndResizeKeepTheBoard(t *testing.T) {
	g, src := seeded()
	e := engine.New(g)
	e.Resize(80, 24)
	e.Advance(time.Second) // dropped first frame
	e.Advance(100 * ms)
	if g.fall != 100*ms {
		t.Fatalf("fall %v after one frame", g.fall)
	}
	before, draws := snapshot(g), src.draws
	at80 := renderStrict(t, g, 80, 24)

	e.Input(engine.Event{Key: engine.KeyPause})
	e.Input(engine.Event{Key: engine.KeyLeft}) // not delivered while paused
	e.Input(engine.Event{Key: engine.KeySelect})
	e.Resize(40, 10)
	e.Resize(120, 40)
	for range 50 {
		e.Advance(100 * ms)
	}
	if !e.Paused() || snapshot(g) != before || src.draws != draws {
		t.Fatalf("paused game changed:\n%s\n%s", snapshot(g), before)
	}
	e.Input(engine.Event{Key: engine.KeyPause})
	e.Advance(time.Minute) // dropped: spans the pause
	if snapshot(g) != before {
		t.Fatal("pause time replayed")
	}
	at120 := renderStrict(t, g, 120, 40)
	if position(at80, "BLOCK DROP") == position(at120, "BLOCK DROP") {
		t.Fatal("layout not re-centered on the larger screen")
	}
	e.Advance(100 * ms)
	if g.fall != 200*ms {
		t.Fatalf("fall %v after resuming", g.fall)
	}
}

func TestEngineRunClearsALine(t *testing.T) {
	g, _ := seeded()
	e := engine.New(g)
	e.Resize(80, 24) // Start resets the run; set up the board afterwards
	press := func(k engine.Key) { e.Input(engine.Event{Key: k}) }

	// Fill the bottom row except under the first piece's lowest cells.
	first := g.cur
	bottom := 0
	for _, c := range first.cells() {
		bottom = max(bottom, c.y)
	}
	row := []byte("xxxxxxxxxx")
	for _, c := range first.cells() {
		if c.y == bottom {
			row[c.x] = '.'
		}
	}
	g.fill(string(row))
	landY := g.landing().y

	e.Advance(time.Second) // dropped first frame
	press(engine.KeyLeft)
	press(engine.KeyRight)
	press(engine.KeyUp)
	press(engine.KeyAction)
	for range 3 {
		press(engine.KeyDown)
	}
	if g.cur.kind != first.kind || g.cur.rot != 0 || g.cur.x != first.x || g.cur.y != first.y+3 {
		t.Fatalf("inputs left the piece at %+v", g.cur)
	}
	press(engine.KeySelect)
	if g.Lines() != 1 || g.Score() != 3+2*(landY-first.y-3)+100 || g.Finished() {
		t.Fatalf("lines %d score %d", g.Lines(), g.Score())
	}
	if gaps := strings.Count(string(row), "."); g.settled() != 4-gaps {
		t.Fatalf("settled %d after clearing the bottom row, want %d", g.settled(), 4-gaps)
	}

	// Let gravity bring the second piece down and lock it.
	settled := g.settled()
	frames := 0
	for g.settled() == settled && frames < 400 {
		e.Advance(100 * ms)
		frames++
	}
	if g.settled() != settled+4 || g.Finished() || frames < 50 {
		t.Fatalf("second piece: settled %d -> %d after %d frames", settled, g.settled(), frames)
	}
}
