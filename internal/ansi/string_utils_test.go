package ansi

import (
	"testing"
)

func TestVisibleLength(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"plain text", "Hello", 5},
		{"red text", "\x1b[31mRed\x1b[0m", 3},
		{"green text with bold", "\x1b[1;32mGreen Text\x1b[0m", 10},
		{"empty string", "", 0},
		{"only ansi codes", "\x1b[31m\x1b[0m", 0},
		{"multiple colors", "\x1b[31mRed\x1b[0m \x1b[32mGreen\x1b[0m", 9}, // "Red Green"
		{"bright colors", "\x1b[1;31mBright Red\x1b[0m", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VisibleLength(tt.input)
			if got != tt.want {
				t.Errorf("VisibleLength(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateVisible(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		max        int
		wantVisLen int
	}{
		{"plain text under limit", "Hello", 10, 5},
		{"plain text exact limit", "Hello", 5, 5},
		{"plain text over limit", "Hello World", 5, 5},
		{"red text under limit", "\x1b[31mRed Text\x1b[0m", 10, 8},
		{"red text over limit", "\x1b[31mRed Text\x1b[0m", 3, 3},
		{"zero limit", "Hello", 0, 0},
		{"empty string", "", 5, 0},
		{"only ansi codes", "\x1b[31m\x1b[0m", 5, 0},
		{"colored text truncated", "\x1b[32mGreen World\x1b[0m", 5, 5}, // "Green"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateVisible(tt.input, tt.max)
			gotVis := VisibleLength(got)
			if gotVis != tt.wantVisLen {
				t.Errorf("TruncateVisible(%q, %d) visible length = %d, want %d", tt.input, tt.max, gotVis, tt.wantVisLen)
			}
			// Verify we don't exceed max
			if gotVis > tt.max {
				t.Errorf("TruncateVisible exceeded max: got %d, max %d", gotVis, tt.max)
			}
		})
	}
}

func TestPadVisible(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		width      int
		padChar    rune
		wantVisLen int
	}{
		{"pad plain text", "Hello", 10, ' ', 10},
		{"no pad needed", "Hello", 5, ' ', 5},
		{"no pad under width", "Hello", 3, ' ', 5}, // Already longer
		{"pad red text", "\x1b[31mRed\x1b[0m", 10, ' ', 10},
		{"pad with dash", "Test", 8, '-', 8},
		{"empty string pad", "", 5, ' ', 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PadVisible(tt.input, tt.width, tt.padChar)
			gotVis := VisibleLength(got)
			if gotVis != tt.wantVisLen {
				t.Errorf("PadVisible(%q, %d, %q) visible length = %d, want %d", tt.input, tt.width, tt.padChar, gotVis, tt.wantVisLen)
			}
		})
	}
}

func TestApplyWidthConstraint(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		width      int
		wantVisLen int
	}{
		{"short text padded", "Hi", 10, 10},
		{"long text truncated", "Hello World", 5, 5},
		{"exact width", "Hello", 5, 5},
		{"red text truncated and padded", "\x1b[31mRed\x1b[0m", 10, 10},
		{"colored long text", "\x1b[32mGreen World\x1b[0m", 8, 8},
		{"zero width", "Test", 0, 4}, // Should return as-is when width <= 0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyWidthConstraint(tt.input, tt.width)
			gotVis := VisibleLength(got)
			if gotVis != tt.wantVisLen {
				t.Errorf("ApplyWidthConstraint(%q, %d) visible length = %d, want %d", tt.input, tt.width, gotVis, tt.wantVisLen)
			}
		})
	}
}

// Test edge cases and special scenarios
func TestANSIEdgeCases(t *testing.T) {
	// Test that ANSI codes are preserved during truncation
	input := "\x1b[31mRed Text\x1b[0m"
	result := TruncateVisible(input, 3)
	if !hasANSI(result) {
		t.Error("TruncateVisible should preserve ANSI codes")
	}

	// Test multiple ANSI sequences
	input2 := "\x1b[1m\x1b[31mBold Red\x1b[0m"
	result2 := VisibleLength(input2)
	if result2 != 8 {
		t.Errorf("VisibleLength with multiple ANSI sequences = %d, want 8", result2)
	}
}

// Helper function to check if string contains ANSI codes
func hasANSI(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '\x1b' && s[i+1] == '[' {
			return true
		}
	}
	return false
}

func TestApplyWidthConstraintAligned(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		width      int
		align      Alignment
		want       string
		wantVisLen int
	}{
		{"left-align short", "Hi", 8, AlignLeft, "Hi      ", 8},
		{"right-align short", "Hi", 8, AlignRight, "      Hi", 8},
		{"center short", "Hi", 8, AlignCenter, "   Hi   ", 8},
		{"center odd", "Hi", 7, AlignCenter, "  Hi   ", 7},
		{"left exact", "Hello", 5, AlignLeft, "Hello", 5},
		{"right exact", "Hello", 5, AlignRight, "Hello", 5},
		{"center exact", "Hello", 5, AlignCenter, "Hello", 5},
		{"right truncated", "Hello World", 5, AlignRight, "Hello", 5},
		{"right with ANSI", "\x1b[31mHi\x1b[0m", 8, AlignRight, "      \x1b[31mHi\x1b[0m", 8},
		{"center with ANSI", "\x1b[32mOK\x1b[0m", 8, AlignCenter, "   \x1b[32mOK\x1b[0m   ", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ApplyWidthConstraintAligned(tt.input, tt.width, tt.align)
			if got != tt.want {
				t.Errorf("ApplyWidthConstraintAligned(%q, %d, %v) = %q, want %q", tt.input, tt.width, tt.align, got, tt.want)
			}
			gotVis := VisibleLength(got)
			if gotVis != tt.wantVisLen {
				t.Errorf("ApplyWidthConstraintAligned(%q, %d, %v) visible length = %d, want %d", tt.input, tt.width, tt.align, gotVis, tt.wantVisLen)
			}
		})
	}
}

func TestParseAlignment(t *testing.T) {
	tests := []struct {
		input string
		want  Alignment
	}{
		{"L", AlignLeft},
		{"R", AlignRight},
		{"C", AlignCenter},
		{"X", AlignLeft},
		{"", AlignLeft},
	}
	for _, tt := range tests {
		got := ParseAlignment(tt.input)
		if got != tt.want {
			t.Errorf("ParseAlignment(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
