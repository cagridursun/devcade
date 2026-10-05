package arcade

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/metrics"
	"github.com/cagridursun/devcade/internal/profile"
)

func TestUsageConsentMustBePersistedAndIsIndependentOfScores(t *testing.T) {
	store, err := metrics.Open(filepath.Join(t.TempDir(), "metrics.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	h := httptest.NewServer(&metrics.Server{Store: store, Password: "a-private-test-password-123456"})
	defer h.Close()
	c, _ := metrics.NewClient(h.URL, "v1")
	p := profile.Default()
	p.Share = false
	p.Metrics = false
	save := profile.Store{Path: filepath.Join(t.TempDir(), "profile.json")}
	a := NewAppWithOptions(Builtin(), nil, Options{Profile: p, Save: save.Save, Metrics: c})
	a.Resize(80, 24)
	a.Input(char('o'))
	for range 4 {
		a.Input(key(engine.KeyDown))
	}
	a.Input(key(engine.KeySelect))
	if !a.profile.Metrics || a.profile.Share || len(a.profile.MetricsID) != 32 {
		t.Fatal("consent is not independent", a.profile)
	}
	persisted, err := save.Load()
	if err != nil || !persisted.Metrics || persisted.MetricsID != a.profile.MetricsID {
		t.Fatal("consent not saved", err)
	}
	a.Input(key(engine.KeyBack))
	a.Input(key(engine.KeySelect))
	a.Input(key(engine.KeySelect))
	a.Advance(time.Millisecond)
	a.Advance(50 * time.Millisecond)
	a.Input(key(engine.KeyBack))
	a.Close()
	r := store.Summary("client", time.Now())
	if r.AppOpens != 1 || r.Games["snake"].Starts != 1 || r.Games["snake"].Left != 1 {
		t.Fatalf("%+v", r)
	}
	failed, _ := metrics.NewClient(h.URL, "v1")
	failedProfile := profile.Default()
	failedProfile.Metrics = false
	b := NewAppWithOptions(Builtin(), nil, Options{Profile: failedProfile, Save: func(profile.Profile) error { return errors.New("read-only") }, Metrics: failed})
	b.Resize(80, 24)
	b.Input(char('o'))
	b.setting = 4
	b.Input(key(engine.KeySelect))
	b.Close()
	if b.profile.Metrics {
		t.Fatal("failed consent write enabled telemetry")
	}
	if store.Summary("client", time.Now()).AppOpens != 1 {
		t.Fatal("failed consent emitted event")
	}
}
