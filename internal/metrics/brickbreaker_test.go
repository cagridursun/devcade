package metrics

import (
	"encoding/json"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/brickbreaker"
	"testing"
	"time"
)

func TestBrickBreakerStatsDurableAndDeduplicated(t *testing.T) {
	s := newStore(t)
	now := time.Now()
	start, end := event(1, "run_start"), event(2, "run_end")
	start.Game = "brickbreaker"
	end.Game = "brickbreaker"
	end.Statistics = &engine.RunStats{Score: 20, BricksDestroyed: 2, LevelsCleared: 0, HighestCombo: 2, BallsLost: 1, PlayTimeMS: 2000}
	if err := s.Record(start, "client", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(end, "client", now); err != nil {
		t.Fatal(err)
	}
	// HTTP retries deserialize a fresh pointer; deduplication compares values.
	b, _ := json.Marshal(end)
	var retry Event
	json.Unmarshal(b, &retry)
	if err := s.Record(retry, "client", now); err != nil {
		t.Fatal(err)
	}
	retry.Statistics.BricksDestroyed = 3
	if err := s.Record(retry, "client", now); err == nil {
		t.Fatal("conflicting stats accepted")
	}
	again, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if again.events[1].Statistics.BricksDestroyed != 2 || again.Summary("client", now).Games["brickbreaker"].Ended != 1 {
		t.Fatal("stats lost on reopen")
	}
	end.Statistics.Score = -1
	if end.valid() {
		t.Fatal("bad summary accepted")
	}
}
func TestTrackedBrickBreakerForwardsStats(t *testing.T) {
	g := Track(brickbreaker.New(), "brickbreaker", nil)
	g.Start(80, 24)
	g.HandleInput(engine.KeySelect)
	g.Update(100 * time.Millisecond)
	stats := g.Game.(interface{ Statistics() engine.RunStats }).Statistics()
	if stats.PlayTimeMS < 99 || g.Score() != stats.Score {
		t.Fatal(stats)
	}
	g.End("left") // disabled metrics are safe
}
