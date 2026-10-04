package arcade

import (
	"strings"
	"testing"

	"github.com/cagridursun/devcade/internal/engine"
)

func TestBuiltinCatalog(t *testing.T) {
	c := Builtin()
	want := []struct{ id, name, milestone string }{
		{"snake", "Snake", ""},          // M3
		{"blockdrop", "Block Drop", ""}, // M4
		{"mazechase", "Maze Chase", ""}, // M5
		{"blastgrid", "Blast Grid", ""}, // M6
		{"terminalfc", "Terminal FC", ""},
	}
	if c.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", c.Len(), len(want))
	}
	for i, w := range want {
		e := c.Entry(i)
		if e.ID != w.id || e.Name != w.name || e.Milestone != w.milestone {
			t.Errorf("entry %d = %s/%s/%s, want %s/%s/%s", i, e.ID, e.Name, e.Milestone, w.id, w.name, w.milestone)
		}
		switch {
		case w.milestone == "" && (!e.Available() || e.Status() != "Available"):
			t.Errorf("%s must be available, status %q", e.ID, e.Status())
		case w.milestone != "" && (e.Available() || e.Status() != "Coming soon ("+w.milestone+")"):
			t.Errorf("%s must be coming soon, status %q", e.ID, e.Status())
		}
		if got, ok := c.Lookup(w.id); !ok || got.Name != w.name {
			t.Errorf("Lookup(%q) = %v, %v", w.id, got.Name, ok)
		}
	}
	for _, id := range []string{"", "Snake", "tetris", "list"} {
		if _, ok := c.Lookup(id); ok {
			t.Errorf("Lookup(%q) succeeded", id)
		}
	}
}

func TestBuiltinFactoriesBuildFreshGames(t *testing.T) {
	c := Builtin()
	for i := range c.Len() {
		e := c.Entry(i)
		a, b := e.New(), e.New()
		if a == nil || a == b {
			t.Fatalf("%s: each launch must build a new game", e.ID)
		}
		if w, h := a.MinimumSize(); w != 80 || h != 24 {
			t.Fatalf("%s: minimum size %dx%d", e.ID, w, h)
		}
		if _, ok := a.(engine.Finisher); !ok {
			t.Fatalf("%s: must implement engine.Finisher so its end screen cannot be paused", e.ID)
		}
		if _, ok := a.(interface{ Score() int }); !ok {
			t.Fatalf("%s: must expose its score for personal bests and leaderboard submissions", e.ID)
		}
	}
}

func TestNewCatalogValidation(t *testing.T) {
	game := func() engine.Game { return nil }
	ok := Entry{ID: "ok", Name: "OK", Description: "d", Milestone: "M9"}
	tests := []struct {
		name  string
		entry Entry
		want  string
	}{
		{"empty ID", Entry{Name: "X", Description: "d", Milestone: "M9"}, "invalid ID"},
		{"upper-case ID", Entry{ID: "Snake", Name: "X", Description: "d", Milestone: "M9"}, "invalid ID"},
		{"ID with space", Entry{ID: "block drop", Name: "X", Description: "d", Milestone: "M9"}, "invalid ID"},
		{"duplicate ID", ok, "duplicate"},
		{"no name", Entry{ID: "x", Description: "d", Milestone: "M9"}, "required"},
		{"no description", Entry{ID: "x", Name: "X", Milestone: "M9"}, "required"},
		{"long description", Entry{ID: "x", Name: "X", Description: strings.Repeat("d", 73), Milestone: "M9"}, "longer"},
		{"available with milestone", Entry{ID: "x", Name: "X", Description: "d", Milestone: "M9", New: game}, "either"},
		{"unavailable without milestone", Entry{ID: "x", Name: "X", Description: "d"}, "either"},
	}
	for _, tt := range tests {
		if _, err := NewCatalog(ok, tt.entry); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestNewCatalogKeepsOrderAndOwnsItsEntries(t *testing.T) {
	entries := []Entry{
		{ID: "b", Name: "B", Description: "d", Milestone: "M9"},
		{ID: "a", Name: "A", Description: "d", New: func() engine.Game { return nil }},
	}
	c, err := NewCatalog(entries...)
	if err != nil {
		t.Fatal(err)
	}
	entries[0].ID = "mutated"
	if c.Entry(0).ID != "b" || c.Entry(1).ID != "a" || !c.Entry(1).Available() || c.Entry(1).Status() != "Available" {
		t.Fatalf("catalog = %+v, %+v", c.Entry(0), c.Entry(1))
	}
}
