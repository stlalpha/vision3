//go:build !windows
// +build !windows

package tea

// prepareOutput is a no-op on non-Windows platforms; ANSI escape sequences
// are handled natively by the terminal without any translation layer.
func (p *Program) prepareOutput() {}
