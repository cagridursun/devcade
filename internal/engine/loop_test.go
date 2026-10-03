package engine

import (
	"testing"
	"time"
)

type fakeGame struct {
	ticks []time.Duration
	inputs []Key
	resizes int
}

func (*fakeGame) MinimumSize() (int, int) { return 80, 24 }
func (g *fakeGame) Resize(int, int) { g.resizes++ }
func (g *fakeGame) HandleInput(k Key) { g.inputs = append(g.inputs, k) }
func (g *fakeGame) Update(dt time.Duration) { g.ticks = append(g.ticks, dt) }
func (*fakeGame) Render(Canvas) {}
func (*fakeGame) Reset() {}

func TestResizeSuspendsAndResumesWithoutUpdatingGameOnTinyScreen(t *testing.T) {
	g := &fakeGame{}
	e := New(g)
	e.Resize(80, 24)
	e.Tick(FrameDuration)
	e.Resize(40, 10)
	e.Input(KeyLeft)
	e.Tick(time.Second)
	if len(g.ticks) != 1 || len(g.inputs) != 0 || g.resizes != 1 {
		t.Fatal("small terminal changed gameplay state")
	}
	if !e.Input(KeyQuit) {
		t.Fatal("quit must work on undersized terminals")
	}
	e.Resize(100, 30)
	e.Tick(FrameDuration)
	if len(g.ticks) != 2 || g.resizes != 2 {
		t.Fatal("game did not resume after resize")
	}
}

func TestPauseAndCatchUpLimit(t *testing.T) {
	g := &fakeGame{}
	e := New(g)
	e.Resize(80, 24)
	e.Input(KeyPause)
	e.Input(KeyUp)
	e.Tick(time.Second)
	if len(g.ticks) != 0 || len(g.inputs) != 0 {
		t.Fatal("paused game advanced")
	}
	e.Resize(100, 30)
	e.Tick(FrameDuration)
	if len(g.ticks) != 0 {
		t.Fatal("resize discarded the user pause")
	}
	e.Input(KeyPause)
	e.Tick(time.Second)
	if len(g.ticks) != 1 || g.ticks[0] != 100*time.Millisecond {
		t.Fatal("elapsed time was not capped")
	}
}
