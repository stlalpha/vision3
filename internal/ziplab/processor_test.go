package ziplab

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// createTestZip creates a valid ZIP file at the given path with the specified files.
func createTestZip(t *testing.T, zipPath string, files map[string]string) {
	t.Helper()
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to add file %s to zip: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write content for %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
}

// --- Step 1: Test Integrity ---

func TestStepTestIntegrity_ValidZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{"hello.txt": "hello world"})

	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	err := p.StepTestIntegrity(zipPath)
	if err != nil {
		t.Fatalf("valid zip should pass integrity test: %v", err)
	}
}

func TestStepTestIntegrity_CorruptZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "corrupt.zip")
	os.WriteFile(zipPath, []byte("this is not a zip file"), 0644)

	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	err := p.StepTestIntegrity(zipPath)
	if err == nil {
		t.Fatal("corrupt file should fail integrity test")
	}
}

func TestStepTestIntegrity_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	err := p.StepTestIntegrity(filepath.Join(tmpDir, "nonexistent.zip"))
	if err == nil {
		t.Fatal("missing file should fail integrity test")
	}
}

func TestStepTestIntegrity_StepDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Steps.TestIntegrity.Enabled = false
	p := NewProcessor(cfg, tmpDir)

	// Should succeed (skip) even with a nonexistent file
	err := p.StepTestIntegrity(filepath.Join(tmpDir, "nonexistent.zip"))
	if err != nil {
		t.Fatalf("disabled step should be skipped: %v", err)
	}
}

// --- Step 2: Extract to Temp ---

func TestStepExtract_ValidZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"hello.txt":   "hello world",
		"sub/deep.txt": "deep content",
	})

	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	workDir, err := p.StepExtract(zipPath)
	if err != nil {
		t.Fatalf("valid zip should extract: %v", err)
	}

	// Check extracted files exist
	content, err := os.ReadFile(filepath.Join(workDir, "hello.txt"))
	if err != nil {
		t.Fatalf("expected hello.txt in work dir: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(content))
	}

	// Check subdirectory extraction
	content, err = os.ReadFile(filepath.Join(workDir, "sub", "deep.txt"))
	if err != nil {
		t.Fatalf("expected sub/deep.txt in work dir: %v", err)
	}
	if string(content) != "deep content" {
		t.Errorf("expected 'deep content', got %q", string(content))
	}
}

func TestStepExtract_CorruptZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "corrupt.zip")
	os.WriteFile(zipPath, []byte("not a zip"), 0644)

	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	_, err := p.StepExtract(zipPath)
	if err == nil {
		t.Fatal("corrupt zip should fail extraction")
	}
}

func TestStepExtract_StepDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Steps.ExtractToTemp.Enabled = false
	p := NewProcessor(cfg, tmpDir)

	workDir, err := p.StepExtract(filepath.Join(tmpDir, "nonexistent.zip"))
	if err != nil {
		t.Fatalf("disabled step should be skipped: %v", err)
	}
	if workDir != "" {
		t.Errorf("disabled step should return empty workDir, got %q", workDir)
	}
}

// --- Step 3: Virus Scan ---

func TestStepVirusScan_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig() // virus scan disabled by default
	p := NewProcessor(cfg, tmpDir)

	err := p.StepVirusScan(tmpDir)
	if err != nil {
		t.Fatalf("disabled virus scan should be skipped: %v", err)
	}
}

// --- Step 5: Extract FILE_ID.DIZ and Remove Ads ---

func TestStepRemoveAdsAndDIZ_ExtractsDIZ(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	// Create a FILE_ID.DIZ in the work directory
	os.WriteFile(filepath.Join(workDir, "FILE_ID.DIZ"), []byte("Test file description"), 0644)

	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	diz, err := p.StepRemoveAdsAndDIZ(workDir)
	if err != nil {
		t.Fatalf("should extract DIZ: %v", err)
	}
	if diz != "Test file description" {
		t.Errorf("expected 'Test file description', got %q", diz)
	}
}

func TestStepRemoveAdsAndDIZ_CaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	// Lowercase filename
	os.WriteFile(filepath.Join(workDir, "file_id.diz"), []byte("lowercase diz"), 0644)

	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	diz, err := p.StepRemoveAdsAndDIZ(workDir)
	if err != nil {
		t.Fatalf("should handle case insensitive DIZ: %v", err)
	}
	if diz != "lowercase diz" {
		t.Errorf("expected 'lowercase diz', got %q", diz)
	}
}

