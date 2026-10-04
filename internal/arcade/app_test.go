package arcade

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

// grid is a clipping test canvas that records text by row.
type grid struct {
	w, h int
	rows [][]rune
}

func newGrid(w, h int) *grid {
	g := &grid{w: w, h: h, rows: make([][]rune, h)}
	for y := range g.rows {
		g.rows[y] = []rune(strings.Repeat(" ", w))
	}
	return g
}

func (g *grid) Size() (int, int) { return g.w, g.h }
func (g *grid) Cell(x, y int, r rune, _ engine.Color) {
	if x >= 0 && y >= 0 && x < g.w && y < g.h {
		g.rows[y][x] = engine.Printable(r)
	}
}
func (g *grid) Text(x, y int, s string, c engine.Color) {
	for _, r := range s {
		g.Cell(x, y, r, c)
		x++
	}
}
func (g *grid) String() string {
	lines := make([]string, len(g.rows))
	for i, r := range g.rows {
		lines[i] = string(r)
	}
	return strings.Join(lines, "\n")
}

func render(a *App, w, h int) string {
	g := newGrid(w, h)
	a.Render(g)
	return g.String()
}

// recGame is a test-only game that records what the engine routes to it.
type recGame struct {
	id      int
	starts  []string
	resizes []string
	inputs  []engine.Key
	updates []time.Duration
}

func (*recGame) MinimumSize() (int, int)    { return 80, 24 }
func (g *recGame) Start(w, h int)           { g.starts = append(g.starts, fmt.Sprintf("%dx%d", w, h)) }
func (g *recGame) Resize(w, h int)          { g.resizes = append(g.resizes, fmt.Sprintf("%dx%d", w, h)) }
func (g *recGame) HandleInput(k engine.Key) { g.inputs = append(g.inputs, k) }
func (g *recGame) Update(dt time.Duration)  { g.updates = append(g.updates, dt) }
func (g *recGame) Render(c engine.Canvas) {
	c.Text(0, 0, fmt.Sprintf("RECGAME #%d", g.id), engine.Default)
}

// fixture is an arcade with one test-only available game in second place,
// a coming-soon entry around it, and a recording diagnostic.
type fixture struct {
	app   *App
	games []*recGame // every instance built by the available factory
	diags []*recGame // every diagnostic instance
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{}
	c, err := NewCatalog(
		Entry{ID: "soon", Name: "Soon Game", Description: "Arrives later.", Milestone: "M9"},
		Entry{ID: "ready", Name: "Ready Game", Description: "Test-only playable game.", New: func() engine.Game {
			g := &recGame{id: len(f.games) + 1}
			f.games = append(f.games, g)
			return g
		}},
		Entry{ID: "later", Name: "Later Game", Description: "Arrives even later.", Milestone: "M10"},
	)
	if err != nil {
		t.Fatal(err)
	}
	f.app = NewApp(c, func() engine.Game {
		g := &recGame{id: 100 + len(f.diags)}
		f.diags = append(f.diags, g)
		return g
	})
	t.Cleanup(f.app.Close)
	f.app.Resize(80, 24)
	return f
}

func key(k engine.Key) engine.Event { return engine.Event{Key: k} }
func char(c rune) engine.Event      { return engine.Event{Char: c} }

func (f *fixture) press(t *testing.T, evs ...engine.Event) {
	t.Helper()
	for _, ev := range evs {
		if f.app.Input(ev) {
			t.Fatalf("%+v exited the app", ev)
		}
	}
}

func TestMenuShowsCatalogWithoutConstructingGames(t *testing.T) {
	a := NewApp(Builtin(), func() engine.Game { t.Fatal("diagnostic built for rendering"); return nil })
	a.Resize(80, 24)
	out := render(a, 80, 24)
	for _, want := range []string{"DEVCADE", " > Snake        Available", "   Block Drop   Available",
		"   Maze Chase   Available", "   Blast Grid   Available",
		"Steer a growing snake", "D  Terminal diagnostic", "Enter: open", "Q / Esc: quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("menu lacks %q:\n%s", want, out)
		}
	}
	for _, line := range []string{menuFooterPlay, menuFooterInfo, activityFooter} {
		if len(line) > MenuWidth {
			t.Errorf("footer wider than %d columns: %q", MenuWidth, line)
		}
	}

	f := newFixture(t)
	render(f.app, 80, 24)
	f.app.Advance(time.Second)
	if len(f.games) != 0 || len(f.diags) != 0 {
		t.Fatal("rendering or advancing the menu built a game")
	}
}

