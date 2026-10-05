package arcade

import (
	"context"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/spaceshooter"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"github.com/cagridursun/devcade/internal/metrics"
	"github.com/cagridursun/devcade/internal/profile"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func shooterCatalog(t *testing.T) Catalog {
	t.Helper()
	c, err := NewCatalog(Entry{ID: "spaceshooter", Name: "Space Shooter", Description: "Test the actual shooter", New: func() engine.Game { return spaceshooter.NewWithSource(rand.NewPCG(7, 19)) }})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func finishShooter(t *testing.T, a *App) int {
	t.Helper()
	f := a.game.(engine.Finisher)
	for i := 0; i < 12000 && !f.Finished(); i++ {
		a.Advance(100 * time.Millisecond)
	}
	if !f.Finished() {
		t.Fatal("real shooter did not finish")
	}
	n := a.game.(interface{ Score() int }).Score()
	if n <= 0 {
		t.Fatal("real shooter earned no points")
	}
	return n
}
func TestSpaceShooterCompletedLifecycleAndConsentCombinations(t *testing.T) {
	for _, share := range []bool{false, true} {
		for _, usage := range []bool{false, true} {
			t.Run(map[bool]string{false: "scoresOff", true: "scoresOn"}[share]+"/"+map[bool]string{false: "usageOff", true: "usageOn"}[usage], func(t *testing.T) {
				scores, err := leaderboard.Open(filepath.Join(t.TempDir(), "scores.json"))
				if err != nil {
					t.Fatal(err)
				}
				journal, err := metrics.Open(filepath.Join(t.TempDir(), "metrics.jsonl"))
				if err != nil {
					t.Fatal(err)
				}
				var scoreWrites, usageWrites atomic.Int64
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/v1/metrics/events" {
						usageWrites.Add(1)
						(&metrics.Server{Store: journal, Password: "test-password-at-least-24"}).ServeHTTP(w, r)
						return
					}
					if r.Method == http.MethodPost {
						scoreWrites.Add(1)
					}
					scores.ServeHTTP(w, r)
				}))
				defer server.Close()
				client, _ := leaderboard.NewClient(server.URL)
				mc, _ := metrics.NewClient(server.URL, "v1")
				p := profile.Default()
				p.Username = "space_runner"
				p.Share = share
				p.Metrics = usage
				disk := profile.Store{Path: filepath.Join(t.TempDir(), "profile.json")}
				a := NewAppWithOptions(shooterCatalog(t), nil, Options{Profile: p, Save: disk.Save, Client: client, Metrics: mc, InitialGame: "spaceshooter"})
				a.Resize(80, 24)
				a.Input(key(engine.KeySelect))
				// Pause and undersize exclude all suspension time from usage duration.
				a.Advance(20 * time.Millisecond)
				a.Input(key(engine.KeyPause))
				a.Advance(time.Second)
				a.Resize(40, 12)
				a.Advance(time.Second)
				a.Resize(80, 24)
				a.Input(key(engine.KeyPause))
				want := finishShooter(t, a)
				waitNetwork(t, a)
				if a.profile.Best["spaceshooter"] != want {
					t.Fatal("best missing")
				}
				saved, err := disk.Load()
				if err != nil || saved.Best["spaceshooter"] != want {
					t.Fatal("best not persisted", err)
				}
				board, err := client.Fetch(context.Background(), "spaceshooter", saved.Identity.ID)
				if err != nil {
					t.Fatal(err)
				}
				if share && (len(board.Rows) != 1 || board.Rows[0].Score != want || board.Rows[0].Username != p.Username) {
					t.Fatal("submission/fetch", board)
				}
				if !share && (len(board.Rows) != 0 || scoreWrites.Load() != 0) {
					t.Fatal("disabled score sharing wrote")
				}
				// A higher previous best survives replay of a lower, genuinely finished run.
				a.profile.Best["spaceshooter"] = want + 1
				a.persist()
				a.Input(key(engine.KeySelect))
				finishShooter(t, a)
				waitNetwork(t, a)
				if a.profile.Best["spaceshooter"] != want+1 {
					t.Fatal("lower replay replaced best")
				}
				// Enter starts a new run. Leaving it while unfinished must not submit a best.
				a.Input(key(engine.KeySelect))
				a.Advance(100 * time.Millisecond)
				a.Input(key(engine.KeyBack))
				if a.profile.Best["spaceshooter"] != want+1 {
					t.Fatal("unfinished exit recorded")
				}
				// Start one more run and close the application: distinct analytics boundary.
				a.Input(key(engine.KeySelect))
				a.Advance(100 * time.Millisecond)
				a.Close()
				if !usage && usageWrites.Load() != 0 {
					t.Fatal("disabled usage emitted requests")
				}
				if usage {
					summary := journal.Summary("client", time.Now())
					g := summary.Games["spaceshooter"]
					if g.Starts != 4 || g.Finished != 2 || g.Left != 1 || g.Closed != 1 {
						t.Fatalf("run boundaries: %+v", g)
					}
				}
			})
		}
	}
}
