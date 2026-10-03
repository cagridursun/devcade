package blastgrid

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

func seeded() (*Game, *countingSource) {
	src := &countingSource{Source: rand.NewPCG(1, 2)}
	g := newGame(rand.New(src))
	g.Start(80, 24)
	return g, src
}

// openRows is a crate-free arena with the real walls and spawns.
var openRows = [Rows]string{
	"#################",
	"#P.............1#",
	"#.#.#.#.#.#.#.#.#",
	"#...............#",
	"#.#.#.#.#.#.#.#.#",
	"#...............#",
	"#.#.#.#.#.#.#.#.#",
	"#...............#",
	"#.#.#.#.#.#.#.#.#",
	"#...............#",
	"#.#.#.#.#.#.#.#.#",
	"#2.............3#",
	"#################",
}

// fixture loads openRows with marks applied ('x' crate, '.' floor, 'P' or
// '1'-'3' moves that actor) and freezes the bots until a test unfreezes them.
func fixture(marks map[point]rune) *Game {
	g, _ := seeded()
	var rows [Rows][]byte
	for y, r := range openRows {
		rows[y] = []byte(r)
	}
	for _, ch := range marks {
		if ch == 'P' || (ch >= '1' && ch <= '3') {
			for y := range rows {
				if x := strings.IndexRune(string(rows[y]), ch); x >= 0 {
					rows[y][x] = '.'
				}
			}
		}
	}
	for p, ch := range marks {
		rows[p.y][p.x] = byte(ch)
	}
	var out [Rows]string
	for y := range rows {
		out[y] = string(rows[y])
	}
	g.load(out)
	g.nextThink = time.Hour
	return g
}

func unfreeze(g *Game) { g.nextThink = g.now + botInterval }

// addBomb places a bomb directly (placement order = call order).
func addBomb(g *Game, p point, owner int, at time.Duration) *bomb {
	b := &bomb{pos: p, owner: owner, seq: g.nextSeq, explodeAt: at, holder: noActor}
	g.nextSeq++
	g.bombs = append(g.bombs, b)
	return b
}

func burning(g *Game) []point {
	var out []point
	for y := range Rows {
		for x := range Cols {
			if g.now < g.flameUntil[y][x] {
				out = append(out, point{x, y})
			}
		}
	}
	return out
}

func sortPoints(ps []point) []point {
	ps = slices.Clone(ps)
	slices.SortFunc(ps, func(a, b point) int {
		if a.y != b.y {
			return a.y - b.y
		}
		return a.x - b.x
	})
	return ps
}

func TestStartState(t *testing.T) {
	g, _ := seeded()
	want := [numActors]point{{1, 1}, {15, 1}, {1, 11}, {15, 11}}
	for i, a := range g.actors {
		if !a.alive || a.pos != want[i] {
			t.Fatalf("actor %d: %+v", i, a)
		}
	}
	if g.Score() != 0 || g.BotsLeft() != 3 || len(g.bombs) != 0 || g.now != 0 || g.nextThink != botInterval || g.Finished() {
		t.Fatalf("score=%d bots=%d bombs=%d now=%v think=%v", g.Score(), g.BotsLeft(), len(g.bombs), g.now, g.nextThink)
	}
	if w, h := g.MinimumSize(); w != 80 || h != 24 {
		t.Fatalf("minimum size %dx%d", w, h)
	}
}

