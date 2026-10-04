package mazechase

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

const ms = time.Millisecond

func started() *Game {
	g := newGame()
	g.Start(80, 24)
	return g
}

// isolate parks every chaser except keep far in the future, so a test can
// control exactly which chasers act.
func isolate(g *Game, keep ...int) {
	for i := range g.chasers {
		kept := false
		for _, k := range keep {
			kept = kept || k == i
		}
		if !kept {
			g.chasers[i] = chaser{pos: maze.chasers[i], removed: true, respawnAt: time.Hour}
		}
	}
}

// onlyItem empties the maze except one ordinary pellet at p.
func onlyItem(g *Game, p point) {
	g.items = [Rows][Cols]item{}
	g.items[p.y][p.x] = pellet
	g.remaining = 1
}

func advanceTo(g *Game, t time.Duration) {
	g.Update(t - g.now)
}

func TestMapIsValid(t *testing.T) {
	m, err := parseMaze(layout[:])
	if err != nil {
		t.Fatal(err)
	}
	if Cols > 29 || Rows > 19 || Cols*2 > 80 {
		t.Fatalf("map %dx%d too large", Cols, Rows)
	}
	if m.powers != 4 || m.pellets < 100 {
		t.Fatalf("powers %d pellets %d", m.powers, m.pellets)
	}
	dist := m.distances(m.player)
	for y := range Rows {
		for x := range Cols {
			if m.items[y][x] != none && (m.wall[y][x] || dist[y][x] < 0) {
				t.Errorf("collectible at %d,%d in a wall or unreachable", x, y)
			}
		}
	}
	spawns := map[point]bool{m.player: true}
	for i, s := range m.chasers {
		if spawns[s] || !m.open(s) || m.items[s.y][s.x] != none || dist[s.y][s.x] < minSpawnDistance {
			t.Errorf("chaser %d spawn %v unsafe", i+1, s)
		}
		spawns[s] = true
	}
	if !m.open(m.player) || m.items[m.player.y][m.player.x] != none {
		t.Errorf("player spawn %v unsafe", m.player)
	}
	// No dead ends: every open cell has at least two open neighbors.
	for y := range Rows {
		for x := range Cols {
			n := 0
			for _, d := range dirOrder {
				if m.open(point{x, y}.add(d)) {
					n++
				}
			}
			if m.open(point{x, y}) && n < 2 {
				t.Errorf("dead end at %d,%d", x, y)
			}
		}
	}
}

func TestBrokenMapsAreRejected(t *testing.T) {
	type edit struct {
		x, y int
		tile byte
	}
	with := func(edits ...edit) []string {
		rows := append([]string(nil), layout[:]...)
		for _, e := range edits {
			b := []byte(rows[e.y])
			b[e.x] = e.tile
			rows[e.y] = string(b)
		}
		return rows
	}
	ragged := with()
	ragged[3] = ragged[3][:Cols-1]
	for name, rows := range map[string][]string{
		"short map":        layout[:Rows-1],
		"ragged row":       ragged,
		"illegal tile":     with(edit{5, 3, 'x'}),
		"open border":      with(edit{0, 3, '.'}),
		"no player":        with(edit{14, 11, '.'}),
		"two players":      with(edit{5, 3, 'P'}),
		"duplicate chaser": with(edit{13, 8, '1'}),
		"missing chaser":   with(edit{16, 8, ' '}),
		"spawn too close":  with(edit{12, 8, ' '}, edit{13, 11, '1'}),
		"three powers":     with(edit{1, 1, '.'}),
		"five powers":      with(edit{5, 3, 'o'}),
		// (13,1) keeps its pellet but loses both open neighbors.
		"unreachable pellet": with(edit{12, 1, '#'}, edit{13, 2, '#'}),
		"target in a wall":   with(edit{27, 17, '#'}, edit{5, 3, 'o'}),
	} {
		if _, err := parseMaze(rows); err == nil {
			t.Errorf("%s: accepted", name)
		} else {
			t.Logf("%s: %v", name, err)
		}
	}
}

