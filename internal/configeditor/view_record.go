package configeditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// viewRecordEdit renders the single-record field editor popup.
func (m Model) viewRecordEdit() string {
	var b strings.Builder

	// Global header
	b.WriteString(m.globalHeaderLine())
	b.WriteByte('\n')

	bgLine := bgFillStyle.Render(strings.Repeat("░", m.width))

	boxW := 70
	// Find max row in fields
	maxRow := 0
	for _, f := range m.recordFields {
		if f.Row > maxRow {
			maxRow = f.Row
		}
	}
	visibleRows := maxRow
	if visibleRows > maxFieldRows {
		visibleRows = maxFieldRows
	}
	// Fixed rows: globalheader(1) + box(border+boxtitle+header+empty+visibleRows+empty+info+border = visibleRows+7) + helptxt(1) + bgline(1) + helpbar(1)
	// Total fixed = visibleRows + 11
	extraV := maxInt(0, m.height-visibleRows-11)
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

	// Box title
	boxTitleText := fmt.Sprintf("Edit %s", m.recordTypeTitle())
	if m.recordType == "ftn" && m.recordEditIdx < 0 {
		boxTitleText = "FTN Global Settings"
	}
	boxTitleLine := editBorderStyle.Render("│") +
		menuHeaderStyle.Render(centerText(boxTitleText, boxW)) +
		editBorderStyle.Render("│")
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) + boxTitleLine +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Record name header
	headerText := m.recordEditHeader()
	headerLine := editBorderStyle.Render("│") +
		menuHeaderStyle.Render(centerText(headerText, boxW)) +
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
		rowContent := m.renderRecordEditRow(row, boxW)
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

	// Record info
	total := m.recordCount()
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
	infoText := fmt.Sprintf("Record %d of %d%s", m.recordEditIdx+1, total, scrollHint)
	if m.recordEditIdx < 0 {
		infoText = "Global Settings" + scrollHint
	}
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
	b.WriteString(m.renderFieldHelpLine(m.recordFields, padL, padR, boxW))
	b.WriteByte('\n')

	b.WriteString(bgLine)
	b.WriteByte('\n')

	helpBarStr := "Enter - Edit  |  PgUp/PgDn - Records  |  ESC - Return"
	if m.recordEditIdx < 0 {
		helpBarStr = "Enter - Edit  |  ESC - Return"
	}
	helpText := centerText(helpBarStr, m.width)
	b.WriteString(helpBarStyle.Render(helpText))

	return b.String()
}

// recordEditHeader returns a header string for the record edit screen.
func (m Model) recordEditHeader() string {
	switch m.recordType {
	case "msgarea":
		if m.recordEditIdx < len(m.configs.MsgAreas) {
			a := m.configs.MsgAreas[m.recordEditIdx]
			return fmt.Sprintf("%s  (ID: %d)", a.Name, a.ID)
		}
	case "filearea":
		if m.recordEditIdx < len(m.configs.FileAreas) {
			a := m.configs.FileAreas[m.recordEditIdx]
			return fmt.Sprintf("%s  (ID: %d)", a.Name, a.ID)
		}
	case "conference":
		if m.recordEditIdx < len(m.configs.Conferences) {
			c := m.configs.Conferences[m.recordEditIdx]
			return fmt.Sprintf("%s  (ID: %d)", c.Name, c.ID)
		}
	case "door":
		keys := m.doorKeys()
		if m.recordEditIdx < len(keys) {
			return m.configs.Doors[keys[m.recordEditIdx]].Name
		}
	case "event":
		if m.recordEditIdx < len(m.configs.Events.Events) {
			return m.configs.Events.Events[m.recordEditIdx].Name
		}
	case "protocol":
		if m.recordEditIdx < len(m.configs.Protocols) {
			return m.configs.Protocols[m.recordEditIdx].Name
		}
	case "archiver":
		if m.recordEditIdx < len(m.configs.Archivers.Archivers) {
			return m.configs.Archivers.Archivers[m.recordEditIdx].Name
		}
	case "ftn":
		if m.recordEditIdx < 0 {
			return "Paths & Storage"
		}
		keys := m.ftnNetworkKeys()
		if m.recordEditIdx < len(keys) {
			return keys[m.recordEditIdx]
		}
	case "ftnlink":
		refs := m.ftnAllLinkRefs()
		if m.recordEditIdx >= 0 && m.recordEditIdx < len(refs) {
			ref := refs[m.recordEditIdx]
			nc := m.configs.FTN.Networks[ref.networkKey]
			if ref.linkIdx < len(nc.Links) {
				lnk := nc.Links[ref.linkIdx]
				if lnk.Name != "" {
					return fmt.Sprintf("%s  (%s)", lnk.Name, ref.networkKey)
				}
				return fmt.Sprintf("%s  (%s)", lnk.Address, ref.networkKey)
			}
		}
	case "login":
		if m.recordEditIdx < len(m.configs.LoginSeq) {
			return fmt.Sprintf("Step %d", m.recordEditIdx+1)
		}
	}
	return "Edit Record"
}

