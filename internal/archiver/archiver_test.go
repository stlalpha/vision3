package archiver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Archivers) == 0 {
		t.Fatal("DefaultConfig should have at least one archiver")
	}

	// ZIP should be the first and enabled/native
	zip := cfg.Archivers[0]
	if zip.ID != "zip" {
		t.Errorf("first archiver ID = %q, want %q", zip.ID, "zip")
	}
	if !zip.Enabled {
		t.Error("ZIP archiver should be enabled by default")
	}
	if !zip.Native {
		t.Error("ZIP archiver should be native by default")
	}
	if zip.Magic != "504B0304" {
		t.Errorf("ZIP magic = %q, want %q", zip.Magic, "504B0304")
	}
}

func TestEnabledArchivers(t *testing.T) {
	cfg := DefaultConfig()
	enabled := cfg.EnabledArchivers()
	// Only ZIP is enabled by default
	if len(enabled) != 1 {
		t.Errorf("EnabledArchivers count = %d, want 1", len(enabled))
	}
	if enabled[0].ID != "zip" {
		t.Errorf("only enabled archiver = %q, want %q", enabled[0].ID, "zip")
	}
}

func TestSupportedExtensions(t *testing.T) {
	cfg := DefaultConfig()
	exts := cfg.SupportedExtensions()
	if len(exts) != 1 || exts[0] != ".zip" {
		t.Errorf("SupportedExtensions = %v, want [\".zip\"]", exts)
	}

	// Enable LHA and check
	for i := range cfg.Archivers {
		if cfg.Archivers[i].ID == "lha" {
			cfg.Archivers[i].Enabled = true
		}
	}
	exts = cfg.SupportedExtensions()
	found := map[string]bool{}
	for _, e := range exts {
		found[e] = true
	}
	if !found[".zip"] || !found[".lha"] || !found[".lzh"] {
		t.Errorf("SupportedExtensions = %v, want .zip, .lha, .lzh", exts)
	}
}

func TestFindByExtension(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		filename string
		wantID   string
		wantOK   bool
	}{
		{"archive.zip", "zip", true},
		{"ARCHIVE.ZIP", "zip", true},
		{"file.rar", "", false}, // disabled by default
		{"file.txt", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		a, ok := cfg.FindByExtension(tt.filename)
		if ok != tt.wantOK {
			t.Errorf("FindByExtension(%q) ok = %v, want %v", tt.filename, ok, tt.wantOK)
		}
		if ok && a.ID != tt.wantID {
			t.Errorf("FindByExtension(%q) ID = %q, want %q", tt.filename, a.ID, tt.wantID)
		}
	}
}

func TestFindByExtension_WithEnabled(t *testing.T) {
	cfg := DefaultConfig()
	// Enable RAR
	for i := range cfg.Archivers {
		if cfg.Archivers[i].ID == "rar" {
			cfg.Archivers[i].Enabled = true
		}
	}
	a, ok := cfg.FindByExtension("file.rar")
	if !ok || a.ID != "rar" {
		t.Errorf("FindByExtension(\"file.rar\") = (%q, %v), want (\"rar\", true)", a.ID, ok)
	}
}

func TestFindByID(t *testing.T) {
	cfg := DefaultConfig()

	a, ok := cfg.FindByID("zip")
	if !ok || a.ID != "zip" {
		t.Errorf("FindByID(\"zip\") = (%q, %v), want (\"zip\", true)", a.ID, ok)
	}

	// FindByID should find disabled archivers too
	a, ok = cfg.FindByID("rar")
	if !ok || a.ID != "rar" {
		t.Errorf("FindByID(\"rar\") = (%q, %v), want (\"rar\", true)", a.ID, ok)
	}

	_, ok = cfg.FindByID("nonexistent")
	if ok {
		t.Error("FindByID(\"nonexistent\") should return false")
	}
}

func TestIsSupported(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.IsSupported("test.zip") {
		t.Error("IsSupported(\"test.zip\") should be true")
	}
	if cfg.IsSupported("test.rar") {
		t.Error("IsSupported(\"test.rar\") should be false (disabled)")
	}
}

func TestMatchesExtension(t *testing.T) {
	a := Archiver{
		ID:         "lha",
		Extension:  ".lha",
		Extensions: []string{".lzh"},
		Enabled:    true,
	}

	tests := []struct {
		filename string
		want     bool
	}{
		{"file.lha", true},
		{"file.LHA", true},
		{"file.lzh", true},
		{"file.LZH", true},
		{"file.zip", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := a.MatchesExtension(tt.filename); got != tt.want {
			t.Errorf("MatchesExtension(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("LoadConfig should not error for missing file: %v", err)
	}
	if len(cfg.Archivers) == 0 {
		t.Error("missing file should return default config with archivers")
	}
}

func TestLoadConfig_ValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Archivers: []Archiver{
			{
				ID:        "zip",
				Name:      "ZIP",
				Extension: ".zip",
				Native:    true,
				Enabled:   true,
			},
			{
				ID:        "rar",
				Name:      "RAR",
				Extension: ".rar",
				Enabled:   true,
				Unpack: CommandDef{
					Command: "/usr/bin/unrar",
					Args:    []string{"x", "{ARCHIVE}", "{OUTDIR}"},
				},
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "archivers.json"), data, 0644); err != nil {
		t.Fatalf("write archivers.json: %v", err)
	}

	loaded, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if len(loaded.Archivers) != 2 {
		t.Fatalf("loaded %d archivers, want 2", len(loaded.Archivers))
	}
	if loaded.Archivers[1].ID != "rar" || !loaded.Archivers[1].Enabled {
		t.Errorf("second archiver = %+v, want rar enabled", loaded.Archivers[1])
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "archivers.json"), []byte("{bad json}"), 0644); err != nil {
		t.Fatalf("write archivers.json: %v", err)
	}

	_, err := LoadConfig(tmpDir)
	if err == nil {
		t.Error("LoadConfig should error on invalid JSON")
	}
}

func TestCommandDefIsEmpty(t *testing.T) {
	empty := CommandDef{}
	if !empty.IsEmpty() {
		t.Error("empty CommandDef.IsEmpty() should be true")
	}

	full := CommandDef{Command: "zip", Args: []string{"-j"}}
	if full.IsEmpty() {
		t.Error("non-empty CommandDef.IsEmpty() should be false")
	}
}

func TestSaveDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	if err := SaveDefaultConfig(tmpDir); err != nil {
		t.Fatalf("SaveDefaultConfig error: %v", err)
	}

	// Verify the file was created and is valid JSON
	data, err := os.ReadFile(filepath.Join(tmpDir, "archivers.json"))
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("saved config is not valid JSON: %v", err)
	}
	if len(cfg.Archivers) == 0 {
		t.Error("saved config should have archivers")
	}
}
