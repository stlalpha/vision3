package ziplab

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunPipeline_ValidZip_AllStepsPass(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"hello.txt":   "hello world",
		"FILE_ID.DIZ": "Test archive for BBS",
	})

	// Create comment and include files
	commentFile := filepath.Join(tmpDir, "ZCOMMENT.TXT")
	os.WriteFile(commentFile, []byte("BBS Comment"), 0644)
	adFile := filepath.Join(tmpDir, "BBS.AD")
	os.WriteFile(adFile, []byte("Visit our BBS"), 0644)

	cfg := DefaultConfig()
	cfg.Steps.AddComment.CommentFile = commentFile
	cfg.Steps.IncludeFile.FilePath = adFile
	// No patterns file for remove ads — just DIZ extraction
	cfg.Steps.RemoveAds.PatternsFile = ""
	p := NewProcessor(cfg, tmpDir)

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
	if result.Description != "Test archive for BBS" {
		t.Errorf("expected DIZ description, got %q", result.Description)
	}

	// Should have called status for each enabled step (D then P)
	if len(statusCalls) < 10 { // 5 steps * 2 calls each (Doing + Pass)
		t.Errorf("expected at least 10 status callbacks, got %d", len(statusCalls))
	}

	// Verify step order: 1, 2, 5, 6, 7 (3 disabled)
	expectedSteps := []StepNumber{StepIntegrity, StepExtract, StepRemoveAds, StepAddComment, StepInclude}
	stepIdx := 0
	for _, sc := range statusCalls {
		if sc.status == StatusDoing {
			if stepIdx >= len(expectedSteps) {
				t.Errorf("unexpected extra step doing: %d", sc.step)
				continue
			}
			if sc.step != expectedSteps[stepIdx] {
				t.Errorf("step %d: expected step %d, got %d", stepIdx, expectedSteps[stepIdx], sc.step)
			}
			stepIdx++
		}
	}
}

func TestRunPipeline_CorruptZip_FailsAtIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "corrupt.zip")
	os.WriteFile(zipPath, []byte("not a zip"), 0644)

	cfg := DefaultConfig()
	p := NewProcessor(cfg, tmpDir)

	result := p.RunPipeline(zipPath, nil)
	if result.Success {
		t.Fatal("pipeline should fail for corrupt zip")
	}
	if len(result.StepResults) != 1 {
		t.Errorf("should stop after first step, got %d results", len(result.StepResults))
	}
	if result.StepResults[0].Status != StatusFail {
		t.Errorf("step 1 should be Fail, got %s", result.StepResults[0].Status)
	}
}

func TestRunPipeline_NilStatusCallback(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{"hello.txt": "hello"})

	cfg := DefaultConfig()
	cfg.Steps.AddComment.Enabled = false
	cfg.Steps.IncludeFile.Enabled = false
	cfg.Steps.RemoveAds.Enabled = false
	p := NewProcessor(cfg, tmpDir)

	result := p.RunPipeline(zipPath, nil)
	if !result.Success {
		t.Fatalf("should succeed with nil callback: %v", result.Error)
	}
}

func TestRunPipeline_StepTimingRecorded(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{"hello.txt": "hello"})

	cfg := DefaultConfig()
	cfg.Steps.AddComment.Enabled = false
	cfg.Steps.IncludeFile.Enabled = false
	cfg.Steps.RemoveAds.Enabled = false
	p := NewProcessor(cfg, tmpDir)

	result := p.RunPipeline(zipPath, nil)
	for _, sr := range result.StepResults {
		if sr.Elapsed <= 0 {
			t.Errorf("step %d should have positive elapsed time", sr.Step)
		}
	}
}

func TestDisplayPipeline_WritesANSIAndStatus(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, zipPath, map[string]string{"hello.txt": "hello"})

	cfg := DefaultConfig()
	cfg.Steps.AddComment.Enabled = false
	cfg.Steps.IncludeFile.Enabled = false
	cfg.Steps.RemoveAds.Enabled = false
	p := NewProcessor(cfg, tmpDir)

	nfo := &NFOConfig{
		Entries: map[string]NFOEntry{
			"1D": {Step: 1, Status: StatusDoing, Col: 45, Row: 10, NormalColor: 112, HiColor: 116, DisplayChars: "███"},
			"1P": {Step: 1, Status: StatusPass, Col: 58, Row: 10, NormalColor: 112, HiColor: 114, DisplayChars: "███"},
			"2D": {Step: 2, Status: StatusDoing, Col: 45, Row: 11, NormalColor: 112, HiColor: 116, DisplayChars: "███"},
			"2P": {Step: 2, Status: StatusPass, Col: 58, Row: 11, NormalColor: 112, HiColor: 114, DisplayChars: "███"},
		},
	}

	ansiContent := []byte("ZIPLAB SCREEN CONTENT")
	var buf bytes.Buffer

	result := p.DisplayPipeline(&buf, nfo, ansiContent, zipPath)
	if !result.Success {
		t.Fatalf("display pipeline should succeed: %v", result.Error)
	}

	output := buf.String()
	// Should contain the ANSI screen content
	if !bytes.Contains(buf.Bytes(), ansiContent) {
		t.Error("output should contain ANSI screen content")
	}
	// Should contain cursor positioning escape sequences
	if len(output) <= len(ansiContent) {
		t.Error("output should contain status update sequences beyond the ANSI content")
	}
}

func TestHandleScanFailure_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "infected.zip")
	os.WriteFile(testFile, []byte("infected"), 0644)

	cfg := DefaultConfig()
	cfg.ScanFailBehavior = "delete"
	p := NewProcessor(cfg, tmpDir)

	p.handleScanFailure(testFile)

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("infected file should have been deleted")
	}
}

func TestHandleScanFailure_Quarantine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "infected.zip")
	os.WriteFile(testFile, []byte("infected"), 0644)

	quarantineDir := filepath.Join(tmpDir, "quarantine")

	cfg := DefaultConfig()
	cfg.ScanFailBehavior = "quarantine"
	cfg.QuarantinePath = quarantineDir
	p := NewProcessor(cfg, tmpDir)

	p.handleScanFailure(testFile)

	// Original should be gone
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("infected file should have been moved from original location")
	}
	// Should be in quarantine
	quarantinedFile := filepath.Join(quarantineDir, "infected.zip")
	if _, err := os.Stat(quarantinedFile); err != nil {
		t.Errorf("file should be in quarantine: %v", err)
	}
}
