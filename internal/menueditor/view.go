package menueditor

import (
	"strings"
)

// View implements tea.Model.
func (m Model) View() string {
	switch m.mode {
	case modeMenuEdit, modeMenuEditField:
		return m.viewMenuEditScreen()
	case modeCommandList, modeDeleteCmdConfirm:
		return m.viewCommandListScreen()
	case modeCommandEdit, modeCommandEditField:
		return m.viewCommandEditScreen()
	default:
		return m.viewMenuListScreen()
	}
}

// bgLine returns a full-width background fill line.
func (m Model) bgLine() string {
	return bgFillStyle.Render(strings.Repeat("░", m.width))
}

// boxTopBorder renders the top border of a box at given width.
func boxTopBorder(boxW int, borderStyle interface{ Render(...string) string }) string {
	return borderStyle.Render("╒" + strings.Repeat("═", boxW) + "╕")
}

// boxBotBorder renders the bottom border of a box at given width.
func boxBotBorder(boxW int, borderStyle interface{ Render(...string) string }) string {
	return borderStyle.Render("╘" + strings.Repeat("═", boxW) + "╛")
}

// padToCol truncates or pads a line to reach a specific visible column.
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

// skipToCol returns everything in a string from visible column n onward,
// replaying the last active ANSI escape so styling is preserved.
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
