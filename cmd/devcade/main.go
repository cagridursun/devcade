// Command devcade is a terminal arcade for developers: an arcade menu over
// Snake, Block Drop, Maze Chase, Blast Grid and Space Shooter, plus a terminal diagnostic.
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

	"github.com/cagridursun/devcade/internal/arcade"
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/probe"
	"github.com/cagridursun/devcade/internal/terminal"
)

// version is overridden at release time with -ldflags "-X main.version=...".
var version = "0.1.0-dev"

const usage = `DevCade - a terminal arcade for developers.

Usage:
  devcade                 open the arcade menu
  devcade list            list the games and whether they are playable
  devcade <game>          open a game's New game / Leaderboard menu
  devcade --diagnostic    start the terminal diagnostic directly
  devcade --help          show this help
  devcade --version       show the version

Games (IDs for "devcade <game>"): snake, blockdrop, mazechase, blastgrid, spaceshooter.
The terminal diagnostic moves an '@' around a box to check input, timing,
resize and terminal restoration. Everything interactive needs a terminal of
at least 80x24.

Menu:
  Up/Down or W/S          select a game
  Enter                   open the game menu or highlighted item
  O                       open Settings (language, palette, username, sharing)
  D                       open the terminal diagnostic
  Q, Esc, Ctrl+C          quit

In every game:
  Space                   pause / resume
  Enter                   play again after game over or a win
  Q, Esc                  back to the game menu, then main menu
  Ctrl+C                  quit DevCade

Snake:       arrows / WASD turn (up to two turns are queued)
Block Drop:  Left/Right (A/D) move, Up (W) rotate clockwise,
             Z rotate counterclockwise, Down (S) soft drop, Enter hard drop
Maze Chase:  arrows / WASD steer (a turn waits for an opening)
Blast Grid:  arrows / WASD move one cell, Z place a bomb

Terminal diagnostic:
  arrows / WASD           change direction
  Space                   pause / resume
  Q, Esc                  back to the menu (quit when started with --diagnostic)
  Ctrl+C                  quit DevCade
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

// catalog, play and requireTerminal are replaced in tests.
var (
	catalog         = arcade.Builtin()
	play            = terminal.Run
	requireTerminal = func() error { return terminal.RequireTerminal(os.Stdin, os.Stdout) }
)

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("devcade", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	showVersion := flags.Bool("version", false, "")
	diagnostic := flags.Bool("diagnostic", false, "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return exitOK
		}
		return usageError(stderr, err.Error())
	}
	rest := flags.Args()
	switch {
	case *showVersion && (*diagnostic || len(rest) > 0):
		return usageError(stderr, "--version cannot be combined with other arguments")
	case *showVersion:
		fmt.Fprintf(stdout, "devcade %s\n", version)
		return exitOK
	case *diagnostic && len(rest) > 0:
		return usageError(stderr, "--diagnostic cannot be combined with a command")
	case len(rest) > 1:
		return usageError(stderr, fmt.Sprintf("unexpected argument %q", rest[1]))
	}

	// newProgram runs only after the terminal checks pass, so no game is
	// constructed for a run that cannot start.
	var newProgram func() terminal.Program
	switch {
	case *diagnostic:
		newProgram = newConfiguredDiagnostic
	case len(rest) == 0:
		newProgram = func() terminal.Program { return newArcade("") }
	case rest[0] == "list":
		printList(stdout)
		return exitOK
	default:
		entry, ok := catalog.Lookup(rest[0])
		if !ok {
			return usageError(stderr, fmt.Sprintf("unknown command or game %q", rest[0]))
		}
		if !entry.Available() {
			fmt.Fprintf(stderr, "devcade: %s is not available yet (planned for %s).\n"+
				"Run 'devcade list' to see which games are playable.\n", entry.Name, entry.Milestone)
			return exitUsage
		}
		newProgram = func() terminal.Program { return newArcade(entry.ID) }
	}

	if err := requireTerminal(); err != nil {
		fmt.Fprintf(stderr, "devcade: %v\nRun devcade directly in a terminal window, without pipes or redirection.\n", err)
		return exitError
	}
	ctx, stop := withSignals(context.Background())
	defer stop()
	program := newProgram()
	if closer, ok := program.(interface{ Close() }); ok {
		defer closer.Close()
	}
	err := play(ctx, program)
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

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "devcade: %s\nRun 'devcade --help' for usage.\n", msg)
	return exitUsage
}

// printList writes the catalog as plain text. It never constructs a game.
func printList(w io.Writer) {
	fmt.Fprintf(w, "%-12s %-12s %s\n", "ID", "GAME", "STATUS")
	for i := range catalog.Len() {
		e := catalog.Entry(i)
		fmt.Fprintf(w, "%-12s %-12s %s\n", e.ID, e.Name, e.Status())
	}
	fmt.Fprintln(w, "\nTool: devcade --diagnostic   terminal diagnostic (moving @)")
}

type signalCause struct{ os.Signal }

func (s signalCause) Error() string { return "received " + s.Signal.String() }

// withSignals cancels the context on interrupt or termination. On Windows,
// Go reports Ctrl+C/Ctrl+Break as os.Interrupt and console close, logoff and
// shutdown as SIGTERM. In raw mode a Ctrl+C keypress arrives as a key event
// instead, which the loop handles as a normal exit.
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

func newDiagnostic() engine.Game { return probe.New() }
