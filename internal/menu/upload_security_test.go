package menu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanDirectoryFiles_SkipsSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file
	os.WriteFile(filepath.Join(tmpDir, "regular.txt"), []byte("hello"), 0644)

	// Create a symlink
	os.Symlink("/etc/passwd", filepath.Join(tmpDir, "evil_link"))

	files, err := scanDirectoryFiles(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := files["evil_link"]; ok {
		t.Error("symlink should be excluded from scan results")
	}
	if _, ok := files["regular.txt"]; !ok {
		t.Error("regular file should be included in scan results")
	}
}

func TestScanDirectoryFiles_SkipsMetadataJson(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "metadata.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "good.zip"), []byte("data"), 0644)

	files, err := scanDirectoryFiles(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := files["metadata.json"]; ok {
		t.Error("metadata.json should be excluded")
	}
	if _, ok := files["good.zip"]; !ok {
		t.Error("regular file should be included")
	}
}

func TestSanitizeControlChars_StripsEscapes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"hello\x1b[31mred\x1b[0m", "hello[31mred[0m"},
		{"line\x00null", "linenull"},
		{"bell\x07ring", "bellring"},
		{"tab\tok", "tab\tok"}, // tabs preserved
		{"newline\nok", "newline\nok"}, // newlines preserved
		{"", ""},
	}

	for _, tt := range tests {
		result := sanitizeControlChars(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeControlChars(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
