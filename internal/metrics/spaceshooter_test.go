package metrics

import (
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/spaceshooter"
	"math/rand/v2"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestShooterActiveDurationFinishedRestartAndDuplicates(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	h := httptest.NewServer(&Server{Store: s, Password: "test-private-password-long"})
	defer h.Close()
	c, _ := NewClient(h.URL, "v1")
	c.Enable("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	g := Track(spaceshooter.NewWithSource(rand.NewPCG(7, 19)), "spaceshooter", c)
	e := engine.New(g)
	e.Resize(80, 24)
	e.Advance(100 * time.Millisecond)
	e.Advance(100 * time.Millisecond)
	e.Input(engine.Event{Key: engine.KeyPause})
	for range 100 {
		e.Advance(100 * time.Millisecond)
	}
	e.Resize(40, 12)
	e.Input(engine.Event{Key: engine.KeyPause})
	e.Advance(100 * time.Millisecond)
	e.Resize(80, 24)
	e.Input(engine.Event{Key: engine.KeyPause})
	e.Advance(100 * time.Millisecond)
	if g.duration != 100*time.Millisecond {
		t.Fatal("suspended time counted", g.duration)
	}
	for i := 0; i < 12000 && !g.Finished(); i++ {
		e.Advance(100 * time.Millisecond)
	}
	if !g.Finished() || g.Score() == 0 {
		t.Fatal("real run failed")
	}
	elapsed := g.duration.Milliseconds()
	g.End("left")
	g.End("closed")
	e.Input(engine.Event{Key: engine.KeySelect})
	e.Advance(100 * time.Millisecond)
	e.Advance(100 * time.Millisecond)
	g.End("left")
	c.Close()
	if len(s.events) != 4 {
		t.Fatalf("unexpected events: %+v", s.events)
	}
	end := s.events[1]
	if end.Game != "spaceshooter" || end.DurationMS != elapsed || end.Outcome != "finished" || end.Score == 0 {
		t.Fatal("end payload", end)
	}
	if s.events[3].DurationMS != 100 || s.events[2].Run == s.events[0].Run {
		t.Fatal("restart boundary", s.events)
	}
	end.Source = ""
	end.At = time.Time{}
	if err := s.Record(end, "client", time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(s.events) != 4 {
		t.Fatal("duplicate event stored")
	}
	reopened, err := Open(s.path)
	if err != nil || len(reopened.events) != 4 {
		t.Fatal("journal compatibility", err)
	}
}
