package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stlalpha/vision3/internal/file"
)

func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name   string
		expect bool
	}{
		{"FILES.BBS", true},
		{"files.bbs", true},
		{"metadata.json", true},
		{".hidden", true},
		{".DS_Store", true},
		{"Thumbs.db", true},
		{"desktop.ini", true},
		{"COOLAPP.ZIP", false},
		{"readme.txt", false},
		{"FILE_ID.DIZ", false},
	}
	for _, tc := range tests {
		got := shouldSkipFile(tc.name)
		if got != tc.expect {
			t.Errorf("shouldSkipFile(%q) = %v, want %v", tc.name, got, tc.expect)
		}
	}
}

func TestFindAreaByTag(t *testing.T) {
	areas := []file.FileArea{
		{ID: 1, Tag: "GENERAL", Name: "General Files"},
		{ID: 2, Tag: "UPLOADS", Name: "Upload Queue"},
	}

	tests := []struct {
		tag    string
		wantID int
		found  bool
	}{
		{"GENERAL", 1, true},
		{"general", 1, true},
		{"General", 1, true},
		{"UPLOADS", 2, true},
		{"MISSING", 0, false},
	}
	for _, tc := range tests {
		got := findAreaByTag(areas, tc.tag)
		if tc.found {
			if got == nil {
				t.Errorf("findAreaByTag(%q) returned nil, want ID %d", tc.tag, tc.wantID)
			} else if got.ID != tc.wantID {
				t.Errorf("findAreaByTag(%q).ID = %d, want %d", tc.tag, got.ID, tc.wantID)
			}
		} else if got != nil {
			t.Errorf("findAreaByTag(%q) = %+v, want nil", tc.tag, got)
		}
	}
}

func TestLoadAndSaveMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	records, err := loadMetadata(tmpDir)
	if err != nil {
		t.Fatalf("loadMetadata on empty dir: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}

	testRecords := []file.FileRecord{
		{Filename: "test.zip", Size: 1024, UploadedBy: "Sysop"},
	}
	if err := saveMetadata(tmpDir, testRecords); err != nil {
		t.Fatalf("saveMetadata: %v", err)
	}

	loaded, err := loadMetadata(tmpDir)
	if err != nil {
		t.Fatalf("loadMetadata after save: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 record, got %d", len(loaded))
	}
	if loaded[0].Filename != "test.zip" {
		t.Errorf("filename = %q, want %q", loaded[0].Filename, "test.zip")
	}
}

func TestLoadFileAreas(t *testing.T) {
	tmpDir := t.TempDir()
	areas := []file.FileArea{
		{ID: 1, Tag: "TEST", Name: "Test Area", Path: "test"},
	}
	data, err := json.MarshalIndent(areas, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file_areas.json"), data, 0644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	loaded, err := loadFileAreas(tmpDir)
	if err != nil {
		t.Fatalf("loadFileAreas: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 area, got %d", len(loaded))
	}
	if loaded[0].Tag != "TEST" {
		t.Errorf("tag = %q, want %q", loaded[0].Tag, "TEST")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tc := range tests {
		got := formatSize(tc.bytes)
		if got != tc.want {
			t.Errorf("formatSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")
	content := []byte("test content")

	os.WriteFile(src, content, 0644)

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		t.Error("source file should still exist after copy")
	}
}

func TestMoveFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")
	content := []byte("move test")

	os.WriteFile(src, content, 0644)

	if err := moveFile(src, dst); err != nil {
		t.Fatalf("moveFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content = %q, want %q", got, content)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source file should not exist after move")
	}
}
