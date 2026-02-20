package usereditor

import (
	"strings"
)

// overlayConfirmDialog renders a confirmation dialog centered over the background.
// Recreates the UE.PAS Ask() procedure with GrowBox + centered title + Y/N buttons.
func (m Model) overlayConfirmDialog(background, title, question string) string {
	lines := strings.Split(background, "\n")

	// Dialog dimensions matching UE.PAS Ask(9,9,71,15)
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

	// Build dialog lines
	// UE.PAS: Ask_Colors → PColor=94 (magenta bg, yellow fg), NormColor=95 (magenta bg, white fg)
	border := dialogBorderStyle.Render("╔" + strings.Repeat("═", dialogW-2) + "╗")
	borderBot := dialogBorderStyle.Render("╚" + strings.Repeat("═", dialogW-2) + "╝")
	side := dialogBorderStyle.Render("║")

	// Title line
	titlePad := (dialogW - 2 - len(title)) / 2
	if titlePad < 0 {
		titlePad = 0
	}
	titleLine := side +
		dialogTitleStyle.Render(strings.Repeat(" ", titlePad)+title+strings.Repeat(" ", max(0, dialogW-2-titlePad-len(title)))) +
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
		dialogTextStyle.Render(strings.Repeat(" ", qPad)+question+strings.Repeat(" ", max(0, dialogW-2-qPad-len(question)))) +
		side

	// Button line: " Yes " (5) + "  " (2) + " No " (4) = 11 visible chars
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
		dialogTextStyle.Render(strings.Repeat(" ", max(0, btnPad))) +
		btnContent +
		dialogTextStyle.Render(strings.Repeat(" ", max(0, dialogW-2-btnPad-btnVisW))) +
		side

	dialogLines := []string{border, titleLine, emptyLine, questionLine, emptyLine, buttonLine, borderBot}

	// Overlay dialog on background, filling remaining width with styled ░
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

// overlayHelpScreen renders the help screen overlay.
// Recreates UE.PAS Help_Screen: Color(4,15) GrowBox(18,4,62,20)
func (m Model) overlayHelpScreen(background string) string {
	lines := strings.Split(background, "\n")

	dialogW := 44
	dialogH := 17 // number of lines in helpLines below
	startRow := (m.height - dialogH) / 2
	startCol := (m.width - dialogW) / 2
	if startRow < 0 {
		startRow = 0
	}

	// Help box styles: Red bg, white fg
	helpBorder := helpBoxStyle
	helpTitle := helpTitleStyle

	border := helpBorder.Render("╔" + strings.Repeat("═", dialogW-2) + "╗")
	borderBot := helpBorder.Render("╚" + strings.Repeat("═", dialogW-2) + "╝")
	side := helpBorder.Render("║")

	helpLines := []string{
		border,
		side + helpTitle.Render(centerText("V/3 User Editor Help", dialogW-2)) + side,
		side + helpBorder.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Enter - Edit Highlighted User", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Up/Down/End/Home/PgUp/PgDn - Scroll", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Left/Right Arrow: Scroll User Data", dialogW-2)) + side,
		side + helpBorder.Render(centerText("F3 - Alphabetize / De-Alphabetize", dialogW-2)) + side,
		side + helpBorder.Render(centerText("F2 - Delete Highlighted User", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Shift-F2 - Delete All Tagged Users", dialogW-2)) + side,
		side + helpBorder.Render(centerText("F5 - Auto Validate Highlighted User", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Shift-F5 - Validate All Tagged Users", dialogW-2)) + side,
		side + helpBorder.Render(centerText("F10 - Tag All  /  Shift-F10 - Untag All", dialogW-2)) + side,
		side + helpBorder.Render(centerText("Space - Toggle Tag on User", dialogW-2)) + side,
		side + helpBorder.Render(centerText("ESC - Exit Program", dialogW-2)) + side,
		side + helpBorder.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpTitle.Render(centerText("HIT A KEY.", dialogW-2)) + side,
		borderBot,
	}

	// Overlay dialog on background, preserving content on both sides
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

