package terminal

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/arcade"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/probe"
	"github.com/cagridursun/devcade/internal/games/snake"
	"github.com/gdamore/tcell/v2"
)

// guard bounds how long a test waits for the loop. It only turns a hang into
// a failure; it never cancels the loop, so it cannot make a test pass.
const guard = 10 * time.Second

// simScreen is a simulation screen that publishes every rendered frame and
// records finalization.
type simScreen struct {
	tcell.SimulationScreen
	w, h      int
	finalized atomic.Bool
	inits     atomic.Int32
	finis     atomic.Int32
	shown     chan string // latest rendered frame text
}

func newSim(w, h int) *simScreen {
	return &simScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), w: w, h: h, shown: make(chan string, 1)}
}

func (s *simScreen) Init() error {
	s.inits.Add(1)
	if err := s.SimulationScreen.Init(); err != nil {
		return err
	}
	s.SetSize(s.w, s.h)
	return nil
}

func (s *simScreen) Fini() {
	s.finis.Add(1)
	s.finalized.Store(true)
	s.SimulationScreen.Fini()
}

func (s *simScreen) Show() { s.SimulationScreen.Show(); s.publish() }
func (s *simScreen) Sync() { s.SimulationScreen.Sync(); s.publish() }

func (s *simScreen) publish() {
	cells, w, _ := s.GetContents()
	var b strings.Builder
	for i, c := range cells {
		if i > 0 && i%w == 0 {
			b.WriteByte('\n')
		}
		if len(c.Bytes) == 0 {
			b.WriteByte(' ')
		} else {
			b.Write(c.Bytes)
		}
	}
	select {
	case <-s.shown:
	default:
	}
	s.shown <- b.String()
}

// resize simulates the user resizing the terminal window.
func (s *simScreen) resize(w, h int) {
	s.SetSize(w, h)
	_ = s.PostEvent(tcell.NewEventResize(w, h))
}

// waitFor blocks until a rendered frame contains text.
func (s *simScreen) waitFor(t *testing.T, text string) string {
	t.Helper()
	timeout := time.After(guard)
	for {
		select {
		case frame := <-s.shown:
			if strings.Contains(frame, text) {
				return frame
			}
		case <-timeout:
			t.Fatalf("no frame containing %q", text)
			return ""
		}
	}
}

func consoleOK() error { return nil }

func simBackend(s tcell.Screen) backend {
	return backend{check: consoleOK, newScreen: func() (tcell.Screen, error) { return s, nil }}
}

type harness struct {
	screen *simScreen
	game   *probe.Probe
	app    *engine.Engine
	frames chan time.Time
	clock  time.Time
	done   chan error
}

func start(t *testing.T, ctx context.Context, s *simScreen, game engine.Game) *harness {
	t.Helper()
	h := &harness{screen: s, app: engine.New(game), frames: make(chan time.Time), clock: time.Now(), done: make(chan error, 1)}
	h.game, _ = game.(*probe.Probe)
	go func() { h.done <- run(ctx, simBackend(s), h.app, h.frames) }()
	return h
}

// tick delivers one frame d after the previous one. The send completes only
// when the loop receives it, and the loop finishes a frame before receiving
// the next event or frame.
func (h *harness) tick(t *testing.T, d time.Duration) {
	t.Helper()
	h.clock = h.clock.Add(d)
	select {
	case h.frames <- h.clock:
	case err := <-h.done:
		t.Fatalf("loop exited early: %v", err)
	case <-time.After(guard):
		t.Fatal("loop stopped accepting frames")
	}
}

func (h *harness) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-h.done:
		if !h.screen.finalized.Load() {
			t.Error("screen not finalized")
		}
		return err
	case <-time.After(guard):
		t.Fatal("loop did not exit")
		return nil
	}
}

