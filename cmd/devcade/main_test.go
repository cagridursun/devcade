package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/cagridursun/devcade/internal/arcade"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/probe"
	"github.com/cagridursun/devcade/internal/terminal"
)

// TestMain lets tests re-run this binary as the real devcade command.
func TestMain(m *testing.M) {
	if os.Getenv("DEVCADE_RUN_MAIN") == "1" {
		main()
	}
	os.Exit(m.Run())
}

// hooks records what run asked of the terminal. The stubs guarantee a test
// can never take over the real terminal or probe the console.
type hooks struct {
	checks  int              // requireTerminal calls
	plays   int              // play calls
	program terminal.Program // what play was given
}

func stub(t *testing.T, termErr error, playErr error) *hooks {
	t.Helper()
	t.Setenv("DEVCADE_CONFIG_DIR", t.TempDir())
	t.Setenv("DEVCADE_LEADERBOARD_URL", "")
	h := &hooks{}
	oldPlay, oldReq, oldCat := play, requireTerminal, catalog
	t.Cleanup(func() { play, requireTerminal, catalog = oldPlay, oldReq, oldCat })
	requireTerminal = func() error { h.checks++; return termErr }
	play = func(_ context.Context, p terminal.Program) error {
		h.plays++
		h.program = p
		return playErr
	}
	return h
}

