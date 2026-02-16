package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// setupTestFileManager creates a FileManager with temp dirs and the given areas config.
func setupTestFileManager(t *testing.T, areas []FileArea) *FileManager {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(configDir, 0755)

	data, err := json.Marshal(areas)
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}
	os.WriteFile(filepath.Join(configDir, "file_areas.json"), data, 0644)

	fm, err := NewFileManager(dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create FileManager: %v", err)
	}
	return fm
}

func TestNewFileManager_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "configs")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(configDir, 0755)

	fm, err := NewFileManager(dataDir, configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.ListAreas()) != 0 {
		t.Errorf("expected 0 areas, got %d", len(fm.ListAreas()))
	}
}

func TestNewFileManager_ValidAreas(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
		{ID: 2, Tag: "TEXTS", Name: "Text Files", Path: "texts"},
	}
	fm := setupTestFileManager(t, areas)

	listed := fm.ListAreas()
	if len(listed) != 2 {
		t.Fatalf("expected 2 areas, got %d", len(listed))
	}
	// Should be sorted by ID
	if listed[0].ID != 1 || listed[1].ID != 2 {
		t.Errorf("areas not sorted by ID: %v", listed)
	}
}

func TestNewFileManager_SkipsInvalidAreas(t *testing.T) {
	areas := []FileArea{
		{ID: 0, Tag: "BAD", Name: "Zero ID", Path: "bad"},         // ID <= 0
		{ID: 1, Tag: "", Name: "Empty Tag", Path: "empty"},        // empty tag
		{ID: 2, Tag: "ABS", Name: "Absolute Path", Path: "/etc"},  // absolute path
		{ID: 3, Tag: "TRAV", Name: "Traversal", Path: "../escape"}, // path traversal
		{ID: 4, Tag: "GOOD", Name: "Valid Area", Path: "good"},     // valid
	}
	fm := setupTestFileManager(t, areas)

	listed := fm.ListAreas()
	if len(listed) != 1 {
		t.Fatalf("expected 1 valid area, got %d", len(listed))
	}
	if listed[0].Tag != "GOOD" {
		t.Errorf("expected GOOD area, got %s", listed[0].Tag)
	}
}

func TestNewFileManager_SkipsDuplicates(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
		{ID: 1, Tag: "DUPE", Name: "Duplicate ID", Path: "dupe"},  // duplicate ID
		{ID: 2, Tag: "UTILS", Name: "Duplicate Tag", Path: "dup2"}, // duplicate tag
	}
	fm := setupTestFileManager(t, areas)

	listed := fm.ListAreas()
	if len(listed) != 1 {
		t.Fatalf("expected 1 area (duplicates skipped), got %d", len(listed))
	}
}

func TestGetAreaByTag(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	area, ok := fm.GetAreaByTag("utils") // case insensitive
	if !ok {
		t.Fatal("expected to find area by tag 'utils'")
	}
	if area.ID != 1 {
		t.Errorf("expected area ID 1, got %d", area.ID)
	}

	_, ok = fm.GetAreaByTag("NONEXISTENT")
	if ok {
		t.Error("expected not to find nonexistent area")
	}
}

func TestGetAreaByID(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	area, ok := fm.GetAreaByID(1)
	if !ok {
		t.Fatal("expected to find area by ID 1")
	}
	if area.Tag != "UTILS" {
		t.Errorf("expected tag UTILS, got %s", area.Tag)
	}

	_, ok = fm.GetAreaByID(999)
	if ok {
		t.Error("expected not to find nonexistent area ID")
	}
}

