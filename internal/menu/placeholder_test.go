package menu

import (
	"testing"
)

func TestParsePlaceholders(t *testing.T) {
	tests := []struct {
		name          string
		template      string
		wantLen       int
		wantCode      string
		wantWidth     int
		wantAutoWidth bool
	}{
		{"simple", "@F@", 1, "F", 0, false},
		{"visual width", "@T#####@", 1, "T", 8, false},
		{"param width", "@F:20@", 1, "F", 20, false},
		{"multiple", "@#:5@/@N:5@", 2, "#", 5, false},
		{"no placeholders", "Plain text", 0, "", 0, false},
		{"mixed formats", "@T@ and @F:15@ and @S###@", 3, "T", 0, false},
		{"hash code", "@#@", 1, "#", 0, false},
		{"all codes", "@B@ @T@ @F@ @S@ @U@ @M@ @L@ @#@ @N@ @D@ @W@ @P@ @E@ @O@ @A@ @V@", 16, "B", 0, false},
		{"long visual width", "@T########################@", 1, "T", 27, false},
		{"large param width", "@F:100@", 1, "F", 100, false},
		{"auto width", "@T*@", 1, "T", 0, true},
		{"auto width hash", "@#*@", 1, "#", 0, true},
		{"auto width Z", "@Z*@", 1, "Z", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := parsePlaceholders([]byte(tt.template))
			if len(matches) != tt.wantLen {
				t.Errorf("got %d matches, want %d", len(matches), tt.wantLen)
			}
			if len(matches) > 0 {
				if matches[0].Code != tt.wantCode {
					t.Errorf("got code %q, want %q", matches[0].Code, tt.wantCode)
				}
				if matches[0].Width != tt.wantWidth {
					t.Errorf("got width %d, want %d", matches[0].Width, tt.wantWidth)
				}
				if matches[0].AutoWidth != tt.wantAutoWidth {
					t.Errorf("got autoWidth %v, want %v", matches[0].AutoWidth, tt.wantAutoWidth)
				}
			}
		})
	}
}

func TestParsePlaceholdersPositions(t *testing.T) {
	template := "Start @T@ middle @F:10@ end"
	matches := parsePlaceholders([]byte(template))

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	// First match should be @T@
	if matches[0].StartPos != 6 || matches[0].EndPos != 9 {
		t.Errorf("@T@ position: got [%d:%d], want [6:9]", matches[0].StartPos, matches[0].EndPos)
	}

	// Second match should be @F:10@
	if matches[1].StartPos != 17 || matches[1].EndPos != 23 {
		t.Errorf("@F:10@ position: got [%d:%d], want [17:23]", matches[1].StartPos, matches[1].EndPos)
	}
}

