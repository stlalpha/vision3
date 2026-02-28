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
// Two spaces of left padding are applied to match the Pascal x=4 column offset.
func (m Model) renderMenuField(fieldIdx int, f fieldDef, d *MenuData, boxW int) string {
	const lpad = 2
	isActive := m.menuEditFld == fieldIdx

	label := f.Label + ": "
	labelLen := len(label)

	var value string
	if f.GetM != nil {
		value = f.GetM(d)
	}

	// Cap display width to available space inside the box
	maxW := boxW - lpad - labelLen
	if maxW < 0 {
		maxW = 0
	}
	dispW := f.Width
	if dispW > maxW {
		dispW = maxW
	}

	rawW := lpad + labelLen + dispW
	leftPad := fieldDisplayStyle.Render(strings.Repeat(" ", lpad))

	// Actively editing this field
	if isActive && m.mode == modeMenuEditField {
		inputW := m.textInput.Width
		if inputW > maxW {
			inputW = maxW
		}
		fillW := max(0, boxW-lpad-labelLen-inputW)
		return leftPad + fieldLabelStyle.Render(label) + m.textInput.View() +
			fieldDisplayStyle.Render(strings.Repeat(" ", fillW))
	}

	// Display value (truncated to dispW)
	displayValue := padRight(value, dispW)

	if isActive {
		// Highlighted field (ready to edit) — truncate to dispW to prevent overflow
		displayVal := value
		if len(displayVal) > dispW {
			displayVal = displayVal[:dispW]
		}
		fillStr := strings.Repeat(string(fieldFillChar), max(0, dispW-len(displayVal)))
		result := leftPad + fieldLabelStyle.Render(label) + fieldEditStyle.Render(displayVal+fillStr)
		result += fieldDisplayStyle.Render(strings.Repeat(" ", max(0, boxW-rawW)))
		return result
	}

	// Normal display
	result := leftPad + fieldLabelStyle.Render(label) + fieldDisplayStyle.Render(displayValue)
	result += fieldDisplayStyle.Render(strings.Repeat(" ", max(0, boxW-rawW)))
	return result
}
