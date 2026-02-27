package configeditor

import (
	"fmt"
	"strconv"
	"strings"
)

// buildRecordFields returns field definitions for the current record type and index.
func (m *Model) buildRecordFields() []fieldDef {
	switch m.recordType {
	case "msgarea":
		return m.fieldsMsgArea()
	case "filearea":
		return m.fieldsFileArea()
	case "conference":
		return m.fieldsConference()
	case "door":
		return m.fieldsDoor()
	case "event":
		return m.fieldsEvent()
	case "ftn":
		if m.recordEditIdx < 0 {
			return m.fieldsFTNGlobal()
		}
		return m.fieldsFTNLink()
	case "ftnlink":
		return m.fieldsFTNLinkEdit()
	case "protocol":
		return m.fieldsProtocol()
	case "archiver":
		return m.fieldsArchiver()
	case "login":
		return m.fieldsLogin()
	}
	return nil
}

// fieldsMsgArea returns fields for editing a message area.
// FTN-specific fields are shown conditionally based on the area type.
func (m *Model) fieldsMsgArea() []fieldDef {
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(m.configs.MsgAreas) {
		return nil
	}
	a := &m.configs.MsgAreas[idx]

	fields := []fieldDef{
		{
			Label: "Position", Help: "Display order (use P key in list to reorder)", Type: ftDisplay, Col: 3, Row: 1, Width: 5,
			Get: func() string { return strconv.Itoa(a.Position) },
		},
		{
			Label: "Tag", Help: "Short identifier for this area (used in configs)", Type: ftString, Col: 3, Row: 2, Width: 30,
			Get: func() string { return a.Tag },
			Set: func(val string) error { a.Tag = val; return nil },
		},
		{
			Label: "Name", Help: "Display name shown to users", Type: ftString, Col: 3, Row: 3, Width: 40,
			Get: func() string { return a.Name },
			Set: func(val string) error { a.Name = val; return nil },
		},
		{
			Label: "Description", Help: "Longer description of this area", Type: ftString, Col: 3, Row: 4, Width: 45,
			Get: func() string { return a.Description },
			Set: func(val string) error { a.Description = val; return nil },
		},
		{
			Label: "Area Type", Help: "local = BBS-only  |  echomail = FTN echo  |  netmail = FTN point-to-point", Type: ftLookup, Col: 3, Row: 5, Width: 10,
			Get: func() string { return a.AreaType },
			Set: func(val string) error { a.AreaType = val; return nil },
			LookupItems: func() []LookupItem {
				return []LookupItem{
					{Value: "local", Display: "local - BBS-only message area"},
					{Value: "echomail", Display: "echomail - FTN echoed message area"},
					{Value: "netmail", Display: "netmail - FTN point-to-point mail"},
				}
			},
		},
		{
			Label: "ACS Read", Help: "Access string for reading (e.g. s20 = level 20+)", Type: ftString, Col: 3, Row: 6, Width: 20,
			Get: func() string { return a.ACSRead },
			Set: func(val string) error { a.ACSRead = val; return nil },
		},
		{
			Label: "ACS Write", Help: "Access string for posting (e.g. s20 = level 20+)", Type: ftString, Col: 3, Row: 7, Width: 20,
			Get: func() string { return a.ACSWrite },
			Set: func(val string) error { a.ACSWrite = val; return nil },
		},
		{
			Label: "Base Path", Help: "Directory path for message base storage", Type: ftString, Col: 3, Row: 8, Width: 45,
			Get: func() string { return a.BasePath },
			Set: func(val string) error { a.BasePath = val; return nil },
		},
		{
			Label: "Conference", Help: "Press Enter to select a conference", Type: ftLookup, Col: 3, Row: 9, Width: 40,
			Get: func() string { return m.conferenceName(a.ConferenceID) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				a.ConferenceID = n
				return nil
			},
			LookupItems: func() []LookupItem { return m.buildConferenceLookupItems() },
		},
		{
			Label: "Max Messages", Help: "Maximum messages before purging (0=unlimited)", Type: ftInteger, Col: 3, Row: 10, Width: 6, Min: 0, Max: 999999,
			Get: func() string { return strconv.Itoa(a.MaxMessages) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				a.MaxMessages = n
				return nil
			},
		},
		{
			Label: "Max Age", Help: "Maximum age in days before purging (0=unlimited)", Type: ftInteger, Col: 3, Row: 11, Width: 5, Min: 0, Max: 99999,
			Get: func() string { return strconv.Itoa(a.MaxAge) },
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				a.MaxAge = n
				return nil
			},
		},
		{
			Label: "Auto Join", Help: "Automatically join new users to this area", Type: ftYesNo, Col: 3, Row: 12, Width: 1,
			Get: func() string { return boolToYN(a.AutoJoin) },
			Set: func(val string) error { a.AutoJoin = ynToBool(val); return nil },
		},
		{
			Label: "Real Name Only", Help: "Require real name for posts (no aliases)", Type: ftYesNo, Col: 3, Row: 13, Width: 1,
			Get: func() string { return boolToYN(a.RealNameOnly) },
			Set: func(val string) error { a.RealNameOnly = ynToBool(val); return nil },
		},
	}

	switch strings.ToLower(a.AreaType) {
	case "echomail":
		fields = append(fields,
			fieldDef{
				Label: "Network", Help: "FTN network key from ftn.json (e.g. fsxnet)", Type: ftLookup, Col: 3, Row: 14, Width: 20,
				Get: func() string { return a.Network },
				Set: func(val string) error { a.Network = val; return nil },
				LookupItems: func() []LookupItem { return m.buildFTNNetworkLookupItems() },
			},
			fieldDef{
				Label: "Echo Tag", Help: "FTN echo tag matching the network's area list (e.g. FSX_GEN)", Type: ftString, Col: 3, Row: 15, Width: 30,
				Get: func() string { return a.EchoTag },
				Set: func(val string) error { a.EchoTag = val; return nil },
			},
			fieldDef{
				Label: "Origin Addr", Help: "Your FTN origin address for this echo (e.g. 21:4/158.1)", Type: ftString, Col: 3, Row: 16, Width: 20,
				Get: func() string { return a.OriginAddr },
				Set: func(val string) error { a.OriginAddr = val; return nil },
			},
			fieldDef{
				Label: "Sponsor", Help: "Handle of the echo sponsor/moderator (optional)", Type: ftString, Col: 3, Row: 17, Width: 30,
				Get: func() string { return a.Sponsor },
				Set: func(val string) error { a.Sponsor = val; return nil },
			},
		)
	case "netmail":
		fields = append(fields,
			fieldDef{
				Label: "Network", Help: "FTN network key this netmail area serves (e.g. fsxnet)", Type: ftLookup, Col: 3, Row: 14, Width: 20,
				Get: func() string { return a.Network },
				Set: func(val string) error { a.Network = val; return nil },
				LookupItems: func() []LookupItem { return m.buildFTNNetworkLookupItems() },
			},
		)
	}

	return fields
}

