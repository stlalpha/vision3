package menueditor

import (
	"fmt"
	"strings"
)

// viewCommandListScreen renders the command list browser for the selected menu.
// Faithfully recreates MENUEDIT.PAS Select_Command.
func (m Model) viewCommandListScreen() string {
	menuTitle := "" // friendly title for display headings
	if m.cmdsMenuIdx >= 0 && m.cmdsMenuIdx < len(m.menus) {
		e := m.menus[m.cmdsMenuIdx]
		menuTitle = e.Data.Title
		if menuTitle == "" {
			menuTitle = e.Name
		}
	}

	var b strings.Builder

	// === Row 1: Title bar ===
	title := centerText("-- ViSiON/3 Menu Editor v1.0 --", m.width)
	b.WriteString(titleBarStyle.Render(title))
	b.WriteByte('\n')

	bg := m.bgLine()

	// Box dimensions: MENUEDIT.PAS GrowBox(4,4,76,22) → 70 cols wide
	boxW := 70
	padL := max(0, (m.width-boxW-2)/2)
	padR := max(0, m.width-padL-boxW-2)

	// Vertical centering: title(1) + box(21) + message(1) + help(1) = 24 rows
	extraV := max(0, m.height-24)
	topPad := max(1, extraV/2)
	bottomPad := max(1, extraV-topPad)

	for i := 0; i < topPad; i++ {
		b.WriteString(bg)
		b.WriteByte('\n')
	}

	// === Top border ===
	// MENUEDIT.PAS: Color(4,12) GrowBox → red bg, light red fg
	topBorder := bgFillStyle.Render(strings.Repeat("░", padL)) +
		listBorderStyle.Render("┌"+strings.Repeat("─", boxW)+"┐") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(topBorder)
	b.WriteByte('\n')

	// === Section title inside box ===
	// MENUEDIT.PAS: Color(4,14) Center_Write('Editing Menu Commands for: ...')
	sectionTitle := fmt.Sprintf("Editing Menu Commands for: %s", menuTitle)
	titleRow := bgFillStyle.Render(strings.Repeat("░", padL)) +
		listBorderStyle.Render("│") +
		listHeaderStyle.Render(centerText(sectionTitle, boxW)) +
		listBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(titleRow)
	b.WriteByte('\n')

	// === Column header ===
	// MENUEDIT.PAS: 'Command Description   Keystroke(s)    Command(s)'
	colText := padRight("   Node Activity          Keys       Command", boxW)
	colLine := bgFillStyle.Render(strings.Repeat("░", padL)) +
		listBorderStyle.Render("│") +
		listColTitleStyle.Render(colText) +
		listBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(colLine)
	b.WriteByte('\n')

	// === Empty separator ===
	emptyLine := bgFillStyle.Render(strings.Repeat("░", padL)) +
		listBorderStyle.Render("│") +
		listItemStyle.Render(strings.Repeat(" ", boxW)) +
		listBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(emptyLine)
	b.WriteByte('\n')

	// === Command list (listVisible rows) ===
	total := len(m.cmds)
	for row := 0; row < listVisible; row++ {
		idx := m.cmdScroll + row
		var rowContent string
		if idx < 0 || idx >= total {
			rowContent = listItemStyle.Render(strings.Repeat(" ", boxW))
		} else {
			rowContent = m.renderCmdRow(idx, boxW)
		}
		line := bgFillStyle.Render(strings.Repeat("░", padL)) +
			listBorderStyle.Render("│") +
			rowContent +
			listBorderStyle.Render("│") +
			bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
		b.WriteString(line)
		b.WriteByte('\n')
	}

	// === Bottom border ===
	botBorder := bgFillStyle.Render(strings.Repeat("░", padL)) +
		listBorderStyle.Render("└"+strings.Repeat("─", boxW)+"┘") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(botBorder)
	b.WriteByte('\n')

	// === Message or fill ===
	if m.message != "" {
		msgLine := bgFillStyle.Render(strings.Repeat("░", padL)) +
			flashMessageStyle.Render(" "+padRight(m.message, boxW)) +
			bgFillStyle.Render(strings.Repeat("░", max(0, padR+1)))
		b.WriteString(msgLine)
	} else {
		b.WriteString(bg)
	}
	b.WriteByte('\n')

	for i := 0; i < bottomPad; i++ {
		b.WriteString(bg)
		b.WriteByte('\n')
	}

	// === Help bar ===
	// MENUEDIT.PAS: 'F2 Delete Command  F5 Add New Command  ALT-H Help  ESC Exits'
	helpText := centerText("(Enter) Edit  F2 Delete  F5 Add New Command  ESC Back", m.width)
	b.WriteString(helpBarStyle.Render(helpText))

	// Overlay dialogs
	result := b.String()
	if m.mode == modeDeleteCmdConfirm {
		desc := ""
		if m.cmdCursor >= 0 && m.cmdCursor < len(m.cmds) {
			desc = m.cmds[m.cmdCursor].NodeActivity
			if desc == "" {
				desc = m.cmds[m.cmdCursor].Keys
			}
		}
		result = m.overlayConfirmDialog(result,
			"-- Delete Command --",
			fmt.Sprintf("Delete command '%s'? ", desc))
	}

	return result
}

// renderCmdRow renders a single command entry row in the list.
func (m Model) renderCmdRow(idx int, boxW int) string {
	cmd := m.cmds[idx]
	isHighlight := idx == m.cmdCursor

	// Columns: node activity (22), keys (10), command (rest)
	activity := padRight(cmd.NodeActivity, 22)
	keys := padRight(cmd.Keys, 10)
	command := padRight(cmd.Command, boxW-36)
	content := "   " + activity + keys + command
	if len(content) < boxW {
		content += strings.Repeat(" ", boxW-len(content))
	} else if len(content) > boxW {
		content = content[:boxW]
	}

	if isHighlight {
		return listHighlightStyle.Render(content)
	}
	return listItemStyle.Render(content)
}
