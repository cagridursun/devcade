package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

const installation = "0123456789abcdef0123456789abcdef"
const runID = "11111111111111111111111111111111"

func event(n int, kind string) Event {
	e := Event{ID: fmt.Sprintf("%032x", n), Installation: installation, Kind: kind, Platform: "linux", Version: "1.1.0"}
	if strings.HasPrefix(kind, "run_") {
		e.Game = "snake"
		e.Run = runID
	}
	if kind == "run_end" {
		e.DurationMS = 2000
		e.Score = 20
		e.Outcome = "finished"
	}
	return e
}
func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "metrics.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestDurableCountsDeduplicationAndConflict(t *testing.T) {
	s := newStore(t)
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	start, end := event(1, "run_start"), event(2, "run_end")
	for _, e := range []Event{start, start, end, end} {
		if err := s.Record(e, "client", now); err != nil {
			t.Fatal(err)
		}
	}
	bad := end
	bad.Score = 30
	if err := s.Record(bad, "client", now); err == nil {
		t.Fatal("id conflict accepted")
	}
	again, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	r := again.Summary("client", now)
	g := r.Games["snake"]
	if r.DAU != 1 || r.WAU != 1 || r.MAU != 1 || g.Starts != 1 || g.Ended != 1 || g.Unfinished != 0 || g.MeanSeconds != 2 || g.MedianSeconds != 2 {
		t.Fatalf("%+v %+v", r, g)
	}
	if got := s.Summary("website", now); got.DAU != 0 || got.Visits != 0 {
		t.Fatal("sources mixed")
	}
}
func TestInvalidLifecycleAndClientOwnedTimestampsRejected(t *testing.T) {
	s := newStore(t)
	now := time.Now()
	badEvents := []Event{event(1, "run_end"), event(2, "unknown")}
	e := event(3, "app_open")
	e.Source = "client"
	badEvents = append(badEvents, e)
	e = event(4, "app_open")
	e.At = now
	badEvents = append(badEvents, e)
	e = event(8, "app_open")
	e.Platform = "browser"
	badEvents = append(badEvents, e)
	for _, e := range badEvents {
		if err := s.Record(e, "client", now); err == nil {
			t.Fatal("accepted", e)
		}
	}
	if err := s.Record(event(9, "app_open"), "simulation", now); err == nil {
		t.Fatal("unexpected source accepted")
	}
	start := event(5, "run_start")
	if err := s.Record(start, "client", now); err != nil {
		t.Fatal(err)
	}
	end := event(6, "run_end")
	end.Game = "mazechase"
	if err := s.Record(end, "client", now); err == nil {
		t.Fatal("wrong game accepted")
	}
	duplicate := event(7, "run_start")
	if err := s.Record(duplicate, "client", now); err == nil {
		t.Fatal("duplicate run start")
	}
}
func TestWriteFailureDoesNotCountAndMalformedJournalFailsClosed(t *testing.T) {
	s := newStore(t)
	if err := os.Mkdir(s.path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(event(1, "app_open"), "client", time.Now()); err == nil {
		t.Fatal("write succeeded")
	}
	if len(s.events) != 0 {
		t.Fatal("failed write counted")
	}
	p := filepath.Join(t.TempDir(), "broken.jsonl")
	if err := os.WriteFile(p, []byte(`{"unfinished":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(p); err == nil {
		t.Fatal("corrupt journal accepted")
	}
}
func TestCalendarWindowsAndStartCohort(t *testing.T) {
	s := newStore(t)
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	for i, days := range []int{35, 6, 1, 0} {
		e := event(i+1, "run_start")
		e.Run = fmt.Sprintf("%032x", i+20)
		e.Installation = fmt.Sprintf("%032x", i+30)
		if err := s.Record(e, "client", now.AddDate(0, 0, -days)); err != nil {
			t.Fatal(err)
		}
	}
	end := event(10, "run_end")
	end.Run = fmt.Sprintf("%032x", 20)
	end.Installation = fmt.Sprintf("%032x", 30)
	if err := s.Record(end, "client", now); err != nil {
		t.Fatal(err)
	}
	r := s.Summary("client", now)
	if r.DAU != 2 || r.WAU != 4 || r.MAU != 4 || r.Games["snake"].Starts != 3 || r.Games["snake"].Ended != 0 || len(r.Days) != 30 {
		t.Fatalf("windows/cohort: %+v", r)
	}
}
func TestWebsiteCountsAreSeparate(t *testing.T) {
	s := newStore(t)
	for i, kind := range []string{"site_visit", "install_copy"} {
		e := event(i+1, kind)
		e.Platform = "browser"
		e.Version = "website"
		if err := s.Record(e, "website", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	r := s.Summary("website", time.Now())
	if r.Visits != 1 || r.Copies != 1 || r.DAU != 0 || s.Summary("client", time.Now()).AppOpens != 0 {
		t.Fatal(r)
	}
}

func TestAdminAuthenticationCORSAndPayloadValidation(t *testing.T) {
	s := newStore(t)
	h := &Server{Store: s, Password: "a-test-password-with-32-characters", SiteOrigin: "https://cagridursun.github.io"}
	for _, path := range []string{"/admin", "/admin/dashboard.js", "/admin/api/metrics"} {
		for _, auth := range []bool{false, true} {
			req := httptest.NewRequest("GET", path, nil)
			if auth {
				req.SetBasicAuth("admin", h.Password)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			want := 401
			if auth {
				want = 200
			}
			if w.Code != want {
				t.Fatalf("%s auth=%v: %d", path, auth, w.Code)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private response cached")
			}
		}
	}
	for _, origin := range []string{"https://cagridursun.github.io", "https://evil.example"} {
		req := httptest.NewRequest("OPTIONS", "/v1/metrics/site", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if origin == h.SiteOrigin {
			if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Fatal("preflight")
			}
		} else if w.Code != 403 {
			t.Fatal("untrusted origin")
		}
	}
	e := event(3, "app_open")
	e.Source = "client"
	b, _ := json.Marshal(e)
	req := httptest.NewRequest("POST", "/v1/metrics/events", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatal("forged metadata", w.Code)
	}
	for _, body := range []string{`{"id":"` + strings.Repeat("a", 5000) + `"}`, `{} {}`} {
		req := httptest.NewRequest("POST", "/v1/metrics/events", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatal("bad body", w.Code)
		}
	}
	if err := ValidPassword("short"); err == nil {
		t.Fatal("short password")
	}
	h.Password = ""
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/admin", nil))
	if w.Code != 404 {
		t.Fatal("unconfigured admin exposed")
	}
}
func TestRateLimitCannotBeBypassedWithForwardedHeader(t *testing.T) {
	h := &Server{Store: newStore(t), Password: "a-test-password-with-32-characters"}
	for i := 0; i < 121; i++ {
		req := httptest.NewRequest("GET", "/admin", nil)
		req.SetBasicAuth("admin", h.Password)
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("1.2.3.%d", i))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if i == 120 && w.Code != 429 {
			t.Fatal("rate limit", w.Code)
		}
	}
}
func TestGitHubDownloadHistoryExcludesScriptsAndRetainsPreviousOnFailure(t *testing.T) {
	fail := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "offline", 503)
			return
		}
		fmt.Fprint(w, `[{"tag_name":"v1","assets":[{"name":"devcade_1_linux_amd64.tar.gz","download_count":4},{"name":"devcade_1_windows_amd64.zip","download_count":2},{"name":"install.sh","download_count":99},{"name":"SHA256SUMS","download_count":99}]}]`)
	}))
	defer api.Close()
	d, err := OpenDownloads(filepath.Join(t.TempDir(), "downloads.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Poll(context.Background(), api.Client(), api.URL); err != nil {
		t.Fatal(err)
	}
	if d.Report().Samples[0].Total != 6 {
		t.Fatal("non-package files counted")
	}
	fail = true
	if err = d.Poll(context.Background(), api.Client(), api.URL); err == nil {
		t.Fatal("failed refresh")
	}
	if len(d.Report().Samples) != 1 || d.Report().Error == "" {
		t.Fatal("history lost")
	}
	again, err := OpenDownloads(d.path)
	if err != nil || again.Report().Samples[0].Total != 6 {
		t.Fatal("not persisted", err)
	}
}

type stubGame struct{ finished bool }

func (g *stubGame) MinimumSize() (int, int) { return 80, 24 }
func (g *stubGame) Start(int, int)          { g.finished = false }
func (g *stubGame) Resize(int, int)         {}
func (g *stubGame) Render(engine.Canvas)    {}
func (g *stubGame) Update(time.Duration)    {}
func (g *stubGame) Finished() bool          { return g.finished }
func (g *stubGame) Score() int              { return 20 }
func (g *stubGame) HandleInput(k engine.Key) {
	if k == engine.KeyAction {
		g.finished = true
	}
	if k == engine.KeySelect {
		g.finished = false
	}
}
func TestClientConsentPauseResizeRestartAndClose(t *testing.T) {
	var mu sync.Mutex
	var events []Event
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var e Event
		_ = json.NewDecoder(r.Body).Decode(&e)
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
		w.WriteHeader(204)
	}))
	defer h.Close()
	c, err := NewClient(h.URL, "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	c.Emit(event(1, "app_open"))
	c.Close()
	mu.Lock()
	n := len(events)
	mu.Unlock()
	if n != 0 {
		t.Fatal("default-off request")
	}
	c.Enable(installation)
	g := Track(&stubGame{}, "snake", c)
	engineInstance := engine.New(g)
	engineInstance.Resize(80, 24)
	engineInstance.Advance(time.Second)
	engineInstance.Advance(50 * time.Millisecond)
	engineInstance.Input(engine.Event{Key: engine.KeyPause})
	engineInstance.Advance(time.Hour)
	engineInstance.Input(engine.Event{Key: engine.KeyPause})
	engineInstance.Advance(time.Hour)
	engineInstance.Advance(20 * time.Millisecond)
	engineInstance.Resize(20, 10)
	engineInstance.Advance(time.Hour)
	engineInstance.Resize(80, 24)
	engineInstance.Advance(time.Hour)
	engineInstance.Advance(30 * time.Millisecond)
	engineInstance.Input(engine.Event{Key: engine.KeyAction})
	g.End("closed")
	engineInstance.Input(engine.Event{Key: engine.KeySelect})
	engineInstance.Advance(10 * time.Millisecond)
	engineInstance.Advance(10 * time.Millisecond)
	g.End("left")
	c.Close()
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 4 {
		t.Fatalf("events: %+v", events)
	}
	if events[1].Kind != "run_end" || events[1].DurationMS != 100 || events[1].Outcome != "finished" || events[3].Outcome != "left" || events[3].DurationMS != 10 {
		t.Fatalf("timing/lifecycle: %+v", events)
	}
}
func TestClientDisableDropsPendingAndRejectsUnsafeEndpoints(t *testing.T) {
	for _, endpoint := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/?key=secret", "https://example.com/path"} {
		if _, err := NewClient(endpoint, "v1"); err == nil {
			t.Fatal(endpoint)
		}
	}
	c, _ := NewClient("http://127.0.0.1:1", "v1")
	c.Enable(installation)
	c.Disable()
	for i := 0; i < 100; i++ {
		c.Emit(event(i, "app_open"))
	}
	start := time.Now()
	c.Close()
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("disabled client blocked")
	}
}
