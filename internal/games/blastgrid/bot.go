package blastgrid

import "time"

// Bot policy. Every botInterval each living bot, in index order, makes one
// decision from the current state:
//
//  1. Build a danger map: active flames, and the predicted flame window of
//     every pending bomb (chains move a bomb's predicted time earlier).
//  2. If its cell will burn, follow the shortest path (in decision ticks) to
//     a cell that is never threatened again, moving or waiting only through
//     cells that are not burning while the bot would be there.
//  3. Otherwise, if its bomb is free and a blast from here would hit an
//     opponent or a crate, place a bomb, but only if the same escape search
//     finds a route that does not re-enter the new bomb; then take the
//     route's first step at once.
//  4. Otherwise walk toward the nearest safe cell from which a bomb would
//     threaten an opponent (within huntSteps), else break a crate, else
//     wander to any safe neighbor.
//
// Ties between equally short routes are broken by a direction order the
// run's seeded RNG shuffles once per decision, so runs are reproducible.
const (
	escapeSteps = int((fuseTime+flameTime)/botInterval) + 2
	huntSteps   = 10
	crateSteps  = 40
)

var stay = point{}

// window is a span of gameplay time during which a cell burns: an actor
// standing there at any moment in [from, to) dies.
type window struct{ from, to time.Duration }

type dangerMap [Rows][Cols][]window

// unsafe reports whether a bot that moves onto p at arrive and stays until
// its next decision at leave can be burned there. Explosions at leave are
// resolved before that decision, so leave is inclusive.
func (dm *dangerMap) unsafe(p point, arrive, leave time.Duration) bool {
	for _, w := range dm[p.y][p.x] {
		if w.from <= leave && arrive < w.to {
			return true
		}
	}
	return false
}

// threatened reports whether p burns at any time at or after at.
func (dm *dangerMap) threatened(p point, at time.Duration) bool {
	for _, w := range dm[p.y][p.x] {
		if w.to > at {
			return true
		}
	}
	return false
}

// danger predicts burning windows from active flames and pending bombs,
// optionally including a hypothetical extra bomb. A bomb reached by another
// bomb's rays explodes no later than that bomb.
func (g *Game) danger(extra *bomb) *dangerMap {
	dm := new(dangerMap)
	for y := range Rows {
		for x := range Cols {
			if until := g.flameUntil[y][x]; g.now < until {
				dm[y][x] = append(dm[y][x], window{g.now, until})
			}
		}
	}
	bs := append([]*bomb(nil), g.bombs...)
	if extra != nil {
		bs = append(bs, extra)
	}
	index := func(p point) int {
		for i, b := range bs {
			if b.pos == p {
				return i
			}
		}
		return -1
	}
	isBomb := func(p point) bool { return index(p) >= 0 }
	eff := make([]time.Duration, len(bs))
	cells := make([][]point, len(bs))
	hits := make([][]int, len(bs))
	for i, b := range bs {
		eff[i] = b.explodeAt
		var hp []point
		cells[i], hp = g.trace(b.pos, isBomb)
		for _, p := range hp {
			hits[i] = append(hits[i], index(p))
		}
	}
	for changed := true; changed; {
		changed = false
		for i := range bs {
			for _, j := range hits[i] {
				if eff[i] < eff[j] {
					eff[j] = eff[i]
					changed = true
				}
			}
		}
	}
	for i := range bs {
		for _, p := range cells[i] {
			dm[p.y][p.x] = append(dm[p.y][p.x], window{eff[i], eff[i] + flameTime})
		}
	}
	return dm
}

// plan searches decision ticks j = 0..maxSteps for bot i. At tick j the bot
// moves (or waits) onto a cell it then occupies from now+j*botInterval until
// the next tick; such a cell must be walkable and not burning meanwhile.
// It returns the first cell of the shortest route to a cell satisfying goal
// (which is told the arrival time), or false if none exists. extra is a
// hypothetical bomb on the bot's cell that blocks re-entry once left.
func (g *Game) plan(i int, extra *bomb, dm *dangerMap, order []point, maxSteps int, goal func(p point, arrive time.Duration) bool) (point, bool) {
	type node struct{ p, first point }
	start := g.actors[i].pos
	frontier := []node{{start, start}}
	for j := 0; j <= maxSteps && len(frontier) > 0; j++ {
		arrive := g.now + time.Duration(j)*botInterval
		leave := arrive + botInterval
		var seen [Rows][Cols]bool
		var next []node
		for _, n := range frontier {
			for _, d := range order {
				q := n.p.add(d)
				if seen[q.y][q.x] || (q != n.p && !g.walkable(q, i, extra)) || dm.unsafe(q, arrive, leave) {
					continue
				}
				seen[q.y][q.x] = true
				first := n.first
				if j == 0 {
					first = q
				}
				if goal(q, arrive) {
					return first, true
				}
				next = append(next, node{q, first})
			}
		}
		frontier = next
	}
	return point{}, false
}

// blastHits reports whether a bomb at p (with the current arena) would reach
// a living opponent of bot i, and whether it would break a crate.
func (g *Game) blastHits(i int, p point) (opponent, crates bool) {
	cells, _ := g.trace(p, func(q point) bool { return g.bombAt(q) != nil })
	for _, c := range cells {
		if g.grid[c.y][c.x] == crate {
			crates = true
		}
		if a := g.actorAt(c); a != noActor && a != i {
			opponent = true
		}
	}
	return opponent, crates
}

// think makes one decision for living bot i (see the policy above).
func (g *Game) think(i int) {
	order := make([]point, 0, 5)
	order = append(order, dirs[:]...)
	g.rng.Shuffle(len(dirs), func(a, b int) { order[a], order[b] = order[b], order[a] })
	order = append(order, stay)

	pos := g.actors[i].pos
	dm := g.danger(nil)
	safe := func(p point, arrive time.Duration) bool { return !dm.threatened(p, arrive) }

	if dm.threatened(pos, g.now) {
		if first, ok := g.plan(i, nil, dm, order, escapeSteps, safe); ok {
			g.step(i, first)
		}
		return // no safe route: hold position
	}

	if opp, crates := g.blastHits(i, pos); (opp || crates) && !g.hasBomb(i) && g.bombAt(pos) == nil {
		nb := &bomb{pos: pos, owner: i, explodeAt: g.now + fuseTime}
		dm2 := g.danger(nb)
		safe2 := func(p point, arrive time.Duration) bool { return !dm2.threatened(p, arrive) }
		if first, ok := g.plan(i, nb, dm2, order, escapeSteps, safe2); ok {
			g.placeBomb(i)
			g.step(i, first)
			return
		}
	}

	var useOpp, useCrate [Rows][Cols]bool
	for y := range Rows {
		for x := range Cols {
			if g.grid[y][x] == floor {
				useOpp[y][x], useCrate[y][x] = g.blastHits(i, point{x, y})
			}
		}
	}
	for _, target := range []struct {
		use   *[Rows][Cols]bool
		steps int
	}{{&useOpp, huntSteps}, {&useCrate, crateSteps}, {nil, 0}} {
		goal := func(p point, arrive time.Duration) bool {
			return p != pos && (target.use == nil || target.use[p.y][p.x]) && safe(p, arrive)
		}
		if first, ok := g.plan(i, nil, dm, order, target.steps, goal); ok {
			g.step(i, first)
			return
		}
	}
}

// step moves bot i onto the adjacent cell first (or keeps it in place).
func (g *Game) step(i int, first point) {
	if d := (point{first.x - g.actors[i].pos.x, first.y - g.actors[i].pos.y}); d != stay {
		g.move(i, d)
	}
}
