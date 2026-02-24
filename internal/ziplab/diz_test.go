package ziplab

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func createTestZipWithDIZ(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
}

func TestExtractDIZFromZip_WithDIZ(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	dizContent := "Test BBS Archive\nVersion 1.0"
	createTestZipWithDIZ(t, zipPath, map[string]string{
		"readme.txt":  "hello world",
		"FILE_ID.DIZ": dizContent,
	})

	got, err := ExtractDIZFromZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dizContent {
		t.Errorf("got %q, want %q", got, dizContent)
	}
}

func TestExtractDIZFromZip_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	dizContent := "lowercase diz"
	createTestZipWithDIZ(t, zipPath, map[string]string{
		"readme.txt":  "hello",
		"file_id.diz": dizContent,
	})

	got, err := ExtractDIZFromZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dizContent {
		t.Errorf("got %q, want %q", got, dizContent)
	}
}

func TestExtractDIZFromZip_NoDIZ(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZipWithDIZ(t, zipPath, map[string]string{
		"readme.txt": "no diz here",
	})

	got, err := ExtractDIZFromZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestExtractDIZFromZip_NestedDIZ(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	dizContent := "nested diz content"
	createTestZipWithDIZ(t, zipPath, map[string]string{
		"readme.txt":          "hello",
		"subdir/FILE_ID.DIZ": dizContent,
	})

	got, err := ExtractDIZFromZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dizContent {
		t.Errorf("got %q, want %q", got, dizContent)
	}
}

func TestExtractDIZFromZip_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "notazip.zip")
	if err := os.WriteFile(badPath, []byte("not a zip"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := ExtractDIZFromZip(badPath)
	if err == nil {
		t.Error("expected error for invalid zip file")
	}
}

func TestExtractDIZFromZip_TrimsTrailingWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZipWithDIZ(t, zipPath, map[string]string{
		"FILE_ID.DIZ": "content with trailing space   \r\n\r\n",
	})

	got, err := ExtractDIZFromZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "content with trailing space" {
		t.Errorf("got %q, want %q", got, "content with trailing space")
	}
}

func TestExtractDIZFromZip_StripsCtrlZ(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZipWithDIZ(t, zipPath, map[string]string{
		"FILE_ID.DIZ": "Maximus BBS DOS v2.0  [2/4]\r\n\x1a",
	})

	got, err := ExtractDIZFromZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Maximus BBS DOS v2.0  [2/4]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractDIZFromZip_StripsEmbeddedCtrlZ(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZipWithDIZ(t, zipPath, map[string]string{
		"FILE_ID.DIZ": "Line one\r\nLine two\r\n\x1a\x1a\x1a",
	})

	got, err := ExtractDIZFromZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Line one\r\nLine two"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCleanDIZ(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trailing ctrl-z", "hello\x1a", "hello"},
		{"ctrl-z after newline", "hello\r\n\x1a", "hello"},
		{"multiple ctrl-z", "hello\x1a\x1a\x1a", "hello"},
		{"embedded ctrl-z", "line1\x1aline2", "line1line2"},
		{"no ctrl-z", "clean content", "clean content"},
		{"only ctrl-z", "\x1a", ""},
		{"mixed trailing", "hello \r\n\x1a\r\n", "hello"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanDIZ(tc.input)
			if got != tc.want {
				t.Errorf("cleanDIZ(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
