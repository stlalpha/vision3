package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStrings(t *testing.T) {
	dir := t.TempDir()
	want := StringsConfig{ConnectionStr: "Welcome"}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "strings.json"), data, 0644); err != nil {
		t.Fatalf("write strings: %v", err)
	}

	got, err := LoadStrings(dir)
	if err != nil {
		t.Fatalf("LoadStrings error: %v", err)
	}
	if got.ConnectionStr != want.ConnectionStr {
		t.Fatalf("got ConnectionStr %q, want %q", got.ConnectionStr, want.ConnectionStr)
	}
}

func TestLoadStringsMissing(t *testing.T) {
	if _, err := LoadStrings(t.TempDir()); err == nil {
		t.Fatalf("expected error when strings.json missing")
	}
}

func TestLoadDoors(t *testing.T) {
	dir := t.TempDir()
	doors := []DoorConfig{{Name: "door1", Command: "run"}}
	data, err := json.Marshal(doors)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, "doors.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write doors: %v", err)
	}

	loaded, err := LoadDoors(path)
	if err != nil {
		t.Fatalf("LoadDoors error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 door, got %d", len(loaded))
	}
	if loaded["door1"].Command != "run" {
		t.Fatalf("unexpected command: %q", loaded["door1"].Command)
	}
}

func TestLoadDoorsDuplicate(t *testing.T) {
	dir := t.TempDir()
	doors := []DoorConfig{{Name: "door1"}, {Name: "door1"}}
	data, _ := json.Marshal(doors)
	path := filepath.Join(dir, "doors.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write doors: %v", err)
	}
	if _, err := LoadDoors(path); err == nil {
		t.Fatalf("expected duplicate door error")
	}
}

func TestLoadDoorsMissingFile(t *testing.T) {
	loaded, err := LoadDoors(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("unexpected error for missing doors file: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty map for missing doors file")
	}
}

func TestLoadOneLiners(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oneliners.dat")
	content := "first\nsecond\n" // newline at end should be ignored
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write oneliners: %v", err)
	}
	got, err := LoadOneLiners(dir)
	if err != nil {
		t.Fatalf("LoadOneLiners error: %v", err)
	}
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("unexpected oneliners: %v", got)
	}
}

func TestLoadThemeConfigDefaults(t *testing.T) {
	theme, err := LoadThemeConfig(t.TempDir())
	if err != nil {
		t.Fatalf("LoadThemeConfig error: %v", err)
	}
	if theme.YesNoHighlightColor != 112 || theme.YesNoRegularColor != 15 {
		t.Fatalf("expected default theme, got %+v", theme)
	}
}

func TestLoadThemeConfigOverride(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"yesNoHighlightColor": 1, "yesNoRegularColor": 2}`)
	if err := os.WriteFile(filepath.Join(dir, "theme.json"), data, 0644); err != nil {
		t.Fatalf("write theme: %v", err)
	}
	theme, err := LoadThemeConfig(dir)
	if err != nil {
		t.Fatalf("LoadThemeConfig error: %v", err)
	}
	if theme.YesNoHighlightColor != 1 || theme.YesNoRegularColor != 2 {
		t.Fatalf("expected overrides, got %+v", theme)
	}
}

func TestSaveAndLoadSystemConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := SystemConfig{BoardName: "Test", BoardPhoneNumber: "555", SysOpLevel: 255}
	if err := SaveSystemConfig(dir, cfg); err != nil {
		t.Fatalf("SaveSystemConfig error: %v", err)
	}
	got, err := LoadSystemConfig(dir)
	if err != nil {
		t.Fatalf("LoadSystemConfig error: %v", err)
	}
	if got.BoardName != cfg.BoardName || got.SysOpLevel != cfg.SysOpLevel {
		t.Fatalf("expected %+v, got %+v", cfg, got)
	}
}