func TestStartState(t *testing.T) {
	g := started()
	if g.player != maze.player || !g.dir.isZero() || !g.want.isZero() || g.lives != 3 || g.score != 0 ||
		g.remaining != maze.pellets+maze.powers || g.Finished() || g.now != 0 || g.vulnerable() {
		t.Fatalf("bad start state: %+v", g)
	}
	for i, c := range g.chasers {
		if c.pos != maze.chasers[i] || c.removed || c.next != chaserStep {
			t.Fatalf("chaser %d: %+v", i, c)
		}
	}
	if g.playerNext != playerStep {
		t.Fatal("player step not scheduled at 140 ms")
	}
}

func TestPlayerStepTimingAndPellets(t *testing.T) {
	g := started()
	isolate(g)
	g.HandleInput(engine.KeyLeft)
	g.Update(139 * ms)
	if g.player != maze.player {
		t.Fatal("moved before 140 ms")
	}
	g.Update(ms)
	if g.player != (point{13, 11}) || g.score != 10 || g.remaining != maze.pellets+maze.powers-1 || g.items[11][13] != none {
		t.Fatalf("player %v score %d", g.player, g.score)
	}
	g.Update(140 * ms) // keeps moving in the current direction
	if g.player != (point{12, 11}) || g.score != 20 {
		t.Fatalf("player %v score %d", g.player, g.score)
	}
}

func TestBufferedTurnAppliesWhenLegal(t *testing.T) {
	g := started()
	isolate(g)
	g.player, g.dir = point{3, 3}, right
	g.HandleInput(engine.KeyUp) // (3,2) and (4,2) are walls; (5,2) is open
	var path []point
	for range 5 {
		g.Update(playerStep)
		path = append(path, g.player)
	}
	want := []point{{4, 3}, {5, 3}, {5, 2}, {5, 1}, {5, 1}}
	if !reflect.DeepEqual(path, want) {
		t.Fatalf("path %v, want %v", path, want)
	}
	if !g.want.isZero() || g.dir != up || g.lives != 3 {
		t.Fatalf("want %v dir %v lives %d", g.want, g.dir, g.lives)
	}

	// A newer press replaces the buffered one; reversal applies at once.
	g.player, g.dir = point{8, 3}, right
	g.HandleInput(engine.KeyUp)
	g.HandleInput(engine.KeyLeft)
	g.Update(playerStep)
	if g.player != (point{7, 3}) || g.dir != left {
		t.Fatalf("player %v dir %v", g.player, g.dir)
	}
}

func TestWallsStopThePlayer(t *testing.T) {
	g := started()
	isolate(g)
	g.player, g.dir = point{13, 1}, right // (14,1) is a wall
	g.HandleInput(engine.KeyUp)           // (13,0) is a wall too
	g.Update(time.Second)
	if g.player != (point{13, 1}) || g.dir != right || g.want != up || g.lives != 3 || g.Finished() {
		t.Fatalf("player %v dir %v want %v lives %d", g.player, g.dir, g.want, g.lives)
	}
	g.HandleInput(engine.KeyDown)
	g.Update(playerStep)
	if g.player != (point{13, 2}) {
		t.Fatalf("player %v did not turn down", g.player)
	}
}

func TestIgnoredKeys(t *testing.T) {
	g := started()
	before := *g
	g.HandleInput(engine.KeyAction)
	g.HandleInput(engine.KeySelect)
	g.HandleInput(engine.KeyPause)
	if !reflect.DeepEqual(*g, before) {
		t.Fatal("non-direction key changed a running game")
	}
}

