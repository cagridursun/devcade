// Package metrics records opt-in usage statistics.
// Installation identifiers are pseudonymous, not verified people.
package metrics

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/cagridursun/devcade/internal/profile"
)

var ErrInvalid = errors.New("invalid metrics event")
var ErrFull = errors.New("metrics capacity reached")

var hexID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var versionName = regexp.MustCompile(`^[a-zA-Z0-9.+_-]{1,64}$`)

type Event struct {
	ID           string    `json:"id"`
	Installation string    `json:"installation"`
	Kind         string    `json:"kind"`
	Run          string    `json:"run,omitempty"`
	Game         string    `json:"game,omitempty"`
	Platform     string    `json:"platform"`
	Version      string    `json:"version"`
	DurationMS   int64     `json:"duration_ms,omitempty"`
	Score        int       `json:"score,omitempty"`
	Outcome      string    `json:"outcome,omitempty"`
	Source       string    `json:"source,omitempty"`
	At           time.Time `json:"at,omitempty"`
}

func (e Event) valid() bool {
	if !hexID.MatchString(e.ID) || !hexID.MatchString(e.Installation) || !versionName.MatchString(e.Version) {
		return false
	}
	if e.Platform != "linux" && e.Platform != "darwin" && e.Platform != "windows" && e.Platform != "browser" {
		return false
	}
	switch e.Kind {
	case "app_open":
		return e.Source != "website" && e.Platform != "browser" && e.Game == "" && e.Run == "" && e.DurationMS == 0 && e.Score == 0 && e.Outcome == ""
	case "site_visit", "install_copy":
		return e.Source == "website" && e.Platform == "browser" && e.Game == "" && e.Run == "" && e.DurationMS == 0 && e.Score == 0 && e.Outcome == ""
	case "run_start", "run_end":
		if e.Source == "website" || e.Platform == "browser" || !hexID.MatchString(e.Run) || !profile.ValidGame(e.Game) {
			return false
		}
		if e.Kind == "run_start" {
			return e.DurationMS == 0 && e.Score == 0 && e.Outcome == ""
		}
		return e.DurationMS >= 0 && e.DurationMS <= int64((24*time.Hour)/time.Millisecond) && profile.ValidScore(e.Game, e.Score) && (e.Outcome == "finished" || e.Outcome == "left" || e.Outcome == "closed")
	}
	return false
}

