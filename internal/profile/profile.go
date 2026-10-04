// Package profile persists preferences, anonymous identity and personal bests.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type Identity struct {
	ID       string `json:"id,omitempty"`
	Token    string `json:"token,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

type Profile struct {
	Version   int            `json:"version"`
	Language  string         `json:"language"`
	Theme     string         `json:"theme"`
	Username  string         `json:"username,omitempty"`
	Share     bool           `json:"share_scores"`
	Metrics   bool           `json:"share_usage"`
	MetricsID string         `json:"usage_id,omitempty"`
	Identity  Identity       `json:"identity,omitempty"`
	Best      map[string]int `json:"personal_best"`
}

func Default() Profile {
	return Profile{Version: 1, Language: "en", Theme: "mono", Best: map[string]int{}}
}

var usageID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var username = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)

func ValidUsername(s string) bool { return username.MatchString(s) }
var validGames = map[string]struct{}{
	"snake": {}, "blockdrop": {}, "mazechase": {}, "blastgrid": {}, "terminalfc": {},
}

func ValidGame(s string) bool {
	_, ok := validGames[s]
	return ok
}

func GameCount() int { return len(validGames) }
func ValidScore(game string, n int) bool {
	if !ValidGame(game) || n < 0 || n > 1000000000 {
		return false
	}
	if game == "snake" {
		return n <= 6450 && n%10 == 0
	}
	if game == "terminalfc" {
		return n <= 1750
	}
	return true
}
func (p Profile) Clone() Profile {
	q := p
	q.Best = map[string]int{}
	for k, v := range p.Best {
		q.Best[k] = v
	}
	return q
}
func (p *Profile) Record(game string, n int) bool {
	if !ValidScore(game, n) {
		return false
	}
	if p.Best == nil {
		p.Best = map[string]int{}
	}
	old, exists := p.Best[game]
	if exists && n <= old {
		return false
	}
	p.Best[game] = n
	return true
}

type Store struct{ Path string }

func DefaultStore() (Store, error) {
	if dir := os.Getenv("DEVCADE_CONFIG_DIR"); dir != "" {
		return Store{filepath.Join(dir, "profile.json")}, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}, err
	}
	return Store{filepath.Join(dir, "devcade", "profile.json")}, nil
}
func (s Store) Load() (Profile, error) {
	p := Default()
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if len(b) > 65536 {
		return p, fmt.Errorf("profile too large")
	}
	if err = json.Unmarshal(b, &p); err != nil {
		return Default(), err
	}
	if p.Version != 1 {
		return Default(), fmt.Errorf("unsupported profile version")
	}
	switch p.Language {
	case "en", "tr", "es", "nl", "fr":
	default:
		p.Language = "en"
	}
	switch p.Theme {
	case "mono", "midnight", "colorful":
	default:
		p.Theme = "mono"
	}
	if p.Username != "" && !ValidUsername(p.Username) {
		return Default(), fmt.Errorf("invalid username")
	}
	if p.MetricsID != "" && !usageID.MatchString(p.MetricsID) {
		p.MetricsID = ""
	}
	if p.Best == nil {
		p.Best = map[string]int{}
	}
	for game, n := range p.Best {
		if !ValidScore(game, n) {
			delete(p.Best, game)
		}
	}
	return p, nil
}
func (s Store) Save(p Profile) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return Write(s.Path, append(b, '\n'))
}

// Write uses a private temporary file in the same directory and never truncates
// the previous snapshot if writing or replacement fails.
func Write(path string, b []byte) error {
	if path == "" {
		return fmt.Errorf("no persistence path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".devcade-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
