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

// ---------------------------------------------------------------------------
// FindEditorPlaceholderPos — additional coverage
// ---------------------------------------------------------------------------

func TestFindEditorPlaceholderPos_NotFound(t *testing.T) {
	template := []byte("Hello World")
	row, col, colorEsc := FindEditorPlaceholderPos(template, 'T')
	if row != 0 || col != 0 || colorEsc != "" {
		t.Errorf("expected (0,0,\"\") for not-found, got (%d,%d,%q)", row, col, colorEsc)
	}
}

func TestFindEditorPlaceholderPos_CursorMovements(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		code    byte
		wantRow int
		wantCol int
	}{
		{
			name:    "cursor absolute position",
			tmpl:    "\x1b[5;10H@T@",
			code:    'T',
			wantRow: 5,
			wantCol: 10,
		},
		{
			name:    "cursor absolute with f terminator",
			tmpl:    "\x1b[3;7f@T@",
			code:    'T',
			wantRow: 3,
			wantCol: 7,
		},
		{
			name:    "cursor up",
			tmpl:    "\x1b[5;1H\x1b[2A@T@",
			code:    'T',
			wantRow: 3,
			wantCol: 1,
		},
		{
			name:    "cursor up clamped to 1",
			tmpl:    "\x1b[1;1H\x1b[10A@T@",
			code:    'T',
			wantRow: 1,
			wantCol: 1,
		},
		{
			name:    "cursor down",
			tmpl:    "\x1b[1;1H\x1b[3B@T@",
			code:    'T',
			wantRow: 4,
			wantCol: 1,
		},
		{
			name:    "cursor forward",
			tmpl:    "\x1b[1;1H\x1b[5C@T@",
			code:    'T',
			wantRow: 1,
			wantCol: 6,
		},
		{
			name:    "cursor back",
			tmpl:    "\x1b[1;10H\x1b[3D@T@",
			code:    'T',
			wantRow: 1,
			wantCol: 7,
		},
		{
			name:    "cursor back clamped to 1",
			tmpl:    "\x1b[1;2H\x1b[10D@T@",
			code:    'T',
			wantRow: 1,
			wantCol: 1,
		},
		{
			name:    "cursor position row only (no semicolon)",
			tmpl:    "\x1b[5H@T@",
			code:    'T',
			wantRow: 5,
			wantCol: 1,
		},
		{
			name:    "cursor position default (ESC[H)",
			tmpl:    "\x1b[5;5H\x1b[H@T@",
			code:    'T',
			wantRow: 1,
			wantCol: 1,
		},
		{
			name:    "tab stop",
			tmpl:    "\t@T@",
			code:    'T',
			wantRow: 1,
			wantCol: 9, // tab from col 1 -> col 9
		},
		{
			name:    "carriage return",
			tmpl:    "Hello\r@T@",
			code:    'T',
			wantRow: 1,
			wantCol: 1,
		},
		{
			name:    "newline advances row",
			tmpl:    "Hello\n@T@",
			code:    'T',
			wantRow: 2,
			wantCol: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col, _ := FindEditorPlaceholderPos([]byte(tt.tmpl), tt.code)
			if row != tt.wantRow || col != tt.wantCol {
				t.Errorf("got (%d,%d), want (%d,%d)", row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}

func TestFindEditorPlaceholderPos_DECPrivateMode(t *testing.T) {
	// ESC[?25l (hide cursor) should be fully consumed without affecting position
	template := []byte("\x1b[?25lHi@T@")
	row, col, _ := FindEditorPlaceholderPos(template, 'T')
	if row != 1 || col != 3 {
		t.Errorf("got (%d,%d), want (1,3)", row, col)
	}
}

func TestFindEditorPlaceholderPos_NonCSIEscape(t *testing.T) {
	// ESC followed by non-[ character should skip 2 bytes
	template := []byte("\x1bMHi@T@")
	row, col, _ := FindEditorPlaceholderPos(template, 'T')
	if row != 1 || col != 3 {
		t.Errorf("got (%d,%d), want (1,3)", row, col)
	}
}

func TestFindEditorPlaceholderPos_IncompleteCSI(t *testing.T) {
	// ESC[ with digits but no terminator — should break out of loop
	template := []byte("\x1b[999")
	row, col, _ := FindEditorPlaceholderPos(template, 'T')
	if row != 0 || col != 0 {
		t.Errorf("expected (0,0) for not-found, got (%d,%d)", row, col)
	}
}

func TestFindEditorPlaceholderPos_PlaceholderFormats(t *testing.T) {
	// Test all placeholder trigger characters: @, :, #, |
	tests := []struct {
		name string
		tmpl string
		code byte
		want bool
	}{
		{"@T@", "@T@", 'T', true},
		{"@T:8@", "@T:8@", 'T', true},
		{"@T#@", "@T####@", 'T', true},
		{"@T|R@", "@T|R@", 'T', true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, _, _ := FindEditorPlaceholderPos([]byte(tt.tmpl), tt.code)
			found := row != 0
			if found != tt.want {
				t.Errorf("found=%v, want=%v", found, tt.want)
			}
		})
	}
}

func TestFindEditorPlaceholderPos_SGRTracking(t *testing.T) {
	// Verify SGR state is tracked through the scan
	tests := []struct {
		name      string
		tmpl      string
		code      byte
		wantColor string
	}{
		{
			name:      "bold + red fg",
			tmpl:      "\x1b[1;31m@T@",
			code:      'T',
			wantColor: "\x1b[0;1;31;0m",
		},
		{
			name:      "reset clears state",
			tmpl:      "\x1b[1;31m\x1b[0m@T@",
			code:      'T',
			wantColor: "\x1b[0m",
		},
		{
			name:      "faint attribute",
			tmpl:      "\x1b[2m@T@",
			code:      'T',
			wantColor: "\x1b[0;2;0;0m",
		},
		{
			name:      "blink attribute",
			tmpl:      "\x1b[5m@T@",
			code:      'T',
			wantColor: "\x1b[0;5;0;0m",
		},
		{
			name:      "background color",
			tmpl:      "\x1b[42m@T@",
			code:      'T',
			wantColor: "\x1b[0;0;42m",
		},
		{
			name:      "bright fg",
			tmpl:      "\x1b[91m@T@",
			code:      'T',
			wantColor: "\x1b[0;91;0m",
		},
		{
			name:      "bright bg",
			tmpl:      "\x1b[104m@T@",
			code:      'T',
			wantColor: "\x1b[0;0;104m",
		},
		{
			name:      "empty params resets (ESC[m)",
			tmpl:      "\x1b[1;31m\x1b[m@T@",
			code:      'T',
			wantColor: "\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, colorEsc := FindEditorPlaceholderPos([]byte(tt.tmpl), tt.code)
			if colorEsc != tt.wantColor {
				t.Errorf("color = %q, want %q", colorEsc, tt.wantColor)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FindEditorColorAtPos
// ---------------------------------------------------------------------------

func TestFindEditorColorAtPos_Basic(t *testing.T) {
	tests := []struct {
		name      string
		tmpl      string
		row       int
		col       int
		wantColor string
	}{
		{
			name:      "no color at position",
			tmpl:      "Hello",
			row:       1,
			col:       1,
			wantColor: "\x1b[0;0;0m",
		},
		{
			name:      "red fg at position",
			tmpl:      "\x1b[31mHello",
			row:       1,
			col:       1,
			wantColor: "\x1b[0;31;0m",
		},
		{
			name:      "color at second row",
			tmpl:      "Line1\n\x1b[32mLine2",
			row:       2,
			col:       1,
			wantColor: "\x1b[0;32;0m",
		},
		{
			name:      "position not reached",
			tmpl:      "Hi",
			row:       5,
			col:       1,
			wantColor: "",
		},
		{
			name:      "past target row returns empty",
			tmpl:      "Line1\nLine2\nLine3",
			row:       1,
			col:       100,
			wantColor: "",
		},
		{
			name:      "tab stop position",
			tmpl:      "\x1b[33m\tX",
			row:       1,
			col:       9,
			wantColor: "\x1b[0;33;0m",
		},
		{
			name:      "carriage return resets col",
			tmpl:      "Hello\r\x1b[34mX",
			row:       1,
			col:       1,
			wantColor: "\x1b[0;0;0m", // returns color at first encounter of col=1 ('H'), before \r
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindEditorColorAtPos([]byte(tt.tmpl), tt.row, tt.col)
			if got != tt.wantColor {
				t.Errorf("FindEditorColorAtPos(%q, %d, %d) = %q, want %q",
					tt.tmpl, tt.row, tt.col, got, tt.wantColor)
			}
		})
	}
}

func TestFindEditorColorAtPos_CursorMovements(t *testing.T) {
	tests := []struct {
		name      string
		tmpl      string
		row       int
		col       int
		wantColor string
	}{
		{
			name:      "cursor position H",
			tmpl:      "\x1b[31m\x1b[3;5HX",
			row:       3,
			col:       5,
			wantColor: "\x1b[0;31;0m",
		},
		{
			name:      "cursor position f",
			tmpl:      "\x1b[32m\x1b[2;3fX",
			row:       2,
			col:       3,
			wantColor: "\x1b[0;32;0m",
		},
		{
			name:      "cursor up",
			tmpl:      "\x1b[33m\x1b[5;1H\x1b[2AX",
			row:       3,
			col:       1,
			wantColor: "\x1b[0;33;0m",
		},
		{
			name:      "cursor down",
			tmpl:      "\x1b[34m\x1b[1;1H\x1b[2BX",
			row:       3,
			col:       1,
			wantColor: "\x1b[0;34;0m",
		},
		{
			name:      "cursor forward",
			tmpl:      "\x1b[35m\x1b[1;1H\x1b[4CX",
			row:       1,
			col:       5,
			wantColor: "\x1b[0;35;0m",
		},
		{
			name:      "cursor back",
			tmpl:      "\x1b[36m\x1b[1;10H\x1b[3DX",
			row:       1,
			col:       7,
			wantColor: "\x1b[0;36;0m",
		},
		{
			name:      "cursor back clamped",
			tmpl:      "\x1b[37m\x1b[1;2H\x1b[10DX",
			row:       1,
			col:       1,
			wantColor: "\x1b[0;37;0m",
		},
		{
			name:      "cursor up clamped",
			tmpl:      "\x1b[31m\x1b[1;1H\x1b[10AX",
			row:       1,
			col:       1,
			wantColor: "\x1b[0;31;0m",
		},
		{
			name:      "cursor row only (no semicolon)",
			tmpl:      "\x1b[32m\x1b[5HX",
			row:       5,
			col:       1,
			wantColor: "\x1b[0;32;0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindEditorColorAtPos([]byte(tt.tmpl), tt.row, tt.col)
			if got != tt.wantColor {
				t.Errorf("got %q, want %q", got, tt.wantColor)
			}
		})
	}
}

func TestFindEditorColorAtPos_DECPrivateMode(t *testing.T) {
	// ESC[?25l should be consumed without error
	tmpl := []byte("\x1b[?25l\x1b[31mHi")
	got := FindEditorColorAtPos(tmpl, 1, 1)
	if got != "\x1b[0;31;0m" {
		t.Errorf("got %q, want %q", got, "\x1b[0;31;0m")
	}
}

func TestFindEditorColorAtPos_NonCSIEscape(t *testing.T) {
	// ESC without [ should skip 2 bytes
	tmpl := []byte("\x1bM\x1b[31mHi")
	got := FindEditorColorAtPos(tmpl, 1, 1)
	if got != "\x1b[0;31;0m" {
		t.Errorf("got %q, want %q", got, "\x1b[0;31;0m")
	}
}

func TestFindEditorColorAtPos_IncompleteCSI(t *testing.T) {
	// ESC[ with digits but no terminator at end
	tmpl := []byte("\x1b[999")
	got := FindEditorColorAtPos(tmpl, 1, 1)
	if got != "" {
		t.Errorf("expected empty for incomplete CSI, got %q", got)
	}
}

func TestFindEditorColorAtPos_SGRAttributes(t *testing.T) {
	tests := []struct {
		name      string
		tmpl      string
		wantColor string
	}{
		{"bold", "\x1b[1mX", "\x1b[0;1;0;0m"},
		{"faint", "\x1b[2mX", "\x1b[0;2;0;0m"},
		{"blink", "\x1b[5mX", "\x1b[0;5;0;0m"},
		{"reset bold (22)", "\x1b[1m\x1b[22mX", "\x1b[0;0;0m"},
		{"reset blink (25)", "\x1b[5m\x1b[25mX", "\x1b[0;0;0m"},
		{"default fg (39)", "\x1b[31m\x1b[39mX", "\x1b[0;0m"},
		{"default bg (49)", "\x1b[41m\x1b[49mX", "\x1b[0;0m"},
		{"bright fg", "\x1b[91mX", "\x1b[0;91;0m"},
		{"bright bg", "\x1b[104mX", "\x1b[0;0;104m"},
		{"empty params reset", "\x1b[1;31m\x1b[mX", "\x1b[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindEditorColorAtPos([]byte(tt.tmpl), 1, 1)
			if got != tt.wantColor {
				t.Errorf("got %q, want %q", got, tt.wantColor)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseSingleParam
// ---------------------------------------------------------------------------

func TestParseSingleParam(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
		def  int
		want int
	}{
		{"empty returns default", []byte{}, 1, 1},
		{"valid number", []byte("5"), 1, 5},
		{"zero returns default", []byte("0"), 1, 1},
		{"large number", []byte("100"), 1, 100},
		{"invalid returns default", []byte("abc"), 1, 1},
		{"default 0", []byte{}, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSingleParam(tt.b, tt.def)
			if got != tt.want {
				t.Errorf("parseSingleParam(%q, %d) = %d, want %d", tt.b, tt.def, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// applyParams
// ---------------------------------------------------------------------------

func TestApplyParams(t *testing.T) {
	tests := []struct {
		name     string
		initial  sgrState
		paramStr string
		want     sgrState
	}{
		{
			name:     "empty resets",
			initial:  sgrState{bold: true, fg: 31, bg: 42},
			paramStr: "",
			want:     sgrState{fg: -1, bg: -1},
		},
		{
			name:     "reset with 0",
			initial:  sgrState{bold: true, fg: 31},
			paramStr: "0",
			want:     sgrState{fg: -1, bg: -1},
		},
		{
			name:     "bold",
			initial:  sgrState{fg: -1, bg: -1},
			paramStr: "1",
			want:     sgrState{bold: true, fg: -1, bg: -1},
		},
		{
			name:     "faint",
			initial:  sgrState{fg: -1, bg: -1},
			paramStr: "2",
			want:     sgrState{faint: true, fg: -1, bg: -1},
		},
		{
			name:     "blink",
			initial:  sgrState{fg: -1, bg: -1},
			paramStr: "5",
			want:     sgrState{blink: true, fg: -1, bg: -1},
		},
		{
			name:     "normal intensity resets bold and faint",
			initial:  sgrState{bold: true, faint: true, fg: -1, bg: -1},
			paramStr: "22",
			want:     sgrState{fg: -1, bg: -1},
		},
		{
			name:     "blink off",
			initial:  sgrState{blink: true, fg: -1, bg: -1},
			paramStr: "25",
			want:     sgrState{fg: -1, bg: -1},
		},
		{
			name:     "foreground color",
			initial:  sgrState{fg: -1, bg: -1},
			paramStr: "31",
			want:     sgrState{fg: 31, bg: -1},
		},
		{
			name:     "default fg",
			initial:  sgrState{fg: 31, bg: -1},
			paramStr: "39",
			want:     sgrState{fg: -1, bg: -1},
		},
		{
			name:     "background color",
			initial:  sgrState{fg: -1, bg: -1},
			paramStr: "42",
			want:     sgrState{fg: -1, bg: 42},
		},
		{
			name:     "default bg",
			initial:  sgrState{fg: -1, bg: 42},
			paramStr: "49",
			want:     sgrState{fg: -1, bg: -1},
		},
		{
			name:     "bright fg",
			initial:  sgrState{fg: -1, bg: -1},
			paramStr: "91",
			want:     sgrState{fg: 91, bg: -1},
		},
		{
			name:     "bright bg",
			initial:  sgrState{fg: -1, bg: -1},
			paramStr: "104",
			want:     sgrState{fg: -1, bg: 104},
		},
		{
			name:     "multiple params",
			initial:  sgrState{fg: -1, bg: -1},
			paramStr: "1;31;42",
			want:     sgrState{bold: true, fg: 31, bg: 42},
		},
		{
			name:     "semicolon with missing param treated as 0 (reset)",
			initial:  sgrState{bold: true, fg: 31, bg: -1},
			paramStr: ";31",
			want:     sgrState{fg: 31, bg: -1}, // leading ; = param 0 = reset, then 31
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.initial
			s.applyParams(tt.paramStr)
			if s != tt.want {
				t.Errorf("after applyParams(%q): got %+v, want %+v", tt.paramStr, s, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// sgrState.escape
// ---------------------------------------------------------------------------

func TestSGRStateEscape(t *testing.T) {
	tests := []struct {
		name string
		s    sgrState
		want string
	}{
		{"default", sgrState{fg: -1, bg: -1}, "\x1b[0m"},
		{"bold only", sgrState{bold: true, fg: -1, bg: -1}, "\x1b[0;1m"},
		{"faint only", sgrState{faint: true, fg: -1, bg: -1}, "\x1b[0;2m"},
		{"blink only", sgrState{blink: true, fg: -1, bg: -1}, "\x1b[0;5m"},
		{"fg only", sgrState{fg: 31, bg: -1}, "\x1b[0;31m"},
		{"bg only", sgrState{fg: -1, bg: 42}, "\x1b[0;42m"},
		{"all set", sgrState{bold: true, faint: true, blink: true, fg: 33, bg: 44}, "\x1b[0;1;2;5;33;44m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.s.escape()
			if got != tt.want {
				t.Errorf("escape() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitSGR
// ---------------------------------------------------------------------------

func TestSplitSGR(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want []int
	}{
		{"single", "31", []int{31}},
		{"multiple", "1;31;42", []int{1, 31, 42}},
		{"leading semicolon", ";31", []int{0, 31}},
		{"trailing semicolon", "31;", []int{31, 0}},
		{"empty between semicolons", "1;;31", []int{1, 0, 31}},
		{"just zero", "0", []int{0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSGR(tt.s)
			if len(got) != len(tt.want) {
				t.Fatalf("splitSGR(%q) = %v (len %d), want %v (len %d)", tt.s, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitSGR(%q)[%d] = %d, want %d", tt.s, i, got[i], tt.want[i])
				}
			}
		})
	}
}
