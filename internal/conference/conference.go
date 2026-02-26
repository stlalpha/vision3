package conference

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const conferencesFile = "conferences.json"

// Conference defines a grouping of message and/or file areas.
type Conference struct {
	ID          int    `json:"id"`
	Position    int    `json:"position"`
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ACS         string `json:"acs"` // ACS string — who can see/enter this conference
	AllowAnon   *bool  `json:"allow_anonymous,omitempty"` // Optional: allow anonymous posts (nil defaults to true)
}

// ConferenceManager handles loading and accessing conference definitions.
type ConferenceManager struct {
	mu               sync.RWMutex
	configPath       string
	conferencesByID  map[int]*Conference
	conferencesByTag map[string]*Conference
}

// NewConferenceManager creates and initializes a new ConferenceManager.
// configPath is the directory containing conferences.json (e.g., "configs").
func NewConferenceManager(configPath string) (*ConferenceManager, error) {
	cm := &ConferenceManager{
		configPath:       filepath.Join(configPath, conferencesFile),
		conferencesByID:  make(map[int]*Conference),
		conferencesByTag: make(map[string]*Conference),
	}

	if err := cm.loadConferences(); err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: %s not found. Starting with no conferences.", conferencesFile)
			return cm, nil
		}
		return nil, fmt.Errorf("failed to load conferences: %w", err)
	}

	log.Printf("INFO: ConferenceManager initialized. Loaded %d conferences.", len(cm.conferencesByID))
	return cm, nil
}

// loadConferences reads conference definitions from JSON.
func (cm *ConferenceManager) loadConferences() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		log.Printf("INFO: %s is empty. No conferences loaded.", cm.configPath)
		return nil
	}

	var confList []*Conference
	if err := json.Unmarshal(data, &confList); err != nil {
		return fmt.Errorf("failed to unmarshal conferences from %s: %w", cm.configPath, err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.conferencesByID = make(map[int]*Conference)
	cm.conferencesByTag = make(map[string]*Conference)

	for _, conf := range confList {
		if conf == nil {
			continue
		}
		if _, exists := cm.conferencesByID[conf.ID]; exists {
			log.Printf("WARN: Duplicate conference ID %d in %s. Skipping.", conf.ID, cm.configPath)
			continue
		}
		if _, exists := cm.conferencesByTag[conf.Tag]; exists {
			log.Printf("WARN: Duplicate conference Tag '%s' in %s. Skipping.", conf.Tag, cm.configPath)
			continue
		}
		cm.conferencesByID[conf.ID] = conf
		cm.conferencesByTag[conf.Tag] = conf
		log.Printf("TRACE: Loaded conference ID %d, Tag '%s', Name '%s'", conf.ID, conf.Tag, conf.Name)
	}

	// Migration: assign positions to any conferences that have Position <= 0.
	// Finds the current max position and assigns sequentially after it.
	maxPos := 0
	hasUnset := false
	for _, conf := range cm.conferencesByID {
		if conf.Position > maxPos {
			maxPos = conf.Position
		}
		if conf.Position <= 0 {
			hasUnset = true
		}
	}
	if hasUnset && len(cm.conferencesByID) > 0 {
		sorted := make([]*Conference, 0, len(cm.conferencesByID))
		for _, conf := range cm.conferencesByID {
			if conf.Position <= 0 {
				sorted = append(sorted, conf)
			}
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].ID < sorted[j].ID
		})
		for _, conf := range sorted {
			maxPos++
			conf.Position = maxPos
		}
		log.Printf("INFO: Auto-assigned positions to %d conferences (migration)", len(sorted))
	}

	return nil
}

// GetByID retrieves a conference by its ID.
func (cm *ConferenceManager) GetByID(id int) (*Conference, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	conf, exists := cm.conferencesByID[id]
	return conf, exists
}

// GetByTag retrieves a conference by its Tag.
func (cm *ConferenceManager) GetByTag(tag string) (*Conference, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	conf, exists := cm.conferencesByTag[tag]
	return conf, exists
}

// ListConferences returns a sorted slice of all loaded conferences.
func (cm *ConferenceManager) ListConferences() []*Conference {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	list := make([]*Conference, 0, len(cm.conferencesByID))
	for _, conf := range cm.conferencesByID {
		list = append(list, conf)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Position < list[j].Position
	})
	return list
}

// GetSortedConferenceIDs returns a sorted list of conference IDs present in the given
// set of conference IDs, ordered by conference Position. ID 0 (ungrouped) is placed first if present.
func (cm *ConferenceManager) GetSortedConferenceIDs(confIDs map[int]bool) []int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var ids []int
	hasZero := false
	for id := range confIDs {
		if id == 0 {
			hasZero = true
			continue
		}
		ids = append(ids, id)
	}
	// Sort by conference Position (fall back to ID for unknown or unset positions).
	// Break ties by ID for deterministic ordering.
	sort.Slice(ids, func(i, j int) bool {
		pi, pj := ids[i], ids[j]
		ci, oki := cm.conferencesByID[pi]
		cj, okj := cm.conferencesByID[pj]
		posI, posJ := pi, pj // fallback to ID
		if oki && ci.Position > 0 {
			posI = ci.Position
		}
		if okj && cj.Position > 0 {
			posJ = cj.Position
		}
		if posI != posJ {
			return posI < posJ
		}
		return pi < pj
	})
	if hasZero {
		ids = append([]int{0}, ids...)
	}
	return ids
}
