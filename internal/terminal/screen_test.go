package terminal

import (
	"context"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/probe"
	"github.com/gdamore/tcell/v2"
)

type observedScreen struct {
	tcell.SimulationScreen
	finalized bool
	ready chan struct{}
}

func (s *observedScreen) Init() error {
	if err := s.SimulationScreen.Init(); err != nil {
		return err
	}
	s.SetSize(80, 24)
	if s.ready != nil {
		close(s.ready)
	}
	return nil
}

func (s *observedScreen) Fini() {
	s.finalized = true
	s.SimulationScreen.Fini()
}

func TestCancellationRestoresScreen(t *testing.T) {
	s := &observedScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runScreen(ctx, s, engine.New(probe.New())); err != nil {
		t.Fatal(err)
	}
	if !s.finalized {
		t.Fatal("screen was not finalized on cancellation")
	}
}

type panicGame struct { *probe.Probe }

func (*panicGame) Render(engine.Canvas) { panic("test failure") }

func TestPanicRestoresScreen(t *testing.T) {
	s := &observedScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8")}
	defer func() {
		if recover() == nil {
			t.Fatal("expected render panic")
		}
		if !s.finalized {
			t.Fatal("screen was not finalized after panic")
		}
	}()
	runScreen(context.Background(), s, engine.New(&panicGame{probe.New()}))
}

func TestQuitInputEndsLoop(t *testing.T) {
	s := &observedScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), ready: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runScreen(ctx, s, engine.New(probe.New()))
	}()
	select {
	case <-s.ready:
		s.InjectKey(tcell.KeyRune, 'q', 0)
	case <-ctx.Done():
		t.Fatal("screen initialization timed out")
	}
	select {
	case err := <-done:
		if err != nil || !s.finalized || ctx.Err() != nil {
			t.Fatal("quit failed to restore screen before timeout", err)
		}
	case <-ctx.Done():
		t.Fatal("quit did not stop the event loop")
	}
}

func TestKeyMapping(t *testing.T) {
	tests := []struct {
		key tcell.Key
		r rune
		want engine.Key
	}{
		{tcell.KeyUp, 0, engine.KeyUp},
		{tcell.KeyDown, 0, engine.KeyDown},
		{tcell.KeyLeft, 0, engine.KeyLeft},
		{tcell.KeyRight, 0, engine.KeyRight},
		{tcell.KeyRune, 'W', engine.KeyUp},
		{tcell.KeyRune, 'a', engine.KeyLeft},
		{tcell.KeyRune, 's', engine.KeyDown},
		{tcell.KeyRune, 'd', engine.KeyRight},
		{tcell.KeyRune, ' ', engine.KeyPause},
		{tcell.KeyRune, 'q', engine.KeyQuit},
		{tcell.KeyEscape, 0, engine.KeyQuit},
		{tcell.KeyCtrlC, 0, engine.KeyQuit},
	}
	for _, tt := range tests {
		if got := keyOf(tcell.NewEventKey(tt.key, tt.r, 0)); got != tt.want {
			t.Errorf("key %v rune %q: got %v, want %v", tt.key, tt.r, got, tt.want)
		}
	}
}
