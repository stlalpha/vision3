package jam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesNewBase(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "test")

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer b.Close()

	if !b.IsOpen() {
		t.Fatal("base should be open")
	}

	// Verify all four files exist
	for _, ext := range []string{".jhr", ".jdt", ".jdx", ".jlr"} {
		if _, err := os.Stat(basePath + ext); err != nil {
			t.Errorf("missing file %s: %v", ext, err)
		}
	}

	// .jhr should be exactly 1024 bytes (fixed header)
	info, _ := os.Stat(basePath + ".jhr")
	if info.Size() != HeaderSize {
		t.Errorf(".jhr size = %d, want %d", info.Size(), HeaderSize)
	}

	// Empty base should have 0 messages
	count, err := b.GetMessageCount()
	if err != nil {
		t.Fatalf("GetMessageCount: %v", err)
	}
	if count != 0 {
		t.Errorf("message count = %d, want 0", count)
	}
}

func TestOpenExistingBase(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "test")

	// Create base
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	b.Close()

	// Reopen
	b, err = Open(basePath)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	defer b.Close()

	if !b.IsOpen() {
		t.Fatal("base should be open after reopen")
	}
}

func TestOpenRecreatesCorruptedBase(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "test")

	// Create a tiny .jhr (corrupted)
	os.WriteFile(basePath+".jhr", []byte("bad"), 0644)

	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open with corrupt base failed: %v", err)
	}
	defer b.Close()

	// Should have recreated with valid header
	info, _ := os.Stat(basePath + ".jhr")
	if info.Size() != HeaderSize {
		t.Errorf(".jhr size = %d after recreation, want %d", info.Size(), HeaderSize)
	}
}

func TestGetNextMsgSerial(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "test")
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer b.Close()

	s1, err := b.GetNextMsgSerial()
	if err != nil {
		t.Fatalf("GetNextMsgSerial: %v", err)
	}
	if s1 == 0 {
		t.Error("first serial should not be 0")
	}

	s2, err := b.GetNextMsgSerial()
	if err != nil {
		t.Fatalf("GetNextMsgSerial: %v", err)
	}
	if s2 != s1+1 {
		t.Errorf("second serial = %d, want %d", s2, s1+1)
	}
}

func TestCloseAndIsOpen(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "test")
	b, err := Open(basePath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if !b.IsOpen() {
		t.Fatal("should be open before close")
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if b.IsOpen() {
		t.Fatal("should not be open after close")
	}
}