func TestQuitKeysEndLoopAndRestoreScreen(t *testing.T) {
	for _, key := range []struct {
		name string
		k    tcell.Key
		r    rune
		mod  tcell.ModMask
	}{
		{"q", tcell.KeyRune, 'q', 0},
		{"Q", tcell.KeyRune, 'Q', tcell.ModShift},
		{"Esc", tcell.KeyEscape, 0, 0},
		{"Ctrl+C", tcell.KeyCtrlC, 0, tcell.ModCtrl},
	} {
		t.Run(key.name, func(t *testing.T) {
			s := newSim(80, 24)
			h := start(t, context.Background(), s, probe.New())
			s.waitFor(t, "DEVCADE")
			s.InjectKey(key.k, key.r, key.mod)
			if err := h.wait(t); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCancellationEndsLoopAndRestoresScreen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := newSim(80, 24)
	h := start(t, ctx, s, probe.New())
	s.waitFor(t, "DEVCADE")
	cancel()
	if err := h.wait(t); err != nil {
		t.Fatal(err)
	}
}

func TestInputMovesProbeOnFrames(t *testing.T) {
	s := newSim(80, 24)
	h := start(t, context.Background(), s, probe.New())
	s.waitFor(t, "Heading right")
	s.InjectKey(tcell.KeyRune, 'S', tcell.ModShift)
	s.waitFor(t, "Heading down")
	h.tick(t, 0)
	h.tick(t, probe.Step)                     // first frame after start: dropped
	h.tick(t, engine.MaxFrameStep)            // 100ms accumulated
	h.tick(t, probe.Step-engine.MaxFrameStep) // 120ms: one step
	s.InjectKey(tcell.KeyRune, 'q', 0)
	if err := h.wait(t); err != nil {
		t.Fatal(err)
	}
	if x, y := h.game.Position(); x != 39 || y != 13 {
		t.Fatalf("position %d,%d, want 39,13", x, y)
	}
}

func TestUndersizedSuspendsAndResumesWithoutCatchUp(t *testing.T) {
	s := newSim(80, 24)
	h := start(t, context.Background(), s, probe.New())
	s.waitFor(t, "DEVCADE")
	h.tick(t, 0)
	h.tick(t, engine.MaxFrameStep) // dropped
	h.tick(t, engine.MaxFrameStep)
	h.tick(t, engine.MaxFrameStep) // 200ms: one step, to x=40

	s.resize(30, 6)
	s.waitFor(t, "Need 80x24, have 30x6")
	s.InjectKey(tcell.KeyLeft, 0, 0) // ignored while undersized
	for range 5 {
		h.tick(t, time.Second)
	}
	s.resize(100, 30)
	s.waitFor(t, "Screen 100x30")
	h.tick(t, time.Second) // spans the undersized period: dropped
	s.InjectKey(tcell.KeyRune, 'q', 0)
	if err := h.wait(t); err != nil {
		t.Fatal(err)
	}
	if x, y := h.game.Position(); x != 40 || y != 12 {
		t.Fatalf("position %d,%d, want 40,12", x, y)
	}
	if dx, _ := h.game.Direction(); dx != 1 {
		t.Fatal("input applied while undersized")
	}
}

func TestPauseSurvivesResize(t *testing.T) {
	s := newSim(80, 24)
	h := start(t, context.Background(), s, probe.New())
	s.waitFor(t, "DEVCADE")
	s.InjectKey(tcell.KeyRune, ' ', 0)
	s.waitFor(t, "PAUSED")
	s.resize(20, 5)
	s.waitFor(t, "Need 80x24")
	s.resize(90, 30)
	s.waitFor(t, "PAUSED")
	h.tick(t, 0)
	for range 10 {
		h.tick(t, engine.MaxFrameStep)
	}
	s.InjectKey(tcell.KeyEscape, 0, 0)
	if err := h.wait(t); err != nil {
		t.Fatal(err)
	}
	if !h.app.Paused() {
		t.Fatal("resize discarded pause")
	}
	if x, y := h.game.Position(); x != 39 || y != 12 {
		t.Fatalf("paused probe moved to %d,%d", x, y)
	}
}

func TestQuitWhileTiny(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {10, 2}, {0, 0}} {
		s := newSim(size[0], size[1])
		h := start(t, context.Background(), s, probe.New())
		h.tick(t, 0) // a frame renders at the tiny size without panicking
		s.InjectKey(tcell.KeyRune, 'q', 0)
		if err := h.wait(t); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInputFloodCannotBlockQuit(t *testing.T) {
	s := newSim(80, 24)
	h := start(t, context.Background(), s, probe.New())
	s.waitFor(t, "DEVCADE")
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = s.PostEvent(tcell.NewEventKey(tcell.KeyRune, 'w', 0))
			}
		}
	}()
	for range 3 {
		h.tick(t, engine.MaxFrameStep)
	}
	s.InjectKey(tcell.KeyRune, 'q', 0) // PostEventWait-style: blocks until queued
	if err := h.wait(t); err != nil {
		t.Fatal(err)
	}
}

// panicGame panics in the method named by where.
type panicGame struct {
	*probe.Probe
	where string
}

func (g panicGame) Start(w, h int) {
	g.maybe("Start")
	g.Probe.Start(w, h)
}
func (g panicGame) HandleInput(k engine.Key) {
	g.maybe("HandleInput")
	g.Probe.HandleInput(k)
}
func (g panicGame) Update(dt time.Duration) {
	g.maybe("Update")
	g.Probe.Update(dt)
}
func (g panicGame) Render(c engine.Canvas) {
	if g.where == "Render" {
		panic("boom in Render")
	}
	g.Probe.Render(c)
}
func (g panicGame) maybe(where string) {
	if g.where == where {
		panic("boom in " + where)
	}
}

func TestPanicRestoresScreenAndStopsLoop(t *testing.T) {
	for _, where := range []string{"Start", "Render", "HandleInput", "Update"} {
		t.Run(where, func(t *testing.T) {
			s := newSim(80, 24)
			h := start(t, context.Background(), s, panicGame{probe.New(), where})
			switch where {
			case "HandleInput", "Update":
				s.waitFor(t, "DEVCADE") // the simulated screen accepts input only after Init
			}
			switch where {
			case "HandleInput":
				s.InjectKey(tcell.KeyUp, 0, 0)
			case "Update":
				h.tick(t, 0)
				h.tick(t, engine.MaxFrameStep) // dropped
				h.tick(t, engine.MaxFrameStep)
			}
			err := h.wait(t)
			var p *PanicError
			if !errors.As(err, &p) || p.Value != "boom in "+where || !strings.Contains(string(p.Stack), "panicGame") {
				t.Fatalf("err = %v, want PanicError from %s with stack", err, where)
			}
			// The loop is gone: nothing receives frames any more.
			select {
			case h.frames <- time.Now():
				t.Fatal("loop still running after panic")
			case <-time.After(10 * time.Millisecond):
			}
		})
	}
}

// failingScreen fails Init and records cleanup.
type failingScreen struct {
	tcell.Screen
	finiPanics bool
	finis      int
}

func (s *failingScreen) Init() error { return errors.New("no tty") }
func (s *failingScreen) Fini() {
	s.finis++
	if s.finiPanics {
		panic("close of nil channel") // what tcell does when Init fails early
	}
}

func TestInitFailureReleasesAndReports(t *testing.T) {
	for _, finiPanics := range []bool{false, true} {
		s := &failingScreen{Screen: tcell.NewSimulationScreen(""), finiPanics: finiPanics}
		app := engine.New(probe.New())
		err := run(context.Background(), simBackend(s), app, nil)
		if err == nil || !strings.Contains(err.Error(), "initialize terminal: no tty") {
			t.Fatalf("err = %v", err)
		}
		if s.finis != 1 {
			t.Fatalf("Fini called %d times after failed Init", s.finis)
		}
	}
}

func TestScreenCreationFailure(t *testing.T) {
	err := run(context.Background(), backend{check: consoleOK, newScreen: func() (tcell.Screen, error) { return nil, errors.New("terminal not supported") }},
		engine.New(probe.New()), nil)
	if err == nil || !strings.Contains(err.Error(), "terminal not supported") {
		t.Fatalf("err = %v", err)
	}
}

func TestInputErrorEndsLoop(t *testing.T) {
	s := newSim(80, 24)
	h := start(t, context.Background(), s, probe.New())
	s.waitFor(t, "DEVCADE")
	_ = s.PostEvent(tcell.NewEventError(errors.New("tty closed")))
	if err := h.wait(t); err == nil || !strings.Contains(err.Error(), "tty closed") {
		t.Fatalf("err = %v", err)
	}
}

// lockingScreen models tcell v2.13.10's Windows cScreen: when VT output is
// unavailable, Init returns an error with its mutex still held, and Fini
// (via disengage) then blocks acquiring the same mutex.
type lockingScreen struct {
	tcell.Screen
	mu          sync.Mutex
	inits, fins atomic.Int32
}

func (s *lockingScreen) Init() error {
	s.inits.Add(1)
	s.mu.Lock()
	return errors.New("failed to initialize: VT output not supported?")
}

func (s *lockingScreen) Fini() {
	s.fins.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
}

// TestUnsupportedVTNeverReachesBackendInit drives the public startup path on a
// console that cannot enable VT output. Before the console check existed,
// run called Init and then Fini on this backend and blocked forever.
func TestUnsupportedVTNeverReachesBackendInit(t *testing.T) {
	console := &fakeConsole{mode: legacyMode}
	screen := &lockingScreen{Screen: tcell.NewSimulationScreen("")}
	created := false
	b := backend{
		check:     func() error { return probeVT(console.ops()) },
		newScreen: func() (tcell.Screen, error) { created = true; return screen, nil },
	}
	done := make(chan error, 1)
	go func() { done <- run(context.Background(), b, engine.New(probe.New()), nil) }()
	var err error
	select {
	case err = <-done:
	case <-time.After(guard):
		t.Fatal("startup hung on an unsupported console") // the reviewed deadlock
	}
	if !errors.Is(err, ErrNoVT) || !strings.Contains(err.Error(), "Windows Terminal") {
		t.Fatalf("err = %v, want actionable ErrNoVT", err)
	}
	if created || screen.inits.Load() != 0 || screen.fins.Load() != 0 {
		t.Fatalf("backend touched: created=%v Init=%d Fini=%d", created, screen.inits.Load(), screen.fins.Load())
	}
	if console.mode != legacyMode || console.open != 0 {
		t.Fatalf("console mode %#x (want %#x), %d handle(s) open", console.mode, legacyMode, console.open)
	}
}

// TestConsoleCheckRunsOnceBeforeNormalStartup checks that a passing console
// check leaves startup, quit and cleanup unchanged.
func TestConsoleCheckRunsOnceBeforeNormalStartup(t *testing.T) {
	console := &fakeConsole{mode: legacyMode, vt: true}
	s := newSim(80, 24)
	var order []string
	b := backend{
		check: func() error { order = append(order, "check"); return probeVT(console.ops()) },
		newScreen: func() (tcell.Screen, error) {
			order = append(order, "newScreen")
			return s, nil
		},
	}
	done := make(chan error, 1)
	go func() { done <- run(context.Background(), b, engine.New(probe.New()), nil) }()
	s.waitFor(t, "DEVCADE")
	s.InjectKey(tcell.KeyRune, 'q', 0)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(guard):
		t.Fatal("loop did not exit")
	}
	if strings.Join(order, ",") != "check,newScreen" || !s.finalized.Load() {
		t.Fatalf("order=%v finalized=%v", order, s.finalized.Load())
	}
	if console.mode != legacyMode || console.open != 0 || console.opens != 1 {
		t.Fatalf("console mode %#x, open %d, opens %d", console.mode, console.open, console.opens)
	}
}

