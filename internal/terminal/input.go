package terminal

import (
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/gdamore/tcell/v2"
)

// keyOf normalizes a tcell key event. Letters are matched case-insensitively
// so Caps Lock and Shift do not change behavior.
func keyOf(ev *tcell.EventKey) engine.Event {
	switch ev.Key() {
	case tcell.KeyCtrlC, tcell.KeyETX:
		// tcell reports Ctrl+C as KeyCtrlC; a raw 0x03 can surface as the
		// distinct ASCII code KeyETX.
		return engine.Event{Key: engine.KeyExit}
	case tcell.KeyEscape:
		return engine.Event{Key: engine.KeyBack}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return engine.Event{Key: engine.KeyErase}
	case tcell.KeyEnter:
		return engine.Event{Key: engine.KeySelect}
	case tcell.KeyUp:
		return engine.Event{Key: engine.KeyUp}
	case tcell.KeyDown:
		return engine.Event{Key: engine.KeyDown}
	case tcell.KeyLeft:
		return engine.Event{Key: engine.KeyLeft}
	case tcell.KeyRight:
		return engine.Event{Key: engine.KeyRight}
	case tcell.KeyRune:
	default:
		return engine.Event{}
	}
	r := ev.Rune()
	if ev.Modifiers()&tcell.ModCtrl != 0 {
		// Extended keyboard protocols may report Ctrl+C as a modified rune.
		if r == 'c' || r == 'C' {
			return engine.Event{Key: engine.KeyExit}
		}
		return engine.Event{}
	}
	if r >= 'A' && r <= 'Z' {
		r += 'a' - 'A'
	}
	out := engine.Event{}
	if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
		out.Char = r
	}
	switch r {
	case 'q':
		out.Key = engine.KeyBack
	case 'w':
		out.Key = engine.KeyUp
	case 's':
		out.Key = engine.KeyDown
	case 'a':
		out.Key = engine.KeyLeft
	case 'd':
		out.Key = engine.KeyRight
	case 'z':
		out.Key = engine.KeyAction
	case 'x':
		out.Key = engine.KeySecondary
	case ' ':
		out.Key = engine.KeyPause
	}
	return out
}