func TestChaserTargets(t *testing.T) {
	g := started()
	g.player, g.dir = point{3, 3}, right
	if got := g.target(0); got != g.player {
		t.Errorf("direct chaser target %v", got)
	}
	if got := g.target(1); got != (point{7, 3}) {
		t.Errorf("ahead target %v, want 7,3", got)
	}
	g.player, g.dir = point{5, 3}, up // 4 ahead is off the map: clamp to 5,0 (wall), snap to 5,1
	if got := g.target(1); got != (point{5, 1}) {
		t.Errorf("snapped ahead target %v, want 5,1", got)
	}
	g.dir = point{}
	if got := g.target(1); got != g.player {
		t.Errorf("standing-still ahead target %v", got)
	}

	if got := g.target(2); got != patrolRoute[0] {
		t.Errorf("patrol target %v", got)
	}
	g.chasers[2].pos = patrolRoute[0]
	g.stepChaser(2)
	if g.chasers[2].waypoint != 1 || g.target(2) != patrolRoute[1] {
		t.Errorf("patrol did not advance: waypoint %d", g.chasers[2].waypoint)
	}

	g.player = maze.player // 9 steps from chaser 4's spawn: chase
	if got := g.target(3); got != g.player {
		t.Errorf("far shy target %v", got)
	}
	g.chasers[3].pos = point{16, 11} // 2 steps: retreat
	if got := g.target(3); got != shyCorner {
		t.Errorf("near shy target %v", got)
	}

	// The four policies give four different targets in the same situation.
	g = started()
	g.player, g.dir = point{3, 3}, right
	g.chasers[3].pos = point{5, 3}
	seen := map[point]int{}
	for i := range numChasers {
		seen[g.target(i)]++
	}
	if len(seen) != numChasers {
		t.Errorf("chaser targets not distinct: %v", seen)
	}
}

func TestChaserTargetsAlwaysOpen(t *testing.T) {
	g := started()
	for y := range Rows {
		for x := range Cols {
			p := point{x, y}
			if !open(p) {
				continue
			}
			for _, d := range append(dirOrder[:], point{}) {
				g.player, g.dir = p, d
				g.chasers[3].pos = p
				for i := range numChasers {
					if tg := g.target(i); !open(tg) {
						t.Fatalf("chaser %d target %v not open (player %v dir %v)", i+1, tg, p, d)
					}
				}
			}
		}
	}
}

func TestChaserTieBreakAndNoReverse(t *testing.T) {
	g := started()
	// Left and right are equally far from a target on the symmetry axis:
	// left wins (order up, left, down, right).
	g.player = maze.player
	g.chasers[0] = chaser{pos: point{14, 3}}
	g.stepChaser(0)
	if g.chasers[0].pos != (point{13, 3}) {
		t.Errorf("left/right tie went to %v", g.chasers[0].pos)
	}
	// Up and down tie toward (5,8): up wins.
	g.player = point{5, 8}
	g.chasers[0] = chaser{pos: point{1, 8}}
	g.stepChaser(0)
	if g.chasers[0].pos != (point{1, 7}) {
		t.Errorf("up/down tie went to %v", g.chasers[0].pos)
	}
	// No reversal: heading left with the player to the right.
	g.player = point{20, 3}
	g.chasers[0] = chaser{pos: point{14, 3}, dir: left}
	g.stepChaser(0)
	if g.chasers[0].pos != (point{13, 3}) {
		t.Errorf("chaser reversed to %v", g.chasers[0].pos)
	}
	// Chase picks the shorter way.
	g.chasers[0] = chaser{pos: point{14, 3}}
	g.stepChaser(0)
	if g.chasers[0].pos != (point{15, 3}) {
		t.Errorf("chaser went %v, away from the player", g.chasers[0].pos)
	}
	// Vulnerable chasers flee: largest distance from the player.
	g.powerUntil = time.Hour
	g.player = point{12, 3}
	g.chasers[0] = chaser{pos: point{14, 3}}
	g.stepChaser(0)
	if g.chasers[0].pos != (point{15, 3}) {
		t.Errorf("fleeing chaser went %v", g.chasers[0].pos)
	}
}

func TestRunsAreDeterministic(t *testing.T) {
	a, b := started(), started()
	for range 100 {
		a.Update(50 * ms)
		b.Update(50 * ms)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two identical runs diverged")
	}
}