// TestArcadeNavigationKeepsOneScreenSession drives the real loop through menu,
// diagnostic and back. The screen is initialized once and finalized once, on
// the final exit.
func TestArcadeNavigationKeepsOneScreenSession(t *testing.T) {
	s := newSim(80, 24)
	var diags []*probe.Probe
	app := arcade.NewApp(arcade.Builtin(), func() engine.Game {
		p := probe.New()
		diags = append(diags, p)
		return p
	})
	frames := make(chan time.Time)
	done := make(chan error, 1)
	go func() { done <- run(context.Background(), simBackend(s), app, frames) }()
	s.waitFor(t, " > Snake        Available")
	s.InjectKey(tcell.KeyDown, 0, 0)
	s.waitFor(t, " > Block Drop")
	s.InjectKey(tcell.KeyEnter, 0, 0)
	s.waitFor(t, "Block Drop is not playable yet: it is planned for M4.")

	s.InjectKey(tcell.KeyRune, 'd', 0)
	s.waitFor(t, "Q / Esc: back to menu")
	s.InjectKey(tcell.KeyRune, ' ', 0) // pause this instance
	s.waitFor(t, "PAUSED")
	s.InjectKey(tcell.KeyEscape, 0, 0)
	s.waitFor(t, " > Block Drop") // selection kept

	s.resize(60, 20)
	s.waitFor(t, "Need 80x24, have 60x20")
	s.InjectKey(tcell.KeyDown, 0, 0) // invisible: ignored
	s.resize(80, 24)
	s.waitFor(t, " > Block Drop")

	s.InjectKey(tcell.KeyRune, 'D', tcell.ModShift)
	if frame := s.waitFor(t, "back to menu"); strings.Contains(frame, "PAUSED") {
		t.Fatalf("relaunched diagnostic is not fresh:\n%s", frame)
	}
	s.InjectKey(tcell.KeyRune, 'q', 0)
	s.waitFor(t, " > Block Drop")
	if s.inits.Load() != 1 || s.finis.Load() != 0 {
		t.Fatalf("navigation re-initialized the terminal: Init=%d Fini=%d", s.inits.Load(), s.finis.Load())
	}
	s.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(guard):
		t.Fatal("loop did not exit")
	}
	if s.inits.Load() != 1 || s.finis.Load() != 1 || len(diags) != 2 {
		t.Fatalf("Init=%d Fini=%d diagnostics=%d", s.inits.Load(), s.finis.Load(), len(diags))
	}
}