func TestArenaIsValid(t *testing.T) {
	g, _ := seeded()
	spawns := 0
	for y, row := range layout {
		if len(row) != Cols {
			t.Fatalf("row %d has %d columns", y, len(row))
		}
		for x, ch := range row {
			border := x == 0 || y == 0 || x == Cols-1 || y == Rows-1
			pillar := x%2 == 0 && y%2 == 0
			if (border || pillar) != (ch == '#') {
				t.Fatalf("cell %d,%d is %q: walls must be exactly the border and even/even pillars", x, y, ch)
			}
			if strings.ContainsRune("P123", ch) {
				spawns++
			}
		}
	}
	if spawns != numActors {
		t.Fatalf("%d spawns", spawns)
	}

	// No sealed pockets: every non-wall cell is connected, so breaking
	// crates can always reach every bot.
	reach := flood(g, g.actors[player].pos, func(p point) bool { return g.grid[p.y][p.x] != wall })
	for y := range Rows {
		for x := range Cols {
			if g.grid[y][x] != wall && !reach[y][x] {
				t.Fatalf("cell %d,%d unreachable", x, y)
			}
		}
	}

	dm0 := g.danger(nil)
	for i, a := range g.actors {
		// Spawns are separate: their crate-free floor areas do not touch and
		// no spawn is in another spawn's blast.
		area := flood(g, a.pos, func(p point) bool { return g.grid[p.y][p.x] == floor })
		cells, _ := g.trace(a.pos, func(point) bool { return false })
		for j, b := range g.actors {
			if j != i && (area[b.pos.y][b.pos.x] || slices.Contains(cells, b.pos)) {
				t.Fatalf("spawns %d and %d are not separate", i, j)
			}
		}
		// Each spawn can start breaking crates.
		if _, crates := g.blastHits(i, a.pos); !crates {
			t.Fatalf("spawn %d breaks no crate", i)
		}
		// A bomb dropped on the spawn at once has an escape within three
		// moves, even at the bots' slower 180 ms pace and before any other
		// actor moves.
		nb := &bomb{pos: a.pos, owner: i, explodeAt: fuseTime}
		dm := g.danger(nb)
		first, ok := g.plan(i, nb, dm, append(dirs[:], stay), 3, func(p point, at time.Duration) bool { return !dm.threatened(p, at) })
		if !ok || first == a.pos {
			t.Fatalf("spawn %d: no escape from a bomb on the spawn", i)
		}
		if dm0.threatened(a.pos, 0) {
			t.Fatalf("spawn %d starts in danger", i)
		}
	}
}

func flood(g *Game, from point, ok func(point) bool) (seen [Rows][Cols]bool) {
	queue := []point{from}
	seen[from.y][from.x] = true
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range dirs {
			q := p.add(d)
			if inside(q) && !seen[q.y][q.x] && ok(q) {
				seen[q.y][q.x] = true
				queue = append(queue, q)
			}
		}
	}
	return seen
}

func TestPlacementAndCapacity(t *testing.T) {
	g := fixture(map[point]rune{{3, 3}: 'P'})
	g.HandleInput(engine.KeyAction)
	if len(g.bombs) != 1 {
		t.Fatalf("bombs %d", len(g.bombs))
	}
	b := g.bombs[0]
	if b.pos != (point{3, 3}) || b.owner != player || b.holder != player || b.explodeAt != fuseTime {
		t.Fatalf("bomb %+v", *b)
	}
	g.HandleInput(engine.KeyAction) // same cell, capacity used
	g.HandleInput(engine.KeyRight)
	g.Update(moveCooldown)
	g.HandleInput(engine.KeyRight)
	g.Update(moveCooldown)
	g.HandleInput(engine.KeyRight)
	g.Update(moveCooldown)
	g.HandleInput(engine.KeyRight)
	g.HandleInput(engine.KeyAction) // second bomb refused: one active bomb per actor
	if len(g.bombs) != 1 || g.actors[player].pos != (point{7, 3}) {
		t.Fatalf("bombs %d player %v", len(g.bombs), g.actors[player].pos)
	}
	g.Update(fuseTime) // explodes; the right ray ends at 6,3, next to the player
	if len(g.bombs) != 0 || g.Finished() {
		t.Fatalf("bombs %d finished %v", len(g.bombs), g.Finished())
	}
	g.HandleInput(engine.KeyAction) // capacity restored
	if len(g.bombs) != 1 || g.bombs[0].pos != (point{7, 3}) || g.bombs[0].explodeAt != g.now+fuseTime {
		t.Fatalf("after capacity restored: %d bombs", len(g.bombs))
	}

	// A dead actor and an occupied bomb cell cannot take a bomb.
	g.actors[1].alive = false
	if g.placeBomb(1) {
		t.Fatal("dead bot placed a bomb")
	}
	g.actors[2].pos = g.actors[player].pos // fixture only: share the bomb cell
	if g.placeBomb(2) {
		t.Fatal("placed a second bomb on one cell")
	}
}

