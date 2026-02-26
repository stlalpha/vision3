package configeditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewSysConfigEdit renders the system config field editor.
func (m Model) viewSysConfigEdit() string {
	var b strings.Builder

	// Global header
	b.WriteString(m.globalHeaderLine())
	b.WriteByte('\n')

	screenName := ""
	if m.sysSubScreen < len(m.sysMenuItems) {
		screenName = m.sysMenuItems[m.sysSubScreen].Label
	}

	bgLine := bgFillStyle.Render(strings.Repeat("░", m.width))

	// Box dimensions
	boxW := 70
	// Find max row in fields
	maxRow := 0
	for _, f := range m.sysFields {
		if f.Row > maxRow {
			maxRow = f.Row
		}
	}
	visibleRows := maxRow
	if visibleRows > maxFieldRows {
		visibleRows = maxFieldRows
	}
	// Fixed rows: globalheader(1) + box(border+header+empty+visibleRows+empty+info+border = visibleRows+6) + helptxt(1) + bgline(1) + helpbar(1)
	// Total fixed = visibleRows + 10
	extraV := maxInt(0, m.height-visibleRows-10)
	topPad := extraV / 2
	bottomPad := extraV - topPad

	for i := 0; i < topPad; i++ {
		b.WriteString(bgLine)
		b.WriteByte('\n')
	}

	padL := maxInt(0, (m.width-boxW-2)/2)
	padR := maxInt(0, m.width-padL-boxW-2)

	// Top border
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("┌"+strings.Repeat("─", boxW)+"┐") +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Header
	headerLine := editBorderStyle.Render("│") +
		menuHeaderStyle.Render(centerText(screenName, boxW)) +
		editBorderStyle.Render("│")
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) + headerLine +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Empty line
	emptyLine := bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("│") +
		fieldDisplayStyle.Render(strings.Repeat(" ", boxW)) +
		editBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR)))
	b.WriteString(emptyLine)
	b.WriteByte('\n')

	// Field rows (windowed by fieldScroll)
	firstRow := m.fieldScroll + 1
	lastRow := m.fieldScroll + visibleRows
	if lastRow > maxRow {
		lastRow = maxRow
	}
	for row := firstRow; row <= lastRow; row++ {
		rowContent := m.renderSysEditRow(row, boxW)
		line := bgFillStyle.Render(strings.Repeat("░", padL)) +
			editBorderStyle.Render("│") +
			rowContent +
			editBorderStyle.Render("│") +
			bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR)))
		b.WriteString(line)
		b.WriteByte('\n')
	}
	// Pad remaining rows if fewer fields than visibleRows
	for row := lastRow + 1; row <= m.fieldScroll+visibleRows; row++ {
		b.WriteString(emptyLine)
		b.WriteByte('\n')
	}

	// Empty line
	b.WriteString(emptyLine)
	b.WriteByte('\n')

	// Screen navigation info
	scrollHint := ""
	if maxRow > maxFieldRows {
		if m.fieldScroll > 0 && lastRow < maxRow {
			scrollHint = " [▲▼ more]"
		} else if m.fieldScroll > 0 {
			scrollHint = " [▲ more]"
		} else if lastRow < maxRow {
			scrollHint = " [▼ more]"
		}
	}
	infoText := fmt.Sprintf("Screen %d of %d%s", m.sysSubScreen+1, len(m.sysMenuItems), scrollHint)
	infoLine := editBorderStyle.Render("│") +
		editInfoLabelStyle.Render(centerText(infoText, boxW)) +
		editBorderStyle.Render("│")
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) + infoLine +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Bottom border
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) +
		editBorderStyle.Render("└"+strings.Repeat("─", boxW)+"┘") +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	for i := 0; i < bottomPad; i++ {
		b.WriteString(bgLine)
		b.WriteByte('\n')
	}

	// Message or field help text
	b.WriteString(m.renderFieldHelpLine(m.sysFields, padL, padR, boxW))
	b.WriteByte('\n')

	b.WriteString(bgLine)
	b.WriteByte('\n')

	helpText := centerText("Enter - Edit  |  PgUp/PgDn - Screens  |  ESC - Return", m.width)
	b.WriteString(helpBarStyle.Render(helpText))

	return b.String()
}

// renderSysEditRow renders a single row of system config fields.
func (m Model) renderSysEditRow(row, boxW int) string {
	var fieldStr string

	for i, f := range m.sysFields {
		if f.Row != row {
			continue
		}
		fieldStr, _ = m.renderSysField(i, f)
	}

	if fieldStr == "" {
		return fieldDisplayStyle.Render(strings.Repeat(" ", boxW))
	}

	// Pad field content to fill the box
	padBefore := 2 // indent
	// Use actual visual width to avoid blow-out from multi-byte characters
	padAfter := boxW - padBefore - lipgloss.Width(fieldStr)
	if padAfter < 0 {
		padAfter = 0
	}

	return fieldDisplayStyle.Render(strings.Repeat(" ", padBefore)) +
		fieldStr +
		fieldDisplayStyle.Render(strings.Repeat(" ", padAfter))
}

// renderSysField renders a single system config field.
func (m Model) renderSysField(fieldIdx int, f fieldDef) (string, int) {
	isActive := m.editField == fieldIdx

	labelText := padRight(f.Label, 16)
	label := labelText + " : "
	labelLen := len(label)

	var value string
	if f.Get != nil {
		value = f.Get()
	}

	rawW := labelLen + f.Width

	if isActive && m.mode == modeSysConfigField {
		return fieldLabelStyle.Render(label) + m.textInput.View(), rawW
	}

	displayValue := padRight(value, f.Width)

	if isActive && m.mode == modeSysConfigEdit {
		v := value
		if len(v) > f.Width {
			v = v[:f.Width]
		}
		fillStr := strings.Repeat(string(fieldFillChar), maxInt(0, f.Width-len(v)))
		return fieldLabelStyle.Render(label) + fieldEditStyle.Render(v+fillStr), rawW
	}

	return fieldLabelStyle.Render(label) + fieldDisplayStyle.Render(displayValue), rawW
}
