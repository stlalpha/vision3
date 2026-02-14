package ziplab

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createTestZipWithTimes creates a ZIP with entries that have specific modification times.
// entries is a slice of {name, content} pairs; modTime is applied to all entries.
func createTestZipWithTimes(t *testing.T, zipPath string, entries []struct{ Name, Content string }, modTime time.Time) {
	t.Helper()
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for _, e := range entries {
		hdr := &zip.FileHeader{
			Name:     e.Name,
			Method:   zip.Deflate,
			Modified: modTime,
		}
		fw, err := w.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("failed to add file %s: %v", e.Name, err)
		}
		if _, err := fw.Write([]byte(e.Content)); err != nil {
			t.Fatalf("failed to write content for %s: %v", e.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
}

func TestFormatArchiveListing_BasicOutput(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	modTime := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	entries := []struct{ Name, Content string }{
		{"readme.txt", "hello world"},
		{"data.bin", "some binary data here"},
	}
	createTestZipWithTimes(t, zipPath, entries, modTime)

	var buf bytes.Buffer
	count, err := formatArchiveListing(&buf, zipPath, "TEST.ZIP", 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	output := buf.String()

	// Verify header contains filename
	if !strings.Contains(output, "TEST.ZIP") {
		t.Error("output missing archive filename")
	}

	// Verify numbered entries appear
	if !strings.Contains(output, "1") {
		t.Error("output missing entry number 1")
	}
	if !strings.Contains(output, "2") {
		t.Error("output missing entry number 2")
	}

	// Verify filenames appear
	if !strings.Contains(output, "readme.txt") {
		t.Error("output missing readme.txt")
	}
	if !strings.Contains(output, "data.bin") {
		t.Error("output missing data.bin")
	}

	// Verify date appears
	if !strings.Contains(output, "06/15/2025") {
		t.Error("output missing date 06/15/2025")
	}

	// Verify summary line
	if !strings.Contains(output, "2 file(s)") {
		t.Error("output missing '2 file(s)' summary")
	}

	// Verify prompt
	if !strings.Contains(output, "Extract") {
		t.Error("output missing Extract prompt")
	}
	if !strings.Contains(output, "Quit") {
		t.Error("output missing Quit prompt")
	}
}

func TestFormatArchiveListing_EmptyZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "empty.zip")

	// Create empty ZIP
	createTestZipWithTimes(t, zipPath, nil, time.Now())

	var buf bytes.Buffer
	count, err := formatArchiveListing(&buf, zipPath, "EMPTY.ZIP", 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}

	output := buf.String()
	if !strings.Contains(output, "0 file(s)") {
		t.Error("output missing '0 file(s)' summary")
	}
}

func TestFormatArchiveListing_CorruptZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "corrupt.zip")

	// Write garbage data
	if err := os.WriteFile(zipPath, []byte("this is not a zip file at all"), 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	var buf bytes.Buffer
	_, err := formatArchiveListing(&buf, zipPath, "CORRUPT.ZIP", 24)
	if err == nil {
		t.Fatal("expected error for corrupt ZIP, got nil")
	}
}

func TestFormatArchiveListing_ManyFiles(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "many.zip")
	modTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	entries := make([]struct{ Name, Content string }, 50)
	for i := range entries {
		entries[i] = struct{ Name, Content string }{
			Name:    fmt.Sprintf("file_%03d.txt", i+1),
			Content: "data",
		}
	}
	createTestZipWithTimes(t, zipPath, entries, modTime)

	var buf bytes.Buffer
	count, err := formatArchiveListing(&buf, zipPath, "MANY.ZIP", 24)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 50 {
		t.Errorf("expected count 50, got %d", count)
	}

	output := buf.String()
	if !strings.Contains(output, "50 file(s)") {
		t.Error("output missing '50 file(s)' summary")
	}
}