func TestCtrlCExitsFromLaunchedActivity(t *testing.T) {
	s := newSim(80, 24)
	app := arcade.NewApp(arcade.Builtin(), func() engine.Game { return probe.New() })
	done := make(chan error, 1)
	go func() { done <- run(context.Background(), simBackend(s), app, nil) }()
	s.waitFor(t, "GAMES")
	s.InjectKey(tcell.KeyRune, 'd', 0)
	s.waitFor(t, "back to menu")
	s.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	select {
	case err := <-done:
		if err != nil || s.finis.Load() != 1 {
			t.Fatalf("err=%v Fini=%d", err, s.finis.Load())
		}
	case <-time.After(guard):
		t.Fatal("Ctrl+C did not exit from the activity")
	}
}

// startArcade runs the real loop over the built-in arcade with injected
// frames.
func startArcade(t *testing.T, s *simScreen) *harness {
	t.Helper()
	h := &harness{screen: s, frames: make(chan time.Time), clock: time.Now(), done: make(chan error, 1)}
	app := arcade.NewApp(arcade.Builtin(), func() engine.Game { return probe.New() })
	go func() { h.done <- run(context.Background(), simBackend(s), app, h.frames) }()
	return h
}

// crashSnake sends frames until the snake (heading right from the center,
// so at most 18 steps) hits the wall.
func (h *harness) crashSnake(t *testing.T) {
	t.Helper()
	h.tick(t, 0)
	for range 60 {
		h.tick(t, engine.MaxFrameStep)
	}
	h.screen.waitFor(t, "GAME OVER")
}

