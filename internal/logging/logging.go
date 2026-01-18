// Package logging provides debug logging utilities for the vision3 BBS.
package logging

import "log"

// DebugEnabled controls whether Debug() produces output.
// Set via -debug flag or DEBUG=1 environment variable.
var DebugEnabled bool

// Debug logs a message only when DebugEnabled is true.
func Debug(format string, args ...any) {
	if DebugEnabled {
		log.Printf("DEBUG: "+format, args...)
	}
}
