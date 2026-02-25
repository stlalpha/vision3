package message

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestEditAreaRoundTrip verifies that SaveAreas persists all fields — including
// the new Sponsor field — and that a subsequent load reads them back correctly.
func TestEditAreaRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	initial := []*MessageArea{
		{
			ID:          1,
			Tag:         "GENERAL",
			Name:        "General Discussion",
			Description: "General area",
			ACSRead:     "",
			ACSWrite:    "",
			BasePath:    "msgbases/general",
			AreaType:    "local",
		},
		{
			ID:          2,
			Tag:         "TECH",
			Name:        "Tech Talk",
			Description: "Technology discussion",
			ACSRead:     "S10",
			ACSWrite:    "S10",
			BasePath:    "msgbases/tech",
			AreaType:    "local",
			Sponsor:     "TechGuru",
		},
	}

	areasFile := filepath.Join(configDir, "message_areas.json")
	data, _ := json.MarshalIndent(initial, "", "  ")
	if err := os.WriteFile(areasFile, data, 0644); err != nil {
		t.Fatalf("write initial areas: %v", err)
	}

	mm, err := NewMessageManager(tmpDir, configDir, "TestBBS", nil)
	if err != nil {
		t.Fatalf("NewMessageManager: %v", err)
	}

	// Mutate: assign a sponsor to GENERAL and update TECH's name.
	// Use UpdateAreaByID (not direct pointer modification) per API contract.
	general, ok := mm.GetAreaByID(1)
	if !ok {
		t.Fatal("area 1 (GENERAL) not found after load")
	}
	modifiedGeneral := *general
	modifiedGeneral.Sponsor = "AliceHandle"
	modifiedGeneral.Name = "General Chat"
	if err := mm.UpdateAreaByID(1, modifiedGeneral); err != nil {
		t.Fatalf("UpdateAreaByID(1): %v", err)
	}

	tech, ok := mm.GetAreaByID(2)
	if !ok {
		t.Fatal("area 2 (TECH) not found after load")
	}
	modifiedTech := *tech
	modifiedTech.MaxMessages = 500
	if err := mm.UpdateAreaByID(2, modifiedTech); err != nil {
		t.Fatalf("UpdateAreaByID(2): %v", err)
	}

	// Save.
	if err := mm.SaveAreas(); err != nil {
		t.Fatalf("SaveAreas: %v", err)
	}

	// Reload from disk into a fresh manager.
	mm2, err := NewMessageManager(tmpDir, configDir, "TestBBS", nil)
	if err != nil {
		t.Fatalf("NewMessageManager (reload): %v", err)
	}

	// Verify GENERAL.
	g2, ok := mm2.GetAreaByID(1)
	if !ok {
		t.Fatal("area 1 missing after reload")
	}
	if g2.Sponsor != "AliceHandle" {
		t.Errorf("GENERAL Sponsor: got %q, want %q", g2.Sponsor, "AliceHandle")
	}
	if g2.Name != "General Chat" {
		t.Errorf("GENERAL Name: got %q, want %q", g2.Name, "General Chat")
	}

	// Verify TECH.
	t2, ok := mm2.GetAreaByID(2)
	if !ok {
		t.Fatal("area 2 missing after reload")
	}
	if t2.Sponsor != "TechGuru" {
		t.Errorf("TECH Sponsor: got %q, want %q", t2.Sponsor, "TechGuru")
	}
	if t2.MaxMessages != 500 {
		t.Errorf("TECH MaxMessages: got %d, want 500", t2.MaxMessages)
	}

	// Confirm the file on disk is valid JSON with correct content.
	raw, err := os.ReadFile(areasFile)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	var saved []*MessageArea
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("unmarshal saved file: %v", err)
	}
	if len(saved) != 2 {
		t.Errorf("saved area count: got %d, want 2", len(saved))
	}
}

// TestSaveAreasPreservesOrder verifies that SaveAreas always writes areas in
// ascending ID order regardless of internal map iteration order.
func TestSaveAreasPreservesOrder(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(configDir, 0755)

	areas := []*MessageArea{
		{ID: 3, Tag: "THIRD", Name: "Third", BasePath: "msgbases/third", AreaType: "local"},
		{ID: 1, Tag: "FIRST", Name: "First", BasePath: "msgbases/first", AreaType: "local"},
		{ID: 2, Tag: "SECOND", Name: "Second", BasePath: "msgbases/second", AreaType: "local"},
	}
	data, _ := json.MarshalIndent(areas, "", "  ")
	os.WriteFile(filepath.Join(configDir, "message_areas.json"), data, 0644)

	mm, err := NewMessageManager(tmpDir, configDir, "TestBBS", nil)
	if err != nil {
		t.Fatalf("NewMessageManager: %v", err)
	}
	if err := mm.SaveAreas(); err != nil {
		t.Fatalf("SaveAreas: %v", err)
	}

	raw, _ := os.ReadFile(filepath.Join(configDir, "message_areas.json"))
	var saved []*MessageArea
	json.Unmarshal(raw, &saved)

	for i, a := range saved {
		expected := i + 1
		if a.ID != expected {
			t.Errorf("saved[%d].ID = %d, want %d", i, a.ID, expected)
		}
	}
}
