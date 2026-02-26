package configeditor

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// fieldType defines the edit behavior for a field.
type fieldType int

const (
	ftString  fieldType = iota // Free-text string input
	ftInteger                  // Integer with min/max validation
	ftYesNo                    // Y/N boolean toggle
	ftDisplay                  // Read-only display
	ftLookup                   // Lookup picker (select from list)
)

// LookupItem represents a selectable item in a lookup picker.
type LookupItem struct {
	Value   string // stored value (e.g. "1")
	Display string // shown in picker list (e.g. "Local Conferences (LOCAL)")
}

// fieldDef defines a single editable field on a config screen.
type fieldDef struct {
	Label       string               // Display label
	Help        string               // 1-line help text shown when field is active
	Type        fieldType            // Edit type
	Col         int                  // Column position (x) inside box
	Row         int                  // Row position (y) inside box
	Width       int                  // Input field width
	Min         int                  // Minimum value (for ftInteger)
	Max         int                  // Maximum value (for ftInteger)
	Get         func() string
	Set         func(val string) error
	LookupItems func() []LookupItem // provider for ftLookup
}

// boolToYN converts a bool to "Y" or "N".
func boolToYN(b bool) string {
	if b {
		return "Y"
	}
	return "N"
}

// ynToBool converts "Y"/"y" to true, anything else to false.
func ynToBool(s string) bool {
	return strings.ToUpper(s) == "Y"
}

// padRight pads a string to width with spaces, truncating if longer.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// padLeft pads a string on the left to width.
func padLeft(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// intFieldLabel returns a formatted label with colon for field display.
func intFieldLabel(label string) string {
	return fmt.Sprintf("%s : ", label)
}

// centerText centers a string within a given width using visual (rune) width.
func centerText(s string, width int) string {
	vis := utf8.RuneCountInString(s)
	if vis >= width {
		return s
	}
	pad := (width - vis) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-pad-vis)
}