func TestMenuSelectionWrapsAndDescribesHighlightedGame(t *testing.T) {
	f := newFixture(t)
	f.press(t, key(engine.KeyUp), key(engine.KeyUp), key(engine.KeyUp)) // cross creator and Settings to the last game
	if f.app.selected != 2 || !strings.Contains(render(f.app, 80, 24), " > Later Game") {
		t.Fatalf("Up from first selected %d", f.app.selected)
	}
	f.press(t, key(engine.KeyDown), key(engine.KeyDown), key(engine.KeyDown)) // cross Settings and creator to first
	if f.app.selected != 0 {
		t.Fatalf("Down from last selected %d", f.app.selected)
	}
	// W/S arrive as Up/Down with their letters; Left/Right and other letters do nothing.
	f.press(t, engine.Event{Key: engine.KeyDown, Char: 's'}, key(engine.KeyLeft), key(engine.KeyRight), char('x'), key(engine.KeyPause))
	if f.app.selected != 1 {
		t.Fatalf("selected %d, want 1", f.app.selected)
	}
	f.press(t, engine.Event{Key: engine.KeyUp, Char: 'w'})
	if out := render(f.app, 80, 24); f.app.selected != 0 || !strings.Contains(out, "Arrives later.") {
		t.Fatalf("selected %d:\n%s", f.app.selected, out)
	}
}

func TestEnterOnComingSoonExplainsAndStaysInMenu(t *testing.T) {
	f := newFixture(t)
	f.press(t, key(engine.KeySelect))
	out := render(f.app, 80, 24)
	if f.app.active != nil || len(f.games) != 0 || !strings.Contains(out, "Soon Game is not playable yet: it is planned for M9.") {
		t.Fatalf("active=%v games=%d:\n%s", f.app.active != nil, len(f.games), out)
	}
	f.press(t, key(engine.KeyUp))
	if strings.Contains(render(f.app, 80, 24), "not playable") {
		t.Fatal("notice not cleared by navigation")
	}
}

func TestDiagnosticLaunchAndReturnPreserveSelection(t *testing.T) {
	for _, back := range []engine.Event{{Key: engine.KeyBack, Char: 'q'}, {Key: engine.KeyBack}} {
		f := newFixture(t)
		f.press(t, key(engine.KeyDown), key(engine.KeyDown))
		// 'd' is also "right" for games; in the menu it opens the diagnostic.
		f.press(t, engine.Event{Key: engine.KeyRight, Char: 'd'})
		if f.app.active == nil || len(f.diags) != 1 || f.diags[0].starts[0] != "80x24" {
			t.Fatal("D did not start the diagnostic")
		}
		out := render(f.app, 80, 24)
		if !strings.Contains(out, "RECGAME #100") || !strings.Contains(out, "Q / Esc: back to menu") {
			t.Fatalf("activity screen:\n%s", out)
		}
		f.press(t, back)
		if f.app.active != nil || f.app.selected != 2 || !strings.Contains(render(f.app, 80, 24), " > Later Game") {
			t.Fatalf("%+v: active=%v selected=%d", back, f.app.active != nil, f.app.selected)
		}
	}
}

func TestExitAndBackDependOnState(t *testing.T) {
	f := newFixture(t)
	if !f.app.Input(key(engine.KeyExit)) || !f.app.Input(engine.Event{Key: engine.KeyBack, Char: 'q'}) || !f.app.Input(key(engine.KeyBack)) {
		t.Fatal("menu must exit on Ctrl+C, Q and Esc")
	}
	f.press(t, char('d'))
	if !f.app.Input(key(engine.KeyExit)) {
		t.Fatal("Ctrl+C must exit from an activity")
	}
	if f.app.active == nil {
		t.Fatal("Ctrl+C exit should not first return to the menu")
	}
	f.app.Resize(20, 5)
	f.press(t, key(engine.KeyBack)) // Q/Esc still leave an undersized activity
	if f.app.active != nil {
		t.Fatal("Back ignored in undersized activity")
	}
}

func TestUndersizedMenuIgnoresNavigationButQuits(t *testing.T) {
	f := newFixture(t)
	f.press(t, key(engine.KeyDown))
	f.app.Resize(79, 24)
	f.press(t, key(engine.KeyDown), key(engine.KeyUp), key(engine.KeySelect), char('d'))
	out := render(f.app, 79, 24)
	if f.app.selected != 1 || f.app.active != nil || len(f.diags)+len(f.games) != 0 {
		t.Fatalf("invisible navigation changed state: selected=%d active=%v", f.app.selected, f.app.active != nil)
	}
	if !strings.Contains(out, "Need 80x24, have 79x24") {
		t.Fatalf("warning:\n%s", out)
	}
	for _, size := range [][2]int{{0, 0}, {1, 1}, {10, 3}} {
		f.app.Resize(size[0], size[1])
		render(f.app, size[0], size[1])
	}
	f.app.Resize(120, 40)
	if out := render(f.app, 120, 40); f.app.selected != 1 || !strings.Contains(out, " > Ready Game") {
		t.Fatalf("selection lost across resize:\n%s", out)
	}
	if !f.app.Input(engine.Event{Key: engine.KeyBack, Char: 'q'}) {
		t.Fatal("Q ignored")
	}
	f.app.Resize(3, 2)
	if !f.app.Input(key(engine.KeyBack)) || !f.app.Input(key(engine.KeyExit)) {
		t.Fatal("quit ignored while undersized")
	}
}

