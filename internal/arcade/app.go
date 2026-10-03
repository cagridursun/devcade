package arcade

import (
	"fmt"
	"strings"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

// Minimum screen size for the menu, matching the M1 core.
const (
	MenuWidth  = 80
	MenuHeight = 24
)

// Footers name only actions that work in the current context: Enter offers
// to play only when the highlighted game is playable.
const (
	menuFooterPlay = " Up/Down or W/S: select   Enter: play   D: diagnostic   Q / Esc: quit"
	menuFooterInfo = " Up/Down or W/S: select   Enter: details   D: diagnostic   Q / Esc: quit"
	activityFooter = " Q / Esc: back to menu   Ctrl+C: quit DevCade"
)

// App is the interactive arcade: a menu over the catalog and, at most, one
// active game or tool. It has two states, menu (active == nil) and active.
// Leaving an activity discards it; the next launch builds a fresh one.
type App struct {
	catalog    Catalog
	diagnostic func() engine.Game

	width, height int
	selected      int
	notice        string         // feedback for the last menu action
	active        *engine.Engine // nil while the menu is shown
}

// NewApp returns an arcade that starts in the menu. diagnostic builds the
// terminal diagnostic launched with D.
func NewApp(catalog Catalog, diagnostic func() engine.Game) *App {
	return &App{catalog: catalog, diagnostic: diagnostic}
}

func (a *App) menuReady() bool { return a.width >= MenuWidth && a.height >= MenuHeight }

// Resize records the screen size and forwards it to the active engine. The
// menu selection is kept across any size change.
func (a *App) Resize(width, height int) {
	a.width, a.height = max(width, 0), max(height, 0)
	if a.active != nil {
		a.active.Resize(a.width, a.height)
	}
}

// Input applies one key press and reports whether DevCade should exit.
// Ctrl+C exits from anywhere; Q/Esc leave the activity, or exit from the menu.
func (a *App) Input(ev engine.Event) (exit bool) {
	if ev.Key == engine.KeyExit {
		return true
	}
	if a.active != nil {
		if ev.Key == engine.KeyBack {
			a.active = nil // back to the menu; the game is discarded
			return false
		}
		a.active.Input(ev)
		return false
	}
	if ev.Key == engine.KeyBack {
		return true
	}
	if !a.menuReady() {
		return false // the menu is not visible: ignore navigation
	}
	if ev.Char == 'd' {
		a.launch(a.diagnostic)
		return false
	}
	n := a.catalog.Len()
	switch ev.Key {
	case engine.KeyUp:
		a.selected = (a.selected + n - 1) % n
		a.notice = ""
	case engine.KeyDown:
		a.selected = (a.selected + 1) % n
		a.notice = ""
	case engine.KeySelect:
		e := a.catalog.Entry(a.selected)
		if !e.Available() {
			a.notice = fmt.Sprintf("%s is not playable yet: it is planned for %s.", e.Name, e.Milestone)
			return false
		}
		a.launch(e.New)
	}
	return false
}

// launch starts a fresh game in a new engine sized to the screen. The
// engine drops its first frame, so no menu time reaches the game.
func (a *App) launch(newGame func() engine.Game) {
	a.notice = ""
	a.active = engine.New(newGame())
	a.active.Resize(a.width, a.height)
}

// Advance forwards elapsed time to the active game only.
func (a *App) Advance(dt time.Duration) {
	if a.active != nil {
		a.active.Advance(dt)
	}
}

// Render draws the active game with a navigation footer, or the menu.
func (a *App) Render(c engine.Canvas) {
	if a.active != nil {
		a.active.Render(c)
		if a.active.Ready() {
			w, h := c.Size()
			c.Text(0, h-1, strings.Repeat(" ", w), engine.Default)
			c.Text(0, h-1, activityFooter, engine.Accent)
		}
		return
	}
	if !a.menuReady() {
		engine.RenderTooSmall(c, MenuWidth, MenuHeight, a.width, a.height)
		return
	}
	a.renderMenu(c)
}

func (a *App) renderMenu(c engine.Canvas) {
	w, h := c.Size()
	c.Text(1, 0, "DEVCADE >_  terminal arcade", engine.Accent)
	c.Text(1, 1, "Quick games for the wait while builds, tests or AI agents run.", engine.Default)
	c.Text(0, 2, strings.Repeat("-", w), engine.Default)

	c.Text(1, 4, "GAMES", engine.Accent)
	row := 5
	for i := range a.catalog.Len() {
		e := a.catalog.Entry(i)
		marker, color := "   ", engine.Default
		if i == a.selected {
			marker, color = " > ", engine.Player
		}
		c.Text(1, row, fmt.Sprintf("%s%-12s %s", marker, e.Name, e.Status()), color)
		row++
	}
	sel := a.catalog.Entry(a.selected)
	c.Text(4, row+1, sel.Description, engine.Default)
	if a.notice != "" {
		c.Text(4, row+2, a.notice, engine.Warning)
	}

	c.Text(1, row+4, "TOOLS", engine.Accent)
	c.Text(1, row+5, "   D  Terminal diagnostic: moving @ to check input, timing and resize", engine.Default)

	footer := menuFooterInfo
	if sel.Available() {
		footer = menuFooterPlay
	}
	c.Text(0, h-1, footer, engine.Accent)
}
