package ftn

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBundleExtension(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"0003007b.mo0", true},
		{"0003007b.TU0", true},
		{"0003007b.we0", true},
		{"0003007b.th0", true},
		{"0003007b.fr0", true},
		{"0003007b.sa0", true},
		{"0003007b.su0", true},
		{"0003007b.mo1", true},
		{"00000001.fra", true},
		{"00000001.saa", true},
		{"00000001.moa", true},
		{"00000001.saz", true},
		{"00000001.SAZ", true},
		{"0003007b.out", true},
		{"0003007b.zip", true},
		{"message.pkt", false},
		{"0003007b.flo", false},
		{"0003007b.try", false},
		{"somefile.txt", false},
	}
	for _, tc := range cases {
		got := BundleExtension(tc.name)
		if got != tc.want {
			t.Errorf("BundleExtension(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestBundleFileName(t *testing.T) {
	// destNet=3 (0x0003), destNode=123 (0x007b), Monday (0)
	got := BundleFileName(3, 123, 0)
	want := "0003007b.mo0"
	if got != want {
		t.Errorf("BundleFileName(3, 123, 0) = %q, want %q", got, want)
	}

	// destNet=4 (0x0004), destNode=158 (0x009e), Wednesday (2)
	got = BundleFileName(4, 158, 2)
	want = "0004009e.we0"
	if got != want {
		t.Errorf("BundleFileName(4, 158, 2) = %q, want %q", got, want)
	}
}

// makeTestBundle creates a ZIP bundle in dir containing a .PKT file with the given content.
func makeTestBundle(t *testing.T, dir, bundleName, pktName string, pktData []byte) string {
	t.Helper()
	bundlePath := filepath.Join(dir, bundleName)
	f, err := os.Create(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create(pktName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(pktData); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	f.Close()
	return bundlePath
}

func TestIsZIPBundle(t *testing.T) {
	dir := t.TempDir()

	// ZIP bundle
	bundlePath := makeTestBundle(t, dir, "test.mo0", "msg.pkt", []byte("pkt data"))
	ok, err := IsZIPBundle(bundlePath)
	if err != nil {
		t.Fatalf("IsZIPBundle: %v", err)
	}
	if !ok {
		t.Error("expected ZIP bundle to return true")
	}

	// Plain text flow file
	flowPath := filepath.Join(dir, "flow.flo")
	if err := os.WriteFile(flowPath, []byte("# flow file\npath/to/packet.pkt"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	ok, err = IsZIPBundle(flowPath)
	if err != nil {
		t.Fatalf("IsZIPBundle on flow file: %v", err)
	}
	if ok {
		t.Error("expected non-ZIP to return false")
	}
}

func TestExtractBundle(t *testing.T) {
	dir := t.TempDir()
	extractDir := filepath.Join(dir, "extracted")

	pktData := []byte("fake packet data for testing")
	bundlePath := makeTestBundle(t, dir, "0003007b.mo0", "abcdef12.pkt", pktData)

	extracted, err := ExtractBundle(bundlePath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle: %v", err)
	}
	if len(extracted) != 1 {
		t.Fatalf("expected 1 extracted file, got %d", len(extracted))
	}

	got, err := os.ReadFile(extracted[0])
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(got, pktData) {
		t.Errorf("extracted content mismatch: got %q, want %q", got, pktData)
	}
}

func TestExtractBundleMultiplePkts(t *testing.T) {
	dir := t.TempDir()
	extractDir := filepath.Join(dir, "extracted")

	bundlePath := filepath.Join(dir, "multi.mo0")
	f, err := os.Create(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range []string{"first.pkt", "second.pkt", "ignore.txt"} {
		w, _ := zw.Create(name)
		w.Write([]byte("data-" + name))
	}
	zw.Close()
	f.Close()

	extracted, err := ExtractBundle(bundlePath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle: %v", err)
	}
	if len(extracted) != 2 {
		t.Errorf("expected 2 .pkt files extracted, got %d", len(extracted))
	}
}

func TestCreateAndExtractBundle(t *testing.T) {
	dir := t.TempDir()

	// Create sample .PKT files
	pkt1 := filepath.Join(dir, "test1.pkt")
	pkt2 := filepath.Join(dir, "test2.pkt")
	os.WriteFile(pkt1, []byte("packet one content"), 0644)
	os.WriteFile(pkt2, []byte("packet two content"), 0644)

	// Pack into a bundle
	bundlePath := filepath.Join(dir, "0003007b.mo0")
	count, err := CreateBundle(bundlePath, []string{pkt1, pkt2})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}

	// Verify it's a valid ZIP
	ok, err := IsZIPBundle(bundlePath)
	if err != nil || !ok {
		t.Fatal("created bundle is not a valid ZIP")
	}

	// Extract and verify contents
	extractDir := filepath.Join(dir, "out")
	extracted, err := ExtractBundle(bundlePath, extractDir)
	if err != nil {
		t.Fatalf("ExtractBundle: %v", err)
	}
	if len(extracted) != 2 {
		t.Fatalf("expected 2 extracted, got %d", len(extracted))
	}
}

func TestCreateBundleEmpty(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "empty.mo0")

	count, err := CreateBundle(bundlePath, nil)
	if err != nil {
		t.Fatalf("CreateBundle(nil): %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}
	// Bundle file should NOT be created for empty input
	if _, err := os.Stat(bundlePath); err == nil {
		t.Error("expected no bundle file created for empty input")
	}
}
