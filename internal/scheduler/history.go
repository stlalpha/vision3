package scheduler

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// LoadHistory loads event history from a JSON file
func LoadHistory(path string) (map[string]*EventHistory, error) {
	history := make(map[string]*EventHistory)

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("INFO: Event history file not found at %s, starting with empty history", path)
		return history, nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON
	var historyList []EventHistory
	if err := json.Unmarshal(data, &historyList); err != nil {
		return nil, err
	}

	// Convert list to map
	for i := range historyList {
		history[historyList[i].EventID] = &historyList[i]
	}

	log.Printf("INFO: Loaded event history for %d events from %s", len(history), path)
	return history, nil
}

// SaveHistory saves event history to a JSON file
func SaveHistory(path string, history map[string]*EventHistory) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Convert map to list for JSON serialization
	var historyList []EventHistory
	for _, h := range history {
		historyList = append(historyList, *h)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(historyList, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	log.Printf("DEBUG: Saved event history for %d events to %s", len(history), path)
	return nil
}

// updateHistory updates the history for a completed event
func (s *Scheduler) updateHistory(result EventResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create history entry
	h, exists := s.history[result.EventID]
	if !exists {
		h = &EventHistory{
			EventID: result.EventID,
		}
		s.history[result.EventID] = h
	}

	// Update statistics
	h.LastRun = result.EndTime
	h.LastDuration = result.EndTime.Sub(result.StartTime).Milliseconds()
	h.RunCount++

	if result.Success {
		h.LastStatus = "success"
		h.SuccessCount++
	} else {
		if result.Error != nil && result.Error.Error() == "context deadline exceeded" {
			h.LastStatus = "timeout"
		} else {
			h.LastStatus = "failure"
		}
		h.FailureCount++
	}

	log.Printf("DEBUG: Updated history for event '%s': status=%s, duration=%dms, runs=%d, success=%d, failures=%d",
		result.EventID, h.LastStatus, h.LastDuration, h.RunCount, h.SuccessCount, h.FailureCount)
}
