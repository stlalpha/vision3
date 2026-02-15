# ZipLab Integration Test Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Write an end-to-end integration test that runs the full ZipLab pipeline against real project assets and verifies the final ZIP binary state.

**Architecture:** Single test file with a shared setup helper and 3 `t.Run()` subtests (happy path, corrupt ZIP, no DIZ). Uses real template files from `ziplab/` and ANSI assets from `menus/v3/ansi/`. Reuses the existing `createTestZip` helper from `processor_test.go`.

**Tech Stack:** Go `testing`, `archive/zip`, `bytes`, `os`, `path/filepath`

---

### Task 1: Write the integration test file

**Files:**
- Create: `internal/ziplab/integration_test.go`

**Step 1: Write the full test file**

```go
package ziplab

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolveProjectRoot returns the project root by walking up from the test file.
// Returns empty string if markers not found.
func resolveProjectRoot(t *testing.T) string {
	t.Helper()
	// Tests run from internal/ziplab/, so project root is ../..
	root := filepath.Join("..", "..")

	// Verify by checking for known project files
	if _, err := os.Stat(filepath.Join(root, "ziplab", "REMOVE.TXT")); err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(root, "menus", "v3", "ansi", "ZIPLAB.NFO")); err != nil {
		return ""
	}
	return root
}

// setupIntegrationProcessor creates a Processor configured with real project assets.
func setupIntegrationProcessor(t *testing.T, root string) *Processor {
	t.Helper()

	cfg := DefaultConfig()
	cfg.Steps.RemoveAds.PatternsFile = filepath.Join(root, "ziplab", "REMOVE.TXT")
	cfg.Steps.AddComment.CommentFile = filepath.Join(root, "ziplab", "ZCOMMENT.TXT")
	cfg.Steps.IncludeFile.FilePath = filepath.Join(root, "ziplab", "BBS.AD")

	return NewProcessor(cfg, root)
}

func TestZipLabPipeline_Integration(t *testing.T) {
	root := resolveProjectRoot(t)
	if root == "" {
		t.Skip("real project assets not found, skipping integration test")
	}

	t.Run("happy_path", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "testfile.zip")
		createTestZip(t, zipPath, map[string]string{
			"readme.txt":   "This is a test archive.",
			"FILE_ID.DIZ":  "Cool utility v1.0 - Does cool things",
			"VENDOR.TXT":   "Some vendor info that should be flagged",
			"src/main.c":   "int main() { return 0; }",
		})

		proc := setupIntegrationProcessor(t, root)

		// Track status callbacks
		var callbacks []struct {
			step   StepNumber
			status Status
		}
		statusFn := func(step StepNumber, status Status) {
			callbacks = append(callbacks, struct {
				step   StepNumber
				status Status
			}{step, status})
		}

		result := proc.RunPipeline(zipPath, statusFn)

		// Verify pipeline succeeded
		if !result.Success {
			t.Fatalf("pipeline should succeed: %v", result.Error)
		}

		// Verify FILE_ID.DIZ was extracted
		if result.Description != "Cool utility v1.0 - Does cool things" {
			t.Errorf("expected DIZ content, got %q", result.Description)
		}

		// Verify all steps passed
		for _, sr := range result.StepResults {
			if sr.Status != StatusPass {
				t.Errorf("step %d (%s) should pass, got %s: %v", sr.Step, sr.Name, sr.Status, sr.Error)
			}
		}

		// Verify status callbacks fired in correct order (Doing then Pass for each step)
		if len(callbacks) < 10 {
			t.Errorf("expected at least 10 callbacks (5 steps x 2), got %d", len(callbacks))
		}

		// Verify final ZIP binary
		r, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatalf("failed to open final zip: %v", err)
		}
		defer r.Close()

		// Check comment was set (from ZCOMMENT.TXT)
		if r.Comment == "" {
			t.Error("zip comment should be set from ZCOMMENT.TXT")
		}

		// Check BBS.AD was added
		fileNames := make(map[string]bool)
		for _, f := range r.File {
			fileNames[f.Name] = true
		}

		if !fileNames["BBS.AD"] {
			t.Error("BBS.AD should have been added to the zip")
		}
		if !fileNames["readme.txt"] {
			t.Error("readme.txt should still be in the zip")
		}
		if !fileNames["FILE_ID.DIZ"] {
			t.Error("FILE_ID.DIZ should still be in the zip (not removed)")
		}
		if !fileNames["src/main.c"] {
			t.Error("src/main.c should still be in the zip")
		}

		// Test DisplayPipeline path with real ANSI assets
		t.Run("display_pipeline", func(t *testing.T) {
			dispZipPath := filepath.Join(tmpDir, "display_test.zip")
			createTestZip(t, dispZipPath, map[string]string{
				"hello.txt": "hello",
			})

			nfoPath := filepath.Join(root, "menus", "v3", "ansi", "ZIPLAB.NFO")
			nfo, err := ParseNFO(nfoPath)
			if err != nil {
				t.Fatalf("failed to parse real ZIPLAB.NFO: %v", err)
			}

			ansiPath := filepath.Join(root, "menus", "v3", "ansi", "ZIPLAB.ANS")
			ansiContent, err := os.ReadFile(ansiPath)
			if err != nil {
				t.Fatalf("failed to read real ZIPLAB.ANS: %v", err)
			}

			dispProc := setupIntegrationProcessor(t, root)
			var buf bytes.Buffer
			dispResult := dispProc.DisplayPipeline(&buf, nfo, ansiContent, dispZipPath)

			if !dispResult.Success {
				t.Fatalf("display pipeline should succeed: %v", dispResult.Error)
			}

			output := buf.Bytes()
			// Should contain the ANSI screen
			if !bytes.Contains(output, ansiContent[:64]) {
				t.Error("output should contain ZIPLAB.ANS content")
			}
			// Should contain ESC[ cursor positioning sequences from NFO updates
			if !bytes.Contains(output, []byte("\x1b[")) {
				t.Error("output should contain ANSI escape sequences from status updates")
			}
		})
	})

	t.Run("corrupt_zip", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "corrupt.zip")
		os.WriteFile(zipPath, []byte("this is definitely not a zip file"), 0644)

		proc := setupIntegrationProcessor(t, root)
		result := proc.RunPipeline(zipPath, nil)

		if result.Success {
			t.Fatal("pipeline should fail for corrupt zip")
		}

		if len(result.StepResults) != 1 {
			t.Errorf("should stop after integrity check, got %d step results", len(result.StepResults))
		}

		if result.StepResults[0].Step != StepIntegrity {
			t.Errorf("failed step should be integrity (1), got %d", result.StepResults[0].Step)
		}

		if result.StepResults[0].Status != StatusFail {
			t.Errorf("integrity step should be Fail, got %s", result.StepResults[0].Status)
		}

		if result.Description != "" {
			t.Errorf("should have no description, got %q", result.Description)
		}
	})

	t.Run("no_diz", func(t *testing.T) {
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "nodiz.zip")
		createTestZip(t, zipPath, map[string]string{
			"readme.txt": "Just a readme, no DIZ here",
			"program.exe": "fake executable bytes",
		})

		proc := setupIntegrationProcessor(t, root)
		result := proc.RunPipeline(zipPath, nil)

		if !result.Success {
			t.Fatalf("pipeline should succeed even without DIZ: %v", result.Error)
		}

		// No DIZ means empty description — caller falls through to manual prompt
		if result.Description != "" {
			t.Errorf("expected empty description without DIZ, got %q", result.Description)
		}

		// Verify ZIP was still modified (comment + BBS.AD)
		r, err := zip.OpenReader(zipPath)
		if err != nil {
			t.Fatalf("failed to open final zip: %v", err)
		}
		defer r.Close()

		if r.Comment == "" {
			t.Error("zip comment should still be set even without DIZ")
		}

		hasAd := false
		for _, f := range r.File {
			if f.Name == "BBS.AD" {
				hasAd = true
			}
		}
		if !hasAd {
			t.Error("BBS.AD should still be included even without DIZ")
		}

		// Verify comment contains text from ZCOMMENT.TXT
		commentContent, _ := os.ReadFile(filepath.Join(root, "ziplab", "ZCOMMENT.TXT"))
		expectedComment := strings.TrimSpace(string(commentContent))
		if r.Comment != expectedComment {
			t.Errorf("comment should match ZCOMMENT.TXT content\ngot:  %q\nwant: %q", r.Comment, expectedComment)
		}
	})
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/ziplab/ -run TestZipLabPipeline_Integration -v -count=1`
Expected: All 3 subtests (+ nested display_pipeline) PASS

**Step 3: Run full test suite to check no regressions**

Run: `go test ./... -count=1`
Expected: All packages PASS

**Step 4: Commit**

```bash
git add internal/ziplab/integration_test.go
git commit -m "test: ZipLab end-to-end integration test with real assets (VIS-49)"
```

---

### Task 2: Close out

**Step 1: Update Linear** — Mark VIS-49 as Done
**Step 2: Close bead** — `bd close vision3-b4o`
**Step 3: Sync and push** — `bd sync && git push`