func TestSameCellContactCostsALife(t *testing.T) {
	g := started()
	isolate(g, 0)
	g.player = point{5, 3}
	g.chasers[0] = chaser{pos: point{7, 3}, dir: left, next: chaserStep}
	g.Update(359 * ms)
	if g.lives != 3 || g.chasers[0].pos != (point{6, 3}) {
		t.Fatalf("lives %d chaser %v", g.lives, g.chasers[0].pos)
	}
	g.Update(ms)
	if g.lives != 2 || g.player != maze.player || g.chasers[0].pos != maze.chasers[0] {
		t.Fatalf("lives %d player %v", g.lives, g.player)
	}
}

func TestStaggeredCrossingIsCaught(t *testing.T) {
	g := started()
	isolate(g, 0)
	g.player, g.dir = point{5, 3}, right
	g.chasers[0] = chaser{pos: point{7, 3}, dir: left, next: chaserStep}
	g.Update(180 * ms) // player reaches 6,3 at 140, chaser enters it at 180
	if g.lives != 2 {
		t.Fatalf("lives %d", g.lives)
	}
}

func TestSwappedCellsContact(t *testing.T) {
	for _, vulnerable := range []bool{false, true} {
		g := started()
		isolate(g, 0)
		g.player, g.dir, g.playerNext = point{5, 3}, right, 180*ms
		g.chasers[0] = chaser{pos: point{6, 3}, dir: left, next: 180 * ms} // only way on is left
		if vulnerable {
			g.powerUntil = time.Hour
		}
		g.Update(180 * ms)
		switch {
		case !vulnerable && (g.lives != 2 || g.player != maze.player):
			t.Errorf("swap not lethal: lives %d player %v chaser %v", g.lives, g.player, g.chasers[0].pos)
		case vulnerable && (g.lives != 3 || !g.chasers[0].removed || g.score != pelletPoints+chaserPoints):
			t.Errorf("swap not eaten: lives %d score %d", g.lives, g.score)
		}
	}
}

func TestPowerActivationAndExpiryBoundary(t *testing.T) {
	g := started()
	isolate(g)
	g.player, g.dir = point{2, 1}, left
	g.Update(140 * ms)
	if g.player != (point{1, 1}) || g.score != powerPoints || g.powerUntil != 140*ms+powerDuration || !g.vulnerable() {
		t.Fatalf("power not active: player %v score %d until %v", g.player, g.score, g.powerUntil)
	}
	advanceTo(g, 140*ms+powerDuration-ms)
	if !g.vulnerable() {
		t.Fatal("power ended early")
	}
	g.Update(ms)
	if g.vulnerable() {
		t.Fatal("power still active at its end time")
	}
}

func TestPowerRefresh(t *testing.T) {
	g := started()
	isolate(g)
	g.player, g.dir = point{2, 1}, left
	g.Update(140 * ms)
	g.player, g.dir = point{26, 1}, right
	g.Update(140 * ms) // eats the second power pellet at 280 ms
	if g.score != 2*powerPoints || g.powerUntil != 280*ms+powerDuration {
		t.Fatalf("score %d until %v", g.score, g.powerUntil)
	}
	advanceTo(g, 140*ms+powerDuration)
	if !g.vulnerable() {
		t.Fatal("refresh did not extend the timer")
	}
	advanceTo(g, 280*ms+powerDuration)
	if g.vulnerable() {
		t.Fatal("refreshed power did not expire")
	}
}

func TestVulnerableChaserInterval(t *testing.T) {
	g := started()
	isolate(g, 0)
	g.graceUntil = time.Hour // keep contacts out of this timing test
	g.player, g.dir = point{2, 1}, left
	g.Update(180 * ms) // power at 140, chaser steps at 180
	if g.chasers[0].next != 440*ms {
		t.Fatalf("vulnerable chaser next step %v, want 440ms", g.chasers[0].next)
	}
	// Steps at 180+260k; the first at or after expiry (8.14 s) is 8.24 s and
	// schedules the next one at the normal pace.
	advanceTo(g, 8240*ms)
	if g.vulnerable() || g.chasers[0].next != 8420*ms {
		t.Fatalf("after expiry next step %v", g.chasers[0].next)
	}
}