func TestStepRemoveAdsAndDIZ_RemovesPatternFiles(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	// Create files that should be removed
	os.WriteFile(filepath.Join(workDir, "BBS.AD"), []byte("some ad"), 0644)
	os.WriteFile(filepath.Join(workDir, "VENDOR.TXT"), []byte("vendor"), 0644)
	os.WriteFile(filepath.Join(workDir, "keepme.txt"), []byte("keep"), 0644)

	// Create a REMOVE.TXT patterns file
	patternsFile := filepath.Join(tmpDir, "REMOVE.TXT")
	os.WriteFile(patternsFile, []byte("BBS.AD\nVENDOR.TXT\n"), 0644)

	cfg := DefaultConfig()
	cfg.Steps.RemoveAds.PatternsFile = patternsFile
	p := NewProcessor(cfg, tmpDir)

	_, err := p.StepRemoveAdsAndDIZ(workDir)
	if err != nil {
		t.Fatalf("should remove ad files: %v", err)
	}

	// BBS.AD and VENDOR.TXT should be gone
	if _, err := os.Stat(filepath.Join(workDir, "BBS.AD")); !os.IsNotExist(err) {
		t.Error("BBS.AD should have been removed")
	}
	if _, err := os.Stat(filepath.Join(workDir, "VENDOR.TXT")); !os.IsNotExist(err) {
		t.Error("VENDOR.TXT should have been removed")
	}
	// keepme.txt should remain
	if _, err := os.Stat(filepath.Join(workDir, "keepme.txt")); err != nil {
		t.Error("keepme.txt should not have been removed")
	}
}

func TestStepRemoveAdsAndDIZ_NoDIZ(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	// No FILE_ID.DIZ present
	os.WriteFile(filepath.Join(workDir, "readme.txt"), []byte("readme"), 0644)

	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	diz, err := p.StepRemoveAdsAndDIZ(workDir)
	if err != nil {
		t.Fatalf("should succeed even without DIZ: %v", err)
	}
	if diz != "" {
		t.Errorf("expected empty DIZ, got %q", diz)
	}
}

func TestStepRemoveAdsAndDIZ_StepDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Steps.RemoveAds.Enabled = false
	p := NewProcessor(cfg, tmpDir)

	diz, err := p.StepRemoveAdsAndDIZ(filepath.Join(tmpDir, "nonexistent"))
	if err != nil {
		t.Fatalf("disabled step should be skipped: %v", err)
	}
	if diz != "" {
		t.Errorf("disabled step should return empty DIZ, got %q", diz)
	}
}

// --- Step 6: Add Comment ---

func TestStepAddComment_NativeZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{"hello.txt": "hello"})

	commentFile := filepath.Join(tmpDir, "ZCOMMENT.TXT")
	os.WriteFile(commentFile, []byte("Downloaded from TestBBS"), 0644)

	cfg := DefaultConfig()
	cfg.Steps.AddComment.CommentFile = commentFile
	p := NewProcessor(cfg, tmpDir)

	err := p.StepAddComment(zipPath)
	if err != nil {
		t.Fatalf("should add comment to zip: %v", err)
	}

	// Verify the comment was added
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open zip after comment: %v", err)
	}
	defer r.Close()
	if r.Comment != "Downloaded from TestBBS" {
		t.Errorf("expected comment 'Downloaded from TestBBS', got %q", r.Comment)
	}
}

func TestStepAddComment_StepDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Steps.AddComment.Enabled = false
	p := NewProcessor(cfg, tmpDir)

	err := p.StepAddComment(filepath.Join(tmpDir, "nonexistent.zip"))
	if err != nil {
		t.Fatalf("disabled step should be skipped: %v", err)
	}
}

// --- Step 7: Include File ---

func TestStepIncludeFile_NativeZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{"hello.txt": "hello"})

	adFile := filepath.Join(tmpDir, "BBS.AD")
	os.WriteFile(adFile, []byte("Visit our BBS!"), 0644)

	cfg := DefaultConfig()
	cfg.Steps.IncludeFile.FilePath = adFile
	p := NewProcessor(cfg, tmpDir)

	err := p.StepIncludeFile(zipPath)
	if err != nil {
		t.Fatalf("should include file in zip: %v", err)
	}

	// Verify BBS.AD was added to the zip
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer r.Close()

	found := false
	for _, f := range r.File {
		if f.Name == "BBS.AD" {
			found = true
			rc, _ := f.Open()
			buf := make([]byte, 100)
			n, _ := rc.Read(buf)
			rc.Close()
			if string(buf[:n]) != "Visit our BBS!" {
				t.Errorf("expected 'Visit our BBS!', got %q", string(buf[:n]))
			}
		}
	}
	if !found {
		t.Error("BBS.AD not found in zip after include")
	}
}

func TestStepIncludeFile_StepDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Steps.IncludeFile.Enabled = false
	p := NewProcessor(cfg, tmpDir)

	err := p.StepIncludeFile(filepath.Join(tmpDir, "nonexistent.zip"))
	if err != nil {
		t.Fatalf("disabled step should be skipped: %v", err)
	}
}

// --- Processor ---

func TestNewProcessor(t *testing.T) {
	cfg := DefaultConfig()
	p := NewProcessor(cfg, "/tmp/test")

	if p == nil {
		t.Fatal("NewProcessor should return non-nil")
	}
	if p.config.Enabled != true {
		t.Error("processor should use provided config")
	}
}
