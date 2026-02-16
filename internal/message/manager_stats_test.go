package message

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetTotalMessageCount_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "message_areas.json"), []byte("[]"), 0644)

	mm, err := NewMessageManager(tmpDir, configDir, "TestBBS", nil)
	if err != nil {
		t.Fatalf("failed to create message manager: %v", err)
	}

	count := mm.GetTotalMessageCount()
	if count != 0 {
		t.Errorf("expected 0 messages for empty areas, got %d", count)
	}
}
