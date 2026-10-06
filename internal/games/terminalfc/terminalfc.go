// Package terminalfc implements DevCade's 5v5 terminal arcade football game.
//
// The simulation is backend-independent. All time is gameplay time delivered
// through Update, input is normalized by the shared engine, and rendering is
// read-only. The pitch uses 36x18 logical cells rendered as two terminal
// columns per cell.
package terminalfc

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/gameui"
)

const (
	PitchW = 36
	PitchH = 18

	fixedStep       = 20 * time.Millisecond
	matchTime       = 180 * time.Second
	kickoffTime     = time.Second
	goalOverlayTime = 1500 * time.Millisecond
	restartTime     = time.Second
	keeperHoldTime  = 2 * time.Second

	moveCooldown        = 80 * time.Millisecond
	actionCooldown      = 300 * time.Millisecond
	humanTackleCooldown = 700 * time.Millisecond
	switchCooldown      = 250 * time.Millisecond
	reclaimGrace        = 150 * time.Millisecond

	passSpeed        = 12.0
	lobSpeed         = 14.0
	shotSpeed        = 20.0
	freeDecel        = 4.0
	interactRadius   = 0.60
	humanTackleRange = 1.20
)

const (
	homeTeam = 0
	awayTeam = 1
	noPlayer = -1
)

type difficulty uint8

const (
	difficultyEasy difficulty = iota
	difficultyNormal
	difficultyHard
)

type difficultyConfig struct {
	name              string
	botInterval       time.Duration
	botSpeed          float64
	botTackleCooldown time.Duration
	botTackleRange    float64
	ownerProtection   time.Duration
	pressDelay        time.Duration
}

var difficultyConfigs = [...]difficultyConfig{
	{
		name:              "EASY",
		botInterval:       240 * time.Millisecond,
		botSpeed:          4.0,
		botTackleCooldown: 1200 * time.Millisecond,
		botTackleRange:    0.85,
		ownerProtection:   550 * time.Millisecond,
		pressDelay:        450 * time.Millisecond,
	},
	{
		name:              "NORMAL",
		botInterval:       180 * time.Millisecond,
		botSpeed:          4.5,
		botTackleCooldown: 1000 * time.Millisecond,
		botTackleRange:    1.0,
		ownerProtection:   450 * time.Millisecond,
		pressDelay:        350 * time.Millisecond,
	},
	{
		name:              "HARD",
		botInterval:       120 * time.Millisecond,
		botSpeed:          6.0,
		botTackleCooldown: 700 * time.Millisecond,
		botTackleRange:    1.20,
		ownerProtection:   250 * time.Millisecond,
		pressDelay:        0,
	},
}

func (d difficulty) config() difficultyConfig {
	return difficultyConfigs[d]
}

type vec struct{ x, y float64 }

func (a vec) add(b vec) vec     { return vec{a.x + b.x, a.y + b.y} }
func (a vec) sub(b vec) vec     { return vec{a.x - b.x, a.y - b.y} }
func (a vec) mul(k float64) vec { return vec{a.x * k, a.y * k} }
func (a vec) len() float64      { return math.Hypot(a.x, a.y) }
func (a vec) norm() vec {
	n := a.len()
	if n == 0 {
		return vec{}
	}
	return vec{a.x / n, a.y / n}
}
func dot(a, b vec) float64            { return a.x*b.x + a.y*b.y }
func dist(a, b vec) float64           { return a.sub(b).len() }
func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

type role uint8

const (
	goalkeeper role = iota
	defender
	midfielder
	forward
)

type player struct {
	id         int
	team       int
	role       role
	pos        vec
	facing     vec
	nextAction time.Duration
	nextTackle time.Duration
}

type ballMode uint8

const (
	ballCarried ballMode = iota
	ballFree
	ballPass
	ballLob
	ballShot
	ballKeeper
)

type ballState struct {
	pos          vec
	vel          vec
	mode         ballMode
	owner        int
	lastTouch    int
	releasedBy   int
	reclaimAfter time.Duration
	protectedTil time.Duration
	holdUntil    time.Duration
}

type phase uint8

const (
	phaseDifficulty phase = iota
	phaseKickoff
	phaseLive
	phaseGoal
	phaseRestart
	phaseFullTime
)

type restartKind uint8

const (
	restartThrow restartKind = iota
	restartGoalKick
	restartCorner
)

// Game implements engine.Game and engine.Finisher.
type Game struct {
	seed *rand.Rand
	rng  *rand.Rand

	players    [10]player
	ball       ballState
	active     int
	difficulty difficulty

	phase          phase
	phaseLeft      time.Duration
	liveLeft       time.Duration
	accum          time.Duration
	now            time.Duration
	nextThink      time.Duration
	nextMove       time.Duration
	nextSwitch     time.Duration
	pressAllowedAt time.Duration
	homeGoals      int
	awayGoals      int
	finalScore     int
	kickoffTeam    int
	restartTeam    int
	restartSpot    vec
	restartKind    restartKind
	restartTaker   int
	passTarget     int
	lastDecision   [10]string
	width, height  int
}

