package arcade

import (
	"strings"
	"testing"

	"github.com/cagridursun/devcade/internal/engine"
)

func TestBuiltinCatalog(t *testing.T) {
	c := Builtin()
	want := []struct{ id, name, milestone string }{
		{"snake", "Snake", "M3"},
		{"blockdrop", "Block Drop", "M4"},
		{"mazechase", "Maze Chase", "M5"},
		{"blastgrid", "Blast Grid", "M6"},
	}
	if c.Len() != len(want) {
		t.Fatalf("Len = %d, want %d", c.Len(), len(want))
	}
	for i, w := range want {
		e := c.Entry(i)
		if e.ID != w.id || e.Name != w.name || e.Milestone != w.milestone {
			t.Errorf("entry %d = %s/%s/%s, want %s/%s/%s", i, e.ID, e.Name, e.Milestone, w.id, w.name, w.milestone)
		}
		if e.Available() || e.New != nil || e.Status() != "Coming soon ("+w.milestone+")" {
			t.Errorf("%s must be an unavailable entry, status %q", e.ID, e.Status())
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
