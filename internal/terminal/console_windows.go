//go:build windows

package terminal

import "golang.org/x/sys/windows"

// checkConsole rejects consoles that cannot process VT output before tcell
// touches them. See probeVT.
func checkConsole() error { return probeVT(windowsConsole) }

var windowsConsole = consoleOps{
	openOutput: func() (uintptr, error) {
		name, err := windows.UTF16PtrFromString("CONOUT$")
		if err != nil {
			return 0, err
		}
		// The same device tcell opens; GetConsoleMode/SetConsoleMode need
		// read and write access.
		h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		return uintptr(h), err
	},
	close: func(h uintptr) error { return windows.CloseHandle(windows.Handle(h)) },
	mode: func(h uintptr) (uint32, error) {
		var m uint32
		err := windows.GetConsoleMode(windows.Handle(h), &m)
		return m, err
	},
	setMode: func(h uintptr, m uint32) error { return windows.SetConsoleMode(windows.Handle(h), m) },
}

// The local constant must match the Windows SDK value.
var _ = [1]struct{}{}[vtProcessing^windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING]
