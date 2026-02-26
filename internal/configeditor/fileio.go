package configeditor

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/stlalpha/vision3/internal/archiver"
	"github.com/stlalpha/vision3/internal/conference"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/file"
	"github.com/stlalpha/vision3/internal/message"
	"github.com/stlalpha/vision3/internal/transfer"
)

// allConfigs holds all loaded configuration data.
type allConfigs struct {
	Server      config.ServerConfig
	Conferences []conference.Conference
	MsgAreas    []message.MessageArea
	FileAreas   []file.FileArea
	Doors       map[string]config.DoorConfig
	Events      config.EventsConfig
	FTN         config.FTNConfig
	Protocols   []transfer.ProtocolConfig
	Archivers   archiver.Config
	LoginSeq    []config.LoginItem
}

// loadAllConfigs loads all configuration files from the given directory.
func loadAllConfigs(configPath string) (allConfigs, error) {
	var ac allConfigs
	var err error

	// Server config
	ac.Server, err = config.LoadServerConfig(configPath)
	if err != nil {
		return ac, fmt.Errorf("loading server config: %w", err)
	}

	// Conferences
	ac.Conferences, err = loadJSONSlice[conference.Conference](configPath, "conferences.json")
	if err != nil {
		return ac, fmt.Errorf("loading conferences: %w", err)
	}

	// Message areas
	ac.MsgAreas, err = loadJSONSlice[message.MessageArea](configPath, "message_areas.json")
	if err != nil {
		return ac, fmt.Errorf("loading message areas: %w", err)
	}

	// Sort message areas by conference position, then by area position within
	// each conference so the TUI list groups areas under their conference.
	sortMsgAreasByConference(ac.MsgAreas, ac.Conferences)

	// File areas
	ac.FileAreas, err = loadJSONSlice[file.FileArea](configPath, "file_areas.json")
	if err != nil {
		return ac, fmt.Errorf("loading file areas: %w", err)
	}

	// Doors
	ac.Doors, err = config.LoadDoors(filepath.Join(configPath, "doors.json"))
	if err != nil {
		// Doors file may not exist; initialize empty
		ac.Doors = make(map[string]config.DoorConfig)
	}

	// Events
	ac.Events, err = config.LoadEventsConfig(configPath)
	if err != nil {
		return ac, fmt.Errorf("loading events: %w", err)
	}

	// FTN
	ac.FTN, err = config.LoadFTNConfig(configPath)
	if err != nil {
		return ac, fmt.Errorf("loading ftn: %w", err)
	}

	// Protocols
	ac.Protocols, err = transfer.LoadProtocols(filepath.Join(configPath, "protocols.json"))
	if err != nil {
		// Protocols file may not exist
		ac.Protocols = nil
	}

	// Archivers
	ac.Archivers, err = archiver.LoadConfig(configPath)
	if err != nil {
		ac.Archivers = archiver.Config{}
	}

	// Login sequence
	ac.LoginSeq, err = loadJSONSlice[config.LoginItem](configPath, "login.json")
	if err != nil {
		ac.LoginSeq = nil
	}

	return ac, nil
}

// loadJSONSlice loads a JSON array from a file into a slice of T.
func loadJSONSlice[T any](configPath, filename string) ([]T, error) {
	filePath := filepath.Join(configPath, filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filename, err)
	}
	return result, nil
}

// saveServerConfig writes the server config back to disk.
func saveServerConfig(configPath string, cfg config.ServerConfig) error {
	return config.SaveServerConfig(configPath, cfg)
}

// saveJSONSlice writes a slice as a JSON array to a file.
func saveJSONSlice[T any](configPath, filename string, data []T) error {
	filePath := filepath.Join(configPath, filename)
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", filename, err)
	}
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}
	return nil
}

// saveJSONMap writes a map as a JSON object to a file.
func saveJSONMap[K comparable, V any](configPath, filename string, data map[K]V) error {
	filePath := filepath.Join(configPath, filename)
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", filename, err)
	}
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}
	return nil
}

// saveDoors writes doors config back to disk as a JSON array (matching LoadDoors format).
func saveDoors(configPath string, doors map[string]config.DoorConfig) error {
	// LoadDoors reads a JSON array and keys by Name, so we save as an array
	doorSlice := make([]config.DoorConfig, 0, len(doors))
	for _, d := range doors {
		doorSlice = append(doorSlice, d)
	}
	return saveJSONSlice(configPath, "doors.json", doorSlice)
}

// saveEventsConfig writes events config back to disk.
func saveEventsConfig(configPath string, cfg config.EventsConfig) error {
	filePath := filepath.Join(configPath, "events.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling events: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}
	return nil
}

// saveFTNConfig writes FTN config back to disk.
func saveFTNConfig(configPath string, cfg config.FTNConfig) error {
	filePath := filepath.Join(configPath, "ftn.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling ftn: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}
	return nil
}

// saveArchivers writes archivers config back to disk.
func saveArchivers(configPath string, cfg archiver.Config) error {
	filePath := filepath.Join(configPath, "archivers.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling archivers: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}
	return nil
}

// saveProtocols writes protocols config back to disk.
func saveProtocols(configPath string, protocols []transfer.ProtocolConfig) error {
	return saveJSONSlice(configPath, "protocols.json", protocols)
}

// saveConferences writes conferences back to disk.
func saveConferences(configPath string, confs []conference.Conference) error {
	return saveJSONSlice(configPath, "conferences.json", confs)
}

// saveMsgAreas writes message areas back to disk.
func saveMsgAreas(configPath string, areas []message.MessageArea) error {
	return saveJSONSlice(configPath, "message_areas.json", areas)
}

// saveFileAreas writes file areas back to disk.
func saveFileAreas(configPath string, areas []file.FileArea) error {
	return saveJSONSlice(configPath, "file_areas.json", areas)
}

// saveLoginSeq writes login sequence back to disk.
func saveLoginSeq(configPath string, items []config.LoginItem) error {
	return saveJSONSlice(configPath, "login.json", items)
}

// sortMsgAreasByConference sorts message areas by conference display position,
// then by area position within each conference. Areas whose conference is not
// found sort to the end.
func sortMsgAreasByConference(areas []message.MessageArea, confs []conference.Conference) {
	// Build lookup: conference ID → conference Position.
	confPos := make(map[int]int, len(confs))
	for _, c := range confs {
		confPos[c.ID] = c.Position
	}
	sort.SliceStable(areas, func(i, j int) bool {
		ci := confPos[areas[i].ConferenceID]
		cj := confPos[areas[j].ConferenceID]
		if ci == 0 {
			ci = math.MaxInt32
		}
		if cj == 0 {
			cj = math.MaxInt32
		}
		if ci != cj {
			return ci < cj
		}
		return areas[i].Position < areas[j].Position
	})
}

// confTagByID returns the conference Tag for the given conference ID,
// or "?" if not found.
func confTagByID(confs []conference.Conference, confID int) string {
	for _, c := range confs {
		if c.ID == confID {
			return c.Tag
		}
	}
	return "?"
}
