package ziplab

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolveProjectRoot walks up from internal/ziplab/ to find the project root.
// It verifies that key asset files exist before returning.
func resolveProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from the directory containing this test file
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	// Walk up until we find the project root markers
	for {
		removeTxt := filepath.Join(dir, "ziplab", "REMOVE.TXT")
		nfoFile := filepath.Join(dir, "menus", "v3", "ansi", "ZIPLAB.NFO")

		_, errRemove := os.Stat(removeTxt)
		_, errNFO := os.Stat(nfoFile)

		if errRemove == nil && errNFO == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("project root not found: missing ziplab/REMOVE.TXT or menus/v3/ansi/ZIPLAB.NFO")
		}
		dir = parent
	}
}

// setupIntegrationProcessor creates a Processor configured with real asset
// file paths from the project's ziplab/ directory.
func setupIntegrationProcessor(t *testing.T, root string) *Processor {
	t.Helper()

	cfg := DefaultConfig()
	// Default config uses relative paths (REMOVE.TXT, ZCOMMENT.TXT, BBS.AD)
	// which resolvePath resolves against baseDir (the ziplab/ directory).
	return NewProcessor(cfg, filepath.Join(root, "ziplab"))
}

func TestZipLabPipeline_Integration(t *testing.T) {
	root := resolveProjectRoot(t)

	t.Run("happy_path", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "upload.zip")

		createTestZip(t, zipPath, map[string]string{
			"FILE_ID.DIZ": "Cool utility v1.0 - Does cool things",
			"VENDOR.TXT":  "Some vendor info",
			"readme.txt":  "Read me first",
			"src/main.c":  "int main() { return 0; }",
		})

		p := setupIntegrationProcessor(t, root)

		var statusCalls []struct {
			step   StepNumber
			status Status
		}
		statusFn := func(step StepNumber, status Status) {
			statusCalls = append(statusCalls, struct {
				step   StepNumber
				status Status
			}{step, status})
		}

		result := p.RunPipeline(zipPath, statusFn)

		if !result.Success {
			t.Fatalf("pipeline should succeed: %v", result.Error)
		}
		if result.Description != "Cool utility v1.0 - Does cool things" {
			t.Errorf("expected DIZ content, got %q", result.Description)
		}

		// All step results should be StatusPass
		for _, sr := range result.StepResults {
			if sr.Status != StatusPass {
				t.Errorf("step %d (%s) expected StatusPass, got %s (err: %v)",
					sr.Step, sr.Name, sr.Status, sr.Error)
			}
		}

		// 5 enabled steps (1,2,5,6,7) x 2 callbacks each (Doing + Pass) = 10
		if len(statusCalls) < 10 {
			t.Errorf("expected at least 10 status callbacks, got %d", len(statusCalls))
		}

		// Open the final ZIP and verify modifications
		r, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatalf("failed to open final zip: %v", err)
		}
		defer r.Close()

		commentData, err := os.ReadFile(filepath.Join(root, "ziplab", "ZCOMMENT.TXT"))
		if err != nil {
			t.Fatalf("failed to read ZCOMMENT.TXT: %v", err)
		}
		expectedComment := strings.TrimSpace(string(commentData))
		if r.Comment != expectedComment {
			t.Errorf("zip comment should match ZCOMMENT.TXT\nexpected: %q\ngot:      %q", expectedComment, r.Comment)
		}

		foundBBSAD := false
		foundReadme := false
		foundDIZ := false
		foundVendor := false
		for _, f := range r.File {
			switch {
			case f.Name == "BBS.AD":
				foundBBSAD = true
			case strings.EqualFold(f.Name, "readme.txt"):
				foundReadme = true
			case strings.EqualFold(f.Name, "FILE_ID.DIZ"):
				foundDIZ = true
			case strings.EqualFold(f.Name, "VENDOR.TXT"):
				foundVendor = true
			}
		}
		if !foundBBSAD {
			t.Error("BBS.AD should be present in final zip")
		}
		if !foundReadme {
			t.Error("readme.txt should be present in final zip")
		}
		if !foundDIZ {
			t.Error("FILE_ID.DIZ should be present in final zip")
		}
		if foundVendor {
			t.Error("VENDOR.TXT should have been removed by ad removal step")
		}

		t.Run("display_pipeline", func(t *testing.T) {
			dispZipPath := filepath.Join(tmpDir, "display_upload.zip")
			createTestZip(t, dispZipPath, map[string]string{
				"FILE_ID.DIZ": "Display test file",
				"readme.txt":  "readme content",
			})

			dispProcessor := setupIntegrationProcessor(t, root)

			nfoPath := filepath.Join(root, "menus", "v3", "ansi", "ZIPLAB.NFO")
			nfo, err := ParseNFO(nfoPath)
			if err != nil {
				t.Fatalf("failed to parse ZIPLAB.NFO: %v", err)
			}

			ansPath := filepath.Join(root, "menus", "v3", "ansi", "ZIPLAB.ANS")
			ansiContent, err := os.ReadFile(ansPath)
			if err != nil {
				t.Fatalf("failed to read ZIPLAB.ANS: %v", err)
			}

			var buf bytes.Buffer
			dispResult := dispProcessor.DisplayPipeline(&buf, nfo, ansiContent, dispZipPath)
			if !dispResult.Success {
				t.Fatalf("display pipeline should succeed: %v", dispResult.Error)
			}

			output := buf.Bytes()
			// Output should contain the ANSI screen content
			if !bytes.Contains(output, ansiContent) {
				t.Error("output should contain ANSI screen content")
			}
			// Output should contain cursor positioning sequences (ESC[row;colH)
			if !bytes.Contains(output, []byte("\x1b[")) {
				t.Error("output should contain ESC[ cursor sequences")
			}
		})
	})

	t.Run("corrupt_zip", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "corrupt.zip")

		if err := os.WriteFile(zipPath, []byte("this is definitely not a zip file"), 0644); err != nil {
			t.Fatalf("failed to write corrupt zip: %v", err)
		}

		p := setupIntegrationProcessor(t, root)

		result := p.RunPipeline(zipPath, nil)

		if result.Success {
			t.Fatal("pipeline should fail for corrupt zip")
		}
		if len(result.StepResults) != 1 {
			t.Errorf("expected exactly 1 step result, got %d", len(result.StepResults))
		}
		if result.StepResults[0].Step != StepIntegrity {
			t.Errorf("failed step should be StepIntegrity, got %d", result.StepResults[0].Step)
		}
		if result.StepResults[0].Status != StatusFail {
			t.Errorf("step status should be StatusFail, got %s", result.StepResults[0].Status)
		}
		if result.Description != "" {
			t.Errorf("expected empty description, got %q", result.Description)
		}
	})

	t.Run("no_diz", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "nodiz.zip")

		createTestZip(t, zipPath, map[string]string{
			"readme.txt":  "Just a readme",
			"program.exe": "MZ fake exe content",
		})

		p := setupIntegrationProcessor(t, root)

		result := p.RunPipeline(zipPath, nil)

		if !result.Success {
			t.Fatalf("pipeline should succeed without DIZ: %v", result.Error)
		}
		if result.Description != "" {
			t.Errorf("expected empty description, got %q", result.Description)
		}

		// Open final ZIP and verify comment + BBS.AD
		r, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatalf("failed to open final zip: %v", err)
		}
		defer r.Close()

		// Comment should match ZCOMMENT.TXT content
		commentData, err := os.ReadFile(filepath.Join(root, "ziplab", "ZCOMMENT.TXT"))
		if err != nil {
			t.Fatalf("failed to read ZCOMMENT.TXT: %v", err)
		}
		expectedComment := strings.TrimSpace(string(commentData))
		if r.Comment != expectedComment {
			t.Errorf("zip comment should match ZCOMMENT.TXT content\nexpected: %q\ngot:      %q",
				expectedComment, r.Comment)
		}

		foundBBSAD := false
		for _, f := range r.File {
			if f.Name == "BBS.AD" {
				foundBBSAD = true
				rc, err := f.Open()
				if err != nil {
					t.Fatalf("failed to open BBS.AD entry: %v", err)
				}
				data, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					t.Fatalf("failed to read BBS.AD entry: %v", err)
				}
				if len(data) == 0 {
					t.Error("BBS.AD should have content")
				}
			}
		}
		if !foundBBSAD {
			t.Error("BBS.AD should be present in final zip")
		}
	})
}
