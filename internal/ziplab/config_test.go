package ziplab

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig_AllStepsEnabled(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}
	if !cfg.RunOnUpload {
		t.Error("expected RunOnUpload to be true by default")
	}
	if !cfg.Steps.TestIntegrity.Enabled {
		t.Error("expected TestIntegrity to be enabled by default")
	}
	if !cfg.Steps.ExtractToTemp.Enabled {
		t.Error("expected ExtractToTemp to be enabled by default")
	}
	if cfg.Steps.VirusScan.Enabled {
		t.Error("expected VirusScan to be disabled by default (requires external tool)")
	}
	if !cfg.Steps.RemoveAds.Enabled {
		t.Error("expected RemoveAds to be enabled by default")
	}
	if !cfg.Steps.AddComment.Enabled {
		t.Error("expected AddComment to be enabled by default")
	}
	if !cfg.Steps.IncludeFile.Enabled {
		t.Error("expected IncludeFile to be enabled by default")
	}
}

func TestDefaultConfig_ScanFailBehavior(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ScanFailBehavior != "delete" {
		t.Errorf("expected ScanFailBehavior to be 'delete', got %q", cfg.ScanFailBehavior)
	}
}

func TestDefaultConfig_HasZipArchiveType(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.ArchiveTypes) == 0 {
		t.Fatal("expected at least one archive type configured")
	}

	found := false
	for _, at := range cfg.ArchiveTypes {
		if at.Extension == ".zip" {
			found = true
			if !at.Native {
				t.Error("expected .zip to be marked as native")
			}
		}
	}
	if !found {
		t.Error("expected .zip archive type in defaults")
	}
}

func TestLoadConfig_MissingFile_ReturnsDefaults(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if !cfg.Enabled {
		t.Error("expected defaults when file missing")
	}
}

func TestLoadConfig_ValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfgData := Config{
		Enabled:          false,
		RunOnUpload:      false,
		ScanFailBehavior: "quarantine",
		QuarantinePath:   "/tmp/quarantine",
	}

	data, _ := json.MarshalIndent(cfgData, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "ziplab.json"), data, 0644)

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected Enabled to be false from JSON")
	}
	if cfg.RunOnUpload {
		t.Error("expected RunOnUpload to be false from JSON")
	}
	if cfg.ScanFailBehavior != "quarantine" {
		t.Errorf("expected ScanFailBehavior 'quarantine', got %q", cfg.ScanFailBehavior)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "ziplab.json"), []byte("{invalid json"), 0644)

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadConfig_PartialOverride(t *testing.T) {
	tmpDir := t.TempDir()
	// Only override one field — rest should remain at defaults
	partialJSON := `{"scanFailBehavior": "quarantine"}`
	os.WriteFile(filepath.Join(tmpDir, "ziplab.json"), []byte(partialJSON), 0644)

	cfg, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ScanFailBehavior != "quarantine" {
		t.Errorf("expected overridden field, got %q", cfg.ScanFailBehavior)
	}
	// Defaults should still apply for non-overridden fields
	if !cfg.Enabled {
		t.Error("expected Enabled default to remain true")
	}
}

func TestIsArchiveSupported(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		filename string
		expected bool
	}{
		{"test.zip", true},
		{"TEST.ZIP", true},
		{"file.txt", false},
		{"archive.rar", false}, // not in defaults
		{"", false},
	}

	for _, tt := range tests {
		result := cfg.IsArchiveSupported(tt.filename)
		if result != tt.expected {
			t.Errorf("IsArchiveSupported(%q) = %v, want %v", tt.filename, result, tt.expected)
		}
	}
}

func TestGetArchiveType(t *testing.T) {
	cfg := DefaultConfig()

	at, ok := cfg.GetArchiveType("test.zip")
	if !ok {
		t.Fatal("expected to find archive type for .zip")
	}
	if at.Extension != ".zip" {
		t.Errorf("expected extension .zip, got %q", at.Extension)
	}

	_, ok = cfg.GetArchiveType("test.rar")
	if ok {
		t.Error("expected no archive type for .rar in defaults")
	}
}
