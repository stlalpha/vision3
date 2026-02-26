package configeditor

import (
	"fmt"
	"strings"
)

// viewRecordList renders the scrollable record list.
func (m Model) viewRecordList() string {
	var b strings.Builder

	// Title bar
	title := centerText(fmt.Sprintf("-- %s --", m.recordTypeTitle()), m.width)
	b.WriteString(titleBarStyle.Render(title))
	b.WriteByte('\n')

	bgLine := bgFillStyle.Render(strings.Repeat("░", m.width))

	boxW := 70
	listVisible := m.recordListVisible()

	// Vertical centering: title + border + header + blank + listVisible + blank + border + msg + help
	fixedRows := listVisible + 7
	extraV := maxInt(0, m.height-fixedRows)
	topPad := maxInt(1, extraV/2)
	bottomPad := maxInt(1, extraV-topPad)

	for i := 0; i < topPad; i++ {
		b.WriteString(bgLine)
		b.WriteByte('\n')
	}

	padL := maxInt(0, (m.width-boxW-2)/2)
	padR := maxInt(0, m.width-padL-boxW-2)

	// Top border
	b.WriteString(bgFillStyle.Render(strings.Repeat("░", padL)) +
		menuBorderStyle.Render("╒"+strings.Repeat("═", boxW)+"╕") +
		bgFillStyle.Render(strings.Repeat("░", maxInt(0, padR))))
	b.WriteByte('\n')

	// Header
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
	for row := 0; row < listVisible; row++ {
		idx := m.recordScroll + row
		isHighlight := idx == m.recordCursor

		var rowContent string
		if idx < 0 || idx >= total {
			rowContent = menuItemStyle.Render(strings.Repeat(" ", boxW))
		} else {
			content := m.renderRecordRow(idx, boxW)
			if isHighlight {
				rowContent = menuHighlightStyle.Render(content)
			} else {
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
		menuBorderStyle.Render("╘"+strings.Repeat("═", boxW)+"╛") +
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

	helpText := centerText("Enter - Edit  |  I - Insert  |  D - Delete  |  ESC - Return", m.width)
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
		return "  #  Tag                  Name                         Type"
	case "filearea":
		return "  #  Tag                  Name                         Path"
	case "conference":
		return "  #  Tag                  Name                         ACS"
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
			content = fmt.Sprintf(" %3d  %-20s %-28s %s", a.ID, padRight(a.Tag, 20), padRight(a.Name, 28), a.AreaType)
		}
	case "filearea":
		if idx < len(m.configs.FileAreas) {
			a := m.configs.FileAreas[idx]
			content = fmt.Sprintf(" %3d  %-20s %-28s %s", a.ID, padRight(a.Tag, 20), padRight(a.Name, 28), a.Path)
		}
	case "conference":
		if idx < len(m.configs.Conferences) {
			c := m.configs.Conferences[idx]
			content = fmt.Sprintf(" %3d  %-20s %-28s %s", c.ID, padRight(c.Tag, 20), padRight(c.Name, 28), c.ACS)
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