var (
	_ engine.Game     = (*Game)(nil)
	_ engine.Finisher = (*Game)(nil)
)

// New creates a fresh game with production randomness used only for bounded
// bot tie-breaking. Normal is selected by default; difficulty changes bot
// pressure and timing while preserving the match rules and scoring formula.
func New() engine.Game {
	return newGame(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
}

func newGame(seed *rand.Rand) *Game {
	g := &Game{seed: seed}
	g.reset()
	return g
}

func (g *Game) MinimumSize() (int, int) { return 80, 24 }
func (g *Game) Start(w, h int)          { g.width, g.height = w, h; g.reset() }
func (g *Game) Resize(w, h int)         { g.width, g.height = w, h }
func (g *Game) Finished() bool          { return g.phase == phaseFullTime }

// Score returns zero before full time and the finalized leaderboard score
// afterwards. Football goals remain separate match state.
func (g *Game) Score() int {
	if !g.Finished() {
		return 0
	}
	return g.finalScore
}

func (g *Game) reset() {
	g.rng = rand.New(rand.NewPCG(g.seed.Uint64(), g.seed.Uint64()))
	g.difficulty = difficultyNormal
	g.phase = phaseDifficulty
	g.phaseLeft = 0
	g.liveLeft = matchTime
	g.homeGoals, g.awayGoals, g.finalScore = 0, 0, 0
	g.now, g.accum, g.nextThink = 0, 0, 0
	g.nextMove, g.nextSwitch, g.pressAllowedAt = 0, 0, 0
	g.kickoffTeam = homeTeam
	g.active = 1
	g.passTarget = noPlayer
	g.setupFormation()
}

func (g *Game) startMatch() {
	cfg := g.difficulty.config()
	g.homeGoals, g.awayGoals, g.finalScore = 0, 0, 0
	g.now, g.accum = 0, 0
	g.nextThink = cfg.botInterval
	g.nextMove, g.nextSwitch, g.pressAllowedAt = 0, 0, 0
	g.liveLeft = matchTime
	g.kickoffTeam = homeTeam
	g.passTarget = noPlayer
	g.setupKickoff(homeTeam)
}

func (g *Game) setupFormation() {
	type spec struct {
		r    role
		x, y float64
	}
	home := [5]spec{
		{goalkeeper, 1.5, 9}, {defender, 8, 9}, {midfielder, 14, 5},
		{midfielder, 14, 13}, {forward, 18, 9},
	}
	for i, s := range home {
		g.players[i] = player{id: i, team: homeTeam, role: s.r, pos: vec{s.x, s.y}, facing: vec{1, 0}}
	}
	for i, s := range home {
		j := i + 5
		g.players[j] = player{id: j, team: awayTeam, role: s.r, pos: vec{PitchW - s.x, s.y}, facing: vec{-1, 0}}
	}
}

func (g *Game) setupKickoff(team int) {
	g.setupFormation()
	g.phase = phaseKickoff
	g.phaseLeft = kickoffTime
	g.kickoffTeam = team
	g.passTarget = noPlayer
	g.clearCooldowns()
	taker := 4
	if team == awayTeam {
		taker = 9
	}
	g.players[taker].pos = vec{PitchW / 2, PitchH / 2}
	g.ball = ballState{pos: g.players[taker].pos, owner: taker, lastTouch: taker, releasedBy: noPlayer, mode: ballCarried}
	if team == homeTeam {
		g.active = taker
	} else {
		g.active = g.nearestHome(g.ball.pos, noPlayer)
	}
}

func (g *Game) clearCooldowns() {
	for i := range g.players {
		g.players[i].nextAction = 0
		g.players[i].nextTackle = 0
	}
	g.nextMove, g.nextSwitch = 0, 0
}

func (g *Game) HandleInput(k engine.Key) {
	if g.phase == phaseDifficulty {
		switch k {
		case engine.KeyUp, engine.KeyLeft:
			g.difficulty = difficulty((int(g.difficulty) + len(difficultyConfigs) - 1) % len(difficultyConfigs))
		case engine.KeyDown, engine.KeyRight:
			g.difficulty = difficulty((int(g.difficulty) + 1) % len(difficultyConfigs))
		case engine.KeySelect:
			g.startMatch()
		}
		return
	}
	if g.phase == phaseFullTime {
		if k == engine.KeySelect {
			g.phase = phaseDifficulty
		}
		return
	}
	if g.phase != phaseLive {
		return
	}
	switch k {
	case engine.KeyTertiary:
		g.switchPlayer()
	case engine.KeyAction:
		if g.ball.owner == g.active {
			g.lobPass(g.active)
		} else {
			g.tackle(g.active)
		}
	case engine.KeySecondary:
		if g.ball.owner == g.active {
			g.pass(g.active, true)
		} else {
			g.pressHuman()
		}
	case engine.KeySelect:
		if g.ball.owner == g.active {
			g.shoot(g.active)
		}
	case engine.KeyUp, engine.KeyDown, engine.KeyLeft, engine.KeyRight:
		g.moveHuman(k)
	}
}

func (g *Game) moveHuman(k engine.Key) {
	if g.now < g.nextMove || g.active < 1 || g.active > 4 {
		return
	}
	d := vec{}
	switch k {
	case engine.KeyUp:
		d.y = -1
	case engine.KeyDown:
		d.y = 1
	case engine.KeyLeft:
		d.x = -1
	case engine.KeyRight:
		d.x = 1
	}
	p := &g.players[g.active]
	p.facing = d
	to := p.pos.add(d)
	if !g.legalPlayerPos(to, p.role) || g.playerBlocked(g.active, to, 0.55) {
		return
	}
	from := p.pos
	p.pos = to
	g.nextMove = g.now + moveCooldown
	if g.ball.owner == g.active {
		g.ball.pos = p.pos
	} else if g.ball.owner == noPlayer {
		g.tryAcquireOnSegment(g.active, from, to)
	}
}

func (g *Game) pressHuman() {
	if g.active < 1 || g.active > 4 {
		return
	}
	target := g.ball.pos
	if g.ball.owner != noPlayer {
		target = g.players[g.ball.owner].pos
	}
	d := target.sub(g.players[g.active].pos)
	if d.len() < 0.5 {
		return
	}
	key := engine.KeyRight
	if math.Abs(d.x) >= math.Abs(d.y) {
		if d.x < 0 {
			key = engine.KeyLeft
		}
	} else if d.y < 0 {
		key = engine.KeyUp
	} else {
		key = engine.KeyDown
	}
	g.moveHuman(key)
}

func (g *Game) switchPlayer() {
	if g.now < g.nextSwitch {
		return
	}
	next := noPlayer
	if g.ball.owner >= 1 && g.ball.owner <= 4 {
		if g.active != g.ball.owner {
			next = g.ball.owner
		}
	}
	if next == noPlayer {
		next = g.nearestHome(g.ball.pos, g.active)
	}
	if next != noPlayer {
		g.active = next
		g.nextSwitch = g.now + switchCooldown
	}
}

func (g *Game) Update(dt time.Duration) {
	if dt <= 0 || g.phase == phaseDifficulty || g.phase == phaseFullTime {
		return
	}
	g.accum += dt
	for g.accum >= fixedStep && g.phase != phaseFullTime {
		g.step(fixedStep)
		g.accum -= fixedStep
	}
}

func (g *Game) step(dt time.Duration) {
	switch g.phase {
	case phaseKickoff, phaseGoal, phaseRestart:
		g.now += dt
		g.phaseLeft -= dt
		if g.phaseLeft <= 0 {
			switch g.phase {
			case phaseKickoff:
				g.phase = phaseLive
				if g.ball.owner != noPlayer && g.players[g.ball.owner].team == homeTeam {
					g.pressAllowedAt = g.now + g.difficulty.config().pressDelay
				}
			case phaseGoal:
				g.setupKickoff(g.kickoffTeam)
			case phaseRestart:
				g.executeRestart()
			}
		}
		return
	case phaseFullTime:
		return
	}

	if g.liveLeft <= 0 {
		g.finish()
		return
	}
	liveDT := dt
	if liveDT > g.liveLeft {
		liveDT = g.liveLeft
	}
	g.now += liveDT
	g.liveLeft -= liveDT

	for g.nextThink <= g.now && g.phase == phaseLive {
		g.botThink()
		g.nextThink += g.difficulty.config().botInterval
	}

	if g.phase == phaseLive {
		g.advanceBall(liveDT)
	}
	if g.phase == phaseLive && g.liveLeft <= 0 {
		g.finish()
	}
}

func (g *Game) finish() {
	if g.phase == phaseFullTime {
		return
	}
	g.phase = phaseFullTime
	g.liveLeft = 0
	result := 0
	switch {
	case g.homeGoals > g.awayGoals:
		result = 1000
	case g.homeGoals == g.awayGoals:
		result = 500
	}
	goals := min(10, g.homeGoals)
	diff := min(10, max(0, g.homeGoals-g.awayGoals))
	g.finalScore = result + 50*goals + 25*diff
	g.ball.vel = vec{}
}

func (g *Game) botThink() {
	for i := range g.players {
		if i == g.active {
			continue
		}
		if g.players[i].role == goalkeeper {
			g.thinkKeeper(i)
			continue
		}
		if g.ball.owner == i {
			g.thinkCarrier(i)
			continue
		}
		g.thinkOutfield(i)
	}
}

func (g *Game) thinkKeeper(i int) {
	p := &g.players[i]
	ownX := 1.5
	if p.team == awayTeam {
		ownX = PitchW - 1.5
	}
	target := vec{ownX, clamp(g.ball.pos.y, 6.5, 11.5)}
	if g.ball.owner == i {
		g.lastDecision[i] = "keeper outlet"
		if g.now >= g.ball.holdUntil {
			g.pass(i, false)
		}
		return
	}
	if g.ball.owner == noPlayer && dist(p.pos, g.ball.pos) < 3.5 {
		target = g.ball.pos
		g.lastDecision[i] = "keeper loose ball"
	} else {
		g.lastDecision[i] = "keeper track"
	}
	g.botMove(i, target)
}

func (g *Game) thinkCarrier(i int) {
	p := &g.players[i]
	goalX := float64(PitchW)
	if p.team == awayTeam {
		goalX = 0
	}
	if math.Abs(p.pos.x-goalX) < 9 && g.now >= p.nextAction {
		g.lastDecision[i] = "shoot"
		g.shoot(i)
		return
	}
	if g.nearestOpponentDistance(i) < 2.2 && g.now >= p.nextAction {
		g.lastDecision[i] = "pass under pressure"
		g.pass(i, false)
		return
	}
	g.lastDecision[i] = "dribble"
	target := vec{goalX, clamp(9+(p.pos.y-9)*0.25, 4, 14)}
	g.botMove(i, target)
}

func (g *Game) thinkOutfield(i int) {
	p := &g.players[i]
	if g.ball.owner != noPlayer && g.players[g.ball.owner].team != p.team {
		if p.team == awayTeam && g.players[g.ball.owner].team == homeTeam && g.now < g.pressAllowedAt {
			g.lastDecision[i] = "delay press"
			g.botMove(i, g.homeZone(i))
			return
		}
		presser := g.nearestTeamTo(p.team, g.ball.pos, goalkeeper)
		if presser == i {
			g.lastDecision[i] = "press"
			g.botMove(i, g.ball.pos)
			if dist(p.pos, g.ball.pos) <= g.difficulty.config().botTackleRange {
				g.tackle(i)
			}
			return
		}
	}
	if g.ball.owner == noPlayer {
		chaser := g.nearestTeamTo(p.team, g.ball.pos, goalkeeper)
		if chaser == i {
			g.lastDecision[i] = "chase free ball"
			g.botMove(i, g.ball.pos)
			return
		}
	}
	g.lastDecision[i] = "hold shape"
	g.botMove(i, g.homeZone(i))
}

func (g *Game) homeZone(i int) vec {
	p := g.players[i]
	x := 8.0
	switch p.role {
	case midfielder:
		x = 15
	case forward:
		x = 23
	}
	y := 9.0
	if i%5 == 2 {
		y = 5
	}
	if i%5 == 3 {
		y = 13
	}
	if p.team == awayTeam {
		x = PitchW - x
	}
	if g.ball.owner != noPlayer && g.players[g.ball.owner].team == p.team {
		if p.team == homeTeam {
			x += 2
		} else {
			x -= 2
		}
	}
	return vec{x, y}
}

func (g *Game) botMove(i int, target vec) {
	p := &g.players[i]
	d := target.sub(p.pos)
	if d.len() < 0.05 {
		return
	}
	cfg := g.difficulty.config()
	step := d.norm().mul(math.Min(cfg.botSpeed*cfg.botInterval.Seconds(), d.len()))
	if math.Abs(step.x) > math.Abs(step.y) {
		p.facing = vec{math.Copysign(1, step.x), 0}
	} else {
		p.facing = vec{0, math.Copysign(1, step.y)}
	}
	to := p.pos.add(step)
	if !g.legalPlayerPos(to, p.role) || g.playerBlocked(i, to, 0.48) {
		// deterministic sidestep, with seeded choice only for an exact tie.
		s := vec{-step.y, step.x}
		if g.rng.IntN(2) == 0 {
			s = s.mul(-1)
		}
		to = p.pos.add(s)
		if !g.legalPlayerPos(to, p.role) || g.playerBlocked(i, to, 0.48) {
			return
		}
	}
	p.pos = to
	if g.ball.owner == i {
		g.ball.pos = to
	} else if g.ball.owner == noPlayer && dist(to, g.ball.pos) <= interactRadius {
		g.acquire(i)
	}
}

func (g *Game) legalPlayerPos(v vec, r role) bool {
	if v.x < 0.5 || v.x > PitchW-0.5 || v.y < 0.5 || v.y > PitchH-0.5 {
		return false
	}
	return true
}

func (g *Game) playerBlocked(i int, to vec, radius float64) bool {
	for j := range g.players {
		if j != i && dist(g.players[j].pos, to) < radius {
			return true
		}
	}
	return false
}

func (g *Game) pass(i int, human bool) {
	g.passBall(i, human, ballPass, passSpeed)
}

func (g *Game) lobPass(i int) {
	g.passBall(i, true, ballLob, lobSpeed)
}

func (g *Game) passBall(i int, human bool, mode ballMode, speed float64) {
	p := &g.players[i]
	if g.ball.owner != i || g.now < p.nextAction {
		return
	}
	target := g.choosePassTarget(i, human)
	var aim vec
	if target != noPlayer {
		aim = g.players[target].pos.sub(p.pos).norm()
		g.passTarget = target
	} else {
		aim = p.facing.norm()
		if aim.len() == 0 {
			aim = vec{1, 0}
			if p.team == awayTeam {
				aim.x = -1
			}
		}
		g.passTarget = noPlayer
	}
	g.release(i, aim.mul(speed), mode)
	p.nextAction = g.now + actionCooldown
}

func (g *Game) choosePassTarget(i int, human bool) int {
	p := g.players[i]
	best := noPlayer
	bestScore := math.Inf(1)
	for j := range g.players {
		q := g.players[j]
		if j == i || q.team != p.team || q.role == goalkeeper {
			continue
		}
		d := q.pos.sub(p.pos)
		n := d.len()
		if n < 1 {
			continue
		}
		facing := p.facing.norm()
		if facing.len() == 0 {
			facing = vec{1, 0}
			if p.team == awayTeam {
				facing.x = -1
			}
		}
		align := dot(d.norm(), facing)
		if human && align < 0.15 {
			continue
		}
		lanePenalty := 0.0
		for k := range g.players {
			if g.players[k].team == p.team {
				continue
			}
			if segmentDistance(g.players[k].pos, p.pos, q.pos) < 1.0 {
				lanePenalty += 20
			}
		}
		score := (1-align)*8 + n*0.15 + lanePenalty
		if score < bestScore || (math.Abs(score-bestScore) < 1e-9 && j < best) {
			best, bestScore = j, score
		}
	}
	return best
}

func (g *Game) shoot(i int) {
	p := &g.players[i]
	if g.ball.owner != i || g.now < p.nextAction {
		return
	}
	goalX := float64(PitchW)
	if p.team == awayTeam {
		goalX = 0
	}
	targetY := 9.0
	if p.facing.y < -0.1 {
		targetY = 7
	} else if p.facing.y > 0.1 {
		targetY = 11
	}
	aim := vec{goalX - p.pos.x, targetY - p.pos.y}.norm()
	g.release(i, aim.mul(shotSpeed), ballShot)
	p.nextAction = g.now + actionCooldown
	g.passTarget = noPlayer
}

func (g *Game) tackle(i int) {
	p := &g.players[i]
	if g.now < p.nextTackle || g.ball.owner == noPlayer {
		return
	}
	owner := g.ball.owner
	if owner == i || g.players[owner].team == p.team || g.now < g.ball.protectedTil {
		return
	}
	d := g.players[owner].pos.sub(p.pos)
	rangeLimit := humanTackleRange
	cooldown := humanTackleCooldown
	if i != g.active {
		cfg := g.difficulty.config()
		rangeLimit = cfg.botTackleRange
		cooldown = cfg.botTackleCooldown
	}
	if d.len() > rangeLimit {
		return
	}
	f := p.facing.norm()
	if f.len() == 0 || dot(d.norm(), f) < 0.15 {
		return
	}
	g.ball.owner = noPlayer
	g.ball.mode = ballFree
	g.ball.pos = g.players[owner].pos
	away := d.norm()
	g.ball.vel = away.mul(5)
	g.ball.lastTouch = i
	g.ball.releasedBy = owner
	g.ball.reclaimAfter = g.now + reclaimGrace
	g.ball.protectedTil = g.now + g.difficulty.config().ownerProtection
	p.nextTackle = g.now + cooldown
	if g.players[owner].team == awayTeam {
		g.active = i
	}
}

func (g *Game) release(i int, velocity vec, mode ballMode) {
	g.ball.owner = noPlayer
	g.ball.mode = mode
	g.ball.pos = g.players[i].pos
	g.ball.vel = velocity
	g.ball.lastTouch = i
	g.ball.releasedBy = i
	g.ball.reclaimAfter = g.now + reclaimGrace
	g.ball.protectedTil = g.now + g.difficulty.config().ownerProtection
}

func (g *Game) acquire(i int) bool {
	if g.ball.owner != noPlayer {
		return false
	}
	if i == g.ball.releasedBy && g.now < g.ball.reclaimAfter {
		return false
	}
	if g.now < g.ball.protectedTil && g.ball.lastTouch != noPlayer && g.players[g.ball.lastTouch].team != g.players[i].team {
		return false
	}
	p := &g.players[i]
	if g.ball.mode == ballShot && g.ball.vel.len() > 10 && p.role != goalkeeper {
		// deterministic block: reflect away from the defender and halve speed.
		n := g.ball.pos.sub(p.pos).norm()
		if n.len() == 0 {
			n = p.facing.mul(-1).norm()
		}
		g.ball.vel = n.mul(g.ball.vel.len() * 0.5)
		g.ball.mode = ballFree
		g.ball.lastTouch = i
		return false
	}
	g.ball.owner = i
	g.ball.vel = vec{}
	g.ball.pos = p.pos
	g.ball.lastTouch = i
	g.ball.releasedBy = noPlayer
	if p.role == goalkeeper {
		g.ball.mode = ballKeeper
		g.ball.holdUntil = g.now + keeperHoldTime
	} else {
		g.ball.mode = ballCarried
	}
	if p.team == homeTeam && p.role != goalkeeper {
		g.active = i
		cfg := g.difficulty.config()
		g.ball.protectedTil = g.now + cfg.ownerProtection
		g.pressAllowedAt = g.now + cfg.pressDelay
	} else if p.team == awayTeam {
		g.active = g.nearestHome(g.ball.pos, noPlayer)
	}
	return true
}

func (g *Game) tryAcquireOnSegment(i int, a, b vec) {
	if segmentDistance(g.ball.pos, a, b) <= interactRadius {
		g.acquire(i)
	}
}

func (g *Game) advanceBall(dt time.Duration) {
	if g.ball.owner != noPlayer {
		g.ball.pos = g.players[g.ball.owner].pos
		return
	}
	if g.ball.vel.len() == 0 {
		g.pickupStationary()
		return
	}
	seconds := dt.Seconds()
	start := g.ball.pos
	end := start.add(g.ball.vel.mul(seconds))

	boundT, boundKind := firstBoundary(start, end)
	hitPlayer, hitT := g.firstPlayerContact(start, end)
	// A boundary crossing wins an exact tie. This makes a goal-line contact at
	// the same instant as crossing unambiguous and prevents late saves.
	if boundKind != 0 && boundT <= hitT {
		cross := start.add(end.sub(start).mul(boundT))
		g.ball.pos = cross
		g.handleBoundary(boundKind, cross)
		return
	}
	if hitPlayer != noPlayer {
		g.ball.pos = start.add(end.sub(start).mul(hitT))
		if g.acquire(hitPlayer) {
			return
		}
		start = g.ball.pos
		end = start.add(g.ball.vel.mul(seconds * (1 - hitT)))
	}
	g.ball.pos = end
	if g.ball.mode == ballFree || g.ball.mode == ballPass || g.ball.mode == ballLob {
		speed := g.ball.vel.len()
		speed = math.Max(0, speed-freeDecel*seconds)
		if speed == 0 {
			g.ball.vel = vec{}
			g.ball.mode = ballFree
		} else {
			g.ball.vel = g.ball.vel.norm().mul(speed)
		}
	}
	g.pickupStationary()
}

func firstBoundary(a, b vec) (float64, int) {
	best := math.Inf(1)
	kind := 0 // 1 left, 2 right, 3 top, 4 bottom
	check := func(t float64, k int) {
		if t >= 0 && t <= 1 && (t < best || (t == best && k < kind)) {
			best, kind = t, k
		}
	}
	if b.x < 0 && b.x != a.x {
		check((0-a.x)/(b.x-a.x), 1)
	}
	if b.x > PitchW && b.x != a.x {
		check((PitchW-a.x)/(b.x-a.x), 2)
	}
	if b.y < 0 && b.y != a.y {
		check((0-a.y)/(b.y-a.y), 3)
	}
	if b.y > PitchH && b.y != a.y {
		check((PitchH-a.y)/(b.y-a.y), 4)
	}
	return best, kind
}

func (g *Game) firstPlayerContact(a, b vec) (int, float64) {
	bestID, bestT := noPlayer, math.Inf(1)
	for i := range g.players {
		if i == g.ball.releasedBy && g.now < g.ball.reclaimAfter {
			continue
		}
		if g.ball.mode == ballLob && i != g.passTarget && g.players[i].role != goalkeeper {
			continue
		}
		t := segmentContactT(g.players[i].pos, a, b, interactRadius)
		if t >= 0 && t < bestT || (t == bestT && i < bestID) {
			bestID, bestT = i, t
		}
	}
	return bestID, bestT
}

func segmentContactT(p, a, b vec, radius float64) float64 {
	d := b.sub(a)
	dd := dot(d, d)
	if dd == 0 {
		if dist(p, a) <= radius {
			return 0
		}
		return math.Inf(1)
	}
	t := clamp(dot(p.sub(a), d)/dd, 0, 1)
	if dist(p, a.add(d.mul(t))) <= radius {
		return t
	}
	return math.Inf(1)
}

func segmentDistance(p, a, b vec) float64 {
	t := segmentContactT(p, a, b, math.Inf(1))
	if math.IsInf(t, 1) {
		return dist(p, a)
	}
	return dist(p, a.add(b.sub(a).mul(t)))
}

func (g *Game) pickupStationary() {
	if g.ball.owner != noPlayer {
		return
	}
	best := noPlayer
	bestD := math.Inf(1)
	for i := range g.players {
		d := dist(g.players[i].pos, g.ball.pos)
		if d <= interactRadius && (d < bestD || (d == bestD && i < best)) {
			best, bestD = i, d
		}
	}
	if best != noPlayer {
		g.acquire(best)
	}
}

func (g *Game) handleBoundary(kind int, at vec) {
	if kind == 1 || kind == 2 {
		if at.y >= 6 && at.y < 12 {
			if kind == 2 {
				g.homeGoals++
				g.kickoffTeam = awayTeam
			} else {
				g.awayGoals++
				g.kickoffTeam = homeTeam
			}
			g.phase = phaseGoal
			g.phaseLeft = goalOverlayTime
			g.ball.vel = vec{}
			g.ball.owner = noPlayer
			return
		}
		// End line outside the goal.
		right := kind == 2
		attacking := homeTeam
		if !right {
			attacking = awayTeam
		}
		lastTeam := awayTeam
		if g.ball.lastTouch != noPlayer {
			lastTeam = g.players[g.ball.lastTouch].team
		}
		if lastTeam == attacking {
			g.beginRestart(1-attacking, restartGoalKick, at)
		} else {
			g.beginRestart(attacking, restartCorner, at)
		}
		return
	}
	lastTeam := awayTeam
	if g.ball.lastTouch != noPlayer {
		lastTeam = g.players[g.ball.lastTouch].team
	}
	g.beginRestart(1-lastTeam, restartThrow, at)
}

func (g *Game) beginRestart(team int, kind restartKind, at vec) {
	g.phase = phaseRestart
	g.phaseLeft = restartTime
	g.restartTeam = team
	g.restartKind = kind
	g.restartSpot = vec{clamp(at.x, 1, PitchW-1), clamp(at.y, 1, PitchH-1)}
	g.ball.owner = noPlayer
	g.ball.vel = vec{}
	g.ball.pos = g.restartSpot
	g.restartTaker = g.nearestTeamTo(team, g.restartSpot, 255)
}

func (g *Game) executeRestart() {
	taker := g.restartTaker
	if taker == noPlayer {
		taker = 1
		if g.restartTeam == awayTeam {
			taker = 6
		}
	}
	g.players[taker].pos = g.restartSpot
	g.ball = ballState{pos: g.restartSpot, owner: taker, lastTouch: taker, releasedBy: noPlayer, mode: ballCarried}
	g.phase = phaseLive
	g.pass(taker, false)
}

func (g *Game) nearestHome(v vec, exclude int) int {
	best, bestD := noPlayer, math.Inf(1)
	for i := 1; i <= 4; i++ {
		if i == exclude {
			continue
		}
		d := dist(g.players[i].pos, v)
		if d < bestD || (d == bestD && i < best) {
			best, bestD = i, d
		}
	}
	return best
}

func (g *Game) nearestTeamTo(team int, v vec, excludeRole role) int {
	best, bestD := noPlayer, math.Inf(1)
	for i := range g.players {
		if g.players[i].team != team || g.players[i].role == excludeRole {
			continue
		}
		d := dist(g.players[i].pos, v)
		if d < bestD || (d == bestD && i < best) {
			best, bestD = i, d
		}
	}
	return best
}

func (g *Game) nearestOpponentDistance(i int) float64 {
	best := math.Inf(1)
	for j := range g.players {
		if g.players[j].team != g.players[i].team {
			best = math.Min(best, dist(g.players[i].pos, g.players[j].pos))
		}
	}
	return best
}

func (g *Game) Render(c engine.Canvas) {
	w, h := c.Size()
	if g.phase == phaseDifficulty {
		g.renderDifficulty(c, w, h)
		return
	}

	const frameW = 78
	const pitchFrameW = 74
	x0 := max(0, (w-frameW)/2)
	y0 := max(0, (h-1-22)/2)
	ox := x0 + (frameW-pitchFrameW)/2
	oy := y0 + 2

	poss := "FREE"
	if g.ball.owner != noPlayer {
		if g.players[g.ball.owner].team == homeTeam {
			poss = "HOME"
		} else {
			poss = "AWAY"
		}
	}
	poss = translate(c, poss)

	c.Text(x0, y0, engine.Format(c, "> TERMINAL FC   HOME %d-%d AWAY   Time %s   %s",
		g.homeGoals, g.awayGoals, formatTime(g.liveLeft), translate(c, g.phaseText())), engine.Accent)
	c.Text(x0, y0+1, engine.Format(c, "H%d %s   Arrows   A lob/tackle   S pass/press   D shoot   W switch   Pause: Space", g.active+1, poss), engine.Muted)

	gameui.Box(c, ox, oy, pitchFrameW, PitchH+2, engine.Border)
	gameui.DotGrid(c, ox, oy, pitchFrameW, PitchH+2, 2)

	// Halfway line and a compact ASCII center circle make the pitch read as a
	// football field even when colors are unavailable.
	midX := ox + 1 + PitchW
	for y := 1; y <= PitchH; y++ {
		c.Cell(midX, oy+y, '|', engine.Border)
	}
	c.Text(midX-4, oy+7, " /---\ ", engine.Border)
	c.Text(midX-5, oy+8, "(  |  )", engine.Border)
	c.Text(midX-5, oy+9, "(  |  )", engine.Border)
	c.Text(midX-4, oy+10, " \---/ ", engine.Border)

	// Goal mouths.
	for y := 6; y < 12; y++ {
		c.Cell(ox, oy+1+y, '[', engine.Border)
		c.Cell(ox+pitchFrameW-1, oy+1+y, ']', engine.Border)
	}

	if g.passTarget != noPlayer && g.phase == phaseLive {
		p := g.players[g.passTarget]
		c.Text(ox+1+int(p.pos.x)*2, oy+1+int(p.pos.y), ">>", engine.Warning)
	}
	for i, p := range g.players {
		label := fmt.Sprintf("■%d", i+1)
		color := engine.TeamHome
		if p.team == awayTeam {
			label = fmt.Sprintf("■%d", i-4)
			color = engine.TeamAway
		}
		if i == g.active {
			label = "▣" + label[1:]
		}
		x := ox + 1 + int(clamp(p.pos.x, 0, PitchW-1))*2
		y := oy + 1 + int(clamp(p.pos.y, 0, PitchH-1))
		c.Text(x, y, label, color)
		if g.ball.owner == i {
			c.Cell(x+1, y, '*', engine.Accent)
		}
	}
	if g.ball.owner == noPlayer {
		x := ox + 1 + int(clamp(g.ball.pos.x, 0, PitchW-1))*2
		y := oy + 1 + int(clamp(g.ball.pos.y, 0, PitchH-1))
		c.Text(x, y, "● ", engine.Accent)
	}

	switch g.phase {
	case phaseKickoff:
		g.overlay(c, engine.Format(c, "KICKOFF  %.1f", math.Max(0, g.phaseLeft.Seconds())))
	case phaseGoal:
		g.overlay(c, "GOAL!")
	case phaseRestart:
		g.overlay(c, engine.Format(c, "RESTART  %.1f", math.Max(0, g.phaseLeft.Seconds())))
	case phaseFullTime:
		result := "DRAW"
		if g.homeGoals > g.awayGoals {
			result = "YOU WIN"
		} else if g.homeGoals < g.awayGoals {
			result = "YOU LOSE"
		}
		g.overlay(c, engine.Format(c, "FULL TIME  %s  %d-%d  Arcade score %d  Enter: play again", result, g.homeGoals, g.awayGoals, g.finalScore))
	}
}

func (g *Game) renderDifficulty(c engine.Canvas, w, h int) {
	center := func(y int, text string, color engine.Color) {
		x := max(0, (w-len([]rune(text)))/2)
		c.Text(x, y, text, color)
	}
	center(max(1, h/2-6), "TERMINAL FC", engine.Accent)
	center(max(2, h/2-4), translate(c, "CHOOSE DIFFICULTY"), engine.Default)

	options := []string{"EASY", "NORMAL", "HARD"}
	for i, option := range options {
		label := "  " + translate(c, option)
		if difficulty(i) == g.difficulty {
			label = "> " + translate(c, option)
		}
		center(max(3, h/2-2+i), label, engine.Default)
	}

	cfg := g.difficulty.config()
	detail := engine.Format(c, "%s  Bot %.1f cells/s  React %dms", translate(c, cfg.name), cfg.botSpeed, cfg.botInterval.Milliseconds())
	center(min(h-4, h/2+3), detail, engine.Default)
	center(min(h-3, h/2+5), translate(c, "ARROWS: SELECT   ENTER: START"), engine.Default)
	center(min(h-2, h/2+6), "Pause: Space   Q / Esc: back", engine.Default)
}

func translate(c engine.Canvas, s string) string {
	if t, ok := c.(interface{ Translate(string) string }); ok {
		return t.Translate(s)
	}
	return s
}

func (g *Game) overlay(c engine.Canvas, s string) {
	w, h := c.Size()
	x := max(0, (w-len([]rune(s)))/2)
	c.Text(x, h/2, s, engine.Warning)
}

func (g *Game) phaseText() string {
	switch g.phase {
	case phaseDifficulty:
		return "DIFFICULTY"
	case phaseKickoff:
		return "KICKOFF"
	case phaseLive:
		return "PLAYING"
	case phaseGoal:
		return "GOAL"
	case phaseRestart:
		return "RESTART"
	default:
		return "FULL TIME"
	}
}

func formatTime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	secs := int(math.Ceil(d.Seconds()))
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
