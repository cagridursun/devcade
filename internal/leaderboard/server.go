package leaderboard

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cagridursun/devcade/internal/profile"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type player struct {
	ID        string         `json:"id"`
	Username  string         `json:"username"`
	TokenHash string         `json:"token_hash"`
	Best      map[string]int `json:"best"`
}
type snapshot struct {
	Version int               `json:"version"`
	Players map[string]player `json:"players"`
}
type bucket struct {
	Start                   time.Time
	Requests, Registrations int
}
type Server struct {
	mu     sync.Mutex
	path   string
	data   snapshot
	limits map[string]bucket
	proxy  *net.IPNet
}

// TrustProxy must be configured before serving. Forwarded addresses are used
// only from the explicitly trusted proxy; other request headers are ignored.
func (s *Server) TrustProxy(cidr string) error {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	s.proxy = network
	return nil
}

// Open fails closed on a malformed persisted snapshot. One process owns a data
// file; the command holds an OS writer lock for the server lifetime.
func Open(path string) (*Server, error) {
	s := &Server{path: path, data: snapshot{Version: 1, Players: map[string]player{}}, limits: map[string]bucket{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 8<<20 {
		return nil, fmt.Errorf("score store too large")
	}
	if err = json.Unmarshal(b, &s.data); err != nil {
		return nil, err
	}
	if s.data.Version != 1 || s.data.Players == nil || len(s.data.Players) > 5000 {
		return nil, fmt.Errorf("invalid score snapshot")
	}
	names := map[string]bool{}
	for id, p := range s.data.Players {
		if id != p.ID || len(id) != 32 || len(p.TokenHash) != 64 || !profile.ValidUsername(p.Username) || names[p.Username] {
			return nil, fmt.Errorf("invalid player")
		}
		names[p.Username] = true
		if len(p.Best) > profile.GameCount() {
			return nil, fmt.Errorf("invalid bests")
		}
		for game, n := range p.Best {
			if !profile.ValidScore(game, n) {
				return nil, fmt.Errorf("invalid stored score")
			}
		}
	}
	return s, nil
}
func (s *Server) persist() error {
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	return profile.Write(s.path, b)
}
func random(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("extra JSON")
	}
	return nil
}
func respond(w http.ResponseWriter, n int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(n)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, n int) {
	respond(w, n, map[string]string{"error": http.StatusText(n)})
}
func (s *Server) allowed(r *http.Request) bool {
	key, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		key = r.RemoteAddr
	}
	if s.proxy != nil && s.proxy.Contains(net.ParseIP(key)) {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if ip := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); ip != nil {
			key = ip.String()
		}
	}
	now := time.Now()
	b := s.limits[key]
	if b.Start.IsZero() || now.Sub(b.Start) >= time.Minute {
		b = bucket{Start: now}
	}
	if len(s.limits) >= 4096 {
		for k, v := range s.limits {
			if now.Sub(v.Start) >= time.Minute {
				delete(s.limits, k)
			}
		}
		if _, ok := s.limits[key]; !ok && len(s.limits) >= 4096 {
			return false
		}
	}
	b.Requests++
	registration := r.Method == "POST" && r.URL.Path == "/v1/players"
	if registration {
		b.Registrations++
	}
	s.limits[key] = b
	return b.Requests <= 120 && b.Registrations <= 10
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/healthz" && r.Method == "GET" {
		respond(w, 200, map[string]string{"status": "ok"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.allowed(r) {
		w.Header().Set("Retry-After", "60")
		fail(w, 429)
		return
	}
	switch {
	case r.URL.Path == "/v1/players" && r.Method == "POST":
		var in struct {
			Username string `json:"username"`
		}
		if decode(w, r, &in) != nil || !profile.ValidUsername(in.Username) {
			fail(w, 400)
			return
		}
		for _, p := range s.data.Players {
			if p.Username == in.Username {
				fail(w, 409)
				return
			}
		}
		if len(s.data.Players) >= 5000 {
			fail(w, 503)
			return
		}
		id, err := random(16)
		if err != nil {
			fail(w, 500)
			return
		}
		token, err := random(32)
		if err != nil {
			fail(w, 500)
			return
		}
		hash := sha256.Sum256([]byte(token))
		s.data.Players[id] = player{ID: id, Username: in.Username, TokenHash: hex.EncodeToString(hash[:]), Best: map[string]int{}}
		if s.persist() != nil {
			delete(s.data.Players, id)
			fail(w, 500)
			return
		}
		respond(w, 201, Registration{ID: id, Token: token})
	case r.URL.Path == "/v1/best" && r.Method == "PUT":
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			fail(w, 401)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		hash := sha256.Sum256([]byte(token))
		want := hex.EncodeToString(hash[:])
		id := ""
		for k, p := range s.data.Players {
			if subtle.ConstantTimeCompare([]byte(p.TokenHash), []byte(want)) == 1 && len(token) == 64 {
				id = k
				break
			}
		}
		if id == "" {
			fail(w, 401)
			return
		}
		var in struct {
			Game  string `json:"game"`
			Score *int   `json:"score"`
		}
		if decode(w, r, &in) != nil || in.Score == nil || !profile.ValidScore(in.Game, *in.Score) {
			fail(w, 400)
			return
		}
		p := s.data.Players[id]
		if p.Best == nil {
			p.Best = map[string]int{}
		}
		old, ok := p.Best[in.Game]
		if !ok || *in.Score > old {
			p.Best[in.Game] = *in.Score
			s.data.Players[id] = p
			if s.persist() != nil {
				if ok {
					p.Best[in.Game] = old
				} else {
					delete(p.Best, in.Game)
				}
				s.data.Players[id] = p
				fail(w, 500)
				return
			}
		}
		respond(w, 200, map[string]int{"best": p.Best[in.Game]})
	case strings.HasPrefix(r.URL.Path, "/v1/leaderboards/") && r.Method == "GET":
		game := strings.TrimPrefix(r.URL.Path, "/v1/leaderboards/")
		if !profile.ValidGame(game) {
			fail(w, 404)
			return
		}
		rows := []Row{}
		for _, p := range s.data.Players {
			if n, ok := p.Best[game]; ok {
				rows = append(rows, Row{PlayerID: p.ID, Username: p.Username, Score: n})
			}
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Score != rows[j].Score {
				return rows[i].Score > rows[j].Score
			}
			return rows[i].Username < rows[j].Username
		})
		b := Board{Rows: []Row{}}
		for i := range rows {
			rows[i].Rank = i + 1
			if rows[i].PlayerID == r.URL.Query().Get("player") {
				own := rows[i]
				b.Own = &own
			}
		}
		b.Rows = rows[:min(len(rows), 20)]
		respond(w, 200, b)
	default:
		fail(w, 404)
	}
}
