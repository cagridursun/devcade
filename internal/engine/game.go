package engine

import "time"

type Key uint8

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyPause
	KeyQuit
)

type Color uint8

const (
	Default Color = iota
	Green
	Cyan
)

// Canvas uses single-cell ASCII glyphs. Games never write terminal escape codes.
type Canvas interface {
	Size() (int, int)
	Cell(x, y int, glyph rune, color Color)
	Text(x, y int, text string, color Color)
}

// Game is the common contract for future built-in arcade games.
// Game state stays independent of the terminal backend.
type Game interface {
	MinimumSize() (int, int)
	Resize(width, height int)
	HandleInput(Key)
	Update(time.Duration)
	Render(Canvas)
	Reset()
}
