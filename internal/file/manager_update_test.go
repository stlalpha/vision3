package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func setupTestFileManagerForUpdate(t *testing.T) (*FileManager, uuid.UUID) {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(configDir, 0755)

	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	data, _ := json.Marshal(areas)
	os.WriteFile(filepath.Join(configDir, "file_areas.json"), data, 0644)

	fm, err := NewFileManager(dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create FileManager: %v", err)
	}

	fileID := uuid.New()
	rec := FileRecord{
		ID:          fileID,
		AreaID:      1,
		Filename:    "TEST.ZIP",
		Description: "Original description",
		Size:        1024,
		UploadedAt:  time.Now(),
		UploadedBy:  "TestUser",
	}
	if err := fm.AddFileRecord(rec); err != nil {
		t.Fatalf("failed to add test record: %v", err)
	}

	return fm, fileID
}

func TestUpdateFileRecord_UpdateDescription(t *testing.T) {
	fm, fileID := setupTestFileManagerForUpdate(t)

	err := fm.UpdateFileRecord(fileID, func(r *FileRecord) {
		r.Description = "Updated from FILE_ID.DIZ"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the update persisted
	files := fm.GetFilesForArea(1)
	found := false
	for _, f := range files {
		if f.ID == fileID {
			found = true
			if f.Description != "Updated from FILE_ID.DIZ" {
				t.Errorf("expected updated description, got %q", f.Description)
			}
		}
	}
	if !found {
		t.Error("file record not found after update")
	}
}

func TestUpdateFileRecord_NotFound(t *testing.T) {
	fm, _ := setupTestFileManagerForUpdate(t)

	err := fm.UpdateFileRecord(uuid.New(), func(r *FileRecord) {
		r.Description = "Should not work"
	})
	if err == nil {
		t.Error("expected error for non-existent file ID")
	}
}

func TestUpdateFileDescription_Convenience(t *testing.T) {
	fm, fileID := setupTestFileManagerForUpdate(t)

	err := fm.UpdateFileDescription(fileID, "New DIZ description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := fm.GetFilesForArea(1)
	for _, f := range files {
		if f.ID == fileID {
			if f.Description != "New DIZ description" {
				t.Errorf("expected 'New DIZ description', got %q", f.Description)
			}
			return
		}
	}
	t.Error("file record not found after update")
}

func TestUpdateFileRecord_PersistsToJSON(t *testing.T) {
	fm, fileID := setupTestFileManagerForUpdate(t)

	err := fm.UpdateFileDescription(fileID, "Persisted description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read metadata.json directly to verify persistence
	metadataPath := filepath.Join(fm.basePath, "utils", "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read metadata.json: %v", err)
	}

	var records []FileRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("failed to parse metadata.json: %v", err)
	}

	for _, r := range records {
		if r.ID == fileID {
			if r.Description != "Persisted description" {
				t.Errorf("expected persisted description, got %q", r.Description)
			}
			return
		}
	}
	t.Error("file record not found in metadata.json")
}
