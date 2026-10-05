package arcade

import (
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/profile"
	"github.com/cagridursun/devcade/internal/ui"
	"strings"
	"testing"
)

type boundedCanvas struct {
	*grid
	t *testing.T
}

func (c boundedCanvas) Text(x, y int, s string, color engine.Color) {
	if x < 0 || y < 0 || y >= 24 || x+len([]rune(strings.TrimRight(s, " "))) > 80 {
		c.t.Errorf("text outside 80x24 at %d,%d: %q", x, y, s)
	}
	c.grid.Text(x, y, s, color)
}
func TestEveryGameHUDAndMenusFitAllFiveLanguages(t *testing.T) {
	for _, lang := range ui.Languages {
		t.Run(lang, func(t *testing.T) {
			p := profile.Default()
			p.Language = lang
			a := NewAppWithOptions(Builtin(), nil, Options{Profile: p})
			defer a.Close()
			a.Resize(80, 24)
			for _, state := range []screen{mainMenu, settings, username, gameMenu, scores} {
				a.state = state
				a.Render(boundedCanvas{newGrid(80, 24), t})
			}
			for i := range a.catalog.Len() {
				if !a.catalog.Entry(i).Available() {
					continue
				}
				a.selected = i
				a.activity = a.entry().ID
				a.launch(a.entry().New)
				c := boundedCanvas{newGrid(80, 24), t}
				a.Render(c)
				if !strings.Contains(c.String(), ui.Translate(lang, "PLAYING")) {
					t.Fatal("status not localized", c.String())
				}
				a.active = nil
			}
		})
	}
}
