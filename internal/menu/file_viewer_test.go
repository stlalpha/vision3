package menu

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stlalpha/vision3/internal/file"
)

// setupTestFileManagerForViewer creates a FileManager with temp dirs, areas, and optional files on disk.
func setupTestFileManagerForViewer(t *testing.T, areas []file.FileArea) (*file.FileManager, string) {
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

	fm, err := file.NewFileManager(dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create FileManager: %v", err)
	}

	// Create area directories
	for _, area := range areas {
		os.MkdirAll(filepath.Join(dataDir, "files", area.Path), 0755)
	}

	return fm, filepath.Join(dataDir, "files")
}

func TestFindFileInArea_ExactMatch(t *testing.T) {
	areas := []file.FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm, _ := setupTestFileManagerForViewer(t, areas)

	// Add a file record
	rec := file.FileRecord{
		ID:         uuid.New(),
		AreaID:     1,
		Filename:   "README.TXT",
		Size:       100,
		UploadedAt: time.Now(),
		UploadedBy: "TestUser",
	}
	err := fm.AddFileRecord(rec)
	if err != nil {
		t.Fatalf("failed to add file record: %v", err)
	}

	// Exact match
	found, err := findFileInArea(fm, 1, "README.TXT")
	if err != nil {
		t.Errorf("expected to find file, got error: %v", err)
	}
	if found == nil || found.Filename != "README.TXT" {
		t.Errorf("expected README.TXT, got %v", found)
	}
}

func TestFindFileInArea_CaseInsensitive(t *testing.T) {
	areas := []file.FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm, _ := setupTestFileManagerForViewer(t, areas)

	rec := file.FileRecord{
		ID:         uuid.New(),
		AreaID:     1,
		Filename:   "README.TXT",
		Size:       100,
		UploadedAt: time.Now(),
		UploadedBy: "TestUser",
	}
	fm.AddFileRecord(rec)

	// Case-insensitive search
	found, err := findFileInArea(fm, 1, "readme.txt")
	if err != nil {
		t.Errorf("expected case-insensitive match, got error: %v", err)
	}
	if found == nil || found.Filename != "README.TXT" {
		t.Errorf("expected README.TXT, got %v", found)
	}
}

