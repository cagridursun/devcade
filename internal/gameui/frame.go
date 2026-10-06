// Package gameui contains small, reusable rendering helpers for the shared
// DevCade in-game visual language. It deliberately stays terminal-native:
// games still render through engine.Canvas and remain readable without color.
package gameui

import (
	"strings"

	"github.com/cagridursun/devcade/internal/engine"
)

// Box draws a simple terminal-safe rectangular border.
func Box(c engine.Canvas, x, y, width, height int, color engine.Color) {
	if width < 2 || height < 2 {
		return
	}
	for px := 1; px < width-1; px++ {
		c.Cell(x+px, y, '-', color)
		c.Cell(x+px, y+height-1, '-', color)
	}
	for py := 0; py < height; py++ {
		glyph := '|'
		if py == 0 || py == height-1 {
			glyph = '+'
		}
		c.Cell(x, y+py, glyph, color)
		c.Cell(x+width-1, y+py, glyph, color)
	}
}

// DotGrid fills the inside of a box with a low-contrast arcade grid. stepX is
// normally 2 for games whose logical cells occupy two terminal columns.
func DotGrid(c engine.Canvas, x, y, width, height, stepX int) {
	if width < 3 || height < 3 {
		return
	}
	if stepX < 1 {
		stepX = 1
	}
	for py := 1; py < height-1; py++ {
		for px := 2; px < width-1; px += stepX {
			c.Cell(x+px, y+py, '.', engine.Muted)
		}
	}
}

// RightText writes text right-aligned inside a fixed-width region.
func RightText(c engine.Canvas, x, y, width int, text string, color engine.Color) {
	n := len([]rune(text))
	if n > width {
		n = width
	}
	c.Text(x+max(0, width-n), y, text, color)
}

// Meter renders a compact filled/empty terminal meter.
func Meter(value, maxValue int) string {
	if maxValue <= 0 {
		return ""
	}
	value = max(0, min(value, maxValue))
	return strings.Repeat("█", value) + strings.Repeat("□", maxValue-value)
}
