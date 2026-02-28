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

	// Box dimensions: MENUEDIT.PAS GrowBox(2,9,78,16) → 74 cols wide, ~9 rows
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
	// MENUEDIT.PAS: Color(15,8) GrowBox → dark gray bg, white fg
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
	// Left column fields; right side shows info (file/number)
	rightColStart := 50 // approximate right column start for info labels

	for i, f := range m.cmdFields {
		row := m.renderCmdField(i, f, &cmd, boxW, rightColStart, i == 0, i == len(m.cmdFields)-3)
		line := bgFillStyle.Render(strings.Repeat("░", padL)) +
			editBorderStyle.Render("│") +
			row +
			editBorderStyle.Render("│") +
			bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
		b.WriteString(line)
		b.WriteByte('\n')
	}

	// === Info row ===
	infoText := fmt.Sprintf("  Command %d of %d", m.cmdEditIdx+1, len(m.cmds))
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
	// MENUEDIT.PAS: 'PgUp/PgDn Scrolls  F8 Aborts  F2 Delete  F5 Add New  ESC Exits'
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
// showFileInfo: show "Current File: ..." on the right side of this row.
// showNumInfo:  show "Current Number: ..." on the right side of this row.
func (m Model) renderCmdField(fieldIdx int, f fieldDef, d *CmdData, boxW, rightColStart int, showFileInfo, showNumInfo bool) string {
	isActive := m.cmdEditFld == fieldIdx

	label := f.Label + ": "
	labelLen := len(label)

	var value string
	if f.GetC != nil {
		value = f.GetC(d)
	}

	// Right-side info text
	var rightInfo string
	menuName := ""
	if m.cmdsMenuIdx >= 0 && m.cmdsMenuIdx < len(m.menus) {
		menuName = m.menus[m.cmdsMenuIdx].Name
	}
	if showFileInfo {
		rightInfo = fmt.Sprintf("File: %s.CFG", menuName)
	} else if showNumInfo {
		rightInfo = fmt.Sprintf("Num: %d of %d", m.cmdEditIdx+1, len(m.cmds))
	}

	// Build left part (label + field)
	var leftPart string
	leftW := rightColStart // width available for left column

	if isActive && m.mode == modeCommandEditField {
		inputView := m.textInput.View()
		inputW := m.textInput.Width
		leftPart = fieldLabelStyle.Render(label) + inputView
		usedW := labelLen + inputW
		if usedW < leftW {
			leftPart += fieldDisplayStyle.Render(strings.Repeat(" ", leftW-usedW))
		}
	} else {
		displayValue := padRight(value, f.Width)
		if isActive {
			fillStr := strings.Repeat(string(fieldFillChar), max(0, f.Width-len(value)))
			leftPart = fieldLabelStyle.Render(label) + fieldEditStyle.Render(value+fillStr)
		} else {
			leftPart = fieldLabelStyle.Render(label) + fieldDisplayStyle.Render(displayValue)
		}
		usedW := labelLen + f.Width
		if usedW < leftW {
			leftPart += fieldDisplayStyle.Render(strings.Repeat(" ", leftW-usedW))
		}
	}

	// Build right part
	var rightPart string
	rightW := boxW - rightColStart
	if rightInfo != "" {
		rightPart = editInfoLabelStyle.Render(padRight(rightInfo, rightW))
	} else {
		rightPart = fieldDisplayStyle.Render(strings.Repeat(" ", rightW))
	}

	return leftPart + rightPart
}
