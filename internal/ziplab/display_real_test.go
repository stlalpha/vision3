package ziplab

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseNFO_RealZipLabFile(t *testing.T) {
	// Test against the actual ZIPLAB.NFO from the menu set
	realNFO := filepath.Join("..", "..", "menus", "v3", "ansi", "ZIPLAB.NFO")
	if _, err := os.Stat(realNFO); os.IsNotExist(err) {
		t.Skip("ZIPLAB.NFO not found at expected path, skipping integration test")
	}

	nfo, err := ParseNFO(realNFO)
	if err != nil {
		t.Fatalf("failed to parse real ZIPLAB.NFO: %v", err)
	}

	// The real file defines steps 1, 2, 3, 5, 6, 7 (step 4 is commented out)
	expectedSteps := []int{1, 2, 3, 5, 6, 7}
	for _, step := range expectedSteps {
		if !nfo.HasStep(step) {
			t.Errorf("expected step %d to be present in real ZIPLAB.NFO", step)
		}
	}

	// Step 4 should NOT be present (commented out in real file)
	if nfo.HasStep(4) {
		t.Error("step 4 should NOT be present (commented out in real file)")
	}

	// Each step should have D, P, and F entries
	for _, step := range expectedSteps {
		for _, status := range []Status{StatusDoing, StatusPass, StatusFail} {
			entry, ok := nfo.GetEntry(step, status)
			if !ok {
				t.Errorf("missing entry for step %d, status %s", step, status)
				continue
			}
			if entry.Col <= 0 || entry.Row <= 0 {
				t.Errorf("step %d status %s: invalid coordinates col=%d row=%d", step, status, entry.Col, entry.Row)
			}
			if entry.DisplayChars == "" {
				t.Errorf("step %d status %s: empty display chars", step, status)
			}
		}
	}

	// Verify specific coordinates from the real file
	entry, _ := nfo.GetEntry(1, StatusDoing)
	if entry.Col != 45 || entry.Row != 10 {
		t.Errorf("step 1D: expected col=45 row=10, got col=%d row=%d", entry.Col, entry.Row)
	}

	// Total should be 6 steps * 3 statuses = 18 entries
	if len(nfo.Entries) != 18 {
		t.Errorf("expected 18 entries from real NFO, got %d", len(nfo.Entries))
	}
}
