//go:build !windows

package terminal

// checkConsole is a no-op off Windows: POSIX terminals are driven through
// terminfo, and tcell reports unusable terminals from Init.
func checkConsole() error { return nil }
