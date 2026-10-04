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

// Exercise the real factory/game rather than a stub that already implements
// Score. The missing Maze Chase accessor used to bypass the shared save path.
func TestRealMazeChaseFinishedScoreIsSavedAndSubmitted(t *testing.T) {
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
	p.Username, p.Share = "maze_runner", true
	store := profile.Store{Path: filepath.Join(t.TempDir(), "profile.json")}
	a := NewAppWithOptions(Builtin(), nil, Options{Profile: p, Save: store.Save, Client: client, InitialGame: "mazechase"})
	defer a.Close()
	a.Resize(80, 24)
	a.Input(key(engine.KeySelect))
	a.Input(key(engine.KeyLeft))
	finisher := a.game.(engine.Finisher)
	for i := 0; i < 1200 && !finisher.Finished(); i++ {
		a.Advance(100 * time.Millisecond)
	}
	if !finisher.Finished() {
		t.Fatal("deterministic Maze Chase run did not finish")
	}
	want := a.game.(interface{ Score() int }).Score()
	if want <= 0 || a.profile.Best["mazechase"] != want {
		t.Fatalf("completed points not recorded: %d %+v", want, a.profile.Best)
	}
	waitNetwork(t, a)
	saved, err := store.Load()
	if err != nil || saved.Best["mazechase"] != want {
		t.Fatal("personal best not durable", err)
	}
	b, err := client.Fetch(context.Background(), "mazechase", saved.Identity.ID)
	if err != nil || len(b.Rows) != 1 || b.Rows[0].Score != want || b.Rows[0].Username != p.Username {
		t.Fatalf("completed score not submitted: %+v %v", b, err)
	}
}
