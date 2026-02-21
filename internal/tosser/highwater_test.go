package tosser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHighWaterMarkBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hwm.json")

	hwm, err := LoadHighWaterMark(path)
	if err != nil {
		t.Fatalf("LoadHighWaterMark: %v", err)
	}

	// Initially zero
	if got := hwm.Get("fsxnet", 3); got != 0 {
		t.Errorf("initial Get: got %d, want 0", got)
	}

	// Set and get
	hwm.Set("fsxnet", 3, 42)
	if got := hwm.Get("fsxnet", 3); got != 42 {
		t.Errorf("Get after Set: got %d, want 42", got)
	}

	// Different network/area are independent
	if got := hwm.Get("tqwnet", 3); got != 0 {
		t.Errorf("different network should be 0, got %d", got)
	}
	if got := hwm.Get("fsxnet", 4); got != 0 {
		t.Errorf("different area should be 0, got %d", got)
	}
}

func TestHighWaterMarkPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hwm.json")

	hwm, _ := LoadHighWaterMark(path)
	hwm.Set("fsxnet", 3, 100)
	hwm.Set("fsxnet", 5, 200)
	hwm.Set("tqwnet", 7, 50)

	if err := hwm.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Reload
	hwm2, err := LoadHighWaterMark(path)
	if err != nil {
		t.Fatalf("reload LoadHighWaterMark: %v", err)
	}

	if got := hwm2.Get("fsxnet", 3); got != 100 {
		t.Errorf("persisted fsxnet/3: got %d, want 100", got)
	}
	if got := hwm2.Get("fsxnet", 5); got != 200 {
		t.Errorf("persisted fsxnet/5: got %d, want 200", got)
	}
	if got := hwm2.Get("tqwnet", 7); got != 50 {
		t.Errorf("persisted tqwnet/7: got %d, want 50", got)
	}
}

func TestHighWaterMarkCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hwm.json")

	os.WriteFile(path, []byte("not json {{{"), 0644)

	hwm, err := LoadHighWaterMark(path)
	if err != nil {
		t.Fatalf("LoadHighWaterMark should handle corrupt file: %v", err)
	}
	if got := hwm.Get("fsxnet", 1); got != 0 {
		t.Errorf("corrupt file should result in zero marks, got %d", got)
	}
}

func TestHighWaterMarkPath(t *testing.T) {
	got := HWMPath("/opt/vision3/data")
	want := "/opt/vision3/data/ftn/export_hwm.json"
	if got != want {
		t.Errorf("HWMPath: got %q, want %q", got, want)
	}
}
