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

// --- DeleteFileRecord tests ---

func TestDeleteFileRecord_RemovesRecord(t *testing.T) {
	fm, fileID := setupTestFileManagerForUpdate(t)

	if err := fm.DeleteFileRecord(fileID, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := fm.GetFilesForArea(1)
	for _, f := range files {
		if f.ID == fileID {
			t.Error("file record still present after delete")
		}
	}
}

func TestDeleteFileRecord_PersistsToJSON(t *testing.T) {
	fm, fileID := setupTestFileManagerForUpdate(t)

	if err := fm.DeleteFileRecord(fileID, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
			t.Error("file record still in metadata.json after delete")
		}
	}
}

func TestDeleteFileRecord_DeletesFromDisk(t *testing.T) {
	fm, fileID := setupTestFileManagerForUpdate(t)

	// Create a real file on disk so os.Remove has something to act on.
	areaPath := filepath.Join(fm.basePath, "utils")
	filePath := filepath.Join(areaPath, "TEST.ZIP")
	if err := os.WriteFile(filePath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	if err := fm.DeleteFileRecord(fileID, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("expected file to be removed from disk")
	}
}

func TestDeleteFileRecord_NotFound(t *testing.T) {
	fm, _ := setupTestFileManagerForUpdate(t)

	err := fm.DeleteFileRecord(uuid.New(), false)
	if err == nil {
		t.Error("expected error for non-existent file ID")
	}
}

// --- MoveFileRecord tests ---

func setupTestFileManagerTwoAreas(t *testing.T) (*FileManager, uuid.UUID) {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(configDir, 0755)

	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
		{ID: 2, Tag: "GAMES", Name: "Games", Path: "games"},
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
		Filename:    "GAME.ZIP",
		Description: "A great game",
		Size:        2048,
		UploadedAt:  time.Now(),
		UploadedBy:  "TestUser",
	}
	if err := fm.AddFileRecord(rec); err != nil {
		t.Fatalf("failed to add test record: %v", err)
	}

	// Create physical file so Rename works.
	srcPath := filepath.Join(fm.basePath, "utils", "GAME.ZIP")
	if err := os.WriteFile(srcPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	return fm, fileID
}

func TestMoveFileRecord_MovesRecordAndFile(t *testing.T) {
	fm, fileID := setupTestFileManagerTwoAreas(t)

	if err := fm.MoveFileRecord(fileID, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Not in source area.
	for _, f := range fm.GetFilesForArea(1) {
		if f.ID == fileID {
			t.Error("record still in source area after move")
		}
	}

	// Present in target area with updated AreaID.
	found := false
	for _, f := range fm.GetFilesForArea(2) {
		if f.ID == fileID {
			found = true
			if f.AreaID != 2 {
				t.Errorf("expected AreaID=2, got %d", f.AreaID)
			}
		}
	}
	if !found {
		t.Error("record not found in target area after move")
	}

	// File exists at destination, not at source.
	dstPath := filepath.Join(fm.basePath, "games", "GAME.ZIP")
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Error("file not found at destination after move")
	}
	srcPath := filepath.Join(fm.basePath, "utils", "GAME.ZIP")
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("file still exists at source after move")
	}
}

func TestMoveFileRecord_InvalidTargetArea(t *testing.T) {
	fm, fileID := setupTestFileManagerTwoAreas(t)

	err := fm.MoveFileRecord(fileID, 99)
	if err == nil {
		t.Error("expected error for non-existent target area")
	}
}

func TestMoveFileRecord_SameArea(t *testing.T) {
	fm, fileID := setupTestFileManagerTwoAreas(t)

	err := fm.MoveFileRecord(fileID, 1)
	if err == nil {
		t.Error("expected error when moving to the same area")
	}
}

func TestMoveFileRecord_NotFound(t *testing.T) {
	fm, _ := setupTestFileManagerTwoAreas(t)

	err := fm.MoveFileRecord(uuid.New(), 2)
	if err == nil {
		t.Error("expected error for non-existent file ID")
	}
}
