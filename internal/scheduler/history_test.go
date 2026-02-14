package scheduler

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadHistory(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "scheduler_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	historyPath := filepath.Join(tmpDir, "event_history.json")

	// Create test history
	history := map[string]*EventHistory{
		"test_event_1": {
			EventID:      "test_event_1",
			LastRun:      time.Now(),
			LastStatus:   "success",
			LastDuration: 1234,
			RunCount:     5,
			SuccessCount: 4,
			FailureCount: 1,
		},
		"test_event_2": {
			EventID:      "test_event_2",
			LastRun:      time.Now().Add(-1 * time.Hour),
			LastStatus:   "failure",
			LastDuration: 5678,
			RunCount:     10,
			SuccessCount: 8,
			FailureCount: 2,
		},
	}

	// Save history
	err = SaveHistory(historyPath, history)
	if err != nil {
		t.Fatalf("Failed to save history: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		t.Fatal("History file was not created")
	}

	// Load history
	loadedHistory, err := LoadHistory(historyPath)
	if err != nil {
		t.Fatalf("Failed to load history: %v", err)
	}

	// Verify loaded history matches
	if len(loadedHistory) != len(history) {
		t.Errorf("Expected %d history entries, got %d", len(history), len(loadedHistory))
	}

	for eventID, expected := range history {
		loaded, exists := loadedHistory[eventID]
		if !exists {
			t.Errorf("Event %s not found in loaded history", eventID)
			continue
		}

		if loaded.EventID != expected.EventID {
			t.Errorf("EventID mismatch: expected %s, got %s", expected.EventID, loaded.EventID)
		}

		if loaded.LastStatus != expected.LastStatus {
			t.Errorf("LastStatus mismatch for %s: expected %s, got %s", eventID, expected.LastStatus, loaded.LastStatus)
		}

		if loaded.RunCount != expected.RunCount {
			t.Errorf("RunCount mismatch for %s: expected %d, got %d", eventID, expected.RunCount, loaded.RunCount)
		}
	}
}

func TestLoadHistory_FileNotExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scheduler_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	historyPath := filepath.Join(tmpDir, "nonexistent.json")

	// Load history from non-existent file should return empty map
	history, err := LoadHistory(historyPath)
	if err != nil {
		t.Fatalf("Expected no error for missing file, got: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d entries", len(history))
	}
}

func TestUpdateHistory(t *testing.T) {
	s := &Scheduler{
		history: make(map[string]*EventHistory),
	}

	// Test successful event
	result := EventResult{
		EventID:   "test_event",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
		Success:   true,
		ExitCode:  0,
	}

	s.updateHistory(result)

	h, exists := s.history["test_event"]
	if !exists {
		t.Fatal("History entry was not created")
	}

	if h.EventID != "test_event" {
		t.Errorf("Expected EventID 'test_event', got %s", h.EventID)
	}

	if h.LastStatus != "success" {
		t.Errorf("Expected LastStatus 'success', got %s", h.LastStatus)
	}

	if h.RunCount != 1 {
		t.Errorf("Expected RunCount 1, got %d", h.RunCount)
	}

	if h.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount 1, got %d", h.SuccessCount)
	}

	if h.FailureCount != 0 {
		t.Errorf("Expected FailureCount 0, got %d", h.FailureCount)
	}

	// Test failed event
	failResult := EventResult{
		EventID:   "test_event",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(1 * time.Second),
		Success:   false,
		ExitCode:  1,
	}

	s.updateHistory(failResult)

	if h.RunCount != 2 {
		t.Errorf("Expected RunCount 2, got %d", h.RunCount)
	}

	if h.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount 1, got %d", h.SuccessCount)
	}

	if h.FailureCount != 1 {
		t.Errorf("Expected FailureCount 1, got %d", h.FailureCount)
	}

	if h.LastStatus != "failure" {
		t.Errorf("Expected LastStatus 'failure', got %s", h.LastStatus)
	}
}
