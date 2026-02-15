package ziplab

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseNFO_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	nfoContent := `; Test NFO
; Activity 1 = Test Integrity
1D = 45,10,112,116,███
1P = 58,10,112,114,███
1F = 64,10,112,126,███
; Activity 2 = UnZIP
2D = 45,11,112,116,███
2P = 58,11,112,114,███
2F = 64,11,112,126,███
`
	os.WriteFile(filepath.Join(tmpDir, "ZIPLAB.NFO"), []byte(nfoContent), 0644)

	nfo, err := ParseNFO(filepath.Join(tmpDir, "ZIPLAB.NFO"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have entries for steps 1 and 2
	if len(nfo.Entries) != 6 {
		t.Errorf("expected 6 entries, got %d", len(nfo.Entries))
	}

	// Check step 1 Doing
	entry, ok := nfo.GetEntry(1, StatusDoing)
	if !ok {
		t.Fatal("expected entry for step 1, status D")
	}
	if entry.Col != 45 || entry.Row != 10 {
		t.Errorf("expected col=45, row=10, got col=%d, row=%d", entry.Col, entry.Row)
	}
	if entry.NormalColor != 112 {
		t.Errorf("expected NormalColor=112, got %d", entry.NormalColor)
	}
	if entry.HiColor != 116 {
		t.Errorf("expected HiColor=116, got %d", entry.HiColor)
	}

	// Check step 1 Pass
	entry, ok = nfo.GetEntry(1, StatusPass)
	if !ok {
		t.Fatal("expected entry for step 1, status P")
	}
	if entry.Col != 58 {
		t.Errorf("expected col=58 for pass, got %d", entry.Col)
	}

	// Check step 1 Fail
	entry, ok = nfo.GetEntry(1, StatusFail)
	if !ok {
		t.Fatal("expected entry for step 1, status F")
	}
	if entry.Col != 64 {
		t.Errorf("expected col=64 for fail, got %d", entry.Col)
	}
}

func TestParseNFO_SkipsComments(t *testing.T) {
	tmpDir := t.TempDir()
	nfoContent := `; This is a comment
; Another comment
1D = 45,10,112,116,███
`
	os.WriteFile(filepath.Join(tmpDir, "test.nfo"), []byte(nfoContent), 0644)

	nfo, err := ParseNFO(filepath.Join(tmpDir, "test.nfo"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nfo.Entries) != 1 {
		t.Errorf("expected 1 entry (comments skipped), got %d", len(nfo.Entries))
	}
}

func TestParseNFO_MissingFile(t *testing.T) {
	_, err := ParseNFO("/nonexistent/ZIPLAB.NFO")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseNFO_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "empty.nfo"), []byte("; only comments\n"), 0644)

	nfo, err := ParseNFO(filepath.Join(tmpDir, "empty.nfo"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nfo.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(nfo.Entries))
	}
}

func TestDOSColorToANSI_Foreground(t *testing.T) {
	tests := []struct {
		dosAttr  int
		wantFG   int
		wantBG   int
		wantBold bool
	}{
		{112, 0, 7, false},  // Light gray BG, black FG
		{116, 4, 7, false},  // Light gray BG, red FG (4 < 8, not bold)
		{114, 2, 7, false},  // Light gray BG, green FG (2 < 8, not bold)
		{126, 14, 7, true},  // Light gray BG, bright yellow FG (14 >= 8, bold)
		{7, 7, 0, false},    // Black BG, light gray FG
		{12, 12, 0, true},   // Black BG, bright red FG (12 >= 8, bold)
	}

	for _, tt := range tests {
		fg, bg, bold := DOSColorToANSI(tt.dosAttr)
		if fg != tt.wantFG || bg != tt.wantBG || bold != tt.wantBold {
			t.Errorf("DOSColorToANSI(%d) = fg=%d bg=%d bold=%v, want fg=%d bg=%d bold=%v",
				tt.dosAttr, fg, bg, bold, tt.wantFG, tt.wantBG, tt.wantBold)
		}
	}
}

func TestNFOConfig_GetEntry_NotFound(t *testing.T) {
	nfo := &NFOConfig{Entries: make(map[string]NFOEntry)}

	_, ok := nfo.GetEntry(99, StatusDoing)
	if ok {
		t.Error("expected not found for non-existent step")
	}
}

func TestNFOConfig_StepEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	// Only step 1 and 3 defined — step 2 is missing (disabled/skipped)
	nfoContent := `1D = 45,10,112,116,███
1P = 58,10,112,114,███
1F = 64,10,112,126,███
3D = 45,12,112,116,███
3P = 58,12,112,114,███
3F = 64,12,112,126,███
`
	os.WriteFile(filepath.Join(tmpDir, "test.nfo"), []byte(nfoContent), 0644)

	nfo, _ := ParseNFO(filepath.Join(tmpDir, "test.nfo"))

	if !nfo.HasStep(1) {
		t.Error("expected step 1 to be present")
	}
	if nfo.HasStep(2) {
		t.Error("expected step 2 to be absent")
	}
	if !nfo.HasStep(3) {
		t.Error("expected step 3 to be present")
	}
}

func TestBuildStatusSequence(t *testing.T) {
	nfo := &NFOConfig{
		Entries: map[string]NFOEntry{
			"1D": {Step: 1, Status: StatusDoing, Col: 45, Row: 10, NormalColor: 112, HiColor: 116, DisplayChars: "███"},
		},
	}

	seq := nfo.BuildStatusSequence(1, StatusDoing)
	if seq == "" {
		t.Error("expected non-empty ANSI sequence")
	}
	// Should contain cursor positioning escape
	if len(seq) < 5 {
		t.Errorf("sequence too short: %q", seq)
	}
}
