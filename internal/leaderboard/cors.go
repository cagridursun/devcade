package leaderboard

import (
	"net/http"
	"strings"
)

// WithReadOrigin permits the project website to read public rankings. It does
// not grant browser access to registration, score writes or administrator data.
func WithReadOrigin(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/leaderboards/") {
			w.Header().Add("Vary", "Origin")
			if origin != "" && r.Header.Get("Origin") == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}
		next.ServeHTTP(w, r)
	})
}