func TestMovementCooldown(t *testing.T) {
	g := fixture(map[point]rune{{3, 3}: 'P'})
	pos := func() point { return g.actors[player].pos }
	g.HandleInput(engine.KeyUp) // 3,2 is floor
	g.HandleInput(engine.KeyUp) // during cooldown: ignored, not queued
	if pos() != (point{3, 2}) {
		t.Fatalf("pos %v", pos())
	}
	g.Update(119 * ms)
	g.HandleInput(engine.KeyDown)
	if pos() != (point{3, 2}) {
		t.Fatal("moved during the cooldown")
	}
	g.Update(ms)
	if pos() != (point{3, 2}) {
		t.Fatal("ignored press was queued")
	}
	g.HandleInput(engine.KeyLeft) // 2,2 is a pillar: blocked, no cooldown
	g.HandleInput(engine.KeyDown)
	if pos() != (point{3, 3}) {
		t.Fatalf("pos %v: blocked press should not start the cooldown", pos())
	}
	g.HandleInput(engine.KeySelect) // no effect while playing
	if pos() != (point{3, 3}) || g.Finished() {
		t.Fatal("select changed the run")
	}
}

func TestBlockedCells(t *testing.T) {
	g := fixture(map[point]rune{{3, 3}: 'P', {4, 3}: 'x', {3, 4}: '1', {2, 3}: '2'})
	addBomb(g, point{3, 2}, 3, time.Hour)
	for _, k := range []engine.Key{engine.KeyRight, engine.KeyDown, engine.KeyLeft, engine.KeyUp} {
		g.HandleInput(k)
		if g.actors[player].pos != (point{3, 3}) {
			t.Fatalf("%v: entered a crate, actor or bomb", k)
		}
	}
	g.actors[1].alive = false
	g.HandleInput(engine.KeyDown)
	if g.actors[player].pos != (point{3, 4}) {
		t.Fatal("a dead bot still blocks")
	}
	g2 := fixture(map[point]rune{{1, 1}: 'P'})
	g2.HandleInput(engine.KeyUp)
	g2.HandleInput(engine.KeyLeft)
	if g2.actors[player].pos != (point{1, 1}) {
		t.Fatal("entered a wall")
	}
}

func TestLeaveOnceBombAccess(t *testing.T) {
	g := fixture(map[point]rune{{3, 3}: 'P'})
	g.HandleInput(engine.KeyAction)
	b := g.bombs[0]
	g.Update(time.Second) // staying on the own bomb is fine
	if b.holder != player || g.actors[player].pos != (point{3, 3}) {
		t.Fatal("placer could not stay on its bomb")
	}
	g.HandleInput(engine.KeyRight)
	if g.actors[player].pos != (point{4, 3}) || b.holder != noActor {
		t.Fatalf("leave: pos %v holder %d", g.actors[player].pos, b.holder)
	}
	g.Update(moveCooldown)
	g.HandleInput(engine.KeyLeft)
	if g.actors[player].pos != (point{4, 3}) {
		t.Fatal("re-entered the bomb")
	}
	// The bot planner applies the same rule to a hypothetical bomb.
	g2 := fixture(map[point]rune{{3, 3}: '1'})
	nb := &bomb{pos: point{3, 3}, owner: 1, explodeAt: time.Hour}
	if g2.walkable(point{3, 3}, 2, nb) || !g2.walkable(point{4, 3}, 1, nb) {
		t.Fatal("walkable ignores the hypothetical bomb")
	}
}

func TestRayStopsAtWallsAndFirstCrate(t *testing.T) {
	g := fixture(map[point]rune{{7, 3}: 'x', {8, 3}: 'x'})
	addBomb(g, point{5, 3}, player, 0) // due immediately
	addBomb(g, point{4, 5}, 1, time.Hour)
	g.Update(ms)
	want := []point{{5, 1}, {5, 2}, {2, 3}, {3, 3}, {4, 3}, {5, 3}, {6, 3}, {7, 3}, {5, 4}, {5, 5}, {5, 6}}
	if got := burning(g); fmt.Sprint(got) != fmt.Sprint(sortPoints(want)) {
		t.Fatalf("flames %v\nwant   %v", got, sortPoints(want))
	}
	if g.grid[3][7] != floor || g.grid[3][8] != crate || g.Score() != crateScore {
		t.Fatalf("crates 7,3=%v 8,3=%v score %d", g.grid[3][7], g.grid[3][8], g.Score())
	}
	if len(g.bombs) != 1 || g.bombs[0].pos != (point{4, 5}) {
		t.Fatal("4,5 is not on a ray (4,4 is a pillar) yet it exploded")
	}
}

