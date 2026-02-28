package menueditor

import (
	"fmt"
	"strings"
)

// viewMenuEditScreen renders the per-menu field editor.
// Faithfully recreates MENUEDIT.PAS EditMenu / Edit_Menu.
func (m Model) viewMenuEditScreen() string {
	if m.menuEditIdx < 0 || m.menuEditIdx >= len(m.menus) {
		return m.viewMenuListScreen()
	}
	entry := m.menus[m.menuEditIdx]

	var b strings.Builder

	// === Row 1: Title bar ===
	title := centerText("-- ViSiON/3 Menu Editor v1.0 --", m.width)
	b.WriteString(titleBarStyle.Render(title))
	b.WriteByte('\n')

	bg := m.bgLine()

	// Box dimensions matching Pascal GrowBOX(2,6,78,20)
	// width = 78-2-2 = 74 interior, height ~15 rows
	boxW := 74
	padL := max(0, (m.width-boxW-2)/2)
	padR := max(0, m.width-padL-boxW-2)

	// Vertical centering: title(1) + gap(4) + box(14) + gap(4) + help(1) = 24
	extraV := max(0, m.height-24)
	topPad := max(4, extraV/2+4)
	bottomPad := max(1, m.height-1-topPad-14)

	for i := 0; i < topPad; i++ {
		b.WriteString(bg)
		b.WriteByte('\n')
	}

	// === Top border ===
	// MENUEDIT.PAS: Color(15,8) → dark gray bg, white fg for box
	topBorder := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("╒"+strings.Repeat("═", boxW)+"╕") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(topBorder)
	b.WriteByte('\n')

	// === Title row inside box ===
	// MENUEDIT.PAS: Color(15,12) center_write 'Command Editing...'
	boxTitle := fmt.Sprintf("Editing Menu: %s.MNU", entry.Name)
	titleRow := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("│") +
		editTitleStyle.Render(centerText(boxTitle, boxW)) +
		editBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(titleRow)
	b.WriteByte('\n')

	// === Empty separator ===
	emptyRow := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("│") +
		fieldDisplayStyle.Render(strings.Repeat(" ", boxW)) +
		editBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(emptyRow)
	b.WriteByte('\n')

	// === Field rows ===
	for i, f := range m.menuFields {
		row := m.renderMenuField(i, f, &entry.Data, boxW)
		line := bgFillStyle.Render(strings.Repeat("░", padL)) +
			editBorderStyle.Render("│") +
			row +
			editBorderStyle.Render("│") +
			bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
		b.WriteString(line)
		b.WriteByte('\n')
	}

	// === Info row: current file + number ===
	infoText := fmt.Sprintf("  Menu %d of %d", m.menuEditIdx+1, len(m.menus))
	infoRow := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("│") +
		editInfoLabelStyle.Render(padRight(infoText, boxW)) +
		editBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(infoRow)
	b.WriteByte('\n')

	// === Empty row at bottom of box ===
	b.WriteString(emptyRow)
	b.WriteByte('\n')

	// === Bottom border ===
	botBorder := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("╘"+strings.Repeat("═", boxW)+"╛") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(botBorder)
	b.WriteByte('\n')

	for i := 0; i < bottomPad; i++ {
		b.WriteString(bg)
		b.WriteByte('\n')
	}

	// === Help bar ===
	helpText := centerText("PgUp/PgDn Prev/Next  F2 Delete  F5 Add New  F10 Edit Commands  ESC Back", m.width)
	b.WriteString(helpBarStyle.Render(helpText))

	// Overlay dialogs
	result := b.String()
	switch m.mode {
	case modeDeleteMenuConfirm:
		result = m.overlayConfirmDialog(result,
			"-- Delete Menu --",
			fmt.Sprintf("Delete %s.MNU and all commands? ", entry.Name))
	case modeAddMenu:
		result = m.overlayInputDialog(result,
			"-- Create New Menu --",
			"New menu filename (8 chars): ",
			m.textInput.View())
	}

	return result
}

// renderMenuField renders a single menu field row inside the edit box.
func (m Model) renderMenuField(fieldIdx int, f fieldDef, d *MenuData, boxW int) string {
	isActive := m.menuEditFld == fieldIdx

	label := f.Label + ": "
	labelLen := len(label)

	var value string
	if f.GetM != nil {
		value = f.GetM(d)
	}

	rawW := labelLen + f.Width

	// Actively editing this field
	if isActive && m.mode == modeMenuEditField {
		left := fieldLabelStyle.Render(label) + m.textInput.View()
		// Pad to box width
		inputW := m.textInput.Width
		fillW := max(0, boxW-labelLen-inputW)
		left += fieldDisplayStyle.Render(strings.Repeat(" ", fillW))
		return left
	}

	// Display value
	displayValue := padRight(value, f.Width)

	if isActive {
		// Highlighted field (ready to edit)
		fillStr := strings.Repeat(string(fieldFillChar), max(0, f.Width-len(value)))
		result := fieldLabelStyle.Render(label) + fieldEditStyle.Render(value+fillStr)
		fillW := max(0, boxW-rawW)
		result += fieldDisplayStyle.Render(strings.Repeat(" ", fillW))
		return result
	}

	// Normal display
	result := fieldLabelStyle.Render(label) + fieldDisplayStyle.Render(displayValue)
	fillW := max(0, boxW-rawW)
	result += fieldDisplayStyle.Render(strings.Repeat(" ", fillW))
	return result
}