func TestFindFileInArea_NotFound(t *testing.T) {
	areas := []file.FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm, _ := setupTestFileManagerForViewer(t, areas)

	_, err := findFileInArea(fm, 1, "NOFILE.ZIP")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestFindFileInArea_EmptyArea(t *testing.T) {
	areas := []file.FileArea{
		{ID: 1, Tag: "UTILS", Name: "Utilities", Path: "utils"},
	}
	fm, _ := setupTestFileManagerForViewer(t, areas)

	_, err := findFileInArea(fm, 1, "anything.txt")
	if err == nil {
		t.Error("expected error for empty area, got nil")
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0"},
		{512, "512"},
		{1023, "1023"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{1572864, "1.5M"},
		{1073741824, "1.0G"},
	}

	for _, tt := range tests {
		result := formatFileSize(tt.size)
		if result != tt.expected {
			t.Errorf("formatFileSize(%d) = %q, want %q", tt.size, result, tt.expected)
		}
	}
}

func TestDisplayArchiveListing_ValidZip(t *testing.T) {
	// Create a test ZIP file with known contents
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")

	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip file: %v", err)
	}

	w := zip.NewWriter(zipFile)

	// Add a file to the ZIP
	f, err := w.Create("hello.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	f.Write([]byte("Hello, World!"))

	f2, err := w.Create("subdir/data.bin")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	f2.Write([]byte("binary data here"))

	w.Close()
	zipFile.Close()

	// Capture output by calling displayArchiveListing with a buffer-based writer
	var buf bytes.Buffer
	displayArchiveListing_toWriter(&buf, zipPath, "test.zip", 24)

	output := buf.String()

	// Verify the output contains expected file names
	if !bytes.Contains([]byte(output), []byte("hello.txt")) {
		t.Errorf("expected output to contain 'hello.txt', got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("subdir/data.bin")) {
		t.Errorf("expected output to contain 'subdir/data.bin', got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("2 file(s)")) {
		t.Errorf("expected output to contain '2 file(s)', got: %s", output)
	}
}

func TestDisplayArchiveListing_InvalidZip(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "notazip.zip")
	os.WriteFile(badPath, []byte("this is not a zip file"), 0644)

	var buf bytes.Buffer
	displayArchiveListing_toWriter(&buf, badPath, "notazip.zip", 24)

	output := buf.String()
	if !strings.Contains(output, "Error reading archive") {
		t.Errorf("expected error message for invalid zip, got: %s", output)
	}
}

func TestDisplayArchiveListing_MissingFile(t *testing.T) {
	var buf bytes.Buffer
	displayArchiveListing_toWriter(&buf, "/nonexistent/path.zip", "nope.zip", 24)

	output := buf.String()
	if !strings.Contains(output, "Error reading archive") {
		t.Errorf("expected error message for missing file, got: %s", output)
	}
}

func TestDisplayTextWithPaging_MissingFile(t *testing.T) {
	var buf bytes.Buffer
	displayTextWithPaging_toWriter(&buf, "/nonexistent/file.txt", "nope.txt", 24)

	output := buf.String()
	if !strings.Contains(output, "Error opening file") {
		t.Errorf("expected error message for missing file, got: %s", output)
	}
}

func TestDisplayArchiveListing_EmptyZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "empty.zip")

	zipFile, _ := os.Create(zipPath)
	w := zip.NewWriter(zipFile)
	w.Close()
	zipFile.Close()

	var buf bytes.Buffer
	displayArchiveListing_toWriter(&buf, zipPath, "empty.zip", 24)

	output := buf.String()
	if !strings.Contains(output, "0 file(s)") {
		t.Errorf("expected '0 file(s)' for empty zip, got: %s", output)
	}
}

func TestDisplayTextWithPaging_SmallFile(t *testing.T) {
	// Create a small test file (fits in one screen)
	tmpDir := t.TempDir()
	textPath := filepath.Join(tmpDir, "small.txt")
	content := "Line 1\nLine 2\nLine 3\n"
	os.WriteFile(textPath, []byte(content), 0644)

	var buf bytes.Buffer
	displayTextWithPaging_toWriter(&buf, textPath, "small.txt", 24)

	output := buf.String()
	if !strings.Contains(output, "Line 1") {
		t.Errorf("expected output to contain 'Line 1', got: %s", output)
	}
	if !strings.Contains(output, "Line 3") {
		t.Errorf("expected output to contain 'Line 3', got: %s", output)
	}
	if !strings.Contains(output, "End of File") {
		t.Errorf("expected output to contain end-of-file marker, got: %s", output)
	}
}

func TestDisplayTextWithPaging_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	textPath := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(textPath, []byte(""), 0644)

	var buf bytes.Buffer
	displayTextWithPaging_toWriter(&buf, textPath, "empty.txt", 24)

	output := buf.String()
	if !strings.Contains(output, "Viewing: empty.txt") {
		t.Errorf("expected header with filename, got: %s", output)
	}
	if !strings.Contains(output, "End of File") {
		t.Errorf("expected end-of-file marker for empty file, got: %s", output)
	}
}

func TestViewFileByRecord_RegistrationExists(t *testing.T) {
	// Verify VIEW_FILE and TYPE_TEXT_FILE are registered commands
	registry := make(map[string]RunnableFunc)
	registerAppRunnables(registry)

	if _, ok := registry["VIEW_FILE"]; !ok {
		t.Error("VIEW_FILE not registered in command registry")
	}
	if _, ok := registry["TYPE_TEXT_FILE"]; !ok {
		t.Error("TYPE_TEXT_FILE not registered in command registry")
	}
}
