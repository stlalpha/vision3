package menueditor

import (
	"fmt"
	"strings"
)

// fieldType defines the edit behaviour for a field.
type fieldType int

const (
	ftString  fieldType = iota // Free-text string input
	ftYesNo                    // Y/N boolean toggle (auto-confirms on keypress)
	ftDisplay                  // Read-only display
)

// fieldDef defines a single editable field on an edit screen.
type fieldDef struct {
	Label  string    // Display label (e.g. "Clear Screen")
	Type   fieldType
	Row    int // Row position (y) relative to box interior top
	Width  int // Input field width
	GetM   func(d *MenuData) string
	SetM   func(d *MenuData, val string) error
	GetC   func(d *CmdData) string
	SetC   func(d *CmdData, val string) error
}

// menuFields returns the ordered editable fields for a MenuData record.
// Matches Pascal EditMenu fields, adapted for the V3 JSON schema.
func menuFields() []fieldDef {
	return []fieldDef{
		{
			Label: "Clear Screen   ", Type: ftYesNo, Row: 3, Width: 1,
			GetM: func(d *MenuData) string { return boolToYN(d.CLR) },
			SetM: func(d *MenuData, val string) error { d.CLR = ynToBool(val); return nil },
		},
		{
			Label: "Use Prompt     ", Type: ftYesNo, Row: 4, Width: 1,
			GetM: func(d *MenuData) string { return boolToYN(d.UsePrompt) },
			SetM: func(d *MenuData, val string) error { d.UsePrompt = ynToBool(val); return nil },
		},
		{
			Label: "Prompt Line 1  ", Type: ftString, Row: 5, Width: 57,
			GetM: func(d *MenuData) string { return d.Prompt1 },
			SetM: func(d *MenuData, val string) error { d.Prompt1 = val; return nil },
		},
		{
			Label: "Prompt Line 2  ", Type: ftString, Row: 6, Width: 57,
			GetM: func(d *MenuData) string { return d.Prompt2 },
			SetM: func(d *MenuData, val string) error { d.Prompt2 = val; return nil },
		},
		{
			Label: "Fallback Menu  ", Type: ftString, Row: 7, Width: 12,
			GetM: func(d *MenuData) string { return d.Fallback },
			SetM: func(d *MenuData, val string) error { d.Fallback = strings.ToUpper(val); return nil },
		},
		{
			Label: "ACS Required   ", Type: ftString, Row: 8, Width: 30,
			GetM: func(d *MenuData) string { return d.ACS },
			SetM: func(d *MenuData, val string) error { d.ACS = strings.ToUpper(val); return nil },
		},
		{
			Label: "Menu Password  ", Type: ftString, Row: 9, Width: 30,
			GetM: func(d *MenuData) string { return d.Password },
			SetM: func(d *MenuData, val string) error { d.Password = val; return nil },
		},
	}
}

// cmdFields returns the ordered editable fields for a CmdData record.
// Matches Pascal Edit_Command fields, adapted for the V3 JSON schema.
func cmdFields() []fieldDef {
	return []fieldDef{
		{
			Label: "Node Activity  ", Type: ftString, Row: 3, Width: 40,
			GetC: func(d *CmdData) string { return d.NodeActivity },
			SetC: func(d *CmdData, val string) error { d.NodeActivity = val; return nil },
		},
		{
			Label: "Keystroke(s)   ", Type: ftString, Row: 4, Width: 20,
			GetC: func(d *CmdData) string { return d.Keys },
			SetC: func(d *CmdData, val string) error { d.Keys = strings.ToUpper(val); return nil },
		},
		{
			Label: "Command(s)     ", Type: ftString, Row: 5, Width: 50,
			GetC: func(d *CmdData) string { return d.Command },
			SetC: func(d *CmdData, val string) error { d.Command = val; return nil },
		},
		{
			Label: "ACS Required   ", Type: ftString, Row: 6, Width: 30,
			GetC: func(d *CmdData) string { return d.ACS },
			SetC: func(d *CmdData, val string) error { d.ACS = strings.ToUpper(val); return nil },
		},
		{
			Label: "Hidden?        ", Type: ftYesNo, Row: 7, Width: 1,
			GetC: func(d *CmdData) string { return boolToYN(d.Hidden) },
			SetC: func(d *CmdData, val string) error { d.Hidden = ynToBool(val); return nil },
		},
		{
			Label: "Auto Run       ", Type: ftString, Row: 8, Width: 30,
			GetC: func(d *CmdData) string { return d.AutoRun },
			SetC: func(d *CmdData, val string) error { d.AutoRun = val; return nil },
		},
	}
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

// centerText centers a string within a given width.
func centerText(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := (width - len(s)) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-pad-len(s))
}

// intFieldLabel returns a formatted label with colon for field display.
func intFieldLabel(label string) string {
	return fmt.Sprintf("%s : ", label)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