func TestSnakeFromMenuRestartAndReturn(t *testing.T) {
	s := newSim(80, 24)
	h := startArcade(t, s)
	s.waitFor(t, " > Snake        Available")
	s.InjectKey(tcell.KeyEnter, 0, 0)
	s.waitFor(t, "Q / Esc: back to menu")
	h.crashSnake(t)

	// Space on the end screen must not pause it, so Enter still restarts.
	s.InjectKey(tcell.KeyRune, ' ', 0)
	s.InjectKey(tcell.KeyEnter, 0, 0)
	if frame := s.waitFor(t, "Score 0     Level 1   Length 3    PLAYING"); strings.Contains(frame, "PAUSED") {
		t.Fatalf("restart blocked by pause:\n%s", frame)
	}

	s.InjectKey(tcell.KeyRune, ' ', 0)
	s.waitFor(t, "PAUSED")
	s.InjectKey(tcell.KeyEscape, 0, 0)
	s.waitFor(t, " > Snake        Available") // selection kept
	s.InjectKey(tcell.KeyEnter, 0, 0)         // fresh, unpaused run
	if frame := s.waitFor(t, "PLAYING"); strings.Contains(frame, "PAUSED") || !strings.Contains(frame, "Score 0") {
		t.Fatalf("relaunch is not fresh:\n%s", frame)
	}
	h.crashSnake(t)
	s.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl) // exit from game over
	if err := h.wait(t); err != nil {
		t.Fatal(err)
	}
	if s.inits.Load() != 1 || s.finis.Load() != 1 {
		t.Fatalf("Init=%d Fini=%d, want one screen session", s.inits.Load(), s.finis.Load())
	}
}

