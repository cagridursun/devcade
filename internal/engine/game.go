// Package engine defines the backend-independent contract shared by DevCade
// games and the timing, pause and resize policy that drives them.
//
// Nothing in this package (or in any game) knows about the terminal backend:
// games receive normalized keys, elapsed time and a Canvas, and never read
// stdin, write stdout, emit escape sequences or handle process signals.
package engine

import "time"

// Key is a normalized input action. Terminals usually report key presses and
// repeats only, so there is deliberately no key-release concept.
type Key uint8

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPause
	KeySelect // confirm a choice (Enter)
	KeyBack   // leave the current screen (Q, Escape)
	KeyExit   // leave the whole application from anywhere (Ctrl+C)
)

func (k Key) String() string {
	switch k {
	case KeyUp:
		return "up"
	case KeyDown:
		return "down"
	case KeyLeft:
		return "left"
	case KeyRight:
		return "right"
	case KeyPause:
		return "pause"
	case KeySelect:
		return "select"
	case KeyBack:
		return "back"
	case KeyExit:
		return "exit"
	}
	return "none"
}

// Event is one normalized key press. Char is the lower-case ASCII letter that
// was typed, or 0, so menus can bind letters without games depending on raw
// keys. The same press may carry both, e.g. 'd' is KeyRight with Char 'd'.
type Event struct {
	Key  Key
	Char rune
}

// Color is a decorative hint. Every state must remain understandable on a
// monochrome terminal, so games must never rely on color alone.
type Color uint8

const (
	Default Color = iota
	Accent        // headers and highlighted text
	Player        // the player-controlled glyph
	Warning       // pause and size warnings
)

// Canvas is a grid of single-width cells addressed from (0, 0) at the top
// left. Drawing outside the grid is silently clipped.
//
// Cells hold printable ASCII only (0x20-0x7E). Wider or non-printable runes
// would break the one-rune-per-cell alignment that games rely on, so
// implementations replace them with '?' (see Printable).
type Canvas interface {
	Size() (width, height int)
	Cell(x, y int, glyph rune, color Color)
	Text(x, y int, text string, color Color)
}

// Printable returns r when it is printable ASCII and '?' otherwise. Canvas
// implementations use it to enforce the single-cell glyph contract.
func Printable(r rune) rune {
	if r < 0x20 || r > 0x7e {
		return '?'
	}
	return r
}

// Game is the contract between the engine and a built-in game. The engine is
// the only caller and calls every method from a single goroutine.
//
// Games receive only Key values; KeyBack and KeyExit are handled before a
// game sees input. The engine calls Start exactly once, the first time the screen meets
// MinimumSize; Start must (re)initialize all game state for that size.
// Afterwards Resize reports size changes, but only while the screen still
// meets MinimumSize. HandleInput and Update are never called while the game is
// paused or undersized. Update receives elapsed gameplay time, already capped
// by the engine (see MaxFrameStep).
type Game interface {
	MinimumSize() (width, height int)
	Start(width, height int)
	Resize(width, height int)
	HandleInput(Key)
	Update(dt time.Duration)
	Render(Canvas)
}
