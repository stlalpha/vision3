package configeditor

import (
	"github.com/charmbracelet/lipgloss"
)

// DOS CGA color palette mapped to ANSI 256-color indices.
// These are the standard 16 DOS colors (0-15).
var dosColors = [16]string{
	"0",  // 0:  Black
	"4",  // 1:  Blue (DOS blue = ANSI 4)
	"2",  // 2:  Green
	"6",  // 3:  Cyan (DOS cyan = ANSI 6)
	"1",  // 4:  Red (DOS red = ANSI 1)
	"5",  // 5:  Magenta
	"3",  // 6:  Brown/Yellow (DOS brown = ANSI 3)
	"7",  // 7:  Light Gray
	"8",  // 8:  Dark Gray
	"12", // 9:  Light Blue (DOS light blue = ANSI 12)
	"10", // 10: Light Green
	"14", // 11: Light Cyan (DOS light cyan = ANSI 14)
	"9",  // 12: Light Red (DOS light red = ANSI 9)
	"13", // 13: Light Magenta
	"11", // 14: Yellow (DOS yellow = ANSI 11)
	"15", // 15: White
}

// DOS CGA background colors mapped to ANSI 256-color indices.
var dosBgColors = [8]string{
	"0", // 0: Black BG
	"4", // 1: Blue BG
	"2", // 2: Green BG
	"6", // 3: Cyan BG
	"1", // 4: Red BG
	"5", // 5: Magenta BG
	"3", // 6: Brown BG
	"7", // 7: Light Gray BG
}

// dosStyle creates a lipgloss style from a DOS TextAttr byte (bg*16 + fg).
func dosStyle(attr byte) lipgloss.Style {
	fg := attr & 0x0F
	bg := (attr >> 4) & 0x07
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(dosColors[fg])).
		Background(lipgloss.Color(dosBgColors[bg]))
}

// dosColor creates a lipgloss style from separate DOS bg, fg values
// matching the Pascal Color(bg, fg) procedure.
func dosColor(bg, fg int) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(dosColors[fg&0x0F])).
		Background(lipgloss.Color(dosBgColors[bg&0x07]))
}

// --- Global header bar (white text on dark gray bg) ---
var globalHeaderBarStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color("8")).
	Bold(true)

// --- Title/Status bars ---
var titleBarStyle = dosColor(0, 15).Bold(true).Background(lipgloss.Color("8"))

// --- Background fill ---
// Fill_Screen('░',7,1) → gray on blue
var bgFillStyle = dosColor(1, 7)

// --- Menu box border ---
var menuBorderStyle = dosColor(1, 9)

// --- Menu header text ---
var menuHeaderStyle = dosColor(1, 14)

// --- Normal menu item ---
var menuItemStyle = dosColor(1, 15)

// --- Highlighted menu item ---
var menuHighlightStyle = dosColor(0, 14)

// --- Field label (prompt) color: blue bg, white fg ---
var fieldLabelStyle = dosStyle(31)

// --- Field value (display mode): blue bg, yellow fg ---
var fieldDisplayStyle = dosStyle(30)

// --- Field value (edit mode): blue bg, yellow fg ---
var fieldEditStyle = dosColor(1, 14)

// --- Edit screen border ---
var editBorderStyle = dosColor(1, 9)

// --- Edit info label/value ---
var editInfoLabelStyle = dosColor(1, 9)
var editInfoValueStyle = dosColor(1, 14)

// --- Dialog styles ---
var dialogBorderStyle = dosStyle(95) // magenta bg, white fg
var dialogTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[13])).
	Bold(true)
var dialogTextStyle = dosStyle(94) // magenta bg, yellow fg

// --- Help screen ---
var helpBoxStyle = dosColor(4, 15)
var helpTitleStyle = dosColor(4, 14)

// --- Bottom help bar ---
var helpBarStyle = dosColor(0, 15).Bold(true).Background(lipgloss.Color("8"))

// --- Flash message ---
var flashMessageStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[14]))

// --- Confirm dialog buttons ---
var buttonActiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[0])).
	Bold(true)

var buttonInactiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[5]))

// --- Reorder source row (green bg, white fg) ---
var reorderSourceStyle = dosColor(2, 15)

// --- Separator style ---
var separatorStyle = dosColor(1, 9)

// --- Edit field fill character ---
const fieldFillChar = '░'
