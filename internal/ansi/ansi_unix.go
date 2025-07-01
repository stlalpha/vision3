//go:build !windows

package ansi

// setConsoleMode is a no-op on non-Windows systems.
func setConsoleMode() error {
	// Assume ANSI/VT100 is handled by the terminal emulator (like macOS Terminal, iTerm2, Linux terminals)
	return nil
}
