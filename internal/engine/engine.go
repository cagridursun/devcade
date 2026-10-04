package engine

import (
	"time"
)

// FrameInterval is the target spacing of update/render opportunities (~30 Hz).
const FrameInterval = time.Second / 30

// MaxFrameStep caps the gameplay time a single frame may deliver. After a
// stall (OS suspend, debugger, slow terminal) the game advances by at most
// this much instead of catching up and teleporting across the board.
const MaxFrameStep = 100 * time.Millisecond

// Engine owns pause and resize policy for one game. It is not safe for
// concurrent use; the terminal loop is its only caller.
type Engine struct {
	game          Game
	width, height int
	gameW, gameH  int // size last reported to the game
	started       bool
	paused        bool // user-requested pause; survives resizes
	running       bool // ready and not paused at the last state change
	discardNext   bool // drop the first frame after resuming
}

// New returns an engine for game. Nothing is called on game until the first
// Resize reports a large enough screen.
func New(game Game) *Engine {
	return &Engine{game: game}
}

// Ready reports whether the screen meets the game's minimum size.
func (e *Engine) Ready() bool {
	w, h := e.game.MinimumSize()
	return e.width >= w && e.height >= h
}

// Paused reports whether the user has paused the game.
func (e *Engine) Paused() bool { return e.paused }

// Resize records the screen size. While undersized the game is suspended but
// keeps its state; once large enough it is started or told the new size.
func (e *Engine) Resize(width, height int) {
	e.width, e.height = max(width, 0), max(height, 0)
	if e.Ready() {
		switch {
		case !e.started:
			e.started = true
			e.game.Start(e.width, e.height)
		case e.width != e.gameW || e.height != e.gameH:
			e.game.Resize(e.width, e.height)
		}
		e.gameW, e.gameH = e.width, e.height
	}
	e.updateRunning()
}

// Input applies one normalized event and reports whether the engine's owner
// should stop it: KeyBack and KeyExit end the activity in every state,
// including on a tiny screen.
func (e *Engine) Input(ev Event) (done bool) {
	switch key := ev.Key; {
	case key == KeyBack || key == KeyExit:
		return true
	case key == KeyNone || !e.Ready():
		// Undersized: the game is not visible, so ignore everything else.
	case key == KeyPause:
		if f, ok := e.game.(Finisher); ok && f.Finished() {
			break // nothing to pause on an end screen
		}
		e.paused = !e.paused
		e.updateRunning()
	case !e.paused:
		e.game.HandleInput(key)
		e.updateRunning() // input can end a run or restart it
	}
	return false
}

// Advance delivers elapsed wall time to the game. Time spent paused or
// undersized is never delivered, the first frame after resuming is dropped
// (it may span the suspended period), and each frame is capped at
// MaxFrameStep.
func (e *Engine) Advance(dt time.Duration) {
	e.updateRunning()
	if !e.running || dt <= 0 {
		return
	}
	if e.discardNext {
		e.discardNext = false
		return
	}
	e.game.Update(min(dt, MaxFrameStep))
	e.updateRunning() // a gameplay step can finish the run
}

func (e *Engine) updateRunning() {
	running := e.Ready() && !e.paused
	if f, ok := e.game.(Finisher); ok && f.Finished() {
		running = false
	}
	if running && !e.running {
		e.discardNext = true
	}
	e.running = running
}

// Render draws the game, a pause banner, or a size warning when undersized.
func (e *Engine) Render(c Canvas) {
	if !e.Ready() {
		w, h := e.game.MinimumSize()
		RenderTooSmall(c, w, h, e.width, e.height)
		return
	}
	e.game.Render(c)
	if e.paused {
		banner := Format(c, "[ PAUSED - press Space to resume ]")
		w, h := c.Size()
		c.Text((w-len([]rune(banner)))/2, h/2, banner, Warning)
	}
}

// RenderTooSmall draws the undersized-screen warning shared by every screen.
// Lines are short so they stay readable when clipped on a tiny terminal.
func RenderTooSmall(c Canvas, minW, minH, width, height int) {
	c.Text(0, 0, "DevCade: window too small", Warning)
	c.Text(0, 1, Format(c, "Need %dx%d, have %dx%d", minW, minH, width, height), Default)
	c.Text(0, 2, "Enlarge it, or press Q", Default)
}
