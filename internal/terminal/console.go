package terminal

import (
	"errors"
	"fmt"
)

// ErrNoVT is returned when the console cannot interpret VT escape sequences,
// which tcell's Windows backend writes unconditionally.
var ErrNoVT = errors.New("this console does not support VT escape sequences; " +
	"use Windows Terminal, or Windows 10 version 1809 or newer")

// vtProcessing is ENABLE_VIRTUAL_TERMINAL_PROCESSING, the console output mode
// flag that turns on VT sequence handling.
const vtProcessing = 0x0004

// consoleOps are the console calls used by probeVT, injectable for tests.
// Handles are Windows HANDLE values.
type consoleOps struct {
	openOutput func() (uintptr, error) // the CONOUT$ console tcell will drive
	close      func(uintptr) error
	mode       func(uintptr) (uint32, error)
	setMode    func(uintptr, uint32) error
}

// probeVT reports ErrNoVT when VT output processing cannot be enabled on the
// console output. It leaves the console mode as it found it on every path:
// when it enables VT processing to test it, it restores the original mode.
//
// Why a preflight: tcell v2.13.10 on Windows drives the console through
// winTty, which sets VT mode without checking the result, so an unsupported
// console would start "successfully" and print raw escape sequences. Its
// legacy cScreen backend does check, but returns that error with its mutex
// still held, so a later Fini deadlocks. Rejecting the console here means
// neither backend Init runs on an unsupported console.
func probeVT(ops consoleOps) (err error) {
	h, err := ops.openOutput()
	if err != nil {
		return fmt.Errorf("open console output: %w", err)
	}
	defer func() {
		if cerr := ops.close(h); cerr != nil && err == nil {
			err = fmt.Errorf("close console output: %w", cerr)
		}
	}()

	orig, err := ops.mode(h)
	if err != nil {
		return fmt.Errorf("read console mode: %w", err)
	}
	if orig&vtProcessing != 0 {
		return nil // already on; nothing changed
	}
	if err := ops.setMode(h, orig|vtProcessing); err != nil {
		// Consoles without VT support reject the flag and keep their mode.
		return fmt.Errorf("%w (%v)", ErrNoVT, err)
	}
	got, verr := ops.mode(h)
	if rerr := ops.setMode(h, orig); rerr != nil {
		return fmt.Errorf("restore console mode: %w", rerr)
	}
	switch {
	case verr != nil:
		return fmt.Errorf("verify console mode: %w", verr)
	case got&vtProcessing == 0:
		return ErrNoVT
	}
	return nil
}
