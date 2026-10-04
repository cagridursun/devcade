// Package terminal adapts the engine to a real terminal through tcell. It owns
// the backend lifecycle: initialization, input, rendering and restoration.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// ErrNotInteractive is returned when stdin or stdout is not a terminal.
var ErrNotInteractive = errors.New("an interactive terminal is required on stdin and stdout")

// PanicError reports a panic recovered from the loop after the terminal has
// been restored. The loop does not continue after a panic.
type PanicError struct {
	Value any
	Stack []byte
}

func (p *PanicError) Error() string { return fmt.Sprintf("internal error: %v", p.Value) }

// RequireTerminal fails fast when stdin or stdout is redirected. tcell opens
// the controlling terminal (/dev/tty, CONIN$/CONOUT$) directly, so without
// this check a redirected invocation could still take over the terminal.
func RequireTerminal(stdin, stdout *os.File) error {
	if !term.IsTerminal(int(stdin.Fd())) || !term.IsTerminal(int(stdout.Fd())) {
		return ErrNotInteractive
	}
	return nil
}

// Program is what the loop drives: a single engine, or the arcade menu with
// its active game. The loop is its only caller, from one goroutine.
type Program interface {
	Resize(width, height int)
	// Input applies one normalized key press and reports whether the
	// application should exit.
	Input(engine.Event) (exit bool)
	Advance(dt time.Duration)
	Render(engine.Canvas)
}

// Run drives app on the user's terminal until it exits, ctx is cancelled, or
// an error occurs. The screen is initialized once and restored before Run
// returns on every path, including panics, which are returned as *PanicError.
func Run(ctx context.Context, app Program) error {
	ticker := time.NewTicker(engine.FrameInterval)
	defer ticker.Stop()
	return run(ctx, backend{check: checkConsole, newScreen: tcell.NewScreen}, app, ticker.C)
}

// backend is the terminal seam injected by tests.
type backend struct {
	// check rejects consoles the backend cannot drive safely. It runs before
	// the screen is created and must leave the console as it found it.
	check     func() error
	newScreen func() (tcell.Screen, error)
}

// run is Run with the backend and frame clock injected for tests. Each value
// received from frames is a monotonic timestamp of one frame opportunity.
func run(ctx context.Context, b backend, app Program, frames <-chan time.Time) (err error) {
	if err := b.check(); err != nil {
		return fmt.Errorf("unsupported terminal: %w", err)
	}
	screen, err := b.newScreen()
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	if err := screen.Init(); err != nil {
		releaseAfterFailedInit(screen)
		return fmt.Errorf("initialize terminal: %w", err)
	}

	// From here on this function owns the screen and the reader.
	reader := startReader(screen.PollEvent)
	defer func() {
		p := recover()
		var stack []byte
		if p != nil {
			stack = debug.Stack()
		}
		reader.stop(screen.Fini)
		if p != nil {
			err = &PanicError{Value: p, Stack: stack}
		}
	}()

	screen.HideCursor()
	width, height := screen.Size()
	app.Resize(width, height)
	draw := func(sync bool) {
		theme := "mono"
		if provider, ok := app.(interface{ Theme() string }); ok {
			theme = provider.Theme()
		}
		if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
			theme = "mono"
		}
		baseStyle := styleOfTheme(theme, engine.Default)
		screen.SetStyle(baseStyle)
		screen.Fill(' ', baseStyle) // explicit palette for blank cells too
		app.Render(canvas{screen: screen, width: width, height: height, theme: theme})
		if sync {
			screen.Sync() // full repaint, only after resize
		} else {
			screen.Show() // emits changed cells only
		}
	}
	draw(true)

	// handle applies one event and reports whether the loop should stop.
	var resized bool
	handle := func(ev tcell.Event) (bool, error) {
		switch ev := ev.(type) {
		case *tcell.EventKey:
			return app.Input(keyOf(ev)), nil
		case *tcell.EventResize:
			width, height = screen.Size()
			app.Resize(width, height)
			resized = true
		case *tcell.EventError:
			return true, fmt.Errorf("terminal input: %w", ev)
		}
		return false, nil
	}

	var last time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-frames:
			if !last.IsZero() {
				app.Advance(now.Sub(last))
			}
			last = now
			draw(false)
		case ev := <-reader.events:
			// Apply what is already queued (bounded, so a flood cannot
			// starve frames), then render once.
			stop, err := handle(ev)
			for n := 1; n < eventBuffer && !stop && err == nil; n++ {
				select {
				case ev := <-reader.events:
					stop, err = handle(ev)
					continue
				default:
				}
				break
			}
			if stop || err != nil {
				return err
			}
			draw(resized)
			resized = false
		}
	}
}

// releaseAfterFailedInit releases whatever a failed Init acquired. This is
// verified for tcell v2.13.10's terminfo screen, the backend NewScreen picks
// on every supported platform (Windows included): it can fail after opening
// the tty (charset or raw-mode setup), and Fini then closes it, but Fini
// panics if Init failed before allocating its state. It is NOT safe for the
// legacy Windows cScreen, whose Init can fail with its mutex held so that Fini
// blocks forever; checkConsole rejects that VT failure before Init runs.
func releaseAfterFailedInit(screen tcell.Screen) {
	defer func() { _ = recover() }()
	screen.Fini()
}
