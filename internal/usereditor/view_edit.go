package usereditor

import (
	"fmt"
	"strings"

	"github.com/stlalpha/vision3/internal/user"
)

// viewEditScreen renders the per-user field editor.
// Faithfully recreates UE.PAS v1.3 Edit_User screen.
func (m Model) viewEditScreen() string {
	if m.editIndex < 0 || m.editIndex >= len(m.users) {
		return "No user selected"
	}
	u := m.users[m.editIndex]

	var b strings.Builder

	// === Row 1: Title bar ===
	// UE.PAS: Color(8,15) Center_Write('╌╌ ViSiON/3 Quick & Easy User Editor v1.0 ╌╌')
	title := centerText("-- ViSiON/3 Quick & Easy User Editor v1.0 --", m.width)
	b.WriteString(editTitleStyle.Render(title))
	b.WriteByte('\n')

	// Background fill line (reused throughout)
	bgLine := bgFillStyle.Render(strings.Repeat("░", m.width))

	// Vertical centering: distribute extra rows above and below box.
	// Fixed content: 1 title + box(21) + help(1) = 23 rows
	extraV := max(0, m.height-23)
	topPad := max(1, extraV/2)
	bottomPad := max(1, extraV-topPad)

	for i := 0; i < topPad; i++ {
		b.WriteString(bgLine)
		b.WriteByte('\n')
	}

	// === Top border of edit box ===
	// UE.PAS: GrowBOX(2,3,78,23) Color(1,9)
	boxW := 76 // columns 2-78
	padL := max(0, (m.width-boxW-2)/2)
	padR := max(0, m.width-padL-boxW-2)

	topBorder := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("╒"+strings.Repeat("═", boxW)+"╕") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(topBorder)
	b.WriteByte('\n')

	// === Rows 4-22: Field area (19 rows inside box) ===
	// Render each row of the edit area
	for row := 4; row <= 22; row++ {
		rowContent := m.renderEditRow(row, u, boxW)
		line := bgFillStyle.Render(strings.Repeat("░", padL)) +
			editBorderStyle.Render("│") +
			rowContent +
			editBorderStyle.Render("│") +
			bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
		b.WriteString(line)
		b.WriteByte('\n')
	}

	// === Row 23: Bottom border ===
	botBorder := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("╘"+strings.Repeat("═", boxW)+"╛") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(botBorder)
	b.WriteByte('\n')

	// Bottom fill rows (vertically centers content)
	for i := 0; i < bottomPad; i++ {
		b.WriteString(bgLine)
		b.WriteByte('\n')
	}

	// === Bottom help bar ===
	// UE.PAS: 'F2 - Delete  F5 - Set Defaults  F10 - Aborts  ESC - Save Changes'
	helpText := centerText("F2 - Delete  F5 - Set Defaults  F10 - Aborts  ESC - Save Changes", m.width)
	b.WriteString(helpBarStyle.Render(helpText))

	// Overlay for password entry
	result := b.String()
	if m.mode == modePasswordEntry {
		result = m.overlayPasswordDialog(result)
	} else if m.mode == modeDeleteConfirm {
		result = m.overlayConfirmDialog(result, "-- Delete User --",
			fmt.Sprintf("Delete %s? ", u.Handle))
	} else if m.mode == modeValidate {
		result = m.overlayConfirmDialog(result, "-- Auto Validate --",
			fmt.Sprintf("Set %s to Defaults? ", u.Handle))
	}

	return result
}

// renderEditRow renders a single row inside the edit box area.
func (m Model) renderEditRow(row int, u *userType, boxW int) string {
	var leftField, rightField string
	var leftRawW, rightRawW int

	// Find fields that belong to this row
	for i, f := range m.fields {
		if f.Row != row {
			continue
		}

		rendered, rawW := m.renderField(i, f, u)

		if f.Col == 3 {
			leftField = rendered
			leftRawW = rawW
		} else if f.Col == 50 {
			rightField = rendered
			rightRawW = rawW
		}
	}

	// Separator row between editable and read-only fields
	if row == 17 {
		sepText := "-- Read Only --"
		sepPad := (boxW - len(sepText)) / 2
		return separatorStyle.Render(strings.Repeat("─", sepPad)) +
			separatorStyle.Render(sepText) +
			separatorStyle.Render(strings.Repeat("─", max(0, boxW-sepPad-len(sepText))))
	}

	// Special row: User Number display (row 22 in the box)
	if row == 22 {
		infoText := fmt.Sprintf("User Number: %d of %d", m.editIndex+1, len(m.users))
		infoRendered := editInfoLabelStyle.Render("User Number: ") +
			editInfoValueStyle.Render(fmt.Sprintf("%d", m.editIndex+1)) +
			editInfoLabelStyle.Render(" of ") +
			editInfoValueStyle.Render(fmt.Sprintf("%d", len(m.users)))
		leftPad := 40
		rawLen := len(infoText)
		return fieldDisplayStyle.Render(strings.Repeat(" ", leftPad)) + infoRendered +
			fieldDisplayStyle.Render(strings.Repeat(" ", max(0, boxW-leftPad-rawLen)))
	}

	if leftField == "" && rightField == "" {
		return fieldDisplayStyle.Render(strings.Repeat(" ", boxW))
	}

	// Build the row using pre-computed raw widths (not ANSI measurement).
	leftW := 41 // Left column width (40 content + 1 gap before right column)
	rightW := boxW - leftW

	var result string
	if leftField != "" {
		result = leftField
		if leftRawW < leftW {
			result += fieldDisplayStyle.Render(strings.Repeat(" ", leftW-leftRawW))
		}
	} else {
		result = fieldDisplayStyle.Render(strings.Repeat(" ", leftW))
	}

	if rightField != "" {
		result += rightField
		if rightRawW < rightW {
			result += fieldDisplayStyle.Render(strings.Repeat(" ", rightW-rightRawW))
		}
	} else {
		result += fieldDisplayStyle.Render(strings.Repeat(" ", rightW))
	}

	return result
}

