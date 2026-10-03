package terminal

import (
	"errors"
	"os"
	"testing"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/gdamore/tcell/v2"
)

func TestKeyNormalization(t *testing.T) {
	tests := []struct {
		key  tcell.Key
		r    rune
		mod  tcell.ModMask
		want engine.Key
	}{
		{tcell.KeyUp, 0, 0, engine.KeyUp},
		{tcell.KeyDown, 0, 0, engine.KeyDown},
		{tcell.KeyLeft, 0, 0, engine.KeyLeft},
		{tcell.KeyRight, 0, 0, engine.KeyRight},
		{tcell.KeyRune, 'w', 0, engine.KeyUp},
		{tcell.KeyRune, 'W', tcell.ModShift, engine.KeyUp},
		{tcell.KeyRune, 'a', 0, engine.KeyLeft},
		{tcell.KeyRune, 'A', 0, engine.KeyLeft},
		{tcell.KeyRune, 's', 0, engine.KeyDown},
		{tcell.KeyRune, 'S', 0, engine.KeyDown},
		{tcell.KeyRune, 'd', 0, engine.KeyRight},
		{tcell.KeyRune, 'D', 0, engine.KeyRight},
		{tcell.KeyRune, ' ', 0, engine.KeyPause},
		{tcell.KeyRune, 'q', 0, engine.KeyQuit},
		{tcell.KeyRune, 'Q', 0, engine.KeyQuit},
		{tcell.KeyEscape, 0, 0, engine.KeyQuit},
		{tcell.KeyCtrlC, 0, tcell.ModCtrl, engine.KeyQuit},
		{tcell.KeyRune, 3, 0, engine.KeyQuit}, // raw ETX byte
		{tcell.KeyRune, 'c', tcell.ModCtrl, engine.KeyQuit},
		{tcell.KeyRune, 'w', tcell.ModCtrl, engine.KeyNone},
		{tcell.KeyRune, 'x', 0, engine.KeyNone},
		{tcell.KeyEnter, 0, 0, engine.KeyNone},
		{tcell.KeyF1, 0, 0, engine.KeyNone},
	}
	for _, tt := range tests {
		ev := tcell.NewEventKey(tt.key, tt.r, tt.mod)
		if got := keyOf(ev); got != tt.want {
			t.Errorf("key %v rune %q mod %v: got %v, want %v", tt.key, tt.r, tt.mod, got, tt.want)
		}
	}
}

func TestCanvasClipsAndKeepsOneCellPerRune(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(10, 2)
	c := canvas{screen: s, width: 10, height: 2}
	c.Text(-2, 0, "xyAB", engine.Default)    // clipped on the left
	c.Text(4, 0, "é界z", engine.Accent)       // non-ASCII replaced
	c.Text(8, 1, "long text", engine.Player) // clipped on the right
	c.Cell(5, 5, '#', engine.Default)        // off-grid: ignored
	s.Show()
	want := []string{"AB  ??z   ", "        lo"}
	cells, w, _ := s.GetContents()
	for y, line := range want {
		for x, r := range line {
			got := string(cells[y*w+x].Bytes)
			if got == "" {
				got = " "
			}
			if got != string(r) {
				t.Errorf("cell %d,%d = %q, want %q", x, y, got, r)
			}
		}
	}
}

func TestReaderStopDrainsWhileFiniRuns(t *testing.T) {
	src := make(chan tcell.Event) // unbuffered: like tcell's blocked internal post
	finalizing := make(chan struct{})
	poll := func() tcell.Event {
		select {
		case ev := <-src:
			return ev
		case <-finalizing:
			return nil
		}
	}
	r := startReader(poll)
	// Fill the forwarding buffer so the reader is blocked on delivery.
	for range eventBuffer + 1 {
		src <- tcell.NewEventKey(tcell.KeyRune, 'w', 0)
	}
	r.stop(func() {
		close(finalizing)
		// Backend goroutines still post while Fini waits for them. Without
		// draining, these sends would block forever.
		for range 2 * eventBuffer {
			src <- tcell.NewEventKey(tcell.KeyRune, 'w', 0)
		}
	})
	select {
	case <-r.done:
	default:
		t.Fatal("reader still running after stop")
	}
}

func TestRequireTerminalRejectsPipes(t *testing.T) {
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()
	defer wr.Close()
	if err := RequireTerminal(rd, wr); !errors.Is(err, ErrNotInteractive) {
		t.Fatalf("err = %v", err)
	}
}
