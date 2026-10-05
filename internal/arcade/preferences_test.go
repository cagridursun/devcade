package arcade

import (
	"context"
	"errors"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"github.com/cagridursun/devcade/internal/profile"
	"github.com/cagridursun/devcade/internal/ui"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFirstRunUsernameCapturesMovementQuitAndDigits(t *testing.T) {
	store := profile.Store{Path: filepath.Join(t.TempDir(), "profile.json")}
	a := NewAppWithOptions(Builtin(), nil, Options{Profile: profile.Default(), Save: store.Save, Onboard: true, InitialGame: "snake"})
	defer a.Close()
	a.Resize(80, 24)
	for _, r := range "qwasd_7" {
		k := engine.KeyNone
		if r == 'q' {
			k = engine.KeyBack
		}
		if a.Input(engine.Event{Key: k, Char: r}) {
			t.Fatal("typed name quit")
		}
	}
	a.Input(key(engine.KeyErase))
	a.Input(char('8'))
	a.Input(key(engine.KeySelect))
	if a.state != gameMenu || a.active != nil || a.profile.Username != "qwasd_8" || a.profile.Share {
		t.Fatal(a.state, a.profile)
	}
	p, err := store.Load()
	if err != nil || p.Username != "qwasd_8" {
		t.Fatal(p, err)
	}
}
func TestSettingsPersistLanguagesPalettesAndRemainReadable(t *testing.T) {
	store := profile.Store{Path: filepath.Join(t.TempDir(), "profile.json")}
	a := NewAppWithOptions(Builtin(), nil, Options{Profile: profile.Default(), Save: store.Save})
	defer a.Close()
	a.Resize(80, 24)
	a.Input(char('o'))
	for i := range 5 {
		if a.profile.Language != ui.Languages[i] {
			t.Fatal(a.profile.Language)
		}
		out := render(a, 80, 24)
		if !strings.Contains(out, ui.Translate(a.profile.Language, "Settings")) || strings.Contains(out, "?") {
			t.Fatal(out)
		}
		a.Input(key(engine.KeyRight))
	}
	a.Input(key(engine.KeyDown))
	for _, theme := range []string{"midnight", "colorful", "mono"} {
		a.Input(key(engine.KeyRight))
		if a.Theme() != theme {
			t.Fatal(a.Theme())
		}
	}
	a.Input(key(engine.KeyUp))
	a.Input(key(engine.KeyRight))
	p, err := store.Load()
	if err != nil || p.Language != "tr" || p.Theme != "mono" {
		t.Fatal(p, err)
	}
	a.Input(key(engine.KeyBack))
	out := render(a, 80, 24)
	if !strings.Contains(out, "Yeni oyunlar yakında") || !strings.Contains(out, "c__dursun") {
		t.Fatal(out)
	}
}

type scoreGame struct {
	recGame
	finished bool
	score    int
}

func (g *scoreGame) Finished() bool { return g.finished }
func (g *scoreGame) Score() int     { return g.score }
func (g *scoreGame) HandleInput(k engine.Key) {
	if k == engine.KeySelect {
		g.finished = false
	}
	if k == engine.KeyAction {
		g.finished = true
	}
}
func TestFinishedRunsSaveOnceAndRestartNeverLowersBest(t *testing.T) {
	for _, id := range []string{"snake", "blockdrop", "mazechase", "blastgrid", "brickbreaker", "spaceshooter"} {
		t.Run(id, func(t *testing.T) {
			g := &scoreGame{score: 100}
			c, _ := NewCatalog(Entry{ID: id, Name: id, Description: "Test game.", New: func() engine.Game { return g }})
			writes := 0
			a := NewAppWithOptions(c, nil, Options{Profile: profile.Default(), Save: func(p profile.Profile) error { writes++; return nil }})
			defer a.Close()
			a.Resize(80, 24)
			a.Input(key(engine.KeySelect))
			if a.active != nil {
				t.Fatal("no submenu")
			}
			a.Input(key(engine.KeySelect))
			a.Input(key(engine.KeyAction))
			for range 50 {
				a.Advance(time.Minute)
			}
			if writes != 1 || a.profile.Best[id] != 100 {
				t.Fatal(writes, a.profile.Best)
			}
			a.Input(key(engine.KeySelect))
			g.score = 20
			a.Input(key(engine.KeyAction))
			if writes != 1 {
				t.Fatal("lower best saved")
			}
			a.Input(key(engine.KeySelect))
			g.score = 200
			a.Input(key(engine.KeyAction))
			if writes != 2 || a.profile.Best[id] != 200 {
				t.Fatal(writes, a.profile.Best)
			}
			a.Input(key(engine.KeyBack))
			if a.state != gameMenu || !strings.Contains(render(a, 80, 24), "Your best: 200") {
				t.Fatal("lost game menu or best")
			}
		})
	}
}
func TestCreatorURLAndFailureFallback(t *testing.T) {
	called := make(chan string, 1)
	a := NewAppWithOptions(Builtin(), nil, Options{Profile: profile.Default(), OpenURL: func(_ context.Context, s string) error { called <- s; return errors.New("headless") }})
	defer a.Close()
	a.Resize(80, 24)
	a.selected = a.catalog.Len() + 1
	a.Input(key(engine.KeySelect))
	if s := <-called; s != TwitterURL {
		t.Fatal(s)
	}
	deadline := time.Now().Add(time.Second)
	for a.notice == "" && time.Now().Before(deadline) {
		a.Advance(0)
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(render(a, 80, 24), TwitterURL) {
		t.Fatal("no visible fallback")
	}
}
func waitNetwork(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for a.busy && time.Now().Before(deadline) {
		a.Advance(0)
		time.Sleep(time.Millisecond)
	}
	if a.busy {
		t.Fatal("network stalled")
	}
}
func TestTwoAppProfilesShareARealGlobalBoard(t *testing.T) {
	s, err := leaderboard.Open(filepath.Join(t.TempDir(), "scores.json"))
	if err != nil {
		t.Fatal(err)
	}
	h := httptest.NewServer(s)
	defer h.Close()
	client, _ := leaderboard.NewClient(h.URL)
	makeApp := func(name string, n int) *App {
		p := profile.Default()
		p.Username = name
		p.Share = true
		p.Best["snake"] = n
		store := profile.Store{Path: filepath.Join(t.TempDir(), name+".json")}
		a := NewAppWithOptions(Builtin(), nil, Options{Profile: p, Save: store.Save, Client: client})
		a.Resize(80, 24)
		t.Cleanup(a.Close)
		return a
	}
	one := makeApp("player_one", 100)
	two := makeApp("player_two", 200)
	one.sync("snake")
	waitNetwork(t, one)
	two.sync("snake")
	waitNetwork(t, two)
	one.sync("snake")
	waitNetwork(t, one)
	one.state = scores
	out := render(one, 80, 24)
	if !strings.Contains(out, "player_two") || !strings.Contains(out, "You: #2  player_one  100") {
		t.Fatal(out)
	}
	if one.profile.Identity.Token == "" {
		t.Fatal("identity not persisted")
	}
}
func TestSlowOfflineServiceDoesNotBlockMenuOrExit(t *testing.T) {
	waiting := make(chan struct{}, 1)
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { waiting <- struct{}{}; <-r.Context().Done() }))
	defer h.Close()
	c, _ := leaderboard.NewClient(h.URL)
	a := NewAppWithOptions(Builtin(), nil, Options{Profile: profile.Default(), Client: c})
	a.Resize(80, 24)
	a.state = scores
	start := time.Now()
	a.sync("snake")
	<-waiting
	a.Input(key(engine.KeyBack))
	if a.state != gameMenu || time.Since(start) > time.Second {
		t.Fatal("network blocked navigation")
	}
	a.Close()
}
