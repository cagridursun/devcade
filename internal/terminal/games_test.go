package terminal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/arcade"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/gdamore/tcell/v2"
)

// endPrompt is shown by every game's end screen (game over or win).
const endPrompt = "Enter: play again"

// endRecipes drive each playable game to its end screen quickly through the
// normal input path. Every available catalog entry must have one.
var endRecipes = map[string]func(t *testing.T, h *harness){
	"brickbreaker": func(t *testing.T, h *harness) {
		for range 10 {
			h.screen.InjectKey(tcell.KeyLeft, 0, 0)
		}
		h.tickUntil(t, endPrompt, 20000)
	},
	"spaceshooter": func(t *testing.T, h *harness) { h.tickUntil(t, endPrompt, 20000) },
	// Heading right from the center, the snake hits the wall.
	"snake": func(t *testing.T, h *harness) { h.tickUntil(t, endPrompt, 200) },
	// Hard-dropping every piece stacks up to the top.
	"blockdrop": func(t *testing.T, h *harness) {
		for range 200 {
			// The unpause barrier consumed the preceding render. With no
			// frame ticks here, this render acknowledges this one Enter.
			h.screen.InjectKey(tcell.KeyEnter, 0, 0)
			select {
			case h.last = <-h.screen.shown:
			case <-time.After(guard):
				t.Fatal("hard drop did not render")
			}
			if strings.Contains(h.last, endPrompt) {
				return
			}
		}
		t.Fatal("Block Drop never topped out")
	},
	// Standing still, the chasers eventually take all three lives.
	"mazechase": func(t *testing.T, h *harness) { h.tickUntil(t, endPrompt, 20000) },
	// Placing a bomb and staying on it ends the run within one fuse, unless
	// a bot gets there first; either way the run ends.
	"blastgrid": func(t *testing.T, h *harness) {
		h.screen.InjectKey(tcell.KeyRune, 'z', 0)
		h.tickUntil(t, endPrompt, 2000)
	},
	// The football match ends after 180 seconds of live play plus kickoff/restarts.
	"terminalfc": func(t *testing.T, h *harness) { h.tickUntil(t, endPrompt, 3000) },
}

// tickUntil sends 100 ms frames until a rendered frame contains text, or
// fails after maxFrames. An empty text returns after the first frame. The
// frame is kept in h.last.
func (h *harness) tickUntil(t *testing.T, text string, maxFrames int) {
	t.Helper()
	for range maxFrames {
		h.tick(t, engine.MaxFrameStep)
		h.last = <-h.screen.shown // every frame publishes exactly one render
		if strings.Contains(h.last, text) {
			return
		}
	}
	t.Fatalf("no frame containing %q after %d frames; last:\n%s", text, maxFrames, h.last)
}

func TestEveryAvailableGameThroughTheArcade(t *testing.T) {
	catalog := arcade.Builtin()
	for i := range catalog.Len() {
		entry := catalog.Entry(i)
		if !entry.Available() {
			continue
		}
		end, ok := endRecipes[entry.ID]
		if !ok {
			t.Errorf("%s: add an end recipe to endRecipes", entry.ID)
			continue
		}
		t.Run(entry.ID, func(t *testing.T) {
			s := newSim(80, 24)
			h := startArcade(t, s)
			if i > 0 {
				s.waitFor(t, "GAMES") // the simulated screen takes input only after Init
				for range i {
					s.InjectKey(tcell.KeyDown, 0, 0)
				}
			}
			s.waitFor(t, " > "+entry.Name)
			s.InjectKey(tcell.KeyEnter, 0, 0)
			s.waitFor(t, "New game")
			s.InjectKey(tcell.KeyEnter, 0, 0)
			s.waitFor(t, "Q / Esc: back to game menu")

			// Pause survives shrink and enlarge.
			s.InjectKey(tcell.KeyRune, ' ', 0)
			s.waitFor(t, "PAUSED")
			s.resize(40, 12)
			s.waitFor(t, "Need 80x24, have 40x12")
			s.resize(100, 30)
			s.waitFor(t, "PAUSED")
			s.InjectKey(tcell.KeyRune, ' ', 0)
			// A tick can render before the input reader delivers Space.
			// Wait for unpause itself, consuming its render before drops.
			unpauseDeadline := time.After(guard)
		unpause:
			for {
				select {
				case frame := <-s.shown:
					if strings.Contains(frame, "Q / Esc: back to game menu") && !strings.Contains(frame, "PAUSED") {
						break unpause
					}
				case <-unpauseDeadline:
					t.Fatal("Space did not unpause the game")
				}
			}

			// End screen: Space must not pause it and Enter restarts.
			end(t, h)
			s.InjectKey(tcell.KeyRune, ' ', 0)
			s.InjectKey(tcell.KeyEnter, 0, 0)
			for n := 0; n == 0 || strings.Contains(h.last, endPrompt); n++ {
				if n == 50 { // the keys are queued behind at most a few frames
					t.Fatalf("Enter did not restart:\n%s", h.last)
				}
				h.tickUntil(t, "", 1)
			}
			if strings.Contains(h.last, "PAUSED") {
				t.Fatalf("restart left the game paused:\n%s", h.last)
			}

			s.InjectKey(tcell.KeyEscape, 0, 0)
			s.waitFor(t, "New game")
			s.InjectKey(tcell.KeyEscape, 0, 0)
			s.waitFor(t, " > "+entry.Name) // selection kept
			s.InjectKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
			if err := h.wait(t); err != nil {
				t.Fatal(err)
			}
			if s.inits.Load() != 1 || s.finis.Load() != 1 {
				t.Fatalf("Init=%d Fini=%d, want one screen session", s.inits.Load(), s.finis.Load())
			}
		})
		t.Run(entry.ID+"/direct", func(t *testing.T) {
			s := newSim(80, 24)
			h := start(t, context.Background(), s, entry.New())
			s.waitFor(t, "Pause: Space")
			s.InjectKey(tcell.KeyRune, 'q', 0)
			if err := h.wait(t); err != nil {
				t.Fatal(err)
			}
		})
	}
}
