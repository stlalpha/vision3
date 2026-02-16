package menu

import (
	"testing"
)

func TestParsePlaceholders(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		wantLen   int
		wantCode  string
		wantWidth int
	}{
		{"simple", "@F@", 1, "F", 0},
		{"visual width", "@T#####@", 1, "T", 5},
		{"param width", "@F:20@", 1, "F", 20},
		{"multiple", "@#:5@/@N:5@", 2, "#", 5},
		{"no placeholders", "Plain text", 0, "", 0},
		{"mixed formats", "@T@ and @F:15@ and @S###@", 3, "T", 0},
		{"hash code", "@#@", 1, "#", 0},
		{"all codes", "@B@ @T@ @F@ @S@ @U@ @M@ @L@ @R@ @#@ @N@ @D@ @W@ @P@ @E@ @O@ @A@", 16, "B", 0},
		{"long visual width", "@T########################@", 1, "T", 24},
		{"large param width", "@F:100@", 1, "F", 100},
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
		{"missing code", "@X@", "@X@"}, // Unknown code preserved
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(processPlaceholderTemplate([]byte(tt.template), substitutions))
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
			got := string(processPlaceholderTemplate([]byte(tt.template), substitutions))
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

	result := string(processPlaceholderTemplate([]byte(template), substitutions))

	expected := `Posted on 01/15/26 2:30 pm       ECHOMAIL READ
From: sysop          To: All
Subj: Welcome to Vision3!`

	if result != expected {
		t.Errorf("Real world template failed.\nGot:\n%s\n\nWant:\n%s", result, expected)
	}
}

func TestPlaceholderRegexCoverage(t *testing.T) {
	// Test all 16 valid placeholder codes
	validCodes := []string{"B", "T", "F", "S", "U", "M", "L", "R", "#", "N", "D", "W", "P", "E", "O", "A"}

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
