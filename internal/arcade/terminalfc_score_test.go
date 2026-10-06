package arcade

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"github.com/cagridursun/devcade/internal/profile"
)

type terminalFCControlProbe struct {
	keys []engine.Key
}

func (g *terminalFCControlProbe) MinimumSize() (int, int)  { return 80, 24 }
func (g *terminalFCControlProbe) Start(int, int)           {}
func (g *terminalFCControlProbe) Resize(int, int)          {}
func (g *terminalFCControlProbe) HandleInput(k engine.Key) { g.keys = append(g.keys, k) }
func (g *terminalFCControlProbe) Update(time.Duration)     {}
func (g *terminalFCControlProbe) Render(engine.Canvas)     {}

func TestTerminalFCRemapsASDWWhileKeepingArrowsForMovement(t *testing.T) {
	probe := &terminalFCControlProbe{}
	c, err := NewCatalog(Entry{ID: "terminalfc", Name: "Terminal FC", Description: "test", New: func() engine.Game { return probe }})
	if err != nil {
		t.Fatal(err)
	}
	a := NewAppWithOptions(c, nil, Options{Profile: profile.Default(), InitialGame: "terminalfc"})
	defer a.Close()
	a.Resize(80, 24)
	a.Input(engine.Event{Key: engine.KeySelect})
	for _, ev := range []engine.Event{
		{Key: engine.KeyLeft},
		{Key: engine.KeyLeft, Char: 'a'},
		{Key: engine.KeyDown, Char: 's'},
		{Key: engine.KeyRight, Char: 'd'},
		{Key: engine.KeyUp, Char: 'w'},
	} {
		a.Input(ev)
	}
	want := []engine.Key{engine.KeyLeft, engine.KeyAction, engine.KeySecondary, engine.KeySelect, engine.KeyTertiary}
	if len(probe.keys) != len(want) {
		t.Fatalf("keys=%v want=%v", probe.keys, want)
	}
	for i := range want {
		if probe.keys[i] != want[i] {
			t.Fatalf("keys=%v want=%v", probe.keys, want)
		}
	}
}

func TestRealTerminalFCFinishedScoreIsSavedSubmittedAndRestarted(t *testing.T) {
	s, err := leaderboard.Open(filepath.Join(t.TempDir(), "scores.json"))
	if err != nil {
		t.Fatal(err)
	}
	h := httptest.NewServer(s)
	defer h.Close()
	client, err := leaderboard.NewClient(h.URL)
	if err != nil {
		t.Fatal(err)
	}
	p := profile.Default()
	p.Username, p.Share = "footballer", true
	store := profile.Store{Path: filepath.Join(t.TempDir(), "profile.json")}
	a := NewAppWithOptions(Builtin(), nil, Options{Profile: p, Save: store.Save, Client: client, InitialGame: "terminalfc"})
	defer a.Close()
	a.Resize(80, 24)
	a.Input(key(engine.KeySelect))
	a.Input(key(engine.KeySelect))

	finisher := a.game.(engine.Finisher)
	for i := 0; i < 4000 && !finisher.Finished(); i++ {
		a.Advance(100 * time.Millisecond)
	}
	if !finisher.Finished() {
		t.Fatal("Terminal FC match did not finish within the bounded lifecycle")
	}
	want := a.game.(interface{ Score() int }).Score()
	got, ok := a.profile.Best["terminalfc"]
	if !ok || got != want {
		t.Fatalf("completed football score not recorded: want=%d best=%+v", want, a.profile.Best)
	}
	waitNetwork(t, a)

	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	got, ok = saved.Best["terminalfc"]
	if !ok || got != want {
		t.Fatalf("football personal best not durable: want=%d saved=%+v", want, saved.Best)
	}
	b, err := client.Fetch(context.Background(), "terminalfc", saved.Identity.ID)
	if err != nil || len(b.Rows) != 1 || b.Rows[0].Score != want || b.Rows[0].Username != p.Username {
		t.Fatalf("completed football score not submitted: %+v %v", b, err)
	}

	// Observing the same completed match again must neither improve the best
	// nor create another server row. Enter starts a fresh unfinished match.
	a.observeScore()
	a.observeScore()
	a.Input(key(engine.KeySelect))
	if a.game.(engine.Finisher).Finished() || a.game.(interface{ Score() int }).Score() != 0 {
		t.Fatal("Enter did not start a fresh football match")
	}
	b, err = client.Fetch(context.Background(), "terminalfc", saved.Identity.ID)
	if err != nil || len(b.Rows) != 1 || b.Rows[0].Score != want {
		t.Fatalf("repeated observation changed leaderboard: %+v %v", b, err)
	}
}

func TestTerminalFCEarlyExitDoesNotRecordCompletedScore(t *testing.T) {
	p := profile.Default()
	a := NewAppWithOptions(Builtin(), nil, Options{Profile: p, InitialGame: "terminalfc"})
	defer a.Close()
	a.Resize(80, 24)
	a.Input(key(engine.KeySelect))
	for i := 0; i < 20; i++ {
		a.Advance(100 * time.Millisecond)
	}
	a.Input(key(engine.KeyBack))
	if _, ok := a.profile.Best["terminalfc"]; ok {
		t.Fatal("unfinished Terminal FC match recorded a personal best")
	}
}
