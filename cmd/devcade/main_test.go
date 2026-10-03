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

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/terminal"
)

// TestMain lets tests re-run this binary as the real devcade command.
func TestMain(m *testing.M) {
	if os.Getenv("DEVCADE_RUN_MAIN") == "1" {
		main()
	}
	os.Exit(m.Run())
}

// stub replaces the terminal hooks so a test can never take over the real
// terminal, and records whether the game would have started.
func stub(t *testing.T, termErr error, playErr error) *bool {
	t.Helper()
	played := false
	oldPlay, oldReq := play, requireTerminal
	t.Cleanup(func() { play, requireTerminal = oldPlay, oldReq })
	requireTerminal = func() error { return termErr }
	play = func(context.Context, *engine.Engine) error { played = true; return playErr }
	return &played
}

func runArgs(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestHelpAndVersionNeedNoTerminal(t *testing.T) {
	played := stub(t, terminal.ErrNotInteractive, nil)
	for _, args := range [][]string{{"--help"}, {"-h"}, {"-help"}} {
		code, out, errOut := runArgs(args...)
		if code != exitOK || !strings.Contains(out, "Usage:") || errOut != "" {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
	for _, args := range [][]string{{"--version"}, {"-version"}} {
		code, out, _ := runArgs(args...)
		if code != exitOK || out != "devcade "+version+"\n" {
			t.Errorf("%v: code=%d stdout=%q", args, code, out)
		}
	}
	if *played {
		t.Fatal("help/version started the terminal")
	}
}

func TestInvalidArgumentsAreUsageErrors(t *testing.T) {
	played := stub(t, nil, nil)
	for _, args := range [][]string{{"snake"}, {"--bogus"}, {"--version=maybe"}, {"--version", "extra"}} {
		code, out, errOut := runArgs(args...)
		if code != exitUsage || out != "" || !strings.Contains(errOut, "--help") {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, out, errOut)
		}
	}
	if *played {
		t.Fatal("invalid arguments started the terminal")
	}
}

func TestNonInteractiveFailsBeforeTouchingTerminal(t *testing.T) {
	played := stub(t, terminal.ErrNotInteractive, nil)
	code, _, errOut := runArgs()
	if code != exitError || !strings.Contains(errOut, "interactive terminal") || *played {
		t.Fatalf("code=%d stderr=%q played=%v", code, errOut, *played)
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
	if played := stub(t, nil, nil); func() bool { code, _, _ := runArgs(); return code != exitOK || !*played }() {
		t.Fatal("normal quit should exit 0")
	}
}

// TestRedirectedProcessFailsPromptly runs the real binary with piped stdio
// and no terminal hooks replaced.
func TestRedirectedProcessFailsPromptly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0]) // TestMain calls main(), which exits
	cmd.Env = append(os.Environ(), "DEVCADE_RUN_MAIN=1")
	cmd.Stdin = strings.NewReader("q")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatal("redirected devcade hung")
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != exitError {
		t.Fatalf("err=%v stdout=%q stderr=%q", err, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "interactive terminal") || out.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", out.String(), errOut.String())
	}
}
