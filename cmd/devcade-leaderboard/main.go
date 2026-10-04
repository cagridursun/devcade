// Command devcade-leaderboard serves persistent anonymous community rankings.
package main

import (
	"context"
	"errors"
	"flag"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"github.com/cagridursun/devcade/internal/metrics"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	addr := flag.String("listen", "127.0.0.1:8080", "HTTP address behind a TLS reverse proxy")
	data := flag.String("data", "data/leaderboard.json", "persistent snapshot path")
	proxy := flag.String("trusted-proxy", "", "CIDR of the sole reverse proxy allowed to supply client IPs")
	flag.Parse()
	password := os.Getenv("DEVCADE_ADMIN_PASSWORD")
	if err := metrics.ValidPassword(password); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*data), 0700); err != nil {
		return err
	}
	unlock, err := leaderboard.LockData(*data)
	if err != nil {
		return errors.New("score file could not be locked; run one server instance and check data-directory permissions")
	}
	defer unlock()
	h, err := leaderboard.Open(*data)
	if err != nil {
		return err
	}
	if *proxy != "" {
		if err = h.TrustProxy(*proxy); err != nil {
			return err
		}
	}
	var handler http.Handler = h
	var downloads *metrics.DownloadStore
	if password != "" {
		store, err := metrics.Open(filepath.Join(filepath.Dir(*data), "metrics.jsonl"))
		if err != nil {
			return err
		}
		downloads, err = metrics.OpenDownloads(filepath.Join(filepath.Dir(*data), "downloads.json"))
		if err != nil {
			return err
		}
		origin := os.Getenv("DEVCADE_SITE_ORIGIN")
		if origin == "" {
			origin = "https://cagridursun.github.io"
		}
		stats := &metrics.Server{Store: store, Downloads: downloads, Password: password, SiteOrigin: origin}
		if *proxy != "" {
			if err = stats.TrustProxy(*proxy); err != nil {
				return err
			}
		}
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/") || strings.HasPrefix(r.URL.Path, "/v1/metrics/") {
				stats.ServeHTTP(w, r)
				return
			}
			h.ServeHTTP(w, r)
		})
	}
	srv := &http.Server{Addr: *addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var collectorDone chan struct{}
	if downloads != nil {
		collectorDone = make(chan struct{})
		go func() {
			defer close(collectorDone)
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				if err := downloads.Poll(ctx, &http.Client{Timeout: 15 * time.Second}, "https://api.github.com"); err != nil && ctx.Err() == nil {
					log.Print("GitHub download statistics could not be refreshed")
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
		defer func() { stop(); <-collectorDone }()
	}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	log.Printf("leaderboard listening on %s", *addr)
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
