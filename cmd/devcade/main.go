// Command devcade is a terminal arcade for developers. In milestone M1 it
// launches a terminal diagnostic; the games arrive in later milestones.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/probe"
	"github.com/cagridursun/devcade/internal/terminal"
)

// version is overridden at release time with -ldflags "-X main.version=...".
var version = "0.1.0-dev"

const usage = `DevCade - a terminal arcade for developers.

Usage:
  devcade              start the terminal diagnostic (M1)
  devcade --help       show this help
  devcade --version    show the version

No games are playable yet. The diagnostic moves an '@' around a box to check
input, timing, resize and terminal restoration. It needs an interactive
terminal of at least 80x24.

Controls:
  arrows / WASD        change direction
  Space                pause / resume
  Q, Esc, Ctrl+C       quit
`

// Exit statuses.
const (
	exitOK        = 0
	exitError     = 1
	exitUsage     = 2
	exitInterrupt = 130 // conventional 128+SIGINT
	exitTerm      = 143 // conventional 128+SIGTERM
)

func main() {
	// os.Exit runs only after run has returned and its terminal cleanup is done.
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// play and requireTerminal are replaced in tests.
var (
	play            = terminal.Run
	requireTerminal = func() error { return terminal.RequireTerminal(os.Stdin, os.Stdout) }
)

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("devcade", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	showVersion := flags.Bool("version", false, "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return exitOK
		}
		fmt.Fprintf(stderr, "devcade: %v\nRun 'devcade --help' for usage.\n", err)
		return exitUsage
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "devcade: unknown command %q (no games are playable yet)\nRun 'devcade --help' for usage.\n", flags.Arg(0))
		return exitUsage
	}
	if *showVersion {
		fmt.Fprintf(stdout, "devcade %s\n", version)
		return exitOK
	}
	if err := requireTerminal(); err != nil {
		fmt.Fprintf(stderr, "devcade: %v\nRun devcade directly in a terminal window, without pipes or redirection.\n", err)
		return exitError
	}

	ctx, stop := withSignals(context.Background())
	defer stop()
	err := play(ctx, engine.New(probe.New()))
	// The terminal is restored by now, so diagnostics are safe to print.
	var panicErr *terminal.PanicError
	switch {
	case errors.As(err, &panicErr):
		fmt.Fprintf(stderr, "devcade: %v\n\n%s", panicErr, panicErr.Stack)
		return exitError
	case err != nil:
		fmt.Fprintf(stderr, "devcade: %v\n", err)
		return exitError
	}
	var sig signalCause
	if errors.As(context.Cause(ctx), &sig) {
		if sig.Signal == syscall.SIGTERM {
			return exitTerm
		}
		return exitInterrupt
	}
	return exitOK
}

type signalCause struct{ os.Signal }

func (s signalCause) Error() string { return "received " + s.Signal.String() }

// withSignals cancels the context on interrupt or termination. On Windows,
// Go reports Ctrl+C/Ctrl+Break as os.Interrupt and console close, logoff and
// shutdown as SIGTERM. In raw mode a Ctrl+C keypress arrives as a key event
// instead, which the loop handles as a normal quit.
func withSignals(parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-signals:
			cancel(signalCause{sig})
		case <-done:
		}
	}()
	return ctx, func() {
		signal.Stop(signals)
		close(done)
		cancel(nil)
	}
}
