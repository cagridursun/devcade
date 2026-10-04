package metrics

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cagridursun/devcade/internal/engine"
)

func NewID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}

type Client struct {
	mu                              sync.Mutex
	endpoint, version, installation string
	http                            *http.Client
	queue                           chan Event
	cancel                          context.CancelFunc
	done                            chan struct{}
}

// Constructing a client makes no requests; Enable is called only after consent
// and the random installation identifier have been saved successfully.
func NewClient(endpoint, version string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("invalid metrics endpoint")
	}
	if u.Scheme != "https" && (u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1")) {
		return nil, fmt.Errorf("metrics requires HTTPS")
	}
	if !versionName.MatchString(version) {
		return nil, fmt.Errorf("invalid version")
	}
	h := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{endpoint: strings.TrimRight(endpoint, "/"), version: version, http: h}, nil
}
func (c *Client) Enable(id string) {
	if c == nil || !hexID.MatchString(id) {
		return
	}
	c.Disable()
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.installation = id
	c.queue = make(chan Event, 64)
	c.done = make(chan struct{})
	go c.worker(ctx, c.queue, c.done)
}
func (c *Client) Disable() {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.queue = nil
	c.installation = ""
	c.cancel = nil
	c.mu.Unlock()
}
func (c *Client) Emit(e Event) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.queue == nil {
		return
	}
	id, err := NewID()
	if err != nil {
		return
	}
	e.ID = id
	e.Installation = c.installation
	e.Version = c.version
	e.Platform = runtime.GOOS
	select {
	case c.queue <- e:
	default: /* bounded best-effort queue; never delay the game */
	}
}
func (c *Client) worker(ctx context.Context, q chan Event, done chan struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-q:
			if !ok {
				return
			}
			if ctx.Err() != nil {
				return
			}
			b, err := json.Marshal(e)
			if err != nil {
				continue
			}
			req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/v1/metrics/events", bytes.NewReader(b))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			response, err := c.http.Do(req)
			if err == nil {
				response.Body.Close()
			}
		}
	}
}

// Close flushes queued events for at most one second. Disabled consent cancels
// immediately instead. Failures and unsent events are never replayed next time.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	q, cancel, done := c.queue, c.cancel, c.done
	c.queue = nil
	c.cancel = nil
	c.installation = ""
	if q != nil {
		close(q)
	}
	c.mu.Unlock()
	if cancel == nil {
		return
	}
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		cancel()
		<-done
	}
	cancel()
}

type TrackedGame struct {
	engine.Game
	client    *Client
	name, run string
	duration  time.Duration
	ended     bool
}

func Track(g engine.Game, name string, c *Client) *TrackedGame {
	return &TrackedGame{Game: g, name: name, client: c, ended: true}
}
func (g *TrackedGame) begin() {
	g.run, _ = NewID()
	g.duration = 0
	g.ended = false
	if g.run != "" {
		g.client.Emit(Event{Kind: "run_start", Game: g.name, Run: g.run})
	}
}
func (g *TrackedGame) Start(w, h int) { g.Game.Start(w, h); g.begin() }
func (g *TrackedGame) Finished() bool { f, ok := g.Game.(engine.Finisher); return ok && f.Finished() }
func (g *TrackedGame) Score() int {
	if s, ok := g.Game.(interface{ Score() int }); ok {
		return s.Score()
	}
	return 0
}
func (g *TrackedGame) HandleInput(k engine.Key) {
	was := g.Finished()
	g.Game.HandleInput(k)
	if was && !g.Finished() {
		g.begin()
	}
	if g.Finished() {
		g.End("finished")
	}
}
func (g *TrackedGame) Update(dt time.Duration) {
	g.Game.Update(dt)
	g.duration += dt
	if g.Finished() {
		g.End("finished")
	}
}
func (g *TrackedGame) End(outcome string) {
	if g.ended {
		return
	}
	g.ended = true
	if g.run != "" {
		e := Event{Kind: "run_end", Game: g.name, Run: g.run, DurationMS: g.duration.Milliseconds(), Score: g.Score(), Outcome: outcome}
		if game, ok := g.Game.(interface{ Statistics() engine.RunStats }); ok {
			s := game.Statistics()
			e.Statistics = &s
		}
		g.client.Emit(e)
	}
}