func TestChainResolvesAtOneTimestamp(t *testing.T) {
	g := fixture(map[point]rune{{8, 3}: 'x', {3, 6}: 'x', {3, 5}: '2', {7, 3}: '3'})
	addBomb(g, point{3, 3}, player, 2*time.Second)
	addBomb(g, point{5, 3}, 1, 3*time.Second) // triggered by the player's bomb
	g.Update(2 * time.Second)
	if len(g.bombs) != 0 || g.hasBomb(1) {
		t.Fatalf("bombs left: %d", len(g.bombs))
	}
	if g.flameUntil[3][6] != 2*time.Second+flameTime || g.flameOwner[3][6] != 1 {
		t.Fatal("chained bomb did not explode at the trigger time")
	}
	if g.flameUntil[3][4] == 0 || g.flameOwner[3][4] != player {
		t.Fatal("4,3 is covered by both; the earlier-placed player bomb is credited")
	}
	// The player's bomb kills bot 2 (+100) and breaks 3,6 (+10). The bot
	// bomb kills bot 3 and breaks 8,3: no score.
	if g.actors[2].alive || g.actors[3].alive || g.Score() != botScore+crateScore || g.grid[3][8] != floor || g.grid[6][3] != floor {
		t.Fatalf("bots %v %v score %d", g.actors[2].alive, g.actors[3].alive, g.Score())
	}
	if !g.placeBomb(1) {
		t.Fatal("bot capacity not restored")
	}
	g.bombs = nil
	g.Update(time.Second + 100*ms) // past the chained bomb's own fuse
	if len(burning(g)) != 0 || g.Score() != botScore+crateScore {
		t.Fatal("chained bomb exploded twice")
	}
}

func TestMutualChainExplodesEachBombOnce(t *testing.T) {
	g := fixture(map[point]rune{{5, 6}: 'x'})
	addBomb(g, point{3, 3}, player, time.Second)
	addBomb(g, point{5, 3}, 1, time.Second) // A and B see each other
	addBomb(g, point{5, 5}, 2, time.Hour)   // reached by B
	g.Update(time.Second)
	if len(g.bombs) != 0 || g.hasBomb(player) || g.hasBomb(1) || g.hasBomb(2) {
		t.Fatalf("bombs left %d", len(g.bombs))
	}
	if g.grid[6][5] != floor {
		t.Fatal("the bomb reached only through the chain did not explode")
	}
}

// TestRaysUseStartOfTimestampOccupancy: two simultaneous blasts both reach
// crate 5,1. The crate stops both rays even though the first bomb in queue
// order destroys it, so bot 2 behind it survives.
func TestRaysUseStartOfTimestampOccupancy(t *testing.T) {
	g := fixture(map[point]rune{{5, 1}: 'x', {4, 1}: '2'})
	addBomb(g, point{5, 3}, player, time.Second)
	addBomb(g, point{7, 1}, 1, time.Second)
	g.Update(time.Second)
	if !g.actors[2].alive || g.flameUntil[1][4] != 0 {
		t.Fatal("ray traveled through a crate destroyed at the same timestamp")
	}
	if g.grid[1][5] != floor || g.Score() != crateScore {
		t.Fatalf("crate %v score %d: crate should break once, credited to the player", g.grid[1][5], g.Score())
	}
}

func TestOverlappingBlastAttribution(t *testing.T) {
	for _, playerFirst := range []bool{true, false} {
		// Crate 5,3 is the last cell of both rays.
		g := fixture(map[point]rune{{5, 3}: 'x'})
		place := func(owner int, p point) { addBomb(g, p, owner, time.Second) }
		if playerFirst {
			place(player, point{3, 3})
			place(1, point{7, 3})
		} else {
			place(1, point{7, 3})
			place(player, point{3, 3})
		}
		g.Update(time.Second)
		want := 0
		if playerFirst {
			want = crateScore
		}
		if g.grid[3][5] != floor || g.Score() != want {
			t.Errorf("crate, player first=%v: score %d want %d", playerFirst, g.Score(), want)
		}

		// Bot 3 at 5,3 is covered by both vertical rays.
		g = fixture(map[point]rune{{5, 3}: '3'})
		if playerFirst {
			place(player, point{5, 1})
			place(1, point{5, 5})
		} else {
			place(1, point{5, 5})
			place(player, point{5, 1})
		}
		g.Update(time.Second)
		want = 0
		if playerFirst {
			want = botScore
		}
		if g.actors[3].alive || g.Score() != want {
			t.Errorf("bot, player first=%v: score %d want %d", playerFirst, g.Score(), want)
		}
	}
}