func TestPowerAppliesBeforeContactOfTheSameStep(t *testing.T) {
	g := started()
	isolate(g, 0)
	g.player, g.dir = point{2, 1}, left
	g.chasers[0] = chaser{pos: point{1, 1}, next: time.Hour} // parked on the power pellet
	g.Update(140 * ms)
	if g.lives != 3 || !g.chasers[0].removed || g.score != powerPoints+chaserPoints {
		t.Fatalf("lives %d removed %v score %d", g.lives, g.chasers[0].removed, g.score)
	}
}

func TestEatenChaserRespawnAndGrace(t *testing.T) {
	g := started()
	isolate(g, 0)
	g.powerUntil = time.Hour
	g.player = point{5, 3}
	g.chasers[0] = chaser{pos: point{6, 3}, dir: left, next: chaserStep}
	g.Update(180 * ms)
	c := &g.chasers[0]
	if !c.removed || g.score != chaserPoints || g.lives != 3 || c.respawnAt != 2180*ms {
		t.Fatalf("not eaten: %+v score %d", *c, g.score)
	}
	advanceTo(g, 2179*ms)
	if !c.removed {
		t.Fatal("respawned early")
	}
	g.Update(ms)
	if c.removed || c.pos != maze.chasers[0] || c.graceUntil != 3180*ms {
		t.Fatalf("respawn: %+v", *c)
	}
	if out := renderStrict(t, g, 80, 24); !strings.Contains(out, "~1") {
		t.Fatalf("harmless chaser not shown:\n%s", out)
	}
	// Park it and put the player on it: harmless during grace.
	c.next = time.Hour
	g.player = c.pos
	advanceTo(g, 3179*ms)
	if c.removed || g.score != chaserPoints || g.lives != 3 {
		t.Fatalf("contact during grace: removed %v score %d", c.removed, g.score)
	}
	g.Update(ms) // grace ends at 3.18 s: the touching chaser is eaten again
	if !c.removed || g.score != 2*chaserPoints {
		t.Fatalf("contact after grace: removed %v score %d", c.removed, g.score)
	}
}

func TestOneLifePerEventAndProgressPreserved(t *testing.T) {
	g := started()
	isolate(g, 0, 1)
	g.player = point{5, 3}
	g.chasers[0] = chaser{pos: point{7, 3}, dir: left, next: chaserStep}
	g.chasers[1] = chaser{pos: point{5, 5}, dir: up, next: chaserStep}
	g.items[17][5], g.items[17][6] = none, none
	g.remaining -= 2
	g.score = 1234
	items, remaining := g.items, g.remaining
	g.Update(360 * ms) // both chasers reach 5,3 in the same batch
	if g.lives != 2 {
		t.Fatalf("lives %d, want exactly one lost", g.lives)
	}
	if g.score != 1234 || g.remaining != remaining || g.items != items {
		t.Fatal("life loss changed score or pellets")
	}
	if g.player != maze.player || !g.dir.isZero() || g.vulnerable() || g.graceUntil != 2360*ms {
		t.Fatalf("player %v dir %v grace %v", g.player, g.dir, g.graceUntil)
	}
	for i, c := range g.chasers {
		if c.pos != maze.chasers[i] || !c.dir.isZero() || c.removed || c.next != 540*ms {
			t.Fatalf("chaser %d not reset: %+v", i, c)
		}
	}
	// Respawn grace: a chaser parked on the player is harmless until 2.36 s.
	g.chasers[0].pos, g.chasers[0].next = g.player, time.Hour
	advanceTo(g, 2359*ms)
	if g.lives != 2 {
		t.Fatalf("lost a life during grace: %d", g.lives)
	}
	g.Update(ms)
	if g.lives != 1 {
		t.Fatalf("lives %d after grace", g.lives)
	}
}

