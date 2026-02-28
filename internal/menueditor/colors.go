package menueditor

import (
	"github.com/charmbracelet/lipgloss"
)

// DOS CGA color palette mapped to ANSI 256-color indices.
var dosColors = [16]string{
	"0",  // 0:  Black
	"4",  // 1:  Blue
	"2",  // 2:  Green
	"6",  // 3:  Cyan
	"1",  // 4:  Red
	"5",  // 5:  Magenta
	"3",  // 6:  Brown/Yellow
	"7",  // 7:  Light Gray
	"8",  // 8:  Dark Gray
	"12", // 9:  Light Blue
	"10", // 10: Light Green
	"14", // 11: Light Cyan
	"9",  // 12: Light Red
	"13", // 13: Light Magenta
	"11", // 14: Yellow
	"15", // 15: White
}

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

// dosStyle creates a lipgloss style from a DOS TextAttr byte (bg<<4 | fg).
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
// MENUEDIT.PAS: Color(8,15) → dark gray bg, white fg
var titleBarStyle = dosColor(0, 15).Bold(true).Background(lipgloss.Color("8"))
var helpBarStyle = dosColor(0, 15).Bold(true).Background(lipgloss.Color("8"))

// --- Background fill ---
// MENUEDIT.PAS: Fill_Screen('░',7,1) → light gray fg, blue bg
var bgFillStyle = dosColor(1, 7)

// --- Menu list box ---
// MENUEDIT.PAS Open_Screen: Color(3,11) GrowBox → cyan bg, light cyan fg
var listBorderStyle = dosColor(3, 11)

// MENUEDIT.PAS: Color(11,3) column header → light cyan bg, cyan fg
var listHeaderStyle = dosColor(3, 11)
var listColTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[3])).
	Background(lipgloss.Color(dosColors[11]))

// MENUEDIT.PAS: Color(3,15) → cyan bg, white fg for normal items
var listItemStyle = dosColor(3, 15)

// MENUEDIT.PAS: Color(1,15) → blue bg, white fg for highlighted item
var listHighlightStyle = dosColor(1, 15)

// --- Edit box (menu/command edit screens) ---
// MENUEDIT.PAS Edit_Menu: Color(8,15) box border → dark gray bg, white fg (actually uses Color(15,8) for White_Colors)
// White_Colors: PColor=$F1(bg=7/white, fg=1/blue), NormColor=$F0(bg=7/white, fg=0/black)
var editBorderStyle = dosColor(0, 15).Background(lipgloss.Color("8"))

// Field label: PColor=$F1 → light gray bg, blue fg
var fieldLabelStyle = dosStyle(0xF1)

// Field value (inactive): NormColor=$F0 → light gray bg, black fg
var fieldDisplayStyle = dosStyle(0xF0)

// Field value (active/highlighted): InColor=$0E → black bg, yellow fg
var fieldEditStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[14]))

// Field fill character for active (not-yet-editing) fields
const fieldFillChar = '░'

// Edit box title / section header: Color(15,12) → white bg, light red fg
var editTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[12])).
	Background(lipgloss.Color(dosColors[15]))

// Info label/value inside edit screens: Color(15,8) and Color(15,4)
var editInfoLabelStyle = dosStyle(0xF0)
var editInfoValueStyle = dosStyle(0xF4) // light gray bg, red fg (for file/number display)

// --- Confirm dialog ---
// MENUEDIT.PAS Ask_Colors: PColor=94($5E), NormColor=95($5F) → magenta bg
var dialogBorderStyle = dosStyle(0x5F) // magenta bg, white fg
var dialogTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[13])).
	Bold(true)
var dialogTextStyle = dosStyle(0x5E) // magenta bg, yellow fg

// --- Input dialog (Add Menu, etc.) ---
var inputDialogBorderStyle = dosStyle(0x5F)
var inputDialogTextStyle = dosStyle(0x5E)

// --- Message box ---
// MENUEDIT.PAS Message: Color(5,13) border → magenta bg, light magenta fg
var messageBorderStyle = dosColor(5, 13)
var messageTextStyle = dosColor(13, 15)

// --- Help screen ---
var helpBoxStyle = dosColor(4, 15)
var helpTitleStyle = dosColor(4, 14)

// --- Confirm buttons ---
var buttonActiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[0])).
	Bold(true)
var buttonInactiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[15])).
	Background(lipgloss.Color(dosColors[5]))

// --- Flash message ---
var flashMessageStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(dosColors[14]))
