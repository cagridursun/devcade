package terminal

import (
	"context"
	"fmt"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/gdamore/tcell/v2"
)

func Run(ctx context.Context, app *engine.Engine) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("create terminal screen: %w", err)
	}
	return runScreen(ctx, screen, app)
}

func runScreen(ctx context.Context, screen tcell.Screen, app *engine.Engine) error {
	if err := screen.Init(); err != nil {
		return fmt.Errorf("initialize terminal (use an interactive terminal): %w", err)
	}
	defer screen.Fini()
	screen.HideCursor()
	events := make(chan tcell.Event, 32)
	quit := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		screen.ChannelEvents(events, quit)
	}()
	defer func() {
		close(quit)
		<-done
	}()
	ticker := time.NewTicker(engine.FrameDuration)
	defer ticker.Stop()
	width, height := screen.Size()
	app.Resize(width, height)
	canvas := canvas{screen: screen}
	draw := func() {
		// Clear changes only the logical buffer. Show emits cell differences;
		// it does not clear the physical terminal every frame.
		screen.Clear()
		app.Render(canvas)
		screen.Show()
	}
	draw()
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return nil
			}
			switch ev := event.(type) {
			case *tcell.EventKey:
				if app.Input(keyOf(ev)) {
					return nil
				}
			case *tcell.EventResize:
				width, height = ev.Size()
				app.Resize(width, height)
				last = time.Now()
				// A full synchronization is needed only on resize.
				screen.Clear()
				app.Render(canvas)
				screen.Sync()
			}
			draw()
		case <-ticker.C:
			now := time.Now()
			app.Tick(now.Sub(last))
			last = now
			draw()
		}
	}
}

type canvas struct { screen tcell.Screen }

func (c canvas) Size() (int, int) { return c.screen.Size() }

func (c canvas) Cell(x, y int, glyph rune, color engine.Color) {
	style := tcell.StyleDefault
	switch color {
	case engine.Green:
		style = style.Foreground(tcell.ColorGreen)
	case engine.Cyan:
		style = style.Foreground(tcell.ColorAqua)
	}
	c.screen.SetContent(x, y, glyph, nil, style)
}

func (c canvas) Text(x, y int, text string, color engine.Color) {
	for _, glyph := range text {
		c.Cell(x, y, glyph, color)
		x++
	}
}
