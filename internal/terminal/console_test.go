package terminal

import (
	"errors"
	"testing"
)

// fakeConsole models the console state probeVT touches: one output mode, the
// handles it opened, and injectable failures.
type fakeConsole struct {
	mode       uint32
	vt         bool // whether the console accepts VT processing
	rejectVT   bool // SetConsoleMode fails instead of silently dropping the flag
	open       int  // currently open handles
	opens      int
	sets       int
	failOpen   bool
	failRead   int // fail the Nth mode read (1-based); 0 never
	failRestor bool
	failClose  bool
	reads      int
}

var errFake = errors.New("fake console failure")

func (c *fakeConsole) ops() consoleOps {
	return consoleOps{
		openOutput: func() (uintptr, error) {
			if c.failOpen {
				return 0, errFake
			}
			c.open++
			c.opens++
			return 42, nil
		},
		close: func(uintptr) error {
			c.open--
			if c.failClose {
				return errFake
			}
			return nil
		},
		mode: func(uintptr) (uint32, error) {
			c.reads++
			if c.reads == c.failRead {
				return 0, errFake
			}
			return c.mode, nil
		},
		setMode: func(_ uintptr, m uint32) error {
			c.sets++
			restoring := m&vtProcessing == 0
			if restoring && c.failRestor {
				return errFake
			}
			if !c.vt && !restoring {
				if c.rejectVT {
					return errFake
				}
				m &^= vtProcessing // legacy consoles may ignore the flag
			}
			c.mode = m
			return nil
		},
	}
}

const legacyMode = 0x0003 // processed output + wrap at EOL, VT off

func TestProbeVT(t *testing.T) {
	tests := []struct {
		name     string
		console  fakeConsole
		wantErr  error // nil, ErrNoVT or errFake
		wantMode uint32
		wantSets int
	}{
		{"VT already enabled", fakeConsole{mode: legacyMode | vtProcessing, vt: true}, nil, legacyMode | vtProcessing, 0},
		{"VT supported, restored to off", fakeConsole{mode: legacyMode, vt: true}, nil, legacyMode, 2},
		{"VT rejected by SetConsoleMode", fakeConsole{mode: legacyMode, rejectVT: true}, ErrNoVT, legacyMode, 1},
		{"VT silently ignored", fakeConsole{mode: legacyMode}, ErrNoVT, legacyMode, 2},
		{"open fails", fakeConsole{mode: legacyMode, vt: true, failOpen: true}, errFake, legacyMode, 0},
		{"read fails", fakeConsole{mode: legacyMode, vt: true, failRead: 1}, errFake, legacyMode, 0},
		{"verify read fails", fakeConsole{mode: legacyMode, vt: true, failRead: 2}, errFake, legacyMode, 2},
		{"restore fails", fakeConsole{mode: legacyMode, vt: true, failRestor: true}, errFake, legacyMode | vtProcessing, 2},
		{"close fails", fakeConsole{mode: legacyMode, vt: true, failClose: true}, errFake, legacyMode, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.console
			err := probeVT(c.ops())
			if tt.wantErr == nil && err != nil || tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if c.mode != tt.wantMode {
				t.Errorf("console mode = %#x, want %#x", c.mode, tt.wantMode)
			}
			if c.sets != tt.wantSets {
				t.Errorf("SetConsoleMode calls = %d, want %d", c.sets, tt.wantSets)
			}
			if c.open != 0 {
				t.Errorf("%d console handle(s) left open", c.open)
			}
		})
	}
}
