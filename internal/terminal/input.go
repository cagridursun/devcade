package terminal

import (
	"unicode"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/gdamore/tcell/v2"
)

func keyOf(event *tcell.EventKey) engine.Key {
	switch event.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
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
		switch unicode.ToLower(event.Rune()) {
		case 'q':
			return engine.KeyQuit
		case 'w':
			return engine.KeyUp
		case 's':
			return engine.KeyDown
		case 'a':
			return engine.KeyLeft
		case 'd':
			return engine.KeyRight
		case ' ':
			return engine.KeyPause
		}
	}
	return engine.KeyNone
}
