package leaderboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func service(t *testing.T) (*Server, *Client, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scores.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := httptest.NewServer(s)
	t.Cleanup(h.Close)
	c, err := NewClient(h.URL)
	if err != nil {
		t.Fatal(err)
	}
	return s, c, path
}
func TestTwoClientsShareBestPerGameAndSurviveRestart(t *testing.T) {
	_, c, path := service(t)
	ctx := context.Background()
	one, err := c.Register(ctx, "player_one")
	if err != nil {
		t.Fatal(err)
	}
	two, err := c.Register(ctx, "player_two")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Register(ctx, "player_one"); !errors.Is(err, ErrNameTaken) {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, game := range []string{"snake", "blockdrop", "mazechase", "blastgrid", "brickbreaker", "terminalfc", "spaceshooter"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, n := range []int{100, 20, 200, 100} {
				if err := c.Submit(ctx, one.Token, game, n); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if err = c.Submit(ctx, two.Token, "snake", 300); err != nil {
		t.Fatal(err)
	}
	for _, game := range []string{"snake", "blockdrop", "mazechase", "blastgrid", "brickbreaker", "terminalfc", "spaceshooter"} {
		b, err := c.Fetch(ctx, game, one.ID)
		if err != nil || b.Own == nil || b.Own.Score != 200 {
			t.Fatal(game, b, err)
		}
		if game == "snake" && (len(b.Rows) != 2 || b.Rows[0].PlayerID != two.ID || b.Own.Rank != 2) {
			t.Fatal(b)
		}
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), one.Token) {
		t.Fatal("server persisted plaintext token")
	}
	restarted, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := httptest.NewServer(restarted)
	defer h.Close()
	fresh, _ := NewClient(h.URL)
	if err = fresh.Submit(ctx, one.Token, "snake", 400); err != nil {
		t.Fatal("identity lost on restart", err)
	}
	b, err := fresh.Fetch(ctx, "snake", one.ID)
	if err != nil || b.Own.Score != 400 || b.Own.Rank != 1 {
		t.Fatal(b, err)
	}
}
func request(s *Server, method, path, body, token, addr string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = addr
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestOwnRankOutsideTop20AndStableTies(t *testing.T) {
	s, _, _ := service(t)
	ids := []string{}
	for i := range 25 {
		addr := fmt.Sprintf("192.0.2.%d:80", i+1)
		w := request(s, "POST", "/v1/players", fmt.Sprintf(`{"username":"player_%02d"}`, i), "", addr)
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body)
		}
		var p Registration
		_ = json.Unmarshal(w.Body.Bytes(), &p)
		ids = append(ids, p.ID)
		score := (i + 1) * 10
		if i == 24 {
			score = 10
		}
		w = request(s, "PUT", "/v1/best", fmt.Sprintf(`{"game":"snake","score":%d}`, score), p.Token, addr)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
	w := request(s, "GET", "/v1/leaderboards/snake?player="+ids[24], "", "", "192.0.2.99:80")
	var b Board
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	if len(b.Rows) != 20 || b.Own == nil || b.Own.Rank != 25 || b.Own.Username != "player_24" {
		t.Fatal(b)
	}
}
func TestValidationAuthenticationAndWriteRollback(t *testing.T) {
	s, c, path := service(t)
	p, err := c.Register(context.Background(), "valid_user")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		body, token string
		status      int
	}{{`{"game":"snake","score":100}`, "wrong", 401}, {`{"game":"bad","score":100}`, p.Token, 400}, {`{"game":"snake","score":101}`, p.Token, 400}, {`{"game":"snake","score":-10}`, p.Token, 400}, {`{"game":"snake"}`, p.Token, 400}, {`{"game":"snake","score":100,"extra":1}`, p.Token, 400}, {`{"game":"snake","score":100} {}`, p.Token, 400}, {strings.Repeat("x", 5000), p.Token, 400}} {
		w := request(s, "PUT", "/v1/best", tt.body, tt.token, "192.0.2.1:90")
		if w.Code != tt.status {
			t.Errorf("status %d want %d", w.Code, tt.status)
		}
	}
	if err = c.Submit(context.Background(), p.Token, "snake", 100); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.path = filepath.Dir(path)
	s.mu.Unlock() // replacement onto a directory fails
	w := request(s, "PUT", "/v1/best", `{"game":"snake","score":200}`, p.Token, "192.0.2.1:90")
	if w.Code != 500 {
		t.Fatal(w.Code)
	}
	s.mu.Lock()
	s.path = path
	s.mu.Unlock()
	b, err := c.Fetch(context.Background(), "snake", p.ID)
	if err != nil || b.Own.Score != 100 {
		t.Fatal("failed write changed best", b, err)
	}
}
func TestRateLimitAndCorruptStore(t *testing.T) {
	s, _, path := service(t)
	for i := range 121 {
		w := request(s, "GET", "/v1/leaderboards/snake", "", "", "192.0.2.2:80")
		if i == 120 && w.Code != 429 {
			t.Fatal(w.Code)
		}
	}
	if err := os.WriteFile(path, []byte("bad JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("corrupt score store accepted")
	}
}
func TestClientRejectsUnsafeURLsAndRedirects(t *testing.T) {
	for _, u := range []string{"http://scores.example", "https://user:secret@scores.example", "https://scores.example?token=secret", "https://scores.example#frag", "file:///tmp/scores"} {
		if _, err := NewClient(u); err == nil {
			t.Error(u)
		}
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("followed redirect carrying identity") }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	c, _ := NewClient(redirect.URL)
	if err := c.Submit(context.Background(), strings.Repeat("a", 64), "snake", 100); err == nil {
		t.Fatal("redirect accepted")
	}
}

func TestOnlyConfiguredProxyMaySupplyClientIP(t *testing.T) {
	s, _, _ := service(t)
	if err := s.TrustProxy("192.0.2.10/32"); err != nil {
		t.Fatal(err)
	}
	for i := range 121 {
		r := httptest.NewRequest("GET", "/v1/leaderboards/snake", nil)
		r.RemoteAddr = "192.0.2.1:80"
		r.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i%250+1))
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if i == 120 && w.Code != 429 {
			t.Fatal("untrusted forwarded address bypassed limit")
		}
	}
	for i := range 121 {
		r := httptest.NewRequest("GET", "/v1/leaderboards/snake", nil)
		r.RemoteAddr = "192.0.2.10:80"
		r.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.1, 198.51.100.%d", i%250+1))
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal("trusted client buckets mixed", w.Code)
		}
	}
}