func TestAvailableGameIsRoutedOnlyWhileActive(t *testing.T) {
	f := newFixture(t)
	f.app.Advance(time.Minute) // menu time
	f.press(t, key(engine.KeyDown), key(engine.KeySelect), key(engine.KeySelect))
	if len(f.games) != 1 || f.app.active == nil {
		t.Fatalf("launch built %d games", len(f.games))
	}
	g := f.games[0]
	f.app.Advance(time.Minute) // first frame after launch: dropped
	f.app.Advance(engine.FrameInterval)
	f.press(t, key(engine.KeyLeft), engine.Event{Key: engine.KeyRight, Char: 'd'}, key(engine.KeySelect))
	f.app.Resize(100, 30)
	if !strings.Contains(render(f.app, 100, 30), "RECGAME #1") {
		t.Fatal("render not routed")
	}
	if fmt.Sprint(g.starts, g.resizes, g.updates, g.inputs) !=
		fmt.Sprint([]string{"80x24"}, []string{"100x30"}, []time.Duration{engine.FrameInterval},
			[]engine.Key{engine.KeyLeft, engine.KeyRight, engine.KeySelect}) {
		t.Fatalf("routed: starts=%v resizes=%v updates=%v inputs=%v", g.starts, g.resizes, g.updates, g.inputs)
	}

	f.press(t, key(engine.KeyBack))
	f.app.Advance(time.Second)
	f.app.Resize(90, 30)
	f.press(t, key(engine.KeyLeft))
	if len(g.updates) != 1 || len(g.resizes) != 1 || len(g.inputs) != 3 {
		t.Fatal("inactive game still receives events")
	}
	if f.app.selected != 1 {
		t.Fatalf("selection after return = %d", f.app.selected)
	}
}

func TestRelaunchBuildsFreshUnpausedGame(t *testing.T) {
	f := newFixture(t)
	f.press(t, key(engine.KeyDown), key(engine.KeySelect), key(engine.KeySelect))
	first := f.app.active
	f.press(t, key(engine.KeyPause))
	if !first.Paused() {
		t.Fatal("pause not routed")
	}
	f.press(t, key(engine.KeyBack))
	f.app.Advance(time.Hour)
	f.press(t, key(engine.KeySelect))
	if len(f.games) != 2 || f.app.active == first || f.app.active.Paused() {
		t.Fatalf("relaunch: games=%d same=%v paused=%v", len(f.games), f.app.active == first, f.app.active.Paused())
	}
	f.app.Advance(time.Hour) // dropped: may span menu time
	f.app.Advance(engine.FrameInterval)
	if g := f.games[1]; len(g.starts) != 1 || fmt.Sprint(g.updates) != fmt.Sprint([]time.Duration{engine.FrameInterval}) {
		t.Fatalf("fresh game: starts=%v updates=%v", g.starts, g.updates)
	}
}

func TestLaunchWhileOnlyGameIsUndersizedWaitsForRoom(t *testing.T) {
	f := newFixture(t)
	f.press(t, key(engine.KeyDown), key(engine.KeySelect), key(engine.KeySelect))
	f.app.Resize(50, 10)
	f.app.Advance(time.Second)
	out := render(f.app, 50, 10)
	if !strings.Contains(out, "Need 80x24") || strings.Contains(out, "back to menu") {
		t.Fatalf("undersized activity:\n%s", out)
	}
	t.Cleanup(f.app.Close)
	f.app.Resize(80, 24)
	f.app.Advance(time.Second) // dropped: spans the undersized period
	if len(f.games[0].updates) != 0 {
		t.Fatal("undersized time replayed")
	}
}

func TestGameMenuRequiresNewGameConfirmation(t *testing.T) {
	f := newFixture(t)
	f.press(t, key(engine.KeyDown), key(engine.KeySelect))
	if f.app.state != gameMenu || f.app.active != nil || len(f.games) != 0 {
		t.Fatal("game started before New game confirmation")
	}
	if out := render(f.app, 80, 24); !strings.Contains(out, "New game") || !strings.Contains(out, "Leaderboard") {
		t.Fatal(out)
	}
	f.press(t, key(engine.KeySelect))
	if len(f.games) != 1 {
		t.Fatal("New game did not start")
	}
}

func TestActionKeyIsIgnoredByMenuAndRoutedToGames(t *testing.T) {
	f := newFixture(t)
	f.press(t, engine.Event{Key: engine.KeyAction, Char: 'z'})
	if f.app.active != nil || f.app.selected != 0 {
		t.Fatal("Z changed the menu")
	}
	f.press(t, key(engine.KeyDown), key(engine.KeySelect), key(engine.KeySelect), engine.Event{Key: engine.KeyAction, Char: 'z'})
	if g := f.games[0]; fmt.Sprint(g.inputs) != fmt.Sprint([]engine.Key{engine.KeyAction}) {
		t.Fatalf("game inputs %v", g.inputs)
	}
}
