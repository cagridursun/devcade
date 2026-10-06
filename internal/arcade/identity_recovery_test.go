package arcade

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"github.com/cagridursun/devcade/internal/profile"
)

func TestRejectedIdentityCanReconnectAndSyncRecentGameBests(t *testing.T) {
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

	old, err := client.Register(context.Background(), "cagridursun")
	if err != nil {
		t.Fatal(err)
	}

	p := profile.Default()
	p.Username = "cagridursun"
	p.Share = true
	p.Identity = profile.Identity{
		ID:       old.ID,
		Token:    strings.Repeat("f", 64),
		Endpoint: client.Endpoint,
	}
	p.Best["brickbreaker"] = 420
	p.Best["terminalfc"] = 1275
	p.Best["spaceshooter"] = 860

	store := profile.Store{Path: filepath.Join(t.TempDir(), "profile.json")}
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}

	a := NewAppWithOptions(Builtin(), nil, Options{
		Profile: p,
		Save:    store.Save,
		Client:  client,
	})
	defer a.Close()
	a.Resize(80, 24)

	a.sync("terminalfc")
	waitNetwork(t, a)

	if a.profile.Identity != (profile.Identity{}) {
		t.Fatalf("rejected identity was not cleared: %+v", a.profile.Identity)
	}
	if !strings.Contains(a.networkNotice, "local bests are safe") {
		t.Fatalf("recovery notice missing: %q", a.networkNotice)
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Identity != (profile.Identity{}) {
		t.Fatalf("cleared identity was not persisted: %+v", saved.Identity)
	}
	for game, want := range p.Best {
		if saved.Best[game] != want {
			t.Fatalf("%s local best changed during recovery: got %d want %d", game, saved.Best[game], want)
		}
	}

	// A new alias creates a fresh server identity. The normal sync uploads all
	// saved personal bests, including the three most recently added games.
	a.returnTo = settings
	a.state = username
	a.nameInput = "cagridursun2"
	a.nameEvent(engine.Event{Key: engine.KeySelect})
	waitNetwork(t, a)

	saved, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Identity.Valid(client.Endpoint) {
		t.Fatalf("new online identity was not persisted: %+v", saved.Identity)
	}
	for game, want := range map[string]int{
		"brickbreaker": 420,
		"terminalfc":    1275,
		"spaceshooter":  860,
	} {
		board, err := client.Fetch(context.Background(), game, saved.Identity.ID)
		if err != nil {
			t.Fatalf("%s fetch: %v", game, err)
		}
		if board.Own == nil || board.Own.Username != "cagridursun2" || board.Own.Score != want {
			t.Fatalf("%s best was not synchronized after reconnect: %+v", game, board)
		}
	}
}
