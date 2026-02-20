package stringeditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles matching the Pascal original's color scheme:
//
//	NormalColor  = 8  (Dark Gray)
//	BoldColor    = 15 (White)
//	BarColor     = 95 (White on Magenta → we use White on Blue for status)
//	InputColor   = 31 (White on Blue)
//	ChoiceColor  = 15 (White)
//	DataColor    = 9  (Light Blue)
var (
	// Status bar styles matching Pascal's SetColor() attributes:
	//   SetColor(16) = bg=blue, fg=black  (fill + separators)
	//   SetColor(25) = bg=blue, fg=light blue (labels)
	//   SetColor(31) = bg=blue, fg=white (values)
	//   SetColor(27) = bg=blue, fg=light cyan (page label)
	statusBarFillStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("4"))

	statusBarLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Background(lipgloss.Color("4"))

	statusBarValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("4")).
				Bold(true)

	statusBarPageLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("14")).
				Background(lipgloss.Color("4"))

	// Item label in bracket: dark gray
	bracketStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	// Normal item label
	labelNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7"))

	// Highlighted item: white on magenta bar (Pascal BarColor=95 → bg=5, fg=15)
	labelHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("5")).
				Bold(true)

	bracketHighlightStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("14")).
				Background(lipgloss.Color("5"))

	// Description bar (row 24): magenta text, centered
	descriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("13"))

	// Dialog box styles
	dialogBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("4"))

	dialogTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Background(lipgloss.Color("4"))

	dialogButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")).
				Background(lipgloss.Color("4"))

	dialogButtonActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("5")).
				Bold(true)

	// Search bar
	searchLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("11")).
				Bold(true)

	// Message flash
	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	// Title/header
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true)

	// Edit mode indicator
	editingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Background(lipgloss.Color("4")).
			Bold(true)

	// Dirty indicator
	dirtyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)
)

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	// === Row 1: Status Bar ===
	b.WriteString(m.renderStatusBar())
	b.WriteByte('\n')

	// === Row 2: Separator / Column Headers ===
	b.WriteString(m.renderColumnHeader())
	b.WriteByte('\n')

	// === Rows 3-22: Item List (20 items per page) ===
	pageStart := m.page * itemsPerPage
	pageEnd := pageStart + itemsPerPage
	if pageEnd > len(m.entries) {
		pageEnd = len(m.entries)
	}

	for row := 0; row < itemsPerPage; row++ {
		idx := pageStart + row
		if idx < pageEnd {
			b.WriteString(m.renderItem(idx))
		} else {
			// Empty row filler
			b.WriteString(strings.Repeat(" ", m.width))
		}
		b.WriteByte('\n')
	}

	// === Row 23: Message / Mode indicator ===
	b.WriteString(m.renderMessageBar())
	b.WriteByte('\n')

	// === Row 24: Description Bar ===
	b.WriteString(m.renderDescriptionBar())
	b.WriteByte('\n')

	// === Row 25: Help Bar ===
	b.WriteString(m.renderHelpBar())

	// === Overlay: Abort Confirm Dialog ===
	if m.mode == modeAbortConfirm {
		return m.overlayDialog(b.String())
	}

	return b.String()
}

