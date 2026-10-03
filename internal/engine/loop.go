package engine

import (
	"fmt"
	"time"
)

const FrameDuration = time.Second / 30

type Engine struct {
	game Game
	width, height int
	paused bool
}

func New(game Game) *Engine {
	return &Engine{game: game}
}

func (e *Engine) Resize(width, height int) {
	e.width, e.height = width, height
	if e.ready() {
		e.game.Resize(width, height)
	}
}

func (e *Engine) ready() bool {
	w, h := e.game.MinimumSize()
	return e.width >= w && e.height >= h
}

// Input returns true when the application should exit, even on a tiny terminal.
func (e *Engine) Input(key Key) bool {
	if key == KeyQuit {
		return true
	}
	if !e.ready() {
		return false
	}
	if key == KeyPause {
		e.paused = !e.paused
	} else if !e.paused {
		e.game.HandleInput(key)
	}
	return false
}

func (e *Engine) Tick(dt time.Duration) {
	if !e.ready() || e.paused || dt <= 0 {
		return
	}
	// Avoid catch-up bursts after the terminal or OS has stalled.
	if dt > 100*time.Millisecond {
		dt = 100*time.Millisecond
	}
	e.game.Update(dt)
}

func (e *Engine) Render(canvas Canvas) {
	if !e.ready() {
		w, h := e.game.MinimumSize()
		canvas.Text(0, 0, "DevCade needs more room.", Cyan)
		canvas.Text(0, 1, fmt.Sprintf("Required: %dx%d | Current: %dx%d", w, h, e.width, e.height), Default)
		canvas.Text(0, 2, "Resize to continue. Q / Esc: quit", Default)
		return
	}
	e.game.Render(canvas)
	if e.paused {
		canvas.Text(2, 2, "PAUSED - Space to resume", Cyan)
	}
}