func TestFinalPelletAndLethalContact(t *testing.T) {
	for _, lives := range []int{1, 2} {
		g := started()
		isolate(g, 0)
		onlyItem(g, point{6, 3})
		g.lives = lives
		g.player, g.dir, g.playerNext = point{5, 3}, right, chaserStep
		g.chasers[0] = chaser{pos: point{7, 3}, dir: left, next: chaserStep}
		g.Update(chaserStep)
		if g.remaining != 0 || g.lives != lives-1 {
			t.Fatalf("lives %d: remaining %d lives %d", lives, g.remaining, g.lives)
		}
		want := won
		if lives == 1 {
			want = lost // lethal contact wins over the final pellet
		}
		if g.state != want {
			t.Errorf("lives %d: state %v, want %v", lives, g.state, want)
		}
	}
}

func TestWin(t *testing.T) {
	g := started()
	isolate(g)
	onlyItem(g, point{6, 3})
	g.player, g.dir = point{5, 3}, right
	g.Update(time.Second)
	if g.state != won || !g.Finished() || g.Score() != pelletPoints || g.now != 140*ms {
		t.Fatalf("state %v score %d now %v", g.state, g.score, g.now)
	}
	frozen := *g
	g.Update(time.Minute)
	g.HandleInput(engine.KeyLeft)
	g.HandleInput(engine.KeyAction)
	if !reflect.DeepEqual(*g, frozen) {
		t.Fatal("finished run changed")
	}
	out := renderStrict(t, g, 80, 24)
	for _, want := range []string{"YOU WIN", "Final score 10", "Enter: play again"} {
		if !strings.Contains(out, want) {
			t.Errorf("win screen lacks %q:\n%s", want, out)
		}
	}
}

func TestLoseAndRestart(t *testing.T) {
	g := started()
	isolate(g, 0)
	g.lives = 1
	g.score = 70
	g.player = point{5, 3}
	g.chasers[0] = chaser{pos: point{6, 3}, dir: left, next: chaserStep}
	g.Update(time.Second)
	if g.state != lost || g.lives != 0 || !g.Finished() || g.Score() != 70 {
		t.Fatalf("state %v lives %d", g.state, g.lives)
	}
	out := renderStrict(t, g, 80, 24)
	for _, want := range []string{"GAME OVER", "Final score 70", "Enter: play again", "Lives 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("game over screen lacks %q:\n%s", want, out)
		}
	}
	g.HandleInput(engine.KeySelect)
	if !reflect.DeepEqual(g, started()) {
		t.Fatalf("restart is not a fresh run: %+v", g)
	}
}

// TestFrameChunkingInvariance replays the same inputs at the same gameplay
// times with different frame sizes; every state must match exactly.
func TestFrameChunkingInvariance(t *testing.T) {
	script := []struct {
		at  time.Duration
		key engine.Key
	}{
		{0, engine.KeyLeft}, {700 * ms, engine.KeyUp}, {1900 * ms, engine.KeyRight},
		{3100 * ms, engine.KeyDown}, {4500 * ms, engine.KeyLeft}, {6000 * ms, engine.KeyUp},
		{8000 * ms, engine.KeyRight}, {11000 * ms, engine.KeyDown}, {15000 * ms, engine.KeyLeft},
		{20000 * ms, engine.KeyUp}, {30000 * ms, engine.KeyNone},
	}
	run := func(chunk time.Duration) []Game {
		g := started()
		var snaps []Game
		for _, s := range script {
			for g.now < s.at && !g.Finished() {
				g.Update(min(chunk, s.at-g.now))
			}
			g.HandleInput(s.key)
			snaps = append(snaps, *g)
		}
		return snaps
	}
	ref := run(ms)
	last := ref[len(ref)-1]
	if last.score == 0 || (last.lives == startLives && last.state == playing) {
		t.Fatalf("script too quiet to be meaningful: score %d lives %d", last.score, last.lives)
	}
	for _, chunk := range []time.Duration{7 * ms, 33 * ms, 100 * ms, time.Second} {
		if got := run(chunk); !reflect.DeepEqual(got, ref) {
			for i := range ref {
				if !reflect.DeepEqual(got[i], ref[i]) {
					t.Fatalf("chunk %v diverged at script step %d:\n got %+v\nwant %+v", chunk, i, got[i], ref[i])
				}
			}
		}
	}
}