// renderRecordEditRow renders a single row of record edit fields.
func (m Model) renderRecordEditRow(row, boxW int) string {
	var fieldStr string

	for i, f := range m.recordFields {
		if f.Row != row {
			continue
		}
		fieldStr, _ = m.renderRecordField(i, f)
	}

	if fieldStr == "" {
		return fieldDisplayStyle.Render(strings.Repeat(" ", boxW))
	}

	padBefore := 2
	// Use actual visual width to avoid blow-out from multi-byte characters
	padAfter := boxW - padBefore - lipgloss.Width(fieldStr)
	if padAfter < 0 {
		padAfter = 0
	}

	return fieldDisplayStyle.Render(strings.Repeat(" ", padBefore)) +
		fieldStr +
		fieldDisplayStyle.Render(strings.Repeat(" ", padAfter))
}

// renderRecordField renders a single record field.
func (m Model) renderRecordField(fieldIdx int, f fieldDef) (string, int) {
	isActive := m.editField == fieldIdx

	labelText := padRight(f.Label, 16)
	label := labelText + " : "
	labelLen := len(label)

	var value string
	if f.Get != nil {
		value = f.Get()
	}

	rawW := labelLen + f.Width

	if isActive && m.mode == modeRecordField {
		return fieldLabelStyle.Render(label) + m.textInput.View(), rawW
	}

	displayValue := padRight(value, f.Width)

	if isActive && m.mode == modeRecordEdit {
		v := value
		if len(v) > f.Width {
			v = v[:f.Width]
		}
		fillStr := strings.Repeat(string(fieldFillChar), maxInt(0, f.Width-len(v)))
		return fieldLabelStyle.Render(label) + fieldEditStyle.Render(v+fillStr), rawW
	}

	if f.Type == ftDisplay {
		return fieldLabelStyle.Render(label) + editInfoValueStyle.Render(displayValue), rawW
	}

	return fieldLabelStyle.Render(label) + fieldDisplayStyle.Render(displayValue), rawW
}

// renderFieldHelpLine returns the message/help line below the bottom border.
// Priority: flash message > active field help text > blank fill.
func (m Model) renderFieldHelpLine(fields []fieldDef, padL, padR, boxW int) string {
	if m.message != "" {
		return bgFillStyle.Render(strings.Repeat("░", padL)) +
			flashMessageStyle.Render(" "+padRight(m.message, boxW)) +
			bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR+1)))
	}
	if m.editField >= 0 && m.editField < len(fields) && fields[m.editField].Help != "" {
		helpText := fields[m.editField].Help
		return bgFillStyle.Render(strings.Repeat("░", padL)) +
			editInfoLabelStyle.Render(centerText(helpText, boxW+1)) +
			bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR+1)))
	}
	return bgFillStyle.Render(strings.Repeat("░", m.width))
}
