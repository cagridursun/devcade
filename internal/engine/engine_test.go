package engine

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakeGame struct {
	starts  []string
	resizes []string
	inputs  []Key
	updates []time.Duration
}

func (*fakeGame) MinimumSize() (int, int)   { return 80, 24 }
func (g *fakeGame) Start(w, h int)          { g.starts = append(g.starts, fmt.Sprintf("%dx%d", w, h)) }
func (g *fakeGame) Resize(w, h int)         { g.resizes = append(g.resizes, fmt.Sprintf("%dx%d", w, h)) }
func (g *fakeGame) HandleInput(k Key)       { g.inputs = append(g.inputs, k) }
func (g *fakeGame) Update(dt time.Duration) { g.updates = append(g.updates, dt) }
func (*fakeGame) Render(c Canvas)           { c.Text(0, 0, "GAME", Default) }

// gridCanvas is a clipping test canvas that records text by row.
type gridCanvas struct {
	w, h int
	rows [][]rune
}

func newGrid(w, h int) *gridCanvas {
	g := &gridCanvas{w: w, h: h, rows: make([][]rune, h)}
	for y := range g.rows {
		g.rows[y] = []rune(strings.Repeat(" ", w))
	}
	return g
}

func (g *gridCanvas) Size() (int, int) { return g.w, g.h }
func (g *gridCanvas) Cell(x, y int, r rune, _ Color) {
	if x >= 0 && y >= 0 && x < g.w && y < g.h {
		g.rows[y][x] = Printable(r)
	}
}
func (g *gridCanvas) Text(x, y int, s string, c Color) {
	for _, r := range s {
		g.Cell(x, y, r, c)
		x++
	}
}
func (g *gridCanvas) String() string {
	lines := make([]string, len(g.rows))
	for i, r := range g.rows {
		lines[i] = string(r)
	}
	return strings.Join(lines, "\n")
}

const frame = FrameInterval

func TestStartOnceWhenFirstLargeEnough(t *testing.T) {
	g := &fakeGame{}
	e := New(g)
	e.Resize(40, 10)
	if len(g.starts) != 0 {
		t.Fatal("game started while undersized")
	}
	e.Resize(80, 24)
	e.Resize(80, 24) // unchanged size: no notification
	e.Resize(100, 30)
	e.Resize(10, 5) // undersized: not reported
	e.Resize(100, 30)
	if got := strings.Join(g.starts, ","); got != "80x24" {
		t.Fatalf("starts = %q, want 80x24", got)
	}
	if got := strings.Join(g.resizes, ","); got != "100x30" {
		t.Fatalf("resizes = %q, want only 100x30", got)
	}
}

func TestAdvanceCapsSlowFramesAndDropsFirstFrame(t *testing.T) {
	g := &fakeGame{}
	e := New(g)
	e.Resize(80, 24)
	e.Advance(frame) // first frame after start may span setup time
	e.Advance(frame)
	e.Advance(5 * time.Second) // OS stall
	e.Advance(0)
	e.Advance(-time.Second) // defensive: never move backwards
	want := []time.Duration{frame, MaxFrameStep}
	if fmt.Sprint(g.updates) != fmt.Sprint(want) {
		t.Fatalf("updates = %v, want %v", g.updates, want)
	}
}

func TestPauseFreezesGameplayAndResumeDoesNotCatchUp(t *testing.T) {
	g := &fakeGame{}
	e := New(g)
	e.Resize(80, 24)
	e.Advance(frame)
	e.Advance(frame)
	e.Input(KeyPause)
	if !e.Paused() {
		t.Fatal("not paused")
	}
	e.Input(KeyUp)
	e.Advance(frame)
	e.Advance(time.Minute)
	if len(g.updates) != 1 || len(g.inputs) != 0 {
		t.Fatalf("paused game advanced: updates=%v inputs=%v", g.updates, g.inputs)
	}
	e.Input(KeyPause)
	e.Advance(time.Minute) // spans the pause: dropped
	e.Advance(frame)
	e.Input(KeyLeft)
	if fmt.Sprint(g.updates) != fmt.Sprint([]time.Duration{frame, frame}) {
		t.Fatalf("updates after resume = %v", g.updates)
	}
	if len(g.inputs) != 1 || g.inputs[0] != KeyLeft {
		t.Fatalf("inputs = %v", g.inputs)
	}
}

func TestUndersizedSuspendsWithoutResetAndQuitStillWorks(t *testing.T) {
	g := &fakeGame{}
	e := New(g)
	e.Resize(80, 24)
	e.Advance(frame)
	e.Advance(frame)
	e.Resize(79, 24)
	for _, k := range []Key{KeyLeft, KeyPause, KeyNone} {
		if e.Input(k) {
			t.Fatalf("%v quit", k)
		}
	}
	e.Advance(frame)
	e.Advance(time.Hour)
	if len(g.updates) != 1 || len(g.inputs) != 0 || e.Paused() {
		t.Fatalf("undersized game changed: updates=%v inputs=%v paused=%v", g.updates, g.inputs, e.Paused())
	}
	if !e.Input(KeyQuit) {
		t.Fatal("quit ignored while undersized")
	}
	e.Resize(80, 24)
	e.Advance(time.Hour) // spans the undersized period: dropped
	e.Advance(frame)
	if len(g.starts) != 1 || len(g.updates) != 2 || g.updates[1] != frame {
		t.Fatalf("resume: starts=%v updates=%v", g.starts, g.updates)
	}
}

func TestManualPauseSurvivesResize(t *testing.T) {
	g := &fakeGame{}
	e := New(g)
	e.Resize(80, 24)
	e.Input(KeyPause)
	e.Resize(20, 5)
	e.Resize(120, 40)
	e.Advance(frame)
	e.Advance(frame)
	if !e.Paused() || len(g.updates) != 0 {
		t.Fatalf("resize discarded pause: paused=%v updates=%v", e.Paused(), g.updates)
	}
	c := newGrid(120, 40)
	e.Render(c)
	if !strings.Contains(c.String(), "PAUSED") {
		t.Fatal("pause banner missing")
	}
}

func TestQuitAlwaysWins(t *testing.T) {
	for _, size := range [][2]int{{0, 0}, {1, 1}, {80, 24}} {
		e := New(&fakeGame{})
		e.Resize(size[0], size[1])
		if !e.Input(KeyQuit) {
			t.Errorf("quit ignored at %v", size)
		}
		e.Input(KeyPause)
		if !e.Input(KeyQuit) {
			t.Errorf("quit ignored while paused at %v", size)
		}
	}
}

func TestUndersizedMessageReportsSizesAndClipsSafely(t *testing.T) {
	e := New(&fakeGame{})
	e.Resize(60, 10)
	c := newGrid(60, 10)
	e.Render(c)
	if out := c.String(); !strings.Contains(out, "Need 80x24, have 60x10") || !strings.Contains(out, "Q") {
		t.Fatalf("message:\n%s", out)
	}
	for _, size := range [][2]int{{0, 0}, {1, 1}, {5, 2}, {-3, -3}} {
		e.Resize(size[0], size[1])
		e.Render(newGrid(max(size[0], 0), max(size[1], 0)))
	}
}

func TestPrintable(t *testing.T) {
	for r, want := range map[rune]rune{'a': 'a', ' ': ' ', '~': '~', '\t': '?', 0x7f: '?', 'é': '?', '界': '?'} {
		if got := Printable(r); got != want {
			t.Errorf("Printable(%q) = %q, want %q", r, got, want)
		}
	}
}
