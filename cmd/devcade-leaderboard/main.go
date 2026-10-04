// Command devcade-leaderboard serves persistent anonymous community rankings.
package main

import (
	"context"
	"errors"
	"flag"
	"github.com/cagridursun/devcade/internal/leaderboard"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	srv := &http.Server{Addr: *addr, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