// The journal is capped, durably appended, and recovered before HTTP starts.
// No usernames, IP addresses, hardware fingerprint or input sequence is saved.
type Store struct {
	mu     sync.Mutex
	path   string
	events []Event
	seen   map[string]Event
	starts map[string]Event
	ends   map[string]bool
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, seen: map[string]Event{}, starts: map[string]Event{}, ends: map[string]bool{}}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 32<<20 {
		return nil, fmt.Errorf("metrics journal exceeds 32 MiB")
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Event
		if err = json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("invalid metrics journal: %w", err)
		}
		if e.At.IsZero() || (e.Source != "client" && e.Source != "website") || !e.valid() {
			return nil, fmt.Errorf("invalid persisted event")
		}
		if err = s.accept(e); err != nil {
			return nil, err
		}
		s.remember(e)
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return s, nil
}
func runKey(e Event) string   { return e.Source + ":" + e.Installation + ":" + e.Run }
func eventKey(e Event) string { return e.Source + ":" + e.Installation + ":" + e.ID }
func (s *Store) accept(e Event) error {
	if len(s.events) >= 50000 {
		return ErrFull
	}
	if _, ok := s.seen[eventKey(e)]; ok {
		return fmt.Errorf("%w: duplicate persisted event", ErrInvalid)
	}
	if e.Kind == "run_start" {
		if _, ok := s.starts[runKey(e)]; ok {
			return fmt.Errorf("%w: run already started", ErrInvalid)
		}
	}
	if e.Kind == "run_end" {
		start, ok := s.starts[runKey(e)]
		if !ok || s.ends[runKey(e)] || start.Game != e.Game || start.Platform != e.Platform || start.Version != e.Version {
			return fmt.Errorf("%w: invalid run lifecycle", ErrInvalid)
		}
	}
	return nil
}
func (s *Store) remember(e Event) {
	s.events = append(s.events, e)
	s.seen[eventKey(e)] = e
	if e.Kind == "run_start" {
		s.starts[runKey(e)] = e
	}
	if e.Kind == "run_end" {
		s.ends[runKey(e)] = true
	}
}
func (s *Store) Record(e Event, source string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if (source != "client" && source != "website") || e.Source != "" || !e.At.IsZero() {
		return fmt.Errorf("%w: server owns source and timestamps", ErrInvalid)
	}
	e.Source = source
	e.At = now.UTC()
	if !e.valid() {
		return fmt.Errorf("%w: invalid event", ErrInvalid)
	}
	if previous, ok := s.seen[eventKey(e)]; ok {
		e.At = previous.At
		if e != previous {
			return fmt.Errorf("%w: event id conflict", ErrInvalid)
		}
		return nil
	}
	if err := s.accept(e); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err = os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	old, err := f.Seek(0, 2)
	if err != nil {
		f.Close()
		return err
	}
	if old+int64(len(b)) > 32<<20 {
		f.Close()
		return ErrFull
	}
	n, err := f.Write(b)
	if err == nil && n != len(b) {
		err = fmt.Errorf("short journal write")
	}
	if err == nil {
		err = f.Sync()
	}
	if err != nil {
		_ = f.Truncate(old)
		_ = f.Sync()
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	s.remember(e)
	return nil
}

type GameStats struct {
	Starts        int     `json:"starts"`
	Finished      int     `json:"finished"`
	Left          int     `json:"left"`
	Closed        int     `json:"closed"`
	Ended         int     `json:"ended"`
	Unfinished    int     `json:"unfinished"`
	MeanSeconds   float64 `json:"mean_seconds"`
	MedianSeconds float64 `json:"median_seconds"`
}
type Day struct {
	Date   string `json:"date"`
	Active int    `json:"active"`
	Starts int    `json:"starts"`
	Ends   int    `json:"ends"`
}
type Summary struct {
	Source        string               `json:"source"`
	DAU           int                  `json:"dau"`
	WAU           int                  `json:"wau"`
	MAU           int                  `json:"mau"`
	AppOpens      int                  `json:"app_opens"`
	Installations int                  `json:"installations"`
	Returning     int                  `json:"returning"`
	Visits        int                  `json:"site_visits"`
	Copies        int                  `json:"install_copies"`
	Games         map[string]GameStats `json:"games"`
	Platforms     map[string]int       `json:"platforms"`
	Versions      map[string]int       `json:"versions"`
	Days          []Day                `json:"days"`
}

func (s *Store) Summary(source string, now time.Time) Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := Summary{Source: source, Games: map[string]GameStats{}, Platforms: map[string]int{}, Versions: map[string]int{}, Days: []Day{}}
	day := func(t time.Time) string { return t.UTC().Format("2006-01-02") }
	today := day(now)
	month := day(now.UTC().AddDate(0, 0, -29))
	week := day(now.UTC().AddDate(0, 0, -6))
	active := map[string]map[string]bool{}
	all := map[string]map[string]bool{}
	weekly := map[string]bool{}
	monthly := map[string]bool{}
	ps := map[string]map[string]bool{}
	vs := map[string]map[string]bool{}
	daily := map[string]*Day{}
	durations := map[string][]int64{}
	for _, e := range s.events {
		if e.Source != source || e.At.After(now) {
			continue
		}
		date := day(e.At)
		if date < month {
			continue
		}
		if daily[date] == nil {
			daily[date] = &Day{Date: date}
		}
		d := daily[date]
		if e.Kind == "site_visit" {
			r.Visits++
		}
		if e.Kind == "install_copy" {
			r.Copies++
		}
		if e.Kind == "app_open" {
			r.AppOpens++
		}
		if source == "website" {
			continue
		}
		if all[e.Installation] == nil {
			all[e.Installation] = map[string]bool{}
		}
		all[e.Installation][date] = true
		if e.Kind != "run_start" && e.Kind != "run_end" {
			continue
		}
		if active[date] == nil {
			active[date] = map[string]bool{}
		}
		active[date][e.Installation] = true
		monthly[e.Installation] = true
		if date >= week {
			weekly[e.Installation] = true
		}
		if ps[e.Platform] == nil {
			ps[e.Platform] = map[string]bool{}
		}
		ps[e.Platform][e.Installation] = true
		if vs[e.Version] == nil {
			vs[e.Version] = map[string]bool{}
		}
		vs[e.Version][e.Installation] = true
		g := r.Games[e.Game]
		if e.Kind == "run_start" {
			g.Starts++
			d.Starts++
		} else {
			d.Ends++
			if day(s.starts[runKey(e)].At) < month {
				continue
			}
			g.Ended++
			durations[e.Game] = append(durations[e.Game], e.DurationMS)
			switch e.Outcome {
			case "finished":
				g.Finished++
			case "left":
				g.Left++
			case "closed":
				g.Closed++
			}
		}
		r.Games[e.Game] = g
	}
	r.DAU = len(active[today])
	r.WAU = len(weekly)
	r.MAU = len(monthly)
	r.Installations = len(all)
	for _, dates := range all {
		if len(dates) > 1 {
			r.Returning++
		}
	}
	for k, v := range ps {
		r.Platforms[k] = len(v)
	}
	for k, v := range vs {
		r.Versions[k] = len(v)
	}
	for i := 29; i >= 0; i-- {
		date := day(now.UTC().AddDate(0, 0, -i))
		d := Day{Date: date}
		if daily[date] != nil {
			d = *daily[date]
		}
		d.Active = len(active[date])
		r.Days = append(r.Days, d)
	}
	for name, g := range r.Games {
		g.Unfinished = max(0, g.Starts-g.Ended)
		ds := durations[name]
		if len(ds) > 0 {
			sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
			var total int64
			for _, n := range ds {
				total += n
			}
			g.MeanSeconds = float64(total) / float64(len(ds)) / 1000
			g.MedianSeconds = float64(ds[len(ds)/2]) / 1000
			if len(ds)%2 == 0 {
				g.MedianSeconds = float64(ds[len(ds)/2]+ds[len(ds)/2-1]) / 2000
			}
		}
		r.Games[name] = g
	}
	return r
}
