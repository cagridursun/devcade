// Package engine defines the backend-independent contract shared by DevCade
// games and the timing, pause and resize policy that drives them.
//
// Nothing in this package (or in any game) knows about the terminal backend:
// games receive normalized keys, elapsed time and a Canvas, and never read
// stdin, write stdout, emit escape sequences or handle process signals.
package engine

import (
	"time"
	"unicode"
)

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
	KeySelect    // confirm a choice (Enter)
	KeyBack      // leave the current screen (Q, Escape)
	KeyExit      // leave the whole application from anywhere (Ctrl+C)
	KeyAction    // the game's primary action (Z): rotate counterclockwise, place a bomb
	KeySecondary // the game's secondary action (X), used by games that need one
	KeyTertiary  // a third game-specific action, remapped only by games that need it
	KeyErase     // Backspace in text entry
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
	case KeyAction:
		return "action"
	case KeySecondary:
		return "secondary"
	case KeyTertiary:
		return "tertiary"
	}
	return "none"
}

// Event is one normalized key press. Char is a lower-case ASCII letter, digit
// or underscore that
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
	Muted         // low-contrast grids and secondary chrome
	Border        // game-area borders and structural lines
	Danger        // hazards, enemies and destructive events
)

// Canvas is a grid of single-width cells addressed from (0, 0) at the top
// left. Drawing outside the grid is silently clipped.
//
// Cells hold ASCII, single-width Latin letters used by the five UI languages,
// and a small allowlist of terminal-safe arcade glyphs. Wider or non-printable
// runes would break the one-rune-per-cell alignment that games rely on, so
// implementations replace them with '?' (see Printable).
type Canvas interface {
	Size() (width, height int)
	Cell(x, y int, glyph rune, color Color)
	Text(x, y int, text string, color Color)
}

// Printable accepts ASCII, precomposed Latin letters and a deliberately small
// set of single-cell arcade glyphs, otherwise '?'. Canvas implementations use
// it to enforce the single-cell glyph contract.
func Printable(r rune) rune {
	if r >= 0x20 && r <= 0x7e {
		return r
	}
	if r >= 0x00a1 && r <= 0x024f && (unicode.IsLetter(r) || r == '¡' || r == '¿') {
		return r
	}
	switch r {
	case '█', '▓', '░', '■', '□', '▣', '✱', '·':
		return r
	}
	return '?'
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

// Finisher is implemented by games that can end, for example on game over or
// a completed board. While Finished reports true the engine ignores the pause
// key, so the end screen stays visible and its restart key (KeySelect) always
// reaches the game. Games that never end need not implement it.
type Finisher interface {
	Finished() bool
}

// RunStats is an optional game-specific run summary, reported with opt-in metrics.
type RunStats struct {
	Score           int   `json:"score"`
	BricksDestroyed int   `json:"bricks_destroyed"`
	LevelsCleared   int   `json:"levels_cleared"`
	HighestCombo    int   `json:"highest_combo"`
	BallsLost       int   `json:"balls_lost"`
	PlayTimeMS      int64 `json:"play_time_ms"`
}
