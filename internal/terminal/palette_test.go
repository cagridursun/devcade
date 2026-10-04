package terminal

import (
	"context"
	"github.com/cagridursun/devcade/internal/arcade"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/profile"
	"github.com/gdamore/tcell/v2"
	"os"
	"testing"
)

func TestThreePalettesPreserveMonochromeDefaultAndDistinctBackgrounds(t *testing.T) {
	for _, role := range []engine.Color{engine.Default, engine.Accent, engine.Player, engine.Warning} {
		fg, bg, _ := styleOfTheme("mono", role).Decompose()
		if fg != tcell.ColorWhite || bg != tcell.ColorBlack {
			t.Fatal("monochrome default")
		}
	}
	_, night, _ := styleOfTheme("midnight", engine.Default).Decompose()
	if night == tcell.ColorBlack {
		t.Fatal("midnight background missing")
	}
	fg1, _, _ := styleOfTheme("colorful", engine.Accent).Decompose()
	fg2, _, _ := styleOfTheme("colorful", engine.Player).Decompose()
	if fg1 == fg2 {
		t.Fatal("colorful roles not distinct")
	}
}

func TestPaletteSwitchPaintsBlankCellsInTheRealLoop(t *testing.T) {
	previous, had := os.LookupEnv("NO_COLOR")
	_ = os.Unsetenv("NO_COLOR")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("NO_COLOR", previous)
		} else {
			_ = os.Unsetenv("NO_COLOR")
		}
	})
	p := profile.Default()
	p.Theme = "midnight"
	a := arcade.NewAppWithOptions(arcade.Builtin(), nil, arcade.Options{Profile: p})
	defer a.Close()
	s := newSim(80, 24)
	done := make(chan error, 1)
	go func() { done <- run(context.Background(), simBackend(s), a, nil) }()
	s.waitFor(t, "GAMES")
	check := func(theme string) {
		_, want, _ := styleOfTheme(theme, engine.Default).Decompose()
		for y := range 24 {
			for x := range 80 {
				_, _, style, _ := s.GetContent(x, y)
				_, bg, _ := style.Decompose()
				if bg != want {
					t.Fatalf("%s cell %d,%d has background %v, want %v", theme, x, y, bg, want)
				}
			}
		}
	}
	check("midnight")
	s.InjectKey(tcell.KeyRune, 'o', 0)
	s.waitFor(t, " > Language")
	s.InjectKey(tcell.KeyDown, 0, 0)
	s.waitFor(t, " > Color palette")
	s.InjectKey(tcell.KeyRight, 0, 0)
	s.waitFor(t, "Color palette: Colorful")
	check("colorful")
	s.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
