package configeditor

import (
	"fmt"
	"strings"
)

// viewRecordList renders the scrollable record list.
func (m Model) viewRecordList() string {
	var b strings.Builder

	// Global header
	b.WriteString(m.globalHeaderLine())
	b.WriteByte('\n')

	bgLine := bgFillStyle.Render(strings.Repeat("░", m.width))

	boxW := 70
	listVisible := m.recordListVisible()

	// Fixed rows: globalheader(1) + border(1) + boxtitle(1) + colheader(1) + sep(1) + listVisible + border(1) + msg(1) + help(1)
	fixedRows := listVisible + 8
	extraV := maxInt(0, m.height-fixedRows)
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
		menuBorderStyle.Render("┌"+strings.Repeat("─", boxW)+"┐") +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Box title (record type)
	boxTitle := menuBorderStyle.Render("│") +
		menuHeaderStyle.Render(centerText(m.recordTypeTitle(), boxW)) +
		menuBorderStyle.Render("│")
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) + boxTitle +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Column header
	colHeader := m.recordColumnHeader(boxW)
	headerLine := menuBorderStyle.Render("│") +
		menuHeaderStyle.Render(padRight(colHeader, boxW)) +
		menuBorderStyle.Render("│")
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) + headerLine +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Separator
	sepLine := bgFillStyle.Render(strings.Repeat("░", padL)) +
		menuBorderStyle.Render("│") +
		separatorStyle.Render(strings.Repeat("─", boxW)) +
		menuBorderStyle.Render("│") +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR)))
	b.WriteString(sepLine)
	b.WriteByte('\n')

	// List rows
	total := m.recordCount()
	inReorder := m.mode == modeRecordReorder
	for row := 0; row < listVisible; row++ {
		idx := m.recordScroll + row
		isHighlight := idx == m.recordCursor
		isSource := inReorder && idx == m.reorderSourceIdx

		var rowContent string
		if idx < 0 || idx >= total {
			rowContent = menuItemStyle.Render(strings.Repeat(" ", boxW))
		} else {
			content := m.renderRecordRow(idx, boxW)
			switch {
			case isSource && isHighlight:
				rowContent = reorderSourceStyle.Render(content)
			case isSource:
				rowContent = reorderSourceStyle.Render(content)
			case isHighlight:
				rowContent = menuHighlightStyle.Render(content)
			default:
				rowContent = menuItemStyle.Render(content)
			}
		}

		line := bgFillStyle.Render(strings.Repeat("░", padL)) +
			menuBorderStyle.Render("│") +
			rowContent +
			menuBorderStyle.Render("│") +
			bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR)))
		b.WriteString(line)
		b.WriteByte('\n')
	}

	// Bottom border
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) +
		menuBorderStyle.Render("└"+strings.Repeat("─", boxW)+"┘") +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Message/fill
	if m.message != "" {
		msgLine := bgFillStyle.Render(strings.Repeat("░", padL)) +
			flashMessageStyle.Render(" "+padRight(m.message, boxW)) +
			bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR+1)))
		b.WriteString(msgLine)
	} else {
		b.WriteString(bgLine)
	}
	b.WriteByte('\n')

	for i := 0; i < bottomPad; i++ {
		b.WriteString(bgLine)
		b.WriteByte('\n')
	}

	var helpStr string
	if inReorder {
		helpStr = "Up/Down - Move  |  Enter - Confirm  |  ESC - Cancel"
		if m.recordType == "msgarea" && m.reorderSourceIdx >= 0 && m.reorderSourceIdx < len(m.configs.MsgAreas) {
			confID := m.configs.MsgAreas[m.reorderSourceIdx].ConferenceID
			cTag := confTagByID(m.configs.Conferences, confID)
			helpStr = fmt.Sprintf("Reorder within: %s  |  Up/Down - Move  |  Enter - Confirm  |  ESC - Cancel", cTag)
		}
	} else if m.recordTypeSupportsReorder() {
		helpStr = "Enter - Edit  |  I - Insert  |  D - Delete  |  P - Position  |  ESC - Return"
	} else {
		helpStr = "Enter - Edit  |  I - Insert  |  D - Delete  |  ESC - Return"
	}
	helpText := centerText(helpStr, m.width)
	b.WriteString(helpBarStyle.Render(helpText))

	return b.String()
}