func TestFuseAndFlameBoundaries(t *testing.T) {
	for _, chunk := range []time.Duration{ms, 7 * ms, 100 * ms} {
		g := fixture(nil)
		g.actors[player].pos = point{9, 5}
		g.HandleInput(engine.KeyAction)
		g.actors[player].pos = point{1, 1} // fixture: step out of range
		advance := func(d time.Duration) {
			for ; d > 0; d -= min(d, chunk) {
				g.Update(min(d, chunk))
			}
		}
		advance(fuseTime - ms)
		if len(g.bombs) != 1 || len(burning(g)) != 0 {
			t.Fatalf("chunk %v: exploded early", chunk)
		}
		advance(ms)
		if len(g.bombs) != 0 || g.now >= g.flameUntil[5][9] {
			t.Fatalf("chunk %v: not exploded at exactly 2 s", chunk)
		}
		advance(flameTime - ms)
		if g.now >= g.flameUntil[5][9] {
			t.Fatalf("chunk %v: flame gone early", chunk)
		}
		advance(ms)
		if len(burning(g)) != 0 {
			t.Fatalf("chunk %v: flame outlived 500 ms", chunk)
		}
	}
}

func TestEnteringActiveFlame(t *testing.T) {
	g := fixture(map[point]rune{{3, 3}: 'P'})
	addBomb(g, point{7, 3}, 2, time.Second)
	g.Update(time.Second) // the left ray burns 6,3..4,3 until 1.5 s
	g.Update(flameTime - ms)
	g.HandleInput(engine.KeyRight) // into 4,3 one ms before it clears
	if g.actors[player].alive || g.state != lost {
		t.Fatal("walked into an active flame and survived")
	}

	// At exactly the expiry time the cell is safe.
	g = fixture(map[point]rune{{3, 3}: 'P'})
	addBomb(g, point{7, 3}, 2, time.Second)
	g.Update(time.Second + flameTime)
	g.HandleInput(engine.KeyRight)
	if !g.actors[player].alive || g.actors[player].pos != (point{4, 3}) {
		t.Fatal("flame still active at its expiry time")
	}

	// A bot walking into the player's flame dies and is credited.
	g = fixture(map[point]rune{{5, 1}: '1'})
	addBomb(g, point{3, 3}, player, time.Second)
	g.Update(time.Second) // 3,1 burns (up ray), 4,1 does not
	g.move(1, left)
	g.move(1, left)
	if g.actors[1].alive || g.Score() != botScore {
		t.Fatalf("bot entering player flame: alive=%v score=%d", g.actors[1].alive, g.Score())
	}
}

func TestDangerPredictsChains(t *testing.T) {
	g := fixture(nil)
	addBomb(g, point{3, 3}, 1, time.Second)
	addBomb(g, point{5, 3}, 2, 2*time.Second)
	dm := g.danger(nil)
	if w := dm[3][7]; len(w) != 1 || w[0] != (window{time.Second, time.Second + flameTime}) {
		t.Fatalf("7,3 windows %v: the chained bomb fires at 1 s", w)
	}
	if !dm.unsafe(point{7, 3}, 800*ms, time.Second) || dm.unsafe(point{7, 3}, 1500*ms, 1700*ms) || dm.unsafe(point{9, 9}, 0, time.Hour) {
		t.Fatal("unsafe boundaries")
	}
}

// soloBot keeps only bot 1 alive and running.
func soloBot(marks map[point]rune) *Game {
	g := fixture(marks)
	g.actors[2].alive, g.actors[3].alive = false, false
	unfreeze(g)
	return g
}

func TestBotBombsCrateAndEscapes(t *testing.T) {
	g := soloBot(map[point]rune{{9, 5}: '1', {9, 6}: 'x'})
	g.Update(botInterval)
	if len(g.bombs) != 1 || g.bombs[0].pos != (point{9, 5}) || g.actors[1].pos == (point{9, 5}) {
		t.Fatalf("bot should bomb the crate and step off at once: bombs %d pos %v", len(g.bombs), g.actors[1].pos)
	}
	for range 30 {
		g.Update(100 * ms)
		if p := g.actors[1].pos; p == (point{9, 5}) && g.bombAt(p) != nil {
			t.Fatal("bot re-entered its bomb")
		}
	}
	if !g.actors[1].alive || g.grid[6][9] != floor {
		t.Fatalf("bot alive=%v crate=%v", g.actors[1].alive, g.grid[6][9])
	}
}

