package configeditor

import (
	"fmt"

	"github.com/stlalpha/vision3/internal/archiver"
	"github.com/stlalpha/vision3/internal/conference"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/transfer"
)

// saveAll writes all modified configs to disk.
func (m *Model) saveAll() {
	if !m.dirty {
		return
	}

	if err := saveServerConfig(m.configPath, m.configs.Server); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveConferences(m.configPath, m.configs.Conferences); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveMsgAreas(m.configPath, m.configs.MsgAreas); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveFileAreas(m.configPath, m.configs.FileAreas); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveDoors(m.configPath, m.configs.Doors); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveEventsConfig(m.configPath, m.configs.Events); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveFTNConfig(m.configPath, m.configs.FTN); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveProtocols(m.configPath, m.configs.Protocols); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveArchivers(m.configPath, m.configs.Archivers); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	if err := saveLoginSeq(m.configPath, m.configs.LoginSeq); err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}

	m.dirty = false
	m.message = "All configurations saved successfully"
}

// --- Record count and helpers ---

func (m Model) recordCount() int {
	switch m.recordType {
	case "msgarea":
		return len(m.configs.MsgAreas)
	case "filearea":
		return len(m.configs.FileAreas)
	case "conference":
		return len(m.configs.Conferences)
	case "door":
		return len(m.configs.Doors)
	case "event":
		return len(m.configs.Events.Events)
	case "ftn":
		return len(m.configs.FTN.Networks)
	case "protocol":
		return len(m.configs.Protocols)
	case "archiver":
		return len(m.configs.Archivers.Archivers)
	case "login":
		return len(m.configs.LoginSeq)
	}
	return 0
}

func (m Model) recordListVisible() int {
	return 13
}

func (m *Model) insertRecord() {
	switch m.recordType {
	case "msgarea":
		newID := 1
		for _, a := range m.configs.MsgAreas {
			if a.ID >= newID {
				newID = a.ID + 1
			}
		}
		m.configs.MsgAreas = append(m.configs.MsgAreas, message.MessageArea{
			ID:       newID,
			Name:     "New Message Area",
			AreaType: "local",
		})
	case "filearea":
		newID := 1
		for _, a := range m.configs.FileAreas {
			if a.ID >= newID {
				newID = a.ID + 1
			}
		}
		m.configs.FileAreas = append(m.configs.FileAreas, file.FileArea{
			ID:   newID,
			Name: "New File Area",
		})
	case "conference":
		newID := 1
		for _, c := range m.configs.Conferences {
			if c.ID >= newID {
				newID = c.ID + 1
			}
		}
		m.configs.Conferences = append(m.configs.Conferences, conference.Conference{
			ID:   newID,
			Name: "New Conference",
		})
	case "door":
		name := fmt.Sprintf("newdoor%d", len(m.configs.Doors)+1)
		m.configs.Doors[name] = config.DoorConfig{
			Name: "New Door",
		}
	case "ftn":
		if m.configs.FTN.Networks == nil {
			m.configs.FTN.Networks = make(map[string]config.FTNNetworkConfig)
		}
		// Use zz_ prefix so new entries sort after real network names
		for i := 1; ; i++ {
			name := fmt.Sprintf("zz_newnet_%d", i)
			if _, exists := m.configs.FTN.Networks[name]; !exists {
				m.configs.FTN.Networks[name] = config.FTNNetworkConfig{}
				break
			}
		}
	case "event":
		newID := fmt.Sprintf("event_%d", len(m.configs.Events.Events)+1)
		m.configs.Events.Events = append(m.configs.Events.Events, config.EventConfig{
			ID:      newID,
			Name:    "New Event",
			Enabled: false,
		})
	case "protocol":
		m.configs.Protocols = append(m.configs.Protocols, transfer.ProtocolConfig{
			Name: "New Protocol",
		})
	case "archiver":
		m.configs.Archivers.Archivers = append(m.configs.Archivers.Archivers, archiver.Archiver{
			Name:    "New Archiver",
			Enabled: false,
		})
	case "login":
		m.configs.LoginSeq = append(m.configs.LoginSeq, config.LoginItem{
			Command: "DISPLAYFILE",
		})
	}
}

func (m *Model) deleteRecord() {
	idx := m.recordCursor
	switch m.recordType {
	case "msgarea":
		if idx >= 0 && idx < len(m.configs.MsgAreas) {
			m.configs.MsgAreas = append(m.configs.MsgAreas[:idx], m.configs.MsgAreas[idx+1:]...)
		}
	case "filearea":
		if idx >= 0 && idx < len(m.configs.FileAreas) {
			m.configs.FileAreas = append(m.configs.FileAreas[:idx], m.configs.FileAreas[idx+1:]...)
		}
	case "conference":
		if idx >= 0 && idx < len(m.configs.Conferences) {
			m.configs.Conferences = append(m.configs.Conferences[:idx], m.configs.Conferences[idx+1:]...)
		}
	case "door":
		keys := m.doorKeys()
		if idx >= 0 && idx < len(keys) {
			delete(m.configs.Doors, keys[idx])
		}
	case "ftn":
		keys := m.ftnNetworkKeys()
		if idx >= 0 && idx < len(keys) {
			delete(m.configs.FTN.Networks, keys[idx])
		}
	case "event":
		if idx >= 0 && idx < len(m.configs.Events.Events) {
			m.configs.Events.Events = append(m.configs.Events.Events[:idx], m.configs.Events.Events[idx+1:]...)
		}
	case "protocol":
		if idx >= 0 && idx < len(m.configs.Protocols) {
			m.configs.Protocols = append(m.configs.Protocols[:idx], m.configs.Protocols[idx+1:]...)
		}
	case "archiver":
		if idx >= 0 && idx < len(m.configs.Archivers.Archivers) {
			m.configs.Archivers.Archivers = append(m.configs.Archivers.Archivers[:idx], m.configs.Archivers.Archivers[idx+1:]...)
		}
	case "login":
		if idx >= 0 && idx < len(m.configs.LoginSeq) {
			m.configs.LoginSeq = append(m.configs.LoginSeq[:idx], m.configs.LoginSeq[idx+1:]...)
		}
	}

	total := m.recordCount()
	if m.recordCursor >= total && total > 0 {
		m.recordCursor = total - 1
	}
	if m.recordCursor < 0 {
		m.recordCursor = 0
	}
}

// doorKeys returns sorted keys of the doors map for stable iteration.
func (m Model) doorKeys() []string {
	keys := make([]string, 0, len(m.configs.Doors))
	for k := range m.configs.Doors {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