// recordTypeTitle returns a human-readable title for the current record type.
func (m Model) recordTypeTitle() string {
	switch m.recordType {
	case "msgarea":
		return "Message Areas"
	case "filearea":
		return "File Areas"
	case "conference":
		return "Conferences"
	case "door":
		return "Door Programs"
	case "event":
		return "Event Scheduler"
	case "ftn":
		return "FTN Network Links"
	case "protocol":
		return "Transfer Protocols"
	case "archiver":
		return "Archivers"
	case "login":
		return "Login Sequence"
	}
	return "Records"
}

// recordColumnHeader returns the column header text for the current record type.
func (m Model) recordColumnHeader(boxW int) string {
	switch m.recordType {
	case "msgarea":
		return "  #  Conf      Tag              Name                        Type"
	case "filearea":
		return "  #  Tag                  Name                         Path"
	case "conference":
		return " Pos  #  Tag               Name                         ACS"
	case "door":
		return "  Key                     Name                         I/O Mode"
	case "event":
		return "  ID                      Name                         Enabled"
	case "ftn":
		return "  Network                 Address                      Tosser"
	case "protocol":
		return "  Key    Name                           Send Cmd"
	case "archiver":
		return "  ID     Name                  Ext      Enabled"
	case "login":
		return "  #  Command          Data"
	}
	return ""
}

// renderRecordRow renders a single row in the record list.
func (m Model) renderRecordRow(idx, boxW int) string {
	var content string

	switch m.recordType {
	case "msgarea":
		if idx < len(m.configs.MsgAreas) {
			a := m.configs.MsgAreas[idx]
			cTag := confTagByID(m.configs.Conferences, a.ConferenceID)
			content = fmt.Sprintf(" %3d  %-8s  %-16s %-27s %s", a.ID, padRight(cTag, 8), padRight(a.Tag, 16), padRight(a.Name, 27), a.AreaType)
		}
	case "filearea":
		if idx < len(m.configs.FileAreas) {
			a := m.configs.FileAreas[idx]
			content = fmt.Sprintf(" %3d  %-20s %-28s %s", a.ID, padRight(a.Tag, 20), padRight(a.Name, 28), a.Path)
		}
	case "conference":
		if idx < len(m.configs.Conferences) {
			c := m.configs.Conferences[idx]
			content = fmt.Sprintf(" %3d %3d  %-17s %-28s %s", c.Position, c.ID, padRight(c.Tag, 17), padRight(c.Name, 28), c.ACS)
		}
	case "door":
		keys := m.doorKeys()
		if idx < len(keys) {
			k := keys[idx]
			d := m.configs.Doors[k]
			content = fmt.Sprintf("  %-22s %-28s %s", padRight(k, 22), padRight(d.Name, 28), d.IOMode)
		}
	case "event":
		if idx < len(m.configs.Events.Events) {
			e := m.configs.Events.Events[idx]
			content = fmt.Sprintf("  %-22s %-28s %s", padRight(e.ID, 22), padRight(e.Name, 28), boolToYN(e.Enabled))
		}
	case "ftn":
		keys := m.ftnNetworkKeys()
		if idx < len(keys) {
			k := keys[idx]
			n := m.configs.FTN.Networks[k]
			content = fmt.Sprintf("  %-22s %-28s %s", padRight(k, 22), padRight(n.OwnAddress, 28), boolToYN(n.InternalTosserEnabled))
		}
	case "protocol":
		if idx < len(m.configs.Protocols) {
			p := m.configs.Protocols[idx]
			content = fmt.Sprintf("  %-6s  %-30s %s", padRight(p.Key, 6), padRight(p.Name, 30), p.SendCmd)
		}
	case "archiver":
		if idx < len(m.configs.Archivers.Archivers) {
			a := m.configs.Archivers.Archivers[idx]
			content = fmt.Sprintf("  %-6s  %-20s %-8s %s", padRight(a.ID, 6), padRight(a.Name, 20), padRight(a.Extension, 8), boolToYN(a.Enabled))
		}
	case "login":
		if idx < len(m.configs.LoginSeq) {
			l := m.configs.LoginSeq[idx]
			content = fmt.Sprintf(" %3d  %-16s %s", idx+1, padRight(l.Command, 16), l.Data)
		}
	}

	if content == "" {
		content = strings.Repeat(" ", boxW)
	}

	// Ensure content fills the box width
	if len(content) < boxW {
		content += strings.Repeat(" ", boxW-len(content))
	} else if len(content) > boxW {
		content = content[:boxW]
	}

	return content
}

// ftnNetworkKeys returns sorted keys of the FTN networks map.
func (m Model) ftnNetworkKeys() []string {
	keys := make([]string, 0, len(m.configs.FTN.Networks))
	for k := range m.configs.FTN.Networks {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