// fieldsFileArea returns fields for editing a file area.
func (m *Model) fieldsFileArea() []fieldDef {
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(m.configs.FileAreas) {
		return nil
	}
	a := &m.configs.FileAreas[idx]
	return []fieldDef{
		{
			Label: "Tag", Help: "Short identifier for this area (used in configs)", Type: ftString, Col: 3, Row: 1, Width: 30,
			Get: func() string { return a.Tag },
			Set: func(val string) error { a.Tag = val; return nil },
		},
		{
			Label: "Name", Help: "Display name shown to users", Type: ftString, Col: 3, Row: 2, Width: 40,
			Get: func() string { return a.Name },
			Set: func(val string) error { a.Name = val; return nil },
		},
		{
			Label: "Description", Help: "Longer description of this area", Type: ftString, Col: 3, Row: 3, Width: 45,
			Get: func() string { return a.Description },
			Set: func(val string) error { a.Description = val; return nil },
		},
		{
			Label: "Path", Help: "Directory path where files are stored", Type: ftString, Col: 3, Row: 4, Width: 45,
			Get: func() string { return a.Path },
			Set: func(val string) error { a.Path = val; return nil },
		},
		{
			Label: "ACS List", Help: "Access string for listing files", Type: ftString, Col: 3, Row: 5, Width: 20,
			Get: func() string { return a.ACSList },
			Set: func(val string) error { a.ACSList = val; return nil },
		},
		{
			Label: "ACS Upload", Help: "Access string for uploading files", Type: ftString, Col: 3, Row: 6, Width: 20,
			Get: func() string { return a.ACSUpload },
			Set: func(val string) error { a.ACSUpload = val; return nil },
		},
		{
			Label: "ACS Download", Help: "Access string for downloading files", Type: ftString, Col: 3, Row: 7, Width: 20,
			Get: func() string { return a.ACSDownload },
			Set: func(val string) error { a.ACSDownload = val; return nil },
		},
		{
			Label: "Conference", Help: "Press Enter to select a conference", Type: ftLookup, Col: 3, Row: 8, Width: 40,
			Get: func() string {
				return m.conferenceName(a.ConferenceID)
			},
			Set: func(val string) error {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				a.ConferenceID = n
				return nil
			},
			LookupItems: func() []LookupItem {
				return m.buildConferenceLookupItems()
			},
		},
	}
}

