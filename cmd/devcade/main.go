package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/games/probe"
	"github.com/cagridursun/devcade/internal/terminal"
	"golang.org/x/term"
)

var version = "0.1.0-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "devcade:", err)
		os.Exit(1)
	}
}

func run() (err error) {
	// Screen cleanup in terminal.Run unwinds before this recovery reports an error.
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("unexpected failure: %v", value)
		}
	}()
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("DevCade", version)
		return nil
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("no games available yet; run devcade without arguments for the terminal probe")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("interactive terminal required on stdin and stdout")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return terminal.Run(ctx, engine.New(probe.New()))
}
