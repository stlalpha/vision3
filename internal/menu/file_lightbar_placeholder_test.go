package menu

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stlalpha/vision3/internal/ansi"
)

func TestFileListPlaceholderRegex(t *testing.T) {
	r := regexp.MustCompile(`@(FPAGE|FTOTAL|FCONFPATH)(?:[\x7C\xB3]([LRC]))?(?::(\d+)|(#+))?@`)
	tests := []struct {
		template string
		wantSubs []string
	}{
		{"@FPAGE|R#####@", []string{"@FPAGE|R#####@", "FPAGE", "R", "", "#####"}},
		{"@FTOTAL|R#@", []string{"@FTOTAL|R#@", "FTOTAL", "R", "", "#"}},
		{"@FPAGE#####@", []string{"@FPAGE#####@", "FPAGE", "", "", "#####"}},
		{"@FPAGE|R:15@", []string{"@FPAGE|R:15@", "FPAGE", "R", "15", ""}},
	}
	for _, tt := range tests {
		subs := r.FindStringSubmatch(tt.template)
		if len(subs) != len(tt.wantSubs) {
			t.Errorf("%q: got %d subs, want %d: %q", tt.template, len(subs), len(tt.wantSubs), subs)
			continue
		}
		for i := range tt.wantSubs {
			if subs[i] != tt.wantSubs[i] {
				t.Errorf("%q subs[%d]: got %q, want %q", tt.template, i, subs[i], tt.wantSubs[i])
			}
		}
	}
}

func TestProcessFileListPlaceholdersAlignment(t *testing.T) {
	fconfpath := "Local > General Files"
	// Width = entire placeholder length (20 chars for @FPAGE|R###########@)
	data := []byte("X@FPAGE|R###########@Y")
	result := processFileListPlaceholders(data, 1, 1, 1, fconfpath)
	got := string(result)
	// 20 cols, right-aligned: "         Page 1 of 1" (9 spaces + 11)
	want := "X         Page 1 of 1Y"
	if got != want {
		t.Errorf("got %q, want %q (len got=%d want=%d)", got, want, len(got), len(want))
	}
}

func TestProcessFileListPlaceholdersModifierNotDestroyed(t *testing.T) {
	// Regression: |FPAGE pipe code must not consume the |FPAGE inside @FPAGE|R#####@
	fconfpath := "Local > General Files"
	data := []byte("@FPAGE|R#####@")
	result := string(processFileListPlaceholders(data, 1, 1, 1, fconfpath))
	// Placeholder is 14 chars. "Page 1 of 1" is 11. Right-align = 3 spaces left.
	want := "   Page 1 of 1"
	if result != want {
		t.Errorf("got %q (len=%d), want %q (len=%d)", result, len(result), want, len(want))
	}
}

func TestProcessFileListPlaceholdersFullTemplate(t *testing.T) {
	// Exact user template (plain text, no embedded pipe codes)
	fconfpath := "Local Areas > General Files"
	fconfpathAnsi := fconfpath
	data := []byte("\xfe @FCONFPATH################################@ Files: @FTOTAL|R#@ @FPAGE|R#####@ ")
	result := string(processFileListPlaceholders(data, 1, 1, 1, fconfpath))

	// FCONFPATH: 43-char placeholder, value with ANSI color codes padded to 43 visible
	// FTOTAL|R: 11-char placeholder, "1" right-aligned
	// FPAGE|R: 14-char placeholder, "Page 1 of 1" right-aligned
	t.Logf("fconfpath ANSI: %q", fconfpathAnsi)
	t.Logf("result: %q", result)
	t.Logf("result len: %d", len(result))

	// Check FPAGE is right-justified (should have leading spaces)
	if !strings.Contains(result, "   Page 1 of 1") {
		t.Errorf("FPAGE not right-justified. Result: %q", result)
	}
	// Check FTOTAL is right-justified (should have leading spaces)
	if !strings.Contains(result, "          1") {
		t.Errorf("FTOTAL not right-justified. Result: %q", result)
	}
	// Check no trailing spaces after "Page 1 of 1" (except the 1 from template)
	idx := strings.Index(result, "Page 1 of 1")
	if idx >= 0 {
		after := result[idx+len("Page 1 of 1"):]
		if after != " " {
			t.Errorf("Expected exactly 1 trailing space after FPAGE, got %q", after)
		}
	}
}

func TestProcessFileListPlaceholdersRightAlign(t *testing.T) {
	// Verify right alignment produces spaces on LEFT
	pageStr := "Page 1 of 1"
	width := 15
	aligned := ansi.ApplyWidthConstraintAligned(pageStr, width, ansi.AlignRight)
	got := aligned
	want := "    Page 1 of 1"
	if got != want {
		t.Errorf("AlignRight: got %q (len=%d), want %q", got, len(got), want)
	}
	// Verify it does NOT have trailing spaces
	if strings.HasSuffix(got, " ") {
		t.Errorf("AlignRight should pad left, got trailing spaces: %q", got)
	}
}
