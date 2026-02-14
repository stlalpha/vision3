package jam

import (
	"fmt"
	"runtime"
	"strings"
)

// Version is the ViSiON/3 version string. Set via build flags:
//
//	-ldflags "-X github.com/stlalpha/vision3/internal/jam.Version=1.0.0"
var Version = "0.1.0"

// AddTearline appends a tearline to the message text.
// Format: "--- ViSiON/3 0.1.0/darwin"
func AddTearline(text string) string {
	return AddCustomTearline(text, "")
}

// AddCustomTearline appends a tearline to the message text.
// If tearline is empty, it uses the default ViSiON/3 tearline.
// If tearline already starts with "---", it is used as-is.
func AddCustomTearline(text, tearline string) string {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	trimmed := strings.TrimSpace(tearline)
	if trimmed == "" {
		trimmed = fmt.Sprintf("ViSiON/3 %s/%s", Version, runtime.GOOS)
	}
	if strings.HasPrefix(trimmed, "---") {
		return text + trimmed + "\n"
	}
	return text + fmt.Sprintf("--- %s\n", trimmed)
}

// AddOriginLine appends an origin line to the message text.
// Format: " * Origin: BBS Name (1:103/705)"
func AddOriginLine(text, systemName, address string) string {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + fmt.Sprintf(" * Origin: %s (%s)\n", systemName, address)
}

// FormatPID returns the PID kludge value.
func FormatPID() string {
	return fmt.Sprintf("ViSiON/3 %s/%s", Version, runtime.GOOS)
}

// FormatTID returns the TID kludge value.
func FormatTID() string {
	return FormatPID()
}