func TestBotFleesPendingBlast(t *testing.T) {
	g := soloBot(map[point]rune{{9, 5}: '1'})
	addBomb(g, point{9, 3}, player, time.Second)
	g.Update(botInterval)
	if g.actors[1].pos == (point{9, 5}) {
		t.Fatal("bot stayed on a future blast cell")
	}
	g.Update(time.Second)
	if !g.actors[1].alive {
		t.Fatal("bot died to a blast it could escape")
	}
}

func TestBotRejectsBombWithoutEscape(t *testing.T) {
	pocket := map[point]rune{{9, 9}: 'P', {1, 1}: '1', {3, 1}: 'x', {1, 2}: 'x'}
	g := soloBot(pocket)
	g.Update(3 * time.Second)
	if g.nextSeq != 0 || !g.actors[1].alive {
		t.Fatalf("bot bombed a pocket with no escape: bombs %d alive %v", g.nextSeq, g.actors[1].alive)
	}

	// Control: the same corner with an off-axis exit at 3,2.
	g = soloBot(map[point]rune{{9, 9}: 'P', {1, 1}: '1', {4, 1}: 'x', {1, 2}: 'x', {3, 3}: 'x'})
	g.Update(3 * time.Second)
	if g.nextSeq == 0 || !g.actors[1].alive || g.grid[1][4] != floor {
		t.Fatalf("bot should bomb and escape: bombs %d alive %v", g.nextSeq, g.actors[1].alive)
	}
}

func TestBotWaitsOutActiveFlame(t *testing.T) {
	// The only way on from 1,1 is through 2,1, which burns for a while.
	g := soloBot(map[point]rune{{9, 9}: 'P', {1, 1}: '1', {1, 2}: 'x'})
	g.flameUntil[1][2] = 500 * ms
	g.flameOwner[1][2] = player
	g.Update(400 * ms)
	if !g.actors[1].alive || g.actors[1].pos != (point{1, 1}) {
		t.Fatal("bot walked into an active flame")
	}
}

func TestWinAndLossPrecedence(t *testing.T) {
	t.Run("win", func(t *testing.T) {
		g := fixture(map[point]rune{{5, 3}: '1'})
		g.actors[2].alive, g.actors[3].alive = false, false
		addBomb(g, point{3, 3}, player, time.Second)
		g.Update(time.Second)
		if g.state != won || g.Score() != botScore || g.BotsLeft() != 0 {
			t.Fatalf("state %v score %d", g.state, g.Score())
		}
		out := renderStrict(t, g, 80, 24)
		for _, want := range []string{"YOU WIN", "Final score 100", "Enter: play again"} {
			if !strings.Contains(out, want) {
				t.Fatalf("win screen lacks %q:\n%s", want, out)
			}
		}
	})
	t.Run("simultaneous last bot and player death loses", func(t *testing.T) {
		g := fixture(map[point]rune{{5, 3}: '1', {1, 3}: 'P'})
		g.actors[2].alive, g.actors[3].alive = false, false
		addBomb(g, point{3, 3}, player, time.Second)
		g.Update(time.Second)
		if g.state != lost || g.BotsLeft() != 0 {
			t.Fatalf("state %v bots %d", g.state, g.BotsLeft())
		}
		out := renderStrict(t, g, 80, 24)
		if !strings.Contains(out, "GAME OVER") || !strings.Contains(out, "Enter: play again") {
			t.Fatalf("loss screen:\n%s", out)
		}
	})
	t.Run("bot bomb kills player", func(t *testing.T) {
		g := fixture(map[point]rune{{1, 3}: 'P'})
		addBomb(g, point{3, 3}, 1, time.Second)
		g.Update(time.Second)
		if g.state != lost || g.BotsLeft() != 3 || g.Score() != 0 {
			t.Fatalf("state %v", g.state)
		}
	})
}

