package usereditor

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

// --- Title/Status bars ---
// UE.PAS: Color(8,15) for title and bottom bar
var titleBarStyle = dosColor(0, 15).Bold(true).Background(lipgloss.Color("8"))

// --- Background fill ---
// UE.PAS: Fill_Screen('░',7,1) → gray on blue
var bgFillStyle = dosColor(1, 7)

// --- List box border ---
// UE.PAS: Color(1,9) GrowBox
var listBorderStyle = dosColor(1, 9)

// --- List header text ---
// UE.PAS: Color(1,14) centered header inside box
var listHeaderStyle = dosColor(1, 14)

// --- Column title ---
// UE.PAS: Color(9,15) for column header row
var columnTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[9]))

// --- Normal list item ---
// UE.PAS: Color(1,15)
var listItemStyle = dosColor(1, 15)

// --- Tagged marker ---
// UE.PAS: Color(1,14) for the √ char
var taggedStyle = dosColor(1, 14)

// --- Highlighted item ---
// UE.PAS: Color(0,9) for tag area, Color(0,14) for text
var highlightTagStyle = dosColor(0, 9)
var highlightTextStyle = dosColor(0, 14)

// --- Edit screen ---
// UE.PAS: PColor=31 → attr: bg=1(blue), fg=15(white)→ actually 31 = 1*16+15
// Wait - PColor=31 in DISPEDIT: normcolor=15, incolor=31, pcolor=9
// But Def_Colors sets PColor:=31, NormColor:=30, InColor:=14
// PColor=31: bg=1(blue), fg=15(white) → field labels
// NormColor=30: bg=1(blue), fg=14(yellow) → displayed values
// InColor=14: just fg=14(yellow), no bg → editing values

// Field label (prompt) color: PColor=31 → blue bg, white fg
var fieldLabelStyle = dosStyle(31)

// Field value (display mode): NormColor=30 → blue bg, yellow fg
var fieldDisplayStyle = dosStyle(30)

// Field value (edit mode): InColor=14 → yellow
var fieldEditStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[14]))

// Edit screen title bar
var editTitleStyle = dosColor(0, 15).Bold(true).Background(lipgloss.Color("8"))

// Edit screen border: Color(1,9) same as list
var editBorderStyle = dosColor(1, 9)

// Edit screen user number text: Color(1,9) then Color(1,14)
var editInfoLabelStyle = dosColor(1, 9)
var editInfoValueStyle = dosColor(1, 14)

// --- Dialog styles ---
// Ask dialogs: PColor = 5*16+14 = 94, NormColor = 5*16+15 = 95
var dialogBorderStyle = dosStyle(95) // magenta bg, white fg
var dialogTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[13])).
	Bold(true)
var dialogTextStyle = dosStyle(94) // magenta bg, yellow fg
var dialogInputStyle = dosStyle(94)

// --- Message box ---
// UE.PAS Message: Color(4,12) border, Color(12,15) text
var messageBorderStyle = dosColor(4, 12)
var messageTextStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[12]))

// --- Help screen ---
// UE.PAS Help_Screen: Color(4,15) box, Color(4,14) title
var helpBoxStyle = dosColor(4, 15)
var helpTitleStyle = dosColor(4, 14)

// --- Bottom help bar ---
var helpBarStyle = dosColor(0, 15).Bold(true).Background(lipgloss.Color("8"))

// --- Flash message ---
var flashMessageStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[14]))

// --- Confirm dialog buttons ---
// Active: bright white on black (high contrast, clearly selected)
var buttonActiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[0])).
	Bold(true)

// Inactive: white on magenta (visible but not highlighted)
var buttonInactiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[5]))

// --- Separator style (dim label between editable and read-only sections) ---
var separatorStyle = dosColor(1, 9)

// --- Edit field fill character ---
const fieldFillChar = '░'
