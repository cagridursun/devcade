//go:build windows

package terminal

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestWindowsNewScreenUsesTerminfoBackend pins the backend tcell selects on
// Windows. releaseAfterFailedInit is only safe for this backend; if a tcell
// upgrade starts returning the legacy cScreen, its locked-Init failure path
// must be re-reviewed. NewScreen does not touch the console for this backend.
func TestWindowsNewScreenUsesTerminfoBackend(t *testing.T) {
	s, err := tcell.NewScreen()
	if err != nil {
		t.Fatal(err)
	}
	impl := reflect.ValueOf(s).Elem().FieldByName("screenImpl")
	if !impl.IsValid() {
		t.Fatal("tcell internals changed; re-review backend selection")
	}
	if got := impl.Elem().Type().String(); got != "*tcell.tScreen" {
		t.Fatalf("NewScreen backend = %s, want *tcell.tScreen", got)
	}
}

// TestRealConsoleCheckLeavesModeUnchanged runs the real probe against the
// process console, when there is one, and compares the mode before and after.
func TestRealConsoleCheckLeavesModeUnchanged(t *testing.T) {
	h, err := windowsConsole.openOutput()
	if err != nil {
		t.Skipf("no console attached to this test process: %v", err)
	}
	defer windowsConsole.close(h)
	before, err := windowsConsole.mode(h)
	if err != nil {
		t.Skipf("CONOUT$ is not a console here: %v", err)
	}
	checkErr := checkConsole()
	after, err := windowsConsole.mode(h)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("console mode %#x, VT already on: %v, check result: %v", before, before&vtProcessing != 0, checkErr)
	if before != after {
		t.Fatalf("console mode changed from %#x to %#x", before, after)
	}
	if checkErr != nil {
		t.Fatalf("modern Windows console rejected: %v", checkErr)
	}
}
