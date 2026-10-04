package terminal

import (
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/gdamore/tcell/v2"
)

// canvas adapts a tcell screen to engine.Canvas. It writes only to tcell's
// logical buffer; tcell's Show emits the cells that changed.
type canvas struct {
	screen        tcell.Screen
	width, height int
	theme         string
}

func (c canvas) Size() (int, int) { return c.width, c.height }

func (c canvas) Cell(x, y int, glyph rune, color engine.Color) {
	if x < 0 || y < 0 || x >= c.width || y >= c.height {
		return
	}
	c.screen.SetContent(x, y, engine.Printable(glyph), nil, styleOfTheme(c.theme, color))
}

// Text draws one cell per supported rune, including the UI's Latin accents.
func (c canvas) Text(x, y int, text string, color engine.Color) {
	for _, r := range text {
		c.Cell(x, y, r, color)
		x++
	}
}

func styleOf(color engine.Color) tcell.Style {
	return styleOfTheme("mono", color)
}

func styleOfTheme(theme string, color engine.Color) tcell.Style {
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	if theme == "mono" || theme == "" {
		return style.Bold(color != engine.Default)
	}
	if theme == "midnight" {
		style = style.Foreground(tcell.NewHexColor(0xd7e0ed)).Background(tcell.NewHexColor(0x101a2a))
		switch color {
		case engine.Accent:
			return style.Foreground(tcell.NewHexColor(0x88c0d0)).Bold(true)
		case engine.Player:
			return style.Foreground(tcell.NewHexColor(0xb4befe)).Bold(true)
		case engine.Warning:
			return style.Foreground(tcell.NewHexColor(0xebcb8b)).Bold(true)
		}
		return style
	}
	switch color {
	case engine.Accent:
		return style.Foreground(tcell.ColorTeal).Bold(true)
	case engine.Player:
		return style.Foreground(tcell.ColorGreen).Bold(true)
	case engine.Warning:
		return style.Foreground(tcell.ColorYellow).Bold(true)
	}
	return style
}
