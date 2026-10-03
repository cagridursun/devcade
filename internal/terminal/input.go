package terminal

import (
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/gdamore/tcell/v2"
)

// keyOf normalizes a tcell key event. Letters are matched case-insensitively
// so Caps Lock and Shift do not change behavior.
func keyOf(ev *tcell.EventKey) engine.Key {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC, tcell.KeyETX:
		// tcell reports Ctrl+C as KeyCtrlC; a raw 0x03 can surface as the
		// distinct ASCII code KeyETX.
		return engine.KeyQuit
	case tcell.KeyUp:
		return engine.KeyUp
	case tcell.KeyDown:
		return engine.KeyDown
	case tcell.KeyLeft:
		return engine.KeyLeft
	case tcell.KeyRight:
		return engine.KeyRight
	case tcell.KeyRune:
	default:
		return engine.KeyNone
	}
	r := ev.Rune()
	if ev.Modifiers()&tcell.ModCtrl != 0 {
		// Extended keyboard protocols may report Ctrl+C as a modified rune.
		if r == 'c' || r == 'C' {
			return engine.KeyQuit
		}
		return engine.KeyNone
	}
	switch r {
	case 'q', 'Q':
		return engine.KeyQuit
	case 'w', 'W':
		return engine.KeyUp
	case 's', 'S':
		return engine.KeyDown
	case 'a', 'A':
		return engine.KeyLeft
	case 'd', 'D':
		return engine.KeyRight
	case ' ':
		return engine.KeyPause
	}
	return engine.KeyNone
}
