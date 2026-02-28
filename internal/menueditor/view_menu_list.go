package menueditor

import (
	"fmt"
	"strings"
)

// viewMenuListScreen renders the main menu browser.
// Faithfully recreates MENUEDIT.PAS Open_Screen + Select_Menu.
func (m Model) viewMenuListScreen() string {
	var b strings.Builder

	// === Row 1: Title bar ===
	// MENUEDIT.PAS: Color(8,15) Center_Write('ViSiON/2 MENU EDITOR v1.0')
	title := centerText("-- ViSiON/3 Menu Editor v1.0 --", m.width)
	b.WriteString(titleBarStyle.Render(title))
	b.WriteByte('\n')

	bg := m.bgLine()

	// Vertical centering: 1 title + box(19) + message(1) + help(1) = 22 rows
	extraV := max(0, m.height-22)
	topPad := max(1, extraV/2)
	bottomPad := max(1, extraV-topPad)

	for i := 0; i < topPad; i++ {
		b.WriteString(bg)
		b.WriteByte('\n')
	}

	// Box dimensions: MENUEDIT.PAS GrowBox(15,5,65,22) → 50 cols wide
	boxW := 50
	padL := max(0, (m.width-boxW-2)/2)
	padR := max(0, m.width-padL-boxW-2)

	// === Top border ===
	// MENUEDIT.PAS: Color(3,11) GrowBox → cyan bg, light cyan fg
	topBorder := bgFillStyle.Render(strings.Repeat("░", padL)) +
		listBorderStyle.Render("╒"+strings.Repeat("═", boxW)+"╕") +
		bgFillStyle.Render(strings.Repeat("░", max(0, padR)))
	b.WriteString(topBorder)
	b.WriteByte('\n')

	// === Column header ===
	// MENUEDIT.PAS: Color(11,3) ' Menu Title           File Names'
	colHeader := padRight(" Menu Name           File Names                ", boxW)
	colLine := bgFillStyle.Render(strings.Repeat("░", padL)) +
		listBorderStyle.Render("│") +
		listColTitleStyle.Render(colHeader) +
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

	// === Menu list (listVisible rows) ===
	total := len(m.menus)
	for row := 0; row < listVisible; row++ {
		idx := m.menuScroll + row
		var rowContent string
		if idx < 0 || idx >= total {
			rowContent = listItemStyle.Render(strings.Repeat(" ", boxW))
		} else {
			rowContent = m.renderMenuRow(idx, boxW)
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
		listBorderStyle.Render("╘"+strings.Repeat("═", boxW)+"╛") +
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
	// MENUEDIT.PAS: '(CR) Edits Menu  F10 Edits Menu Commands  F2 Delete  F5 Add Menu  ESC Exits'
	helpText := centerText("(Enter) Edit Menu  F10 Commands  F2 Delete  F5 Add Menu  ESC Exit", m.width)
	b.WriteString(helpBarStyle.Render(helpText))

	// Overlay dialogs
	result := b.String()
	switch m.mode {
	case modeDeleteMenuConfirm:
		name := ""
		if m.menuCursor >= 0 && m.menuCursor < len(m.menus) {
			name = m.menus[m.menuCursor].Name
		}
		result = m.overlayConfirmDialog(result,
			"-- Delete Menu --",
			fmt.Sprintf("Delete %s.MNU and all commands? ", name))
	case modeExitConfirm:
		result = m.overlayConfirmDialog(result,
			"-- Unsaved Changes --",
			"Save all changes before exit? ")
	case modeAddMenu:
		result = m.overlayInputDialog(result,
			"-- Create New Menu --",
			"New menu filename (8 chars): ",
			m.textInput.View())
	case modeHelp:
		result = m.overlayHelpScreen(result)
	}

	return result
}

// renderMenuRow renders a single menu entry row in the list.
func (m Model) renderMenuRow(idx int, boxW int) string {
	entry := m.menus[idx]
	isHighlight := idx == m.menuCursor

	// Build: " {name:20} {name}.MNU / {name}.CFG"
	name := padRight(entry.Name, 20)
	files := fmt.Sprintf("%s.MNU / %s.CFG", entry.Name, entry.Name)
	files = padRight(files, boxW-22)
	content := " " + name + " " + files
	if len(content) < boxW {
		content += strings.Repeat(" ", boxW-len(content))
	} else if len(content) > boxW {
		content = content[:boxW]
	}

	if isHighlight {
		// MENUEDIT.PAS: Color(1,15) for highlighted row
		return listHighlightStyle.Render(content)
	}
	return listItemStyle.Render(content)
}