// renderStatusBar creates the top status bar matching the Pascal original.
// Pascal gotopage procedure row 1 layout:
//
//	SetColor(16); ClrEOL;  (fill entire row blue)
//	SetColor(25); Write(' Current Topic Number:');
//	SetColor(31); Write(' '+strr(top));
//	SetColor(16); Write('│');
//	SetColor(25); Write(' ViSiON/2 BBS String Configuration');
//	SetColor(16); Write(' │');
//	SetColor(27); Write(' Current Page:');
//	SetColor(31); Write(' '+strr(page));
func (m Model) renderStatusBar() string {
	// First item on current page (1-based, matching Pascal's top variable)
	topItem := m.page*itemsPerPage + 1
	pageNum := m.page + 1

	// Build segments exactly matching Pascal's SetColor/Write sequence
	seg1 := statusBarLabelStyle.Render(" Current Topic Number:")
	seg2 := statusBarValueStyle.Render(fmt.Sprintf(" %d", topItem))
	sep1 := statusBarFillStyle.Render(" │")
	seg3 := statusBarLabelStyle.Render(" ViSiON/3 BBS String Configuration")
	sep2 := statusBarFillStyle.Render(" │")
	seg4 := statusBarPageLabelStyle.Render(" Current Page:")
	seg5 := statusBarValueStyle.Render(fmt.Sprintf(" %d", pageNum))

	content := seg1 + seg2 + sep1 + seg3 + sep2 + seg4 + seg5

	// Calculate visible length (display columns, not bytes) to pad the rest with blue background.
	// Use a plain ASCII format string so len() == display width.
	plainText := fmt.Sprintf(" Current Topic Number: %d | ViSiON/3 BBS String Configuration | Current Page: %d", topItem, pageNum)
	visLen := len(plainText)

	padding := m.width - visLen
	if padding < 0 {
		padding = 0
	}

	return content + statusBarFillStyle.Render(strings.Repeat(" ", padding))
}

// renderColumnHeader creates a subtle column header line.
func (m Model) renderColumnHeader() string {
	header := bracketStyle.Render("  # ") +
		bracketStyle.Render("Name") +
		bracketStyle.Render(strings.Repeat(" ", labelCol-9)) +
		bracketStyle.Render("Value")
	pad := m.width - labelCol - 5 + 4
	if pad > 0 {
		header += bracketStyle.Render(strings.Repeat(" ", pad))
	}
	return header
}

// renderItem renders a single list item.
func (m Model) renderItem(idx int) string {
	entry := m.entries[idx]
	isSelected := idx == m.cursor
	itemNum := idx + 1
	value := m.getValue(entry.Key)
	valueWidth := m.width - labelCol - 1
	if valueWidth < 10 {
		valueWidth = 10
	}

	// Format item number (3 chars wide)
	numStr := fmt.Sprintf("%3d", itemNum)

	var line string
	if isSelected {
		if m.mode == modeEdit {
			// Show text input in the value area
			numPart := bracketHighlightStyle.Render(numStr)
			bracket1 := bracketHighlightStyle.Render("[")
			label := labelHighlightStyle.Render(padOrTrunc(entry.Label, labelCol-7))
			bracket2 := bracketHighlightStyle.Render("]")
			line = numPart + bracket1 + label + bracket2 + m.textInput.View()
		} else {
			// Highlighted item
			numPart := bracketHighlightStyle.Render(numStr)
			bracket1 := bracketHighlightStyle.Render("[")
			label := labelHighlightStyle.Render(padOrTrunc(entry.Label, labelCol-7))
			bracket2 := bracketHighlightStyle.Render("]")
			renderedVal := RenderColorString(value, valueWidth)
			line = numPart + bracket1 + label + bracket2 + renderedVal
		}
	} else {
		// Normal item
		numPart := bracketStyle.Render(numStr)
		bracket1 := bracketStyle.Render("[")
		label := labelNormalStyle.Render(padOrTrunc(entry.Label, labelCol-7))
		bracket2 := bracketStyle.Render("]")
		renderedVal := RenderColorString(value, valueWidth)
		line = numPart + bracket1 + label + bracket2 + renderedVal
	}

	return line
}

// renderMessageBar renders the message/mode indicator line.
func (m Model) renderMessageBar() string {
	switch m.mode {
	case modeEdit:
		key := m.editKey
		return editingStyle.Render(fmt.Sprintf(" Editing: %s (Enter=Save, Esc=Cancel)", key)) +
			strings.Repeat(" ", max(0, m.width-50-len(key)))
	case modeSearch:
		return searchLabelStyle.Render(" Search: ") + m.searchInput.View() +
			strings.Repeat(" ", max(0, m.width-40))
	default:
		if m.message != "" {
			return messageStyle.Render(" "+m.message) + strings.Repeat(" ", max(0, m.width-len(m.message)-2))
		}
		return strings.Repeat(" ", m.width)
	}
}