// fieldsConference returns fields for editing a conference.
func (m *Model) fieldsConference() []fieldDef {
	idx := m.recordEditIdx
	if idx < 0 || idx >= len(m.configs.Conferences) {
		return nil
	}
	c := &m.configs.Conferences[idx]
	return []fieldDef{
		{
			Label: "Position", Help: "Display order (use P key in list to reorder)", Type: ftDisplay, Col: 3, Row: 1, Width: 5,
			Get: func() string { return strconv.Itoa(c.Position) },
		},
		{
			Label: "Tag", Help: "Short identifier (e.g. LOCAL, FSXNET)", Type: ftString, Col: 3, Row: 2, Width: 20,
			Get: func() string { return c.Tag },
			Set: func(val string) error { c.Tag = val; return nil },
		},
		{
			Label: "Name", Help: "Display name shown to users", Type: ftString, Col: 3, Row: 3, Width: 30,
			Get: func() string { return c.Name },
			Set: func(val string) error { c.Name = val; return nil },
		},
		{
			Label: "Description", Help: "Longer description of this conference", Type: ftString, Col: 3, Row: 4, Width: 40,
			Get: func() string { return c.Description },
			Set: func(val string) error { c.Description = val; return nil },
		},
		{
			Label: "ACS", Help: "Access string for conference access", Type: ftString, Col: 3, Row: 5, Width: 10,
			Get: func() string { return c.ACS },
			Set: func(val string) error { c.ACS = val; return nil },
		},
	}
}

// conferenceName returns a display string for a conference ID, e.g. "Local Conferences (ID: 1)".
func (m *Model) conferenceName(id int) string {
	if id == 0 {
		return "Ungrouped (ID: 0)"
	}
	for _, c := range m.configs.Conferences {
		if c.ID == id {
			return fmt.Sprintf("%s (ID: %d)", c.Name, c.ID)
		}
	}
	return fmt.Sprintf("(ID: %d)", id)
}

// buildConferenceLookupItems builds lookup items from loaded conferences.
func (m *Model) buildConferenceLookupItems() []LookupItem {
	items := make([]LookupItem, 0, len(m.configs.Conferences)+1)
	items = append(items, LookupItem{
		Value:   "0",
		Display: "Ungrouped (ID: 0)",
	})
	for _, c := range m.configs.Conferences {
		items = append(items, LookupItem{
			Value:   strconv.Itoa(c.ID),
			Display: fmt.Sprintf("%s (%s)", c.Name, c.Tag),
		})
	}
	return items
}