func TestSnakePauseSurvivesResizeWithoutCatchUp(t *testing.T) {
	s := newSim(80, 24)
	h := startArcade(t, s)
	s.waitFor(t, "GAMES")
	s.InjectKey(tcell.KeyEnter, 0, 0)
	s.waitFor(t, "PLAYING")
	s.InjectKey(tcell.KeyRune, ' ', 0)
	s.waitFor(t, "PAUSED")
	s.resize(50, 12)
	s.waitFor(t, "Need 80x24, have 50x12")
	s.resize(90, 30)
	s.waitFor(t, "PAUSED")
	h.tick(t, 0)
	for range 100 { // 10 s paused: would crash the snake if it advanced
		h.tick(t, engine.MaxFrameStep)
	}
	s.InjectKey(tcell.KeyRune, ' ', 0)
	h.tick(t, time.Minute) // first frame after resuming: dropped
	h.tick(t, engine.MaxFrameStep)
	s.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	if err := h.wait(t); err != nil {
		t.Fatal(err)
	}
	if frame := <-s.shown; strings.Contains(frame, "GAME OVER") {
		t.Fatalf("paused or resumed time was replayed:\n%s", frame)
	}
}

func TestDirectSnakeQuitsOnQ(t *testing.T) {
	s := newSim(80, 24)
	h := start(t, context.Background(), s, snake.New())
	s.waitFor(t, "SNAKE")
	s.InjectKey(tcell.KeyRune, 'q', 0)
	if err := h.wait(t); err != nil {
		t.Fatal(err)
	}
}

func TestCtrlCExitsSnakeWhilePausedOrUndersized(t *testing.T) {
	for _, name := range []string{"paused", "undersized"} {
		t.Run(name, func(t *testing.T) {
			s := newSim(80, 24)
			h := startArcade(t, s)
			s.waitFor(t, "GAMES")
			s.InjectKey(tcell.KeyEnter, 0, 0)
			s.waitFor(t, "PLAYING")
			if name == "paused" {
				s.InjectKey(tcell.KeyRune, ' ', 0)
				s.waitFor(t, "PAUSED")
			} else {
				s.resize(20, 4)
				s.waitFor(t, "Need 80x24")
			}
			s.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
			if err := h.wait(t); err != nil {
				t.Fatal(err)
			}
		})
	}
}
