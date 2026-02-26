package configeditor

import (
	"strings"
)

// overlayConfirmDialog renders a confirmation dialog centered over the background.
func (m Model) overlayConfirmDialog(background, title, question string) string {
	lines := strings.Split(background, "\n")

	dialogW := 62
	dialogH := 7
	startRow := (m.height - dialogH) / 2
	startCol := (m.width - dialogW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	border := dialogBorderStyle.Render("┌" + strings.Repeat("─", dialogW-2) + "┐")
	borderBot := dialogBorderStyle.Render("└" + strings.Repeat("─", dialogW-2) + "┘")
	side := dialogBorderStyle.Render("│")

	// Title line
	titlePad := (dialogW - 2 - len(title)) / 2
	if titlePad < 0 {
		titlePad = 0
	}
	titleLine := side +
		dialogTitleStyle.Render(strings.Repeat(" ", titlePad)+title+strings.Repeat(" ", maxInt(0, dialogW-2-titlePad-len(title)))) +
		side

	// Empty line
	emptyLine := side +
		dialogTextStyle.Render(strings.Repeat(" ", dialogW-2)) +
		side

	// Question line
	qPad := (dialogW - 2 - len(question)) / 2
	if qPad < 0 {
		qPad = 0
	}
	questionLine := side +
		dialogTextStyle.Render(strings.Repeat(" ", qPad)+question+strings.Repeat(" ", maxInt(0, dialogW-2-qPad-len(question)))) +
		side

	// Button line
	btnVisW := 11
	var yesBtn, noBtn string
	if m.confirmYes {
		yesBtn = buttonActiveStyle.Render(" Yes ")
		noBtn = buttonInactiveStyle.Render(" No ")
	} else {
		yesBtn = buttonInactiveStyle.Render(" Yes ")
		noBtn = buttonActiveStyle.Render(" No ")
	}
	btnGap := dialogTextStyle.Render("  ")
	btnContent := yesBtn + btnGap + noBtn
	btnPad := (dialogW - 2 - btnVisW) / 2
	buttonLine := side +
		dialogTextStyle.Render(strings.Repeat(" ", maxInt(0, btnPad))) +
		btnContent +
		dialogTextStyle.Render(strings.Repeat(" ", maxInt(0, dialogW-2-btnPad-btnVisW))) +
		side

	dialogLines := []string{border, titleLine, emptyLine, questionLine, emptyLine, buttonLine, borderBot}

	// Overlay dialog on background, preserving content on both sides
	endCol := startCol + dialogW
	for i, dl := range dialogLines {
		row := startRow + i
		if row >= 0 && row < len(lines) {
			left := padToCol(lines[row], startCol)
			right := skipToCol(lines[row], endCol)
			lines[row] = left + dl + right
		}
	}

	return strings.Join(lines, "\n")
}

// overlayHelpScreen renders the help screen overlay.
func (m Model) overlayHelpScreen(background string) string {
	lines := strings.Split(background, "\n")

	dialogW := 50
	dialogH := 19
	startRow := (m.height - dialogH) / 2
	startCol := (m.width - dialogW) / 2
	if startRow < 0 {
		startRow = 0
	}

	helpBorder := helpBoxStyle
	helpTitle := helpTitleStyle

	border := helpBorder.Render("┌" + strings.Repeat("─", dialogW-2) + "┐")
	borderBot := helpBorder.Render("└" + strings.Repeat("─", dialogW-2) + "┘")
	side := helpBorder.Render("│")

	helpLines := []string{
		border,
		side + helpTitle.Render(centerText("V/3 Configuration Editor Help", dialogW-2)) + side,
		side + helpBorder.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Enter - Select / Edit", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Up/Down - Navigate", dialogW-2)) + side,
		side + helpBorder.Render(centerText("PgUp/PgDn - Previous/Next Screen", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Home/End - First/Last Item", dialogW-2)) + side,
		side + helpBorder.Render(centerText("ESC - Go Back / Cancel Edit", dialogW-2)) + side,
		side + helpBorder.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Record List Keys:", dialogW-2)) + side,
		side + helpBorder.Render(centerText("I/Insert - Add New Record", dialogW-2)) + side,
		side + helpBorder.Render(centerText("D/Delete - Delete Record", dialogW-2)) + side,
		side + helpBorder.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpBorder.Render(centerText("1-9, A - Quick Menu Selection", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Q - Quit Program", dialogW-2)) + side,
		side + helpBorder.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpTitle.Render(centerText("HIT A KEY.", dialogW-2)) + side,
		borderBot,
	}

	endCol := startCol + dialogW
	for i, hl := range helpLines {
		row := startRow + i
		if row >= 0 && row < len(lines) {
			left := padToCol(lines[row], startCol)
			right := skipToCol(lines[row], endCol)
			lines[row] = left + hl + right
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

// approximateVisibleLen estimates the visible length of a styled string.
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

// skipToCol returns everything from visible column n onward.
func skipToCol(s string, n int) string {
	var lastESC strings.Builder
	var curESC strings.Builder
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
			return lastESC.String() + s[i:]
		}
		count++
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
