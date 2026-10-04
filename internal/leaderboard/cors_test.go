package leaderboard

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestWebsiteMayReadOnlyPublicRankings(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "scores.json"))
	if err != nil {
		t.Fatal(err)
	}
	origin := "https://cagridursun.github.io"
	h := WithReadOrigin(s, origin)
	for _, tt := range []struct {
		method, path, origin string
		allowed              bool
	}{
		{"GET", "/v1/leaderboards/snake", origin, true},
		{"GET", "/v1/leaderboards/mazechase", origin, true},
		{"GET", "/v1/leaderboards/snake", "https://other.example", false},
		{"GET", "/v1/leaderboards/snake", "", false},
		{"GET", "/healthz", origin, false},
		{"POST", "/v1/players", origin, false},
		{"PUT", "/v1/best", origin, false},
		{"OPTIONS", "/v1/leaderboards/snake", origin, false},
	} {
		r := httptest.NewRequest(tt.method, tt.path, nil)
		r.Header.Set("Origin", tt.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		got := w.Header().Get("Access-Control-Allow-Origin")
		if (got == origin) != tt.allowed || (got != "" && got != origin) {
			t.Fatalf("%s %s %s: CORS %q", tt.method, tt.path, tt.origin, got)
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatal("credentials must not be enabled")
		}
		if tt.method == "GET" && tt.path != "/healthz" && (w.Code != 200 || w.Header().Get("Vary") != "Origin") {
			t.Fatalf("public/native ranking changed: %d", w.Code)
		}
	}
}