func TestEngineIntegrationPauseResizeRestart(t *testing.T) {
	g := newGame()
	e := engine.New(g)
	e.Resize(80, 24)
	isolate(g)
	e.Advance(time.Second) // dropped first frame
	if g.now != 0 {
		t.Fatal("first frame not dropped")
	}
	e.Input(engine.Event{Key: engine.KeyLeft})
	e.Advance(100 * ms)
	e.Advance(100 * ms)
	if g.player != (point{13, 11}) || g.now != 200*ms {
		t.Fatalf("player %v now %v", g.player, g.now)
	}
	at80 := renderStrict(t, g, 80, 24)
	snap := *g

	e.Input(engine.Event{Key: engine.KeyPause})
	e.Input(engine.Event{Key: engine.KeyUp}) // not delivered while paused
	e.Resize(40, 10)                         // undersized: suspended
	e.Advance(time.Minute)
	e.Resize(120, 40)
	e.Advance(time.Minute)
	e.Input(engine.Event{Key: engine.KeyPause})
	e.Advance(time.Minute) // dropped: spans the pause
	snap.width, snap.height = 120, 40
	if !e.Ready() || e.Paused() || !reflect.DeepEqual(*g, snap) {
		t.Fatalf("pause/resize changed the game:\n got %+v\nwant %+v", *g, snap)
	}
	at120 := renderStrict(t, g, 120, 40)
	if playerCell(at80) == playerCell(at120) {
		t.Fatal("maze not re-centered on the larger screen")
	}
	e.Advance(100 * ms)
	if g.now != 300*ms || g.player != (point{12, 11}) {
		t.Fatalf("did not resume: now %v player %v", g.now, g.player)
	}

	g.state = lost // end screen: Space is ignored, Enter restarts
	e.Input(engine.Event{Key: engine.KeyPause})
	if e.Paused() {
		t.Fatal("end screen paused")
	}
	e.Input(engine.Event{Key: engine.KeySelect})
	fresh := newGame()
	fresh.Start(120, 40)
	if !reflect.DeepEqual(g, fresh) {
		t.Fatal("Enter did not restart a fresh run")
	}
}

func playerCell(screen string) [2]int {
	for y, line := range strings.Split(screen, "\n") {
		if x := strings.Index(line, "@@"); x >= 0 {
			return [2]int{x, y}
		}
	}
	return [2]int{-1, -1}
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

func TestRenderFits(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {81, 25}, {120, 40}} {
		g := started()
		g.Resize(size[0], size[1])
		out := renderStrict(t, g, size[0], size[1])
		if n := strings.Count(out, "@@"); n != 2 { // the player and its legend entry
			t.Errorf("%v: %d @@ glyphs, want the player plus the legend:\n%s", size, n, out)
		}
		for _, want := range []string{"MAZE CHASE", "Score 0", "Lives 3", "Power  --", "C1", "C2", "C3", "C4", "()",
			"Move: arrows / WASD", "Pause: Space", "Leave: Q / Esc", "Exit: Ctrl+C"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: screen lacks %q", size, want)
			}
		}
		if strings.Count(out, "()") != 5 { // four power pellets plus the legend
			t.Errorf("%v: power pellets: %d", size, strings.Count(out, "()"))
		}
	}

	g := started()
	g.powerUntil = 7500 * ms
	out := renderStrict(t, g, 80, 24)
	if !strings.Contains(out, "Power  7.5s") || !strings.Contains(out, "c2") || strings.Contains(out, "C2") {
		t.Errorf("vulnerable rendering:\n%s", out)
	}
}

func TestRenderShowsExactlyOnePlayer(t *testing.T) {
	g := started()
	for range 60 {
		g.Update(100 * ms)
		out := renderStrict(t, g, 80, 24)
		board := strings.Split(out, "\n")[hudH : hudH+Rows]
		if n := strings.Count(strings.Join(board, "\n"), "@@"); n != 1 {
			t.Fatalf("%d player glyphs on the board:\n%s", n, out)
		}
	}
}
