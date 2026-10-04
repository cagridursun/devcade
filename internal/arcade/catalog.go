// Package arcade holds the built-in game catalog and the menu that navigates
// it. It wires games to the engine; game packages never import it.
package arcade

import (
	"fmt"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/blastgrid"
	"github.com/cagridursun/devcade/internal/games/blockdrop"
	"github.com/cagridursun/devcade/internal/games/mazechase"
	"github.com/cagridursun/devcade/internal/games/snake"
)

// Entry describes one built-in game. An entry is playable exactly when New
// is set; otherwise Milestone names the release that will deliver it.
type Entry struct {
	ID          string // stable command-line name, e.g. "snake"
	Name        string
	Description string             // one line, at most 72 characters
	Milestone   string             // planned milestone for an unavailable entry
	New         func() engine.Game // fresh game per launch; nil if unavailable
}

// Available reports whether the entry can be launched.
func (e Entry) Available() bool { return e.New != nil }

// Status is the availability text shown in the menu and by "devcade list".
func (e Entry) Status() string {
	if e.Available() {
		return "Available"
	}
	return "Coming soon (" + e.Milestone + ")"
}

// Catalog is an ordered, validated list of entries. It is the single source
// for the menu, "devcade list" and launching a game by ID.
type Catalog struct {
	entries []Entry
}

// NewCatalog validates entries and keeps their order. IDs must be unique,
// non-empty lower-case ASCII words; every entry needs a name and a short
// description; an unavailable entry must name its milestone and an available
// one must not.
func NewCatalog(entries ...Entry) (Catalog, error) {
	seen := make(map[string]bool, len(entries))
	for i, e := range entries {
		switch {
		case !validID(e.ID):
			return Catalog{}, fmt.Errorf("entry %d: invalid ID %q", i, e.ID)
		case seen[e.ID]:
			return Catalog{}, fmt.Errorf("duplicate ID %q", e.ID)
		case e.Name == "" || e.Description == "":
			return Catalog{}, fmt.Errorf("%s: name and description are required", e.ID)
		case len(e.Description) > 72:
			return Catalog{}, fmt.Errorf("%s: description longer than 72 characters", e.ID)
		case e.Available() == (e.Milestone != ""):
			return Catalog{}, fmt.Errorf("%s: set either a factory or a planned milestone", e.ID)
		}
		seen[e.ID] = true
	}
	return Catalog{entries: append([]Entry(nil), entries...)}, nil
}

func validID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// Len returns the number of entries.
func (c Catalog) Len() int { return len(c.entries) }

// Entry returns the i-th entry in display order.
func (c Catalog) Entry(i int) Entry { return c.entries[i] }

// Lookup finds an entry by ID.
func (c Catalog) Lookup(id string) (Entry, bool) {
	for _, e := range c.entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Builtin returns the games that ship with DevCade, in menu order. A planned
// game names its milestone; the milestone that delivers it sets New instead.
func Builtin() Catalog {
	c, err := NewCatalog(
		Entry{ID: "snake", Name: "Snake", New: snake.New,
			Description: "Steer a growing snake to food without hitting walls or yourself."},
		Entry{ID: "blockdrop", Name: "Block Drop", New: blockdrop.New,
			Description: "Rotate falling blocks and clear full rows before the stack tops out."},
		Entry{ID: "mazechase", Name: "Maze Chase", New: mazechase.New,
			Description: "Clear the maze of dots, dodge four chasers, power up to eat them."},
		Entry{ID: "blastgrid", Name: "Blast Grid", New: blastgrid.New,
			Description: "Bomb crates and outlast three bots in a fixed 17x13 blast arena."},
	)
	if err != nil {
		panic(err) // a programming error in this table; covered by tests
	}
	return c
}
