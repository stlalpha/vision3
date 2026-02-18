package ansi

import (
	"testing"
)

func TestProcessEditorPlaceholders(t *testing.T) {
	subs := map[byte]string{
		'S': "Jane",
		'F': "John Doe",
		'E': "Hello World",
		'T': "3:04 PM",
		'D': "02/18/2026",
		'I': "Ins",
	}

	tests := []struct {
		name string
		tmpl string
		want string
	}{
		{"simple", "@T@", "3:04 PM"},
		{"explicit width", "@T:8@", "3:04 PM "},
		{"visual width", "@T########@", "3:04 PM    "},
		{"left modifier", "@T|L8@", "3:04 PM "},
		{"right modifier", "@T|R8@", " 3:04 PM"},
		{"center modifier", "@T|C8@", "3:04 PM "}, // 7 chars, padding=1 -> left=0, right=1
		{"center even", "@T|C10@", " 3:04 PM  "},  // 7 chars, padding=3 -> left=1, right=2
		{"right colon width", "@T|R:8@", " 3:04 PM"},
		{"right visual width", "@T|R########@", "      3:04 PM"}, // 13-char placeholder = width 13
		{"modifier no width", "@T|R@", "3:04 PM"},                // No width = no padding/align
		{"subject right", "@E|R20@", "         Hello World"},
		{"multiple", "To: @S|L10@ Time: @T|R8@", "To: Jane       Time:  3:04 PM"},
		{"unknown code preserved", "@Q|R8@", "@Q|R8@"},
		{"no modifier", "@S:6@", "Jane  "},
		{"insert mode", "@I@", "Ins"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(ProcessEditorPlaceholders([]byte(tt.tmpl), subs))
			if got != tt.want {
				t.Errorf("ProcessEditorPlaceholders(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestProcessEditorPlaceholdersANSI(t *testing.T) {
	subs := map[byte]string{
		'T': "\x1b[32m3:04 PM\x1b[0m",
	}

	// Right-justify with ANSI: padding goes before the ANSI-decorated value
	got := string(ProcessEditorPlaceholders([]byte("@T|R10@"), subs))
	visLen := VisibleLength(got)
	if visLen != 10 {
		t.Errorf("visible length = %d, want 10", visLen)
	}
	// Should start with spaces (padding before ANSI value)
	if got[0] != ' ' {
		t.Errorf("right-justified value should start with space, got %q", got)
	}
}

func TestFindEditorPlaceholderPosWithModifier(t *testing.T) {
	// Template with modifier format: position should still be found
	template := []byte("Header\r\n@T|R8@\r\n")
	row, col, _ := FindEditorPlaceholderPos(template, 'T')
	if row != 2 || col != 1 {
		t.Errorf("FindEditorPlaceholderPos with modifier: got (%d,%d), want (2,1)", row, col)
	}
}

func TestFindEditorPlaceholderPosColor(t *testing.T) {
	// Template that sets bold cyan on blue background before @I@
	template := []byte("\x1b[0;44m     \x1b[1;36m@I@\x1b[0;44m ")
	row, col, colorEsc := FindEditorPlaceholderPos(template, 'I')
	if row != 1 || col != 6 {
		t.Errorf("FindEditorPlaceholderPos color: pos got (%d,%d), want (1,6)", row, col)
	}
	// Should restore bold, cyan fg (36), blue bg (44)
	if colorEsc == "" {
		t.Error("FindEditorPlaceholderPos color: expected non-empty color escape")
	}
	// The escape should contain bold (1), fg 36, bg 44
	want := "\x1b[0;1;36;44m"
	if colorEsc != want {
		t.Errorf("FindEditorPlaceholderPos color: got %q, want %q", colorEsc, want)
	}
}