// renderDescriptionBar renders the description for the current item (row 24).
// In the Pascal original, this is centered magenta text.
func (m Model) renderDescriptionBar() string {
	if m.cursor >= 0 && m.cursor < len(m.entries) {
		desc := m.entries[m.cursor].Description
		if desc == "" {
			desc = m.entries[m.cursor].Key
		}
		// Center the description text
		pad := (m.width - len(desc)) / 2
		if pad < 0 {
			pad = 0
		}
		return strings.Repeat(" ", pad) + descriptionStyle.Render(desc)
	}
	return strings.Repeat(" ", m.width)
}

// renderHelpBar renders the bottom help bar.
func (m Model) renderHelpBar() string {
	help := " ↑↓ Navigate │ PgUp/PgDn Pages │ Enter Edit │ F1 Edit(Prefill) │ F10 Save │ Esc Quit │ / Search"
	visLen := 0
	for _, r := range help {
		_ = r
		visLen++
	}
	pad := m.width - visLen
	if pad < 0 {
		pad = 0
	}
	padded := help + strings.Repeat(" ", pad)
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")).
		Background(lipgloss.Color("4")).
		Bold(true)
	return style.Render(padded)
}

// overlayDialog renders the abort confirmation dialog centered over the content.
func (m Model) overlayDialog(background string) string {
	lines := strings.Split(background, "\n")

	// Dialog dimensions
	dialogW := 30
	dialogH := 5

	// Calculate center position
	startRow := (m.height - dialogH) / 2
	startCol := (m.width - dialogW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	// Build dialog lines
	border := dialogBorderStyle.Render("╔" + strings.Repeat("═", dialogW-2) + "╗")
	borderBot := dialogBorderStyle.Render("╚" + strings.Repeat("═", dialogW-2) + "╝")
	borderSide := dialogBorderStyle.Render("║")

	title := "Abort Without Saving?"
	titlePad := (dialogW - 2 - len(title)) / 2
	titleLine := borderSide +
		dialogTextStyle.Render(strings.Repeat(" ", titlePad)+title+strings.Repeat(" ", dialogW-2-titlePad-len(title))) +
		borderSide

	emptyLine := borderSide +
		dialogTextStyle.Render(strings.Repeat(" ", dialogW-2)) +
		borderSide

	// Buttons
	var yesBtn, noBtn string
	if m.abortYes {
		yesBtn = dialogButtonActiveStyle.Render(" Yes ")
		noBtn = dialogButtonStyle.Render(" No  ")
	} else {
		yesBtn = dialogButtonStyle.Render(" Yes ")
		noBtn = dialogButtonActiveStyle.Render(" No  ")
	}
	btnPad := (dialogW - 2 - 12) / 2 // 5+2+5 = 12 visible chars
	buttonLine := borderSide +
		dialogTextStyle.Render(strings.Repeat(" ", btnPad)) +
		yesBtn + dialogTextStyle.Render("  ") + noBtn +
		dialogTextStyle.Render(strings.Repeat(" ", max(0, dialogW-2-btnPad-12))) +
		borderSide

	dialogLines := []string{border, titleLine, emptyLine, buttonLine, borderBot}

	// Overlay dialog on background
	for i, dl := range dialogLines {
		row := startRow + i
		if row >= 0 && row < len(lines) {
			line := lines[row]
			// Replace the portion of the line with the dialog
			// This is approximate since lines contain ANSI codes
			runeLen := visualLen(line)
			if startCol+dialogW <= runeLen {
				lines[row] = truncateVisual(line, startCol) + dl + skipVisual(line, startCol+dialogW)
			} else {
				lines[row] = truncateVisual(line, startCol) + dl
			}
		}
	}

	return strings.Join(lines, "\n")
}

// padOrTrunc pads or truncates a string to exactly width characters.
func padOrTrunc(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}

// visualLen returns an approximate visible length (counting runes, ignoring ANSI).
func visualLen(s string) int {
	// Strip ANSI escape sequences for length calculation
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

// truncateVisual returns the first n visible characters (preserving ANSI codes).
func truncateVisual(s string, n int) string {
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

// skipVisual skips the first n visible characters and returns the rest.
func skipVisual(s string, n int) string {
	inEsc := false
	count := 0
	for i, r := range s {
		if count >= n && !inEsc {
			return s[i:]
		}
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
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
