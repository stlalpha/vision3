package menueditor

import (
	"fmt"
	"strings"
)

// viewCommandEditScreen renders the per-command field editor.
// Faithfully recreates MENUEDIT.PAS Edit_Command.
func (m Model) viewCommandEditScreen() string {
	if m.cmdEditIdx < 0 || m.cmdEditIdx >= len(m.cmds) {
		return m.viewCommandListScreen()
	}

	menuName := ""
	if m.cmdsMenuIdx >= 0 && m.cmdsMenuIdx < len(m.menus) {
		menuName = m.menus[m.cmdsMenuIdx].Name
	}

	cmd := m.cmds[m.cmdEditIdx]
	var b strings.Builder

	// === Row 1: Title bar ===
	title := centerText("-- ViSiON/3 Menu Editor v1.0 --", m.width)
	b.WriteString(titleBarStyle.Render(title))
	b.WriteByte('\n')

	bg := m.bgLine()

	// Box dimensions: MENUEDIT.PAS GrowBox(2,9,78,16) → 74 cols wide
	boxW := 74
	padL := max(0, (m.width-boxW-2)/2)
	padR := max(0, m.width-padL-boxW-2)

	// Vertical centering
	extraV := max(0, m.height-24)
	topPad := max(4, extraV/2+4)
	bottomPad := max(1, m.height-1-topPad-12)

	for i := 0; i < topPad; i++ {
		b.WriteString(bg)
		b.WriteByte('\n')
	}

	// === Top border ===
	topBorder := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("╒"+strings.Repeat("═", boxW)+"╕") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(topBorder)
	b.WriteByte('\n')

	// === Title row inside box ===
	// MENUEDIT.PAS: Color(15,12) Center_Write('Command Editing (MenuTitle)')
	boxTitle := fmt.Sprintf("Command Editing (%s)", menuName)
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
	for i, f := range m.cmdFields {
		row := m.renderCmdField(i, f, &cmd, boxW)
		line := bgFillStyle.Render(strings.Repeat("░", padL)) +
			editBorderStyle.Render("│") +
			row +
			editBorderStyle.Render("│") +
			bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
		b.WriteString(line)
		b.WriteByte('\n')
	}

	// === Info row: file + command number ===
	// MENUEDIT.PAS shows file at col 50 row 11 and number at col 50 row 15;
	// consolidated here to avoid per-field right-column overflow.
	infoFile := fmt.Sprintf("  File: %s.CFG", menuName)
	infoNum := fmt.Sprintf("Cmd: %d of %d", m.cmdEditIdx+1, len(m.cmds))
	infoText := padRight(infoFile, 40) + infoNum
	infoRow := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("│") +
		editInfoLabelStyle.Render(padRight(infoText, boxW)) +
		editBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(infoRow)
	b.WriteByte('\n')

	// === Empty bottom row ===
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
	helpText := centerText("PgUp/PgDn Prev/Next  F2 Delete  F5 Add New  F8 Abort  ESC Save+Back", m.width)
	b.WriteString(helpBarStyle.Render(helpText))

	// Overlay dialogs
	result := b.String()
	if m.mode == modeDeleteCmdConfirm {
		desc := cmd.NodeActivity
		if desc == "" {
			desc = cmd.Keys
		}
		result = m.overlayConfirmDialog(result,
			"-- Delete Command --",
			fmt.Sprintf("Delete command '%s'? ", desc))
	}

	return result
}

// renderCmdField renders a single command field row inside the edit box.
// Uses the full box width — max value width = boxW - lpad(2) - labelLen(17).
func (m Model) renderCmdField(fieldIdx int, f fieldDef, d *CmdData, boxW int) string {
	const lpad = 2
	isActive := m.cmdEditFld == fieldIdx

	label := f.Label + ": "
	labelLen := len(label)

	var value string
	if f.GetC != nil {
		value = f.GetC(d)
	}

	// Cap display/input width to available space inside the box
	maxW := boxW - lpad - labelLen
	if maxW < 0 {
		maxW = 0
	}
	dispW := f.Width
	if dispW > maxW {
		dispW = maxW
	}

	rawW := lpad + labelLen + dispW
	leftPadStr := fieldDisplayStyle.Render(strings.Repeat(" ", lpad))

	// Actively editing this field
	if isActive && m.mode == modeCommandEditField {
		inputW := m.textInput.Width
		if inputW > maxW {
			inputW = maxW
		}
		fillW := max(0, boxW-lpad-labelLen-inputW)
		return leftPadStr + fieldLabelStyle.Render(label) + m.textInput.View() +
			fieldDisplayStyle.Render(strings.Repeat(" ", fillW))
	}

	// Display value
	displayValue := padRight(value, dispW)

	if isActive {
		// Highlighted (ready to edit)
		fillStr := strings.Repeat(string(fieldFillChar), max(0, dispW-len(value)))
		result := leftPadStr + fieldLabelStyle.Render(label) + fieldEditStyle.Render(value+fillStr)
		result += fieldDisplayStyle.Render(strings.Repeat(" ", max(0, boxW-rawW)))
		return result
	}

	result := leftPadStr + fieldLabelStyle.Render(label) + fieldDisplayStyle.Render(displayValue)
	result += fieldDisplayStyle.Render(strings.Repeat(" ", max(0, boxW-rawW)))
	return result
}