// renderField renders a single field (label + value).
// Returns the styled string and the raw (unstyled) visible character width.
func (m Model) renderField(fieldIdx int, f fieldDef, u *userType) (string, int) {
	isActive := m.editField == fieldIdx

	// Pad labels to consistent widths so colons align vertically.
	// Left column: longest is "Messages Posted" at 15 chars.
	// Right column: longest is "Screen Height" at 13 chars.
	labelText := f.Label
	if f.Col == 3 {
		labelText = padRight(labelText, 15)
	} else if f.Col == 50 {
		labelText = padRight(labelText, 13)
	}
	label := labelText + " : "
	labelLen := len(label)

	var value string
	if f.Get != nil {
		value = f.Get(u)
	}

	// Raw width is always label + field width (value is padded/clamped to f.Width)
	rawW := labelLen + f.Width

	// If actively editing this field
	if isActive && m.mode == modeEditField {
		return fieldLabelStyle.Render(label) + m.textInput.View(), rawW
	}

	// Display the value
	displayValue := padRight(value, f.Width)

	if isActive && m.mode == modeEdit {
		// Highlighted field (ready to edit)
		fillStr := strings.Repeat(string(fieldFillChar), max(0, f.Width-len(value)))
		return fieldLabelStyle.Render(label) + fieldEditStyle.Render(value+fillStr), rawW
	}

	switch f.Type {
	case ftDisplay:
		return fieldLabelStyle.Render(label) + editInfoValueStyle.Render(displayValue), rawW
	case ftAction:
		return fieldLabelStyle.Render(label) + editInfoValueStyle.Render(displayValue), rawW
	default:
		return fieldLabelStyle.Render(label) + fieldDisplayStyle.Render(displayValue), rawW
	}
}

// approximateVisibleLen estimates the visible length of a styled string
// by stripping ANSI escape sequences.
func approximateVisibleLen(s string) int {
	inEsc := false
	count := 0
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		count++
	}
	return count
}

// overlayPasswordDialog renders a password entry dialog over the background.
func (m Model) overlayPasswordDialog(background string) string {
	lines := strings.Split(background, "\n")
	dialogW := 40
	dialogH := 5
	startRow := (m.height - dialogH) / 2
	startCol := (m.width - dialogW) / 2

	border := dialogBorderStyle.Render("╔" + strings.Repeat("═", dialogW-2) + "╗")
	borderBot := dialogBorderStyle.Render("╚" + strings.Repeat("═", dialogW-2) + "╝")
	side := dialogBorderStyle.Render("║")

	titleText := "Enter New Password"
	titlePad := (dialogW - 2 - len(titleText)) / 2
	titleLine := side +
		dialogTextStyle.Render(strings.Repeat(" ", titlePad)+titleText+strings.Repeat(" ", dialogW-2-titlePad-len(titleText))) +
		side

	emptyLine := side +
		dialogTextStyle.Render(strings.Repeat(" ", dialogW-2)) +
		side

	inputLine := side +
		dialogTextStyle.Render(" ") +
		m.textInput.View() +
		dialogTextStyle.Render(strings.Repeat(" ", max(0, dialogW-3-m.textInput.Width))) +
		side

	dialogLines := []string{border, titleLine, emptyLine, inputLine, borderBot}

	// Fill remaining width with styled ░
	tailW := max(0, m.width-startCol-dialogW)
	tail := bgFillStyle.Render(strings.Repeat("░", tailW))
	for i, dl := range dialogLines {
		row := startRow + i
		if row >= 0 && row < len(lines) {
			lines[row] = padToCol(lines[row], startCol) + dl + tail
		}
	}

	return strings.Join(lines, "\n")
}

// padToCol truncates or pads a line to reach a specific column.
func padToCol(line string, col int) string {
	vis := approximateVisibleLen(line)
	if vis >= col {
		return truncateToVisual(line, col)
	}
	return line + strings.Repeat(" ", col-vis)
}

// truncateToVisual truncates a string to n visible characters, preserving ANSI.
func truncateToVisual(s string, n int) string {
	var b strings.Builder
	inEsc := false
	count := 0
	for _, r := range s {
		if count >= n && !inEsc {
			break
		}
		b.WriteRune(r)
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		count++
	}
	return b.String()
}

// skipToCol returns everything in a string from visible column n onward,
// replaying the last active ANSI escape sequence so styling is preserved.
func skipToCol(s string, n int) string {
	var lastESC strings.Builder // tracks the most recent ANSI sequence
	var curESC strings.Builder  // builds current escape sequence
	inEsc := false
	count := 0
	for i, r := range s {
		if r == '\x1b' {
			inEsc = true
			curESC.Reset()
			curESC.WriteRune(r)
			continue
		}
		if inEsc {
			curESC.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
				lastESC.Reset()
				lastESC.WriteString(curESC.String())
			}
			continue
		}
		if count == n {
			// Prepend the last ANSI sequence to restore styling context
			return lastESC.String() + s[i:]
		}
		count++
	}
	return ""
}

// userType is a type alias for user.User (keeps method signatures clean).
type userType = user.User

// Ensure the user package is imported for the type alias.
var _ *user.User