func TestGetFilesForArea_Empty(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	files := fm.GetFilesForArea(1)
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestGetFileCountForArea(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	count, err := fm.GetFileCountForArea(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files, got %d", count)
	}

	count, err = fm.GetFileCountForArea(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for nonexistent area, got %d", count)
	}
}

func TestAddFileRecord(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	record := FileRecord{
		ID:         uuid.New(),
		AreaID:     1,
		Filename:   "test.zip",
		Description: "A test file",
		Size:       1024,
		UploadedAt: time.Now(),
		UploadedBy: "testuser",
	}

	err := fm.AddFileRecord(record)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := fm.GetFilesForArea(1)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Filename != "test.zip" {
		t.Errorf("expected test.zip, got %s", files[0].Filename)
	}

	count, _ := fm.GetFileCountForArea(1)
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestAddFileRecord_Validation(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	// Missing ID
	err := fm.AddFileRecord(FileRecord{AreaID: 1, Filename: "test.zip"})
	if err == nil {
		t.Error("expected error for nil UUID")
	}

	// Missing AreaID
	err = fm.AddFileRecord(FileRecord{ID: uuid.New(), Filename: "test.zip"})
	if err == nil {
		t.Error("expected error for zero AreaID")
	}

	// Missing filename
	err = fm.AddFileRecord(FileRecord{ID: uuid.New(), AreaID: 1})
	if err == nil {
		t.Error("expected error for empty filename")
	}

	// Nonexistent area
	err = fm.AddFileRecord(FileRecord{ID: uuid.New(), AreaID: 999, Filename: "test.zip"})
	if err == nil {
		t.Error("expected error for nonexistent area")
	}
}

func TestGetFilesForAreaPaginated(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	// Add 5 records
	for i := 0; i < 5; i++ {
		fm.AddFileRecord(FileRecord{
			ID:       uuid.New(),
			AreaID:   1,
			Filename: "file" + string(rune('A'+i)) + ".zip",
			Size:     int64(i * 100),
		})
	}

	// Page 1, size 2
	page, err := fm.GetFilesForAreaPaginated(1, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page) != 2 {
		t.Errorf("expected 2 records on page 1, got %d", len(page))
	}

	// Page 3, size 2 (only 1 record left)
	page, err = fm.GetFilesForAreaPaginated(1, 3, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page) != 1 {
		t.Errorf("expected 1 record on last page, got %d", len(page))
	}

	// Page beyond range
	page, err = fm.GetFilesForAreaPaginated(1, 10, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page) != 0 {
		t.Errorf("expected 0 records beyond range, got %d", len(page))
	}

	// Invalid page
	_, err = fm.GetFilesForAreaPaginated(1, 0, 2)
	if err == nil {
		t.Error("expected error for page 0")
	}
}

func TestIncrementDownloadCount(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	fileID := uuid.New()
	fm.AddFileRecord(FileRecord{
		ID:       fileID,
		AreaID:   1,
		Filename: "test.zip",
	})

	err := fm.IncrementDownloadCount(fileID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files := fm.GetFilesForArea(1)
	if files[0].DownloadCount != 1 {
		t.Errorf("expected download count 1, got %d", files[0].DownloadCount)
	}

	// Increment again
	fm.IncrementDownloadCount(fileID)
	files = fm.GetFilesForArea(1)
	if files[0].DownloadCount != 2 {
		t.Errorf("expected download count 2, got %d", files[0].DownloadCount)
	}

	// Nonexistent file
	err = fm.IncrementDownloadCount(uuid.New())
	if err == nil {
		t.Error("expected error for nonexistent file ID")
	}
}

func TestGetTotalFileCount_Empty(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
		{ID: 2, Tag: "GAMES", Name: "Games", Path: "games"},
	}
	fm := setupTestFileManager(t, areas)

	count := fm.GetTotalFileCount()
	if count != 0 {
		t.Errorf("expected 0 files for empty areas, got %d", count)
	}
}

func TestIsSupportedArchive(t *testing.T) {
	fm := &FileManager{}

	tests := []struct {
		filename string
		want     bool
	}{
		{"test.zip", true},
		{"test.ZIP", true},
		{"test.Zip", true},
		{"test.tar.gz", false},
		{"test.rar", false},
		{"test.txt", false},
		{"", false},
	}
	for _, tt := range tests {
		got := fm.IsSupportedArchive(tt.filename)
		if got != tt.want {
			t.Errorf("IsSupportedArchive(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}

func TestGetFilePath(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	fileID := uuid.New()
	fm.AddFileRecord(FileRecord{
		ID:       fileID,
		AreaID:   1,
		Filename: "safe_file.zip",
	})

	path, err := fm.GetFilePath(fileID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %s", path)
	}
	if filepath.Base(path) != "safe_file.zip" {
		t.Errorf("expected safe_file.zip, got %s", filepath.Base(path))
	}

	// Nonexistent file
	_, err = fm.GetFilePath(uuid.New())
	if err == nil {
		t.Error("expected error for nonexistent file ID")
	}
}

func TestGetFilePath_TraversalRejected(t *testing.T) {
	areas := []FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm := setupTestFileManager(t, areas)

	// Directly inject a record with a malicious filename to test GetFilePath's safety checks.
	// AddFileRecord uses filepath.Base internally in GetFilePath, so we bypass validation
	// by writing directly to the internal map.
	maliciousID := uuid.New()
	fm.muFiles.Lock()
	fm.fileRecords[1] = append(fm.fileRecords[1], FileRecord{
		ID:       maliciousID,
		AreaID:   1,
		Filename: "../../etc/passwd",
	})
	fm.muFiles.Unlock()

	path, err := fm.GetFilePath(maliciousID)
	if err != nil {
		// GetFilePath rejects the traversal — this is correct
		return
	}
	// If no error, the path must have been sanitized to just "passwd" via filepath.Base
	if filepath.Base(path) != "passwd" {
		t.Errorf("expected sanitized filename 'passwd', got %s", filepath.Base(path))
	}
	// The resolved path must still be within the base directory
	absBase, _ := filepath.Abs(fm.basePath)
	if !strings.HasPrefix(path, absBase) {
		t.Errorf("path %s escaped base directory %s", path, absBase)
	}
}
