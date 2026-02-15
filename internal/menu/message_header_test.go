package menu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractHeaderNumber(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int
		wantErr  bool
	}{
		{"single digit", "MSGHDR.1.ans", 1, false},
		{"double digit", "MSGHDR.15.ans", 15, false},
		{"triple digit", "MSGHDR.100.ans", 100, false},
		{"invalid format", "MSGHDR.ans", 0, true},
		{"non-numeric", "MSGHDR.ABC.ans", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractHeaderNumber(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractHeaderNumber(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("extractHeaderNumber(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestDiscoverMessageHeaders(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()
	msgHdrDir := filepath.Join(tmpDir, "message_headers")
	if err := os.MkdirAll(msgHdrDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test template files
	testFiles := []string{
		"MSGHDR.1.ans",
		"MSGHDR.2.ans",
		"MSGHDR.10.ans",
		"MSGHDR.15.ans",
		"MSGHDR.ANS", // Should be excluded
	}

	for _, filename := range testFiles {
		path := filepath.Join(msgHdrDir, filename)
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Run discovery
	templates, err := discoverMessageHeaders(tmpDir)
	if err != nil {
		t.Fatalf("discoverMessageHeaders() error = %v", err)
	}

	// Verify results
	expectedCount := 4 // Should exclude MSGHDR.ANS
	if len(templates) != expectedCount {
		t.Errorf("Got %d templates, want %d", len(templates), expectedCount)
	}

	// Verify sorted order (1, 2, 10, 15)
	expectedNumbers := []int{1, 2, 10, 15}
	for i, tmpl := range templates {
		if tmpl.Number != expectedNumbers[i] {
			t.Errorf("Template[%d].Number = %d, want %d", i, tmpl.Number, expectedNumbers[i])
		}
	}

	// Verify first template details
	if len(templates) > 0 {
		first := templates[0]
		if first.Filename != "MSGHDR.1.ans" {
			t.Errorf("First template filename = %q, want %q", first.Filename, "MSGHDR.1.ans")
		}
		// Display name comes from BAR file - verify it's not empty
		// (actual name is "Generic Blue Box" per MSGHDR.BAR configuration)
		if first.DisplayName == "" {
			t.Errorf("First template display name is empty, want non-empty string from BAR file")
		}
	}
}

func TestDiscoverMessageHeadersEmpty(t *testing.T) {
	// Create temporary test directory with no templates
	tmpDir := t.TempDir()
	msgHdrDir := filepath.Join(tmpDir, "message_headers")
	if err := os.MkdirAll(msgHdrDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	templates, err := discoverMessageHeaders(tmpDir)
	if err != nil {
		t.Fatalf("discoverMessageHeaders() error = %v", err)
	}

	if len(templates) != 0 {
		t.Errorf("Expected empty result, got %d templates", len(templates))
	}
}
