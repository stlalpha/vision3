package menueditor

import (
	"strings"
)

// overlayConfirmDialog renders a Y/N confirm dialog centered over the background.
// Recreates MENUEDIT.PAS Ask() with GrowBox + title + Y/N buttons.
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

	border := dialogBorderStyle.Render("╔" + strings.Repeat("═", dialogW-2) + "╗")
	borderBot := dialogBorderStyle.Render("╚" + strings.Repeat("═", dialogW-2) + "╝")
	side := dialogBorderStyle.Render("║")

	titlePad := (dialogW - 2 - len(title)) / 2
	if titlePad < 0 {
		titlePad = 0
	}
	titleLine := side +
		dialogTitleStyle.Render(strings.Repeat(" ", titlePad)+title+strings.Repeat(" ", max(0, dialogW-2-titlePad-len(title)))) +
		side

	emptyLine := side +
		dialogTextStyle.Render(strings.Repeat(" ", dialogW-2)) +
		side

	qPad := (dialogW - 2 - len(question)) / 2
	if qPad < 0 {
		qPad = 0
	}
	questionLine := side +
		dialogTextStyle.Render(strings.Repeat(" ", qPad)+question+strings.Repeat(" ", max(0, dialogW-2-qPad-len(question)))) +
		side

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

// overlayInputDialog renders a text-input dialog centered over the background.
// Used for the "Create New Menu" filename prompt.
func (m Model) overlayInputDialog(background, title, prompt, inputView string) string {
	lines := strings.Split(background, "\n")

	dialogW := 54
	dialogH := 6
	startRow := (m.height - dialogH) / 2
	startCol := (m.width - dialogW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	border := inputDialogBorderStyle.Render("╔" + strings.Repeat("═", dialogW-2) + "╗")
	borderBot := inputDialogBorderStyle.Render("╚" + strings.Repeat("═", dialogW-2) + "╝")
	side := inputDialogBorderStyle.Render("║")

	titlePad := (dialogW - 2 - len(title)) / 2
	if titlePad < 0 {
		titlePad = 0
	}
	titleLine := side +
		dialogTitleStyle.Render(strings.Repeat(" ", titlePad)+title+strings.Repeat(" ", max(0, dialogW-2-titlePad-len(title)))) +
		side

	emptyLine := side +
		inputDialogTextStyle.Render(strings.Repeat(" ", dialogW-2)) +
		side

	promptPad := 1
	promptLine := side +
		inputDialogTextStyle.Render(strings.Repeat(" ", promptPad)+prompt) +
		side

	inputLine := side +
		inputDialogTextStyle.Render("  ") +
		inputView +
		inputDialogTextStyle.Render(strings.Repeat(" ", max(0, dialogW-4-m.textInput.Width))) +
		side

	hintLine := side +
		inputDialogTextStyle.Render(centerText("Enter=Confirm  ESC=Cancel", dialogW-2)) +
		side

	dialogLines := []string{border, titleLine, emptyLine, promptLine, inputLine, hintLine, borderBot}

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

// overlayHelpScreen renders the help overlay.
func (m Model) overlayHelpScreen(background string) string {
	lines := strings.Split(background, "\n")

	dialogW := 50
	startRow := (m.height - 20) / 2
	startCol := (m.width - dialogW) / 2
	if startRow < 0 {
		startRow = 0
	}

	border := helpBoxStyle.Render("╔" + strings.Repeat("═", dialogW-2) + "╗")
	borderBot := helpBoxStyle.Render("╚" + strings.Repeat("═", dialogW-2) + "╝")
	side := helpBoxStyle.Render("║")

	helpLines := []string{
		border,
		side + helpTitleStyle.Render(centerText("ViSiON/3 Menu Editor Help", dialogW-2)) + side,
		side + helpBoxStyle.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("Menu List Screen", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("Up/Down/PgUp/PgDn/Home/End - Navigate", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("Enter - Edit Highlighted Menu", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("F10 - Edit Commands for Menu", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("F2  - Delete Menu", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("F5  - Add New Menu", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("ESC - Exit Program", dialogW-2)) + side,
		side + helpBoxStyle.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("Menu Edit Screen", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("Up/Down/Tab - Navigate Fields", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("Enter - Edit Field  (Y/N = toggle)", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("PgUp/PgDn - Prev/Next Menu", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("F10 - Jump to Command List", dialogW-2)) + side,
		side + helpBoxStyle.Render(centerText("ESC - Save and Return to List", dialogW-2)) + side,
		side + helpBoxStyle.Render(strings.Repeat(" ", dialogW-2)) + side,
		side + helpTitleStyle.Render(centerText("Press any key to close.", dialogW-2)) + side,
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