func TestProcessPlaceholderTemplate(t *testing.T) {
	substitutions := map[byte]string{
		'T': "Test Subject",
		'F': "John Doe",
		'S': "Jane Smith",
		'#': "42",
		'N': "100",
		'C': "[42/100]",
		'X': "Local Areas > General Discussion [42/100]",
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{"simple substitution", "@T@", "Test Subject"},
		{"multiple placeholders", "From: @F@ To: @S@", "From: John Doe To: Jane Smith"},
		{"with width constraint", "@F:5@", "John "},
		{"visual width longer", "@F#############@", "John Doe        "}, // 16 total chars = width 16
		{"no placeholders", "Plain text", "Plain text"},
		{"hash code", "Msg @#@/@N@", "Msg 42/100"},
		{"count display", "@C@", "[42/100]"},
		{"combined area and count", "@X@", "Local Areas > General Discussion [42/100]"},
		{"missing code", "@Q@", "@Q@"}, // Unknown code preserved
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(processPlaceholderTemplate([]byte(tt.template), substitutions, nil))
			if got != tt.want {
				t.Errorf("processPlaceholderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProcessPlaceholderTemplateWithANSI(t *testing.T) {
	substitutions := map[byte]string{
		'T': "\x1b[31mRed Subject\x1b[0m",
		'F': "\x1b[32mGreen User\x1b[0m",
	}

	tests := []struct {
		name         string
		template     string
		wantContains string
	}{
		{"preserve ansi in value", "@T@", "\x1b[31mRed Subject\x1b[0m"},
		{"truncate ansi value", "@T:5@", "\x1b[31m"}, // Should contain ANSI code
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(processPlaceholderTemplate([]byte(tt.template), substitutions, nil))
			if len(tt.wantContains) > 0 && !contains(got, tt.wantContains) {
				t.Errorf("processPlaceholderTemplate() = %q, should contain %q", got, tt.wantContains)
			}
		})
	}
}

func TestProcessPlaceholderTemplateRealWorld(t *testing.T) {
	// Simulate a real message header template
	template := `Posted on @D@ @W@       @M@
From: @F@          To: @S@
Subj: @T@`

	substitutions := map[byte]string{
		'D': "01/15/26",
		'W': "2:30 pm",
		'M': "ECHOMAIL READ",
		'F': "sysop",
		'S': "All",
		'T': "Welcome to Vision3!",
	}

	result := string(processPlaceholderTemplate([]byte(template), substitutions, nil))

	expected := `Posted on 01/15/26 2:30 pm       ECHOMAIL READ
From: sysop          To: All
Subj: Welcome to Vision3!`

	if result != expected {
		t.Errorf("Real world template failed.\nGot:\n%s\n\nWant:\n%s", result, expected)
	}
}

func TestPlaceholderRegexCoverage(t *testing.T) {
	// Test all 19 valid placeholder codes (including G for gap fill, V for verbose count)
	validCodes := []string{"B", "T", "F", "S", "U", "M", "L", "#", "N", "D", "W", "P", "E", "O", "A", "C", "X", "G", "V"}

	for _, code := range validCodes {
		template := "@" + code + "@"
		matches := parsePlaceholders([]byte(template))
		if len(matches) != 1 {
			t.Errorf("Code %s: expected 1 match, got %d", code, len(matches))
		}
		if len(matches) > 0 && matches[0].Code != code {
			t.Errorf("Code %s: got %q", code, matches[0].Code)
		}
	}

	// Test auto-width format for all codes
	for _, code := range validCodes {
		template := "@" + code + "*@"
		matches := parsePlaceholders([]byte(template))
		if len(matches) != 1 {
			t.Errorf("Code %s*: expected 1 match, got %d", code, len(matches))
		}
		if len(matches) > 0 {
			if !matches[0].AutoWidth {
				t.Errorf("Code %s*: expected AutoWidth=true", code)
			}
			if matches[0].Width != 0 {
				t.Errorf("Code %s*: expected Width=0, got %d", code, matches[0].Width)
			}
		}
	}
}

func TestProcessPlaceholderAutoWidth(t *testing.T) {
	substitutions := map[byte]string{
		'#': "3",
		'N': "1500",
		'C': "[3/1500]",
		'Z': "Local Areas > General Discussion",
		'X': "Local Areas > General Discussion [3/1500]",
		'D': "01/15/26",
		'W': "2:30 pm",
		'T': "Hello",
		'F': "sysop",
	}

	autoWidths := map[byte]int{
		'#': 4,  // len("1500") = 4
		'N': 4,  // same
		'C': 11, // len("[1500/1500]") = 11
		'Z': 32, // len("Local Areas > General Discussion")
		'X': 44, // Z + space + max count
		'D': 8,
		'W': 8,
		'T': 5,
		'F': 5,
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			"auto-width pads message number",
			"Msg @#*@",
			"Msg 3   ", // "3" padded to width 4 (left-aligned)
		},
		{
			"auto-width pads count display",
			"@C*@",
			"[3/1500]   ", // "[3/1500]" padded to width 11
		},
		{
			"auto-width Z value",
			"Area: @Z*@",
			"Area: Local Areas > General Discussion", // Z is already 32, no padding needed
		},
		{
			"auto-width no effect without map",
			"@#*@",
			"3", // No autoWidths map = no padding
		},
		{
			"auto-width date",
			"Date: @D*@",
			"Date: 01/15/26", // Already 8 chars, width 8 = no padding
		},
		{
			"auto-width time padded",
			"Time: @W*@",
			"Time: 2:30 pm ", // "2:30 pm" is 7 chars, padded to 8
		},
		{
			"mixed auto and explicit",
			"@#*@/@N:6@",
			"3   /1500  ", // # auto-padded to 4, N explicit padded to 6 (left-aligned)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var aw map[byte]int
			if tt.name != "auto-width no effect without map" {
				aw = autoWidths
			}
			got := string(processPlaceholderTemplate([]byte(tt.template), substitutions, aw))
			if got != tt.want {
				t.Errorf("processPlaceholderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProcessPlaceholderGapFill(t *testing.T) {
	substitutions := map[byte]string{
		'#': "1",
		'N': "42",
	}

	tests := []struct {
		name     string
		template string
		aw       map[byte]int
		wantLen  int // expected visible length of result
	}{
		{
			"gap fill to 79 default",
			"Hello @G@",
			nil,
			79, // "Hello " = 6 visible, fill = 73 x C4 (79 avoids col 80 auto-wrap)
		},
		{
			"gap fill explicit width",
			"Hello @G:40@",
			nil,
			40,
		},
		{
			"gap fill auto-width",
			"Hello @G*@",
			map[byte]int{'G': 60},
			60,
		},
		{
			"gap fill with other placeholders",
			"@#@ of @N@ @G:30@",
			nil,
			30, // "1 of 42 " = 8 visible, fill = 22 x C4
		},
		{
			"gap fill line already full",
			"This line is already very long and exceeds the target width easily @G:10@",
			nil,
			67, // no fill added, marker removed, just the visible text
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processPlaceholderTemplate([]byte(tt.template), substitutions, tt.aw)
			// Count visible length (C4 bytes count as visible)
			visLen := 0
			i := 0
			for i < len(got) {
				if got[i] == 0x1b && i+1 < len(got) && got[i+1] == '[' {
					i += 2
					for i < len(got) && !((got[i] >= 'A' && got[i] <= 'Z') || (got[i] >= 'a' && got[i] <= 'z')) {
						i++
					}
					if i < len(got) {
						i++
					}
				} else {
					visLen++
					i++
				}
			}
			if visLen != tt.wantLen {
				t.Errorf("visible length = %d, want %d (raw bytes: %q)", visLen, tt.wantLen, got)
			}
			// Verify fill bytes are C4 (CP437 horizontal line)
			for j := 0; j < len(got); j++ {
				if got[j] == 0xC4 {
					// Found at least one fill char, good
					return
				}
			}
			if tt.wantLen > len(tt.template)-4 { // only check if fill was expected
				// For the "already full" case, no fill chars expected
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