func TestFinishedRunIgnoresEverythingButRestart(t *testing.T) {
	g := fixture(map[point]rune{{1, 3}: 'P'})
	addBomb(g, point{3, 3}, 1, time.Second)
	g.Update(time.Second)
	now := g.now
	g.Update(time.Minute)
	for _, k := range []engine.Key{engine.KeyUp, engine.KeyRight, engine.KeyAction, engine.KeyPause} {
		g.HandleInput(k)
	}
	if g.now != now || len(g.bombs) != 0 || !g.Finished() {
		t.Fatal("finished run changed")
	}
}

func TestEnterRestartsAFreshRun(t *testing.T) {
	g, src := seeded()
	fresh, _ := seeded()
	oldRNG := g.rng
	for range 50 {
		g.Update(100 * ms)
	}
	g.HandleInput(engine.KeyAction)
	g.actors[player].alive = false // fixture: end the run
	g.score = 1234
	g.checkEnd()
	if !g.Finished() {
		t.Fatal("setup")
	}
	draws := src.draws
	g.HandleInput(engine.KeySelect)
	if g.Finished() || g.grid != fresh.grid || g.actors != fresh.actors || len(g.bombs) != 0 || g.nextSeq != 0 ||
		g.flameUntil != fresh.flameUntil || g.now != 0 || g.nextMove != 0 || g.nextThink != botInterval || g.Score() != 0 {
		t.Fatalf("restart did not reset: now %v score %d bombs %d", g.now, g.Score(), len(g.bombs))
	}
	if g.rng == oldRNG || src.draws == draws {
		t.Fatal("restart did not rebuild the bot RNG from the injected source")
	}
}

// snapshot is a comparable dump of all gameplay state.
func snapshot(g *Game) string {
	var b strings.Builder
	fmt.Fprint(&b, g.grid, g.actors, g.flameUntil, g.flameOwner, g.now, g.nextMove, g.nextThink, g.score, g.state, g.nextSeq)
	for _, bm := range g.bombs {
		fmt.Fprint(&b, *bm)
	}
	return b.String()
}

// script plays the same player inputs at the same gameplay times.
var script = map[int][]engine.Key{
	3: {engine.KeyRight}, 5: {engine.KeyRight}, 6: {engine.KeyAction, engine.KeyLeft},
	8: {engine.KeyLeft}, 9: {engine.KeyDown}, 30: {engine.KeyDown}, 32: {engine.KeyAction},
	33: {engine.KeyUp}, 34: {engine.KeyRight}, 60: {engine.KeyRight}, 61: {engine.KeyRight},
}

func TestFrameChunkingInvariance(t *testing.T) {
	run := func(chunks []time.Duration) string {
		g, _ := seeded()
		for tick := range 400 { // 40 s, inputs on 100 ms boundaries
			for _, k := range script[tick] {
				g.HandleInput(k)
			}
			for _, c := range chunks {
				g.Update(c)
			}
		}
		return snapshot(g)
	}
	want := run([]time.Duration{100 * ms})
	for _, chunks := range [][]time.Duration{{50 * ms, 50 * ms}, {13 * ms, 29 * ms, 58 * ms}, {ms, 99 * ms}} {
		if got := run(chunks); got != want {
			t.Fatalf("chunks %v changed the outcome", chunks)
		}
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

func TestRenderFitsAndShowsTheArena(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {81, 25}, {120, 40}} {
		g, _ := seeded()
		g.Resize(size[0], size[1])
		out := renderStrict(t, g, size[0], size[1])
		if strings.Count(out, "@@") != 2 { // the player and the legend
			t.Errorf("%v: want one player glyph plus legend:\n%s", size, out)
		}
		for _, want := range []string{"BLAST GRID", "Score 0", "Bots left 3", "Bomb ready", "B1", "B2", "B3",
			"Bomb: Z", "Pause: Space", "Leave: Q / Esc", "Exit: Ctrl+C", "[]", strings.Repeat("##", Cols)} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: screen lacks %q:\n%s", size, want, out)
			}
		}
	}
	g := fixture(map[point]rune{{3, 3}: 'P'})
	g.HandleInput(engine.KeyAction)
	addBomb(g, point{9, 9}, 1, 0)
	g.Update(ms)
	out := renderStrict(t, g, 80, 24)
	for _, want := range []string{"(@", "**", "Bomb armed"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q:\n%s", want, out)
		}
	}
}