func runArgs(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// testGame is a minimal available game for proving the launch path.
type testGame struct{ *probe.Probe }

func testCatalog(t *testing.T, factoryCalls *int) arcade.Catalog {
	t.Helper()
	c, err := arcade.NewCatalog(
		arcade.Entry{ID: "testgame", Name: "Test Game", Description: "Test-only game.",
			New: func() engine.Game { *factoryCalls++; return testGame{probe.New()} }},
		arcade.Entry{ID: "later", Name: "Later", Description: "Not yet.", Milestone: "M9"},
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHelpVersionAndListNeedNoTerminal(t *testing.T) {
	h := stub(t, terminal.ErrNotInteractive, nil)
	for _, args := range [][]string{{"--help"}, {"-h"}, {"-help"}} {
		code, out, errOut := runArgs(args...)
		if code != exitOK || !strings.Contains(out, "Usage:") || !strings.Contains(out, "--diagnostic") || errOut != "" {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
	for _, args := range [][]string{{"--version"}, {"-version"}} {
		code, out, _ := runArgs(args...)
		if code != exitOK || out != "devcade "+version+"\n" {
			t.Errorf("%v: code=%d stdout=%q", args, code, out)
		}
	}
	code, out, errOut := runArgs("list")
	if code != exitOK || errOut != "" {
		t.Fatalf("list: code=%d stderr=%q", code, errOut)
	}
	for _, want := range []string{"snake", "Snake", "blockdrop", "Block Drop",
		"mazechase", "Maze Chase", "blastgrid", "Blast Grid", "spaceshooter", "Space Shooter", "--diagnostic"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output lacks %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "snake") > strings.Index(out, "blockdrop") ||
		strings.Index(out, "blockdrop") > strings.Index(out, "mazechase") ||
		strings.Index(out, "mazechase") > strings.Index(out, "blastgrid") {
		t.Errorf("list order is not the catalog order:\n%s", out)
	}
	if h.checks != 0 || h.plays != 0 {
		t.Fatalf("help/version/list touched the terminal: %+v", *h)
	}
}

func TestListNeverConstructsGames(t *testing.T) {
	h := stub(t, nil, nil)
	calls := 0
	catalog = testCatalog(t, &calls)
	code, out, _ := runArgs("list")
	if code != exitOK || calls != 0 || h.plays != 0 {
		t.Fatalf("code=%d factory calls=%d plays=%d", code, calls, h.plays)
	}
	if !strings.Contains(out, "testgame") || !strings.Contains(out, "Available") || !strings.Contains(out, "Coming soon (M9)") {
		t.Fatalf("list:\n%s", out)
	}
}

func TestAllBuiltinGamesAreListedAsAvailable(t *testing.T) {
	stub(t, nil, nil)
	_, out, _ := runArgs("list")
	if n := strings.Count(out, "Available"); n != catalog.Len() || strings.Contains(out, "Coming soon") {
		t.Fatalf("want all catalog games available:\n%s", out)
	}
}

func TestComingSoonGamesFailBeforeTerminalAccess(t *testing.T) {
	h := stub(t, nil, nil)
	calls := 0
	catalog = testCatalog(t, &calls) // v1 ships no coming-soon game; keep the path covered
	for _, id := range []string{"later"} {
		code, out, errOut := runArgs(id)
		if code != exitUsage || out != "" || !strings.Contains(errOut, "not available yet") || !strings.Contains(errOut, "devcade list") {
			t.Errorf("%s: code=%d stdout=%q stderr=%q", id, code, out, errOut)
		}
	}
	if h.checks != 0 || h.plays != 0 {
		t.Fatalf("coming-soon IDs touched the terminal: %+v", *h)
	}
}

func TestInvalidArgumentsAreUsageErrors(t *testing.T) {
	h := stub(t, nil, nil)
	for _, args := range [][]string{
		{"tetris"}, {"Snake"}, {"--bogus"}, {"--version=maybe"},
		{"--version", "extra"}, {"--version", "--diagnostic"}, {"--diagnostic", "--version"},
		{"--diagnostic", "snake"}, {"--diagnostic", "list"}, {"list", "extra"}, {"snake", "extra"},
		{"list", "--diagnostic"},
	} {
		code, out, errOut := runArgs(args...)
		if code != exitUsage || out != "" || !strings.Contains(errOut, "--help") {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
	if h.checks != 0 || h.plays != 0 {
		t.Fatalf("usage errors touched the terminal: %+v", *h)
	}
}

func TestDefaultLaunchOpensMenu(t *testing.T) {
	h := stub(t, nil, nil)
	if code, _, errOut := runArgs(); code != exitOK {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if _, ok := h.program.(*arcade.App); !ok || h.checks != 1 || h.plays != 1 {
		t.Fatalf("default launch played %T (%+v)", h.program, *h)
	}
}

func TestSnakeLaunchesDirectlyThroughCatalog(t *testing.T) {
	h := stub(t, nil, nil)
	if code, _, errOut := runArgs("snake"); code != exitOK {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	e, ok := h.program.(*arcade.App)
	if !ok || h.checks != 1 || h.plays != 1 {
		t.Fatalf("devcade snake played %T (%+v)", h.program, *h)
	}
	// Direct IDs open the same game menu, after optional username entry.
	e.Resize(80, 24)
	e.Input(engine.Event{Key: engine.KeyBack})
	e.Input(engine.Event{Key: engine.KeySelect})
	if e.Input(engine.Event{Key: engine.KeyBack}) {
		t.Fatal("Q should return to game menu")
	}
}

func TestDiagnosticFlagStartsDiagnosticDirectly(t *testing.T) {
	h := stub(t, nil, nil)
	if code, _, errOut := runArgs("--diagnostic"); code != exitOK {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	e, ok := h.program.(diagnosticProgram)
	if !ok || h.checks != 1 {
		t.Fatalf("--diagnostic played %T (%+v)", h.program, *h)
	}
	// Direct mode: Q/Esc end the program instead of revealing a menu.
	e.Resize(80, 24)
	for _, k := range []engine.Key{engine.KeyBack, engine.KeyExit} {
		if !e.Input(engine.Event{Key: k}) {
			t.Errorf("%v did not end direct diagnostic", k)
		}
	}
}

func TestAvailableGameLaunchesByIDThroughCatalog(t *testing.T) {
	h := stub(t, nil, nil)
	calls := 0
	catalog = testCatalog(t, &calls)
	var order []string
	requireTerminal = func() error { order = append(order, "check"); return nil }
	play = func(_ context.Context, p terminal.Program) error {
		order = append(order, "play")
		h.program = p
		return nil
	}
	if code, _, errOut := runArgs("testgame"); code != exitOK {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if _, ok := h.program.(*arcade.App); !ok || calls != 0 || strings.Join(order, ",") != "check,play" {
		t.Fatalf("program=%T factory calls=%d order=%v", h.program, calls, order)
	}

	app := h.program.(*arcade.App)
	app.Resize(80, 24)
	app.Input(engine.Event{Key: engine.KeyBack})
	app.Input(engine.Event{Key: engine.KeySelect})
	if calls != 1 {
		t.Fatal("confirmation did not construct game")
	}
	// A failed terminal check constructs nothing.
	calls = 0
	requireTerminal = func() error { return terminal.ErrNotInteractive }
	if code, _, _ := runArgs("testgame"); code != exitError || calls != 0 {
		t.Fatalf("code=%d factory calls=%d", code, calls)
	}
	if code, _, errOut := runArgs("later"); code != exitUsage || !strings.Contains(errOut, "M9") {
		t.Fatalf("later: code=%d stderr=%q", code, errOut)
	}
}

func TestNonInteractiveFailsBeforeTouchingTerminal(t *testing.T) {
	h := stub(t, terminal.ErrNotInteractive, nil)
	for _, args := range [][]string{{}, {"--diagnostic"}, {"snake"}} {
		code, _, errOut := runArgs(args...)
		if code != exitError || !strings.Contains(errOut, "interactive terminal") {
			t.Fatalf("%v: code=%d stderr=%q", args, code, errOut)
		}
	}
	if h.plays != 0 {
		t.Fatal("play called without a terminal")
	}
}

func TestRuntimeErrorsAreReportedAfterPlay(t *testing.T) {
	stub(t, nil, errors.New("initialize terminal: boom"))
	if code, _, errOut := runArgs(); code != exitError || !strings.Contains(errOut, "boom") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	stub(t, nil, &terminal.PanicError{Value: "bad state", Stack: []byte("goroutine 1 [running]:\n")})
	if code, _, errOut := runArgs(); code != exitError || !strings.Contains(errOut, "bad state") || !strings.Contains(errOut, "goroutine 1") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	h := stub(t, nil, nil)
	if code, _, _ := runArgs(); code != exitOK || h.plays != 1 {
		t.Fatal("normal quit should exit 0")
	}
}

func TestBuiltinIDsDoNotShadowCommands(t *testing.T) {
	for i := range arcade.Builtin().Len() {
		if id := arcade.Builtin().Entry(i).ID; id == "list" {
			t.Fatalf("catalog ID %q collides with a command", id)
		}
	}
}

// TestRedirectedProcessFailsPromptly runs the real binary with piped stdio
// and no terminal hooks replaced.
func TestRedirectedProcessFailsPromptly(t *testing.T) {
	for _, args := range [][]string{{}, {"--diagnostic"}, {"snake"}, {"blockdrop"}, {"mazechase"}, {"blastgrid"}, {"spaceshooter"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], args...) // TestMain calls main(), which exits
		cmd.Env = append(os.Environ(), "DEVCADE_RUN_MAIN=1")
		cmd.Stdin = strings.NewReader("q")
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		err := cmd.Run()
		hung := ctx.Err() != nil
		cancel()
		if hung {
			t.Fatalf("%v: redirected devcade hung", args)
		}
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != exitError {
			t.Fatalf("%v: err=%v stdout=%q stderr=%q", args, err, out.String(), errOut.String())
		}
		if !strings.Contains(errOut.String(), "interactive terminal") || out.Len() != 0 {
			t.Fatalf("%v: stdout=%q stderr=%q", args, out.String(), errOut.String())
		}
	}
}
