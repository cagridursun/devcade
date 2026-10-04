package metrics

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed dashboard.html dashboard.css dashboard.js
var dashboard embed.FS

type Server struct {
	Store      *Store
	Downloads  *DownloadStore
	Password   string
	SiteOrigin string
	mu         sync.Mutex
	limits     map[string]limit
	proxy      *net.IPNet
}
type limit struct {
	at    time.Time
	count int
}

func (s *Server) TrustProxy(cidr string) error {
	_, n, err := net.ParseCIDR(cidr)
	s.proxy = n
	return err
}
func (s *Server) authorized(r *http.Request) bool {
	u, p, ok := r.BasicAuth()
	a, b := sha256.Sum256([]byte(p)), sha256.Sum256([]byte(s.Password))
	return len(s.Password) >= 16 && ok && u == "admin" && subtle.ConstantTimeCompare(a[:], b[:]) == 1
}
func (s *Server) allow(r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.limits == nil {
		s.limits = map[string]limit{}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if s.proxy != nil && s.proxy.Contains(net.ParseIP(ip)) {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if p := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); p != nil {
			ip = p.String()
		}
	}
	now := time.Now()
	b := s.limits[ip]
	if now.Sub(b.at) >= time.Minute {
		b = limit{at: now}
	}
	if len(s.limits) >= 4096 {
		for k, v := range s.limits {
			if now.Sub(v.at) >= time.Minute {
				delete(s.limits, k)
			}
		}
		if _, ok := s.limits[ip]; !ok && len(s.limits) >= 4096 {
			return false
		}
	}
	b.count++
	s.limits[ip] = b
	return b.count <= 120
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if len(s.Password) < 16 {
		http.NotFound(w, r)
		return
	}
	if !s.allow(r) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "Too many requests", 429)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin") {
		if !s.authorized(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="DevCade statistics", charset="UTF-8"`)
			http.Error(w, "Authentication required", 401)
			return
		}
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		switch r.URL.Path {
		case "/admin", "/admin/":
			s.asset(w, "dashboard.html", "text/html; charset=utf-8")
		case "/admin/dashboard.css":
			s.asset(w, "dashboard.css", "text/css; charset=utf-8")
		case "/admin/dashboard.js":
			s.asset(w, "dashboard.js", "text/javascript; charset=utf-8")
		case "/admin/api/metrics":
			w.Header().Set("Content-Type", "application/json")
			var downloads any
			if s.Downloads != nil {
				downloads = s.Downloads.Report()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"generated_at": time.Now().UTC(), "timezone": "UTC", "client": s.Store.Summary("client", time.Now()), "website": s.Store.Summary("website", time.Now()), "downloads": downloads})
		default:
			http.NotFound(w, r)
		}
		return
	}
	source := "client"
	if r.URL.Path == "/v1/metrics/site" {
		source = "website"
		origin := r.Header.Get("Origin")
		if s.SiteOrigin == "" || origin != s.SiteOrigin {
			http.Error(w, "Origin not allowed", 403)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", s.SiteOrigin)
		w.Header().Set("Vary", "Origin")
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(204)
			return
		}
	} else if r.URL.Path != "/v1/metrics/events" {
		http.NotFound(w, r)
		return
	} else if r.Header.Get("Origin") != "" {
		http.Error(w, "Use site endpoint", 403)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Expected JSON", 415)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var e Event
	if err := decoder.Decode(&e); err != nil {
		http.Error(w, "Invalid event", 400)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Invalid event", 400)
		return
	}
	if err := s.Store.Record(e, source, time.Now()); err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, ErrInvalid) {
			status = http.StatusBadRequest
		}
		http.Error(w, "Event rejected", status)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) asset(w http.ResponseWriter, name, mime string) {
	b, err := dashboard.ReadFile(name)
	if err != nil {
		http.Error(w, "Missing dashboard asset", 500)
		return
	}
	w.Header().Set("Content-Type", mime)
	_, _ = w.Write(b)
}
func ValidPassword(p string) error {
	if p != "" && len(p) < 16 {
		return fmt.Errorf("metrics admin password must have at least 16 characters")
	}
	return nil
}