func TestEngineResizeAndPause(t *testing.T) {
	g, _ := seeded()
	e := engine.New(g)
	e.Resize(80, 24)
	e.Advance(time.Second) // dropped first frame
	e.Advance(100 * ms)
	e.Advance(100 * ms) // bots decide at 180 ms
	before := snapshot(g)
	at80 := renderStrict(t, g, 80, 24)

	e.Resize(30, 8) // undersized: suspended
	e.Advance(time.Minute)
	e.Input(engine.Event{Key: engine.KeyRight}) // invisible: not delivered
	e.Input(engine.Event{Key: engine.KeyAction, Char: 'z'})
	e.Resize(120, 40)
	e.Advance(time.Minute) // dropped: spans the undersized period
	if snapshot(g) != before {
		t.Fatal("resize changed the game")
	}
	if strings.Index(renderStrict(t, g, 120, 40), "@@") == strings.Index(at80, "@@") {
		t.Fatal("arena not re-centered on the larger screen")
	}

	e.Input(engine.Event{Key: engine.KeyPause})
	e.Input(engine.Event{Key: engine.KeyAction})
	e.Resize(80, 24)
	for range 50 {
		e.Advance(100 * ms)
	}
	if !e.Paused() || snapshot(g) != before {
		t.Fatal("paused game advanced")
	}
	e.Input(engine.Event{Key: engine.KeyPause})
	e.Advance(time.Minute) // dropped
	if snapshot(g) != before {
		t.Fatal("pause time replayed")
	}
	e.Input(engine.Event{Key: engine.KeyAction, Char: 'z'})
	if len(g.bombs) == 0 || !g.hasBomb(player) {
		t.Fatal("Z did not place a bomb after resuming")
	}
}

func TestEngineEndScreenAndRestart(t *testing.T) {
	g, _ := seeded()
	e := engine.New(g)
	e.Resize(80, 24)
	e.Input(engine.Event{Key: engine.KeyAction}) // bomb on the spawn, then stay
	e.Advance(time.Second)                       // dropped first frame
	for range 25 {
		e.Advance(100 * ms)
	}
	if !g.Finished() || g.state != lost {
		t.Fatalf("player standing on its own bomb should lose, state %v", g.state)
	}
	e.Input(engine.Event{Key: engine.KeyPause})
	if e.Paused() {
		t.Fatal("end screen was paused")
	}
	e.Input(engine.Event{Key: engine.KeySelect})
	if g.Finished() || g.now != 0 || len(g.bombs) != 0 || e.Paused() {
		t.Fatal("restart failed")
	}
}

// TestEngineDrivenRun plays 60 s of seeded games through the engine and
// checks invariants every frame and that the run is reproducible.
func TestEngineDrivenRun(t *testing.T) {
	run := func(seed uint64) string {
		g := newGame(rand.New(rand.NewPCG(seed, 7)))
		e := engine.New(g)
		e.Resize(80, 24)
		e.Advance(time.Second) // dropped first frame
		for tick := range 600 {
			for _, k := range script[tick%100] {
				e.Input(engine.Event{Key: k})
			}
			e.Advance(engine.FrameInterval)
			checkInvariants(t, g)
			if g.Finished() {
				e.Input(engine.Event{Key: engine.KeySelect})
			}
		}
		renderStrict(t, g, 80, 24)
		return snapshot(g)
	}
	for seed := range uint64(5) {
		if a, b := run(seed), run(seed); a != b {
			t.Fatalf("seed %d is not reproducible", seed)
		}
	}
}

func checkInvariants(t *testing.T, g *Game) {
	t.Helper()
	owners := map[int]bool{}
	for _, b := range g.bombs {
		if owners[b.owner] || g.grid[b.pos.y][b.pos.x] != floor {
			t.Fatalf("bad bomb %+v", *b)
		}
		owners[b.owner] = true
	}
	cells := map[point]bool{}
	for i, a := range g.actors {
		if !a.alive {
			continue
		}
		if g.grid[a.pos.y][a.pos.x] != floor || cells[a.pos] {
			t.Fatalf("actor %d at %v", i, a.pos)
		}
		if g.now < g.flameUntil[a.pos.y][a.pos.x] {
			t.Fatalf("living actor %d inside a flame", i)
		}
		if b := g.bombAt(a.pos); b != nil && b.holder != i {
			t.Fatalf("actor %d on a bomb it does not hold", i)
		}
		cells[a.pos] = true
	}
}
