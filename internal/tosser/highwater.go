package tosser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// hwmFile is the on-disk format for export high-water marks.
// Structure: { "networkName": { "areaID": lastMsgNum } }
type hwmFile struct {
	Networks map[string]map[int]int `json:"networks"`
}

// HighWaterMark manages per-area export position persistence.
// It tracks the highest message number already exported per area per network,
// so that v3mail scan can resume from where it left off rather than re-scanning
// from message 1 on every invocation.
type HighWaterMark struct {
	mu       sync.Mutex
	path     string
	networks map[string]map[int]int // networkName -> areaID -> lastMsgNum
}

// LoadHighWaterMark loads the HWM database from path, creating an empty one
// if the file does not exist.
func LoadHighWaterMark(path string) (*HighWaterMark, error) {
	hwm := &HighWaterMark{
		path:     path,
		networks: make(map[string]map[int]int),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return hwm, nil
		}
		return nil, fmt.Errorf("load hwm %s: %w", path, err)
	}
	if len(data) == 0 {
		return hwm, nil
	}

	var f hwmFile
	if err := json.Unmarshal(data, &f); err != nil {
		// Corrupt file — start fresh rather than failing
		return hwm, nil
	}
	if f.Networks != nil {
		hwm.networks = f.Networks
	}
	return hwm, nil
}

// Get returns the last scanned message number for a given network/area pair.
// Returns 0 if no mark has been recorded.
func (h *HighWaterMark) Get(network string, areaID int) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.networks[network]; ok {
		return m[areaID]
	}
	return 0
}

// Set records the last scanned message number for a network/area pair.
func (h *HighWaterMark) Set(network string, areaID int, msgNum int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.networks[network] == nil {
		h.networks[network] = make(map[int]int)
	}
	h.networks[network][areaID] = msgNum
}

// Save persists the current high-water marks atomically.
func (h *HighWaterMark) Save() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	f := hwmFile{Networks: h.networks}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(h.path, data, 0644)
}

// HWMPath returns the default HWM file path relative to a data directory.
func HWMPath(dataDir string) string {
	return filepath.Join(dataDir, "ftn", "export_hwm.json")
}
